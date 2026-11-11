package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	infra "orderapp/internal/infrastructure/postgres"
	"sort"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
)

func semiFinishedCandidatesTx(ctx context.Context, tx pgx.Tx, schema string, plan app.ProductionPlanDetail) (app.SemiFinishedStockResult, error) {
	out := app.SemiFinishedStockResult{DraftToken: plan.DraftToken, Items: []app.SemiFinishedStockItem{}, Changes: []app.SemiFinishedStockChange{}}
	var policy string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT semi_finished_stock_policy FROM %s.production_plans WHERE id=$1`, schema), plan.ID).Scan(&policy); err != nil {
		return out, err
	}
	if policy == "" && plan.SourceType != "stock" && (len(plan.SupplyPlan) == 0 || string(plan.SupplyPlan) == "null") {
		var hasSemi bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_plan_component_sources s JOIN %[1]s.materials m ON m.id=s.component_id WHERE s.production_plan_id=$1 AND s.component_type='material' AND COALESCE(m.is_semi_finished,false) AND %[2]s)`, schema, legacySemiFinishedManufacturableSQL(schema, "$1", "m.id")), plan.ID).Scan(&hasSemi); err != nil {
			return out, err
		}
		if hasSemi {
			return out, fmt.Errorf("历史草稿未保存半成品供应明细，无法在此选择或推断原有抵扣；请查看生产计划的已有任务和物料记录，核对后重新制定计划")
		}
	}
	var graph domain.ManufacturingPlan
	if len(plan.SupplyPlan) > 0 {
		if err := json.Unmarshal(plan.SupplyPlan, &graph); err != nil {
			return out, err
		}
	}
	normalized, err := normalizedSelectedSemiFinishedTx(ctx, tx, schema, plan.ID)
	if err != nil {
		return out, err
	}
	selected := map[string][2]int64{}
	for _, row := range normalized {
		selected[fmt.Sprintf("%d:%d:%s", row.materialID, row.batchID, row.warehouse)] = [2]int64{row.qtyG, row.qtyUnits}
	}
	for _, node := range graph.Nodes {
		if node.Item.Type != "material" {
			continue
		}
		var semi bool
		var owner int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(m.is_semi_finished,false) AND %s,COALESCE(m.owner_customer_id,0) FROM %s.materials m WHERE m.id=$1`, legacySemiFinishedManufacturableSQL(schema, "$2", "m.id"), schema), node.Item.ID, plan.ID).Scan(&semi, &owner); err != nil {
			return out, err
		}
		if !semi {
			continue
		}
		item := app.SemiFinishedStockItem{Key: node.Item.Key(), Name: node.Item.Name, Unit: node.Item.Unit, RequiredQty: node.RequiredQty, StockCoveredQty: node.StockCoveredQty, InflightCoveredQty: node.InflightCoveredQty, ManufactureQty: node.ShortageQty, Legacy: policy != "manual", Batches: []app.SemiFinishedBatch{}}
		seenBatches := map[string]bool{}
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code,l.warehouse,COALESCE(w.name,l.warehouse),COALESCE(c.name,'工厂'),COALESCE(b.quality_status,'unchecked'),l.qty_g,l.qty_units,COALESCE(r.g,0),COALESCE(r.n,0),COALESCE(s.qty_g,0),COALESCE(s.qty_units,0),COALESCE(b.received_at::text,''),COALESCE(wo.work_order_no,''),COALESCE(e.entry_no,'')
 FROM %[1]s.material_batch_locations l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id JOIN %[1]s.materials m ON m.id=b.material_id
 LEFT JOIN %[1]s.customers c ON c.id=b.owner_customer_id LEFT JOIN %[1]s.warehouses w ON w.code=l.warehouse
 LEFT JOIN LATERAL(SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) n FROM %[1]s.work_order_material_reservation_batches WHERE material_batch_id=b.id AND warehouse=l.warehouse AND status='reserved') r ON true
 LEFT JOIN %[1]s.production_plan_semi_finished_selections s ON s.production_plan_id=$2 AND s.item_key=$3 AND s.batch_id=b.id AND s.warehouse=l.warehouse
 LEFT JOIN LATERAL(SELECT work_order_id,stock_entry_id FROM %[1]s.production_material_receipts WHERE material_batch_id=b.id ORDER BY id DESC LIMIT 1) receipt ON true
 LEFT JOIN %[1]s.work_orders wo ON wo.id=receipt.work_order_id LEFT JOIN %[1]s.stock_entries e ON e.id=receipt.stock_entry_id
 WHERE l.material_id=$1 AND b.status='active' AND b.quality_status NOT IN('hold','reject') AND b.owner_customer_id=COALESCE(m.owner_customer_id,0) AND (l.qty_g>0 OR l.qty_units>0) ORDER BY b.received_at,b.id,l.warehouse`, schema), node.Item.ID, plan.ID, item.Key)
		if err != nil {
			return out, err
		}
		for rows.Next() {
			var b app.SemiFinishedBatch
			var g, n, rg, rn, sg, sn int64
			if err = rows.Scan(&b.BatchID, &b.BatchCode, &b.Warehouse, &b.WarehouseName, &b.OwnerName, &b.QualityStatus, &g, &n, &rg, &rn, &sg, &sn, &b.ReceivedAt, &b.SourceWorkOrderNo, &b.SourceEntryNo); err != nil {
				rows.Close()
				return out, err
			}
			b.OnhandQty = manufacturingQtyFromCanonical(g, n, item.Unit)
			b.ReservedQty = manufacturingQtyFromCanonical(rg, rn, item.Unit)
			b.AvailableQty = manufacturingQtyFromCanonical(nonnegativeQuantity(g-rg), nonnegativeQuantity(n-rn), item.Unit)
			chosen := selected[fmt.Sprintf("%d:%d:%s", node.Item.ID, b.BatchID, b.Warehouse)]
			b.SelectedQty = manufacturingQtyFromCanonical(chosen[0], chosen[1], item.Unit)
			item.Batches = append(item.Batches, b)
			seenBatches[fmt.Sprintf("%d:%s", b.BatchID, b.Warehouse)] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return out, err
		}
		unboundByWarehouse := map[string]float64{}
		for j := range item.Batches {
			b := &item.Batches[j]
			unbound, ok := unboundByWarehouse[b.Warehouse]
			if !ok {
				g, n, e := unboundWorkOrderReservationQtyTx(ctx, tx, schema, 0, node.Item.ID, b.Warehouse, owner)
				if e != nil {
					return out, e
				}
				unbound = manufacturingQtyFromCanonical(g, n, item.Unit)
				unboundByWarehouse[b.Warehouse] = unbound
			}
			protect := math.Min(b.AvailableQty, unbound)
			b.AvailableQty -= protect
			b.ReservedQty += protect
		}
		// Keep invalidated selections visible so reopening a draft cannot silently
		// erase a batch that was frozen, moved outside the plan, or consumed.
		for _, chosen := range normalized {
			if chosen.materialID != node.Item.ID || seenBatches[fmt.Sprintf("%d:%s", chosen.batchID, chosen.warehouse)] {
				continue
			}
			b := app.SemiFinishedBatch{BatchID: chosen.batchID, BatchCode: chosen.batchCode, Warehouse: chosen.warehouse, WarehouseName: chosen.warehouse, QualityStatus: "不可用", SelectedQty: manufacturingQtyFromCanonical(chosen.qtyG, chosen.qtyUnits, item.Unit)}
			item.Batches = append(item.Batches, b)
		}
		out.Items = append(out.Items, item)
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Key < out.Items[j].Key })
	return out, nil
}

