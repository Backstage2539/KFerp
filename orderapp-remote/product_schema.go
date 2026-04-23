package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductPricingColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS retail_price_227g NUMERIC(12,2);
UPDATE %s.products
SET retail_price_227g = COALESCE(default_price, 0)
WHERE retail_price_227g IS NULL;
ALTER TABLE %s.products ALTER COLUMN retail_price_227g SET DEFAULT 0;
ALTER TABLE %s.products ALTER COLUMN retail_price_227g SET NOT NULL;
`, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
