package appmain

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureAuditTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT NULL,
	action TEXT NOT NULL DEFAULT '',
	field TEXT NULL,
	old_value TEXT NULL,
	new_value TEXT NULL,
	meta JSONB NULL
);
CREATE INDEX IF NOT EXISTS audit_logs_ts_idx ON %s.audit_logs(ts DESC);
CREATE INDEX IF NOT EXISTS audit_logs_entity_idx ON %s.audit_logs(entity_type, entity_id, ts DESC);
CREATE INDEX IF NOT EXISTS audit_logs_actor_idx ON %s.audit_logs(actor, ts DESC);

CREATE TABLE IF NOT EXISTS %s.order_audit_logs (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL,
	actor TEXT NOT NULL DEFAULT '',
	field TEXT NOT NULL DEFAULT '',
	old_value TEXT NULL,
	new_value TEXT NULL,
	changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS order_audit_logs_order_idx ON %s.order_audit_logs(order_id, id DESC);

CREATE TABLE IF NOT EXISTS %s.operation_logs (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	actor TEXT NOT NULL DEFAULT '',
	employee_id BIGINT NULL,
	method TEXT NOT NULL DEFAULT '',
	path TEXT NOT NULL DEFAULT '',
	route TEXT NOT NULL DEFAULT '',
	query TEXT NOT NULL DEFAULT '',
	status INTEGER NOT NULL DEFAULT 0,
	duration_ms BIGINT NOT NULL DEFAULT 0,
	ip TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	referer TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS operation_logs_ts_idx ON %s.operation_logs(ts DESC);
CREATE INDEX IF NOT EXISTS operation_logs_actor_idx ON %s.operation_logs(actor, ts DESC);
CREATE INDEX IF NOT EXISTS operation_logs_route_idx ON %s.operation_logs(method, route, ts DESC);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
