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
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS roast_level TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_category_id BIGINT;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_category_position INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS base_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'public';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS custom_type TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS margin_rate_override NUMERIC(14,6);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_kind TEXT;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS green_bean_type TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS green_bean_bom_product_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_bag_grams NUMERIC(12,3);
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS drip_box_bag_count INT;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS allow_fulfillment_order BOOLEAN;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS allow_mall_order BOOLEAN;
UPDATE %[1]s.products SET visibility='public' WHERE COALESCE(visibility,'')='';
UPDATE %[1]s.products SET product_kind='roasted_bean' WHERE COALESCE(product_kind,'')='';
UPDATE %[1]s.products SET drip_bag_grams = 10 WHERE drip_bag_grams IS NULL;
UPDATE %[1]s.products SET drip_box_bag_count = 10 WHERE drip_box_bag_count IS NULL;
UPDATE %[1]s.products SET allow_fulfillment_order = true WHERE allow_fulfillment_order IS NULL;
UPDATE %[1]s.products SET allow_mall_order = false WHERE allow_mall_order IS NULL;
ALTER TABLE %[1]s.products ALTER COLUMN product_kind SET DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.products ALTER COLUMN drip_bag_grams SET DEFAULT 10;
ALTER TABLE %[1]s.products ALTER COLUMN drip_box_bag_count SET DEFAULT 10;
ALTER TABLE %[1]s.products ALTER COLUMN allow_fulfillment_order SET DEFAULT true;
ALTER TABLE %[1]s.products ALTER COLUMN allow_mall_order SET DEFAULT false;
ALTER TABLE %[1]s.products ALTER COLUMN product_kind SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN drip_bag_grams SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN drip_box_bag_count SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN allow_fulfillment_order SET NOT NULL;
ALTER TABLE %[1]s.products ALTER COLUMN allow_mall_order SET NOT NULL;
ALTER TABLE %[1]s.products DROP CONSTRAINT IF EXISTS products_name_key;
DROP INDEX IF EXISTS %[1]s.products_name_key;
CREATE INDEX IF NOT EXISTS products_customer_visibility_idx ON %[1]s.products(customer_id, visibility, active);
CREATE INDEX IF NOT EXISTS products_base_product_idx ON %[1]s.products(base_product_id);
CREATE INDEX IF NOT EXISTS products_kind_active_idx ON %[1]s.products(product_kind, active);
CREATE TABLE IF NOT EXISTS %[1]s.product_categories (
	id BIGSERIAL PRIMARY KEY,
	parent_id BIGINT,
	customer_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	level INT NOT NULL DEFAULT 1,
	position INT NOT NULL DEFAULT 1,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS gradient_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS source_category_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS template_state TEXT NOT NULL DEFAULT 'customer_owned';
CREATE TABLE IF NOT EXISTS %[1]s.pricing_gradient_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	display_unit TEXT NOT NULL DEFAULT 'lb',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS customer_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS source_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS template_state TEXT NOT NULL DEFAULT 'customer_owned';
DROP INDEX IF EXISTS %[1]s.pricing_gradient_templates_name_active_uniq;
CREATE UNIQUE INDEX IF NOT EXISTS pricing_gradient_templates_customer_name_active_uniq
ON %[1]s.pricing_gradient_templates (customer_id, lower(name))
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.pricing_gradient_template_tiers (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL REFERENCES %[1]s.pricing_gradient_templates(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	min_weight_g NUMERIC(14,3) NOT NULL DEFAULT 0,
	max_weight_g NUMERIC(14,3) NULL,
	margin_rate NUMERIC(14,6) NOT NULL DEFAULT 0,
	position INT NOT NULL DEFAULT 1,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS pricing_gradient_template_tiers_template_idx
ON %[1]s.pricing_gradient_template_tiers(template_id, active, position, id);
DROP INDEX IF EXISTS %[1]s.product_categories_parent_name_uniq;
DROP INDEX IF EXISTS %[1]s.product_categories_customer_parent_name_uniq;
CREATE UNIQUE INDEX product_categories_customer_parent_name_uniq
ON %[1]s.product_categories (customer_id, COALESCE(parent_id,0), lower(name))
WHERE active=true;
CREATE UNIQUE INDEX IF NOT EXISTS product_categories_customer_source_active_uniq
ON %[1]s.product_categories (customer_id, source_category_id)
WHERE active=true AND source_category_id > 0;
CREATE UNIQUE INDEX IF NOT EXISTS pricing_gradient_templates_customer_source_active_uniq
ON %[1]s.pricing_gradient_templates (customer_id, source_template_id)
WHERE active=true AND source_template_id > 0;
UPDATE %[1]s.product_categories child
SET parent_id=active_parent.id, updated_at=now()
FROM %[1]s.product_categories inactive_parent
JOIN %[1]s.product_categories active_parent
  ON active_parent.active=true
 AND active_parent.customer_id=inactive_parent.customer_id
 AND COALESCE(active_parent.parent_id,0)=COALESCE(inactive_parent.parent_id,0)
 AND lower(active_parent.name)=lower(inactive_parent.name)
WHERE child.active=true
  AND child.parent_id=inactive_parent.id
  AND inactive_parent.active=false;
UPDATE %[1]s.product_categories parent
SET active=true, updated_at=now()
WHERE parent.active=false
  AND EXISTS (
    SELECT 1 FROM %[1]s.product_categories child
    WHERE child.parent_id=parent.id AND child.active=true
  )
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.product_categories active_parent
    WHERE active_parent.active=true
      AND active_parent.customer_id=parent.customer_id
      AND COALESCE(active_parent.parent_id,0)=COALESCE(parent.parent_id,0)
      AND lower(active_parent.name)=lower(parent.name)
  );
CREATE TABLE IF NOT EXISTS %[1]s.customer_sku_public_usage (
	customer_id BIGINT PRIMARY KEY,
	use_public_sku BOOLEAN NOT NULL DEFAULT false,
	use_public_categories BOOLEAN NOT NULL DEFAULT false,
	use_public_gradient_templates BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.customer_sku_public_usage ADD COLUMN IF NOT EXISTS use_public_gradient_templates BOOLEAN NOT NULL DEFAULT false;
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
UPDATE %[1]s.product_categories
SET template_state = CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE COALESCE(NULLIF(template_state,''),'customer_owned') END
WHERE active=true;
UPDATE %[1]s.pricing_gradient_templates
SET template_state = CASE WHEN COALESCE(customer_id,0)=0 THEN 'public_template' ELSE COALESCE(NULLIF(template_state,''),'customer_owned') END
WHERE active=true;
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
