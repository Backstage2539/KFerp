package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/stock"
)

func automaticPickingWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	exists, err := stockSchemaColumnExistsTx(ctx, tx, schema, "production_plans", "picking_version")
	if err != nil || !exists {
		return false, err
	}
	var enabled bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_plans p ON p.id=w.production_plan_id WHERE w.id=$1 AND p.picking_version>0)`, schema), id).Scan(&enabled)
	return enabled, err
}

type pickingBinding struct {
	ID, ReservationID, MaterialID, ComponentID, SpecID, VariantID, SpecG, MaterialBatchID, StockBatchID, Owner, G, N int64
	ComponentType, BatchCode, Warehouse                                                                              string
}

func pickingBindingsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, item app.StockDocumentItemRow, warehouse string) ([]pickingBinding, error) {
	componentType, id := "material", item.MaterialID
	if item.ItemType == itemTypeFinishedProduct {
		componentType, id = "product", item.ProductID
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.reservation_id,b.material_id,b.component_type,b.component_id,b.component_bom_spec_id,b.component_bom_variant_id,b.component_spec_g,b.material_batch_id,b.stock_batch_id,b.owner_customer_id,b.batch_code,b.warehouse,GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g),GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)
 FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id
 WHERE b.work_order_id=$1 AND b.component_type=$2 AND b.component_id=$3 AND b.component_bom_spec_id=$4 AND b.component_spec_g=$5 AND b.batch_code=$6 AND b.warehouse=$7 AND b.owner_customer_id=$8 AND b.component_bom_variant_id=$9 AND b.status='reserved' AND r.status='reserved' ORDER BY b.id FOR UPDATE OF b,r`, schema), workOrderID, componentType, id, item.BomSpecID, item.SpecG, item.BatchCode, warehouse, item.OwnerCustomerID, item.BomVariantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pickingBinding{}
	for rows.Next() {
		var b pickingBinding
		if err = rows.Scan(&b.ID, &b.ReservationID, &b.MaterialID, &b.ComponentType, &b.ComponentID, &b.SpecID, &b.VariantID, &b.SpecG, &b.MaterialBatchID, &b.StockBatchID, &b.Owner, &b.BatchCode, &b.Warehouse, &b.G, &b.N); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func validateAutomaticPickingTx(ctx context.Context, tx pgx.Tx, schema string, detail app.StockDocumentDetail) error {
	used := map[string][2]int64{}
	for _, item := range detail.Items {
		if item.BatchCode == "" || item.FromWarehouse == "" {
			return fmt.Errorf("领料须保留冻结的来源仓、货主和批次")
		}
		if (!detail.IsReturn && item.ToWarehouse != "wip") || (detail.IsReturn && item.FromWarehouse != "wip") {
			return fmt.Errorf("生产领料必须转入 WIP，退料必须从 WIP 转出")
		}
		if detail.IsReturn {
			var permitted bool
			err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_plan_component_sources s ON s.production_plan_item_id=w.production_plan_item_id CROSS JOIN LATERAL jsonb_array_elements(s.allocations_json) a WHERE w.id=$1 AND s.component_id=$2 AND a->>'warehouse'=$3 AND COALESCE((a->>'owner_customer_id')::bigint,0)=$4)`, schema), detail.WorkOrderID, func() int64 {
				if item.ItemType == itemTypeFinishedProduct {
					return item.ProductID
				}
				return item.MaterialID
			}(), item.ToWarehouse, item.OwnerCustomerID).Scan(&permitted)
			if err != nil {
				return err
			}
			if !permitted {
				return fmt.Errorf("退料目标须为本工单冻结的原来源仓")
			}
		}
		bindings, err := pickingBindingsTx(ctx, tx, schema, detail.WorkOrderID, item, item.FromWarehouse)
		if err != nil {
			return err
		}
		var g, n int64
		for _, b := range bindings {
			g += b.G
			n += b.N
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s:%s:%d", item.ItemType, item.MaterialID, item.ProductID, item.BomSpecID, item.SpecG, item.BatchCode, item.FromWarehouse, item.OwnerCustomerID)
		q := used[key]
		ig, in := pickingTransferQuantities(item)
		q[0] += ig
		q[1] += in
		used[key] = q
		if len(bindings) == 0 || q[0] > g || q[1] > n {
			return fmt.Errorf("%s / %s 的待领预留已变化，请刷新领料建议", item.ItemName, item.BatchCode)
		}
	}
	return nil
}

// Move only the posted quantity. Consumed/returned history stays on its original
// binding, while unconsumed reservations follow the physical stock into WIP.
func movePickingReservationTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, item app.StockDocumentItemRow, direction int64) error {
	from, to := item.FromWarehouse, item.ToWarehouse
	if direction < 0 {
		from, to = to, from
	}
	targetBatchID := int64(0)
	targetBatchCode := ""
	if item.ItemType == itemTypeFinishedProduct {
		var sourceID, movedID int64
		var sourceCode, movedCode string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_batch_id,source_batch_code,target_batch_id,target_batch_code FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_item_id=$1`, schema), item.ID).Scan(&sourceID, &sourceCode, &movedID, &movedCode); err != nil {
			return err
		}
		if direction > 0 {
			item.BatchCode = sourceCode
			targetBatchID = movedID
			targetBatchCode = movedCode
		} else {
			item.BatchCode = movedCode
			targetBatchID = sourceID
			targetBatchCode = sourceCode
		}
	}
	bindings, err := pickingBindingsTx(ctx, tx, schema, workOrderID, item, from)
	if err != nil {
		return err
	}
	g, n := pickingTransferQuantities(item)
	for _, b := range bindings {
		takeG, takeN := minInt64(g, b.G), minInt64(n, b.N)
		if takeG == 0 && takeN == 0 {
			continue
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=reserved_g-$2,reserved_units=reserved_units-$3,updated_at=now() WHERE id=$1`, schema), b.ID, takeG, takeN); err != nil {
			return err
		}
		if targetBatchID > 0 {
			b.StockBatchID = targetBatchID
			b.BatchCode = targetBatchCode
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,material_batch_id,stock_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,reserved_units,status)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,'reserved')
 ON CONFLICT(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse) DO UPDATE SET reserved_g=work_order_material_reservation_batches.reserved_g+excluded.reserved_g,reserved_units=work_order_material_reservation_batches.reserved_units+excluded.reserved_units,status='reserved',updated_at=now()`, schema), b.ReservationID, workOrderID, b.MaterialID, b.ComponentType, b.ComponentID, b.SpecID, b.VariantID, b.SpecG, b.MaterialBatchID, b.StockBatchID, b.BatchCode, to, b.Owner, takeG, takeN); err != nil {
			return err
		}
		g -= takeG
		n -= takeN
	}
	if g > 0 || n > 0 {
		return fmt.Errorf("本次领料对应的预留不足或已耗用，不能重复领料或撤销")
	}
	return nil
}

