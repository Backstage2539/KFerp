package production

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) executionBatchListTx(ctx context.Context, tx pgx.Tx, id int64) (app.ExecutionBatchList, error) {
	out := app.ExecutionBatchList{Batches: []app.ExecutionBatch{}}
	p, _, err := r.operationCompletionContextTx(ctx, tx, id, false)
	if err != nil {
		return out, err
	}
	out.Version = p.Version
	var count, history int
	var size float64
	var mode, status string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(planned_batch_count,0),COALESCE(NULLIF(custom_batch_size_qty,0),batch_size_qty,0)::float8,COALESCE(scheduling_mode,''),status,((SELECT count(*) FROM %[1]s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted') + CASE WHEN COALESCE(actual_input_qty,0)>0 OR COALESCE(actual_output_qty,0)>0 THEN 1 ELSE 0 END) FROM %[1]s.job_cards WHERE id=$1`, r.schema), id).Scan(&count, &size, &mode, &status, &history)
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,batch_no,planned_input_qty::float8,COALESCE(NULLIF(input_unit,''),$2),actual_input_qty::float8,actual_output_qty::float8,status FROM %s.production_execution_batches WHERE job_card_id=$1 ORDER BY batch_no`, r.schema), id, p.InputUnit)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var b app.ExecutionBatch
		if err = rows.Scan(&b.ID, &b.BatchNo, &b.PlannedInputQty, &b.InputUnit, &b.ActualInputQty, &b.ActualOutputQty, &b.Status); err != nil {
			rows.Close()
			return out, err
		}
		out.Batches = append(out.Batches, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(out.Batches) > 0 {
		return out, nil
	}
	if history > 0 {
		out.Reason = "已有历史汇总报工，请继续使用原报工方式"
		return out, nil
	}
	if status == "completed" || status == "cancelled" {
		out.Reason = "任务已结束"
		return out, nil
	}
	if p.MaterialConsumptionPolicy != "bom_start_v1" || p.Execution.ReportingType == "piece" || p.Execution.ReportingType == "incoming" || app.OperationUnitFactor(p.InputUnit) <= 0 || mode != "batch" || count < 1 || size <= 0 {
		out.Reason = "当前任务不支持分批报工"
		return out, nil
	}
	out.CanInitialize = true
	return out, nil
}
func (r Repository) ListExecutionBatches(ctx context.Context, id int64) (app.ExecutionBatchList, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return app.ExecutionBatchList{}, err
	}
	defer tx.Rollback(ctx)
	return r.executionBatchListTx(ctx, tx, id)
}
func (r Repository) InitializeExecutionBatches(ctx context.Context, c app.ExecutionBatchInitCommand) (app.ExecutionBatchList, error) {
	if c.EmployeeID <= 0 {
		return app.ExecutionBatchList{}, fmt.Errorf("请先绑定员工")
	}
	if c.ID <= 0 || strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.ExpectedVersion) == "" {
		return app.ExecutionBatchList{}, fmt.Errorf("请刷新后重试")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return app.ExecutionBatchList{}, err
	}
	defer tx.Rollback(ctx)
	var cardID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), c.ID).Scan(&cardID); err != nil {
		return app.ExecutionBatchList{}, err
	}
	var station string
	var stationID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT workstation_id,workstation FROM %s.job_cards WHERE id=$1`, r.schema), c.ID).Scan(&stationID, &station); err != nil {
		return app.ExecutionBatchList{}, err
	}
	owner, roster, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
	if err != nil {
		return app.ExecutionBatchList{}, err
	}
	if roster && owner.EmployeeID != c.EmployeeID {
		return app.ExecutionBatchList{}, fmt.Errorf("请由今日工位负责人启用分批报工")
	}
	out, err := r.executionBatchListTx(ctx, tx, c.ID)
	if err != nil {
		return out, err
	}
	if len(out.Batches) > 0 {
		var key, version string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT init_request_id,init_expected_version FROM %s.production_execution_batches WHERE id=$1`, r.schema), out.Batches[0].ID).Scan(&key, &version); err != nil {
			return out, err
		}
		if key != c.RequestID || version != c.ExpectedVersion {
			return out, fmt.Errorf("任务已启用分批报工，请刷新查看")
		}
		return out, tx.Commit(ctx)
	}
	if !out.CanInitialize {
		return out, fmt.Errorf("%s", out.Reason)
	}
	if out.Version != c.ExpectedVersion {
		return out, fmt.Errorf("任务已变化，请刷新后重新核对")
	}
	p, _, err := r.operationCompletionContextTx(ctx, tx, c.ID, false)
	if err != nil {
		return out, err
	}
	var count int
	var size float64
	var unit string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT planned_batch_count,COALESCE(NULLIF(custom_batch_size_qty,0),batch_size_qty)::float8,COALESCE(batch_size_unit,'') FROM %s.job_cards WHERE id=$1`, r.schema), c.ID).Scan(&count, &size, &unit)
	if err != nil {
		return out, err
	}
	if unit != p.InputUnit {
		a, b := app.OperationUnitFactor(unit), app.OperationUnitFactor(p.InputUnit)
		if a <= 0 || b <= 0 {
			return out, fmt.Errorf("批次单位与投入单位不一致")
		}
		size = size * a / b
	}
	total := p.InputQty
	if p.StartedInputQty > 0 {
		total = p.StartedInputQty
	}
	plan, err := app.ExecutionBatchPlan(total, size, count)
	if err != nil {
		return out, err
	}
	for i, n := range plan {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_execution_batches(job_card_id,batch_no,planned_input_qty,init_request_id,init_expected_version,input_unit) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), c.ID, i+1, n, c.RequestID, c.ExpectedVersion, p.InputUnit); err != nil {
			return out, err
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "job_card", &c.ID, "initialize_execution_batches", nil, nil, nil, infra.AuditMeta{"request_id": c.RequestID, "batch_count": count}); err != nil {
		return out, err
	}
	out, err = r.executionBatchListTx(ctx, tx, c.ID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// A downstream batch consumes the nearest preceding frozen operation's output,
// which may use a different unit than this operation's own output.
func frozenExecutionInputUnit(ops []processSnapshotOperation, seq int, fallback string) (string, bool) {
	var current, prior *processSnapshotOperation
	for i := range ops {
		op := &ops[i]
		if op.Seq == seq {
			current = op
		}
		if op.Seq < seq && (prior == nil || op.Seq > prior.Seq) {
			prior = op
		}
	}
	if current == nil {
		return "", false
	}
	source := current
	if prior != nil {
		source = prior
	}
	cfg, invalid, err := decodeFrozenOperationExecution(source.ExecutionJSON)
	if err != nil || invalid || cfg.Version != 1 {
		return "", false
	}
	unit := firstNonEmpty(cfg.OutputUnit, fallback)
	return unit, app.OperationUnitFactor(unit) > 0
}
