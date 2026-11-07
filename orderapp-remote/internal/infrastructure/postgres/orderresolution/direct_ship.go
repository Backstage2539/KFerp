package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
)

// A package belongs to one sales order. Releasing its unshipped reservation
// never changes sibling orders, physical inventory, fees, or cash receipts.
func (r Repository) cancelDirectShip(ctx context.Context, tx pgx.Tx, orderID, resolutionID, requestID int64, note, actor string) error {
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT d.status FROM %[1]s.customer_direct_ship_requests d JOIN %[1]s.orders o ON o.customer_id=d.customer_id WHERE d.id=$1 AND o.id=$2 FOR UPDATE OF d`, r.schema), requestID, orderID).Scan(&status); err != nil {
		return err
	}
	var own bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_direct_ship_request_orders WHERE request_id=$1 AND order_id=$2)`, r.schema), requestID, orderID).Scan(&own); err != nil {
		return err
	}
	if !own {
		var linked, active bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id WHERE s.request_id=$1 AND e.source_order_id=$2),EXISTS(SELECT 1 FROM %[1]s.customer_direct_ship_request_orders WHERE request_id=$1 AND status<>'cancelled')`, r.schema), requestID, orderID).Scan(&linked, &active); err != nil {
			return err
		}
		if !linked {
			return fmt.Errorf("代发申请不属于本订单的关联需求")
		}
		if status != "cancelled" || active {
			return fmt.Errorf("请先打开关联代发订单，逐笔选择费用、资金和预订处置；全部取消后再确认本项")
		}
		return nil
	}
	var packageStatus string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_direct_ship_request_orders WHERE request_id=$1 AND order_id=$2 FOR UPDATE`, r.schema), requestID, orderID).Scan(&packageStatus); err != nil {
		return err
	}
	if packageStatus == "cancelled" {
		return nil
	}
	var shipped bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(o.ship_tracking_no,'')<>'' OR COALESCE(s.name,'') IN('已发货','已出库','已签收','已收货','已完成') OR EXISTS(SELECT 1 FROM %[1]s.order_stock_deductions WHERE order_id=o.id) OR EXISTS(SELECT 1 FROM %[1]s.order_shipment_orders WHERE order_id=o.id AND shipped_at IS NOT NULL) FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id WHERE o.id=$1`, r.schema), orderID).Scan(&shipped); err != nil {
		return err
	}
	if shipped || one(packageStatus, "shipped", "delivered", "completed") {
		return fmt.Errorf("包裹已有发货或运单记录，请先由仓库核实撤销发货或办理退货")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.order_resolution_reservation_releases(request_id,order_id,allocation_id,snapshot_json,actor)SELECT $1,$2,a.id,to_jsonb(a),$3 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=$2 AND a.request_id=$4 AND(a.allocated_units>0 OR a.allocated_g>0)`, r.schema), resolutionID, orderID, actor, requestID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_stock_batch_allocations SET allocated_g=0,allocated_units=0 WHERE order_id=$1 AND request_id=$2`, r.schema), orderID, requestID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_allocations SET status='released' WHERE request_id=$1 AND order_id=$2 AND status='reserved'`, r.schema), requestID, orderID); err != nil {
		return err
	}
	for _, table := range []string{"customer_processing_output_reservations", "customer_stock_supply_reservations"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.%s SET released_qty=GREATEST(released_qty,reserved_qty-converted_qty),status='released',updated_at=now() WHERE request_id=$1 AND order_id=$2 AND status IN('reserved','partially_converted')`, r.schema, table), requestID, orderID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_direct_ship_request_orders SET status='cancelled',updated_at=now()WHERE request_id=$1 AND order_id=$2`, r.schema), requestID, orderID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.customer_direct_ship_requests SET status='cancelled',cancelled_at=now(),updated_at=now()WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM %[1]s.customer_direct_ship_request_orders WHERE request_id=$1 AND status<>'cancelled')`, r.schema), requestID); err != nil {
		return err
	}
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_direct_ship_request", &requestID, "resolution_cancel_package", nil, nil, nil, infra.AuditMeta{"order_id": orderID, "resolution_id": resolutionID, "note": note, "physical_stock_unchanged": true})
}
