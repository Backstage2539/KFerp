package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductPricingColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS retail_price_227g NUMERIC(12,2);
ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS retail_price_100g NUMERIC(12,2);
ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS retail_price_200g NUMERIC(12,2);
ALTER TABLE %s.products ADD COLUMN IF NOT EXISTS retail_price_250g NUMERIC(12,2);
UPDATE %s.products
SET retail_price_227g = COALESCE(default_price, 0)
WHERE retail_price_227g IS NULL;
UPDATE %s.products SET retail_price_100g = 0 WHERE retail_price_100g IS NULL;
UPDATE %s.products SET retail_price_200g = 0 WHERE retail_price_200g IS NULL;
UPDATE %s.products SET retail_price_250g = 0 WHERE retail_price_250g IS NULL;
ALTER TABLE %s.products ALTER COLUMN retail_price_100g SET DEFAULT 0;
ALTER TABLE %s.products ALTER COLUMN retail_price_100g SET NOT NULL;
ALTER TABLE %s.products ALTER COLUMN retail_price_200g SET DEFAULT 0;
ALTER TABLE %s.products ALTER COLUMN retail_price_200g SET NOT NULL;
ALTER TABLE %s.products ALTER COLUMN retail_price_227g SET DEFAULT 0;
ALTER TABLE %s.products ALTER COLUMN retail_price_227g SET NOT NULL;
ALTER TABLE %s.products ALTER COLUMN retail_price_250g SET DEFAULT 0;
ALTER TABLE %s.products ALTER COLUMN retail_price_250g SET NOT NULL;
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
