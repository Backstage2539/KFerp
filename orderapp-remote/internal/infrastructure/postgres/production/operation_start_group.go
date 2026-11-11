package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	"orderapp/internal/infrastructure/postgres/customerstock"
	"sort"
	"strings"
)

func (r Repository) PreviewOperationStartGroup(ctx context.Context, ids []int64) (app.OperationStartGroupPreview, error) {
	var out app.OperationStartGroupPreview
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out, err = r.startGroupContextTx(ctx, tx, ids)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (r Repository) startGroupContextTx(ctx context.Context, tx pgx.Tx, ids []int64) (app.OperationStartGroupPreview, error) {
	var out app.OperationStartGroupPreview
	group, sources, err := r.operationCompletionGroupContextTx(ctx, tx, ids, false)
	if err != nil {
		return out, err
	}
	out.Version = group.Version
	seen := map[int64]bool{}
	for _, s := range sources {
		if seen[s.cardID] {
			return out, fmt.Errorf("合并任务重复")
		}
		seen[s.cardID] = true
		if s.p.MaterialConsumptionPolicy != "bom_start_v1" {
			return out, fmt.Errorf("请选择使用按批开工扣料的同组任务")
		}
		var status string
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.job_cards WHERE id=$1`, r.schema), s.cardID).Scan(&status); err != nil {
			return out, err
		}
		if !validJobCardTransition(status, "start") {
			return out, fmt.Errorf("任务已开工或结束，请刷新")
		}
		out.Tasks = append(out.Tasks, s.p)
	}
	return out, nil
}
func (r Repository) StartOperationGroup(ctx context.Context, c app.OperationStartGroupCommand) (app.OperationStartGroupResult, error) {
	var result app.OperationStartGroupResult
	_, err := retryOperationTransaction(ctx, func() (app.JobCardActionResult, error) {
		var unused app.JobCardActionResult
		if c.EmployeeID <= 0 || strings.TrimSpace(c.Operator) == "" {
			return unused, fmt.Errorf("请先绑定员工")
		}
		if strings.TrimSpace(c.RequestID) == "" || strings.TrimSpace(c.ExpectedVersion) == "" {
			return unused, fmt.Errorf("请刷新合并开工预览")
		}
		tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return unused, err
		}
		defer tx.Rollback(ctx)
		week, _, err := app.NormalizeProductionRosterWeek(productionWorkDate(), nil)
		if err != nil {
			return unused, err
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_roster:"+week); err != nil {
			return unused, err
		}
		ids := []int64{}
		for _, item := range c.Items {
			ids = append(ids, item.ID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		type lockRow struct{ work, run int64 }
		locks := []lockRow{}
		for _, id := range ids {
			var x lockRow
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), id).Scan(&x.work, &x.run); err != nil {
				return unused, err
			}
			locks = append(locks, x)
		}
		sort.Slice(locks, func(i, j int) bool { return locks[i].work < locks[j].work })
		for _, x := range locks {
			if x.run > 0 {
				if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, x.run); err != nil {
					return unused, err
				}
			}
		}
		if err = lockProductionFinancePeriodTx(ctx, tx, r.schema); err != nil {
			return unused, err
		}
		for _, x := range locks {
			if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, x.work); err != nil {
				return unused, err
			}
		}
		replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_start_group", c.Operator, c.RequestID, c)
		if err != nil {
			return unused, err
		}
		if replayID > 0 {
			return unused, json.Unmarshal(replay, &result)
		}
		p, err := r.startGroupContextTx(ctx, tx, ids)
		if err != nil {
			return unused, err
		}
		if p.Version != c.ExpectedVersion {
			return unused, fmt.Errorf("任务或库存已变化，请刷新合并开工预览")
		}
		versions := map[int64]string{}
		for _, task := range p.Tasks {
			versions[task.JobCardID] = task.Version
		}
		result.Items = nil
		for _, requested := range c.Items {
			item := requested.Command()
			if item.ExpectedVersion != versions[item.ID] {
				return unused, fmt.Errorf("来源任务版本已变化，请刷新")
			}
			// All client versions were checked before any writes; prior tasks in this
			// same transaction may legitimately change shared inventory's version.
			current, _, e := r.operationCompletionContextTx(ctx, tx, item.ID, true)
			if e != nil {
				return unused, e
			}
			item.ExpectedVersion = current.Version
			item.Operator = c.Operator
			item.EmployeeID = c.EmployeeID
			item.Action = "start"
			item.RequestID = fmt.Sprintf("%s:%d", c.RequestID, item.ID)
			row, e := r.startOperationLockedTx(ctx, tx, item)
			if e != nil {
				return unused, e
			}
			result.Items = append(result.Items, row)
		}
		if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_start_group", c.Operator, c.RequestID, ids[0], result); err != nil {
			return unused, err
		}
		return unused, tx.Commit(ctx)
	})
	return result, err
}
