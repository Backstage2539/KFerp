package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_227g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_100g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_200g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS retail_price_250g NUMERIC(12,2);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_category_id BIGINT;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_category_position INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS base_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'public';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS custom_type TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.products SET visibility='public' WHERE COALESCE(visibility,'')='';
CREATE INDEX IF NOT EXISTS products_customer_visibility_idx ON %[1]s.products(customer_id, visibility, active);
CREATE INDEX IF NOT EXISTS products_base_product_idx ON %[1]s.products(base_product_id);
CREATE TABLE IF NOT EXISTS %[1]s.product_categories (
	id BIGSERIAL PRIMARY KEY,
	parent_id BIGINT,
	customer_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	level INT NOT NULL DEFAULT 1,
	position INT NOT NULL DEFAULT 1,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
DROP INDEX IF EXISTS %[1]s.product_categories_parent_name_uniq;
CREATE UNIQUE INDEX IF NOT EXISTS product_categories_customer_parent_name_uniq
ON %[1]s.product_categories (customer_id, COALESCE(parent_id,0), lower(name))
WHERE active=true;
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position)
SELECT NULL,0,'咖啡豆',1,1
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='咖啡豆');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position)
SELECT NULL,0,'挂耳',1,2
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='挂耳');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position)
SELECT p.id,p.customer_id,'意式拼配',2,1 FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='咖啡豆'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='意式拼配');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position)
SELECT p.id,p.customer_id,'单品豆',2,2 FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='咖啡豆'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='单品豆');
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
