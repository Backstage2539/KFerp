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
	theme_key TEXT NOT NULL DEFAULT 'coffee_factory',
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
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CONSTRAINT customer_service_capabilities_config_object_chk CHECK (jsonb_typeof(config_json) = 'object')
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_service_capabilities_customer_code_uq
	ON %s.customer_service_capabilities(customer_id, capability_code);

ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS theme_key TEXT NOT NULL DEFAULT 'coffee_factory';
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := ensureBusinessSchema(ctx, pool, schema); err != nil {
		return err
	}
	return ensureCapabilityConfigConstraint(ctx, pool, schema)
}

func ensureBusinessSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.direct_ship_import_batches (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	batch_no TEXT NOT NULL DEFAULT '',
	source_name TEXT NOT NULL DEFAULT '',
	source_file_asset_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'submitted',
	total_rows INTEGER NOT NULL DEFAULT 0,
	valid_rows INTEGER NOT NULL DEFAULT 0,
	invalid_rows INTEGER NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	created_by_mini_user_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS direct_ship_import_batches_customer_idx
	ON %s.direct_ship_import_batches(customer_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS direct_ship_import_batches_batch_no_uq
	ON %s.direct_ship_import_batches(batch_no)
	WHERE batch_no <> '';

CREATE TABLE IF NOT EXISTS %s.processing_job_requests (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	request_no TEXT NOT NULL DEFAULT '',
	input_material_id BIGINT NOT NULL DEFAULT 0,
	input_qty_g BIGINT NOT NULL DEFAULT 0,
	target_product_id BIGINT NOT NULL DEFAULT 0,
	target_spec_g BIGINT NOT NULL DEFAULT 0,
	target_qty INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'submitted',
	note TEXT NOT NULL DEFAULT '',
	created_by_mini_user_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	accepted_at TIMESTAMPTZ NULL,
	linked_work_order_id BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS processing_job_requests_customer_idx
	ON %s.processing_job_requests(customer_id, created_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS processing_job_requests_request_no_uq
	ON %s.processing_job_requests(request_no)
	WHERE request_no <> '';

CREATE TABLE IF NOT EXISTS %s.customer_inventory_items (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'available',
	note TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS customer_inventory_items_customer_idx
	ON %s.customer_inventory_items(customer_id, item_type, item_name);
CREATE UNIQUE INDEX IF NOT EXISTS customer_inventory_items_customer_item_uq
	ON %s.customer_inventory_items(customer_id, item_type, item_id, spec_g, warehouse);

CREATE TABLE IF NOT EXISTS %s.customer_fee_items (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	source_type TEXT NOT NULL DEFAULT '',
	source_id BIGINT NOT NULL DEFAULT 0,
	fee_type TEXT NOT NULL CHECK (fee_type IN ('product','processing','shipping','direct_ship_service','packaging','storage','adjustment')),
	amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	currency TEXT NOT NULL DEFAULT 'CNY',
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	settlement_batch_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'unsettled',
	note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS customer_fee_items_customer_status_idx
	ON %s.customer_fee_items(customer_id, status, occurred_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS %s.customer_settlement_batches (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	settlement_no TEXT NOT NULL DEFAULT '',
	period_from DATE NULL,
	period_to DATE NULL,
	status TEXT NOT NULL DEFAULT 'draft',
	total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	confirmed_at TIMESTAMPTZ NULL,
	paid_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_settlement_batches_no_uq
	ON %s.customer_settlement_batches(settlement_no)
	WHERE settlement_no <> '';
CREATE INDEX IF NOT EXISTS customer_settlement_batches_customer_status_idx
	ON %s.customer_settlement_batches(customer_id, status, created_at DESC, id DESC);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureCapabilityConfigConstraint(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname='%[1]s'
		  AND t.relname='customer_service_capabilities'
		  AND c.conname='customer_service_capabilities_config_object_chk'
	) THEN
		ALTER TABLE %[1]s.customer_service_capabilities
			ADD CONSTRAINT customer_service_capabilities_config_object_chk CHECK (jsonb_typeof(config_json) = 'object') NOT VALID;
	END IF;
	IF NOT EXISTS (
		SELECT 1
		FROM %[1]s.customer_service_capabilities
		WHERE jsonb_typeof(config_json) <> 'object'
	) THEN
		ALTER TABLE %[1]s.customer_service_capabilities
			VALIDATE CONSTRAINT customer_service_capabilities_config_object_chk;
	END IF;
END $$;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
