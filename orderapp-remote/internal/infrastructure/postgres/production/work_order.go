package production

import (
	"context"
	"encoding/json"
	"fmt"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5"
)

func workOrderNo(runningItemID int64) string {
	return fmt.Sprintf("WO-%010d", runningItemID)
}

type operationTemplateStepRow struct {
	ID          int64
	Operation   string
	Workstation string
	CostType    string
	CostRate    float64
}

func loadOperationTemplateStepsTx(ctx context.Context, tx pgx.Tx, schema string, operationTemplateID int64) ([]operationTemplateStepRow, error) {
	if operationTemplateID <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id, COALESCE(NULLIF(operation,''),'production'), COALESCE(workstation,''), COALESCE(cost_type,''), COALESCE(cost_rate,0)::float8
		FROM %s.operation_template_steps
		WHERE template_id=$1 AND active=true
		ORDER BY position,id
	`, schema), operationTemplateID)
	if err != nil {
		if strings.Contains(err.Error(), "operation_template_steps") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := make([]operationTemplateStepRow, 0)
	for rows.Next() {
		var row operationTemplateStepRow
		if err := rows.Scan(&row.ID, &row.Operation, &row.Workstation, &row.CostType, &row.CostRate); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func defaultOperationTemplateSteps() []operationTemplateStepRow {
	return []operationTemplateStepRow{{Operation: "production", Workstation: "workstation"}}
}

func createWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, batchID string, productID int64, productName string, specG int64, plannedG int64, materialSnapshot []byte, operationTemplateID int64, operator string) (int64, error) {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,material_snapshot,operation_template_id,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,'running',$8,$9,now())
		ON CONFLICT (running_item_id) DO UPDATE SET status='running', material_snapshot=excluded.material_snapshot, operation_template_id=excluded.operation_template_id
		RETURNING id
	`, schema), workOrderNo(runningItemID), runningItemID, batchID, productID, productName, specG, plannedG, materialSnapshot, operationTemplateID).Scan(&workOrderID); err != nil {
		return 0, err
	}
	steps, err := loadOperationTemplateStepsTx(ctx, tx, schema, operationTemplateID)
	if err != nil {
		return 0, err
	}
	if len(steps) == 0 {
		steps = defaultOperationTemplateSteps()
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.job_cards WHERE work_order_id=$1`, schema), workOrderID); err != nil {
		return 0, err
	}
	for _, step := range steps {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.job_cards(work_order_id,operation,workstation,status,started_at,operator,planned_input_qty,operation_template_step_id,cost_type,cost_rate)
			VALUES($1,$2,$3,'running',now(),$4,$5,$6,$7,$8)
		`, schema), workOrderID, step.Operation, step.Workstation, operator, plannedG, step.ID, step.CostType, step.CostRate); err != nil {
			return 0, err
		}
	}
	return workOrderID, nil
}

func completeWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, actualCost float64, actualInputQty int64, actualOutputQty int64, operator string) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='completed', actual_cost=$2, completed_at=now()
		WHERE running_item_id=$1
		RETURNING id
	`, schema), runningItemID, actualCost).Scan(&workOrderID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	actualLossQty := 0.0
	actualLossRate := 0.0
	if actualInputQty > 0 {
		lossQty, lossRate, err := productiondomain.ActualLossMetrics(float64(actualInputQty), float64(actualOutputQty))
		if err != nil {
			return err
		}
		actualLossQty = lossQty
		actualLossRate = lossRate
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',
		    completed_at=now(),
		    operator=COALESCE(NULLIF(operator,''),$2),
		    actual_input_qty=$3,
		    actual_output_qty=$4,
		    actual_loss_qty=$5,
		    actual_loss_rate=$6
		WHERE work_order_id=$1 AND status <> 'completed'
	`, schema), workOrderID, operator, actualInputQty, actualOutputQty, actualLossQty, actualLossRate)
	return err
}

func cancelWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, operator string) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET status='cancelled', completed_at=now()
		WHERE running_item_id=$1
		RETURNING id
	`, schema), runningItemID).Scan(&workOrderID); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='cancelled', completed_at=now(), operator=COALESCE(NULLIF(operator,''),$2)
		WHERE work_order_id=$1 AND status <> 'completed'
	`, schema), workOrderID, operator)
	if err != nil {
		return err
	}
	return releaseMaterialReservationsForRunningItemTx(ctx, tx, schema, runningItemID)
}

func recordBatchCostForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, finishedTotalG int64) (float64, error) {
	var materialCost float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(
			CASE
				WHEN l.deduct_g > 0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(NULLIF(mb.unit_cost,0), NULLIF(m.purchase_price,0), 0)
				WHEN l.deduct_units > 0 THEN l.deduct_units::numeric * COALESCE(NULLIF(m.purchase_price,0), 0)
				ELSE 0
			END
		),0)
		FROM %s.material_consumption_logs l
		LEFT JOIN %s.material_batches mb ON mb.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		WHERE l.running_item_id=$1
	`, schema, schema, schema), r.ID).Scan(&materialCost)
	if err != nil && strings.Contains(err.Error(), "material_batches") {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(
				CASE
					WHEN l.deduct_g > 0 THEN (l.deduct_g::numeric / 1000.0) * COALESCE(NULLIF(m.purchase_price,0), 0)
					WHEN l.deduct_units > 0 THEN l.deduct_units::numeric * COALESCE(NULLIF(m.purchase_price,0), 0)
					ELSE 0
				END
			),0)
			FROM %s.material_consumption_logs l
			LEFT JOIN %s.materials m ON m.id=l.material_id
			WHERE l.running_item_id=$1
		`, schema, schema), r.ID).Scan(&materialCost)
	}
	if err != nil {
		return 0, err
	}
	var operationCost float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(CASE
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('per_kg_input','per_input_kg')
				THEN COALESCE(jc.cost_rate,0) * COALESCE(NULLIF(ri.input_g,0), $2)::numeric / 1000.0
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('per_kg_output','per_finished_kg','per_kg')
				THEN COALESCE(jc.cost_rate,0) * $2::numeric / 1000.0
			WHEN COALESCE(NULLIF(jc.cost_type,''),'fixed') IN ('fixed','per_unit','per_quote_unit')
				THEN COALESCE(jc.cost_rate,0)
			ELSE 0
		END),0)::float8
		FROM %s.work_orders wo
		JOIN %s.job_cards jc ON jc.work_order_id=wo.id
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		WHERE wo.running_item_id=$1
	`, schema, schema, schema), r.ID, finishedTotalG).Scan(&operationCost); err != nil {
		return 0, err
	}
	totalCost := materialCost + operationCost
	unitCost := 0.0
	if finishedTotalG > 0 {
		unitCost = totalCost / (float64(finishedTotalG) / 1000.0)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_batch_costs(running_item_id,batch_id,product_name,material_cost,operation_cost,total_cost,finished_g,unit_cost_per_kg,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
		ON CONFLICT (running_item_id) DO UPDATE SET
			material_cost=excluded.material_cost,
			operation_cost=excluded.operation_cost,
			total_cost=excluded.total_cost,
			finished_g=excluded.finished_g,
			unit_cost_per_kg=excluded.unit_cost_per_kg,
			created_at=now()
	`, schema), r.ID, r.BatchID, r.Product, materialCost, operationCost, totalCost, finishedTotalG, unitCost)
	return totalCost, err
}

