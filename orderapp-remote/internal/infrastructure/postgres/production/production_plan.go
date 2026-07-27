package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func productionPlanNo(id int64) string {
	return fmt.Sprintf("PP-%010d", id)
}

func releasedWorkOrderNo(planID, itemID int64) string {
	return fmt.Sprintf("WO-PP-%010d-%010d", planID, itemID)
}

func startRunGroupKey(group startRunGroup) string {
	return fmt.Sprintf("%d-%d", group.ProductID, group.SpecG)
}

func (r Repository) CreateProductionPlan(ctx context.Context, cmd productionapp.CreateProductionPlanCommand) (productionapp.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	needs, err := r.productionPlanSelectedNeeds(ctx, tx, cmd)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if len(needs) == 0 {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("selected production items required")
	}
	refs := startNeedRefs(needs)
	if err := lockStartRefsTx(ctx, tx, r.schema, refs); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	// A concurrent request may have planned the same source demand while this
	// transaction was waiting for the order/request row locks. Re-read every
	// demand and derived plan status through the current transaction after the
	// locks are held; never create a plan from the stale pre-lock selection.
	needs, err = r.productionPlanSelectedNeeds(ctx, tx, cmd)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if len(needs) == 0 {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("selected production items required")
	}

	groups := groupStartNeedsForRuns(needs, cmd.InputByKey, map[int64]float64{})
	if len(groups) == 0 {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("selected production items required")
	}

	tmpNo := fmt.Sprintf("PP-TMP-%d", time.Now().UnixNano())
	var planID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plans(plan_no,source_type,status,from_date,to_date,customer_id,created_by,created_at)
		VALUES($1,$2,'draft',NULLIF($3,'')::date,NULLIF($4,'')::date,$5,$6,now())
		RETURNING id
	`, r.schema), tmpNo, firstNonEmpty(cmd.SourceType, "erp_order"), cmd.From, cmd.To, cmd.CustomerID, cmd.Operator).Scan(&planID); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	planNo := productionPlanNo(planID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET plan_no=$2 WHERE id=$1`, r.schema), planID, planNo); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}

	for _, group := range groups {
		if group.NeedG <= 0 && group.PlannedInventoryQty <= 0 {
			continue
		}
		if _, err := createProductionPlanItemForGroupTx(ctx, tx, r.schema, planID, group); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &planID, "create", postgresinfra.StrPtr("status"), nil, postgresinfra.StrPtr("draft"), postgresinfra.AuditMeta{"source_type": cmd.SourceType}); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	return r.GetProductionPlan(ctx, planID)
}

func (r Repository) productionPlanSelectedNeeds(ctx context.Context, tx pgx.Tx, cmd productionapp.CreateProductionPlanCommand) ([]productionapp.StartNeed, error) {
	rows, err := fetchUnproducedNeeds(ctx, tx, r.schema, cmd.From, cmd.To, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	rows, err = r.splitUnproducedNeedsByProductionPlanQuery(ctx, tx, rows)
	if err != nil {
		return nil, err
	}
	appRows := unprodRowsToApp(rows)
	if err := r.attachProductionDemandStatusesQuery(ctx, tx, appRows); err != nil {
		return nil, err
	}
	for _, row := range appRows {
		if cmd.Selected[producePlanKey(row.ProductID, row.SpecG)] &&
			strings.TrimSpace(row.BlockingReason) != "" {
			return nil, fmt.Errorf("%s", row.BlockingReason)
		}
	}
	return selectedProductionPlanStartNeeds(appRows, cmd.Selected), nil
}

func selectedProductionPlanStartNeeds(rows []productionapp.UnprodNeedRow, selected map[string]bool) []productionapp.StartNeed {
	out := make([]productionapp.StartNeed, 0)
	for _, row := range rows {
		key := producePlanKey(row.ProductID, row.SpecG)
		if !selected[key] ||
			strings.TrimSpace(row.BlockingReason) != "" ||
			(row.GapG <= 0 && row.GapInventoryQty <= 0) ||
			row.DemandStatus != "unplanned" {
			continue
		}
		out = append(out, productionapp.StartNeed{
			ProductID:                row.ProductID,
			ParentProductID:          row.ParentProductID,
			ProductName:              row.Product,
			SpecLabel:                row.SpecLabel,
			SalesUnit:                row.SalesUnit,
			SpecG:                    row.SpecG,
			GapG:                     row.GapG,
			SalesSpecCount:           row.GapSalesSpecCount,
			InventoryQtyPerSalesUnit: row.InventoryQtyPerSalesUnit,
			InventoryUnit:            row.InventoryUnit,
			PlannedInventoryQty:      row.GapInventoryQty,
			SalesSpecSnapshotJSON:    row.SalesSpecSnapshotJSON,
			OrderNos:                 row.OrderNos,
			OperationTemplateID:      row.OperationTemplateID,
		})
	}
	return out
}

func freezeProductionPlanSalesSpecSnapshot(raw string, salesSpecCount, plannedInventoryQty float64) ([]byte, error) {
	snapshot := map[string]any{}
	raw = strings.TrimSpace(raw)
	if raw != "" && raw != "{}" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			return nil, fmt.Errorf("sales spec snapshot invalid: %w", err)
		}
	}
	snapshot["sales_spec_count"] = salesSpecCount
	snapshot["planned_inventory_qty"] = plannedInventoryQty
	return json.Marshal(snapshot)
}

func validateProductionDemandInventoryUnitAgainstBomOutput(group startRunGroup, bomRoute latestUsableBomRoute) error {
	inventoryUnit, inventoryDimension := normalizeProductionPlanQuantityDimension(group.InventoryUnit)
	if inventoryUnit == "" {
		// Legacy direct-production commands did not freeze an inventory unit.
		// Formal order demand always supplies it through the authoritative
		// production quantity snapshot or the current concrete SKU conversion.
		return nil
	}
	_, bomOutputDimension := normalizeProductionPlanQuantityDimension(bomRoute.BomOutputUnit)
	if inventoryDimension == "weight" && bomOutputDimension == "weight" {
		return nil
	}
	reason := "production demand unit incompatible with BOM output"
	if inventoryDimension != "weight" {
		reason = "formal production planning requires a weight inventory unit"
	}
	return fmt.Errorf(
		"%s: order %s / product %s / spec %s: inventory unit %s, BOM output unit %s",
		reason,
		firstNonEmpty(group.OrderNos, "(unknown)"),
		firstNonEmpty(group.ProductName, fmt.Sprintf("product#%d", group.ProductID)),
		firstNonEmpty(group.SpecLabel, group.SalesUnit, "(unknown)"),
		firstNonEmpty(strings.TrimSpace(group.InventoryUnit), "(empty)"),
		firstNonEmpty(strings.TrimSpace(bomRoute.BomOutputUnit), "(empty)"),
	)
}

func normalizeProductionPlanQuantityDimension(unit string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
		return "g", "weight"
	case "kg", "千克", "公斤":
		return "kg", "weight"
	case "lb", "磅":
		return "lb", "weight"
	case "unit", "units", "pc", "pcs", "件", "个":
		return "unit", "count"
	default:
		normalized := strings.ToLower(strings.TrimSpace(unit))
		if normalized == "" {
			return "", ""
		}
		return normalized, "named"
	}
}

func createProductionPlanItemForGroupTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, group startRunGroup) (productionapp.ProductionPlanItem, error) {
	bomRoute, err := resolveProductionBomForDemandProductTx(ctx, tx, schema, group.ProductID, group.ParentProductID, group.ProductName)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	if err := validateProductionDemandInventoryUnitAgainstBomOutput(group, bomRoute); err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	bomMaterialLossRate := normalizeMaterialLossRate(bomRoute.BomMaterialLossRate)
	normalizedYield := 1 - bomMaterialLossRate
	if !group.ManualInput {
		group.InputG = productionInputGFromBomMaterialLoss(group.NeedG, bomMaterialLossRate)
	}
	plan := runningInventoryPlan(group.SpecG, group.NeedG, group.InputG, normalizedYield)
	run := ProduceRunRow{
		Product:             group.ProductName,
		ProductID:           group.ProductID,
		SpecG:               group.SpecG,
		NeedG:               group.NeedG,
		InputG:              group.InputG,
		BomYieldRate:        normalizedYield,
		PlanUnits:           plan.Units,
		PlanLooseG:          plan.LooseG,
		OrderNos:            group.OrderNos,
		OperationTemplateID: group.OperationTemplateID,
		Outputs:             group.Outputs,
	}
	materialSnapshot, err := buildMaterialSnapshotForBomVersionTx(
		ctx,
		tx,
		schema,
		run,
		bomRoute.BomVersionID,
		bomMaterialLossRate > 0,
	)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	processSnapshot, processSnapshotJSON, err := loadProcessRouteSnapshotByIDTx(ctx, tx, schema, bomRoute.ProcessRouteID, group.ProductID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	if processSnapshot == nil {
		return productionapp.ProductionPlanItem{}, fmt.Errorf("最新可用 BOM 版本未配置工艺路线: %s/%s/%s", bomRoute.BomName, bomRoute.BomVersionNo, bomRoute.ProductName)
	}
	if len(processSnapshotJSON) == 0 {
		processSnapshotJSON = []byte("{}")
	}
	processSnapshot.ProductID = group.ProductID
	processSnapshot.ProductName = group.ProductName
	processSnapshot.BomVersionID = bomRoute.BomVersionID
	processSnapshot.BomVersionNo = bomRoute.BomVersionNo
	processSnapshot.YieldRate = normalizedYield
	processSnapshot.RouteID = bomRoute.ProcessRouteID
	processSnapshot.RouteName = bomRoute.ProcessRouteName
	processSnapshotJSON, err = json.Marshal(processSnapshot)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	processRouteID := bomRoute.ProcessRouteID
	productionConfigSnapshot, err := loadProductProductionConfigSnapshotForWorkOrderTx(ctx, tx, schema, bomRoute.BomSourceProductID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	customerProductSnapshot, err := loadCustomerProductSnapshotByOrderNosTx(ctx, tx, schema, group.OrderNos, group.ProductID, group.SpecG)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	bomVersionID := bomRoute.BomVersionID
	salesSpecSnapshotJSON, err := freezeProductionPlanSalesSpecSnapshot(
		group.SalesSpecSnapshotJSON,
		group.SalesSpecCount,
		group.PlannedInventoryQty,
	)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}

	var item productionapp.ProductionPlanItem
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			production_plan_id,product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
			sales_spec_count,inventory_qty_per_sales_unit,inventory_unit,planned_inventory_qty,sales_spec_snapshot_json,bom_inherited,
			planned_g,planned_output_g,gap_g,order_nos,
			bom_version_id,operation_template_id,process_route_id,component_snapshot_json,process_route_snapshot_json,
			production_config_snapshot_json,customer_product_snapshot_json,created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,$21::jsonb,$22::jsonb,$23::jsonb,now())
		RETURNING id,production_plan_id,product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
		          sales_spec_count::float8,inventory_qty_per_sales_unit::float8,inventory_unit,planned_inventory_qty::float8,
		          COALESCE(sales_spec_snapshot_json,'{}'::jsonb)::text,bom_inherited,
		          planned_g,planned_output_g,gap_g,order_nos,
		          bom_version_id,operation_template_id,process_route_id,
		          COALESCE(component_snapshot_json,'[]'::jsonb)::text,
		          COALESCE(process_route_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(production_config_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(customer_product_snapshot_json,'[]'::jsonb)::text
	`, schema),
		planID, group.ProductID, bomRoute.ParentProductID, bomRoute.BomSourceProductID, group.ProductName, group.SpecG,
		group.SalesSpecCount, group.InventoryQtyPerSalesUnit, group.InventoryUnit, group.PlannedInventoryQty, salesSpecSnapshotJSON, bomRoute.BomInherited,
		group.InputG, group.NeedG, group.NeedG, group.OrderNos, bomVersionID, group.OperationTemplateID, processRouteID,
		materialSnapshot, processSnapshotJSON, productionConfigSnapshot, customerProductSnapshot,
	).Scan(
		&item.ID, &item.PlanID, &item.ProductID, &item.ParentProductID, &item.BomSourceProductID, &item.ProductName, &item.SpecG,
		&item.SalesSpecCount, &item.InventoryQtyPerSalesUnit, &item.InventoryUnit, &item.PlannedInventoryQty, &item.SalesSpecSnapshotJSON, &item.BomInherited,
		&item.PlannedG, &item.PlannedOutputG, &item.GapG, &item.OrderNos,
		&item.BomVersionID, &item.OperationTemplateID, &item.ProcessRouteID, &item.MaterialSnapshot, &item.ProcessSnapshotJSON, &item.ProductionConfigSnapshotJSON, &item.CustomerProductSnapshotJSON,
	); err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	return item, nil
}

func createLegacyProductionPlanItemForGroupTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	planID int64,
	group startRunGroup,
	legacyYieldRate float64,
) (productionapp.ProductionPlanItem, error) {
	item, err := createProductionPlanItemForGroupTx(ctx, tx, schema, planID, group)
	if err == nil {
		return item, nil
	}
	if !isProductionBomNotConfiguredError(err) {
		// A present-but-invalid child/default BOM must never be hidden by the
		// legacy compatibility path.
		return productionapp.ProductionPlanItem{}, err
	}
	var hasLegacyBom bool
	if scanErr := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.product_bom pb
			WHERE pb.product_id=$1
			  AND EXISTS (
			    SELECT 1 FROM %s.product_bom_items item WHERE item.product_id=pb.product_id
			  )
		)
	`, schema, schema), group.ProductID).Scan(&hasLegacyBom); scanErr != nil {
		return productionapp.ProductionPlanItem{}, scanErr
	}
	if !hasLegacyBom {
		return productionapp.ProductionPlanItem{}, err
	}

	normalizedYield := normalizeYieldRate(legacyYieldRate)
	if !group.ManualInput {
		group.InputG = defaultProductionInputG(group.NeedG, normalizedYield)
	}
	plan := runningInventoryPlan(group.SpecG, group.NeedG, group.InputG, normalizedYield)
	run := ProduceRunRow{
		Product:             group.ProductName,
		ProductID:           group.ProductID,
		SpecG:               group.SpecG,
		NeedG:               group.NeedG,
		InputG:              group.InputG,
		BomYieldRate:        normalizedYield,
		PlanUnits:           plan.Units,
		PlanLooseG:          plan.LooseG,
		OrderNos:            group.OrderNos,
		OperationTemplateID: group.OperationTemplateID,
		Outputs:             group.Outputs,
	}
	materialSnapshot := []byte("[]")
	if group.SpecG > 0 {
		materialSnapshot, err = buildMaterialSnapshotForRunningItemTx(ctx, tx, schema, run)
		if err != nil {
			return productionapp.ProductionPlanItem{}, err
		}
	}
	productionConfigSnapshot, err := loadProductProductionConfigSnapshotForWorkOrderTx(ctx, tx, schema, group.ProductID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	customerProductSnapshot, err := loadCustomerProductSnapshotByOrderNosTx(ctx, tx, schema, group.OrderNos, group.ProductID, group.SpecG)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}

	var legacyItem productionapp.ProductionPlanItem
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,
			component_snapshot_json,process_route_snapshot_json,production_config_snapshot_json,
			customer_product_snapshot_json,created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,'{}'::jsonb,$10::jsonb,$11::jsonb,now())
		RETURNING id,production_plan_id,product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
		          sales_spec_count::float8,inventory_qty_per_sales_unit::float8,inventory_unit,planned_inventory_qty::float8,
		          COALESCE(sales_spec_snapshot_json,'{}'::jsonb)::text,bom_inherited,
		          planned_g,planned_output_g,gap_g,order_nos,bom_version_id,operation_template_id,process_route_id,
		          COALESCE(component_snapshot_json,'[]'::jsonb)::text,
		          COALESCE(process_route_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(production_config_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(customer_product_snapshot_json,'[]'::jsonb)::text
	`, schema),
		planID, group.ProductID, group.ProductName, group.SpecG, group.InputG, group.NeedG, group.NeedG,
		group.OrderNos, materialSnapshot, productionConfigSnapshot, customerProductSnapshot,
	).Scan(
		&legacyItem.ID, &legacyItem.PlanID, &legacyItem.ProductID, &legacyItem.ParentProductID,
		&legacyItem.BomSourceProductID, &legacyItem.ProductName, &legacyItem.SpecG,
		&legacyItem.SalesSpecCount, &legacyItem.InventoryQtyPerSalesUnit, &legacyItem.InventoryUnit,
		&legacyItem.PlannedInventoryQty, &legacyItem.SalesSpecSnapshotJSON, &legacyItem.BomInherited,
		&legacyItem.PlannedG, &legacyItem.PlannedOutputG, &legacyItem.GapG, &legacyItem.OrderNos,
		&legacyItem.BomVersionID, &legacyItem.OperationTemplateID, &legacyItem.ProcessRouteID,
		&legacyItem.MaterialSnapshot, &legacyItem.ProcessSnapshotJSON,
		&legacyItem.ProductionConfigSnapshotJSON, &legacyItem.CustomerProductSnapshotJSON,
	); err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	return legacyItem, nil
}

func createLegacyProductionPlanForStartGroupsTx(ctx context.Context, tx pgx.Tx, schema string, groups []startRunGroup, yieldByProductID map[int64]float64, operator string) (int64, map[string]int64, error) {
	if len(groups) == 0 {
		return 0, nil, nil
	}
	tmpNo := fmt.Sprintf("PP-LEGACY-TMP-%d", time.Now().UnixNano())
	var planID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plans(plan_no,source_type,status,created_by,submitted_by,submitted_at,created_at)
		VALUES($1,'legacy_produce_start','in_progress',$2,$2,now(),now())
		RETURNING id
	`, schema), tmpNo, operator).Scan(&planID); err != nil {
		return 0, nil, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET plan_no=$2 WHERE id=$1`, schema), planID, productionPlanNo(planID)); err != nil {
		return 0, nil, err
	}
	itemIDs := map[string]int64{}
	for _, group := range groups {
		if group.NeedG <= 0 && group.PlannedInventoryQty <= 0 {
			continue
		}
		item, err := createLegacyProductionPlanItemForGroupTx(ctx, tx, schema, planID, group, yieldByProductID[group.ProductID])
		if err != nil {
			return 0, nil, err
		}
		itemIDs[startRunGroupKey(group)] = item.ID
	}
	return planID, itemIDs, nil
}

func productionPlanTimeFieldColumn(field string) string {
	switch strings.TrimSpace(field) {
	case "submitted_at":
		return "pp.submitted_at"
	case "completed_at":
		return "pp.completed_at"
	default:
		return "pp.created_at"
	}
}

func (r Repository) ListProductionPlans(ctx context.Context, query productionapp.ProductionPlanQuery) ([]productionapp.ProductionPlanRow, error) {
	args := []any{}
	where := "1=1"
	if strings.TrimSpace(query.Status) != "" {
		args = append(args, strings.TrimSpace(query.Status))
		where += fmt.Sprintf(" AND pp.status=$%d", len(args))
	}
	timeColumn := productionPlanTimeFieldColumn(query.TimeField)
	if strings.TrimSpace(query.From) != "" {
		args = append(args, strings.TrimSpace(query.From))
		where += fmt.Sprintf(" AND %s >= NULLIF($%d,'')::date", timeColumn, len(args))
	}
	if strings.TrimSpace(query.To) != "" {
		args = append(args, strings.TrimSpace(query.To))
		where += fmt.Sprintf(" AND %s < (NULLIF($%d,'')::date + interval '1 day')", timeColumn, len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pp.id,pp.plan_no,pp.source_type,pp.status,COUNT(pi.id)::bigint,
		       pp.created_by,to_char(pp.created_at,'YYYY-MM-DD HH24:MI'),
		       pp.submitted_by,COALESCE(to_char(pp.submitted_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(pp.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(pp.cancelled_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.production_plans pp
		LEFT JOIN %s.production_plan_items pi ON pi.production_plan_id=pp.id
		WHERE %s
		GROUP BY pp.id
		ORDER BY pp.created_at DESC, pp.id DESC
		LIMIT $%d
	`, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionPlanRow, 0)
	for rows.Next() {
		var row productionapp.ProductionPlanRow
		if err := rows.Scan(
			&row.ID, &row.PlanNo, &row.SourceType, &row.Status, &row.ItemCount,
			&row.CreatedBy, &row.CreatedAt, &row.SubmittedBy, &row.SubmittedAt,
			&row.CompletedAt, &row.CancelledAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetProductionPlan(ctx context.Context, id int64) (productionapp.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	return detail, nil
}

func loadProductionPlanDetailTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.ProductionPlanDetail, error) {
	var detail productionapp.ProductionPlanDetail
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,plan_no,source_type,status,created_by,to_char(created_at,'YYYY-MM-DD HH24:MI'),
		       submitted_by,COALESCE(to_char(submitted_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(cancelled_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.production_plans
		WHERE id=$1
	`, schema), id).Scan(
		&detail.ID, &detail.PlanNo, &detail.SourceType, &detail.Status,
		&detail.CreatedBy, &detail.CreatedAt, &detail.SubmittedBy,
		&detail.SubmittedAt, &detail.CompletedAt, &detail.CancelledAt,
	)
	if err == pgx.ErrNoRows {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan not found")
	}
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	items, err := loadProductionPlanItemsTx(ctx, tx, schema, id)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	detail.Items = items
	splits, err := loadProductionPlanOperationSplitsTx(ctx, tx, schema, id)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	detail.OperationSplits = splits
	detail.MaterialSummary = aggregateProductionPlanMaterialSummary(items)
	relatedWorkOrders, jobCardCount, err := loadProductionPlanRelatedWorkOrdersTx(ctx, tx, schema, id)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	detail.RelatedWorkOrders = relatedWorkOrders
	detail.JobCardCount = jobCardCount
	return detail, nil
}

func loadProductionPlanItemsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanItem, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
		       sales_spec_count::float8,inventory_qty_per_sales_unit::float8,inventory_unit,planned_inventory_qty::float8,
		       COALESCE(sales_spec_snapshot_json,'{}'::jsonb)::text,bom_inherited,
		       planned_g,planned_output_g,gap_g,order_nos,
		       bom_version_id,operation_template_id,process_route_id,
		       COALESCE(component_snapshot_json,'[]'::jsonb)::text,
		       COALESCE(process_route_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(production_config_snapshot_json,'{}'::jsonb)::text,
		       COALESCE(customer_product_snapshot_json,'[]'::jsonb)::text
		FROM %s.production_plan_items
		WHERE production_plan_id=$1
		ORDER BY id
	`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionPlanItem, 0)
	for rows.Next() {
		var item productionapp.ProductionPlanItem
		if err := rows.Scan(
			&item.ID, &item.PlanID, &item.ProductID, &item.ParentProductID, &item.BomSourceProductID, &item.ProductName, &item.SpecG,
			&item.SalesSpecCount, &item.InventoryQtyPerSalesUnit, &item.InventoryUnit, &item.PlannedInventoryQty,
			&item.SalesSpecSnapshotJSON, &item.BomInherited,
			&item.PlannedG, &item.PlannedOutputG, &item.GapG, &item.OrderNos, &item.BomVersionID,
			&item.OperationTemplateID, &item.ProcessRouteID, &item.MaterialSnapshot, &item.ProcessSnapshotJSON,
			&item.ProductionConfigSnapshotJSON, &item.CustomerProductSnapshotJSON,
		); err != nil {
			return nil, err
		}
		if item.BomInherited {
			item.BomSource = "parent"
		} else {
			item.BomSource = "sku"
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func loadProductionPlanOperationSplitsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanOperationSplit, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,production_plan_item_id,operation_seq,operation_id,operation,
		       workstation_id,workstation,workstation_capacity_id,workstation_capacity_name,
		       COALESCE(batch_size_qty,0)::float8,COALESCE(batch_size_unit,''),standard_minutes,
		       COALESCE(hourly_rate,0)::float8,planned_batch_count,COALESCE(planned_qty,0)::float8,
		       planned_qty_g,planned_minutes,COALESCE(planned_operation_cost,0)::float8,note
		FROM %s.production_plan_operation_splits
		WHERE production_plan_id=$1
		ORDER BY production_plan_item_id, operation_seq, id
	`, schema), planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionPlanOperationSplit, 0)
	for rows.Next() {
		var row productionapp.ProductionPlanOperationSplit
		if err := rows.Scan(
			&row.ID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.OperationSeq, &row.OperationID, &row.Operation,
			&row.WorkstationID, &row.Workstation, &row.WorkstationCapacityID, &row.WorkstationCapacityName,
			&row.BatchSizeQty, &row.BatchSizeUnit, &row.StandardMinutes, &row.HourlyRate, &row.PlannedBatchCount, &row.PlannedQty,
			&row.PlannedQtyG, &row.PlannedMinutes, &row.PlannedOperationCost, &row.Note,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) SaveProductionPlanOperationSplits(ctx context.Context, cmd productionapp.SaveProductionPlanOperationSplitsCommand) ([]productionapp.ProductionPlanOperationSplit, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("production plan not found")
		}
		return nil, err
	}
	if status != "draft" {
		return nil, fmt.Errorf("production plan must be draft to edit operation splits")
	}
	itemRows, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return nil, err
	}
	itemIDs := map[int64]bool{}
	itemSpecs := map[int64]int64{}
	for _, item := range itemRows {
		itemIDs[item.ID] = true
		itemSpecs[item.ID] = item.SpecG
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_operation_splits WHERE production_plan_id=$1`, r.schema), cmd.ID); err != nil {
		return nil, err
	}
	for _, item := range cmd.Items {
		if !itemIDs[item.ProductionPlanItemID] {
			return nil, fmt.Errorf("production_plan_item_id does not belong to production plan")
		}
		item, err = prepareOperationSplitForSaveTx(ctx, tx, r.schema, item, cmd.ID, item.ProductionPlanItemID, itemSpecs[item.ProductionPlanItemID])
		if err != nil {
			return nil, err
		}
		if err := insertProductionPlanOperationSplitTx(ctx, tx, r.schema, item); err != nil {
			return nil, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.ID, "save_operation_splits", postgresinfra.StrPtr("operation_splits"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(cmd.Items))), postgresinfra.AuditMeta{"split_count": len(cmd.Items)}); err != nil {
		return nil, err
	}
	out, err := loadProductionPlanOperationSplitsTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (r Repository) PreviewProductionPlanOperationSplits(ctx context.Context, cmd productionapp.PreviewProductionPlanOperationSplitsCommand) (productionapp.ProductionPlanOperationSplitPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanOperationSplitPreview{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT 1 FROM %s.production_plans WHERE id=$1`, r.schema), cmd.ID).Scan(&exists); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.ProductionPlanOperationSplitPreview{}, fmt.Errorf("production plan not found")
		}
		return productionapp.ProductionPlanOperationSplitPreview{}, err
	}
	itemRows, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanOperationSplitPreview{}, err
	}
	itemIDs := map[int64]bool{}
	itemSpecs := map[int64]int64{}
	for _, item := range itemRows {
		itemIDs[item.ID] = true
		itemSpecs[item.ID] = item.SpecG
	}
	splits := make([]productionapp.ProductionPlanOperationSplit, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		if !itemIDs[item.ProductionPlanItemID] {
			return productionapp.ProductionPlanOperationSplitPreview{}, fmt.Errorf("production_plan_item_id does not belong to production plan")
		}
		prepared, err := prepareOperationSplitForSaveTx(ctx, tx, r.schema, item, cmd.ID, item.ProductionPlanItemID, itemSpecs[item.ProductionPlanItemID])
		if err != nil {
			return productionapp.ProductionPlanOperationSplitPreview{}, err
		}
		splits = append(splits, prepared)
	}
	return previewProductionPlanOperationSplits(itemRows, splits), nil
}

func (r Repository) SaveWorkOrderOperationSplits(ctx context.Context, cmd productionapp.SaveWorkOrderOperationSplitsCommand) (productionapp.WorkOrderOperationSplitsResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	wo, err := loadWorkOrderForOperationSplitTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if wo.Status != "released" || wo.RunningItemID > 0 {
		return productionapp.WorkOrderOperationSplitsResult{}, fmt.Errorf("work order must be released to edit operation splits")
	}
	var nonPendingCount int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.job_cards WHERE work_order_id=$1 AND status<>'pending'`, r.schema), wo.ID).Scan(&nonPendingCount); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if nonPendingCount > 0 {
		return productionapp.WorkOrderOperationSplitsResult{}, fmt.Errorf("work order job cards must be pending to edit operation splits")
	}

	splits := make([]productionapp.ProductionPlanOperationSplit, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		item, err = prepareOperationSplitForSaveTx(ctx, tx, r.schema, item, wo.ProductionPlanID, wo.ProductionPlanItemID, wo.SpecG)
		if err != nil {
			return productionapp.WorkOrderOperationSplitsResult{}, err
		}
		splits = append(splits, item)
	}
	if err := validateProductionPlanOperationSplitCoverage(productionapp.ProductionPlanItem{ID: wo.ProductionPlanItemID, ProductName: wo.ProductName, PlannedG: wo.PlannedG}, splits); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.job_cards WHERE work_order_id=$1`, r.schema), wo.ID); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if wo.ProductionPlanID > 0 && wo.ProductionPlanItemID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_operation_splits WHERE production_plan_id=$1 AND production_plan_item_id=$2`, r.schema), wo.ProductionPlanID, wo.ProductionPlanItemID); err != nil {
			return productionapp.WorkOrderOperationSplitsResult{}, err
		}
		for _, split := range splits {
			if err := insertProductionPlanOperationSplitTx(ctx, tx, r.schema, split); err != nil {
				return productionapp.WorkOrderOperationSplitsResult{}, err
			}
		}
	}
	cards, err := createPendingJobCardsForWorkOrderTx(ctx, tx, r.schema, wo.ID, wo.ProcessSnapshotJSON, wo.OperationTemplateID, wo.PlannedG, splits)
	if err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	summary := operationRowsJSON(cards)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, r.schema), wo.ID, summary); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &wo.ID, "save_operation_splits", postgresinfra.StrPtr("operation_splits"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", len(splits))), postgresinfra.AuditMeta{"split_count": len(splits)}); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderOperationSplitsResult{}, err
	}
	wo.OperationSummaryJSON = summary
	return productionapp.WorkOrderOperationSplitsResult{WorkOrder: wo, JobCards: cards}, nil
}

func loadWorkOrderForOperationSplitTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.WorkOrderRow, error) {
	var row productionapp.WorkOrderRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,
		       planned_g,planned_output_g,order_nos,status,bom_version_id,operation_template_id,process_template_id,process_template_name,
		       COALESCE(process_snapshot_json,'{}'::jsonb)::text
		FROM %s.work_orders
		WHERE id=$1
		FOR UPDATE
	`, schema), id).Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.OrderNos, &row.Status, &row.BomVersionID, &row.OperationTemplateID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON)
	if err == pgx.ErrNoRows {
		return productionapp.WorkOrderRow{}, fmt.Errorf("work order not found")
	}
	return row, err
}

func prepareOperationSplitForSaveTx(ctx context.Context, tx pgx.Tx, schema string, item productionapp.ProductionPlanOperationSplit, productionPlanID int64, productionPlanItemID int64, specG int64) (productionapp.ProductionPlanOperationSplit, error) {
	snapshot, err := loadWorkstationCapacitySnapshotForSplitTx(ctx, tx, schema, item.WorkstationCapacityID)
	if err != nil {
		return productionapp.ProductionPlanOperationSplit{}, err
	}
	item.ProductionPlanID = productionPlanID
	item.ProductionPlanItemID = productionPlanItemID
	item.WorkstationID = snapshot.WorkstationID
	item.Workstation = snapshot.Workstation
	item.WorkstationCapacityName = snapshot.WorkstationCapacityName
	item.BatchSizeQty = snapshot.BatchSizeQty
	item.BatchSizeUnit = snapshot.BatchSizeUnit
	item.StandardMinutes = snapshot.StandardMinutes
	item.HourlyRate = snapshot.HourlyRate
	return plannedCapacitySplitMetrics(item, specG), nil
}

func insertProductionPlanOperationSplitTx(ctx context.Context, tx pgx.Tx, schema string, item productionapp.ProductionPlanOperationSplit) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation_id,operation,
			workstation_id,workstation,workstation_capacity_id,workstation_capacity_name,
			batch_size_qty,batch_size_unit,standard_minutes,hourly_rate,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes,planned_operation_cost,note,created_at,updated_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,now(),now())
	`, schema), item.ProductionPlanID, item.ProductionPlanItemID, item.OperationSeq, item.OperationID, item.Operation,
		item.WorkstationID, item.Workstation, item.WorkstationCapacityID, item.WorkstationCapacityName,
		item.BatchSizeQty, item.BatchSizeUnit, item.StandardMinutes, item.HourlyRate, item.PlannedBatchCount,
		item.PlannedQty, item.PlannedQtyG, item.PlannedMinutes, item.PlannedOperationCost, item.Note)
	return err
}

