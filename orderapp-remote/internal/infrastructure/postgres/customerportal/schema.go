package customerportal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.mini_users (
	id BIGSERIAL PRIMARY KEY,
	openid TEXT NOT NULL,
	unionid TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	nickname TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_login_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS mini_users_openid_uq ON %s.mini_users(openid);

CREATE TABLE IF NOT EXISTS %s.mini_sessions (
	token TEXT PRIMARY KEY,
	mini_user_id BIGINT NOT NULL REFERENCES %s.mini_users(id) ON DELETE CASCADE,
	current_customer_id BIGINT NULL REFERENCES %s.customers(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	expire_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS mini_sessions_user_idx ON %s.mini_sessions(mini_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.customer_portal_profiles (
	customer_id BIGINT PRIMARY KEY REFERENCES %s.customers(id) ON DELETE CASCADE,
	display_name TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'active',
	default_settlement_cycle TEXT NOT NULL DEFAULT 'monthly',
	default_payment_terms TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT true,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS %s.customer_portal_user_bindings (
	id BIGSERIAL PRIMARY KEY,
	mini_user_id BIGINT NOT NULL REFERENCES %s.mini_users(id) ON DELETE CASCADE,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	role TEXT NOT NULL DEFAULT 'member',
	status TEXT NOT NULL DEFAULT 'approved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	approved_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_portal_user_bindings_user_customer_uq
	ON %s.customer_portal_user_bindings(mini_user_id, customer_id);

CREATE TABLE IF NOT EXISTS %s.customer_service_capabilities (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	capability_code TEXT NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT true,
	config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_service_capabilities_customer_code_uq
	ON %s.customer_service_capabilities(customer_id, capability_code);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
