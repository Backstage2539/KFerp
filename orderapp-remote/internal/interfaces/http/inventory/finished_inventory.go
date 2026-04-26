package inventory

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureFinishedInventoryTable(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.finished_inventory (
		product_id BIGINT NOT NULL,
		spec_g BIGINT NOT NULL,
		onhand_units BIGINT NOT NULL DEFAULT 0,
		onhand_loose_g BIGINT NOT NULL DEFAULT 0,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY(product_id, spec_g)
	)`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return nil
}
