package costing

import (
	"context"
	"fmt"

	domain "orderapp/internal/domain/costing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s.cost_parameters (
	key TEXT PRIMARY KEY,
	label TEXT NOT NULL DEFAULT '',
	value NUMERIC(14,6) NOT NULL DEFAULT 0,
	unit TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.cost_calculation_runs (
	id BIGSERIAL PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'draft',
	actor TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT 'erp',
	notes TEXT NOT NULL DEFAULT '',
	product_count INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	published_at TIMESTAMPTZ NULL
);
CREATE TABLE IF NOT EXISTS %[1]s.cost_calculation_items (
	id BIGSERIAL PRIMARY KEY,
	run_id BIGINT NOT NULL REFERENCES %[1]s.cost_calculation_runs(id) ON DELETE CASCADE,
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	result_json JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS cost_calculation_items_run_idx ON %[1]s.cost_calculation_items(run_id, id);
CREATE TABLE IF NOT EXISTS %[1]s.bean_list_publications (
	id BIGSERIAL PRIMARY KEY,
	list_type TEXT NOT NULL,
	product_type_category_id BIGINT NOT NULL DEFAULT 0,
	product_type_name TEXT NOT NULL DEFAULT '',
	version_no TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'published',
	owner_type TEXT NOT NULL DEFAULT 'official',
	owner_key TEXT NOT NULL DEFAULT '',
	price_source_publication_id BIGINT NULL,
	style_source_publication_id BIGINT NULL,
	source_version_no TEXT NOT NULL DEFAULT '',
	config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	changelog TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT '',
	published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	withdrawn_at TIMESTAMPTZ NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS product_type_category_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS product_type_name TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'official';
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS owner_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS price_source_publication_id BIGINT NULL;
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS style_source_publication_id BIGINT NULL;
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS source_version_no TEXT NOT NULL DEFAULT '';
UPDATE %[1]s.bean_list_publications
SET product_type_name = CASE
	WHEN list_type IN ('commercial','retail') THEN '熟豆'
	WHEN list_type='green' THEN '生豆'
	WHEN list_type='drip' THEN '挂耳'
	ELSE product_type_name
END
WHERE COALESCE(product_type_name,'')='';
DO $$
BEGIN
	IF to_regclass('%[1]s.product_categories') IS NOT NULL THEN
		EXECUTE 'UPDATE %[1]s.bean_list_publications b
		SET product_type_category_id=pc.id
		FROM %[1]s.product_categories pc
		WHERE COALESCE(b.product_type_category_id,0)=0
		  AND pc.active=true
		  AND pc.level=1
		  AND pc.customer_id=0
		  AND pc.name=b.product_type_name';
	END IF;
END $$;
CREATE INDEX IF NOT EXISTS bean_list_publications_type_created_idx ON %[1]s.bean_list_publications(list_type, created_at DESC);
CREATE INDEX IF NOT EXISTS bean_list_publications_product_type_owner_status_idx
	ON %[1]s.bean_list_publications(product_type_category_id, owner_type, owner_key, status, published_at DESC, id DESC);
DROP INDEX IF EXISTS %[1]s.bean_list_publications_one_published_idx;
DROP INDEX IF EXISTS %[1]s.bean_list_publications_one_published_owner_idx;
CREATE INDEX IF NOT EXISTS bean_list_publications_owner_status_idx
	ON %[1]s.bean_list_publications(owner_type, owner_key, status, published_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS %[1]s.bean_list_publication_assets (
	id BIGSERIAL PRIMARY KEY,
	publication_id BIGINT NOT NULL REFERENCES %[1]s.bean_list_publications(id) ON DELETE CASCADE,
	asset_type TEXT NOT NULL DEFAULT 'pdf',
	content_type TEXT NOT NULL DEFAULT 'application/pdf',
	cache_key TEXT NOT NULL DEFAULT '',
	payload BYTEA NOT NULL DEFAULT ''::bytea,
	created_by TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(publication_id, asset_type)
);
CREATE TABLE IF NOT EXISTS %[1]s.customer_bean_list_acknowledgements (
	id BIGSERIAL PRIMARY KEY,
	customer_id BIGINT NOT NULL REFERENCES %[1]s.customers(id) ON DELETE CASCADE,
	publication_id BIGINT NOT NULL REFERENCES %[1]s.bean_list_publications(id) ON DELETE CASCADE,
	acknowledged_by TEXT NOT NULL DEFAULT '',
	acknowledged_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(customer_id, publication_id)
);
CREATE TABLE IF NOT EXISTS %[1]s.drip_price_templates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	bag_grams NUMERIC(12,3) NOT NULL DEFAULT 10,
	box_bag_count INT NOT NULL DEFAULT 10,
	include_packaging BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS %[1]s.drip_price_template_tiers (
	id BIGSERIAL PRIMARY KEY,
	template_id BIGINT NOT NULL REFERENCES %[1]s.drip_price_templates(id) ON DELETE CASCADE,
	label TEXT NOT NULL,
	min_bags NUMERIC(14,3) NOT NULL,
	max_bags NUMERIC(14,3) NULL,
	multiplier NUMERIC(14,6) NOT NULL,
	position INT NOT NULL DEFAULT 1,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE INDEX IF NOT EXISTS drip_price_template_tiers_template_idx ON %[1]s.drip_price_template_tiers(template_id, position, id);
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS product_kind TEXT NOT NULL DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS price_basis TEXT NOT NULL DEFAULT 'weight';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS sales_unit TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS unit_bag_count INT NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.product_price_tiers ADD COLUMN IF NOT EXISTS price_source_json JSONB NOT NULL DEFAULT '{}'::jsonb;
UPDATE %[1]s.product_price_tiers SET product_kind='roasted_bean' WHERE COALESCE(product_kind,'')='';
UPDATE %[1]s.product_price_tiers SET price_basis='weight' WHERE COALESCE(price_basis,'')='';
UPDATE %[1]s.product_price_tiers SET sales_unit='' WHERE sales_unit IS NULL;
UPDATE %[1]s.product_price_tiers SET unit_bag_count=0 WHERE unit_bag_count IS NULL;
UPDATE %[1]s.product_price_tiers SET price_source_json='{}'::jsonb WHERE price_source_json IS NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN product_kind SET DEFAULT 'roasted_bean';
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN product_kind SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_basis SET DEFAULT 'weight';
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_basis SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN sales_unit SET DEFAULT '';
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN sales_unit SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN unit_bag_count SET DEFAULT 0;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN unit_bag_count SET NOT NULL;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_source_json SET DEFAULT '{}'::jsonb;
ALTER TABLE %[1]s.product_price_tiers ALTER COLUMN price_source_json SET NOT NULL;
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := seedParameters(ctx, pool, schema); err != nil {
		return err
	}
	return seedDefaultDripPriceTemplate(ctx, pool, schema)
}

func seedParameters(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	params := domain.DefaultParameters()
	rows := []struct {
		key, label, unit string
		value            float64
	}{
		{"roast_yield_rate", "生豆到熟豆转化率", "ratio", params.RoastYieldRate},
		{"kg_to_lb_factor", "kg 到 lb 换算", "lb/kg", params.KgToLbFactor},
		{"small_batch_production_cost_per_kg", "小批量生产成本", "元/kg", params.SmallBatchProductionCostPerKg},
		{"large_batch_production_cost_per_kg", "大批量生产成本", "元/kg", params.LargeBatchProductionCostPerKg},
		{"wholesale_package_cost_per_kg", "批发包装成本", "元/kg", params.WholesalePackageCostPerKg},
		{"product_loss_per_kg", "产品损耗", "元/kg", params.ProductLossPerKg},
		{"retail_bean_margin_rate", "零售熟豆利润系数", "ratio", params.RetailBeanMarginRate},
		{"retail_tax_rate", "零售税率", "ratio", params.RetailTaxRate},
		{"retail_logistics_per_kg", "零售熟豆物流", "元/kg", params.RetailLogisticsPerKg},
		{"retail_drip_logistics_per_10_bags", "零售挂耳物流", "元/10袋", params.RetailDripLogisticsPer10Bags},
		{"drip_green_ratio_kg_per_bag", "挂耳单袋咖啡消耗", "kg/袋", params.DripGreenRatioKgPerBag},
		{"drip_process_cost_per_bag", "挂耳加工成本", "元/袋", params.DripProcessCostPerBag},
		{"drip_extra_cost_per_bag", "挂耳额外成本", "元/袋", params.DripExtraCostPerBag},
		{"drip_packing_material_per_bag", "挂耳外包装材料", "元/袋", params.DripPackingMaterialPerBag},
		{"retail_drip_multiplier", "零售挂耳利润系数", "ratio", params.RetailDripMultiplier},
		{"wholesale_kg_margin_rate_1", "商用熟豆 2包-13包 利润系数", "ratio", params.WholesaleKgMarginRates[0]},
		{"wholesale_kg_margin_rate_2", "商用熟豆 14包-23包 利润系数", "ratio", params.WholesaleKgMarginRates[1]},
		{"wholesale_kg_margin_rate_3", "商用熟豆 24包-47包 利润系数", "ratio", params.WholesaleKgMarginRates[2]},
		{"wholesale_kg_margin_rate_4", "商用熟豆 48包+ / 24-49kg 利润系数", "ratio", params.WholesaleKgMarginRates[3]},
		{"wholesale_kg_margin_rate_5", "商用熟豆 50-99kg 利润系数", "ratio", params.WholesaleKgMarginRates[4]},
		{"wholesale_kg_margin_rate_6", "商用熟豆 100-199kg 利润系数", "ratio", params.WholesaleKgMarginRates[5]},
		{"wholesale_drip_multiplier_1", "商用挂耳 100包 利润系数", "ratio", params.WholesaleDripMultipliers[0]},
		{"wholesale_drip_multiplier_2", "商用挂耳 200包 利润系数", "ratio", params.WholesaleDripMultipliers[1]},
		{"wholesale_drip_multiplier_3", "商用挂耳 300包 利润系数", "ratio", params.WholesaleDripMultipliers[2]},
		{"wholesale_drip_multiplier_4", "商用挂耳 500包 利润系数", "ratio", params.WholesaleDripMultipliers[3]},
	}
	q := fmt.Sprintf(`INSERT INTO %s.cost_parameters(key,label,value,unit)
		VALUES($1,$2,$3,$4)
		ON CONFLICT(key) DO UPDATE SET label=EXCLUDED.label, unit=EXCLUDED.unit`, schema)
	for _, r := range rows {
		if _, err := pool.Exec(ctx, q, r.key, r.label, r.value, r.unit); err != nil {
			return err
		}
	}
	return nil
}

func seedDefaultDripPriceTemplate(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var id int64
	err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.drip_price_templates(name, active, bag_grams, box_bag_count, include_packaging)
		SELECT '默认挂耳供应价', true, 10, 10, true
		WHERE NOT EXISTS (SELECT 1 FROM %s.drip_price_templates WHERE name='默认挂耳供应价')
		RETURNING id
	`, schema, schema)).Scan(&id)
	if err != nil {
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.drip_price_templates WHERE name='默认挂耳供应价' ORDER BY id LIMIT 1`, schema)).Scan(&id); err != nil {
			return err
		}
	}
	rows := []struct {
		label      string
		minBags    float64
		multiplier float64
		position   int
	}{
		{"100袋", 100, 2.2, 1},
		{"1000袋", 1000, 1.8, 2},
		{"5000袋", 5000, 1.6, 3},
		{"10000袋", 10000, 1.35, 4},
	}
	for _, row := range rows {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.drip_price_template_tiers(template_id, label, min_bags, multiplier, position, active)
			SELECT $1, $2, $3, $4, $5, true
			WHERE NOT EXISTS (
				SELECT 1 FROM %s.drip_price_template_tiers WHERE template_id=$1 AND label=$2
			)
		`, schema, schema), id, row.label, row.minBags, row.multiplier, row.position); err != nil {
			return err
		}
	}
	return nil
}
