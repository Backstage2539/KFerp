package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
)

// Only the pending portion can change source. Bind the user's explicit choice
// to the original physical allocations and receipt history; never rebuild them.
func (r Repository) convertedSupplyFacts(ctx context.Context, q querier, orderID, reservationID int64) (int64, string, string, error) {
	var converted int64
	var raw []byte
	var valid bool
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT s.converted_qty,
 jsonb_build_object(
 'receipts',(SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.customer_stock_entitlement_receipts x WHERE x.entitlement_id=e.id),
 'allocations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.order_stock_batch_allocations x WHERE x.order_id=s.order_id AND x.order_item_id=s.order_item_id),
 'direct_allocations',(SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.customer_direct_ship_request_allocations x WHERE x.order_id=s.order_id AND x.order_item_id=s.order_item_id),
 'batches',(SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM %[1]s.stock_batches x WHERE x.id IN(SELECT stock_batch_id FROM %[1]s.customer_stock_entitlement_receipts WHERE entitlement_id=e.id)),
 'inventory',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.finished_inventory x WHERE x.product_id=e.product_id AND x.bom_spec_id=e.bom_spec_id AND x.warehouse=e.warehouse_code),
 'deductions',(SELECT jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text) FROM %[1]s.order_stock_deductions x WHERE x.order_id=s.order_id)),
 e.received_qty>=s.converted_qty AND s.customer_id=e.customer_id AND s.customer_id=o.customer_id AND i.order_id=o.id AND i.product_id=e.product_id AND COALESCE(i.bom_spec_id,0)=e.bom_spec_id AND COALESCE(i.bom_variant_id,0)=e.bom_variant_id
 AND COALESCE((SELECT sum(x.allocated_qty) FROM %[1]s.customer_direct_ship_request_allocations x JOIN %[1]s.stock_batches b ON b.id=x.batch_id WHERE x.request_id=s.request_id AND x.order_id=s.order_id AND x.order_item_id=s.order_item_id AND x.status='reserved' AND x.product_id=e.product_id AND x.bom_spec_id=e.bom_spec_id AND x.bom_variant_id=e.bom_variant_id AND x.spec_g=e.spec_g AND x.warehouse_code=e.warehouse_code AND b.owner_customer_id=e.customer_id AND b.item_id=e.product_id AND b.bom_spec_id=e.bom_spec_id AND b.bom_variant_id=e.bom_variant_id AND b.spec_g=e.spec_g AND (CASE WHEN e.bom_spec_id>0 THEN b.remaining_units ELSE b.remaining_g/NULLIF(e.spec_g,0) END)>=x.allocated_qty AND b.id IN(SELECT stock_batch_id FROM %[1]s.customer_stock_entitlement_receipts WHERE entitlement_id=e.id)),0)>=s.converted_qty
 AND COALESCE((SELECT sum(CASE WHEN e.bom_spec_id>0 THEN x.allocated_units ELSE x.allocated_g/NULLIF(e.spec_g,0) END) FROM %[1]s.order_stock_batch_allocations x WHERE x.order_id=s.order_id AND x.order_item_id=s.order_item_id AND x.product_id=e.product_id AND x.bom_spec_id=e.bom_spec_id AND x.bom_variant_id=e.bom_variant_id AND x.spec_g=e.spec_g AND x.warehouse=e.warehouse_code AND x.batch_id IN(SELECT stock_batch_id FROM %[1]s.customer_stock_entitlement_receipts WHERE entitlement_id=e.id)),0)>=s.converted_qty
 AND COALESCE((SELECT sum(CASE WHEN e.bom_spec_id>0 THEN x.onhand_units ELSE x.onhand_loose_g/NULLIF(e.spec_g,0) END) FROM %[1]s.finished_inventory x WHERE x.product_id=e.product_id AND x.bom_spec_id=e.bom_spec_id AND x.bom_variant_id=e.bom_variant_id AND x.spec_g=e.spec_g AND x.warehouse=e.warehouse_code AND x.owner_customer_id=e.customer_id),0)>=s.converted_qty
 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_stock_deductions WHERE order_id=s.order_id)
 FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id JOIN %[1]s.orders o ON o.id=s.order_id JOIN %[1]s.order_items i ON i.id=s.order_item_id WHERE s.id=$1 AND e.source_order_id=$2`, r.schema), reservationID, orderID).Scan(&converted, &raw, &valid)
	if err != nil {
		return 0, "", "", err
	}
	if converted == 0 {
		return 0, "", "", nil
	}
	if !valid {
		return converted, hash(raw), "已入库预订与原批次、仓库或客户货权不一致，请先核对实物预留", nil
	}
	return converted, hash(raw), "", nil
}

func (r Repository) verifyCompletedSupply(ctx context.Context, tx pgx.Tx, orderID int64, original domain.Context, choice domain.Disposition) error {
	for _, s := range choice.Supplies {
		var expected domain.SupplyReservation
		for _, d := range original.Dependencies {
			if d.Kind == choice.Kind && d.ID == choice.ID {
				for _, source := range d.SupplyReservations {
					if source.ID == s.ReservationID {
						expected = source
					}
				}
			}
		}
		if expected.ID == 0 {
			return ErrStale
		}
		var pending, converted, entitlement, packageOrder, packageItem int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT s.reserved_qty-s.converted_qty-s.released_qty,s.converted_qty,s.entitlement_id,s.order_id,s.order_item_id FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id WHERE s.id=$1 AND s.request_id=$2 AND e.source_order_id=$3`, r.schema), s.ReservationID, choice.ID, orderID).Scan(&pending, &converted, &entitlement, &packageOrder, &packageItem); err != nil {
			return err
		}
		// The warehouse step already verified and committed physical retention.
		// Later target receipts or shipping are legitimate independent business;
		// final finance approval must verify the old pending obligation is gone,
		// not demand that all physical inventory stays frozen forever.
		if pending != 0 || converted != expected.ConvertedQuantity || entitlement != expected.EntitlementID || packageOrder != expected.OrderID || packageItem != expected.OrderItemID {
			return fmt.Errorf("%w：已完成的货源切换记录发生变化，请重新核对", ErrStale)
		}
	}
	return nil
}
