package production

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func uniquePositiveProductionIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func productionPlanItemStartNeed(item productionapp.ProductionPlanItem) productionapp.StartNeed {
	label := strings.TrimSpace(item.InventoryUnit)
	salesUnit := strings.TrimSpace(item.InventoryUnit)
	parentProductID := item.ParentProductID
	if item.SpecG > 0 {
		label = fmt.Sprintf("%dg", item.SpecG)
	}
	if raw := strings.TrimSpace(item.SalesSpecSnapshotJSON); raw != "" {
		var frozen productionQuantitySnapshot
		if json.Unmarshal([]byte(raw), &frozen) == nil {
			label = firstNonEmpty(frozen.SpecLabel, label)
			salesUnit = firstNonEmpty(frozen.SalesUnit, salesUnit)
			if parentProductID <= 0 {
				parentProductID = frozen.ParentProductID
			}
		}
	}
	return productionapp.StartNeed{
		OrderDetails: item.DemandSources, ProductID: item.ProductID, ParentProductID: parentProductID,
		BomSpecID: item.BomSpecID, BomVariantID: item.BomVariantID, ProductName: item.ProductName,
		SpecLabel: label, SalesUnit: salesUnit, SpecG: item.SpecG, GapG: item.PlannedOutputG,
		SalesSpecCount: item.SalesSpecCount, InventoryQtyPerSalesUnit: item.InventoryQtyPerSalesUnit,
		InventoryUnit: item.InventoryUnit, PlannedInventoryQty: item.PlannedInventoryQty,
		SalesSpecSnapshotJSON: item.SalesSpecSnapshotJSON, OrderNos: item.OrderNos,
		OperationTemplateID: item.OperationTemplateID, CustomerID: item.CustomerID,
		TargetWarehouse: item.TargetWarehouse, ProcessingRequestItemID: item.ProcessingRequestItemID,
	}
}

func productionReplanDemandFromNeed(itemID int64, need productionapp.StartNeed) productionapp.ProductionReplanDemand {
	return productionapp.ProductionReplanDemand{
		ProductionPlanItemID: itemID, ProductID: need.ProductID, ProductName: need.ProductName,
		SpecLabel: need.SpecLabel, Quantity: need.PlannedInventoryQty, Unit: need.InventoryUnit,
		QuantityG: need.GapG, OrderNos: need.OrderNos, Orders: need.OrderDetails,
	}
}

func connectedProductionReplanRoots(rootIDs, affectedIDs []int64, items []productionapp.ProductionPlanItem) ([]int64, []productionapp.StartNeed, []productionapp.ProductionReplanDemand, int64) {
	selected := map[int64]bool{}
	for _, id := range rootIDs {
		selected[id] = true
	}
	affected := map[int64]bool{}
	for _, id := range affectedIDs {
		affected[id] = true
	}
	ids := []int64{}
	needs := []productionapp.StartNeed{}
	demands := []productionapp.ProductionReplanDemand{}
	var totalG int64
	for _, item := range items {
		if !affected[item.ID] || selected[item.ID] || item.ReplanStatus == "withdrawn" || strings.ToLower(strings.TrimSpace(item.OutputType)) != "product" {
			continue
		}
		need := productionPlanItemStartNeed(item)
		if len(need.OrderDetails) == 0 && strings.TrimSpace(need.OrderNos) == "" {
			continue
		}
		selected[item.ID] = true
		ids = append(ids, item.ID)
		needs = append(needs, need)
		demands = append(demands, productionReplanDemandFromNeed(item.ID, need))
		totalG += need.GapG
	}
	return ids, needs, demands, totalG
}

func productionReplanNeedDestinationKey(need productionapp.StartNeed) string {
	snapshot := productionQuantitySnapshot{
		SKUID:                    need.ProductID,
		ParentProductID:          need.ParentProductID,
		BomSpecID:                need.BomSpecID,
		BomVariantID:             need.BomVariantID,
		SpecLabel:                strings.TrimSpace(need.SpecLabel),
		SalesUnit:                strings.TrimSpace(need.SalesUnit),
		InventoryUnit:            strings.TrimSpace(need.InventoryUnit),
		InventoryQtyPerSalesUnit: need.InventoryQtyPerSalesUnit,
		CustomerID:               need.CustomerID,
		ProcessingRequestItemID:  need.ProcessingRequestItemID,
	}
	if raw := strings.TrimSpace(need.SalesSpecSnapshotJSON); raw != "" {
		var frozen productionQuantitySnapshot
		if json.Unmarshal([]byte(raw), &frozen) == nil {
			snapshot.ConversionSource = frozen.ConversionSource
			if frozen.CustomerID > 0 {
				snapshot.CustomerID = frozen.CustomerID
			}
			if frozen.ProcessingRequestItemID > 0 {
				snapshot.ProcessingRequestItemID = frozen.ProcessingRequestItemID
			}
		}
	}
	return productionQuantitySnapshotGroupKey(snapshot)
}

