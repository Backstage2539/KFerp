package production

import (
	"context"
	"crypto/sha256"
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
	if len(ids) < 2 || len(ids) > 100 {
		return out, fmt.Errorf("请选择 2 至 100 张同工位待开工工单")
	}
	sortedIDs := append([]int64(nil), ids...)
	sort.Slice(sortedIDs, func(i, j int) bool { return sortedIDs[i] < sortedIDs[j] })
	seen := map[int64]bool{}
	seenOrders := map[int64]bool{}
	stationKey := ""
	versionParts := make([]any, 0, len(ids))
	for _, id := range sortedIDs {
		if id <= 0 || seen[id] {
			return out, fmt.Errorf("所选任务无效或重复")
		}
		seen[id] = true
		var status, station, no string
		var stationID, workID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT j.status,j.workstation,COALESCE(j.workstation_id,0),w.id,w.work_order_no FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), id).Scan(&status, &station, &stationID, &workID, &no); err != nil {
			return out, err
		}
		if seenOrders[workID] {
			return out, fmt.Errorf("%s：同一工单包含多个包装任务，请逐卡处理", no)
		}
		seenOrders[workID] = true
		currentStation := fmt.Sprintf("%d:%s", stationID, strings.TrimSpace(station))
		if len(out.Tasks) > 0 && currentStation != stationKey {
			return out, fmt.Errorf("%s：请选择同一工位的待开工工单", no)
		}
		stationKey = currentStation
		if !validJobCardTransition(status, "start") {
			return out, fmt.Errorf("%s：任务已开工或结束，请刷新", no)
		}
		p, wo, err := r.operationCompletionContextTx(ctx, tx, id, false)
		if err != nil {
			return out, fmt.Errorf("%s：%w", no, err)
		}
		if p.BlockedReason != "" {
			return out, fmt.Errorf("%s：%s", no, p.BlockedReason)
		}
		legacy := p.Execution.ReportingType == "" || p.Execution.ReportingType == "legacy"
		if p.MaterialConsumptionPolicy != "bom_start_v1" || !p.Enabled || !p.Final || !p.Execution.ReceiptEnabled || (p.Execution.ReportingType != "piece" && !(legacy && isPackagingOperationName(taskOperationForCard(ctx, tx, r.schema, id)))) {
			return out, fmt.Errorf("%s：请选择按开工扣料的包装任务", no)
		}
		if wo.Status == "completed" || wo.Status == "cancelled" {
			return out, fmt.Errorf("%s：工单已结束", no)
		}
		out.Tasks = append(out.Tasks, p)
		versionParts = append(versionParts, []any{id, p.Version, stationID, station})
	}
	raw, _ := json.Marshal(versionParts)
	out.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
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
		if len(c.Items) < 2 || len(c.Items) > 100 {
			return unused, fmt.Errorf("请选择 2 至 100 张同工位待开工工单")
		}
		ids := []int64{}
		seenIDs := map[int64]bool{}
		for _, item := range c.Items {
			if item.ID <= 0 || seenIDs[item.ID] {
				return unused, fmt.Errorf("所选任务无效或重复")
			}
			seenIDs[item.ID] = true
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
		for _, requested := range c.Items {
			if requested.ExpectedVersion != versions[requested.ID] {
				return unused, fmt.Errorf("工序卡 %d：来源任务版本已变化，请刷新", requested.ID)
			}
		}
		result.Items = nil
		for _, requested := range c.Items {
			item := requested.Command()
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
				return unused, fmt.Errorf("工序卡 %d：%w", item.ID, e)
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
