package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
)

func (r Repository) replacementSources(ctx context.Context, q querier, c *domain.Context) error {
	hasSource := false
	for _, d := range c.Dependencies {
		if d.Kind == "direct_ship" && d.Relation == "supply" && !one(d.Status, "cancelled", "released") {
			hasSource = true
			break
		}
	}
	if !hasSource {
		return nil
	}
	supplies, err := customerstock.ListSupply(ctx, q, r.schema, c.CustomerID, "", false)
	if err != nil {
		return err
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT s.id,s.request_id,s.entitlement_id,s.order_id,s.order_item_id,s.reserved_qty-s.converted_qty-s.released_qty,s.converted_qty,e.product_id,e.bom_spec_id,e.bom_variant_id,e.spec_g,e.warehouse_code,COALESCE(i.item_name,''),COALESCE(i.unit,''),o.order_no,COALESCE((SELECT string_agg(DISTINCT x.batch_code,'、' ORDER BY x.batch_code) FROM %[1]s.customer_stock_entitlement_receipts x WHERE x.entitlement_id=e.id AND EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=s.order_id AND a.order_item_id=s.order_item_id AND a.batch_id=x.stock_batch_id AND (a.allocated_units>0 OR a.allocated_g>0))),'')
 FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_stock_entitlements e ON e.id=s.entitlement_id JOIN %[1]s.order_items i ON i.id=s.order_item_id JOIN %[1]s.orders o ON o.id=s.order_id
 WHERE e.source_order_id=$1 AND s.status IN('reserved','partially_converted','converted','released') AND (s.reserved_qty>s.converted_qty+s.released_qty OR s.converted_qty>0)
 AND s.customer_id=$2 AND e.customer_id=$2 AND o.customer_id=$2 AND i.order_id=o.id AND i.product_id=e.product_id AND COALESCE(i.bom_spec_id,0)=e.bom_spec_id AND COALESCE(i.bom_variant_id,0)=e.bom_variant_id
 ORDER BY s.id`, r.schema), c.OrderID, c.CustomerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var source domain.SupplyReservation
		var request, product, spec, variant, grams int64
		var warehouse, name, number string
		if err = rows.Scan(&source.ID, &request, &source.EntitlementID, &source.OrderID, &source.OrderItemID, &source.Quantity, &source.ConvertedQuantity, &product, &spec, &variant, &grams, &warehouse, &name, &source.Unit, &number, &source.PhysicalBatches); err != nil {
			return err
		}
		source.Label = fmt.Sprintf("%s · %s · %s · 预订 #%d", number, name, warehouse, source.ID)
		source.Targets = []domain.SupplyTarget{}
		for _, supply := range supplies {
			if source.Quantity == 0 {
				break
			}
			if supply.OrderID == c.OrderID || supply.ProductID != product || supply.BomSpecID != spec || supply.BomVariantID != variant || supply.SpecG != grams || supply.Warehouse != warehouse || supply.AvailableQty <= 0 {
				continue
			}
			source.Targets = append(source.Targets, domain.SupplyTarget{ID: supply.ID, Label: fmt.Sprintf("%s · %s %s · %s · 权益 #%d", supply.OrderNo, supply.ProductName, supply.BomSpecName, supply.Warehouse, supply.ID), AvailableQty: supply.AvailableQty})
		}
		for n := range c.Dependencies {
			d := &c.Dependencies[n]
			if d.Kind == "direct_ship" && d.Relation == "supply" && d.ID == request && !one(d.Status, "cancelled", "released") {
				d.SupplyReservations = append(d.SupplyReservations, source)
			}
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for n := range c.Dependencies {
		d := &c.Dependencies[n]
		for j := range d.SupplyReservations {
			source := &d.SupplyReservations[j]
			if source.ConvertedQuantity == 0 {
				continue
			}
			_, source.PhysicalVersion, source.PhysicalIssue, err = r.convertedSupplyFacts(ctx, q, c.OrderID, source.ID)
			if err != nil {
				return err
			}
			if source.PhysicalIssue != "" {
				d.ReplacementIssue = source.PhysicalIssue
			}
		}
	}
	raw, err := json.Marshal(c.Dependencies)
	if err != nil {
		return err
	}
	c.Version = hash([]byte(c.Version + string(raw)))
	return nil
}

func (r Repository) replaceSupply(ctx context.Context, tx pgx.Tx, orderID, resolutionID, requestID int64, choice domain.Disposition, note, actor string) error {
	// Lock every package before inspecting shipping facts. Source and target
	// reservations are checked again in this transaction, never from the UI.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`SELECT o.id FROM %[1]s.orders o JOIN %[1]s.customer_direct_ship_request_orders p ON p.order_id=o.id WHERE p.request_id=$1 ORDER BY o.id FOR UPDATE OF o,p`, r.schema), requestID); err != nil {
		return err
	}
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return err
	}
	var dependency domain.Dependency
	for _, d := range current.Dependencies {
		if d.Kind == "direct_ship" && d.ID == requestID && d.Relation == "supply" {
			dependency = d
		}
	}
	if err = domain.ValidateSupplyReplacement(dependency, choice); err != nil {
		return err
	}
	changes := []map[string]any{}
	for _, selected := range choice.Supplies {
		var old []byte
		var customer, requestItem, packageOrder, packageItem int64
		var packageStatus, requestStatus string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(s),s.customer_id,s.request_item_id,s.order_id,s.order_item_id,p.status,d.status FROM %[1]s.customer_stock_supply_reservations s JOIN %[1]s.customer_direct_ship_request_orders p ON p.order_id=s.order_id AND p.request_id=s.request_id JOIN %[1]s.customer_direct_ship_requests d ON d.id=s.request_id WHERE s.id=$1 AND s.request_id=$2 FOR UPDATE OF s,p,d`, r.schema), selected.ReservationID, requestID).Scan(&old, &customer, &requestItem, &packageOrder, &packageItem, &packageStatus, &requestStatus); err != nil {
			return err
		}
		if one(packageStatus, "cancelled", "shipped", "delivered", "completed") || one(requestStatus, "cancelled", "shipped", "delivered", "completed") {
			return fmt.Errorf("代发包裹状态已变化，不能更换货源")
		}
		var unavailable bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.is_void OR o.fulfillment_paused OR o.resolution_status IN('closed','cancelled') OR COALESCE(o.ship_tracking_no,'')<>'' OR COALESCE(s.name,'') IN('已发货','已出库','已签收','已收货','已完成') OR EXISTS(SELECT 1 FROM %[1]s.order_stock_deductions WHERE order_id=o.id) OR EXISTS(SELECT 1 FROM %[1]s.order_shipment_orders WHERE order_id=o.id AND shipped_at IS NOT NULL) OR EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests WHERE order_id=o.id AND status='pending') FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses s ON s.id=o.ship_status_id WHERE o.id=$1`, r.schema), packageOrder).Scan(&unavailable); err != nil {
			return err
		}
		if unavailable {
			return fmt.Errorf("代发包裹已暂停、处理中或已有发货，请先核对包裹")
		}
		if selected.Quantity == 0 {
			continue
		} // Explicitly retained physical reservation; no mutation.
		targetBefore, targetAfter, err := r.reserveReplacementTarget(ctx, tx, selected.TargetID, customer, requestID, requestItem, packageOrder, packageItem, selected.Quantity)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET released_qty=released_qty+$2,status='released',updated_at=now() WHERE id=$1 AND status IN('reserved','partially_converted') AND reserved_qty-converted_qty-released_qty=$2`, r.schema), selected.ReservationID, selected.Quantity)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrStale
		}
		changes = append(changes, map[string]any{"source_before": json.RawMessage(old), "target_before": targetBefore, "target_after": targetAfter, "target_entitlement_id": selected.TargetID, "quantity": selected.Quantity, "package_order_id": packageOrder})
	}
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, "customer_direct_ship_request", &requestID, "resolution_replace_supply", nil, nil, nil, infra.AuditMeta{"source_order_id": orderID, "resolution_id": resolutionID, "note": note, "changes": changes, "physical_stock_unchanged": true})
}

