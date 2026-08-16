package customerfulfillment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.customer_fulfillment_import_batches (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	import_type TEXT NOT NULL DEFAULT '',
	source_filename TEXT NOT NULL DEFAULT '',
	source_sha256 TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'parsed',
	total_rows INTEGER NOT NULL DEFAULT 0,
	valid_rows INTEGER NOT NULL DEFAULT 0,
	invalid_rows INTEGER NOT NULL DEFAULT 0,
	summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	applied_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_fulfillment_import_batches_customer_type_sha_idx
	ON %[1]s.customer_fulfillment_import_batches(customer_id, import_type, source_sha256);

CREATE TABLE IF NOT EXISTS %[1]s.customer_erp_user_bindings (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	employee_id BIGINT NOT NULL REFERENCES %[1]s.company_employees(id) ON DELETE CASCADE,
	role TEXT NOT NULL DEFAULT 'customer',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);
WITH active_customer_bindings AS (
	SELECT id,
	       ROW_NUMBER() OVER (PARTITION BY customer_id ORDER BY updated_at DESC, id DESC) AS rn
	FROM %[1]s.customer_erp_user_bindings
	WHERE status='active'
)
UPDATE %[1]s.customer_erp_user_bindings b
SET status='inactive',
    updated_at=now(),
    updated_by=COALESCE(NULLIF(b.updated_by,''),'system:migration')
FROM active_customer_bindings r
WHERE b.id=r.id AND r.rn > 1;
WITH active_employee_bindings AS (
	SELECT id,
	       ROW_NUMBER() OVER (PARTITION BY employee_id ORDER BY updated_at DESC, id DESC) AS rn
	FROM %[1]s.customer_erp_user_bindings
	WHERE status='active'
)
UPDATE %[1]s.customer_erp_user_bindings b
SET status='inactive',
    updated_at=now(),
    updated_by=COALESCE(NULLIF(b.updated_by,''),'system:migration')
FROM active_employee_bindings r
WHERE b.id=r.id AND r.rn > 1;
CREATE UNIQUE INDEX IF NOT EXISTS customer_erp_user_bindings_employee_customer_uq
	ON %[1]s.customer_erp_user_bindings(employee_id, customer_id);
CREATE UNIQUE INDEX IF NOT EXISTS customer_erp_user_bindings_customer_active_uq
	ON %[1]s.customer_erp_user_bindings(customer_id)
	WHERE status='active';
CREATE UNIQUE INDEX IF NOT EXISTS customer_erp_user_bindings_employee_active_uq
	ON %[1]s.customer_erp_user_bindings(employee_id)
	WHERE status='active';
CREATE INDEX IF NOT EXISTS customer_erp_user_bindings_employee_status_idx
	ON %[1]s.customer_erp_user_bindings(employee_id, status, customer_id);
CREATE INDEX IF NOT EXISTS customer_erp_user_bindings_customer_idx
	ON %[1]s.customer_erp_user_bindings(customer_id, employee_id);

CREATE TABLE IF NOT EXISTS %[1]s.customer_fulfillment_import_rows (
	id BIGSERIAL PRIMARY KEY,
	batch_id BIGINT NOT NULL REFERENCES %[1]s.customer_fulfillment_import_batches(id) ON DELETE CASCADE,
	sheet_name TEXT NOT NULL DEFAULT '',
	row_no INTEGER NOT NULL DEFAULT 0,
	row_type TEXT NOT NULL DEFAULT '',
	external_key TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'valid',
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	error TEXT NOT NULL DEFAULT '',
	target_type TEXT NOT NULL DEFAULT '',
	target_id BIGINT NOT NULL DEFAULT 0,
	applied_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.customer_fulfillment_import_rows ADD COLUMN IF NOT EXISTS target_type TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.customer_fulfillment_import_rows ADD COLUMN IF NOT EXISTS target_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_fulfillment_import_rows ADD COLUMN IF NOT EXISTS applied_at TIMESTAMPTZ NULL;
CREATE INDEX IF NOT EXISTS customer_fulfillment_import_rows_batch_type_status_idx
	ON %[1]s.customer_fulfillment_import_rows(batch_id, row_type, status);

CREATE TABLE IF NOT EXISTS %[1]s.customer_custody_items (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	item_type TEXT NOT NULL DEFAULT '',
	external_code TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	spec TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '',
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_custody_items_customer_type_external_code_idx
	ON %[1]s.customer_custody_items(customer_id, item_type, external_code)
	WHERE external_code <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_custody_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	item_id BIGINT NOT NULL DEFAULT 0,
	item_type TEXT NOT NULL DEFAULT '',
	source_type TEXT NOT NULL DEFAULT '',
	source_id BIGINT NOT NULL DEFAULT 0,
	source_external_key TEXT NOT NULL DEFAULT '',
	movement_type TEXT NOT NULL DEFAULT '',
	qty_g_delta BIGINT NOT NULL DEFAULT 0,
	qty_units_delta BIGINT NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.customer_custody_ledger_entries ADD COLUMN IF NOT EXISTS movement_type TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS customer_custody_ledger_customer_item_idx
	ON %[1]s.customer_custody_ledger_entries(customer_id, item_type, item_id, occurred_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS customer_custody_ledger_source_uq
	ON %[1]s.customer_custody_ledger_entries(customer_id, source_type, source_external_key, item_id, movement_type)
	WHERE source_external_key <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_custody_balances (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	item_id BIGINT NOT NULL DEFAULT 0,
	item_type TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	spec TEXT NOT NULL DEFAULT '',
	quantity_g BIGINT NOT NULL DEFAULT 0,
	quantity_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_custody_balances_customer_item_idx
	ON %[1]s.customer_custody_balances(customer_id, item_type, item_id);

CREATE TABLE IF NOT EXISTS %[1]s.customer_inventory_items (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
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
	ON %[1]s.customer_inventory_items(customer_id, item_type, item_name);
CREATE UNIQUE INDEX IF NOT EXISTS customer_inventory_items_customer_item_uq
	ON %[1]s.customer_inventory_items(customer_id, item_type, item_id, spec_g, warehouse);

CREATE TABLE IF NOT EXISTS %[1]s.customer_processing_work_orders (
	id BIGSERIAL PRIMARY KEY,
	batch_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	external_key TEXT NOT NULL DEFAULT '',
	work_order_no TEXT NOT NULL DEFAULT '',
	order_date DATE NULL,
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	input_quantity_g BIGINT NOT NULL DEFAULT 0,
	planned_output_units BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_processing_work_orders_customer_external_idx
	ON %[1]s.customer_processing_work_orders(customer_id, external_key)
	WHERE external_key <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_processing_work_order_inputs (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL REFERENCES %[1]s.customer_processing_work_orders(id) ON DELETE CASCADE,
	raw_bean_item_id BIGINT NOT NULL DEFAULT 0,
	raw_bean_name TEXT NOT NULL DEFAULT '',
	quantity_g BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS %[1]s.customer_processing_packaging_jobs (
	id BIGSERIAL PRIMARY KEY,
	batch_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	external_key TEXT NOT NULL DEFAULT '',
	work_order_no TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	packaging_name TEXT NOT NULL DEFAULT '',
	quantity_units BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_processing_packaging_jobs_customer_external_idx
	ON %[1]s.customer_processing_packaging_jobs(customer_id, external_key)
	WHERE external_key <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_inventory_conversion_jobs (
	id BIGSERIAL PRIMARY KEY,
	batch_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	external_key TEXT NOT NULL DEFAULT '',
	job_no TEXT NOT NULL DEFAULT '',
	from_product TEXT NOT NULL DEFAULT '',
	to_product TEXT NOT NULL DEFAULT '',
	quantity_units BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_inventory_conversion_jobs_customer_external_idx
	ON %[1]s.customer_inventory_conversion_jobs(customer_id, external_key)
	WHERE external_key <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_import_orders (
	id BIGSERIAL PRIMARY KEY,
	batch_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	external_order_no TEXT NOT NULL DEFAULT '',
	external_seq TEXT NOT NULL DEFAULT '',
	order_date DATE NULL,
	receiver_address TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	order_id BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_direct_ship_import_orders_customer_external_idx
	ON %[1]s.customer_direct_ship_import_orders(customer_id, external_order_no, external_seq)
	WHERE external_order_no <> '';

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_import_order_items (
	id BIGSERIAL PRIMARY KEY,
	import_order_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_import_orders(id) ON DELETE CASCADE,
	batch_id BIGINT NOT NULL DEFAULT 0,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	product_title TEXT NOT NULL DEFAULT '',
	spec TEXT NOT NULL DEFAULT '',
	quantity_units BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS %[1]s.order_shipping_trackings (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	tracking_no TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(order_id, tracking_no)
);
CREATE INDEX IF NOT EXISTS idx_%[1]s_order_shipping_trackings_order ON %[1]s.order_shipping_trackings(order_id, id);
CREATE INDEX IF NOT EXISTS idx_%[1]s_order_shipping_trackings_no ON %[1]s.order_shipping_trackings(tracking_no);

CREATE TABLE IF NOT EXISTS %[1]s.order_shipping_tracking_events (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	tracking_no TEXT NOT NULL DEFAULT '',
	event_time TIMESTAMPTZ NOT NULL,
	status TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	location TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT '',
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS order_shipping_tracking_events_order_time_idx
	ON %[1]s.order_shipping_tracking_events(order_id,event_time,id);

CREATE TABLE IF NOT EXISTS %[1]s.customer_billing_rules (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	fee_type TEXT NOT NULL DEFAULT '',
	fee_name TEXT NOT NULL DEFAULT '',
	unit_price_cents BIGINT NOT NULL DEFAULT 0,
	unit TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS customer_billing_rules_customer_fee_idx
	ON %[1]s.customer_billing_rules(customer_id, fee_type, active);

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_requests (
	id BIGSERIAL PRIMARY KEY,
	request_no TEXT NOT NULL DEFAULT '',
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	employee_id BIGINT NOT NULL DEFAULT 0,
	mini_user_id BIGINT NOT NULL DEFAULT 0,
	idempotency_key TEXT NOT NULL DEFAULT '',
	request_hash TEXT NOT NULL DEFAULT '',
	recipient_name TEXT NOT NULL DEFAULT '',
	recipient_phone TEXT NOT NULL DEFAULT '',
	province TEXT NOT NULL DEFAULT '',
	city TEXT NOT NULL DEFAULT '',
	district TEXT NOT NULL DEFAULT '',
	detail_address TEXT NOT NULL DEFAULT '',
	recipient_company TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'reserved',
	note TEXT NOT NULL DEFAULT '',
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	cancelled_at TIMESTAMPTZ NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_direct_ship_requests_customer_key_uq
	ON %[1]s.customer_direct_ship_requests(customer_id, idempotency_key)
	WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS customer_direct_ship_requests_customer_created_idx
	ON %[1]s.customer_direct_ship_requests(customer_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_request_items (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_requests(id) ON DELETE CASCADE,
	line_no INTEGER NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_key TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	sku_code TEXT NOT NULL DEFAULT '',
	spec_label TEXT NOT NULL DEFAULT '',
	inventory_unit TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	qty BIGINT NOT NULL DEFAULT 0,
	snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(request_id, line_no)
);

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_request_orders (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_requests(id) ON DELETE CASCADE,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	warehouse_code TEXT NOT NULL DEFAULT '',
	order_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(request_id, warehouse_code),
	UNIQUE(order_id)
);

CREATE TABLE IF NOT EXISTS %[1]s.customer_direct_ship_request_allocations (
	id BIGSERIAL PRIMARY KEY,
	request_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_requests(id) ON DELETE CASCADE,
	request_item_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_request_items(id) ON DELETE CASCADE,
	request_order_id BIGINT NOT NULL REFERENCES %[1]s.customer_direct_ship_request_orders(id) ON DELETE CASCADE,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	order_item_id BIGINT NOT NULL REFERENCES %[1]s.order_items(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse_code TEXT NOT NULL DEFAULT '',
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	allocated_qty BIGINT NOT NULL DEFAULT 0,
	allocated_units BIGINT NOT NULL DEFAULT 0,
	allocated_g BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS customer_direct_ship_request_alloc_request_idx
	ON %[1]s.customer_direct_ship_request_allocations(request_id, status, id);
CREATE INDEX IF NOT EXISTS customer_direct_ship_request_alloc_batch_idx
	ON %[1]s.customer_direct_ship_request_allocations(batch_id, warehouse_code, status);

ALTER TABLE %[1]s.customer_direct_ship_request_items ADD COLUMN IF NOT EXISTS spec_label TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.customer_direct_ship_request_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_direct_ship_request_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_direct_ship_request_items ADD COLUMN IF NOT EXISTS bom_spec_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.customer_direct_ship_request_items ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.customer_direct_ship_request_allocations ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_direct_ship_request_allocations ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_direct_ship_request_allocations ADD COLUMN IF NOT EXISTS allocated_units BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS %[1]s.order_stock_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	need_g BIGINT NOT NULL DEFAULT 0,
	batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	allocated_g BIGINT NOT NULL DEFAULT 0,
	allocated_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS order_item_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS request_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS need_units BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS allocated_units BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS customer_direct_ship_order_stock_request_idx
	ON %[1]s.order_stock_batch_allocations(request_id, order_id);
`, schema)
	_, err := pool.Exec(ctx, q)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.customer_direct_ship_import_order_items ADD COLUMN IF NOT EXISTS product_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customer_direct_ship_import_order_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customer_direct_ship_import_order_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.customer_direct_ship_request_items DROP CONSTRAINT IF EXISTS customer_direct_ship_request_items_request_id_product_id_spec_g_key`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS customer_direct_ship_request_items_legacy_identity_uq ON %s.customer_direct_ship_request_items(request_id,product_id,spec_g) WHERE bom_spec_id=0`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS customer_direct_ship_request_items_bom_spec_identity_uq ON %s.customer_direct_ship_request_items(request_id,product_id,bom_spec_id) WHERE bom_spec_id>0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_product_alias_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_product_display_name_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS customer_item_code_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS product_code_snapshot TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.order_items ADD COLUMN IF NOT EXISTS product_name_snapshot TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	if err := backfillSubmittedDirectShipERPOrders(ctx, pool, schema); err != nil {
		return err
	}
	if err := repairSubmittedDirectShipERPOrderReceivers(ctx, pool, schema); err != nil {
		return err
	}
	return repairSubmittedDirectShipERPOrderDiscounts(ctx, pool, schema)
}
