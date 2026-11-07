package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	"reflect"
	"strings"
)

func (r Repository) Refresh(ctx context.Context, orderID, requestID int64, cmd domain.Command, actor string) (Result, error) {
	var out Result
	if strings.TrimSpace(actor) == "" || strings.TrimSpace(cmd.IdempotencyKey) == "" || len(cmd.IdempotencyKey) > 128 || cmd.Version == "" {
		return out, fmt.Errorf("缺少操作人、预览版本或提交标识")
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, orderID); err != nil {
		return out, err
	}
	var originalRaw []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT context_json FROM %s.order_resolution_requests WHERE id=$1 AND order_id=$2 FOR UPDATE`, r.schema), requestID, orderID).Scan(&originalRaw); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	var prior string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash FROM %s.order_resolution_revisions WHERE request_id=$1 AND idempotency_key=$2`, r.schema), requestID, cmd.IdempotencyKey).Scan(&prior)
	if err == nil {
		if prior != hash(raw) {
			return out, fmt.Errorf("提交标识已用于不同内容")
		}
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if out.Status != "pending" || out.Applied {
		return out, fmt.Errorf("仅可更新尚未执行订单变更的处理单")
	}
	if cmd.Action != out.Command.Action {
		return out, fmt.Errorf("更新方案不能改变业务操作；尚未执行时可撤回后重新选择")
	}
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return out, err
	}
	if cmd.Version != current.Version {
		return out, ErrStale
	}
	var original domain.Context
	if err = json.Unmarshal(originalRaw, &original); err != nil {
		return out, err
	}
	current.Paused = original.Paused
	plan, err := domain.Preview(current, cmd)
	if err != nil {
		return out, err
	}
	if cmd.PostingMode == "open_period" {
		if err = r.checkPeriod(ctx, tx, cmd.PostingDate); err != nil {
			return out, err
		}
	}
	var acted bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.order_resolution_step_actions a JOIN %[1]s.order_resolution_steps s ON s.id=a.step_id WHERE s.request_id=$1)`, r.schema), requestID).Scan(&acted); err != nil {
		return out, err
	}
	if acted && (!reflect.DeepEqual(plan.Lines, out.Plan.Lines) || plan.CreditCents != out.Plan.CreditCents || !reflect.DeepEqual(plan.Fees, out.Plan.Fees)) {
		return out, fmt.Errorf("已有处置结果，不能改动商品范围和冲减金额；请完成原方案后另办更正")
	}
	before, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	kept := map[int64]bool{}
	for _, task := range plan.Tasks {
		matched := false
		for _, step := range out.Steps {
			if step.Status == "superseded" || kept[step.ID] {
				continue
			}
			if task.Kind == "period" && step.Status == "completed" {
				continue
			}
			if task.Kind == "external" && step.Status == "completed" && current.ExternalVersion != original.ExternalVersion {
				continue
			}
			if step.Kind == task.Kind && step.SourceID == task.SourceID && step.Permission == task.Permission && (step.Choice == task.Choice || task.Kind == "funds" || (task.Kind == "external" && step.Status == "completed" && current.ExternalVersion == original.ExternalVersion)) {
				kept[step.ID] = true
				matched = true
				break
			}
		}
		if !matched {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_steps(request_id,kind,source_id,choice,label,permission)VALUES($1,$2,$3,$4,$5,$6)`, r.schema), requestID, task.Kind, task.SourceID, task.Choice, task.Label, task.Permission); err != nil {
				return out, err
			}
		}
	}
	for _, step := range out.Steps {
		if !kept[step.ID] && one(step.Status, "pending", "failed") {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_steps SET status='superseded',updated_at=now() WHERE id=$1`, r.schema), step.ID); err != nil {
				return out, err
			}
		}
	}
	planRaw, _ := json.Marshal(plan)
	contextRaw, _ := json.Marshal(current)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET command_json=$2,plan_json=$3,context_json=$4,updated_at=now() WHERE id=$1`, r.schema), requestID, raw, planRaw, contextRaw); err != nil {
		return out, err
	}
	refreshed, err := r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	pending := []domain.Task{}
	for _, s := range refreshed.Steps {
		if one(s.Status, "pending", "failed") {
			pending = append(pending, s.Task)
		}
	}
	if len(pending) == 0 || (cmd.Timing == "stop_first" && onlyFunds(pending)) {
		if err = r.apply(ctx, tx, orderID, requestID, current, cmd, plan, actor); err != nil {
			return out, err
		}
	}
	if len(pending) == 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET status='completed' WHERE id=$1`, r.schema), requestID); err != nil {
			return out, err
		}
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	after, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_revisions(request_id,idempotency_key,request_hash,before_json,after_json,actor)VALUES($1,$2,$3,$4,$5,$6)`, r.schema), requestID, cmd.IdempotencyKey, hash(raw), before, after, actor); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_refresh", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "command": cmd, "plan": plan}); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
