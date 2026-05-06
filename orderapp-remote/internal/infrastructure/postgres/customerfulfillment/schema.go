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
	product_title TEXT NOT NULL DEFAULT '',
	spec TEXT NOT NULL DEFAULT '',
	quantity_units BIGINT NOT NULL DEFAULT 0,
	payload JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

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
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
