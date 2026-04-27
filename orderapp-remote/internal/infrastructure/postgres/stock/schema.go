package stock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		return err
	}
	if err := ensureMaterialBatchTables(ctx, pool, schema); err != nil {
		return err
	}
	return ensureStockAdjustmentTables(ctx, pool, schema)
}

func ensureStockLedgerTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS stock_batches_source_uq
	ON %s.stock_batches(source_doc_type, source_doc_id, item_type, item_id, spec_g)
	WHERE source_doc_type <> '';
CREATE INDEX IF NOT EXISTS stock_batches_item_idx
	ON %s.stock_batches(item_type, item_id, spec_g, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS stock_ledger_source_idx
	ON %s.stock_ledger_entries(source_doc_type, source_doc_id, id);
CREATE INDEX IF NOT EXISTS stock_ledger_item_idx
	ON %s.stock_ledger_entries(item_type, item_id, spec_g, created_at DESC);
`, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_g BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS remaining_units BIGINT NOT NULL DEFAULT 0`, schema))
	_, _ = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.stock_batches ADD COLUMN IF NOT EXISTS unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0`, schema))
	return nil
}

func ensureMaterialBatchTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.material_receipts (
	id BIGSERIAL PRIMARY KEY,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS material_receipts_material_idx
	ON %s.material_receipts(material_id, created_at DESC);

CREATE TABLE IF NOT EXISTS %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	receipt_id BIGINT NOT NULL DEFAULT 0,
	qty_g BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT NOT NULL DEFAULT '',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS material_batches_material_fifo_idx
	ON %s.material_batches(material_id, status, received_at, id);
`, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func ensureStockAdjustmentTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.stock_adjustments (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'submitted',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.stock_adjustment_items (
	id BIGSERIAL PRIMARY KEY,
	adjustment_id BIGINT NOT NULL,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS stock_adjustments_item_idx
	ON %s.stock_adjustments(item_type, item_id, created_at DESC);
`, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
