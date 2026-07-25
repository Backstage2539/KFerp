package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureProductCompatibilityColumns(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureStockEntryTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProduceBatchTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMachineCapacityTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureOperationTemplateTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionRunTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionLogTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureWorkOrderTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureQualityInspectionTables(ctx, pool, schema); err != nil {
		return err
	}
	return backfillQualityStatusesFromInspections(ctx, pool, schema)
}

func ensureProductCompatibilityColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, fmt.Sprintf(`
DO $$
BEGIN
	IF to_regclass('%[1]s.products') IS NOT NULL THEN
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT '';
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
	END IF;
END $$;`, schema))
	return err
}

func ensureMachineCapacityTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.roast_machines (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		capacity_g BIGINT NOT NULL,
		allowed_specs TEXT NOT NULL DEFAULT '',
		min_roast_g BIGINT NOT NULL DEFAULT 0,
		active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return nil
}

func ensureOperationTemplateTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.operation_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.operation_template_steps (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL DEFAULT 0,
	position INTEGER NOT NULL DEFAULT 1,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	cost_type TEXT NOT NULL DEFAULT '',
	cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS operation_template_steps_template_idx ON %s.operation_template_steps(template_id, position, id);
`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureWorkOrderTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.production_plans (
	id BIGSERIAL PRIMARY KEY,
	plan_no TEXT NOT NULL UNIQUE,
	source_type TEXT NOT NULL DEFAULT 'manual',
	status TEXT NOT NULL DEFAULT 'draft',
	from_date DATE,
	to_date DATE,
	customer_id BIGINT NOT NULL DEFAULT 0,
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	submitted_by TEXT NOT NULL DEFAULT '',
	submitted_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	cancelled_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS production_plans_status_idx ON %s.production_plans(status, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.production_plan_items (
	id BIGSERIAL PRIMARY KEY,
	production_plan_id BIGINT NOT NULL,
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	planned_g BIGINT NOT NULL DEFAULT 0,
	planned_output_g BIGINT NOT NULL DEFAULT 0,
	gap_g BIGINT NOT NULL DEFAULT 0,
	order_nos TEXT NOT NULL DEFAULT '',
	bom_version_id BIGINT NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	process_route_id BIGINT NOT NULL DEFAULT 0,
	component_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	process_route_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	production_config_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	customer_product_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS production_plan_items_plan_idx ON %s.production_plan_items(production_plan_id, id);

CREATE TABLE IF NOT EXISTS %s.production_plan_operation_splits (
	id BIGSERIAL PRIMARY KEY,
	production_plan_id BIGINT NOT NULL,
	production_plan_item_id BIGINT NOT NULL,
	operation_seq INT NOT NULL DEFAULT 0,
	operation_id BIGINT NOT NULL DEFAULT 0,
	operation TEXT NOT NULL DEFAULT '',
	workstation_id BIGINT NOT NULL DEFAULT 0,
	workstation TEXT NOT NULL DEFAULT '',
	workstation_capacity_id BIGINT NOT NULL DEFAULT 0,
	workstation_capacity_name TEXT NOT NULL DEFAULT '',
	batch_size_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	batch_size_unit TEXT NOT NULL DEFAULT '',
	standard_minutes INT NOT NULL DEFAULT 0,
	hourly_rate NUMERIC(14,4) NOT NULL DEFAULT 0,
	planned_batch_count INT NOT NULL DEFAULT 0,
	planned_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	planned_qty_g BIGINT NOT NULL DEFAULT 0,
	planned_minutes INT NOT NULL DEFAULT 0,
	planned_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS production_plan_operation_splits_plan_idx ON %s.production_plan_operation_splits(production_plan_id, id);
CREATE INDEX IF NOT EXISTS production_plan_operation_splits_item_operation_idx ON %s.production_plan_operation_splits(production_plan_item_id, operation_seq, id);

CREATE TABLE IF NOT EXISTS %s.work_orders (
	id BIGSERIAL PRIMARY KEY,
	work_order_no TEXT NOT NULL UNIQUE,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	production_plan_id BIGINT NOT NULL DEFAULT 0,
	production_plan_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	planned_g BIGINT NOT NULL DEFAULT 0,
	planned_output_g BIGINT NOT NULL DEFAULT 0,
	order_nos TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'running',
	actual_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ,
	material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
	bom_version_id BIGINT NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	process_template_id BIGINT NOT NULL DEFAULT 0,
	process_template_name TEXT NOT NULL DEFAULT '',
	process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	production_config_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	customer_product_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	planned_start_at TIMESTAMPTZ,
	planned_end_at TIMESTAMPTZ,
	shift_code TEXT NOT NULL DEFAULT '',
	assigned_to TEXT NOT NULL DEFAULT '',
	priority INT NOT NULL DEFAULT 0,
	scheduling_note TEXT NOT NULL DEFAULT '',
	work_center TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS work_orders_status_idx ON %s.work_orders(status, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS work_orders_running_item_started_uq ON %s.work_orders(running_item_id) WHERE running_item_id > 0;

CREATE TABLE IF NOT EXISTS %s.job_cards (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL,
	sequence_no INT NOT NULL DEFAULT 1,
	operation_id BIGINT NOT NULL DEFAULT 0,
	workstation_id BIGINT NOT NULL DEFAULT 0,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	workstation_capacity_id BIGINT NOT NULL DEFAULT 0,
	workstation_capacity_name TEXT NOT NULL DEFAULT '',
	batch_size_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	batch_size_unit TEXT NOT NULL DEFAULT '',
	planned_batch_count INT NOT NULL DEFAULT 0,
	planned_minutes INT NOT NULL DEFAULT 0,
	hourly_rate NUMERIC(14,4) NOT NULL DEFAULT 0,
	planned_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_minutes INT NOT NULL DEFAULT 0,
	actual_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	paused_at TIMESTAMPTZ,
	resumed_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	operator TEXT NOT NULL DEFAULT '',
	planned_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	records_loss BOOLEAN NOT NULL DEFAULT false,
	loss_reason TEXT NOT NULL DEFAULT '',
	exception_reason TEXT NOT NULL DEFAULT '',
	metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	operation_template_step_id BIGINT NOT NULL DEFAULT 0,
	cost_type TEXT NOT NULL DEFAULT '',
	cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0,
	planned_start_at TIMESTAMPTZ,
	planned_end_at TIMESTAMPTZ,
	shift_code TEXT NOT NULL DEFAULT '',
	assigned_to TEXT NOT NULL DEFAULT '',
	priority INT NOT NULL DEFAULT 0,
	scheduling_note TEXT NOT NULL DEFAULT '',
	work_center TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS job_cards_work_order_idx ON %s.job_cards(work_order_id, id);
CREATE INDEX IF NOT EXISTS job_cards_status_idx ON %s.job_cards(status, started_at DESC);

CREATE TABLE IF NOT EXISTS %s.production_batch_costs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL,
	batch_id TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	material_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	operation_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	total_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	finished_g BIGINT NOT NULL DEFAULT 0,
	unit_cost_per_kg NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS production_batch_costs_running_item_uq ON %s.production_batch_costs(running_item_id);
CREATE INDEX IF NOT EXISTS production_batch_costs_created_idx ON %s.production_batch_costs(created_at DESC);

CREATE TABLE IF NOT EXISTS %s.work_order_material_reservations (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL DEFAULT 0,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	material_name TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '',
	required_g BIGINT NOT NULL DEFAULT 0,
	required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,
	reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,
	returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS work_order_material_reservations_running_idx ON %s.work_order_material_reservations(running_item_id, status);
CREATE INDEX IF NOT EXISTS work_order_material_reservations_material_idx ON %s.work_order_material_reservations(material_id, status);

CREATE TABLE IF NOT EXISTS %s.work_center_capacity_calendar (
	id BIGSERIAL PRIMARY KEY,
	work_center TEXT NOT NULL DEFAULT '',
	work_date DATE NOT NULL,
	shift_code TEXT NOT NULL DEFAULT '',
	available_minutes INT NOT NULL DEFAULT 0,
	downtime_minutes INT NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS work_center_capacity_calendar_uq ON %s.work_center_capacity_calendar(work_center, work_date, shift_code);
CREATE INDEX IF NOT EXISTS work_center_capacity_calendar_lookup_idx ON %s.work_center_capacity_calendar(work_date, work_center);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.work_orders DROP CONSTRAINT IF EXISTS work_orders_running_item_id_key`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ALTER COLUMN running_item_id SET DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS production_plan_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS production_plan_item_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS planned_output_g BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS order_nos TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS bom_version_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS operation_template_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_template_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_template_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS production_config_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS customer_product_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS planned_start_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS planned_end_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS shift_code TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS assigned_to TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS scheduling_note TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS work_center TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS sequence_no INT NOT NULL DEFAULT 1`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS operation_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS workstation_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS workstation_capacity_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS workstation_capacity_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS batch_size_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS batch_size_unit TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_batch_count INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_minutes INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS hourly_rate NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_minutes INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ALTER COLUMN status SET DEFAULT 'pending'`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS paused_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS resumed_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_loss_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS records_loss BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS loss_reason TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS exception_reason TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS operation_template_step_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS cost_type TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_start_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_end_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS shift_code TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS assigned_to TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS scheduling_note TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS work_center TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS work_orders_running_item_started_uq ON %s.work_orders(running_item_id) WHERE running_item_id > 0`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureStockEntryTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_entries (
	id BIGSERIAL PRIMARY KEY,
	entry_no TEXT NOT NULL UNIQUE,
	entry_type TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	work_order_id BIGINT NOT NULL DEFAULT 0,
	job_card_id BIGINT NOT NULL DEFAULT 0,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	source_type TEXT NOT NULL DEFAULT '',
	source_id BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_entries_work_order_idx ON %s.stock_entries(work_order_id, created_at DESC);
CREATE INDEX IF NOT EXISTS stock_entries_type_idx ON %s.stock_entries(entry_type, status, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_entry_items (
	id BIGSERIAL PRIMARY KEY,
	stock_entry_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,
	item_type TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	from_warehouse TEXT NOT NULL DEFAULT '',
	to_warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL DEFAULT '',
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	total_cost NUMERIC(12,4) NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS stock_entry_items_entry_idx ON %s.stock_entry_items(stock_entry_id, id);
`, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS is_return BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS return_source TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS legacy BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS work_order_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS job_card_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS running_item_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entries ADD COLUMN IF NOT EXISTS source_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS from_warehouse TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS to_warehouse TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS total_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS supplier TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS crop_season TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.stock_entry_items ADD COLUMN IF NOT EXISTS producer_flavor_description TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func ensureProductionRunTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.produce_running_items (
		id BIGSERIAL PRIMARY KEY,
		batch_id TEXT NOT NULL,
		product_id BIGINT NOT NULL,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL,
		need_g BIGINT NOT NULL,
		order_nos TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'running',
		started_by TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		finished_by TEXT,
		finished_at TIMESTAMPTZ
	);
	CREATE INDEX IF NOT EXISTS produce_running_items_status_idx ON %s.produce_running_items(status, started_at DESC);
	CREATE TABLE IF NOT EXISTS %s.produce_running_outputs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL REFERENCES %s.produce_running_items(id) ON DELETE CASCADE,
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		need_g BIGINT NOT NULL DEFAULT 0,
		order_nos TEXT NOT NULL DEFAULT '',
		planned_units BIGINT NOT NULL DEFAULT 0,
		planned_loose_g BIGINT NOT NULL DEFAULT 0,
		finished_units BIGINT NOT NULL DEFAULT 0,
		finished_loose_g BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		UNIQUE(running_item_id, product_id, spec_g)
	);
	CREATE INDEX IF NOT EXISTS produce_running_outputs_running_idx ON %s.produce_running_outputs(running_item_id, spec_g);`, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS input_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_loose_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS operation_template_id BIGINT NOT NULL DEFAULT 0`, schema))
	return nil
}

func ensureProductionLogTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.production_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL,
		completion_no BIGINT NOT NULL DEFAULT 1,
		batch_id TEXT NOT NULL DEFAULT '',
		product_id BIGINT NOT NULL DEFAULT 0,
		product_name TEXT NOT NULL DEFAULT '',
		spec_g BIGINT NOT NULL DEFAULT 0,
		order_nos TEXT NOT NULL DEFAULT '',
		planned_need_g BIGINT NOT NULL DEFAULT 0,
		input_g BIGINT NOT NULL DEFAULT 0,
		bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
		finished_units BIGINT NOT NULL DEFAULT 0,
		finished_loose_g BIGINT NOT NULL DEFAULT 0,
		finished_total_g BIGINT NOT NULL DEFAULT 0,
		actual_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
		started_by TEXT NOT NULL DEFAULT '',
		started_at TIMESTAMPTZ,
		finished_by TEXT NOT NULL DEFAULT '',
		finished_at TIMESTAMPTZ,
		inventory_units_before BIGINT NOT NULL DEFAULT 0,
		inventory_loose_g_before BIGINT NOT NULL DEFAULT 0,
		inventory_units_after BIGINT NOT NULL DEFAULT 0,
		inventory_loose_g_after BIGINT NOT NULL DEFAULT 0,
		material_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS production_logs_finished_idx ON %s.production_logs(finished_at DESC, id DESC);
	CREATE INDEX IF NOT EXISTS production_logs_product_idx ON %s.production_logs(product_id, finished_at DESC);`, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.production_logs DROP CONSTRAINT IF EXISTS production_logs_running_item_id_key`, schema)); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.production_logs ADD COLUMN IF NOT EXISTS completion_no BIGINT NOT NULL DEFAULT 1`, schema)); err != nil {
		return err
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE INDEX IF NOT EXISTS production_logs_running_completion_idx ON %s.production_logs(running_item_id, completion_no)`, schema))
	return err
}

func ensureQualityInspectionTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.quality_inspections (
	id BIGSERIAL PRIMARY KEY,
	scope TEXT NOT NULL DEFAULT '',
	reference_type TEXT NOT NULL DEFAULT '',
	reference_no TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT 'hold',
	metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	note TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS quality_inspections_ref_idx ON %s.quality_inspections(reference_type, reference_no, created_at DESC);
CREATE INDEX IF NOT EXISTS quality_inspections_scope_idx ON %s.quality_inspections(scope, result, created_at DESC);
`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