type productionPlanCapacitySnapshot struct {
	WorkstationID           int64
	Workstation             string
	WorkstationCapacityName string
	BatchSizeQty            float64
	BatchSizeUnit           string
	StandardMinutes         int
	HourlyRate              float64
}

func loadWorkstationCapacitySnapshotForSplitTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionPlanCapacitySnapshot, error) {
	var row productionPlanCapacitySnapshot
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.workstation_id,COALESCE(w.name,''),c.name,
		       COALESCE(c.batch_size_qty,0)::float8,COALESCE(c.batch_size_unit,''),
		       c.standard_minutes,COALESCE(NULLIF(w.hourly_rate,0), c.hourly_rate, 0)::float8
		FROM %s.manufacturing_workstation_capacities c
		LEFT JOIN %s.manufacturing_workstations w ON w.id=c.workstation_id
		WHERE c.id=$1 AND c.status='active'
	`, schema, schema), id).Scan(&row.WorkstationID, &row.Workstation, &row.WorkstationCapacityName, &row.BatchSizeQty, &row.BatchSizeUnit, &row.StandardMinutes, &row.HourlyRate)
	if err == pgx.ErrNoRows {
		return productionPlanCapacitySnapshot{}, fmt.Errorf("workstation capacity not found or inactive")
	}
	return row, err
}

func plannedCapacitySplitMetrics(split productionapp.ProductionPlanOperationSplit, specG ...int64) productionapp.ProductionPlanOperationSplit {
	itemSpecG := int64(0)
	if len(specG) > 0 {
		itemSpecG = specG[0]
	}
	if split.PlannedQty <= 0 && split.PlannedBatchCount > 0 && split.BatchSizeQty > 0 {
		split.PlannedQty = split.BatchSizeQty * float64(split.PlannedBatchCount)
	}
	if split.PlannedQty > 0 {
		split.PlannedQty = roundProductionPlanQuantity(split.PlannedQty)
		split.PlannedQtyG = plannedCapacitySplitQtyG(split.PlannedQty, split.BatchSizeUnit, itemSpecG)
	}
	if split.PlannedQty > 0 && split.BatchSizeQty > 0 {
		split.PlannedBatchCount = int(math.Ceil(split.PlannedQty / split.BatchSizeQty))
	}
	if split.PlannedBatchCount > 0 && split.StandardMinutes > 0 {
		split.PlannedMinutes = split.PlannedBatchCount * split.StandardMinutes
	}
	if split.PlannedMinutes > 0 && split.HourlyRate > 0 {
		split.PlannedOperationCost = roundProductionPlanMoney(float64(split.PlannedMinutes) / 60 * split.HourlyRate)
	}
	return split
}

func plannedCapacitySplitQtyG(qty float64, unit string, specG ...int64) int64 {
	itemSpecG := int64(0)
	if len(specG) > 0 {
		itemSpecG = specG[0]
	}
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		return int64(math.Round(qty * 1000))
	case "g", "克":
		return int64(math.Round(qty))
	case "件", "个", "袋", "盒", "unit", "units", "pc", "pcs":
		if itemSpecG <= 0 {
			return 0
		}
		return int64(math.Round(qty * float64(itemSpecG)))
	default:
		return 0
	}
}

func roundProductionPlanMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundProductionPlanQuantity(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func productionPlanSplitsByItem(splits []productionapp.ProductionPlanOperationSplit) map[int64][]productionapp.ProductionPlanOperationSplit {
	out := map[int64][]productionapp.ProductionPlanOperationSplit{}
	for _, split := range splits {
		out[split.ProductionPlanItemID] = append(out[split.ProductionPlanItemID], split)
	}
	return out
}

func previewProductionPlanOperationSplits(items []productionapp.ProductionPlanItem, splits []productionapp.ProductionPlanOperationSplit) productionapp.ProductionPlanOperationSplitPreview {
	splitsByItem := productionPlanSplitsByItem(splits)
	coverageRows := make([]productionapp.ProductionPlanOperationSplitCoverageRow, 0)
	itemFactors := map[int64]float64{}
	var requiredG int64
	var arrangedG int64
	for _, item := range items {
		itemRequiredG := productionPlanItemTargetG(item)
		requiredG += itemRequiredG
		itemSplits := splitsByItem[item.ID]
		ops := productionPlanPreviewOperations(item, itemSplits)
		itemArrangedG := int64(0)
		if len(ops) > 0 {
			const maxInt64 = int64(^uint64(0) >> 1)
			itemArrangedG = maxInt64
			for _, op := range ops {
				opArrangedG := productionPlanPreviewOperationArrangedG(op, itemSplits)
				if opArrangedG < itemArrangedG {
					itemArrangedG = opArrangedG
				}
				coverageRows = append(coverageRows, productionapp.ProductionPlanOperationSplitCoverageRow{
					ProductionPlanItemID: item.ID,
					ProductName:          item.ProductName,
					OperationSeq:         op.Seq,
					OperationID:          op.OperationID,
					Operation:            strings.TrimSpace(op.Operation),
					RequiredG:            itemRequiredG,
					ArrangedG:            opArrangedG,
					DiffG:                opArrangedG - itemRequiredG,
					Status:               productionPlanPreviewStatus(itemRequiredG, opArrangedG),
				})
			}
			if itemArrangedG == maxInt64 {
				itemArrangedG = 0
			}
		} else {
			itemArrangedG = productionPlanPreviewAllSplitArrangedG(itemSplits)
		}
		arrangedG += itemArrangedG
		if itemRequiredG > 0 {
			itemFactors[item.ID] = float64(itemArrangedG) / float64(itemRequiredG)
		}
	}
	return productionapp.ProductionPlanOperationSplitPreview{
		CoverageSummary: productionapp.ProductionPlanOperationSplitCoverageSummary{
			RequiredG: requiredG,
			ArrangedG: arrangedG,
			DiffG:     arrangedG - requiredG,
			Status:    productionPlanPreviewStatus(requiredG, arrangedG),
		},
		OperationCoverage: coverageRows,
		MaterialSummary:   previewProductionPlanMaterialSummary(items, itemFactors),
		Warnings:          nil,
	}
}

func productionPlanItemTargetG(item productionapp.ProductionPlanItem) int64 {
	switch {
	case item.PlannedG > 0:
		return item.PlannedG
	case item.PlannedOutputG > 0:
		return item.PlannedOutputG
	case item.GapG > 0:
		return item.GapG
	default:
		return 0
	}
}

func productionPlanPreviewOperations(item productionapp.ProductionPlanItem, splits []productionapp.ProductionPlanOperationSplit) []processSnapshotOperation {
	ops := operationsFromProcessSnapshot(item.ProcessSnapshotJSON)
	if len(ops) > 0 {
		return ops
	}
	seen := map[string]bool{}
	out := make([]processSnapshotOperation, 0)
	for _, split := range splits {
		key := fmt.Sprintf("%d:%d:%s", split.OperationSeq, split.OperationID, strings.TrimSpace(split.Operation))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, processSnapshotOperation{Seq: split.OperationSeq, OperationID: split.OperationID, Operation: strings.TrimSpace(split.Operation)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seq == out[j].Seq {
			return out[i].Operation < out[j].Operation
		}
		return out[i].Seq < out[j].Seq
	})
	return out
}

func productionPlanPreviewOperationArrangedG(op processSnapshotOperation, splits []productionapp.ProductionPlanOperationSplit) int64 {
	var total int64
	for _, split := range operationSplitsForSnapshotOperation(op, splits) {
		if split.PlannedQtyG > 0 {
			total += split.PlannedQtyG
		}
	}
	return total
}

func productionPlanPreviewAllSplitArrangedG(splits []productionapp.ProductionPlanOperationSplit) int64 {
	var total int64
	for _, split := range splits {
		if split.PlannedQtyG > 0 {
			total += split.PlannedQtyG
		}
	}
	return total
}

func productionPlanPreviewStatus(required, arranged int64) string {
	switch {
	case required <= 0 && arranged <= 0:
		return "missing"
	case arranged <= 0:
		return "missing"
	case arranged < required:
		return "short"
	case arranged > required:
		return "over"
	default:
		return "matched"
	}
}

func previewProductionPlanMaterialSummary(items []productionapp.ProductionPlanItem, itemFactors map[int64]float64) []productionapp.ProductionPlanOperationSplitMaterialPreview {
	required := aggregateProductionPlanMaterialSummary(items)
	scaledItems := make([]productionapp.ProductionPlanItem, 0, len(items))
	for _, item := range items {
		factor := itemFactors[item.ID]
		scaled := item
		scaled.PlannedG = int64(math.Round(float64(item.PlannedG) * factor))
		scaled.PlannedOutputG = int64(math.Round(float64(item.PlannedOutputG) * factor))
		scaled.GapG = int64(math.Round(float64(item.GapG) * factor))
		scaledItems = append(scaledItems, scaled)
	}
	arranged := aggregateProductionPlanMaterialSummary(scaledItems)
	type key struct {
		name string
		unit string
	}
	rows := map[key]productionapp.ProductionPlanOperationSplitMaterialPreview{}
	for _, item := range required {
		k := key{name: item.Name, unit: item.Unit}
		row := rows[k]
		row.Name = item.Name
		row.Unit = item.Unit
		row.RequiredQty = materialNeedDisplayQty(item)
		rows[k] = row
	}
	for _, item := range arranged {
		k := key{name: item.Name, unit: item.Unit}
		row := rows[k]
		row.Name = item.Name
		row.Unit = item.Unit
		row.ArrangedQty = materialNeedDisplayQty(item)
		rows[k] = row
	}
	out := make([]productionapp.ProductionPlanOperationSplitMaterialPreview, 0, len(rows))
	for _, row := range rows {
		row.DiffQty = row.ArrangedQty - row.RequiredQty
		row.Status = productionPlanMaterialPreviewStatus(row.RequiredQty, row.ArrangedQty)
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Unit < out[j].Unit
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func aggregateProductionPlanMaterialSummary(items []productionapp.ProductionPlanItem) []productionapp.MaterialNeed {
	type key struct {
		name              string
		unit              string
		componentType     string
		upstreamProductID int64
		upstreamShortageG int64
	}
	merged := map[key]productionapp.MaterialNeed{}
	for _, item := range items {
		raw := strings.TrimSpace(item.MaterialSnapshot)
		if raw == "" || raw == "[]" || raw == "null" {
			continue
		}
		var rows []materialSnapshotRow
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			continue
		}
		for _, row := range rows {
			name := strings.TrimSpace(row.MaterialName)
			if name == "" {
				continue
			}
			unit := strings.TrimSpace(row.Unit)
			if unit == "" {
				unit = "g"
			}
			qty := productionPlanMaterialSnapshotQty(item, row, unit)
			if qty <= 0 {
				continue
			}
			componentType := strings.TrimSpace(row.ComponentType)
			if componentType == "" {
				componentType = strings.TrimSpace(row.Source)
			}
			k := key{
				name:              name,
				unit:              unit,
				componentType:     componentType,
				upstreamProductID: row.ComponentProductID,
			}
			current := merged[k]
			if current.Name == "" {
				current = productionapp.MaterialNeed{
					Name:              name,
					Unit:              unit,
					ComponentType:     componentType,
					UpstreamProductID: row.ComponentProductID,
				}
			}
			current.ExactQty += qty
			current.Qty = int64(math.Ceil(current.ExactQty))
			merged[k] = current
		}
	}
	out := make([]productionapp.MaterialNeed, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Unit < out[j].Unit
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func materialNeedDisplayQty(item productionapp.MaterialNeed) float64 {
	if item.ExactQty > 0 {
		return item.ExactQty
	}
	return float64(item.Qty)
}

func productionPlanMaterialSnapshotQty(item productionapp.ProductionPlanItem, row materialSnapshotRow, unit string) float64 {
	source := strings.TrimSpace(row.Source)
	if source == "" {
		source = "bom"
	}
	rawG := item.PlannedG
	if rawG <= 0 {
		rawG = item.GapG
	}
	outputG := item.PlannedOutputG
	if outputG <= 0 {
		outputG = item.GapG
	}
	packedUnits := productionPlanOutputUnits(item)
	if source == "packaging" {
		if row.QtyPerUnit > 0 && packedUnits > 0 {
			return math.Ceil(float64(packedUnits) * row.QtyPerUnit)
		}
		return float64(packedUnits)
	}
	ratioPct := row.RatioPct
	if normalizeBomConsumeUnit(row.ConsumeUnit) == "ratio_pct" && !isWeightMaterialUnit(unit) && ratioPct <= 0 {
		ratioPct = 100
	}
	materialLossRate := row.MaterialLossRate
	if row.InputIncludesMaterialLoss {
		materialLossRate = 0
	}
	if isWeightMaterialUnit(unit) {
		grams := componentConsumptionWeightGramsWithMaterialLoss(
			row.ConsumeUnit, row.QtyPerUnit, ratioPct, unit, rawG, outputG,
			packedUnits, 0, row.OutputQty, row.OutputUnit, materialLossRate,
		)
		return float64(grams) / productionWeightUnitGrams(unit)
	}
	return float64(componentConsumptionQtyWithMaterialLoss(row.ConsumeUnit, row.QtyPerUnit, ratioPct, unit, rawG, outputG, packedUnits, 0, row.OutputQty, row.OutputUnit, materialLossRate))
}

func productionPlanMaterialPreviewStatus(required, arranged float64) string {
	switch {
	case required <= 0 && arranged <= 0:
		return "missing"
	case arranged <= 0:
		return "missing"
	case arranged+0.000000001 < required:
		return "short"
	case arranged > required+0.000000001:
		return "over"
	default:
		return "matched"
	}
}

func productionPlanOutputUnits(item productionapp.ProductionPlanItem) int64 {
	if item.SpecG > 0 && item.PlannedOutputG > 0 {
		return ceilDiv64(item.PlannedOutputG, item.SpecG)
	}
	if item.PlannedOutputG > 0 {
		return item.PlannedOutputG
	}
	if item.GapG > 0 && item.SpecG > 0 {
		return ceilDiv64(item.GapG, item.SpecG)
	}
	return 0
}

func loadProductionPlanRelatedWorkOrdersTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanRelatedWorkOrder, int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.production_plan_id,wo.production_plan_item_id,wo.product_name,wo.spec_g,
		       wo.planned_g,COALESCE(NULLIF(wo.planned_output_g,0),wo.planned_g),wo.status,
		       to_char(wo.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COUNT(jc.id)::bigint
		FROM %s.work_orders wo
		LEFT JOIN %s.job_cards jc ON jc.work_order_id=wo.id
		WHERE wo.production_plan_id=$1
		GROUP BY wo.id
		ORDER BY wo.id
	`, schema, schema), planID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionPlanRelatedWorkOrder, 0)
	var totalJobCards int64
	for rows.Next() {
		var row productionapp.ProductionPlanRelatedWorkOrder
		if err := rows.Scan(
			&row.ID, &row.WorkOrderNo, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.ProductName, &row.SpecG,
			&row.PlannedG, &row.PlannedOutputG, &row.Status, &row.CreatedAt, &row.CompletedAt, &row.JobCardCount,
		); err != nil {
			return nil, 0, err
		}
		totalJobCards += row.JobCardCount
		out = append(out, row)
	}
	return out, totalJobCards, rows.Err()
}

