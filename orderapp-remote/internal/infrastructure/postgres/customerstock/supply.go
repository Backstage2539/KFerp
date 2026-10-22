package customerstock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

type Querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Supply struct {
	ID, OrderID, OrderItemID, CustomerID, ProductID, BomSpecID, BomVariantID, SpecG, PendingQty, ReservedQty, AvailableQty int64
	Warehouse, ProductName, BomSpecName, BomSpecKey, InventoryUnit, SKUCode                                                string
}

func ListSupply(ctx context.Context, q Querier, schema string, customerID int64, warehouse string, lock bool) ([]Supply, error) {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".customer_stock_entitlements").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return []Supply{}, nil
	}
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF e FOR SHARE OF o"
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT e.id,e.source_order_id,e.source_order_item_id,e.customer_id,e.product_id,e.bom_spec_id,e.bom_variant_id,e.spec_g,e.warehouse_code,
 GREATEST(0,e.promised_qty-e.received_qty),COALESCE(r.qty,0),GREATEST(0,e.promised_qty-e.received_qty-COALESCE(r.qty,0)),COALESCE(p.name,''),
 COALESCE(s.name,''),COALESCE(s.spec_key,''),COALESCE(s.inventory_unit,''),COALESCE(NULLIF(s.code,''),p.sku_code,'')
 FROM %[1]s.customer_stock_entitlements e JOIN %[1]s.orders o ON o.id=e.source_order_id JOIN %[1]s.products p ON p.id=e.product_id AND p.active=true
 LEFT JOIN %[1]s.production_bom_specs s ON s.id=e.bom_spec_id
 LEFT JOIN LATERAL(SELECT SUM(reserved_qty-converted_qty-released_qty)::bigint qty FROM %[1]s.customer_stock_supply_reservations WHERE entitlement_id=e.id AND status IN('reserved','partially_converted'))r ON true
 WHERE e.customer_id=$1 AND ($2='' OR e.warehouse_code=$2) AND e.status='active' AND NOT o.is_void AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted'
 AND e.promised_qty>e.received_qty ORDER BY e.created_at,e.id%[2]s`, schema, suffix), customerID, warehouse)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Supply{}
	for rows.Next() {
		var s Supply
		if err = rows.Scan(&s.ID, &s.OrderID, &s.OrderItemID, &s.CustomerID, &s.ProductID, &s.BomSpecID, &s.BomVariantID, &s.SpecG, &s.Warehouse, &s.PendingQty, &s.ReservedQty, &s.AvailableQty, &s.ProductName, &s.BomSpecName, &s.BomSpecKey, &s.InventoryUnit, &s.SKUCode); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func ReserveTx(ctx context.Context, tx pgx.Tx, schema string, supplyID, customerID, requestID, requestItemID, orderID, orderItemID, qty int64) error {
	var available int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.promised_qty-e.received_qty-COALESCE((SELECT SUM(r.reserved_qty-r.converted_qty-r.released_qty) FROM %[1]s.customer_stock_supply_reservations r WHERE r.entitlement_id=e.id AND r.status IN('reserved','partially_converted')),0)
 FROM %[1]s.customer_stock_entitlements e JOIN %[1]s.orders o ON o.id=e.source_order_id WHERE e.id=$1 AND e.customer_id=$2 AND e.status='active' AND NOT o.is_void AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted' FOR UPDATE OF e FOR SHARE OF o`, schema), supplyID, customerID).Scan(&available)
	if err != nil {
		return err
	}
	if qty <= 0 || available < qty {
		return fmt.Errorf("客户已购库存权益不足，请先补购")
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_stock_supply_reservations(entitlement_id,customer_id,request_id,request_item_id,order_id,order_item_id,reserved_qty)VALUES($1,$2,$3,$4,$5,$6,$7)`, schema), supplyID, customerID, requestID, requestItemID, orderID, orderItemID, qty)
	return err
}

// ReceiveProductionTx runs after the physical movement, in that same transaction.
// Cumulative delivered quantities never decrease when a later dropship ships.
func ReceiveProductionTx(ctx context.Context, tx pgx.Tx, schema string, runningID, productID, bomSpecID, bomVariantID, specG, qty int64, receiptKey, batchCode, warehouse, orderNos, actor string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".customer_stock_entitlements").Scan(&exists); err != nil {
		return err
	}
	if !exists || qty <= 0 {
		return nil
	}
	if err := LockFulfillmentCustomersTx(ctx, tx, schema, orderNos); err != nil {
		return err
	}
	var prior int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(quantity),0) FROM %s.customer_stock_entitlement_receipts WHERE receipt_key=$1`, schema), receiptKey).Scan(&prior); err != nil {
		return err
	}
	if prior > 0 {
		return nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT e.id,e.customer_id,e.source_order_id,e.promised_qty-e.received_qty,e.bom_variant_id FROM %[1]s.customer_stock_entitlements e JOIN %[1]s.orders o ON o.id=e.source_order_id WHERE e.status='active' AND NOT o.is_void AND e.product_id=$1 AND e.bom_spec_id=$2 AND e.spec_g=$3 AND e.warehouse_code=$4 AND o.order_no=ANY(string_to_array(replace($5,' ',''),',')) AND e.promised_qty>e.received_qty ORDER BY e.created_at,e.id FOR UPDATE OF e`, schema), productID, bomSpecID, specG, warehouse, orderNos)
	if err != nil {
		return err
	}
	type source struct{ id, customer, order, pending, variant int64 }
	sources := []source{}
	for rows.Next() {
		var s source
		if err = rows.Scan(&s.id, &s.customer, &s.order, &s.pending, &s.variant); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, s)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	if len(sources) == 0 {
		return nil
	}
	for _, s := range sources {
		if bomSpecID > 0 && s.variant != bomVariantID {
			return fmt.Errorf("留厂生产规格版本与原销售冻结版本不符，不能交付入库")
		}
	}
	var batchID, owner int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,owner_customer_id FROM %s.stock_batches WHERE batch_code=$1 AND item_type='finished_product' AND item_id=$2 AND bom_spec_id=$3 AND spec_g=$4 AND ($3=0 OR bom_variant_id=$5) ORDER BY id DESC LIMIT 1`, schema), batchCode, productID, bomSpecID, specG, bomVariantID).Scan(&batchID, &owner); err != nil {
		return err
	}
	remaining := qty
	for _, s := range sources {
		if remaining <= 0 {
			break
		}
		if s.customer != owner {
			return fmt.Errorf("留厂生产入库货权不符")
		}
		take := s.pending
		if take > remaining {
			take = remaining
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_stock_entitlement_receipts(entitlement_id,receipt_key,running_item_id,stock_batch_id,batch_code,warehouse_code,quantity)VALUES($1,$2,$3,$4,$5,$6,$7)`, schema), s.id, receiptKey, runningID, batchID, batchCode, warehouse, take); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET received_qty=received_qty+$2,updated_at=now() WHERE id=$1`, schema), s.id, take); err != nil {
			return err
		}
		if err = convertReservationsTx(ctx, tx, schema, s.id, s.customer, productID, bomSpecID, bomVariantID, specG, take, batchID, batchCode, warehouse, actor); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders o SET delivery_status=CASE WHEN NOT EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlements e WHERE e.source_order_id=o.id AND e.status='active' AND e.received_qty<e.promised_qty) THEN 'received' ELSE 'partially_received' END WHERE id=$1`, schema), s.order); err != nil {
			return err
		}
		if err = postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer_stock_entitlement", &s.id, "production_receive", nil, nil, nil, postgresinfra.AuditMeta{"source_order_id": s.order, "quantity": take, "receipt_key": receiptKey, "batch_code": batchCode}); err != nil {
			return err
		}
		remaining -= take
	}
	return nil
}

func convertReservationsTx(ctx context.Context, tx pgx.Tx, schema string, entitlementID, customerID, productID, bomSpecID, bomVariantID, specG, qty, batchID int64, batchCode, warehouse, actor string) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,request_id,request_item_id,order_id,order_item_id,reserved_qty-converted_qty-released_qty FROM %s.customer_stock_supply_reservations WHERE entitlement_id=$1 AND status IN('reserved','partially_converted') ORDER BY created_at,id FOR UPDATE`, schema), entitlementID)
	if err != nil {
		return err
	}
	type reservation struct{ id, request, item, order, orderItem, pending int64 }
	all := []reservation{}
	for rows.Next() {
		var r reservation
		if err = rows.Scan(&r.id, &r.request, &r.item, &r.order, &r.orderItem, &r.pending); err != nil {
			rows.Close()
			return err
		}
		all = append(all, r)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, r := range all {
		if qty <= 0 {
			break
		}
		take := r.pending
		if take > qty {
			take = qty
		}
		if take <= 0 {
			continue
		}
		g, u := take*specG, int64(0)
		if bomSpecID > 0 {
			g, u = 0, take
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,warehouse,request_id,operator)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$7,$8,$11,$12,$13)`, schema), r.order, r.orderItem, productID, bomSpecID, bomVariantID, specG, g, u, batchID, batchCode, warehouse, r.request, actor); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customer_direct_ship_request_allocations(request_id,request_item_id,request_order_id,order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,warehouse_code,batch_id,batch_code,allocated_qty,allocated_units,allocated_g,status)SELECT $1,$2,ro.id,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'reserved' FROM %[1]s.customer_direct_ship_request_orders ro WHERE ro.request_id=$1 AND ro.order_id=$3`, schema), r.request, r.item, r.order, r.orderItem, productID, bomSpecID, bomVariantID, specG, warehouse, batchID, batchCode, take, u, g); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_supply_reservations SET converted_qty=converted_qty+$2,status=CASE WHEN converted_qty+$2+released_qty>=reserved_qty THEN 'converted' ELSE 'partially_converted' END,updated_at=now() WHERE id=$1`, schema), r.id, take); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders o SET process_status_id=(SELECT id FROM %[1]s.order_process_statuses WHERE name='库存待发货' ORDER BY id LIMIT 1) WHERE o.id=$1 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_items i WHERE i.order_id=o.id AND COALESCE((SELECT SUM(a.allocated_qty)FROM %[1]s.customer_direct_ship_request_allocations a WHERE a.order_item_id=i.id AND a.status='reserved'),0)<i.qty)`, schema), r.order); err != nil {
			return err
		}
		if err = postgresinfra.AuditInsertTx(ctx, tx, schema, actor, "customer_stock_supply_reservation", &r.id, "production_output_convert", nil, nil, nil, postgresinfra.AuditMeta{"customer_id": customerID, "order_id": r.order, "quantity": take, "stock_batch_id": batchID}); err != nil {
			return err
		}
		qty -= take
	}
	return nil
}