func alignProductionReplanNeedDestinations(needs []productionapp.StartNeed) []productionapp.StartNeed {
	out := append([]productionapp.StartNeed(nil), needs...)
	preferred := map[string]string{}
	for _, need := range out {
		if target := strings.TrimSpace(need.TargetWarehouse); target != "" {
			key := productionReplanNeedDestinationKey(need)
			if preferred[key] == "" {
				preferred[key] = target
			}
		}
	}
	for index := range out {
		if strings.TrimSpace(out[index].TargetWarehouse) == "" {
			out[index].TargetWarehouse = preferred[productionReplanNeedDestinationKey(out[index])]
		}
	}
	return out
}

func productionReplanClosureTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, roots []int64) ([]int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH RECURSIVE closure(id) AS (
			SELECT unnest($2::bigint[])
			UNION
			SELECT CASE WHEN dep.production_plan_item_id=closure.id THEN dep.depends_on_plan_item_id ELSE dep.production_plan_item_id END
			FROM %s.production_plan_item_dependencies dep
			JOIN closure ON dep.production_plan_item_id=closure.id OR dep.depends_on_plan_item_id=closure.id
			WHERE dep.production_plan_id=$1
		)
		SELECT id FROM closure ORDER BY id
	`, schema), planID, roots)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r Repository) productionReplanPreviewTx(ctx context.Context, tx pgx.Tx, cmd productionapp.ProductionReplanPreviewCommand, lock bool) (productionapp.ProductionReplanPreview, []productionapp.StartNeed, error) {
	preview := productionapp.ProductionReplanPreview{ProductionPlanID: cmd.ProductionPlanID, Revision: cmd.Revision, WIPNotice: "已领入在制仓的物料保留原库存位置，新草稿会重新核对可用来源。"}
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	var status string
	var revision int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT plan_no,status,revision FROM %s.production_plans WHERE id=$1%s`, r.schema, lockClause), cmd.ProductionPlanID).Scan(&preview.ProductionPlanNo, &status, &revision); err != nil {
		if err == pgx.ErrNoRows {
			return preview, nil, fmt.Errorf("production plan not found")
		}
		return preview, nil, err
	}
	preview.Revision = revision
	if revision != cmd.Revision {
		return preview, nil, fmt.Errorf("生产计划版本已变化，请重新读取后再操作")
	}
	if status != "submitted" {
		return preview, nil, fmt.Errorf("只有已提交且尚未开工的生产需求可以撤回重排")
	}
	rootIDs := uniquePositiveProductionIDs(cmd.ProductionPlanItemIDs)
	items, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.ProductionPlanID)
	if err != nil {
		return preview, nil, err
	}
	itemByID := map[int64]productionapp.ProductionPlanItem{}
	for _, item := range items {
		itemByID[item.ID] = item
	}
	originalNeeds := []productionapp.StartNeed{}
	for _, id := range rootIDs {
		item, ok := itemByID[id]
		if !ok || item.PlanID != cmd.ProductionPlanID || item.ReplanStatus == "withdrawn" {
			return preview, nil, fmt.Errorf("所选生产需求已失效，请重新读取")
		}
		if strings.ToLower(strings.TrimSpace(item.OutputType)) != "product" {
			return preview, nil, fmt.Errorf("请从可独立恢复订单需求的商品任务发起重排")
		}
		need := productionPlanItemStartNeed(item)
		if len(need.OrderDetails) == 0 && strings.TrimSpace(need.OrderNos) == "" {
			return preview, nil, fmt.Errorf("所选商品缺少订单需求追溯，需人工核对后处理")
		}
		originalNeeds = append(originalNeeds, need)
		preview.OriginalDemands = append(preview.OriginalDemands, productionReplanDemandFromNeed(id, need))
		preview.TotalQuantityG += need.GapG
	}
	preview.RootItemIDs = rootIDs
	preview.AffectedItemIDs, err = productionReplanClosureTx(ctx, tx, r.schema, cmd.ProductionPlanID, rootIDs)
	if err != nil {
		return preview, nil, err
	}
	// Shared upstream items can connect multiple customer-facing roots. Carry
	// every connected root into the replacement draft so no demand is lost.
	extraRootIDs, extraNeeds, extraDemands, extraTotalG := connectedProductionReplanRoots(rootIDs, preview.AffectedItemIDs, items)
	if len(extraRootIDs) > 0 {
		rootIDs = append(rootIDs, extraRootIDs...)
		originalNeeds = append(originalNeeds, extraNeeds...)
		preview.OriginalDemands = append(preview.OriginalDemands, extraDemands...)
		preview.TotalQuantityG += extraTotalG
		preview.ScopeExpanded = true
	}
	preview.RootItemIDs = rootIDs

	additionalNeeds := []productionapp.StartNeed{}
	if len(cmd.Selected) > 0 {
		additionalNeeds, err = r.productionPlanSelectedNeeds(ctx, tx, productionapp.CreateProductionPlanCommand{
			From: cmd.From, To: cmd.To, CustomerID: cmd.CustomerID, SourceType: "erp_order",
			Selected: cmd.Selected, InputByKey: cmd.InputByKey,
		})
		if err != nil {
			return preview, nil, err
		}
		for _, need := range additionalNeeds {
			preview.AdditionalDemands = append(preview.AdditionalDemands, productionReplanDemandFromNeed(0, need))
			preview.TotalQuantityG += need.GapG
		}
	}
	if lock && len(preview.AffectedItemIDs) > 0 {
		lockedRows, lockErr := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND production_plan_item_id=ANY($2::bigint[]) ORDER BY id FOR UPDATE`, r.schema), cmd.ProductionPlanID, preview.AffectedItemIDs)
		if lockErr != nil {
			return preview, nil, lockErr
		}
		lockedRows.Close()
		if lockErr = lockedRows.Err(); lockErr != nil {
			return preview, nil, lockErr
		}
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.production_plan_item_id,wo.status,wo.running_item_id,
		       COALESCE(string_agg(DISTINCT NULLIF(jc.workstation,''),','),''),COUNT(DISTINCT jc.id)::bigint,
		       COUNT(*) FILTER (WHERE jc.started_at IS NOT NULL OR jc.status NOT IN ('pending','ready','cancelled'))::bigint
		FROM %s.work_orders wo
		LEFT JOIN %s.job_cards jc ON jc.work_order_id=wo.id
		WHERE wo.production_plan_id=$1 AND wo.production_plan_item_id=ANY($2::bigint[])
		GROUP BY wo.id
		ORDER BY wo.id
	`, r.schema, r.schema), cmd.ProductionPlanID, preview.AffectedItemIDs)
	if err != nil {
		return preview, nil, err
	}
	for rows.Next() {
		var row productionapp.ProductionReplanWorkOrder
		var runningID, startedCount int64
		if err := rows.Scan(&row.ID, &row.WorkOrderNo, &row.ProductionPlanItemID, &row.Status, &runningID, &row.Workstation, &row.BatchCount, &startedCount); err != nil {
			rows.Close()
			return preview, nil, err
		}
		preview.WorkOrders = append(preview.WorkOrders, row)
		if runningID > 0 || startedCount > 0 || (row.Status != "released" && row.Status != "cancelled") {
			preview.BlockingReasons = append(preview.BlockingReasons, fmt.Sprintf("工单 %s 已开工或状态已变化，不能撤回", row.WorkOrderNo))
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return preview, nil, err
	}
	rows.Close()
	workOrderIDs := make([]int64, 0, len(preview.WorkOrders))
	for _, row := range preview.WorkOrders {
		workOrderIDs = append(workOrderIDs, row.ID)
	}
	if len(workOrderIDs) > 0 {
		var irreversible int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT
			  (SELECT COUNT(*) FROM %s.work_order_material_reservations WHERE work_order_id=ANY($1::bigint[]) AND (consumed_g>0 OR consumed_units>0)) +
			  (SELECT COUNT(*) FROM %s.stock_entries WHERE work_order_id=ANY($1::bigint[]) AND status='submitted' AND entry_type<>'material_issue_to_wip')
		`, r.schema, r.schema), workOrderIDs).Scan(&irreversible); err != nil {
			return preview, nil, err
		}
		if irreversible > 0 {
			preview.BlockingReasons = append(preview.BlockingReasons, "关联任务已有耗料、产出或不可逆库存记录，不能撤回")
		}
	}
	preview.CanReplan = len(preview.BlockingReasons) == 0
	return preview, append(originalNeeds, additionalNeeds...), nil
}

func (r Repository) PreviewProductionReplan(ctx context.Context, cmd productionapp.ProductionReplanPreviewCommand) (productionapp.ProductionReplanPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return productionapp.ProductionReplanPreview{}, err
	}
	defer tx.Rollback(ctx)
	preview, _, err := r.productionReplanPreviewTx(ctx, tx, cmd, false)
	return preview, err
}

func (r Repository) createReplanDraftFromNeedsTx(ctx context.Context, tx pgx.Tx, cmd productionapp.ProductionReplanCommand, needs []productionapp.StartNeed) (int64, error) {
	groups := groupStartNeedsForRuns(alignProductionReplanNeedDestinations(needs), cmd.InputByKey)
	if len(groups) == 0 {
		return 0, fmt.Errorf("撤回和新增需求合计必须大于零")
	}
	tmpNo := fmt.Sprintf("PP-TMP-RP-%d", time.Now().UnixNano())
	var planID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plans(plan_no,source_type,status,from_date,to_date,customer_id,created_by,created_at,picking_version,replanned_from_plan_id,replan_note)
		VALUES($1,'replan','draft',NULLIF($2,'')::date,NULLIF($3,'')::date,$4,$5,now(),1,$6,$7)
		RETURNING id
	`, r.schema), tmpNo, cmd.From, cmd.To, cmd.CustomerID, cmd.Operator, cmd.ProductionPlanID, "撤回未开工需求并重新安排").Scan(&planID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET plan_no=$2 WHERE id=$1`, r.schema), planID, productionPlanNo(planID)); err != nil {
		return 0, err
	}
	rootItems := make([]productionapp.ProductionPlanItem, 0, len(groups))
	for _, group := range groups {
		item, err := createProductionPlanItemForGroupTx(ctx, tx, r.schema, planID, group)
		if err != nil {
			return 0, err
		}
		rootItems = append(rootItems, item)
	}
	for i := range rootItems {
		root := &rootItems[i]
		root.OutputType, root.OutputProductID, root.OutputName = "product", root.ProductID, root.ProductName
		root.OutputQty, root.OutputUnit = root.PlannedInventoryQty, root.InventoryUnit
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET output_type='product',output_product_id=product_id,output_material_id=0,output_name=product_name,output_qty=planned_inventory_qty,output_unit=inventory_unit WHERE id=$1`, r.schema), root.ID); err != nil {
			return 0, err
		}
	}
	usesTyped, err := productionPlanUsesTypedOutputBindingsTx(ctx, tx, r.schema, planID)
	if err != nil {
		return 0, err
	}
	if usesTyped {
		if err := createMultilevelProductionPlanItemsTx(ctx, tx, r.schema, planID, rootItems); err != nil {
			return 0, err
		}
	}
	allItems, err := loadProductionPlanItemsTx(ctx, tx, r.schema, planID)
	if err != nil {
		return 0, err
	}
	if err := syncProductionPlanComponentSourcesTx(ctx, tx, r.schema, planID, allItems); err != nil {
		return 0, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &planID, "create_replan_draft", postgresinfra.StrPtr("source_type"), nil, postgresinfra.StrPtr("replan"), postgresinfra.AuditMeta{"previous_plan_id": cmd.ProductionPlanID, "item_count": len(rootItems)}); err != nil {
		return 0, err
	}
	return planID, nil
}

