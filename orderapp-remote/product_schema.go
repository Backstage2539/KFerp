package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ensureProductPricingColumns(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_227g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_100g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_200g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_250g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS spec_g BIGINT;
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS min_qty_units NUMERIC;
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS max_qty_units NUMERIC;
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS price_per_unit NUMERIC(12,2);
UPDATE %[1]s.products
SET retail_price_227g = COALESCE(default_price, 0)
WHERE retail_price_227g IS NULL;
UPDATE %[1]s.products SET retail_price_100g = 0 WHERE retail_price_100g IS NULL;
UPDATE %[1]s.products SET retail_price_200g = 0 WHERE retail_price_200g IS NULL;
UPDATE %[1]s.products SET retail_price_250g = 0 WHERE retail_price_250g IS NULL;
UPDATE %[1]s.products p
SET roast_level = CASE
	WHEN COALESCE(b.yield_rate, 0) >= 0.8199 THEN '浅烘'
	WHEN COALESCE(b.yield_rate, 0) >= 0.8149 THEN '中烘'
	WHEN COALESCE(b.yield_rate, 0) >= 0.8099 THEN '中深烘'
	WHEN COALESCE(b.yield_rate, 0) > 0 THEN '深烘'
	ELSE roast_level
END
FROM %[1]s.product_bom b
WHERE b.product_id = p.id
  AND COALESCE(NULLIF(p.roast_level,''), '') = '';
UPDATE %[1]s.products SET roast_level = '深烘' WHERE COALESCE(NULLIF(roast_level,''), '') = '';
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_100g SET DEFAULT 0;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_100g SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_200g SET DEFAULT 0;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_200g SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_227g SET DEFAULT 0;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_227g SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_250g SET DEFAULT 0;
ALTER TABLE %[1]s.products ALTER COLUMN retail_price_250g SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN roast_level SET DEFAULT '';
ALTER TABLE %[1]s.products ALTER COLUMN roast_level SET NOT NULL;
UPDATE %[1]s.product_price_tiers
SET spec_g = 454
WHERE spec_g IS NULL OR spec_g <= 0;
UPDATE %[1]s.product_price_tiers
SET min_qty_units = COALESCE(min_qty_units, min_qty_lb),
    max_qty_units = COALESCE(max_qty_units, max_qty_lb),
    price_per_unit = COALESCE(price_per_unit, price_per_lb)
WHERE min_qty_units IS NULL OR price_per_unit IS NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN spec_g SET DEFAULT 454;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN spec_g SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN min_qty_units SET DEFAULT 0;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN min_qty_units SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_per_unit SET DEFAULT 0;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_per_unit SET NOT NULL;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
