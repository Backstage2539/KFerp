package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	"orderapp/internal/infrastructure/postgres/customerstock"
)

// Keep derived IDs within the same 160-character limit as user request IDs.
// Short IDs retain their readable form; longer IDs get a deterministic digest.
func batchChildRequestID(requestID string, batchID int64) string {
	key := fmt.Sprintf("%s:%d", requestID, batchID)
	if len(key) <= 160 {
		return key
	}
	return fmt.Sprintf("bulk:%x:%d", sha256.Sum256([]byte(requestID)), batchID)
}

func (r Repository) CompleteExecutionBatches(ctx context.Context, cmd app.ExecutionBatchCompleteAllCommand) (app.ExecutionBatchCompleteAllResult, error) {
	return retryOperationTransaction(ctx, func() (app.ExecutionBatchCompleteAllResult, error) { return r.completeExecutionBatchesOnce(ctx, cmd) })
}
func (r Repository) completeExecutionBatchesOnce(ctx context.Context, cmd app.ExecutionBatchCompleteAllCommand) (app.ExecutionBatchCompleteAllResult, error) {
	out := app.ExecutionBatchCompleteAllResult{Items: []app.JobCardActionResult{}}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var workID, runID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), cmd.ID).Scan(&workID, &runID); err != nil {
		return out, err
	}
	if runID > 0 {
		if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, runID); err != nil {
			return out, err
		}
	}
	if err = lockProductionFinancePeriodTx(ctx, tx, r.schema); err != nil {
		return out, err
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, workID); err != nil {
		return out, err
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "execution_batches_complete_all", cmd.Operator, cmd.RequestID, struct {
		CardID  int64
		Command app.ExecutionBatchCompleteAllCommand
	}{cmd.ID, cmd})
	if err != nil {
		return out, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	p, _, err := r.operationCompletionContextTx(ctx, tx, cmd.ID, true)
	if err != nil {
		return out, err
	}
	if p.Version != cmd.ExpectedVersion {
		return out, fmt.Errorf("任务或库存已变化，请刷新并重新核对")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,batch_no FROM %s.production_execution_batches WHERE job_card_id=$1 AND status='pending' ORDER BY batch_no FOR UPDATE`, r.schema), cmd.ID)
	if err != nil {
		return out, err
	}
	pending := map[int64]int{}
	for rows.Next() {
		var id int64
		var batchNo int
		if err = rows.Scan(&id, &batchNo); err != nil {
			rows.Close()
			return out, err
		}
		pending[id] = batchNo
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	if len(pending) != len(cmd.Batches) || len(pending) == 0 {
		return out, fmt.Errorf("待完成批次已变化，请刷新并重新核对")
	}
	for _, b := range cmd.Batches {
		if pending[b.ExecutionBatchID] == 0 {
			return out, fmt.Errorf("待完成批次已变化，请刷新并重新核对")
		}

	}
	for _, b := range cmd.Batches {
		p, _, err = r.operationCompletionContextTx(ctx, tx, cmd.ID, false)
		if err != nil {
			return out, err
		}
		child := app.JobCardActionCommand{ID: cmd.ID, EmployeeID: cmd.EmployeeID, Operator: cmd.Operator, AssignedEmployeeName: cmd.Operator, RequestID: batchChildRequestID(cmd.RequestID, b.ExecutionBatchID), ExpectedVersion: p.Version, ExecutionBatchID: b.ExecutionBatchID, ActualInputQty: *b.ActualInputQty, ActualOutputQty: *b.ActualOutputQty, ActualOutputPresent: true, ActualMinutes: *b.ActualMinutes, ActualMinutesExplicit: true, DifferenceReason: strings.TrimSpace(b.DifferenceReason), InputUnit: b.InputUnit, OutputUnit: b.OutputUnit, MetricsJSON: "{}"}
		// Reserve the same idempotency namespace used by a single completion
		// before the shared completion code writes its receipt. Without this,
		// its final UPDATE could overwrite an unrelated single-request receipt.
		priorID, _, e := requestProductionKeyTx(ctx, tx, r.schema, "operation_complete", child.Operator, child.RequestID, child)
		if e != nil {
			return out, fmt.Errorf("第 %d 批：%w", pending[b.ExecutionBatchID], e)
		}
		if priorID > 0 {
			return out, fmt.Errorf("第 %d 批：报工请求标识已被使用，请重新操作", pending[b.ExecutionBatchID])
		}
		result, e := r.completeOperationLockedTx(ctx, tx, child, workID, runID)
		if e != nil {
			return out, fmt.Errorf("第 %d 批：%w", pending[b.ExecutionBatchID], e)
		}
		out.Items = append(out.Items, result)
	}
	entityID := out.Items[0].Completion.EventID
	if err = finishProductionKeyTx(ctx, tx, r.schema, "execution_batches_complete_all", cmd.Operator, cmd.RequestID, entityID, out); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
