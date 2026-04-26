package production

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProduceBatchSummaryItem struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	SpecG       int64  `json:"spec_g"`
	NeedUnits   int64  `json:"need_units"`
	NeedG       int64  `json:"need_g"`
	DeductedG   int64  `json:"deducted_g"`
	GapG        int64  `json:"gap_g"`
}

type ProduceBatchCreateResult struct {
	BatchID    string                    `json:"batch_id"`
	OrderCount int                       `json:"order_count"`
	Summary    []ProduceBatchSummaryItem `json:"summary"`
}

func ensureProduceBatchTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.produce_batches (
	batch_id TEXT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'planned',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %s.produce_batch_items (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	need_units BIGINT NOT NULL,
	need_g BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(batch_id, product_id, spec_g)
);
CREATE TABLE IF NOT EXISTS %s.produce_batch_order_items (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	order_id BIGINT NOT NULL,
	order_item_id BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %s.produce_batch_order_items DROP CONSTRAINT IF EXISTS produce_batch_order_items_order_item_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS produce_batch_order_items_batch_item_uq ON %s.produce_batch_order_items(batch_id, order_item_id);
-- DEV-044 v2: allow partial allocation across multiple batches
CREATE TABLE IF NOT EXISTS %s.produce_batch_allocations (
	id BIGSERIAL PRIMARY KEY,
	batch_id TEXT NOT NULL,
	order_id BIGINT NOT NULL,
	order_item_id BIGINT NOT NULL,
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	allocated_units BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(batch_id, order_item_id)
);
CREATE INDEX IF NOT EXISTS produce_batch_items_batch_idx ON %s.produce_batch_items(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_order_items_batch_idx ON %s.produce_batch_order_items(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_allocations_batch_idx ON %s.produce_batch_allocations(batch_id);
CREATE INDEX IF NOT EXISTS produce_batch_allocations_order_item_idx ON %s.produce_batch_allocations(order_item_id);
CREATE TABLE IF NOT EXISTS %s.produce_batch_idempotency (
	idempotency_key TEXT PRIMARY KEY,
	batch_id TEXT NOT NULL,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
