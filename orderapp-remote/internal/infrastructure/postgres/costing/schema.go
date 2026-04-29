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
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'official';
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS owner_key TEXT NOT NULL DEFAULT '';
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS price_source_publication_id BIGINT NULL;
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS style_source_publication_id BIGINT NULL;
ALTER TABLE %[1]s.bean_list_publications ADD COLUMN IF NOT EXISTS source_version_no TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS bean_list_publications_type_created_idx ON %[1]s.bean_list_publications(list_type, created_at DESC);
DROP INDEX IF EXISTS %[1]s.bean_list_publications_one_published_idx;
CREATE UNIQUE INDEX IF NOT EXISTS bean_list_publications_one_published_owner_idx
	ON %[1]s.bean_list_publications(list_type, owner_type, owner_key)
	WHERE status = 'published';
`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	return seedParameters(ctx, pool, schema)
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
