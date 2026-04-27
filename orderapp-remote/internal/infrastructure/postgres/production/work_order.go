package production

import (
	"context"
	"fmt"
	"strings"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5"
)

func workOrderNo(runningItemID int64) string {
	return fmt.Sprintf("WO-%010d", runningItemID)
}

func createWorkOrderForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, batchID string, productID int64, productName string, specG int64, plannedG int64, operator string) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,'running',now())
		ON CONFLICT (running_item_id) DO UPDATE SET status='running'
		RETURNING id
	`, schema), workOrderNo(runningItemID), runningItemID, batchID, productID, productName, specG, plannedG).Scan(&workOrderID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.job_cards(work_order_id,operation,workstation,status,started_at,operator)
		VALUES($1,'roast','roaster','running',now(),$2)
	`, schema), workOrderID, operator)
	return err
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
	return err
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
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,batch_id,product_id,product_name,spec_g,planned_g,status,
		       COALESCE(actual_cost,0),to_char(created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.work_orders
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		if err := rows.Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
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
