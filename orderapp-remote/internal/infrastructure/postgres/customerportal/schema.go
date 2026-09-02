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
	processing_warehouse_code TEXT NOT NULL DEFAULT '',
	default_sender_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	default_settlement_cycle TEXT NOT NULL DEFAULT 'monthly',
	default_payment_terms TEXT NOT NULL DEFAULT '',
	theme_key TEXT NOT NULL DEFAULT 'coffee_factory',
	miniapp_entry_mode TEXT NOT NULL DEFAULT 'services',
	capability_template_key TEXT NOT NULL DEFAULT '',
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
ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS miniapp_entry_mode TEXT NOT NULL DEFAULT 'services';
ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS capability_template_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS bean_list_mode TEXT NOT NULL DEFAULT 'latest';
ALTER TABLE %s.customer_portal_profiles
	ADD COLUMN IF NOT EXISTS bean_list_publication_id BIGINT NOT NULL DEFAULT 0;
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := ensureBusinessSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProcessingRequestSchema(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensurePortalProfileColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureCapabilityTemplateSchema(ctx, pool, schema); err != nil {
		return err
	}
	return ensureCapabilityConfigConstraint(ctx, pool, schema)
}

func ensureCapabilityTemplateSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.customer_capability_templates (
	template_key TEXT PRIMARY KEY,
	parent_template_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	theme_key TEXT NOT NULL DEFAULT 'coffee_factory',
	miniapp_entry_mode TEXT NOT NULL DEFAULT 'services',
	erp_role_codes JSONB NOT NULL DEFAULT '[]'::jsonb,
	erp_permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
	erp_view_keys JSONB NOT NULL DEFAULT '[]'::jsonb,
	capabilities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INTEGER NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	CONSTRAINT customer_capability_templates_roles_array_chk CHECK (jsonb_typeof(erp_role_codes) = 'array'),
	CONSTRAINT customer_capability_templates_permissions_array_chk CHECK (jsonb_typeof(erp_permissions) = 'array'),
	CONSTRAINT customer_capability_templates_views_array_chk CHECK (jsonb_typeof(erp_view_keys) = 'array'),
	CONSTRAINT customer_capability_templates_capabilities_array_chk CHECK (jsonb_typeof(capabilities_json) = 'array')
);
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	stmts := []string{
		`ALTER TABLE %[1]s.customer_capability_templates ADD COLUMN IF NOT EXISTS parent_template_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customer_capability_templates ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE %[1]s.customer_capability_templates ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
}

func ensurePortalProfileColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		`ALTER TABLE %[1]s.customer_portal_profiles ADD COLUMN IF NOT EXISTS processing_warehouse_code TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE %[1]s.customer_portal_profiles ADD COLUMN IF NOT EXISTS default_sender_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE %[1]s.customer_portal_profiles ADD COLUMN IF NOT EXISTS miniapp_entry_mode TEXT NOT NULL DEFAULT 'services'`,
		`ALTER TABLE %[1]s.customer_portal_profiles ADD COLUMN IF NOT EXISTS capability_template_key TEXT NOT NULL DEFAULT ''`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, fmt.Sprintf(stmt, schema)); err != nil {
			return err
		}
	}
	return nil
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

CREATE TABLE IF NOT EXISTS %s.customer_processing_production_demands (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %s.processing_job_requests(id) ON DELETE CASCADE,
	request_no TEXT NOT NULL DEFAULT '',
	customer_id BIGINT NOT NULL REFERENCES %s.customers(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_name TEXT NOT NULL DEFAULT '',
	inventory_unit TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	target_qty BIGINT NOT NULL DEFAULT 0,
	need_g BIGINT NOT NULL DEFAULT 0,
	target_warehouse TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'planned',
	linked_batch_id TEXT NOT NULL DEFAULT '',
	linked_running_item_id BIGINT NOT NULL DEFAULT 0,
	linked_work_order_id BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(request_id)
);
CREATE INDEX IF NOT EXISTS customer_processing_production_demands_status_idx
	ON %s.customer_processing_production_demands(status, customer_id, product_id, spec_g);

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
	fee_type TEXT NOT NULL CHECK (fee_type IN ('product','roasting','labor','material','processing','shipping','direct_ship_service','packaging','storage','adjustment')),
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

CREATE TABLE IF NOT EXISTS %s.mall_products (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL REFERENCES %s.products(id) ON DELETE RESTRICT,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_name TEXT NOT NULL DEFAULT '',
	inventory_unit TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	subtitle TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	image_url TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 454,
	unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	template_key TEXT NOT NULL DEFAULT 'hero',
	status TEXT NOT NULL DEFAULT 'draft',
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS mall_products_status_sort_idx
	ON %s.mall_products(status, sort_order, id);
ALTER TABLE %[1]s.mall_products ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.mall_products ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.mall_products ADD COLUMN IF NOT EXISTS spec_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.mall_products ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS %[1]s.order_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE IF EXISTS %[1]s.order_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureProcessingRequestSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.processing_job_request_items (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %[1]s.processing_job_requests(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL DEFAULT 1,
	product_id BIGINT NOT NULL DEFAULT 0,
	parent_product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_name TEXT NOT NULL DEFAULT '',
	inventory_unit TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	target_qty BIGINT NOT NULL DEFAULT 0,
	need_g BIGINT NOT NULL DEFAULT 0,
	target_warehouse TEXT NOT NULL DEFAULT '',
	bom_version_id BIGINT NOT NULL DEFAULT 0,
	bom_version_no TEXT NOT NULL DEFAULT '',
	bom_source_product_id BIGINT NOT NULL DEFAULT 0,
	bom_inherited BOOLEAN NOT NULL DEFAULT false,
	material_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	production_plan_id BIGINT NOT NULL DEFAULT 0,
	production_plan_item_id BIGINT NOT NULL DEFAULT 0,
	linked_work_order_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'submitted',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(request_id, line_no)
);
CREATE INDEX IF NOT EXISTS processing_job_request_items_request_idx
	ON %[1]s.processing_job_request_items(request_id, line_no, id);
CREATE INDEX IF NOT EXISTS processing_job_request_items_work_order_idx
	ON %[1]s.processing_job_request_items(linked_work_order_id)
	WHERE linked_work_order_id > 0;

ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS request_item_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS production_plan_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS production_plan_item_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS spec_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.customer_processing_production_demands
	ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.processing_job_request_items
	ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.processing_job_request_items
	ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.processing_job_request_items
	ADD COLUMN IF NOT EXISTS spec_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.processing_job_request_items
	ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT '';

INSERT INTO %[1]s.processing_job_request_items(
	request_id,line_no,product_id,product_name,spec_g,target_qty,need_g,target_warehouse,
	linked_work_order_id,status,created_at,updated_at
)
SELECT r.id,1,r.target_product_id,COALESCE(p.name,''),r.target_spec_g,r.target_qty,
	GREATEST(0,r.target_spec_g*r.target_qty::bigint),COALESCE(d.target_warehouse,''),
	GREATEST(COALESCE(d.linked_work_order_id,0),COALESCE(r.linked_work_order_id,0)),
	CASE
		WHEN COALESCE(d.status,'')='done' THEN 'completed'
		WHEN COALESCE(d.status,'')='running' THEN 'running'
		WHEN COALESCE(d.linked_work_order_id,0)>0 THEN 'released'
		ELSE COALESCE(NULLIF(r.status,''),'submitted')
	END,
	r.created_at,now()
FROM %[1]s.processing_job_requests r
LEFT JOIN %[1]s.products p ON p.id=r.target_product_id
LEFT JOIN %[1]s.customer_processing_production_demands d ON d.request_id=r.id
WHERE lower(COALESCE(r.status,'')) NOT IN ('completed','cancelled','closed')
  AND NOT EXISTS (
	SELECT 1 FROM %[1]s.processing_job_request_items item WHERE item.request_id=r.id
);

UPDATE %[1]s.customer_processing_production_demands d
SET request_item_id=(
	SELECT i.id
	FROM %[1]s.processing_job_request_items i
	WHERE i.request_id=d.request_id
	ORDER BY
		CASE WHEN i.product_id=d.product_id AND i.spec_g=d.spec_g THEN 0 ELSE 1 END,
	i.line_no,
	i.id
	LIMIT 1
)
WHERE d.request_item_id=0
  AND EXISTS (
	SELECT 1 FROM %[1]s.processing_job_request_items i WHERE i.request_id=d.request_id
  );

ALTER TABLE %[1]s.customer_processing_production_demands
	DROP CONSTRAINT IF EXISTS customer_processing_production_demands_request_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS customer_processing_production_demands_request_item_uq
	ON %[1]s.customer_processing_production_demands(request_item_id)
	WHERE request_item_id > 0;
CREATE INDEX IF NOT EXISTS customer_processing_production_demands_plan_item_idx
	ON %[1]s.customer_processing_production_demands(production_plan_item_id)
	WHERE production_plan_item_id > 0;

CREATE TABLE IF NOT EXISTS %[1]s.customer_processing_material_reservations (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %[1]s.processing_job_requests(id) ON DELETE CASCADE,
	request_item_id BIGINT NOT NULL REFERENCES %[1]s.processing_job_request_items(id) ON DELETE CASCADE,
	customer_id BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	required_g BIGINT NOT NULL DEFAULT 0,
	required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,
	reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,
	returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_type TEXT NOT NULL DEFAULT 'factory',
	source_customer_id BIGINT NOT NULL DEFAULT 0,
	source_warehouse_code TEXT NOT NULL DEFAULT '',
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	finished_stock_batch_id BIGINT NOT NULL DEFAULT 0,
	production_plan_id BIGINT NOT NULL DEFAULT 0,
	production_plan_item_id BIGINT NOT NULL DEFAULT 0,
	work_order_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.customer_processing_material_reservations
	ADD COLUMN IF NOT EXISTS consumed_g BIGINT NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS consumed_units BIGINT NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS returned_g BIGINT NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS returned_units BIGINT NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS finished_stock_batch_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS customer_processing_material_reservations_available_idx
	ON %[1]s.customer_processing_material_reservations(material_id,component_type,source_warehouse_code,status);
CREATE INDEX IF NOT EXISTS customer_processing_material_reservations_request_idx
	ON %[1]s.customer_processing_material_reservations(request_id,request_item_id,id);
CREATE INDEX IF NOT EXISTS customer_processing_material_reservations_work_order_idx
	ON %[1]s.customer_processing_material_reservations(work_order_id,status)
	WHERE work_order_id > 0;
CREATE INDEX IF NOT EXISTS customer_processing_material_reservations_finished_batch_idx
	ON %[1]s.customer_processing_material_reservations(finished_stock_batch_id,status)
	WHERE finished_stock_batch_id > 0;
`, schema)
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