func (r Repository) SubmitProductionPlan(ctx context.Context, cmd productionapp.SubmitProductionPlanCommand) (productionapp.ProductionPlanSubmitResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_plans WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.ProductionPlanSubmitResult{}, fmt.Errorf("production plan not found")
		}
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	if status != "draft" {
		return productionapp.ProductionPlanSubmitResult{}, fmt.Errorf("production plan must be draft to submit")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET status='submitted', submitted_by=$2, submitted_at=now() WHERE id=$1`, r.schema), cmd.ID, cmd.Operator); err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	items, err := loadProductionPlanItemsTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	if len(items) == 0 {
		return productionapp.ProductionPlanSubmitResult{}, fmt.Errorf("production plan has no items")
	}
	planSplits, err := loadProductionPlanOperationSplitsTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	splitsByItem := productionPlanSplitsByItem(planSplits)
	workOrders := make([]productionapp.WorkOrderRow, 0, len(items))
	jobCards := make([]productionapp.JobCardRow, 0)
	for _, item := range items {
		splits := splitsByItem[item.ID]
		if err := validateProductionPlanOperationSplitCoverage(item, splits); err != nil {
			return productionapp.ProductionPlanSubmitResult{}, err
		}
		wo, cards, err := createReleasedWorkOrderForPlanItemTx(ctx, tx, r.schema, item, cmd.Operator, splits)
		if err != nil {
			return productionapp.ProductionPlanSubmitResult{}, err
		}
		workOrders = append(workOrders, wo)
		jobCards = append(jobCards, cards...)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan", &cmd.ID, "submit", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("submitted"), postgresinfra.AuditMeta{"work_order_count": len(workOrders)}); err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	plan, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	return productionapp.ProductionPlanSubmitResult{Plan: plan, WorkOrders: workOrders, JobCards: jobCards}, nil
}

func (r Repository) CancelProductionPlan(ctx context.Context, cmd productionapp.CancelProductionPlanCommand) (productionapp.ProductionPlanDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var planNo, status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT plan_no,status
		FROM %s.production_plans
		WHERE id=$1
		FOR UPDATE
	`, r.schema), cmd.ID).Scan(&planNo, &status); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan not found")
		}
		return productionapp.ProductionPlanDetail{}, err
	}
	if status == "cancelled" {
		detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ID)
		if err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return productionapp.ProductionPlanDetail{}, err
		}
		return detail, nil
	}
	if status != "draft" {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan must be draft to cancel")
	}

	var workOrderCount, itemCount int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(
				SELECT COUNT(*)::bigint
				FROM %s.work_orders wo
				WHERE wo.production_plan_id=$1
				   OR EXISTS (
						SELECT 1
						FROM %s.production_plan_items pi
						WHERE pi.id=wo.production_plan_item_id
						  AND pi.production_plan_id=$1
				   )
			),
			(SELECT COUNT(*)::bigint FROM %s.production_plan_items WHERE production_plan_id=$1)
	`, r.schema, r.schema, r.schema), cmd.ID).Scan(&workOrderCount, &itemCount); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if workOrderCount > 0 {
		return productionapp.ProductionPlanDetail{}, fmt.Errorf("production plan with work order cannot be cancelled as draft")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_plans
		SET status='cancelled',cancelled_at=now()
		WHERE id=$1
	`, r.schema), cmd.ID); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(
		ctx,
		tx,
		r.schema,
		cmd.Operator,
		"production_plan",
		&cmd.ID,
		"cancel",
		postgresinfra.StrPtr("status"),
		postgresinfra.StrPtr("draft"),
		postgresinfra.StrPtr("cancelled"),
		postgresinfra.AuditMeta{
			"plan_no":    planNo,
			"item_count": itemCount,
			"note":       cmd.Note,
		},
	); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	detail, err := loadProductionPlanDetailTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	return detail, nil
}

