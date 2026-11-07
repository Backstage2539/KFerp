package orderresolution

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
	infra "orderapp/internal/infrastructure/postgres"
	"strings"
)

func (r Repository) Withdraw(ctx context.Context, orderID, requestID int64, note, actor string) (Result, error) {
	var out Result
	if strings.TrimSpace(note) == "" || strings.TrimSpace(actor) == "" {
		return out, fmt.Errorf("请填写撤回原因")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, orderID); err != nil {
		return out, err
	}
	var applied, actions bool
	var state string
	var raw []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,applied,context_json,EXISTS(SELECT 1 FROM %[1]s.order_resolution_step_actions a JOIN %[1]s.order_resolution_steps s ON s.id=a.step_id WHERE s.request_id=r.id) FROM %[1]s.order_resolution_requests r WHERE id=$1 AND order_id=$2 FOR UPDATE`, r.schema), requestID, orderID).Scan(&state, &applied, &raw, &actions)
	if err != nil {
		return out, err
	}
	if state == "withdrawn" {
		return r.result(ctx, tx, requestID)
	}
	if state != "pending" || applied || actions {
		return out, fmt.Errorf("已有处置结果，不能撤回；请继续完成待办或另办更正")
	}
	var original domain.Context
	if err = json.Unmarshal(raw, &original); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET fulfillment_paused=$2 WHERE id=$1`, r.schema), orderID, original.Paused); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_requests SET status='withdrawn',updated_at=now() WHERE id=$1`, r.schema), requestID); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_withdraw", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "note": note}); err != nil {
		return out, err
	}
	out, err = r.result(ctx, tx, requestID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
