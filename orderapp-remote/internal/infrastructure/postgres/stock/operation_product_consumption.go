package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/stock"
)

func (r Repository) consumeOperationProductTx(ctx context.Context, tx pgx.Tx, detail app.StockDocumentDetail, item *app.StockDocumentItemRow, actor string) error {
	var reservationID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND component_type='product' AND component_id=$2 AND component_bom_spec_id=$3 AND component_spec_g=$4 AND source_owner_customer_id=$5 AND component_bom_variant_id=$6 ORDER BY id LIMIT 1 FOR UPDATE`, r.schema), detail.WorkOrderID, item.ProductID, item.BomSpecID, item.SpecG, item.OwnerCustomerID, item.BomVariantID).Scan(&reservationID); err != nil {
		return fmt.Errorf("商品组件缺少冻结来源身份：%w", err)
	}
	beforeN, beforeL, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, item.ProductID, item.BomSpecID, item.SpecG, item.FromWarehouse, item.OwnerCustomerID)
	if err != nil {
		return err
	}
	needG, needN := pickingTransferQuantities(*item)
	weightOnly := item.BomSpecID == 0 && item.QtyUnits == 0 && item.QtyG > 0
	if weightOnly {
		needN = 0
	}
	if item.BomSpecID > 0 {
		needG = 0
	}
	beforeG := beforeN*item.SpecG + beforeL
	if (item.BomSpecID > 0 && beforeN < needN) || (item.BomSpecID == 0 && beforeG < needG) {
		return fmt.Errorf("商品组件在指定来源仓的库存不足")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,b.remaining_g,b.remaining_units,b.unit_cost::float8 FROM %[1]s.stock_batches b
 LEFT JOIN LATERAL(SELECT warehouse FROM %[1]s.stock_ledger_entries WHERE item_type=b.item_type AND item_id=b.item_id AND source_batch_code=b.batch_code AND owner_customer_id=b.owner_customer_id ORDER BY id DESC LIMIT 1) l ON true
 WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3 AND b.owner_customer_id=$4 AND (b.remaining_g>0 OR b.remaining_units>0) AND b.quality_status NOT IN('hold','reject') AND COALESCE(l.warehouse,'finished_goods')=$5 ORDER BY b.created_at,b.id FOR UPDATE OF b`, r.schema), item.ProductID, item.BomSpecID, item.SpecG, item.OwnerCustomerID, item.FromWarehouse)
	if err != nil {
		return err
	}
	type candidate struct {
		id, g, n, ownG, ownN int64
		code                 string
		cost                 float64
	}
	all := []candidate{}
	for rows.Next() {
		var b candidate
		if err = rows.Scan(&b.id, &b.code, &b.g, &b.n, &b.cost); err != nil {
			rows.Close()
			return err
		}
		all = append(all, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	reserved, free := []candidate{}, []candidate{}
	for _, b := range all {
		var otherG, otherN int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)) FILTER(WHERE rb.work_order_id<>$2),0)::bigint,COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)) FILTER(WHERE rb.work_order_id<>$2),0)::bigint,COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)) FILTER(WHERE rb.work_order_id=$2),0)::bigint,COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)) FILTER(WHERE rb.work_order_id=$2),0)::bigint FROM %[1]s.work_order_material_reservation_batches rb JOIN %[1]s.work_order_material_reservations r ON r.id=rb.reservation_id WHERE rb.stock_batch_id=$1 AND rb.warehouse=$3 AND rb.owner_customer_id=$4 AND rb.status='reserved' AND r.status='reserved'`, r.schema), b.id, detail.WorkOrderID, item.FromWarehouse, item.OwnerCustomerID).Scan(&otherG, &otherN, &b.ownG, &b.ownN)
		if err != nil {
			return err
		}
		cpG, cpN, e := operationOtherProcessingReservedTx(ctx, tx, r.schema, detail.WorkOrderID, b.id, item.ProductID, item.OwnerCustomerID, item.FromWarehouse, true)
		if e != nil {
			return e
		}
		otherG += cpG
		otherN += cpN
		var retained bool
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_stock_entitlement_receipts r JOIN %s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE r.stock_batch_id=$1 AND e.status='active')`, r.schema, r.schema), b.id).Scan(&retained); e != nil {
			return e
		}
		if retained {
			continue
		}
		b.g = nonnegativeStockQty(b.g - otherG)
		b.n = nonnegativeStockQty(b.n - otherN)
		own := b
		own.g = minInt64(b.g, b.ownG)
		own.n = minInt64(b.n, b.ownN)
		reserved = append(reserved, own)
		b.g -= own.g
		b.n -= own.n
		free = append(free, b)
	}
	remainingG, remainingN := needG, needN
	totalCost := 0.0
	for _, b := range append(reserved, free...) {
		g, n := minInt64(remainingG, b.g), minInt64(remainingN, b.n)
		if item.BomSpecID == 0 && item.SpecG > 0 && !weightOnly {
			n = g / item.SpecG
		}
		if g == 0 && n == 0 {
			continue
		}
		if err = consumeOperationProcessingReservationTx(ctx, tx, r.schema, detail.WorkOrderID, b.id, item.ProductID, item.OwnerCustomerID, item.FromWarehouse, g, n, true); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g-$2,remaining_units=CASE WHEN $4::boolean AND spec_g>0 THEN (remaining_g-$2)/spec_g ELSE remaining_units-$3 END WHERE id=$1`, r.schema), b.id, g, n, weightOnly); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_entry_batch_allocations(stock_entry_item_id,material_batch_id,batch_code,qty_g,qty_units,unit_cost) VALUES($1,0,$2,$3,$4,$5)`, r.schema), item.ID, b.code, g, n, b.cost); err != nil {
			return err
		}
		var bindingID, ownG, ownN int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,GREATEST(0,reserved_g-consumed_g-returned_g),GREATEST(0,reserved_units-consumed_units-returned_units) FROM %s.work_order_material_reservation_batches WHERE reservation_id=$1 AND stock_batch_id=$2 AND warehouse=$3 ORDER BY id LIMIT 1 FOR UPDATE`, r.schema), reservationID, b.id, item.FromWarehouse).Scan(&bindingID, &ownG, &ownN)
		if err != nil && err != pgx.ErrNoRows {
			return err
		}
		extraG, extraN := nonnegativeStockQty(g-ownG), nonnegativeStockQty(n-ownN)
		if bindingID > 0 {
			_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=reserved_g+$2,reserved_units=reserved_units+$3,consumed_g=consumed_g+$4,consumed_units=consumed_units+$5,updated_at=now() WHERE id=$1`, r.schema), bindingID, extraG, extraN, g, n)
		} else {
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(material_id,reservation_id,work_order_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,stock_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,reserved_units,consumed_g,consumed_units,status) VALUES(0,$1,$2,'product',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$11,$12,'consumed')`, r.schema), reservationID, detail.WorkOrderID, item.ProductID, item.BomSpecID, item.BomVariantID, item.SpecG, b.id, b.code, item.FromWarehouse, item.OwnerCustomerID, g, n)
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET reserved_g=reserved_g+$2,reserved_units=reserved_units+$3,consumed_g=consumed_g+$4,consumed_units=consumed_units+$5,updated_at=now() WHERE id=$1`, r.schema), reservationID, extraG, extraN, g, n); err != nil {
			return err
		}
		totalCost += stockItemTotalCost(g, n, b.cost)
		remainingG -= g
		remainingN -= n
		afterLedgerN := beforeN - n
		if weightOnly && item.SpecG > 0 {
			afterLedgerN = (beforeG - g) / item.SpecG
		}
		if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: item.ItemName, OwnerCustomerID: item.OwnerCustomerID, SpecG: item.SpecG, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, Warehouse: item.FromWarehouse, SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchCode: b.code, SourceBatchID: detail.EntryNo, BeforeG: beforeG, ChangeG: -g, AfterG: beforeG - g, BeforeUnits: beforeN, ChangeUnits: afterLedgerN - beforeN, AfterUnits: afterLedgerN, Operator: actor}); err != nil {
			return err
		}
		beforeG -= g
		beforeN = afterLedgerN
	}
	if remainingG > 0 || remainingN > 0 {
		return fmt.Errorf("商品组件合格库存不足，其他工单预留不会被占用")
	}
	afterN, afterL := beforeN, beforeL
	if item.BomSpecID == 0 {
		afterN, afterL, _, err = normalizeFinishedQty(item.SpecG, 0, beforeG)
		if err != nil {
			return err
		}
	}
	if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, item.ProductID, item.BomSpecID, item.BomVariantID, item.SpecG, item.FromWarehouse, afterN, afterL, item.OwnerCustomerID); err != nil {
		return err
	}
	unitCost := 0.0
	if needN > 0 {
		unitCost = totalCost / float64(needN)
	}
	if needG > 0 {
		unitCost = totalCost * 1000 / float64(needG)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET unit_cost=$2,total_cost=$3 WHERE id=$1`, r.schema), item.ID, unitCost, totalCost)
	return err
}