func validateProductionPlanOperationSplitCoverage(item productionapp.ProductionPlanItem, splits []productionapp.ProductionPlanOperationSplit) error {
	if len(splits) == 0 || item.PlannedG <= 0 {
		return nil
	}
	type opKey struct {
		seq int
		id  int64
		op  string
	}
	coverage := map[opKey]int64{}
	for _, split := range splits {
		if split.PlannedQtyG <= 0 {
			continue
		}
		key := opKey{seq: split.OperationSeq, id: split.OperationID, op: strings.TrimSpace(split.Operation)}
		coverage[key] += split.PlannedQtyG
	}
	for key, plannedG := range coverage {
		if plannedG < item.PlannedG {
			label := key.op
			if label == "" {
				label = fmt.Sprintf("operation_seq %d", key.seq)
			}
			return fmt.Errorf("operation capacity split for %s must cover planned_g %d", label, item.PlannedG)
		}
	}
	return nil
}

func createReleasedWorkOrderForPlanItemTx(ctx context.Context, tx pgx.Tx, schema string, item productionapp.ProductionPlanItem, operator string, splits []productionapp.ProductionPlanOperationSplit) (productionapp.WorkOrderRow, []productionapp.JobCardRow, error) {
	processTemplateID, processTemplateName := processTemplateFieldsFromSnapshot(item.ProcessSnapshotJSON)
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(
			work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,
			product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
			sales_spec_count,inventory_qty_per_sales_unit,inventory_unit,planned_inventory_qty,sales_spec_snapshot_json,bom_inherited,
			planned_g,planned_output_g,order_nos,status,material_snapshot,bom_version_id,operation_template_id,
			process_template_id,process_template_name,process_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,created_at
		)
		VALUES($1,0,$2,$3,'',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,
		       $15,$16,$17,'released',$18::jsonb,$19,$20,$21,$22,$23::jsonb,$24::jsonb,$25::jsonb,now())
		RETURNING id
	`, schema),
		releasedWorkOrderNo(item.PlanID, item.ID), item.PlanID, item.ID,
		item.ProductID, item.ParentProductID, item.BomSourceProductID, item.ProductName, item.SpecG,
		item.SalesSpecCount, item.InventoryQtyPerSalesUnit, item.InventoryUnit, item.PlannedInventoryQty,
		defaultJSONObject(item.SalesSpecSnapshotJSON), item.BomInherited,
		item.PlannedG, item.PlannedOutputG, item.OrderNos, defaultJSONArray(item.MaterialSnapshot),
		item.BomVersionID, item.OperationTemplateID, processTemplateID, processTemplateName,
		defaultJSONObject(item.ProcessSnapshotJSON), defaultJSONObject(item.ProductionConfigSnapshotJSON),
		defaultJSONArray(item.CustomerProductSnapshotJSON),
	).Scan(&id); err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	cards, err := createPendingJobCardsForWorkOrderTx(ctx, tx, schema, id, item.ProcessSnapshotJSON, item.OperationTemplateID, item.PlannedG, splits)
	if err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	wo := productionapp.WorkOrderRow{
		ID:                       id,
		WorkOrderNo:              releasedWorkOrderNo(item.PlanID, item.ID),
		RunningItemID:            0,
		ProductionPlanID:         item.PlanID,
		ProductionPlanItemID:     item.ID,
		ProductID:                item.ProductID,
		ParentProductID:          item.ParentProductID,
		BomSourceProductID:       item.BomSourceProductID,
		BomSource:                item.BomSource,
		BomInherited:             item.BomInherited,
		ProductName:              item.ProductName,
		SpecG:                    item.SpecG,
		SalesSpecCount:           item.SalesSpecCount,
		InventoryQtyPerSalesUnit: item.InventoryQtyPerSalesUnit,
		InventoryUnit:            item.InventoryUnit,
		PlannedInventoryQty:      item.PlannedInventoryQty,
		SalesSpecSnapshotJSON:    item.SalesSpecSnapshotJSON,
		PlannedG:                 item.PlannedG,
		PlannedOutputG:           item.PlannedOutputG,
		Status:                   "released",
		OrderNos:                 item.OrderNos,
		BomVersionID:             item.BomVersionID,
		OperationTemplateID:      item.OperationTemplateID,
		ProcessTemplateID:        processTemplateID,
		ProcessTemplateName:      processTemplateName,
		ProcessSnapshotJSON:      defaultJSONObject(item.ProcessSnapshotJSON),
		MaterialSummary:          formatMaterialSnapshotSummary(item.MaterialSnapshot),
		OperationSummaryJSON:     operationRowsJSON(cards),
		ExpectedYieldRate:        0,
		ExpectedLossRate:         0,
		SuggestedInputG:          item.PlannedG,
		WIPRemainingReservedG:    0,
	}
	return wo, cards, nil
}

func createPendingJobCardsForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, processSnapshotJSON string, operationTemplateID int64, plannedG int64, splits []productionapp.ProductionPlanOperationSplit) ([]productionapp.JobCardRow, error) {
	ops := operationsFromProcessSnapshot(processSnapshotJSON)
	if len(ops) == 0 {
		steps, err := loadOperationTemplateStepsTx(ctx, tx, schema, operationTemplateID)
		if err != nil {
			return nil, err
		}
		if len(steps) == 0 {
			steps = defaultOperationTemplateSteps()
		}
		for _, step := range steps {
			ops = append(ops, processSnapshotOperation{Seq: step.Position, Operation: step.Operation, Workstation: step.Workstation, RecordsLoss: true, ParameterSchemaJSON: "{}", DefaultMinutes: 0})
		}
	}
	out := make([]productionapp.JobCardRow, 0, len(ops))
	for _, op := range ops {
		matchedSplits := operationSplitsForSnapshotOperation(op, splits)
		if len(matchedSplits) > 0 {
			for _, split := range matchedSplits {
				card, err := insertPendingJobCardForOperationSplitTx(ctx, tx, schema, workOrderID, op, split, plannedG)
				if err != nil {
					return nil, err
				}
				out = append(out, card)
			}
			continue
		}
		var id int64
		metrics := plannedJobCardMetrics(op, plannedG)
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.job_cards(
				work_order_id,sequence_no,operation_id,workstation_id,operation,workstation,
				workstation_capacity_id,workstation_capacity_name,batch_size_qty,batch_size_unit,
				planned_batch_count,planned_minutes,hourly_rate,planned_operation_cost,
				status,started_at,operator,planned_input_qty,records_loss,parameter_schema_json
			)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'pending',now(),'',$15,$16,$17::jsonb)
			RETURNING id
		`, schema), workOrderID, op.Seq, op.OperationID, op.WorkstationID, op.Operation, op.Workstation, op.WorkstationCapacityID, op.WorkstationCapacityName, op.BatchSizeQty, op.BatchSizeUnit, metrics.PlannedBatchCount, metrics.PlannedMinutes, op.HourlyRate, metrics.PlannedOperationCost, plannedG, op.RecordsLoss, defaultJSONObject(op.ParameterSchemaJSON)).Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, productionapp.JobCardRow{
			ID:                      id,
			WorkOrderID:             workOrderID,
			SequenceNo:              op.Seq,
			OperationID:             op.OperationID,
			WorkstationID:           op.WorkstationID,
			Operation:               op.Operation,
			Workstation:             op.Workstation,
			WorkstationCapacityID:   op.WorkstationCapacityID,
			WorkstationCapacityName: op.WorkstationCapacityName,
			BatchSizeQty:            op.BatchSizeQty,
			BatchSizeUnit:           op.BatchSizeUnit,
			PlannedBatchCount:       metrics.PlannedBatchCount,
			PlannedMinutes:          metrics.PlannedMinutes,
			HourlyRate:              op.HourlyRate,
			PlannedOperationCost:    metrics.PlannedOperationCost,
			Status:                  "pending",
			PlannedInputQty:         float64(plannedG),
			RecordsLoss:             op.RecordsLoss,
			ParameterSchemaJSON:     defaultJSONObject(op.ParameterSchemaJSON),
		})
	}
	return out, nil
}

