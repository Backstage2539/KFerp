package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
