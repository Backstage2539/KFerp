package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	infra "orderapp/internal/infrastructure/postgres"
)

type StepFailure struct {
	Message   string    `json:"message"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
}

func (r Repository) recordStepFailure(ctx context.Context, orderID, requestID, stepID int64, cmd StepCommand, actor string, cause error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = r.lock(ctx, tx, orderID); err != nil {
		return err
	}
	var eligible bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.order_resolution_steps s JOIN %[1]s.order_resolution_requests r ON r.id=s.request_id WHERE s.id=$1 AND r.id=$2 AND r.order_id=$3 AND s.status IN('pending','failed') AND r.status='pending' AND NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_step_actions a WHERE a.step_id=s.id AND a.idempotency_key=$4))`, r.schema), stepID, requestID, orderID, cmd.IdempotencyKey).Scan(&eligible)
	if err != nil || !eligible {
		return err
	}
	message := cause.Error()
	var dbError *pgconn.PgError
	if errors.As(cause, &dbError) {
		message = "本次登记未保存，请刷新核对后重试；钱货变更已回滚"
		if dbError.Code == "40001" || dbError.Code == "40P01" {
			message = ErrStale.Error()
		}
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	inserted, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_step_failures(step_id,idempotency_key,request_hash,command_json,message,actor)VALUES($1,$2,$3,$4,$5,$6)ON CONFLICT DO NOTHING`, r.schema), stepID, cmd.IdempotencyKey, hash(raw), raw, message, actor)
	if err != nil || inserted.RowsAffected() == 0 {
		return err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_steps SET status='failed',last_error=$2,updated_at=now()WHERE id=$1`, r.schema), stepID, message); err != nil {
		return err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "resolution_step_failed", nil, nil, nil, infra.AuditMeta{"resolution_id": requestID, "step_id": stepID, "command": cmd, "message": message}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