func (r Repository) ReplanProduction(ctx context.Context, cmd productionapp.ProductionReplanCommand) (productionapp.ProductionReplanResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	defer tx.Rollback(ctx)
	previousID, _, err := requestProductionKeyTx(ctx, tx, r.schema, "replan_plan", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if previousID > 0 {
		newPlan, err := loadProductionPlanDetailTx(ctx, tx, r.schema, previousID)
		if err != nil {
			return productionapp.ProductionReplanResult{}, err
		}
		var oldNo string
		_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT plan_no FROM %s.production_plans WHERE id=$1`, r.schema), cmd.ProductionPlanID).Scan(&oldNo)
		return productionapp.ProductionReplanResult{PreviousPlanID: cmd.ProductionPlanID, PreviousPlanNo: oldNo, NewPlan: newPlan}, nil
	}
	preview, needs, err := r.productionReplanPreviewTx(ctx, tx, cmd.ProductionReplanPreviewCommand, true)
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if !preview.CanReplan {
		return productionapp.ProductionReplanResult{}, fmt.Errorf("%s", strings.Join(preview.BlockingReasons, "；"))
	}
	needsHash := scheduleHash(needs)
	if err := lockStartRefsTx(ctx, tx, r.schema, startNeedRefs(needs)); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	// A normal plan creation can claim one of the selected additional demands
	// while this transaction is waiting for its source-order lock. Re-read the
	// complete scope after acquiring those locks and reject a changed preview;
	// never silently create a replacement draft with only part of the selection.
	currentPreview, currentNeeds, err := r.productionReplanPreviewTx(ctx, tx, cmd.ProductionReplanPreviewCommand, true)
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if !currentPreview.CanReplan || scheduleHash(currentNeeds) != needsHash {
		return productionapp.ProductionReplanResult{}, fmt.Errorf("待计划需求或关联任务版本已变化，请重新预览后再操作")
	}
	preview, needs = currentPreview, currentNeeds
	for _, wo := range preview.WorkOrders {
		if wo.Status != "cancelled" {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='cancelled',completed_at=now(),replan_status='withdrawn',replan_reason='撤回并重新安排' WHERE id=$1`, r.schema), wo.ID); err != nil {
				return productionapp.ProductionReplanResult{}, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='cancelled',completed_at=now(),operator=COALESCE(NULLIF(operator,''),$2) WHERE work_order_id=$1 AND status NOT IN ('completed','cancelled')`, r.schema), wo.ID, cmd.Operator); err != nil {
				return productionapp.ProductionReplanResult{}, err
			}
			if err := releaseMaterialReservationsForWorkOrderTx(ctx, tx, r.schema, wo.ID); err != nil {
				return productionapp.ProductionReplanResult{}, err
			}
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET replan_status='withdrawn',replan_reason='撤回并重新安排' WHERE production_plan_id=$1 AND id=ANY($2::bigint[])`, r.schema), cmd.ProductionPlanID, preview.AffectedItemIDs); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	newPlanID, err := r.createReplanDraftFromNeedsTx(ctx, tx, cmd, needs)
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET replaced_by_plan_id=$3 WHERE production_plan_id=$1 AND id=ANY($2::bigint[])`, r.schema), cmd.ProductionPlanID, preview.AffectedItemIDs, newPlanID); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	workOrderIDs := make([]int64, 0, len(preview.WorkOrders))
	for _, wo := range preview.WorkOrders {
		workOrderIDs = append(workOrderIDs, wo.ID)
	}
	if len(workOrderIDs) > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET replaced_by_plan_id=$2 WHERE id=ANY($1::bigint[])`, r.schema), workOrderIDs, newPlanID); err != nil {
			return productionapp.ProductionReplanResult{}, err
		}
	}
	var remaining int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_plan_items WHERE production_plan_id=$1 AND replan_status<>'withdrawn' AND COALESCE(NULLIF(output_type,''),'product')='product'`, r.schema), cmd.ProductionPlanID).Scan(&remaining); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET status='replanned',revision=revision+1,replan_note='全部未开工需求已撤回重排' WHERE id=$1`, r.schema), cmd.ProductionPlanID); err != nil {
			return productionapp.ProductionReplanResult{}, err
		}
	} else if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET revision=revision+1,replan_note='部分未开工需求已撤回重排' WHERE id=$1`, r.schema), cmd.ProductionPlanID); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	for _, wo := range preview.WorkOrders {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &wo.ID, "withdraw_replan", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(wo.Status), postgresinfra.StrPtr("cancelled"), postgresinfra.AuditMeta{"previous_plan_id": cmd.ProductionPlanID, "new_plan_id": newPlanID}); err != nil {
			return productionapp.ProductionReplanResult{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.ProductionPlanID, "replan", postgresinfra.StrPtr("revision"), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.Revision)), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.Revision+1)), postgresinfra.AuditMeta{"new_plan_id": newPlanID, "root_item_ids": preview.RootItemIDs, "affected_item_ids": preview.AffectedItemIDs}); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	newPlan, err := loadProductionPlanDetailTx(ctx, tx, r.schema, newPlanID)
	if err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	result := productionapp.ProductionReplanResult{PreviousPlanID: cmd.ProductionPlanID, PreviousPlanNo: preview.ProductionPlanNo, NewPlan: newPlan}
	if err := finishProductionKeyTx(ctx, tx, r.schema, "replan_plan", cmd.Operator, cmd.RequestID, newPlanID, result); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionReplanResult{}, err
	}
	return result, nil
}