func (r Repository) GetSemiFinishedStock(ctx context.Context, id int64) (app.SemiFinishedStockResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	defer tx.Rollback(ctx)
	plan, err := loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	return semiFinishedCandidatesTx(ctx, tx, r.schema, plan)
}

// UpdateSemiFinishedStock previews in a transaction that always rolls back; the
// confirming request repeats all checks while holding the plan and batch locks.
func (r Repository) UpdateSemiFinishedStock(ctx context.Context, cmd app.SemiFinishedStockCommand) (app.SemiFinishedStockResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	defer tx.Rollback(ctx)
	var status, sourceType string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,source_type FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), cmd.PlanID).Scan(&status, &sourceType); err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	if status != "draft" || sourceType == "stock" {
		return app.SemiFinishedStockResult{}, fmt.Errorf("仅订单生产草稿可以调整半成品库存")
	}
	plan, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.PlanID)
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	if plan.DraftToken != cmd.DraftToken {
		return app.SemiFinishedStockResult{}, fmt.Errorf("生产计划版本已变化，请重新核对")
	}
	capacitySnapshot, err := lockSemiFinishedCapacityReviewTx(ctx, tx, r.schema, plan)
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	for _, doc := range plan.MaterialTransferDocuments {
		if doc.Status == "draft" {
			return app.SemiFinishedStockResult{}, fmt.Errorf("调拨单 %s 待确认或取消，请处理后再调整", doc.EntryNo)
		}
	}
	// Serialize with warehouse confirmation and other reservations before validating.
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT b.id FROM %[1]s.material_batches b WHERE b.id=ANY($1) ORDER BY b.id FOR UPDATE`, r.schema), selectionBatchIDs(cmd.Selections))
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return app.SemiFinishedStockResult{}, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT l.material_batch_id FROM %s.material_batch_locations l WHERE l.material_batch_id=ANY($1) ORDER BY l.material_batch_id,l.warehouse FOR UPDATE`, r.schema), selectionBatchIDs(cmd.Selections))
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return app.SemiFinishedStockResult{}, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	before, err := semiFinishedCandidatesTx(ctx, tx, r.schema, plan)
	if err != nil {
		return before, err
	}
	byKey := map[string]app.SemiFinishedStockItem{}
	for _, item := range before.Items {
		byKey[item.Key] = item
	}
	seen := map[string]bool{}
	totals := map[string]float64{}
	for _, s := range cmd.Selections {
		item, ok := byKey[s.Key]
		if !ok || s.BatchID <= 0 || strings.TrimSpace(s.Warehouse) == "" || s.Qty <= 0 || math.IsNaN(s.Qty) || math.IsInf(s.Qty, 0) {
			return before, fmt.Errorf("半成品或选择数量无效，请重新选择")
		}
		selectionKey := fmt.Sprintf("%d:%s", s.BatchID, s.Warehouse)
		if seen[selectionKey] {
			return before, fmt.Errorf("同一批次只能选择一次")
		}
		seen[selectionKey] = true
		var selected *app.SemiFinishedBatch
		for i := range item.Batches {
			b := &item.Batches[i]
			if b.BatchID == s.BatchID && b.Warehouse == s.Warehouse {
				selected = b
				break
			}
		}
		if selected == nil || s.Qty > selected.AvailableQty+0.000001 {
			return before, fmt.Errorf("%s 的批次可用数量已变化，请重新选择", item.Name)
		}
		g, n := canonicalFromManufacturingQty(s.Qty, item.Unit)
		if math.Abs(manufacturingQtyFromCanonical(g, n, item.Unit)-s.Qty) > 0.000001 {
			return before, fmt.Errorf("%s 的选择数量超出库存单位精度", item.Name)
		}
		totals[s.Key] += s.Qty
		if totals[s.Key] > item.RequiredQty+0.000001 {
			return before, fmt.Errorf("%s 的选择数量超过需求", item.Name)
		}
	}
	oldItems, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.PlanID)
	if err != nil {
		return before, err
	}
	_, oldUpstream, err := semiFinishedRootsTx(ctx, tx, r.schema, cmd.PlanID, oldItems)
	if err != nil {
		return before, err
	}
	for _, item := range before.Items {
		if !item.Legacy || item.StockCoveredQty <= 0 {
			continue
		}
		if _, ok := oldUpstream[item.Key]; ok {
			continue
		}
		var frozen bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=$1 AND item_key=$2)`, r.schema), cmd.PlanID, item.Key).Scan(&frozen); err != nil {
			return before, err
		}
		if !frozen {
			return before, fmt.Errorf("%s 原计划由库存完全覆盖，未保存历史生产配方和工艺，无法安全重算；请先核对生产资料", item.Name)
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='manual' WHERE id=$1`, r.schema), cmd.PlanID); err != nil {
		return before, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_semi_finished_selections WHERE production_plan_id=$1`, r.schema), cmd.PlanID); err != nil {
		return before, err
	}
	for _, s := range cmd.Selections {
		item := byKey[s.Key]
		var materialID int64
		if _, err = fmt.Sscanf(s.Key, "material:%d", &materialID); err != nil || materialID <= 0 {
			return before, fmt.Errorf("物料标识无效")
		}
		g, n := canonicalFromManufacturingQty(s.Qty, item.Unit)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units,transfer_baseline_g,transfer_baseline_units)
 SELECT $1,$2,$3,$4,b.id,$6,$7,COALESCE(t.g,0),COALESCE(t.n,0)
 FROM %[1]s.material_batches b
 LEFT JOIN LATERAL (SELECT SUM(i.qty_g) g,SUM(i.qty_units) n
 FROM %[1]s.production_plan_material_transfer_links l JOIN %[1]s.stock_entries e ON e.id=l.stock_entry_id AND e.status='submitted'
 JOIN %[1]s.stock_entry_items i ON i.id=l.stock_entry_item_id AND i.stock_entry_id=e.id
 WHERE l.production_plan_id=$1 AND i.item_type='material' AND i.material_id=$3 AND i.batch_code=b.batch_code AND i.from_warehouse=$4 AND i.owner_customer_id=b.owner_customer_id) t ON true
 WHERE b.id=$5`, r.schema), cmd.PlanID, s.Key, materialID, s.Warehouse, s.BatchID, g, n); err != nil {
			return before, err
		}
	}
	items, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.PlanID)
	if err != nil {
		return before, err
	}
	roots, reuse, err := semiFinishedRootsTx(ctx, tx, r.schema, cmd.PlanID, items)
	if err != nil {
		return before, err
	}
	expansion, err := buildManufacturingExpansionTx(ctx, tx, r.schema, roots)
	if err != nil {
		return before, err
	}
	afterByKey := map[string]domain.ManufacturingPlanNode{}
	for _, node := range expansion.Plan.Nodes {
		afterByKey[node.Item.Key()] = node
	}
	before.Changes, err = previewSemiFinishedCapacity(ctx, tx, r.schema, plan, expansion, reuse)
	if err != nil {
		return before, err
	}
	for i := range before.Items {
		item := &before.Items[i]
		node, ok := afterByKey[item.Key]
		if !ok && totals[item.Key] > 0 {
			return before, fmt.Errorf("%s 已不再需要，请重新选择", item.Name)
		}
		if ok {
			item.RequiredQty = node.RequiredQty
			item.StockCoveredQty = node.StockCoveredQty
			item.InflightCoveredQty = node.InflightCoveredQty
			item.ManufactureQty = node.ShortageQty
			item.Legacy = false
		}
		for j := range item.Batches {
			item.Batches[j].SelectedQty = 0
			for _, s := range cmd.Selections {
				if s.Key == item.Key && s.BatchID == item.Batches[j].BatchID && s.Warehouse == item.Batches[j].Warehouse {
					item.Batches[j].SelectedQty = s.Qty
				}
			}
		}
	}
	before.PreviewToken, err = semiFinishedPreviewToken(cmd, before, expansion, capacitySnapshot)
	if err != nil {
		return before, err
	}
	if cmd.CheckOnly {
		return before, nil
	}
	if cmd.PreviewToken == "" || cmd.PreviewToken != before.PreviewToken {
		return app.SemiFinishedStockResult{}, fmt.Errorf("库存或工位产能已变化，请重新预览后确认")
	}
	adjustments, err := preservedPickingAdjustmentsTx(ctx, tx, r.schema, cmd.PlanID, items)
	if err != nil {
		return before, err
	}
	oldByKey := map[string]app.ProductionPlanItem{}
	for key, item := range reuse {
		oldByKey[key] = item
	}
	for _, table := range []string{"production_supply_allocations", "production_plan_item_dependencies", "production_plan_supply_gaps"} {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_id=$1`, r.schema, table), cmd.PlanID); err != nil {
			return before, err
		}
	}
	expanded, err := createExpandedProductionItemsReusingTx(ctx, tx, r.schema, cmd.PlanID, roots, reuse, &expansion)
	if err != nil {
		return before, err
	}
	for _, item := range reuse {
		if err = archiveSemiFinishedCapacityTx(ctx, tx, r.schema, cmd.PlanID, item); err != nil {
			return before, err
		}
		for _, table := range []string{"production_plan_operation_splits", "production_plan_component_sources"} {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.%s WHERE production_plan_item_id=$1`, r.schema, table), item.ID); err != nil {
				return before, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_items WHERE id=$1`, r.schema), item.ID); err != nil {
			return before, err
		}
	}
	rootIDs := map[int64]bool{}
	for _, root := range roots {
		rootIDs[root.ID] = true
	}
	for _, item := range expanded.Items {
		if rootIDs[item.ID] {
			continue
		}
		typ, outputID := item.OutputType, item.OutputProductID
		if typ == "material" {
			outputID = item.OutputMaterialID
		}
		key := manufacturingItemIdentityKey(typ, outputID, item.BomSpecID)
		old, wasLive := oldByKey[key]
		splits := []app.ProductionPlanOperationSplit{}
		if wasLive {
			for _, split := range plan.OperationSplits {
				if split.ProductionPlanItemID == old.ID {
					splits = append(splits, split)
				}
			}
		} else {
			old, splits, err = loadArchivedSemiFinishedCapacityTx(ctx, tx, r.schema, cmd.PlanID, key)
			if err != nil {
				return before, err
			}
		}
		if old.OutputQty <= 0 || len(splits) == 0 || (wasLive && math.Abs(old.OutputQty-item.OutputQty) < 0.000001) {
			continue
		}
		if err = applySemiFinishedCapacityTx(ctx, tx, r.schema, cmd.PlanID, old, item, splits); err != nil {
			return before, err
		}
	}
	if err = syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, cmd.PlanID, expanded.Items); err != nil {
		return before, err
	}
	if err = restorePickingAdjustmentsTx(ctx, tx, r.schema, cmd.PlanID, expanded.Items, adjustments); err != nil {
		return before, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET picking_version=1,revision=revision+1 WHERE id=$1`, r.schema), cmd.PlanID); err != nil {
		return before, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.PlanID, "select_semi_finished_stock", nil, nil, nil, infra.AuditMeta{"selections": cmd.Selections, "changes": before.Changes}); err != nil {
		return before, err
	}
	updated, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.PlanID)
	if err != nil {
		return before, err
	}
	result, err := semiFinishedCandidatesTx(ctx, tx, r.schema, updated)
	if err != nil {
		return before, err
	}
	result.Plan = &updated
	result.Changes = before.Changes
	if err = tx.Commit(ctx); err != nil {
		return app.SemiFinishedStockResult{}, err
	}
	return result, nil
}

func selectionBatchIDs(items []app.SemiFinishedSelection) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.BatchID)
	}
	return ids
}

func semiFinishedRootsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []app.ProductionPlanItem) ([]app.ProductionPlanItem, map[string]app.ProductionPlanItem, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT DISTINCT depends_on_plan_item_id FROM %s.production_plan_item_dependencies WHERE production_plan_id=$1`, schema), planID)
	if err != nil {
		return nil, nil, err
	}
	children := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, err
		}
		children[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	roots := []app.ProductionPlanItem{}
	reuse := map[string]app.ProductionPlanItem{}
	for _, item := range items {
		if !children[item.ID] {
			roots = append(roots, item)
			continue
		}
		typ, id := item.OutputType, item.OutputProductID
		if typ == "material" {
			id = item.OutputMaterialID
		}
		key := manufacturingItemIdentityKey(typ, id, item.BomSpecID)
		if _, ok := reuse[key]; ok {
			return nil, nil, fmt.Errorf("存在多个相同产出任务，请先核对")
		}
		reuse[key] = item
	}
	return roots, reuse, nil
}