func (r Repository) reserveReplacementTarget(ctx context.Context, tx pgx.Tx, targetID, customerID, requestID, requestItemID, orderID, orderItemID, quantity int64) (json.RawMessage, json.RawMessage, error) {
	// Reuse the same capacity and source-order locks as normal reservations.
	supplies, err := customerstock.ListSupply(ctx, tx, r.schema, customerID, "", true)
	if err != nil {
		return nil, nil, err
	}
	available := int64(0)
	for _, supply := range supplies {
		if supply.ID == targetID {
			available = supply.AvailableQty
		}
	}
	if quantity <= 0 || available < quantity {
		return nil, nil, fmt.Errorf("替代货源可用数量不足，请重新预览")
	}
	var before, after []byte
	var same bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(s),customer_id=$3 AND request_id=$4 AND request_item_id=$5 AND order_id=$6 FROM %s.customer_stock_supply_reservations s WHERE entitlement_id=$1 AND order_item_id=$2 FOR UPDATE`, r.schema), targetID, orderItemID, customerID, requestID, requestItemID, orderID).Scan(&before, &same)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		before = []byte(`null`)
		err = customerstock.ReserveTx(ctx, tx, r.schema, targetID, customerID, requestID, requestItemID, orderID, orderItemID, quantity)
	case err != nil:
		return nil, nil, err
	case !same:
		return nil, nil, fmt.Errorf("替代货源已有预订的包裹身份不一致，请核对")
	default:
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET reserved_qty=reserved_qty+$3,status=CASE WHEN converted_qty>0 THEN 'partially_converted' ELSE 'reserved' END,updated_at=now() WHERE entitlement_id=$1 AND order_item_id=$2`, r.schema), targetID, orderItemID, quantity)
	}
	if err != nil {
		return nil, nil, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(s) FROM %s.customer_stock_supply_reservations s WHERE entitlement_id=$1 AND order_item_id=$2`, r.schema), targetID, orderItemID).Scan(&after)
	return json.RawMessage(before), json.RawMessage(after), err
}
