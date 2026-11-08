package orderresolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
)

// Preview uses one read-only snapshot. For an in-flight return, quantities
// already received by this same request are restored only for revalidating its
// unchanged plan; the public context continues to report the actual receipt.
func (r Repository) Preview(ctx context.Context, orderID int64, cmd domain.Command) (domain.Plan, string, error) {
	var plan domain.Plan
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return plan, "", err
	}
	defer tx.Rollback(ctx)
	current, err := r.load(ctx, tx, orderID)
	if err != nil {
		return plan, "", err
	}
	version := current.Version
	if cmd.Version != "" && cmd.Version != version {
		return plan, version, ErrStale
	}
	var requestID int64
	var originalRaw []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,context_json FROM %s.order_resolution_requests WHERE order_id=$1 AND status='pending' AND NOT applied AND command_json->>'action'=$2`, r.schema), orderID, cmd.Action).Scan(&requestID, &originalRaw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return plan, version, err
	}
	if requestID > 0 {
		var original domain.Context
		if err = json.Unmarshal(originalRaw, &original); err != nil {
			return plan, version, err
		}
		if err = r.normalizePostedReturns(ctx, tx, orderID, requestID, original, cmd, &current); err != nil {
			return plan, version, err
		}
		current.Paused = original.Paused
	}
	plan, err = domain.Preview(current, cmd)
	if err != nil {
		return plan, version, err
	}
	if requestID > 0 {
		prior, err := r.result(ctx, tx, requestID)
		if err != nil {
			return plan, version, err
		}
		var acted bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.order_resolution_step_actions a JOIN %[1]s.order_resolution_steps s ON s.id=a.step_id WHERE s.request_id=$1)`, r.schema), requestID).Scan(&acted); err != nil {
			return plan, version, err
		}
		if acted && (!reflect.DeepEqual(plan.Lines, prior.Plan.Lines) || plan.CreditCents != prior.Plan.CreditCents || !reflect.DeepEqual(plan.Fees, prior.Plan.Fees)) {
			return plan, version, fmt.Errorf("已有处置结果，不能改动商品范围和冲减金额；请完成原方案后另办更正")
		}
	}
	return plan, version, tx.Commit(ctx)
}
