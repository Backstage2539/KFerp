package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) SaveOperationProgress(ctx context.Context, c app.JobCardActionCommand) (app.OperationCompletionPreview, error) {
	var p app.OperationCompletionPreview
	if c.EmployeeID <= 0 {
		return p, fmt.Errorf("请先绑定员工")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return p, err
	}
	defer tx.Rollback(ctx)
	var workID int64
	var status, station string
	var stationID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT work_order_id,status,workstation,workstation_id FROM %s.job_cards WHERE id=$1`, r.schema), c.ID).Scan(&workID, &status, &station, &stationID); err != nil {
		return p, err
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, workID); err != nil {
		return p, err
	}
	p, _, err = r.operationCompletionContextTx(ctx, tx, c.ID, true)
	if err != nil {
		return p, err
	}
	if status == "completed" || status == "cancelled" || !p.Enabled {
		return p, fmt.Errorf("本工序不能再保存进度")
	}
	if p.Version != c.ExpectedVersion {
		return p, fmt.Errorf("任务或库存已变化，请刷新后保存；本次未过账")
	}
	owner, enabled, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
	if err != nil {
		return p, err
	}
	if enabled && owner.EmployeeID != c.EmployeeID {
		return p, fmt.Errorf("请由今日工位负责人保存进度")
	}
	raw, _ := json.Marshal(map[string]any{"actual_input_qty": c.ActualInputQty, "actual_output_qty": c.ActualOutputQty, "actual_minutes": c.ActualMinutes, "difference_reason": c.DifferenceReason, "materials": c.Materials})
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET completion_draft=$2::jsonb WHERE id=$1`, r.schema), c.ID, string(raw)); err != nil {
		return p, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, c.Operator, "job_card", &c.ID, "save_progress", nil, nil, nil, infra.AuditMeta{"draft": c, "inventory_changed": false}); err != nil {
		return p, err
	}
	p, _, err = r.operationCompletionContextTx(ctx, tx, c.ID, false)
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}