func (r Repository) ListWorkOrders(ctx context.Context, query productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	args := []any{}
	where := "1=1"
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND wo.status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	machines, err := r.ListMachines(ctx, true)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.running_item_id,wo.batch_id,wo.product_id,wo.product_name,wo.spec_g,wo.planned_g,wo.status,
		       COALESCE(wo.actual_cost,0),to_char(wo.created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(wo.completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(p.roast_level,''),
		       COALESCE(NULLIF(ri.bom_yield_rate,0), pb.yield_rate, 0),
		       COALESCE(NULLIF(ri.input_g,0), wo.planned_g, 0),
		       COALESCE(ri.planned_units,0),
		       COALESCE(ri.planned_loose_g,0),
		       COALESCE(ri.order_nos,''),
		       COALESCE(wo.material_snapshot,'[]'::jsonb)::text,
		       COALESCE((
		           SELECT SUM(r.reserved_g)::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id
		       ),0),
		       COALESCE((
		           SELECT SUM(r.consumed_g)::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id
		       ),0),
		       COALESCE((
		           SELECT SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g))::bigint
		           FROM %s.work_order_material_reservations r
		           WHERE r.running_item_id=wo.running_item_id AND r.status='reserved'
		       ),0),
		       COALESCE((
		           SELECT string_agg(COALESCE(m.name,'') || ' ' || COALESCE(NULLIF(trim(trailing '.' from trim(trailing '0' from COALESCE(bi.ratio_pct,0)::text)), ''), '0') || '%%', '、' ORDER BY bi.id)
		           FROM %s.product_bom_items bi
		           LEFT JOIN %s.materials m ON m.id=bi.material_id
		           WHERE bi.product_id=wo.product_id
		       ), ''),
		       COALESCE((
		           SELECT jsonb_build_object(
		               'planned_input_qty', COALESCE(SUM(jc.planned_input_qty),0),
		               'actual_input_qty', COALESCE(SUM(jc.actual_input_qty),0),
		               'actual_output_qty', COALESCE(SUM(jc.actual_output_qty),0),
		               'actual_loss_qty', COALESCE(SUM(jc.actual_loss_qty),0),
		               'actual_loss_rate', CASE WHEN COALESCE(SUM(jc.actual_input_qty),0) > 0 THEN ROUND((COALESCE(SUM(jc.actual_loss_qty),0) / NULLIF(SUM(jc.actual_input_qty),0))::numeric, 4) ELSE 0 END,
		               'completed_cards', COALESCE(COUNT(*) FILTER (WHERE jc.status='completed'),0),
		               'total_cards', COALESCE(COUNT(*),0)
		           )::text
		           FROM %s.job_cards jc
		           WHERE jc.work_order_id=wo.id
		       ), '{}')
		FROM %s.work_orders wo
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		LEFT JOIN %s.products p ON p.id=wo.product_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=wo.product_id
		WHERE %s
		ORDER BY wo.created_at DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		var snapshotText, fallbackMaterialSummary string
		if err := rows.Scan(
			&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt,
			&row.RoastLevel, &row.YieldRate, &row.SuggestedInputG, &row.PlannedUnits, &row.PlannedLooseG, &row.OrderNos, &snapshotText, &row.WIPReservedG, &row.WIPConsumedG, &row.WIPRemainingReservedG, &fallbackMaterialSummary, &row.OperationSummaryJSON,
		); err != nil {
			return nil, err
		}
		row.MaterialSummary = formatMaterialSnapshotSummary(snapshotText)
		if row.MaterialSummary == "" {
			row.MaterialSummary = fallbackMaterialSummary
		}
		row.ExpectedYieldRate = productiondomain.NormalizeYieldRate(row.YieldRate)
		row.ExpectedLossRate = productiondomain.ExpectedLossRate(row.ExpectedYieldRate)
		row.YieldRate = row.ExpectedYieldRate
		if row.SuggestedInputG <= 0 {
			row.SuggestedInputG = row.PlannedG
		}
		machine, batches := pickMachineAndBatches(row.SuggestedInputG, machines)
		row.SuggestedMachine = machine.Name
		row.SuggestedBatchCount = int64(len(batches))
		if row.SuggestedBatchCount == 0 && row.SuggestedInputG > 0 {
			row.SuggestedBatchCount = 1
		}
		if len(batches) > 0 {
			row.SuggestedBatchG = batches[0]
		} else {
			row.SuggestedBatchG = row.SuggestedInputG
		}
		row.SuggestedBatchPlan = formatWorkOrderBatchPlan(batches, row.SuggestedInputG)
		out = append(out, row)
	}
	return out, rows.Err()
}

