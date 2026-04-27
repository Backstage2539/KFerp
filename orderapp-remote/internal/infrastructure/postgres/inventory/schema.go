package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureFinishedInventoryTable(ctx, pool, schema); err != nil {
		return err
	}
	return ensureFinishedAllocationLogTable(ctx, pool, schema)
}

func ensureFinishedInventoryTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.finished_inventory (
		product_id BIGINT NOT NULL,
		spec_g BIGINT NOT NULL,
		warehouse TEXT NOT NULL DEFAULT 'finished_goods',
		onhand_units BIGINT NOT NULL DEFAULT 0,
		onhand_loose_g BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY(product_id, spec_g, warehouse)
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
ALTER TABLE %s.finished_inventory ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT 'finished_goods';
UPDATE %s.finished_inventory SET warehouse='finished_goods' WHERE COALESCE(warehouse,'')='';
DO $$
DECLARE
	pk_name TEXT;
BEGIN
	SELECT c.conname INTO pk_name
	FROM pg_constraint c
	WHERE c.conrelid = '%s.finished_inventory'::regclass
	  AND c.contype = 'p';
	IF pk_name IS NOT NULL THEN
		EXECUTE format('ALTER TABLE %s.finished_inventory DROP CONSTRAINT %%I', pk_name);
	END IF;
	ALTER TABLE %s.finished_inventory ADD PRIMARY KEY(product_id, spec_g, warehouse);
EXCEPTION WHEN duplicate_object THEN
	NULL;
END $$;
CREATE INDEX IF NOT EXISTS finished_inventory_warehouse_idx
	ON %s.finished_inventory(warehouse, product_id, spec_g);
`, schema, schema, schema, schema, schema, schema)); err != nil {
		return err
	}
	return nil
}

func ensureFinishedAllocationLogTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.finished_allocation_logs (
		id BIGSERIAL PRIMARY KEY,
		batch_id TEXT NOT NULL,
		product_id BIGINT NOT NULL,
		spec_g BIGINT NOT NULL,
		need_g BIGINT NOT NULL,
		deducted_g BIGINT NOT NULL,
		gap_g BIGINT NOT NULL,
		operator TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE INDEX IF NOT EXISTS finished_allocation_logs_batch_id_idx ON %s.finished_allocation_logs(batch_id);
	CREATE INDEX IF NOT EXISTS finished_allocation_logs_prod_spec_idx ON %s.finished_allocation_logs(product_id, spec_g);
	`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
