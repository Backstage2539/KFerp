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

type PolicyRecord struct {
	Revision int64         `json:"revision"`
	Policy   domain.Policy `json:"policy"`
}

func (r Repository) Policy(ctx context.Context) (PolicyRecord, error) { return r.policy(ctx, r.pool) }
func (r Repository) policy(ctx context.Context, q querier) (PolicyRecord, error) {
	var out PolicyRecord
	var raw []byte
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT revision,policy_json FROM %s.order_resolution_policy WHERE id=1`, r.schema)).Scan(&out.Revision, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &out.Policy)
	}
	return out, err
}
func (r Repository) SavePolicy(ctx context.Context, revision int64, policy domain.Policy, actor string) (PolicyRecord, error) {
	var out PolicyRecord
	if strings.TrimSpace(actor) == "" {
		return out, fmt.Errorf("缺少配置操作人")
	}
	if err := policy.Validate(); err != nil {
		return out, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	// Configuration takes effect between complete business transactions, never
	// halfway through a disposition validated under the preceding policy.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":finance-period"); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.order_resolution_policy WHERE id=1 FOR UPDATE`, r.schema)); err != nil {
		return out, err
	}
	before, err := r.policy(ctx, tx)
	if err != nil {
		return out, err
	}
	if revision != before.Revision {
		return out, ErrStale
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_policy SET policy_json=$1,revision=revision+1,updated_by=$2,updated_at=now() WHERE id=1`, r.schema), raw, actor); err != nil {
		return out, err
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order_resolution_policy", nil, "resolution_policy", nil, nil, nil, infra.AuditMeta{"before": before, "policy": policy}); err != nil {
		return out, err
	}
	out, err = r.policy(ctx, tx)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
