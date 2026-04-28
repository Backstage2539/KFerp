package production

import (
	"context"
	"encoding/json"
	"fmt"
	catalogdomain "orderapp/internal/domain/catalog"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5"
)

func workOrderNo(runningItemID int64) string {
	return fmt.Sprintf("WO-%010d", runningItemID)
}

func createWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, batchID string, productID int64, productName string, specG int64, plannedG int64, materialSnapshot []byte, operator string) (int64, error) {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,material_snapshot,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,'running',$8,now())
		ON CONFLICT (running_item_id) DO UPDATE SET status='running', material_snapshot=excluded.material_snapshot
		RETURNING id
	`, schema), workOrderNo(runningItemID), runningItemID, batchID, productID, productName, specG, plannedG, materialSnapshot).Scan(&workOrderID); err != nil {
		return 0, err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.job_cards(work_order_id,operation,workstation,status,started_at,operator)
		VALUES($1,'roast','roaster','running',now(),$2)
	`, schema), workOrderID, operator)
	return workOrderID, err
}

func completeWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, actualCost float64, operator string) error {
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
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed', completed_at=now(), operator=COALESCE(NULLIF(operator,''),$2)
		WHERE work_order_id=$1 AND status <> 'completed'
	`, schema), workOrderID, operator)
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
	operationCost := 0.0
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
		           SELECT string_agg(COALESCE(m.name,'') || ' ' || COALESCE(NULLIF(trim(trailing '.' from trim(trailing '0' from COALESCE(bi.ratio_pct,0)::text)), ''), '0') || '%%', '、' ORDER BY bi.id)
		           FROM %s.product_bom_items bi
		           LEFT JOIN %s.materials m ON m.id=bi.material_id
		           WHERE bi.product_id=wo.product_id
		       ), '')
		FROM %s.work_orders wo
		LEFT JOIN %s.produce_running_items ri ON ri.id=wo.running_item_id
		LEFT JOIN %s.products p ON p.id=wo.product_id
		LEFT JOIN %s.product_bom pb ON pb.product_id=wo.product_id
		WHERE %s
		ORDER BY wo.created_at DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
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
			&row.RoastLevel, &row.YieldRate, &row.SuggestedInputG, &row.PlannedUnits, &row.PlannedLooseG, &row.OrderNos, &snapshotText, &fallbackMaterialSummary,
		); err != nil {
			return nil, err
		}
		row.MaterialSummary = formatMaterialSnapshotSummary(snapshotText)
		if row.MaterialSummary == "" {
			row.MaterialSummary = fallbackMaterialSummary
		}
		row.YieldRate = catalogdomain.ResolveYieldRate(row.RoastLevel, row.YieldRate)
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
		       to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator
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
		if err := rows.Scan(&row.ID, &row.WorkOrderID, &row.Operation, &row.Workstation, &row.Status, &row.StartedAt, &row.CompletedAt, &row.Operator); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
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