func operationSplitsForSnapshotOperation(op processSnapshotOperation, splits []productionapp.ProductionPlanOperationSplit) []productionapp.ProductionPlanOperationSplit {
	out := make([]productionapp.ProductionPlanOperationSplit, 0)
	for _, split := range splits {
		switch {
		case split.OperationSeq > 0 && split.OperationSeq == op.Seq:
			out = append(out, split)
		case split.OperationID > 0 && split.OperationID == op.OperationID:
			out = append(out, split)
		case split.OperationSeq == 0 && strings.TrimSpace(split.Operation) != "" && strings.TrimSpace(split.Operation) == strings.TrimSpace(op.Operation):
			out = append(out, split)
		}
	}
	return out
}

func insertPendingJobCardForOperationSplitTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, op processSnapshotOperation, split productionapp.ProductionPlanOperationSplit, plannedG int64) (productionapp.JobCardRow, error) {
	var id int64
	plannedInputQty := float64(plannedG)
	if split.PlannedQtyG > 0 {
		plannedInputQty = float64(split.PlannedQtyG)
	}
	operation := firstNonEmpty(strings.TrimSpace(split.Operation), op.Operation)
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.job_cards(
			work_order_id,sequence_no,operation_id,workstation_id,operation,workstation,
			workstation_capacity_id,workstation_capacity_name,batch_size_qty,batch_size_unit,
			planned_batch_count,planned_minutes,hourly_rate,planned_operation_cost,
			status,started_at,operator,planned_input_qty,records_loss,parameter_schema_json
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'pending',now(),'',$15,$16,$17::jsonb)
		RETURNING id
	`, schema), workOrderID, op.Seq, firstPositiveInt64(split.OperationID, op.OperationID), split.WorkstationID, operation, split.Workstation, split.WorkstationCapacityID, split.WorkstationCapacityName, split.BatchSizeQty, split.BatchSizeUnit, split.PlannedBatchCount, split.PlannedMinutes, split.HourlyRate, split.PlannedOperationCost, plannedInputQty, op.RecordsLoss, defaultJSONObject(op.ParameterSchemaJSON)).Scan(&id); err != nil {
		return productionapp.JobCardRow{}, err
	}
	return productionapp.JobCardRow{
		ID:                      id,
		WorkOrderID:             workOrderID,
		SequenceNo:              op.Seq,
		OperationID:             firstPositiveInt64(split.OperationID, op.OperationID),
		WorkstationID:           split.WorkstationID,
		Operation:               operation,
		Workstation:             split.Workstation,
		WorkstationCapacityID:   split.WorkstationCapacityID,
		WorkstationCapacityName: split.WorkstationCapacityName,
		BatchSizeQty:            split.BatchSizeQty,
		BatchSizeUnit:           split.BatchSizeUnit,
		PlannedBatchCount:       split.PlannedBatchCount,
		PlannedMinutes:          split.PlannedMinutes,
		HourlyRate:              split.HourlyRate,
		PlannedOperationCost:    split.PlannedOperationCost,
		Status:                  "pending",
		PlannedInputQty:         plannedInputQty,
		RecordsLoss:             op.RecordsLoss,
		ParameterSchemaJSON:     defaultJSONObject(op.ParameterSchemaJSON),
	}, nil
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (r Repository) StartWorkOrder(ctx context.Context, cmd productionapp.WorkOrderStartCommand) (productionapp.WorkOrderStartResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	wo, materialSnapshot, err := loadReleasedWorkOrderForStartTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if wo.Status != "released" || wo.RunningItemID > 0 {
		return productionapp.WorkOrderStartResult{}, fmt.Errorf("work order already started")
	}
	refs := splitOrderNos(wo.OrderNos)
	if err := lockStartRefsTx(ctx, tx, r.schema, refs); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if err := ensureStartRefsNotRunningTx(ctx, tx, r.schema, refs); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}

	batchID := newBatchID()
	yieldRate := frozenProductionPlanYieldRate(wo.ProcessSnapshotJSON)
	if yieldRate <= 0 {
		yieldRate, err = loadFrozenBomVersionYieldRateTx(ctx, tx, r.schema, wo.BomVersionID)
		if err != nil {
			return productionapp.WorkOrderStartResult{}, err
		}
	}
	yieldRate = normalizeYieldRate(yieldRate)
	plannedOutputG := wo.PlannedOutputG
	if plannedOutputG <= 0 {
		plannedOutputG = wo.PlannedG
	}
	plan := runningInventoryPlan(wo.SpecG, plannedOutputG, wo.PlannedG, yieldRate)
	run := ProduceRunRow{
		BatchID:             batchID,
		Product:             wo.ProductName,
		ProductID:           wo.ProductID,
		SpecG:               wo.SpecG,
		NeedG:               plannedOutputG,
		InputG:              wo.PlannedG,
		BomYieldRate:        yieldRate,
		PlanUnits:           plan.Units,
		PlanLooseG:          plan.LooseG,
		OrderNos:            wo.OrderNos,
		OperationTemplateID: wo.OperationTemplateID,
		MaterialSnapshot:    string(materialSnapshot),
	}
	needs, err := runningItemWorkOrderMaterialNeedsTx(ctx, tx, r.schema, wo.ID, run, materialSnapshot)
	if err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if err := ensureWIPStockForWorkOrderNeedsTx(ctx, tx, r.schema, wo.ID, needs); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}

	var runningItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(batch_id,product_id,product_name,spec_g,need_g,order_nos,status,started_by,started_at,input_g,bom_yield_rate,planned_units,planned_loose_g,material_snapshot,operation_template_id)
		VALUES($1,$2,$3,$4,$5,$6,'running',$7,now(),$8,$9,$10,$11,$12,$13)
		RETURNING id
	`, r.schema), batchID, wo.ProductID, wo.ProductName, wo.SpecG, plannedOutputG, wo.OrderNos, cmd.Operator, wo.PlannedG, yieldRate, plan.Units, plan.LooseG, materialSnapshot, wo.OperationTemplateID).Scan(&runningItemID); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET running_item_id=$2,batch_id=$3,status='running'
		WHERE id=$1 AND status='released' AND running_item_id=0
	`, r.schema), wo.ID, runningItemID, batchID); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status='running', started_at=now(), operator=$2 WHERE work_order_id=$1 AND status='pending'`, r.schema), wo.ID, cmd.Operator); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if err := createMaterialReservationsForRunningItemTx(ctx, tx, r.schema, wo.ID, runningItemID, needs); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if err := markProcessingDemandsRunningTx(ctx, tx, r.schema, wo.OrderNos, batchID, runningItemID, wo.ID); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if err := setOrdersProcessStatusByNeedsTx(ctx, tx, r.schema, []UnprodNeedRow{{ProductID: wo.ProductID, Product: wo.ProductName, SpecG: wo.SpecG, OrderNos: wo.OrderNos}}, "生产中"); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if wo.ProductionPlanID > 0 {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET status='in_progress' WHERE id=$1 AND status='submitted'`, r.schema), wo.ProductionPlanID); err != nil {
			return productionapp.WorkOrderStartResult{}, err
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &wo.ID, "start", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("released"), postgresinfra.StrPtr("running"), postgresinfra.AuditMeta{"running_item_id": runningItemID, "batch_id": batchID}); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	wo.RunningItemID = runningItemID
	wo.BatchID = batchID
	wo.Status = "running"
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	return productionapp.WorkOrderStartResult{BatchID: batchID, RunningItemID: runningItemID, WorkOrder: wo}, nil
}

func loadReleasedWorkOrderForStartTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.WorkOrderRow, []byte, error) {
	var row productionapp.WorkOrderRow
	var materialSnapshot string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,
		       product_id,parent_product_id,bom_source_product_id,product_name,spec_g,
		       sales_spec_count::float8,inventory_qty_per_sales_unit::float8,inventory_unit,planned_inventory_qty::float8,
		       COALESCE(sales_spec_snapshot_json,'{}'::jsonb)::text,bom_inherited,
		       planned_g,planned_output_g,order_nos,status,bom_version_id,operation_template_id,process_template_id,process_template_name,
		       COALESCE(process_snapshot_json,'{}'::jsonb)::text,COALESCE(material_snapshot,'[]'::jsonb)::text
		FROM %s.work_orders
		WHERE id=$1
		FOR UPDATE
	`, schema), id).Scan(
		&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID,
		&row.ProductID, &row.ParentProductID, &row.BomSourceProductID, &row.ProductName, &row.SpecG,
		&row.SalesSpecCount, &row.InventoryQtyPerSalesUnit, &row.InventoryUnit, &row.PlannedInventoryQty,
		&row.SalesSpecSnapshotJSON, &row.BomInherited,
		&row.PlannedG, &row.PlannedOutputG, &row.OrderNos, &row.Status, &row.BomVersionID,
		&row.OperationTemplateID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON, &materialSnapshot,
	)
	if err == pgx.ErrNoRows {
		return productionapp.WorkOrderRow{}, nil, fmt.Errorf("work order not found")
	}
	if err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	return row, []byte(defaultJSONArray(materialSnapshot)), nil
}

