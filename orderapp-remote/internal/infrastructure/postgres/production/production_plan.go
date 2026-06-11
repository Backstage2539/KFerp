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

	yieldByProductID, err := loadProductYieldRateMapTx(ctx, tx, r.schema)
	if err != nil {
		return productionapp.ProductionPlanDetail{}, err
	}
	groups := groupStartNeedsForRuns(needs, cmd.InputByKey, yieldByProductID)
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
		if group.NeedG <= 0 {
			continue
		}
		if _, err := createProductionPlanItemForGroupTx(ctx, tx, r.schema, planID, group, yieldByProductID[group.ProductID]); err != nil {
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
	_ = tx
	rows, err := fetchUnproducedNeeds(ctx, r.pool, r.schema, cmd.From, cmd.To, cmd.CustomerID)
	if err != nil {
		return nil, err
	}
	out := make([]productionapp.StartNeed, 0)
	for _, row := range startNeedsToApp(rows) {
		key := producePlanKey(row.ProductID, row.SpecG)
		if !cmd.Selected[key] || row.GapG <= 0 {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func createProductionPlanItemForGroupTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, group startRunGroup, yieldRate float64) (productionapp.ProductionPlanItem, error) {
	normalizedYield := normalizeYieldRate(yieldRate)
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
	var err error
	if group.SpecG > 0 {
		materialSnapshot, err = buildMaterialSnapshotForRunningItemTx(ctx, tx, schema, run)
		if err != nil {
			return productionapp.ProductionPlanItem{}, err
		}
	}
	processSnapshot, processSnapshotJSON, err := loadProcessRouteSnapshotForWorkOrderTx(ctx, tx, schema, group.ProductID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	if processSnapshot == nil {
		processSnapshot, processSnapshotJSON, err = loadActiveProcessTemplateSnapshotTx(ctx, tx, schema, group.ProductID)
		if err != nil {
			return productionapp.ProductionPlanItem{}, err
		}
	}
	if len(processSnapshotJSON) == 0 {
		processSnapshotJSON = []byte("{}")
	}
	processRouteID := int64(0)
	if processSnapshot != nil {
		processRouteID = processSnapshot.RouteID
		if processRouteID == 0 && processSnapshot.Source == "process_route" {
			processRouteID = processSnapshot.ID
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
	bomVersionID, err := loadBoundBomVersionIDForProductTx(ctx, tx, schema, group.ProductID)
	if err != nil {
		return productionapp.ProductionPlanItem{}, err
	}

	var item productionapp.ProductionPlanItem
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,
			bom_version_id,operation_template_id,process_route_id,component_snapshot_json,process_route_snapshot_json,
			production_config_snapshot_json,customer_product_snapshot_json,created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14::jsonb,$15::jsonb,now())
		RETURNING id,production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,
		          bom_version_id,operation_template_id,process_route_id,
		          COALESCE(component_snapshot_json,'[]'::jsonb)::text,
		          COALESCE(process_route_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(production_config_snapshot_json,'{}'::jsonb)::text,
		          COALESCE(customer_product_snapshot_json,'[]'::jsonb)::text
	`, schema), planID, group.ProductID, group.ProductName, group.SpecG, group.InputG, group.NeedG, group.NeedG, group.OrderNos, bomVersionID, group.OperationTemplateID, processRouteID, materialSnapshot, processSnapshotJSON, productionConfigSnapshot, customerProductSnapshot).Scan(
		&item.ID, &item.PlanID, &item.ProductID, &item.ProductName, &item.SpecG, &item.PlannedG, &item.PlannedOutputG, &item.GapG, &item.OrderNos,
		&item.BomVersionID, &item.OperationTemplateID, &item.ProcessRouteID, &item.MaterialSnapshot, &item.ProcessSnapshotJSON, &item.ProductionConfigSnapshotJSON, &item.CustomerProductSnapshotJSON,
	); err != nil {
		return productionapp.ProductionPlanItem{}, err
	}
	return item, nil
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
		if group.NeedG <= 0 {
			continue
		}
		item, err := createProductionPlanItemForGroupTx(ctx, tx, schema, planID, group, yieldByProductID[group.ProductID])
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
		       COALESCE(to_char(pp.completed_at,'YYYY-MM-DD HH24:MI'),'')
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
		if err := rows.Scan(&row.ID, &row.PlanNo, &row.SourceType, &row.Status, &row.ItemCount, &row.CreatedBy, &row.CreatedAt, &row.SubmittedBy, &row.SubmittedAt, &row.CompletedAt); err != nil {
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
		       COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.production_plans
		WHERE id=$1
	`, schema), id).Scan(&detail.ID, &detail.PlanNo, &detail.SourceType, &detail.Status, &detail.CreatedBy, &detail.CreatedAt, &detail.SubmittedBy, &detail.SubmittedAt, &detail.CompletedAt)
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
	return detail, nil
}

func loadProductionPlanItemsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanItem, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,product_id,product_name,spec_g,planned_g,planned_output_g,gap_g,order_nos,
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
		if err := rows.Scan(&item.ID, &item.PlanID, &item.ProductID, &item.ProductName, &item.SpecG, &item.PlannedG, &item.PlannedOutputG, &item.GapG, &item.OrderNos, &item.BomVersionID, &item.OperationTemplateID, &item.ProcessRouteID, &item.MaterialSnapshot, &item.ProcessSnapshotJSON, &item.ProductionConfigSnapshotJSON, &item.CustomerProductSnapshotJSON); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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
	workOrders := make([]productionapp.WorkOrderRow, 0, len(items))
	jobCards := make([]productionapp.JobCardRow, 0)
	for _, item := range items {
		wo, cards, err := createReleasedWorkOrderForPlanItemTx(ctx, tx, r.schema, item, cmd.Operator)
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

func createReleasedWorkOrderForPlanItemTx(ctx context.Context, tx pgx.Tx, schema string, item productionapp.ProductionPlanItem, operator string) (productionapp.WorkOrderRow, []productionapp.JobCardRow, error) {
	processTemplateID, processTemplateName := processTemplateFieldsFromSnapshot(item.ProcessSnapshotJSON)
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(
			work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,
			planned_g,planned_output_g,order_nos,status,material_snapshot,bom_version_id,operation_template_id,
			process_template_id,process_template_name,process_snapshot_json,production_config_snapshot_json,customer_product_snapshot_json,created_at
		)
		VALUES($1,0,$2,$3,'',$4,$5,$6,$7,$8,$9,'released',$10::jsonb,$11,$12,$13,$14,$15::jsonb,$16::jsonb,$17::jsonb,now())
		RETURNING id
	`, schema), releasedWorkOrderNo(item.PlanID, item.ID), item.PlanID, item.ID, item.ProductID, item.ProductName, item.SpecG, item.PlannedG, item.PlannedOutputG, item.OrderNos, defaultJSONArray(item.MaterialSnapshot), item.BomVersionID, item.OperationTemplateID, processTemplateID, processTemplateName, defaultJSONObject(item.ProcessSnapshotJSON), defaultJSONObject(item.ProductionConfigSnapshotJSON), defaultJSONArray(item.CustomerProductSnapshotJSON)).Scan(&id); err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	cards, err := createPendingJobCardsForWorkOrderTx(ctx, tx, schema, id, item.ProcessSnapshotJSON, item.OperationTemplateID, item.PlannedG)
	if err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	wo := productionapp.WorkOrderRow{
		ID:                    id,
		WorkOrderNo:           releasedWorkOrderNo(item.PlanID, item.ID),
		RunningItemID:         0,
		ProductionPlanID:      item.PlanID,
		ProductionPlanItemID:  item.ID,
		ProductID:             item.ProductID,
		ProductName:           item.ProductName,
		SpecG:                 item.SpecG,
		PlannedG:              item.PlannedG,
		PlannedOutputG:        item.PlannedOutputG,
		Status:                "released",
		OrderNos:              item.OrderNos,
		BomVersionID:          item.BomVersionID,
		OperationTemplateID:   item.OperationTemplateID,
		ProcessTemplateID:     processTemplateID,
		ProcessTemplateName:   processTemplateName,
		ProcessSnapshotJSON:   defaultJSONObject(item.ProcessSnapshotJSON),
		MaterialSummary:       formatMaterialSnapshotSummary(item.MaterialSnapshot),
		OperationSummaryJSON:  operationRowsJSON(cards),
		ExpectedYieldRate:     0,
		ExpectedLossRate:      0,
		SuggestedInputG:       item.PlannedG,
		WIPRemainingReservedG: 0,
	}
	return wo, cards, nil
}

func createPendingJobCardsForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, processSnapshotJSON string, operationTemplateID int64, plannedG int64) ([]productionapp.JobCardRow, error) {
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
		var id int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.job_cards(
				work_order_id,sequence_no,operation,workstation,status,started_at,operator,
				planned_input_qty,records_loss,parameter_schema_json
			)
			VALUES($1,$2,$3,$4,'pending',now(),'',$5,$6,$7::jsonb)
			RETURNING id
		`, schema), workOrderID, op.Seq, op.Operation, op.Workstation, plannedG, op.RecordsLoss, defaultJSONObject(op.ParameterSchemaJSON)).Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, productionapp.JobCardRow{
			ID:                  id,
			WorkOrderID:         workOrderID,
			SequenceNo:          op.Seq,
			Operation:           op.Operation,
			Workstation:         op.Workstation,
			Status:              "pending",
			PlannedInputQty:     float64(plannedG),
			RecordsLoss:         op.RecordsLoss,
			ParameterSchemaJSON: defaultJSONObject(op.ParameterSchemaJSON),
		})
	}
	return out, nil
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
	yieldByProductID, err := loadProductYieldRateMapTx(ctx, tx, r.schema)
	if err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	yieldRate := normalizeYieldRate(yieldByProductID[wo.ProductID])
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
	if err := ensureWIPStockForRunningItemTx(ctx, tx, r.schema, run, materialSnapshot); err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
	if err != nil {
		return productionapp.WorkOrderStartResult{}, err
	}
	if !ok {
		needs, err = currentMaterialNeedsTx(ctx, tx, r.schema, run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
		if err != nil {
			return productionapp.WorkOrderStartResult{}, err
		}
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
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,
		       planned_g,planned_output_g,order_nos,status,bom_version_id,operation_template_id,process_template_id,process_template_name,
		       COALESCE(process_snapshot_json,'{}'::jsonb)::text,COALESCE(material_snapshot,'[]'::jsonb)::text
		FROM %s.work_orders
		WHERE id=$1
		FOR UPDATE
	`, schema), id).Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.OrderNos, &row.Status, &row.BomVersionID, &row.OperationTemplateID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON, &materialSnapshot)
	if err == pgx.ErrNoRows {
		return productionapp.WorkOrderRow{}, nil, fmt.Errorf("work order not found")
	}
	if err != nil {
		return productionapp.WorkOrderRow{}, nil, err
	}
	return row, []byte(defaultJSONArray(materialSnapshot)), nil
}

func loadCustomerProductSnapshotByOrderNosTx(ctx context.Context, tx pgx.Tx, schema string, orderNos string, productID int64, specG int64) ([]byte, error) {
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
			CROSS JOIN LATERAL (
				SELECT COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g
			) spec
			WHERE o.order_no = ANY($1)
			  AND COALESCE(oi.product_id,0)=$2
			  AND ($3::bigint=0 OR spec.spec_g=$3)
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
	`, schema), refs, productID, specG).Scan(&raw)
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
			"id":                card.ID,
			"sequence_no":       card.SequenceNo,
			"operation":         card.Operation,
			"workstation":       card.Workstation,
			"status":            card.Status,
			"records_loss":      card.RecordsLoss,
			"planned_input_qty": card.PlannedInputQty,
		})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(b)
}
