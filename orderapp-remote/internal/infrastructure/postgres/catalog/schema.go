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
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb;
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
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS gradient_template_id_override BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS operation_template_id_override BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS classification_template_id BIGINT NOT NULL DEFAULT 0;
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
CREATE INDEX IF NOT EXISTS products_classification_template_idx ON %[1]s.products(classification_template_id, active);
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
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS source_category_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS template_state TEXT NOT NULL DEFAULT 'customer_owned';
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS operation_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS price_list_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS inventory_unit TEXT NOT NULL DEFAULT 'kg';
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS quote_unit TEXT NOT NULL DEFAULT 'kg';
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS order_unit TEXT NOT NULL DEFAULT 'kg';
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS integer_unit BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.product_unit_definitions (
	code TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	unit_type TEXT NOT NULL DEFAULT 'other',
	allow_decimal BOOLEAN NOT NULL DEFAULT true,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_unit_definitions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
INSERT INTO %[1]s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
VALUES
	('kg','kg','weight',true,true),
	('g','g','weight',true,true),
	('lb','磅','weight',true,true),
	('盒','盒','package',false,true)
ON CONFLICT (code) DO NOTHING;
CREATE TABLE IF NOT EXISTS %[1]s.product_unit_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	inventory_unit TEXT NOT NULL DEFAULT 'kg',
	quote_unit TEXT NOT NULL DEFAULT 'kg',
	order_unit TEXT NOT NULL DEFAULT 'kg',
	unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	integer_unit BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_unit_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS product_unit_templates_name_active_uniq
ON %[1]s.product_unit_templates (lower(name))
WHERE active=true;
INSERT INTO %[1]s.product_unit_templates(name, inventory_unit, quote_unit, order_unit, unit_conversion_json, integer_unit, active)
VALUES ('默认kg单位','kg','kg','kg','{}'::jsonb,false,true)
ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS %[1]s.business_groups (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	code TEXT NOT NULL DEFAULT '',
	remark TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS business_groups_name_active_uniq
ON %[1]s.business_groups(lower(name))
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.business_group_items (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL,
	parent_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	code TEXT NOT NULL DEFAULT '',
	remark TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS business_group_items_group_parent_idx
ON %[1]s.business_group_items(group_id, parent_id, active, sort_order, id);
CREATE UNIQUE INDEX IF NOT EXISTS business_group_items_group_code_active_uniq
ON %[1]s.business_group_items(group_id, lower(code))
WHERE active=true AND code <> '';
CREATE TABLE IF NOT EXISTS %[1]s.business_group_usages (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL,
	usage_key TEXT NOT NULL,
	usage_label TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS business_group_usages_group_key_uq
ON %[1]s.business_group_usages(group_id, lower(usage_key))
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.business_group_assignments (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL,
	group_item_id BIGINT NOT NULL DEFAULT 0,
	usage_key TEXT NOT NULL,
	object_key TEXT NOT NULL,
	object_id BIGINT NOT NULL,
	object_ref TEXT NOT NULL DEFAULT '',
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.business_group_assignments ADD COLUMN IF NOT EXISTS object_ref TEXT NOT NULL DEFAULT '';
DROP INDEX IF EXISTS %[1]s.business_group_assignments_object_uq;
CREATE UNIQUE INDEX IF NOT EXISTS business_group_assignments_object_uq
ON %[1]s.business_group_assignments(group_id, lower(usage_key), lower(object_key), object_id, lower(object_ref));
CREATE INDEX IF NOT EXISTS business_group_assignments_object_ref_idx
ON %[1]s.business_group_assignments(lower(usage_key), lower(object_key), object_id, lower(object_ref));
CREATE TABLE IF NOT EXISTS %[1]s.product_customer_references (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL,
	customer_id BIGINT NOT NULL,
	customer_item_code TEXT NOT NULL DEFAULT '',
	customer_display_name TEXT NOT NULL DEFAULT '',
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS product_customer_references_product_customer_uq
ON %[1]s.product_customer_references(product_id, customer_id)
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.product_pricing_rules (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	code TEXT NOT NULL DEFAULT '',
	cost_source_mode TEXT NOT NULL DEFAULT 'product_cost_context',
	margin_rate NUMERIC(14,6) NOT NULL DEFAULT 0,
	tax_rate NUMERIC(14,6) NOT NULL DEFAULT 0,
	rounding_mode TEXT NOT NULL DEFAULT 'none',
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS product_pricing_rules_active_idx
ON %[1]s.product_pricing_rules(active, id);
CREATE TABLE IF NOT EXISTS %[1]s.price_tier_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS %[1]s.price_tier_template_tiers (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	min_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	max_qty NUMERIC(14,4),
	quantity_unit TEXT NOT NULL DEFAULT 'kg',
	position INT NOT NULL DEFAULT 100,
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS price_tier_template_tiers_template_idx
ON %[1]s.price_tier_template_tiers(template_id, active, position, id);
ALTER TABLE %[1]s.price_tier_template_tiers ADD COLUMN IF NOT EXISTS pricing_rule_id BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.product_price_groups (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	sort_order INT NOT NULL DEFAULT 100,
	active BOOLEAN NOT NULL DEFAULT true,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS product_price_groups_name_active_uniq
ON %[1]s.product_price_groups(lower(name))
WHERE active=true AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS product_price_groups_sort_idx
ON %[1]s.product_price_groups(active, sort_order, id);
CREATE TABLE IF NOT EXISTS %[1]s.product_price_records (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL DEFAULT 0,
	customer_product_alias_id BIGINT NOT NULL DEFAULT 0,
	final_unit_price NUMERIC(14,4) NOT NULL,
	price_unit TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT 'CNY',
	price_group_id BIGINT NOT NULL DEFAULT 0,
	price_group_name TEXT NOT NULL DEFAULT '',
	inventory_unit TEXT NOT NULL DEFAULT 'kg',
	inventory_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	status TEXT NOT NULL DEFAULT 'draft',
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS product_price_records_product_idx
ON %[1]s.product_price_records(product_id, active, status, price_group_id, id);
CREATE INDEX IF NOT EXISTS product_price_records_alias_idx
ON %[1]s.product_price_records(customer_product_alias_id, active, status, price_group_id, id);
CREATE TABLE IF NOT EXISTS %[1]s.product_tier_price_schemes (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	product_id BIGINT NOT NULL DEFAULT 0,
	customer_product_alias_id BIGINT NOT NULL DEFAULT 0,
	price_group_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS product_tier_price_schemes_product_idx
ON %[1]s.product_tier_price_schemes(product_id, active, price_group_id, id);
CREATE INDEX IF NOT EXISTS product_tier_price_schemes_alias_idx
ON %[1]s.product_tier_price_schemes(customer_product_alias_id, active, price_group_id, id);
CREATE TABLE IF NOT EXISTS %[1]s.product_tier_price_scheme_tiers (
	id BIGSERIAL PRIMARY KEY,
	scheme_id BIGINT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	min_qty NUMERIC(14,4) NOT NULL DEFAULT 0,
	max_qty NUMERIC(14,4),
	source_price_record_id BIGINT NOT NULL,
	final_unit_price NUMERIC(14,4) NOT NULL,
	price_unit TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT 'CNY',
	position INT NOT NULL DEFAULT 100,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS product_tier_price_scheme_tiers_scheme_idx
ON %[1]s.product_tier_price_scheme_tiers(scheme_id, active, position, id);
INSERT INTO %[1]s.customers(name, raw_name, customer_type, active, created_at, updated_at)
SELECT '工厂自营', '工厂自营', 'wholesale', true, now(), now()
WHERE NOT EXISTS (
	SELECT 1 FROM %[1]s.customers WHERE active=true AND name='工厂自营'
);
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_aliases (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL,
	product_id BIGINT NOT NULL,
	display_name TEXT NOT NULL,
	customer_item_code TEXT NOT NULL DEFAULT '',
	brand_name TEXT NOT NULL DEFAULT '',
	display_category_id BIGINT NOT NULL DEFAULT 0,
	classification_template_id BIGINT NOT NULL DEFAULT 0,
	product_config_template_id BIGINT NOT NULL DEFAULT 0,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	unit_template_id BIGINT NOT NULL DEFAULT 0,
	sort_order INT NOT NULL DEFAULT 0,
	include_in_price_list BOOLEAN NOT NULL DEFAULT true,
	active BOOLEAN NOT NULL DEFAULT true,
	remark TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.customer_product_aliases ADD COLUMN IF NOT EXISTS classification_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_product_aliases ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_product_aliases ADD COLUMN IF NOT EXISTS gradient_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.customer_product_aliases ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS customer_product_aliases_customer_active_idx
ON %[1]s.customer_product_aliases(customer_id, active, sort_order, id);
CREATE INDEX IF NOT EXISTS customer_product_aliases_product_idx
ON %[1]s.customer_product_aliases(product_id, active);
CREATE INDEX IF NOT EXISTS customer_product_aliases_classification_template_idx
ON %[1]s.customer_product_aliases(classification_template_id, active);
CREATE INDEX IF NOT EXISTS customer_product_aliases_legacy_readonly_idx
ON %[1]s.customer_product_aliases(customer_id, product_id, active);
CREATE TABLE IF NOT EXISTS %[1]s.product_classification_templates (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL DEFAULT 0,
	source_template_id BIGINT NOT NULL DEFAULT 0,
	template_state TEXT NOT NULL DEFAULT 'owned',
	name TEXT NOT NULL,
	remark TEXT NOT NULL DEFAULT '',
	product_config_template_id BIGINT NOT NULL DEFAULT 0,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	unit_template_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS product_classification_templates_customer_name_active_uniq
ON %[1]s.product_classification_templates(customer_id, lower(name))
WHERE active=true;
	CREATE INDEX IF NOT EXISTS product_classification_templates_sort_idx
	ON %[1]s.product_classification_templates(customer_id, active, sort_order, id);
	ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT '';
	ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
	ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS gradient_template_id BIGINT NOT NULL DEFAULT 0;
	ALTER TABLE %[1]s.product_classification_templates ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
CREATE TABLE IF NOT EXISTS %[1]s.product_classification_template_categories (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL,
	parent_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	level INT NOT NULL DEFAULT 1,
	sort_order INT NOT NULL DEFAULT 100,
	product_config_template_id BIGINT NOT NULL DEFAULT 0,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	unit_template_id BIGINT NOT NULL DEFAULT 0,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_classification_template_categories ADD COLUMN IF NOT EXISTS gradient_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_classification_template_categories ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_classification_template_categories ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS product_classification_template_categories_name_uniq
ON %[1]s.product_classification_template_categories(template_id, parent_id, lower(name))
WHERE active=true;
CREATE INDEX IF NOT EXISTS product_classification_template_categories_sort_idx
ON %[1]s.product_classification_template_categories(template_id, active, sort_order, id);
CREATE TABLE IF NOT EXISTS %[1]s.product_classification_template_usages (
	classification_template_id BIGINT PRIMARY KEY,
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS product_classification_template_usages_sort_idx
ON %[1]s.product_classification_template_usages(active, sort_order, classification_template_id);
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_alias_classification_template_usages (
	customer_id BIGINT NOT NULL,
	classification_template_id BIGINT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(customer_id, classification_template_id)
);
CREATE INDEX IF NOT EXISTS customer_product_alias_classification_template_usages_sort_idx
ON %[1]s.customer_product_alias_classification_template_usages(customer_id, active, sort_order, classification_template_id);
CREATE TABLE IF NOT EXISTS %[1]s.product_classification_assignments (
	product_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	category_id BIGINT NOT NULL DEFAULT 0,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(product_id, template_id)
);
CREATE INDEX IF NOT EXISTS product_classification_assignments_template_category_idx
ON %[1]s.product_classification_assignments(template_id, category_id, sort_order, product_id);
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_alias_classification_assignments (
	alias_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	category_id BIGINT NOT NULL DEFAULT 0,
	sort_order INT NOT NULL DEFAULT 100,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(alias_id, template_id)
);
CREATE INDEX IF NOT EXISTS customer_product_alias_classification_assignments_template_category_idx
ON %[1]s.customer_product_alias_classification_assignments(template_id, category_id, sort_order, alias_id);
CREATE TABLE IF NOT EXISTS %[1]s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	production_bom_id BIGINT NOT NULL DEFAULT 0,
	production_bom_version_id BIGINT NOT NULL DEFAULT 0,
	process_route_id BIGINT NOT NULL DEFAULT 0,
	industry_field_template_id BIGINT NOT NULL DEFAULT 0,
	expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT '',
	updated_by TEXT NOT NULL DEFAULT ''
);
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS production_bom_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS production_bom_version_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS process_route_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS industry_field_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS expected_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS created_by TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_production_configs ADD COLUMN IF NOT EXISTS updated_by TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS product_production_configs_bom_idx
ON %[1]s.product_production_configs(production_bom_id, production_bom_version_id);
CREATE INDEX IF NOT EXISTS product_production_configs_process_route_idx
ON %[1]s.product_production_configs(process_route_id);
CREATE TABLE IF NOT EXISTS %[1]s.product_production_config_fields (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	field_type TEXT NOT NULL DEFAULT 'text',
	unit TEXT NOT NULL DEFAULT '',
	value_text TEXT NOT NULL DEFAULT '',
	value_number NUMERIC(14,4),
	value_bool BOOLEAN,
	template_field_key TEXT NOT NULL DEFAULT '',
	required BOOLEAN NOT NULL DEFAULT false,
	options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	show_in_price_list BOOLEAN NOT NULL DEFAULT true,
	sort_order INT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS field_type TEXT NOT NULL DEFAULT 'text';
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS value_text TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS value_number NUMERIC(14,4);
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS value_bool BOOLEAN;
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS template_field_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS options_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS show_in_price_list BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE %[1]s.product_production_config_fields ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS product_production_config_fields_product_key_uq
ON %[1]s.product_production_config_fields(product_id, lower(field_key));
CREATE INDEX IF NOT EXISTS product_production_config_fields_product_sort_idx
ON %[1]s.product_production_config_fields(product_id, sort_order, id);
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_alias_industry_field_values (
	id BIGSERIAL PRIMARY KEY,
	alias_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	value_text TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_product_alias_industry_field_values_alias_key_uq
ON %[1]s.customer_product_alias_industry_field_values(alias_id, lower(field_key));
CREATE INDEX IF NOT EXISTS customer_product_alias_industry_field_values_alias_idx
ON %[1]s.customer_product_alias_industry_field_values(alias_id);
CREATE TABLE IF NOT EXISTS %[1]s.product_config_templates (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL DEFAULT 0,
	source_template_id BIGINT NOT NULL DEFAULT 0,
	template_state TEXT NOT NULL DEFAULT 'customer_owned',
	name TEXT NOT NULL,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	price_list_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	inventory_unit TEXT NOT NULL DEFAULT 'kg',
	quote_unit TEXT NOT NULL DEFAULT 'kg',
	order_unit TEXT NOT NULL DEFAULT 'kg',
	unit_conversion_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	integer_unit BOOLEAN NOT NULL DEFAULT false,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS product_config_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS special_attrs_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE UNIQUE INDEX IF NOT EXISTS product_config_templates_customer_source_active_uniq
ON %[1]s.product_config_templates (customer_id, source_template_id)
WHERE active=true AND source_template_id > 0;
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
ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS unit_template_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS allow_customer_resale BOOLEAN NOT NULL DEFAULT false;
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
WITH duplicate_source_parents AS (
  SELECT id,
         MIN(id) OVER (PARTITION BY customer_id, source_category_id) AS keeper_id
  FROM %[1]s.product_categories parent
  WHERE parent.active=false
    AND parent.source_category_id > 0
    AND EXISTS (
      SELECT 1 FROM %[1]s.product_categories child
      WHERE child.parent_id=parent.id AND child.active=true
    )
)
UPDATE %[1]s.product_categories child
SET parent_id=duplicate_source_parents.keeper_id, updated_at=now()
FROM duplicate_source_parents
WHERE child.active=true
  AND child.parent_id=duplicate_source_parents.id
  AND duplicate_source_parents.id <> duplicate_source_parents.keeper_id
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.product_categories existing_child
    WHERE existing_child.active=true
      AND existing_child.parent_id=duplicate_source_parents.keeper_id
      AND lower(existing_child.name)=lower(child.name)
  );
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
  )
  AND (
    parent.source_category_id=0
    OR parent.id = (
      SELECT MIN(candidate.id)
      FROM %[1]s.product_categories candidate
      WHERE candidate.active=false
        AND candidate.customer_id=parent.customer_id
        AND candidate.source_category_id=parent.source_category_id
        AND candidate.source_category_id > 0
        AND EXISTS (
          SELECT 1 FROM %[1]s.product_categories child
          WHERE child.parent_id=candidate.id AND child.active=true
        )
    )
  )
  AND NOT EXISTS (
    SELECT 1 FROM %[1]s.product_categories active_source_parent
    WHERE parent.source_category_id > 0
      AND active_source_parent.active=true
      AND active_source_parent.customer_id=parent.customer_id
      AND active_source_parent.source_category_id=parent.source_category_id
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
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_templates (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_product_rule_templates_customer_name_active_uniq
ON %[1]s.customer_product_rule_templates (customer_id, lower(name))
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_template_items (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL REFERENCES %[1]s.customer_product_rule_templates(id) ON DELETE CASCADE,
	product_subtype_category_id BIGINT NOT NULL DEFAULT 0,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	price_list_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	unit_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_product_rule_template_items_template_subtype_uniq
ON %[1]s.customer_product_rule_template_items (template_id, product_subtype_category_id)
WHERE active=true;
CREATE TABLE IF NOT EXISTS %[1]s.customer_product_rule_overrides (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL DEFAULT 0,
	product_subtype_category_id BIGINT NOT NULL DEFAULT 0,
	gradient_template_id BIGINT NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,
	price_list_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	unit_rule_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	active BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS customer_product_rule_overrides_customer_subtype_uniq
ON %[1]s.customer_product_rule_overrides (customer_id, product_subtype_category_id)
WHERE active=true;
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT NULL,0,'熟豆',1,1,'kg','kg','kg',false
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='熟豆');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT NULL,0,'生豆',1,2,'kg','kg','kg',false
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='生豆');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT NULL,0,'挂耳',1,3,'kg','袋','盒',true
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='挂耳');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT NULL,0,'速溶咖啡',1,4,'kg','盒','盒',true
WHERE NOT EXISTS (SELECT 1 FROM %[1]s.product_categories WHERE active=true AND customer_id=0 AND COALESCE(parent_id,0)=0 AND name='速溶咖啡');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT p.id,p.customer_id,'默认熟豆',2,1,'kg','kg','kg',false FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='熟豆'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='默认熟豆');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,integer_unit)
SELECT p.id,p.customer_id,'默认生豆',2,1,'kg','kg','kg',false FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='生豆'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='默认生豆');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,unit_conversion_json,integer_unit)
SELECT p.id,p.customer_id,'默认挂耳',2,1,'kg','袋','盒','{"袋":1,"盒":10}'::jsonb,true FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='挂耳'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='默认挂耳');
INSERT INTO %[1]s.product_categories(parent_id,customer_id,name,level,position,inventory_unit,quote_unit,order_unit,unit_conversion_json,integer_unit)
SELECT p.id,p.customer_id,'默认速溶咖啡',2,1,'kg','盒','盒','{"盒":1}'::jsonb,true FROM %[1]s.product_categories p
WHERE p.active=true AND p.customer_id=0 AND COALESCE(p.parent_id,0)=0 AND p.name='速溶咖啡'
  AND NOT EXISTS (SELECT 1 FROM %[1]s.product_categories c WHERE c.active=true AND c.customer_id=p.customer_id AND c.parent_id=p.id AND c.name='默认速溶咖啡');
UPDATE %[1]s.products p
SET product_category_id = subtype.id
FROM %[1]s.product_categories type
JOIN %[1]s.product_categories subtype ON subtype.parent_id=type.id AND subtype.active=true AND subtype.name='默认熟豆'
WHERE type.active=true AND type.customer_id=0 AND COALESCE(type.parent_id,0)=0 AND type.name='熟豆'
  AND COALESCE(p.product_category_id,0)=0
  AND (p.product_kind='roasted_bean' OR p.product_kind='roasted');
UPDATE %[1]s.products p
SET product_category_id = subtype.id
FROM %[1]s.product_categories type
JOIN %[1]s.product_categories subtype ON subtype.parent_id=type.id AND subtype.active=true AND subtype.name='默认生豆'
WHERE type.active=true AND type.customer_id=0 AND COALESCE(type.parent_id,0)=0 AND type.name='生豆'
  AND COALESCE(p.product_category_id,0)=0
  AND p.product_kind='green_bean';
UPDATE %[1]s.products p
SET product_category_id = subtype.id
FROM %[1]s.product_categories type
JOIN %[1]s.product_categories subtype ON subtype.parent_id=type.id AND subtype.active=true AND subtype.name='默认挂耳'
WHERE type.active=true AND type.customer_id=0 AND COALESCE(type.parent_id,0)=0 AND type.name='挂耳'
  AND COALESCE(p.product_category_id,0)=0
  AND p.product_kind='drip_bag';
UPDATE %[1]s.products p
SET product_category_id = subtype.id
FROM %[1]s.product_categories type
JOIN %[1]s.product_categories subtype ON subtype.parent_id=type.id AND subtype.active=true AND subtype.name='默认速溶咖啡'
WHERE type.active=true AND type.customer_id=0 AND COALESCE(type.parent_id,0)=0 AND type.name='速溶咖啡'
  AND COALESCE(p.product_category_id,0)=0
  AND p.product_kind='instant_coffee';
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
UPDATE %[1]s.products
SET special_attrs_json = jsonb_set(COALESCE(special_attrs_json, '{}'::jsonb), '{roast_level}', to_jsonb(roast_level), true)
WHERE COALESCE(NULLIF(roast_level,''), '') <> ''
  AND NOT (COALESCE(special_attrs_json, '{}'::jsonb) ? 'roast_level');
UPDATE %[1]s.product_config_templates
SET special_attrs_schema_json = '[{"key":"roast_level","label":"烘焙度","value_type":"select","options":["浅烘","中烘","中深烘","深烘"],"required":false,"show_in_price_list":true,"position":1}]'::jsonb
WHERE active=true
  AND COALESCE(special_attrs_schema_json, '[]'::jsonb) = '[]'::jsonb
  AND (name LIKE '%%熟豆%%' OR name LIKE '%%咖啡%%');
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
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := migrateProductCategoriesToBusinessGroups(ctx, pool, schema); err != nil {
		return err
	}
	if err := migrateProductionBomGroupsToBusinessGroups(ctx, pool, schema); err != nil {
		return err
	}
	if err := migrateWarehousesToBusinessGroups(ctx, pool, schema); err != nil {
		return err
	}
	return backfillProductProductionConfigs(ctx, pool, schema)
}

func migrateProductCategoriesToBusinessGroups(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
WITH group_row AS (
	INSERT INTO %[1]s.business_groups(name, code, remark, active, sort_order, created_by, updated_by)
	VALUES('商品默认分组','default_product_catalog','PR-442: legacy product categories migrated to generic business group assignments',true,10,'system-pr442-migration','system-pr442-migration')
	ON CONFLICT DO NOTHING
	RETURNING id
),
target_group AS (
	SELECT id FROM group_row
	UNION ALL
	SELECT id FROM %[1]s.business_groups WHERE code='default_product_catalog' OR name='商品默认分组'
	ORDER BY id
	LIMIT 1
),
usage_upsert AS (
	INSERT INTO %[1]s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
	SELECT id, 'product_catalog', '商品档案归组', true, 'system-pr442-migration', 'system-pr442-migration'
	FROM target_group
	ON CONFLICT DO NOTHING
	RETURNING id
),
root_items AS (
	INSERT INTO %[1]s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
	SELECT tg.id, 0, pc.name, 'legacy_product_category_' || pc.id::text, '旧商品分类迁移', pc.active, COALESCE(pc.position,100)
	FROM %[1]s.product_categories pc
	CROSS JOIN target_group tg
	WHERE COALESCE(pc.parent_id,0)=0
	ON CONFLICT DO NOTHING
	RETURNING id
)
INSERT INTO %[1]s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
SELECT tg.id,
       COALESCE(item.id,0),
       'product_catalog',
       'product',
       p.id,
       '',
       COALESCE(p.product_category_position,100),
       'system-pr442-migration',
       'system-pr442-migration'
FROM %[1]s.products p
CROSS JOIN target_group tg
LEFT JOIN %[1]s.product_categories pc ON pc.id=COALESCE(p.product_category_id,0)
LEFT JOIN %[1]s.business_group_items item ON item.group_id=tg.id AND item.code='legacy_product_category_' || COALESCE(pc.id,0)::text
WHERE COALESCE(p.product_category_id,0)>0
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func migrateProductionBomGroupsToBusinessGroups(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasBomGroups bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".production_bom_groups").Scan(&hasBomGroups); err != nil {
		return err
	}
	if !hasBomGroups {
		return nil
	}
	q := fmt.Sprintf(`
WITH group_row AS (
	INSERT INTO %[1]s.business_groups(name, code, remark, active, sort_order, created_by, updated_by)
	VALUES('生产 BOM 默认分组','default_production_bom','PR-442: legacy production BOM groups migrated to generic business group assignments',true,20,'system-pr442-migration','system-pr442-migration')
	ON CONFLICT DO NOTHING
	RETURNING id
),
target_group AS (
	SELECT id FROM group_row
	UNION ALL
	SELECT id FROM %[1]s.business_groups WHERE code='default_production_bom' OR name='生产 BOM 默认分组'
	ORDER BY id
	LIMIT 1
),
usage_upsert AS (
	INSERT INTO %[1]s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
	SELECT id, 'production_bom', '生产 BOM 归组', true, 'system-pr442-migration', 'system-pr442-migration'
	FROM target_group
	ON CONFLICT DO NOTHING
	RETURNING id
),
group_items AS (
	INSERT INTO %[1]s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
	SELECT tg.id, 0, g.name, 'legacy_production_bom_group_' || g.id::text, '旧生产 BOM 大组迁移', g.active, COALESCE(g.sort_order,100)
	FROM %[1]s.production_bom_groups g
	CROSS JOIN target_group tg
	ON CONFLICT DO NOTHING
	RETURNING id
),
category_items AS (
	INSERT INTO %[1]s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
	SELECT tg.id, parent_item.id, c.name, 'legacy_production_bom_group_category_' || c.id::text, '旧生产 BOM 组内分类迁移', c.active, COALESCE(c.sort_order,100)
	FROM %[1]s.production_bom_group_categories c
	CROSS JOIN target_group tg
	JOIN %[1]s.business_group_items parent_item ON parent_item.group_id=tg.id AND parent_item.code='legacy_production_bom_group_' || c.group_id::text
	ON CONFLICT DO NOTHING
	RETURNING id
)
INSERT INTO %[1]s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
SELECT tg.id,
       COALESCE(category_item.id, group_item.id, 0),
       'production_bom',
       'production_bom',
       pb.id,
       '',
       100,
       'system-pr442-migration',
       'system-pr442-migration'
FROM %[1]s.production_boms pb
CROSS JOIN target_group tg
LEFT JOIN %[1]s.business_group_items group_item ON group_item.group_id=tg.id AND group_item.code='legacy_production_bom_group_' || COALESCE(pb.group_id,0)::text
LEFT JOIN %[1]s.business_group_items category_item ON category_item.group_id=tg.id AND category_item.code='legacy_production_bom_group_category_' || COALESCE(pb.group_category_id,0)::text
WHERE COALESCE(pb.group_id,0)>0 OR COALESCE(pb.group_category_id,0)>0
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func migrateWarehousesToBusinessGroups(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasWarehouses bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+".warehouses").Scan(&hasWarehouses); err != nil {
		return err
	}
	if !hasWarehouses {
		return nil
	}
	q := fmt.Sprintf(`
WITH group_row AS (
	INSERT INTO %[1]s.business_groups(name, code, remark, active, sort_order, created_by, updated_by)
	VALUES('仓库库存默认分组','default_warehouse_inventory','PR-442: warehouses migrated to generic business group assignments by warehouse code',true,30,'system-pr442-migration','system-pr442-migration')
	ON CONFLICT DO NOTHING
	RETURNING id
),
target_group AS (
	SELECT id FROM group_row
	UNION ALL
	SELECT id FROM %[1]s.business_groups WHERE code='default_warehouse_inventory' OR name='仓库库存默认分组'
	ORDER BY id
	LIMIT 1
),
usage_upsert AS (
	INSERT INTO %[1]s.business_group_usages(group_id, usage_key, usage_label, active, created_by, updated_by)
	SELECT id, 'warehouse_inventory', '仓库库存视图仓库归组', true, 'system-pr442-migration', 'system-pr442-migration'
	FROM target_group
	ON CONFLICT DO NOTHING
	RETURNING id
),
items AS (
	INSERT INTO %[1]s.business_group_items(group_id, parent_id, name, code, remark, active, sort_order)
	SELECT tg.id, 0,
	       CASE
	         WHEN COALESCE(w.kind,'')='customer' THEN '客户仓库'
	         WHEN COALESCE(w.kind,'') IN ('scrap','loss','waste') THEN '损耗/报废'
	         ELSE '普通仓库'
	       END,
	       CASE
	         WHEN COALESCE(w.kind,'')='customer' THEN 'customer_warehouses'
	         WHEN COALESCE(w.kind,'') IN ('scrap','loss','waste') THEN 'loss_scrap_warehouses'
	         ELSE 'normal_warehouses'
	       END,
	       'PR-442 默认库存仓库分组',
	       true,
	       CASE WHEN COALESCE(w.kind,'')='customer' THEN 20 WHEN COALESCE(w.kind,'') IN ('scrap','loss','waste') THEN 30 ELSE 10 END
	FROM %[1]s.warehouses w
	CROSS JOIN target_group tg
	GROUP BY tg.id, 1, 2, 3, 4, 5, 6
	ON CONFLICT DO NOTHING
	RETURNING id
)
INSERT INTO %[1]s.business_group_assignments(group_id, group_item_id, usage_key, object_key, object_id, object_ref, sort_order, created_by, updated_by)
SELECT tg.id,
       item.id,
       'warehouse_inventory',
       'warehouse',
       0,
       w.code,
       COALESCE(w.sort_order,100),
       'system-pr442-migration',
       'system-pr442-migration'
FROM %[1]s.warehouses w
CROSS JOIN target_group tg
JOIN %[1]s.business_group_items item ON item.group_id=tg.id AND item.code = CASE
	WHEN COALESCE(w.kind,'')='customer' THEN 'customer_warehouses'
	WHEN COALESCE(w.kind,'') IN ('scrap','loss','waste') THEN 'loss_scrap_warehouses'
	ELSE 'normal_warehouses'
END
ON CONFLICT DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}

func backfillProductProductionConfigs(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var hasProducts, hasProductionBoms bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL, to_regclass($2) IS NOT NULL`, schema+".products", schema+".production_bom_versions").Scan(&hasProducts, &hasProductionBoms); err != nil {
		return err
	}
	if !hasProducts || !hasProductionBoms {
		return nil
	}
	q := fmt.Sprintf(`
INSERT INTO %[1]s.product_production_configs(
	product_id, production_bom_id, production_bom_version_id, expected_loss_rate, note, created_by, updated_by
)
SELECT p.id,
       COALESCE(pbb.bom_id,0),
       COALESCE(pbb.bom_version_id,0),
       GREATEST(0, LEAST(0.9999, 1 - COALESCE(NULLIF(pbv.yield_rate,0), NULLIF(pb.yield_rate,0), CASE WHEN COALESCE(NULLIF(p.product_kind,''),'roasted_bean')='instant_coffee' THEN 1 ELSE 0.8 END))),
       'legacy-backfill',
       'system-backfill',
       'system-backfill'
FROM %[1]s.products p
LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=p.id
LEFT JOIN %[1]s.production_bom_versions pbv ON pbv.id=pbb.bom_version_id
LEFT JOIN %[1]s.product_bom pb ON pb.product_id=p.id
WHERE COALESCE(p.active,true)=true
ON CONFLICT (product_id) DO NOTHING;

WITH source_attrs AS (
	SELECT p.id AS product_id,
	       CASE
	         WHEN COALESCE(NULLIF(p.roast_level,''),'') <> ''
	         THEN jsonb_set(COALESCE(p.special_attrs_json, '{}'::jsonb), '{roast_level}', to_jsonb(p.roast_level), true)
	         ELSE COALESCE(p.special_attrs_json, '{}'::jsonb)
	       END AS attrs_json
	FROM %[1]s.products p
	JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id
),
attr_rows AS (
	SELECT source_attrs.product_id,
	       kv.key AS field_key,
	       kv.value AS value_text,
	       row_number() OVER (PARTITION BY source_attrs.product_id ORDER BY kv.key)::int AS sort_order
	FROM source_attrs
	CROSS JOIN LATERAL jsonb_each_text(source_attrs.attrs_json) AS kv(key,value)
	WHERE COALESCE(kv.key,'') <> '' AND COALESCE(kv.value,'') <> ''
)
INSERT INTO %[1]s.product_production_config_fields(
	product_id, field_key, label, field_type, unit, value_text, show_in_price_list, sort_order
)
SELECT product_id, field_key, field_key, 'text', '', value_text, true, sort_order
FROM attr_rows
ON CONFLICT (product_id, lower(field_key)) DO NOTHING;
`, schema)
	_, err := pool.Exec(ctx, q)
	return err
}
