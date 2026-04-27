package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProduceBatchTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMachineCapacityTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionRunTable(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureProductionLogTable(ctx, pool, schema); err != nil {
		return err
	}
	return ensureWorkOrderTables(ctx, pool, schema)
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
	completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS work_orders_status_idx ON %s.work_orders(status, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.job_cards (
	id BIGSERIAL PRIMARY KEY,
	work_order_id BIGINT NOT NULL,
	operation TEXT NOT NULL DEFAULT '',
	workstation TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'running',
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ,
	operator TEXT NOT NULL DEFAULT ''
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
`, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
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
	CREATE INDEX IF NOT EXISTS produce_running_items_status_idx ON %s.produce_running_items(status, started_at DESC);`, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS input_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_loose_g BIGINT NOT NULL DEFAULT 0`, schema))
	return nil
}

func ensureProductionLogTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.production_logs (
		id BIGSERIAL PRIMARY KEY,
		running_item_id BIGINT NOT NULL UNIQUE,
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
	_, err := pool.Exec(ctx, q)
	return err
}
