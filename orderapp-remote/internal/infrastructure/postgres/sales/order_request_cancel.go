package sales

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/sales"
	pg "orderapp/internal/infrastructure/postgres"
)

// CancelUncommittedOrderRequest serializes with SaveOrder, and permanently
// prevents a delayed original request from committing after the check.
func (r Repository) CancelUncommittedOrderRequest(ctx context.Context, cmd app.OrderRequestCancelCommand) (app.OrderRequestResolution, error) {
	var out app.OrderRequestResolution
	if len(strings.TrimSpace(cmd.RequestID)) < 16 || len(cmd.RequestID) > 100 || strings.TrimSpace(cmd.RequestActor) == "" || strings.TrimSpace(cmd.RequestHash) == "" || strings.TrimSpace(cmd.Actor) == "" || strings.TrimSpace(cmd.Reason) == "" {
		return out, fmt.Errorf("原请求及核对原因必填")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "order-create-global:"+cmd.RequestID); err != nil {
		return out, err
	}
	var owner, hash string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_actor,request_hash FROM %s.order_create_request_cancellations WHERE request_id=$1`, r.schema), cmd.RequestID).Scan(&owner, &hash)
	if err == nil {
		if cmd.RequestActor != "unknown" && (owner != cmd.RequestActor || hash != cmd.RequestHash) && owner != "unknown" {
			return out, fmt.Errorf("原请求内容不匹配")
		}
		out.Status = "cancelled"
		return out, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return out, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT order_id FROM (SELECT order_id FROM %[1]s.order_create_idempotency_keys WHERE request_id=$1 UNION SELECT id FROM %[1]s.orders WHERE customer_request_id=$1 UNION SELECT order_id FROM %[1]s.order_content_versions WHERE request_id=$1) x LIMIT 1`, r.schema), cmd.RequestID).Scan(&out.OrderID)
	if err == nil {
		out.Status = "committed"
		return out, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_create_request_cancellations(request_id,request_actor,request_hash,cancelled_by,reason) VALUES($1,$2,$3,$4,$5)`, r.schema), cmd.RequestID, cmd.RequestActor, cmd.RequestHash, cmd.Actor, cmd.Reason); err != nil {
		return out, err
	}
	if err = pg.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order_create_request", nil, "cancel_uncommitted", nil, nil, nil, pg.AuditMeta{"request_id": cmd.RequestID, "request_actor": cmd.RequestActor, "reason": cmd.Reason}); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Status = "cancelled"
	return out, nil
}

func (r Repository) OrderRequestCancelled(ctx context.Context, id string) (bool, error) {
	var cancelled bool
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_create_request_cancellations WHERE request_id=$1)`, r.schema), id).Scan(&cancelled)
	return cancelled, err
}