func formatMaterialSnapshotSummary(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return ""
	}
	var rows []materialSnapshotRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return ""
	}
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.MaterialName)
		if name == "" {
			continue
		}
		if row.Source == "packaging" {
			parts = append(parts, name+" 包材")
			continue
		}
		ratio := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", row.RatioPct), "0"), ".")
		if ratio == "" || ratio == "0" {
			parts = append(parts, name)
		} else {
			parts = append(parts, name+" "+ratio+"%")
		}
	}
	return strings.Join(parts, "、")
}

func formatWorkOrderBatchPlan(batches []int64, fallbackG int64) string {
	if len(batches) == 0 {
		if fallbackG <= 0 {
			return "0"
		}
		return formatKg(fallbackG) + "kg"
	}
	parts := make([]string, 0, len(batches))
	used := make([]bool, len(batches))
	for i, batch := range batches {
		if used[i] {
			continue
		}
		count := 1
		for j := i + 1; j < len(batches); j++ {
			if batches[j] == batch {
				used[j] = true
				count++
			}
		}
		part := formatKg(batch) + "kg"
		if count > 1 {
			part += fmt.Sprintf(" x %d", count)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " + ")
}

func (r Repository) ListJobCards(ctx context.Context, query productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	args := []any{}
	where := "1=1"
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,operation,workstation,status,
		       to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator,
		       COALESCE(planned_input_qty,0)::float8,
		       COALESCE(actual_input_qty,0)::float8,
		       COALESCE(actual_output_qty,0)::float8,
		       COALESCE(actual_loss_qty,0)::float8,
		       COALESCE(actual_loss_rate,0)::float8,
		       COALESCE(exception_reason,''),
		       COALESCE(metrics_json,'{}'::jsonb)::text
		FROM %s.job_cards
		WHERE %s
		ORDER BY started_at DESC, id DESC
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.JobCardRow, 0)
	for rows.Next() {
		var row productionapp.JobCardRow
		if err := rows.Scan(&row.ID, &row.WorkOrderID, &row.Operation, &row.Workstation, &row.Status, &row.StartedAt, &row.CompletedAt, &row.Operator, &row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate, &row.ExceptionReason, &row.MetricsJSON); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) UpdateJobCardActuals(ctx context.Context, cmd productionapp.JobCardActualsCommand) error {
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET planned_input_qty=$2,
		    actual_input_qty=$3,
		    actual_output_qty=$4,
		    actual_loss_qty=$5,
		    actual_loss_rate=$6,
		    exception_reason=$7,
		    metrics_json=$8::jsonb
		WHERE id=$1
	`, r.schema), cmd.ID, cmd.PlannedInputQty, cmd.ActualInputQty, cmd.ActualOutputQty, cmd.ActualLossQty, cmd.ActualLossRate, cmd.ExceptionReason, cmd.MetricsJSON)
	if err == nil && tag.RowsAffected() == 0 {
		return fmt.Errorf("job card not found")
	}
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "job_card", &cmd.ID, "update_actuals", postgresinfra.StrPtr("actual_loss_rate"), nil, postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.ActualLossRate)), postgresinfra.AuditMeta{"job_card_id": cmd.ID, "planned_input_qty": cmd.PlannedInputQty, "actual_input_qty": cmd.ActualInputQty, "actual_output_qty": cmd.ActualOutputQty, "actual_loss_qty": cmd.ActualLossQty, "actual_loss_rate": cmd.ActualLossRate, "exception_reason": cmd.ExceptionReason})
	}
	return err
}

func (r Repository) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,batch_id,product_name,COALESCE(material_cost,0),COALESCE(operation_cost,0),
		       COALESCE(total_cost,0),finished_g,COALESCE(unit_cost_per_kg,0),to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_batch_costs
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, r.schema), query.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.BatchCostRow, 0)
	for rows.Next() {
		var row productionapp.BatchCostRow
		if err := rows.Scan(&row.ID, &row.RunningItemID, &row.BatchID, &row.ProductName, &row.MaterialCost, &row.OperationCost, &row.TotalCost, &row.FinishedG, &row.UnitCostPerKG, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