// Finished-product components already use split stock batches for transfers.
// Preserve that model, including the source/target batch link and frozen cost.
func (r Repository) postPickingProductTransferTx(ctx context.Context, tx pgx.Tx, detail app.StockDocumentDetail, item app.StockDocumentItemRow, actor string, direction int64) error {
	totalG, units := item.QtyG+item.QtyUnits*item.SpecG, item.QtyUnits
	if item.BomSpecID > 0 {
		totalG = 0
	} else if item.SpecG > 0 {
		units = totalG / item.SpecG
	}
	var sourceID, targetID int64
	var sourceCode, targetCode, quality string
	var cost float64
	if direction > 0 {
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,b.unit_cost,b.quality_status FROM %[1]s.stock_batches b
  LEFT JOIN LATERAL(SELECT warehouse FROM %[1]s.stock_ledger_entries WHERE item_type='finished_product' AND item_id=b.item_id AND source_batch_code=b.batch_code ORDER BY id DESC LIMIT 1) l ON true
  WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3 AND b.batch_code=$4 AND b.owner_customer_id=$5 AND b.remaining_g>=$6 AND b.remaining_units>=$7 AND b.quality_status NOT IN('hold','reject') AND COALESCE(l.warehouse,'finished_goods')=$8 FOR UPDATE OF b`, r.schema), item.ProductID, item.BomSpecID, item.SpecG, item.BatchCode, item.OwnerCustomerID, totalG, units, item.FromWarehouse).Scan(&sourceID, &sourceCode, &cost, &quality)
		if err != nil {
			return fmt.Errorf("商品组件批次已变化或数量不足，请刷新领料建议：%w", err)
		}
		var moveID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_entry_finished_batch_moves(stock_entry_id,stock_entry_item_id,source_batch_id,source_batch_code,bom_spec_id,bom_variant_id,spec_g,qty_g,qty_units) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`, r.schema), detail.ID, item.ID, sourceID, sourceCode, item.BomSpecID, item.BomVariantID, item.SpecG, totalG, units).Scan(&moveID); err != nil {
			return err
		}
		targetCode = fmt.Sprintf("FP-MOVE-%010d", moveID)
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,owner_customer_id,bom_spec_id,bom_variant_id,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,unit_cost,quality_status,operator) VALUES($1,'finished_product',$2,$3,$4,$5,$6,$7,'stock_entry_transfer',$8,$9,$10,$11,$10,$11,$12,$13,$14) RETURNING id`, r.schema), targetCode, item.ProductID, item.ItemName, item.OwnerCustomerID, item.BomSpecID, item.BomVariantID, item.SpecG, moveID, sourceCode, totalG, units, cost, quality, actor).Scan(&targetID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_finished_batch_moves SET target_batch_id=$2,target_batch_code=$3 WHERE id=$1`, r.schema), moveID, targetID, targetCode); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g-$2,remaining_units=remaining_units-$3 WHERE id=$1`, r.schema), sourceID, totalG, units); err != nil {
			return err
		}
	} else {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT m.source_batch_id,m.source_batch_code,m.target_batch_id,m.target_batch_code,b.unit_cost FROM %[1]s.stock_entry_finished_batch_moves m JOIN %[1]s.stock_batches b ON b.id=m.target_batch_id WHERE m.stock_entry_item_id=$1 AND b.remaining_g>=m.qty_g AND b.remaining_units>=m.qty_units FOR UPDATE OF b`, r.schema), item.ID).Scan(&sourceID, &sourceCode, &targetID, &targetCode, &cost); err != nil {
			return fmt.Errorf("已领商品批次已耗用，不能撤销：%w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g-$2,remaining_units=remaining_units-$3 WHERE id=$1`, r.schema), targetID, totalG, units); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_g=remaining_g+$2,remaining_units=remaining_units+$3 WHERE id=$1`, r.schema), sourceID, totalG, units); err != nil {
			return err
		}
	}
	for _, location := range []struct {
		warehouse, code string
		delta           int64
	}{{item.FromWarehouse, sourceCode, -direction}, {item.ToWarehouse, targetCode, direction}} {
		beforeU, beforeL, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, item.ProductID, item.BomSpecID, item.SpecG, location.warehouse, item.OwnerCustomerID)
		if err != nil {
			return err
		}
		afterU, afterL := beforeU+location.delta*units, int64(0)
		beforeG := int64(0)
		if item.BomSpecID == 0 {
			beforeG = beforeU*item.SpecG + beforeL
			afterU, afterL, _, err = normalizeFinishedQty(item.SpecG, 0, beforeG+location.delta*totalG)
			if err != nil {
				return err
			}
		}
		if afterU < 0 || afterL < 0 {
			return fmt.Errorf("商品组件来源库存不足")
		}
		if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, item.ProductID, item.BomSpecID, item.BomVariantID, item.SpecG, location.warehouse, afterU, afterL, item.OwnerCustomerID); err != nil {
			return err
		}
		docType := "stock_entry"
		if direction < 0 {
			docType = "stock_entry_cancel"
		}
		if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: item.ItemName, OwnerCustomerID: item.OwnerCustomerID, BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, SpecG: item.SpecG, Warehouse: location.warehouse, SourceDocType: docType, SourceDocID: detail.ID, SourceBatchCode: location.code, SourceBatchID: detail.EntryNo, BeforeG: beforeG, ChangeG: location.delta * totalG, AfterG: beforeG + location.delta*totalG, BeforeUnits: beforeU, ChangeUnits: afterU - beforeU, AfterUnits: afterU, Operator: actor}); err != nil {
			return err
		}
	}
	if direction > 0 {
		_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET unit_cost=$2,total_cost=$3 WHERE id=$1`, r.schema), item.ID, cost, stockItemTotalCost(totalG, units, cost))
		return err
	}
	return nil
}

func pickingTransferQuantities(item app.StockDocumentItemRow) (int64, int64) {
	g, n := item.QtyG, item.QtyUnits
	if item.ItemType == itemTypeFinishedProduct && item.SpecG > 0 {
		g += n * item.SpecG
		n = g / item.SpecG
	}
	return g, n
}

func recordPickingConsumptionTx(ctx context.Context, tx pgx.Tx, schema string, detail app.StockDocumentDetail, direction int64) error {
	for _, item := range detail.Items {
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT a.material_batch_id,a.qty_g,a.qty_units FROM %s.stock_entry_batch_allocations a WHERE a.stock_entry_item_id=$1 ORDER BY a.id`, schema), item.ID)
		if err != nil {
			return err
		}
		type allocation struct{ id, g, n int64 }
		allocations := []allocation{}
		for rows.Next() {
			var a allocation
			if err = rows.Scan(&a.id, &a.g, &a.n); err != nil {
				rows.Close()
				return err
			}
			allocations = append(allocations, a)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, a := range allocations {
			rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,CASE WHEN $5>0 THEN GREATEST(0,reserved_g-consumed_g-returned_g) ELSE consumed_g END,CASE WHEN $5>0 THEN GREATEST(0,reserved_units-consumed_units-returned_units) ELSE consumed_units END FROM %s.work_order_material_reservation_batches WHERE work_order_id=$1 AND material_batch_id=$2 AND warehouse='wip' AND owner_customer_id=$3 AND component_id=$4 AND status IN('reserved','consumed') ORDER BY id FOR UPDATE`, schema), detail.WorkOrderID, a.id, item.OwnerCustomerID, item.MaterialID, direction)
			if err != nil {
				return err
			}
			bindings := []allocation{}
			for rows.Next() {
				var b allocation
				if err = rows.Scan(&b.id, &b.g, &b.n); err != nil {
					rows.Close()
					return err
				}
				bindings = append(bindings, b)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			g, n := a.g, a.n
			for _, b := range bindings {
				takeG, takeN := minInt64(g, b.g), minInt64(n, b.n)
				if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,status=CASE WHEN consumed_g+$2+returned_g>=reserved_g AND consumed_units+$3+returned_units>=reserved_units THEN 'consumed' ELSE 'reserved' END,updated_at=now() WHERE id=$1`, schema), b.id, direction*takeG, direction*takeN); err != nil {
					return err
				}
				g -= takeG
				n -= takeN
			}
			if g > 0 || n > 0 {
				return fmt.Errorf("WIP 冻结批次的可耗用数量已变化")
			}
		}
	}
	return nil
}
