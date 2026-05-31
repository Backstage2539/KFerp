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
CREATE TABLE IF NOT EXISTS %s.work_orders (
	id BIGSERIAL PRIMARY KEY,
	work_order_no TEXT NOT NULL UNIQUE,
	running_item_id BIGINT NOT NULL UNIQUE,
	batch_id TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	planned_g BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'running',
	actual_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ,
	material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	process_template_id BIGINT NOT NULL DEFAULT 0,
	process_template_name TEXT NOT NULL DEFAULT '',
	process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	customer_product_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE INDEX IF NOT EXISTS work_orders_status_idx ON %s.work_orders(status, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.job_cards (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL,
	sequence_no INT NOT NULL DEFAULT 1,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'running',
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ,
	operator TEXT NOT NULL DEFAULT '',
	planned_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	records_loss BOOLEAN NOT NULL DEFAULT false,
	exception_reason TEXT NOT NULL DEFAULT '',
	metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	operation_template_step_id BIGINT NOT NULL DEFAULT 0,
	cost_type TEXT NOT NULL DEFAULT '',
	cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS job_cards_work_order_idx ON %s.job_cards(work_order_id, id);
CREATE INDEX IF NOT EXISTS job_cards_status_idx ON %s.job_cards(status, started_at DESC);

CREATE TABLE IF NOT EXISTS %s.production_batch_costs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL UNIQUE,
	batch_id TEXT NOT NULL DEFAULT '',
	product_name TEXT NOT NULL DEFAULT '',
	material_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	operation_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	total_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	finished_g BIGINT NOT NULL DEFAULT 0,
	unit_cost_per_kg NUMERIC(12,4) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
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
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS operation_template_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_template_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_template_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS operation_summary_json JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.work_orders ADD COLUMN IF NOT EXISTS customer_product_snapshot_json JSONB NOT NULL DEFAULT '[]'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS sequence_no INT NOT NULL DEFAULT 1`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS planned_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_input_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_output_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_loss_qty NUMERIC(14,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS actual_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS records_loss BOOLEAN NOT NULL DEFAULT false`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS exception_reason TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS parameter_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS operation_template_step_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS cost_type TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.job_cards ADD COLUMN IF NOT EXISTS cost_rate NUMERIC(12,4) NOT NULL DEFAULT 0`, schema),
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