func loadFrozenBomVersionYieldRateTx(ctx context.Context, tx pgx.Tx, schema string, bomVersionID int64) (float64, error) {
	if bomVersionID <= 0 {
		return 0, fmt.Errorf("work order has no frozen production BOM version")
	}
	var yieldRate float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(yield_rate,0),1)::float8
		FROM %s.production_bom_versions
		WHERE id=$1
	`, schema), bomVersionID).Scan(&yieldRate)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("frozen production BOM version not found")
	}
	return yieldRate, err
}

func frozenProductionPlanYieldRate(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return 0
	}
	var snapshot struct {
		YieldRate float64 `json:"yield_rate"`
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return 0
	}
	return snapshot.YieldRate
}

func loadCustomerProductSnapshotByOrderNosTx(ctx context.Context, tx pgx.Tx, schema string, orderNos string, productID int64, _ int64) ([]byte, error) {
	refs := splitOrderNos(orderNos)
	if len(refs) == 0 {
		return []byte("[]"), nil
	}
	var raw string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH lines AS (
			SELECT DISTINCT
			       o.order_no,
			       COALESCE(o.customer_id,0) AS customer_id,
			       COALESCE(oi.customer_product_alias_id,0) AS customer_product_alias_id,
			       COALESCE(oi.customer_product_display_name_snapshot,'') AS customer_product_display_name_snapshot,
			       COALESCE(oi.customer_item_code_snapshot,'') AS customer_item_code_snapshot,
			       COALESCE(oi.product_code_snapshot,'') AS product_code_snapshot,
			       COALESCE(oi.product_name_snapshot,'') AS product_name_snapshot
			FROM %[1]s.order_items oi
			JOIN %[1]s.orders o ON o.id=oi.order_id
			WHERE o.order_no = ANY($1)
			  AND COALESCE(oi.product_id,0)=$2
		)
		SELECT COALESCE(jsonb_agg(jsonb_build_object(
			'order_no', order_no,
			'customer_id', customer_id,
			'customer_product_alias_id', customer_product_alias_id,
			'customer_product_display_name_snapshot', customer_product_display_name_snapshot,
			'customer_item_code_snapshot', customer_item_code_snapshot,
			'product_code_snapshot', product_code_snapshot,
			'product_name_snapshot', product_name_snapshot
		)), '[]'::jsonb)::text
		FROM lines
	`, schema), refs, productID).Scan(&raw)
	if err != nil {
		if strings.Contains(err.Error(), "customer_product_alias_id") || strings.Contains(err.Error(), "customer_product_display_name_snapshot") {
			return []byte("[]"), nil
		}
		return nil, err
	}
	return []byte(raw), nil
}

func operationsFromProcessSnapshot(raw string) []processSnapshotOperation {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil
	}
	var snapshot processTemplateSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil
	}
	return snapshot.Operations
}

func processTemplateFieldsFromSnapshot(raw string) (int64, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return 0, ""
	}
	var snapshot processTemplateSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return 0, ""
	}
	name := firstNonEmpty(snapshot.Name, snapshot.RouteName)
	return snapshot.ID, name
}

func defaultJSONArray(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return "[]"
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return "[]"
	}
	return raw
}

func operationRowsJSON(cards []productionapp.JobCardRow) string {
	rows := make([]map[string]any, 0, len(cards))
	for _, card := range cards {
		rows = append(rows, map[string]any{
			"id":                        card.ID,
			"sequence_no":               card.SequenceNo,
			"operation_id":              card.OperationID,
			"workstation_id":            card.WorkstationID,
			"operation":                 card.Operation,
			"workstation":               card.Workstation,
			"workstation_capacity_id":   card.WorkstationCapacityID,
			"workstation_capacity_name": card.WorkstationCapacityName,
			"batch_size_qty":            card.BatchSizeQty,
			"batch_size_unit":           card.BatchSizeUnit,
			"planned_batch_count":       card.PlannedBatchCount,
			"planned_minutes":           card.PlannedMinutes,
			"hourly_rate":               card.HourlyRate,
			"planned_operation_cost":    card.PlannedOperationCost,
			"actual_minutes":            card.ActualMinutes,
			"actual_operation_cost":     card.ActualOperationCost,
			"status":                    card.Status,
			"records_loss":              card.RecordsLoss,
			"planned_input_qty":         card.PlannedInputQty,
		})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(b)
}
