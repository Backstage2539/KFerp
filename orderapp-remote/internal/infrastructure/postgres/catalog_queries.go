package postgres

import (
	"context"
	"fmt"

	catalogdomain "orderapp/internal/domain/catalog"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Option struct {
	ID   int64
	Name string
}

type ProductTierOption struct {
	ID        int64
	SpecG     int64
	MinQty    float64
	MaxQty    *float64
	UnitPrice float64
}

type ProductOption struct {
	ID                          int64
	SKUID                       int64
	ParentProductID             int64
	EffectiveParentProductID    int64
	SKUName                     string
	SKUCode                     string
	Barcode                     string
	SpecLabel                   string
	NetContentQty               float64
	NetContentUnit              string
	IsDefaultSKU                bool
	Name                        string
	Remark                      string
	ProductKind                 string
	GreenBeanType               string
	GreenBeanBomProductID       int64
	RoastLevel                  string
	SpecialAttrsJSON            string
	DripBagGrams                float64
	DripBoxBagCount             int
	AllowFulfillmentOrder       bool
	AllowMallOrder              bool
	SalesUnits                  []string
	DefaultPrice                float64
	RetailPrice100G             float64
	RetailPrice200G             float64
	RetailPrice227G             float64
	RetailPrice250G             float64
	YieldRate                   float64
	ExpectedLossRate            float64
	ProcessRouteID              int64
	ProductionConfigNote        string
	ProductCategoryID           int64
	ProductCategoryPosition     int
	ClassificationTemplateID    int64
	Active                      bool
	CustomerID                  int64
	BaseProductID               int64
	Visibility                  string
	CustomType                  string
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitRuleOverrideJSON        string
	InventoryUnit               string
	IntegerInventoryUnit        bool
	DefaultSalesUnit            string
	UnitConversionJSON          string
	SalesUnitRulesJSON          string
	UnitTemplateID              int64
	UnitTemplateName            string
	UnitRuleSource              string
	ProductConfigTemplateID     int64
	BomItemCount                int
	BomStatus                   string
	BomSourceType               string
	EffectiveProductID          int64
	EffectiveBomVersionID       int64
	SourceProductID             int64
	SourceProductCode           string
	SourceProductName           string
	SourceBomVersionID          int64
	SourceBomVersionNo          string
	DerivedFromLabel            string
	CanEditBOM                  bool
	ProductionBomID             int64
	ProductionBomCode           string
	ProductionBomName           string
	ProductionBomVersionID      int64
	ProductionBomVersionNo      string
	LatestBomVersionID          int64
	LatestBomVersionNo          string
	IsLatestBomVersion          bool
	ProductionBomGroupID        int64
	ProductionBomGroupName      string
	OrderUsageCount             int
	RetailSpecs                 []int64
	Tiers                       []ProductTierOption
}

func FetchOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Option, 0)
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func FetchProducts(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProductOption, error) {
	sqlstr := fmt.Sprintf(`SELECT p.id,
		p.id AS sku_id,
		COALESCE(p.parent_product_id,0) AS parent_product_id,
		CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(p.parent_product_id,0) ELSE p.id END AS effective_parent_product_id,
		COALESCE(NULLIF(p.sku_name,''), CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.name ELSE '默认规格' END) AS sku_name,
		COALESCE(p.sku_code,'') AS sku_code,
		COALESCE(p.barcode,'') AS barcode,
		COALESCE(p.spec_label,'') AS spec_label,
		COALESCE(p.net_content_qty,0)::float8 AS net_content_qty,
		COALESCE(p.net_content_unit,'') AS net_content_unit,
		(COALESCE(p.is_default_sku,false) OR COALESCE(p.parent_product_id,0)=0) AS is_default_sku,
		p.name, COALESCE(p.remark,''), COALESCE(p.roast_level,''), COALESCE(p.special_attrs_json::text,'{}'), p.default_price,
		COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'),
		COALESCE(p.green_bean_type, ''),
		COALESCE(p.green_bean_bom_product_id, 0),
		COALESCE(p.drip_bag_grams, 10)::float8,
		COALESCE(p.drip_box_bag_count, 10),
		COALESCE(p.allow_fulfillment_order, true),
		COALESCE(p.allow_mall_order, false),
		COALESCE(p.retail_price_100g, 0),
		COALESCE(p.retail_price_200g, 0),
		COALESCE(p.retail_price_227g, p.default_price, 0),
		COALESCE(p.retail_price_250g, 0),
		CASE WHEN ppc.product_id IS NOT NULL THEN 1 - COALESCE(NULLIF(ppc.expected_loss_rate,0), 0) ELSE COALESCE(pbv.yield_rate, b.yield_rate, 0.8) END,
		COALESCE(ppc.expected_loss_rate, GREATEST(0, 1 - COALESCE(pbv.yield_rate, b.yield_rate, 0.8))),
		COALESCE(ppc.process_route_id,0),
		COALESCE(ppc.note,''),
		COALESCE(p.product_category_id, 0),
		COALESCE(p.product_category_position, 0),
		COALESCE(p.classification_template_id, 0),
		COALESCE(p.active,true),
		COALESCE(p.customer_id, 0),
		COALESCE(p.base_product_id, 0),
		COALESCE(NULLIF(p.visibility,''), 'public'),
		COALESCE(p.custom_type, ''),
			p.margin_rate_override::float8,
			COALESCE(p.gradient_template_id_override,0),
			COALESCE(p.operation_template_id_override,0),
			COALESCE(p.unit_rule_override_json::text,'{}'),
			COALESCE(NULLIF(p.unit_rule_override_json->>'inventory_unit',''), NULLIF(product_direct_unit_template.inventory_unit,''), NULLIF(product_config.inventory_unit,''), NULLIF(product_unit_template.inventory_unit,''), NULLIF(category_config.inventory_unit,''), NULLIF(category_unit_template.inventory_unit,''), 'kg') AS inventory_unit,
			COALESCE(
				CASE WHEN lower(p.unit_rule_override_json->>'integer_inventory_unit') IN ('true','1','yes') THEN true WHEN lower(p.unit_rule_override_json->>'integer_inventory_unit') IN ('false','0','no') THEN false ELSE NULL END,
				CASE WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('true','1','yes') THEN true WHEN lower(p.unit_rule_override_json->>'integer_unit') IN ('false','0','no') THEN false ELSE NULL END,
				product_direct_unit_template.integer_unit,
				product_config.integer_unit,
				product_unit_template.integer_unit,
				category_config.integer_unit,
				category_unit_template.integer_unit,
				false
			) AS integer_inventory_unit,
			COALESCE(
				NULLIF(p.unit_rule_override_json->>'default_sales_unit',''),
				NULLIF(p.unit_rule_override_json->>'order_unit',''),
				NULLIF(p.unit_rule_override_json->>'quote_unit',''),
				NULLIF(product_direct_unit_template.order_unit,''),
				NULLIF(product_direct_unit_template.quote_unit,''),
				NULLIF(product_config.order_unit,''),
				NULLIF(product_config.quote_unit,''),
				NULLIF(product_unit_template.order_unit,''),
				NULLIF(product_unit_template.quote_unit,''),
				NULLIF(category_config.order_unit,''),
				NULLIF(category_config.quote_unit,''),
				NULLIF(category_unit_template.order_unit,''),
				NULLIF(category_unit_template.quote_unit,''),
				NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
				NULLIF(product_direct_unit_template.inventory_unit,''),
				NULLIF(product_config.inventory_unit,''),
				NULLIF(product_unit_template.inventory_unit,''),
				NULLIF(category_config.inventory_unit,''),
				NULLIF(category_unit_template.inventory_unit,''),
				'kg'
			) AS default_sales_unit,
			COALESCE(
				NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
				NULLIF(p.unit_rule_override_json->>'conversion_json',''),
				NULLIF(product_direct_unit_template.unit_conversion_json::text,'{}'),
				NULLIF(product_config.unit_conversion_json::text,'{}'),
				NULLIF(product_unit_template.unit_conversion_json::text,'{}'),
				NULLIF(category_config.unit_conversion_json::text,'{}'),
				NULLIF(category_unit_template.unit_conversion_json::text,'{}'),
				'{}'
			) AS unit_conversion_json,
			COALESCE(
				NULLIF(p.unit_rule_override_json->>'sales_unit_rules',''),
				CASE WHEN COALESCE(product_direct_unit_template.integer_unit,false) THEN jsonb_build_object(COALESCE(NULLIF(product_direct_unit_template.order_unit,''), NULLIF(product_direct_unit_template.quote_unit,''), NULLIF(product_direct_unit_template.inventory_unit,''), 'kg'), jsonb_build_object('integer', true))::text ELSE NULL END,
				'{}'
			) AS sales_unit_rules,
			COALESCE(p.unit_template_id,0) AS unit_template_id,
			COALESCE(product_direct_unit_template.name,'') AS unit_template_name,
			CASE
				WHEN p.unit_rule_override_json ?| array['inventory_unit','integer_inventory_unit','integer_unit','default_sales_unit','quote_unit','order_unit','unit_conversion_json','conversion_json','sales_unit_rules'] THEN 'product_override'
				WHEN COALESCE(p.unit_template_id,0)>0 AND product_direct_unit_template.id IS NOT NULL THEN 'product_unit_template'
				WHEN product_config.id IS NOT NULL OR product_unit_template.id IS NOT NULL THEN 'legacy_template'
				WHEN category_config.id IS NOT NULL OR category_unit_template.id IS NOT NULL THEN 'category'
				ELSE 'default'
			END AS unit_rule_source,
			COALESCE(p.product_config_template_id,0),
		CASE WHEN COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id) IS NOT NULL THEN COALESCE((SELECT COUNT(*) FROM %[1]s.production_bom_version_items pbi WHERE pbi.version_id=pbv.id),0)
			ELSE COALESCE((SELECT COUNT(*) FROM %[1]s.product_bom_items bi WHERE bi.product_id=CASE
				WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
				ELSE p.id
			END), 0)
		END,
		CASE WHEN COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id) IS NOT NULL THEN COALESCE(NULLIF(pb.status,''), 'active')
			ELSE COALESCE(NULLIF(b.status,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'active' END)
		END,
		COALESCE((
			SELECT COUNT(*)
			FROM %[1]s.order_items oi
			JOIN %[1]s.orders o ON o.id=oi.order_id
			WHERE oi.product_id=p.id AND COALESCE(o.is_void,false)=false
		),0) AS order_usage_count,
		CASE WHEN COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id) IS NOT NULL THEN 'owned'
			ELSE COALESCE(NULLIF(bs.source_type,''), CASE WHEN b.product_id IS NULL THEN 'missing' ELSE 'owned' END)
		END AS bom_source_type,
		CASE WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id ELSE p.id END AS effective_product_id,
		COALESCE(bs.source_bom_version_id,0) AS effective_bom_version_id,
		COALESCE(bs.source_product_id,0) AS source_product_id,
		COALESCE(NULLIF(bs.source_product_code_snapshot,''), CASE WHEN COALESCE(bs.source_product_id,0)>0 THEN 'SKU-' || bs.source_product_id::text ELSE '' END) AS source_product_code,
		COALESCE(bs.source_product_name_snapshot,'') AS source_product_name,
		COALESCE(bs.source_bom_version_id,0) AS source_bom_version_id,
		COALESCE(NULLIF(bs.source_bom_version_no_snapshot,''),'当前BOM') AS source_bom_version_no,
		'' AS derived_from_label,
		CASE WHEN COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id) IS NOT NULL THEN true
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') THEN false
			ELSE true
		END AS can_edit_bom,
		COALESCE(pb.id,0) AS production_bom_id,
		COALESCE(pb.code,'') AS production_bom_code,
		COALESCE(pb.name,'') AS production_bom_name,
		COALESCE(pbv.id,0) AS production_bom_version_id,
		COALESCE(pbv.version_no,'') AS production_bom_version_no,
		COALESCE(latest_bom_version.id,0) AS latest_bom_version_id,
		COALESCE(latest_bom_version.version_no,'') AS latest_bom_version_no,
		CASE WHEN pbv.id IS NULL THEN true ELSE pbv.id=COALESCE(latest_bom_version.id,0) END AS is_latest_bom_version,
		COALESCE(pbg.id,0) AS production_bom_group_id,
		COALESCE(pbg.name,'') AS production_bom_group_name
			FROM %[1]s.products p
			LEFT JOIN %[1]s.product_config_templates product_config ON product_config.id=COALESCE(p.product_config_template_id,0) AND product_config.deleted_at IS NULL
			LEFT JOIN %[1]s.product_unit_templates product_direct_unit_template ON product_direct_unit_template.id=COALESCE(p.unit_template_id,0) AND product_direct_unit_template.active=true AND product_direct_unit_template.deleted_at IS NULL
			LEFT JOIN %[1]s.product_unit_templates product_unit_template ON product_unit_template.id=COALESCE(product_config.unit_template_id,0) AND product_unit_template.deleted_at IS NULL
			LEFT JOIN %[1]s.product_categories category_config ON category_config.id=COALESCE(p.product_category_id,0)
			LEFT JOIN %[1]s.product_unit_templates category_unit_template ON category_unit_template.id=COALESCE(category_config.unit_template_id,0) AND category_unit_template.deleted_at IS NULL
			LEFT JOIN %[1]s.product_bom_sources bs ON bs.product_id=p.id
		LEFT JOIN %[1]s.product_bom b ON b.product_id=CASE
			WHEN COALESCE(NULLIF(bs.source_type,''),'') IN ('inherit_current','inherit_version') AND COALESCE(bs.source_product_id,0)>0 THEN bs.source_product_id
			ELSE p.id
		END
		LEFT JOIN %[1]s.product_production_configs ppc ON ppc.product_id=p.id
		LEFT JOIN %[1]s.product_production_bom_bindings pbb ON pbb.product_id=p.id
		LEFT JOIN %[1]s.production_boms pb ON pb.id=COALESCE(NULLIF(ppc.production_bom_id,0), pbb.bom_id)
		LEFT JOIN %[1]s.production_bom_versions pbv ON pbv.id=COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id)
		LEFT JOIN %[1]s.production_bom_groups pbg ON pbg.id=pb.group_id
		LEFT JOIN LATERAL (
			SELECT id, version_no
			FROM %[1]s.production_bom_versions latest
			WHERE latest.bom_id=pb.id AND latest.status='published'
			ORDER BY latest.published_at DESC NULLS LAST, latest.id DESC
			LIMIT 1
		) latest_bom_version ON true
		WHERE 1=1 ORDER BY p.active DESC, p.name`, schema)
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProductOption, 0)
	for rows.Next() {
		var p ProductOption
		if err := rows.Scan(&p.ID, &p.SKUID, &p.ParentProductID, &p.EffectiveParentProductID, &p.SKUName, &p.SKUCode, &p.Barcode, &p.SpecLabel, &p.NetContentQty, &p.NetContentUnit, &p.IsDefaultSKU, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ExpectedLossRate, &p.ProcessRouteID, &p.ProductionConfigNote, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.ClassificationTemplateID, &p.Active, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.InventoryUnit, &p.IntegerInventoryUnit, &p.DefaultSalesUnit, &p.UnitConversionJSON, &p.SalesUnitRulesJSON, &p.UnitTemplateID, &p.UnitTemplateName, &p.UnitRuleSource, &p.ProductConfigTemplateID, &p.BomItemCount, &p.BomStatus, &p.OrderUsageCount, &p.BomSourceType, &p.EffectiveProductID, &p.EffectiveBomVersionID, &p.SourceProductID, &p.SourceProductCode, &p.SourceProductName, &p.SourceBomVersionID, &p.SourceBomVersionNo, &p.DerivedFromLabel, &p.CanEditBOM, &p.ProductionBomID, &p.ProductionBomCode, &p.ProductionBomName, &p.ProductionBomVersionID, &p.ProductionBomVersionNo, &p.LatestBomVersionID, &p.LatestBomVersionNo, &p.IsLatestBomVersion, &p.ProductionBomGroupID, &p.ProductionBomGroupName); err != nil {
			return nil, err
		}
		p.ProductKind = catalogdomain.NormalizeProductKind(p.ProductKind)
		if p.ProductKind == catalogdomain.ProductKindDripBag {
			p.SalesUnits = []string{"bag", "box"}
		}
		if !catalogdomain.ProductKindSupportsBomParams(p.ProductKind) {
			p.RoastLevel = ""
			p.YieldRate = 0
		}
		p.RetailSpecs = salesdomain.RetailAvailableSpecs(salesdomain.RetailSpecPrices{
			Price100G: p.RetailPrice100G,
			Price200G: p.RetailPrice200G,
			Price227G: p.RetailPrice227G,
			Price250G: p.RetailPrice250G,
		})
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tierSQL := fmt.Sprintf(`
		SELECT id, product_id,
		       COALESCE(NULLIF(spec_g,0), 454),
		       COALESCE(min_qty_units, min_qty_lb),
		       COALESCE(max_qty_units, max_qty_lb),
		       COALESCE(price_per_unit, price_per_lb)
		FROM %s.product_price_tiers
		WHERE active=true
		ORDER BY product_id, COALESCE(NULLIF(spec_g,0), 454), COALESCE(min_qty_units, min_qty_lb)
	`, schema)
	trs, err := pool.Query(ctx, tierSQL)
	if err != nil {
		return out, nil
	}
	defer trs.Close()

	tierMap := map[int64][]ProductTierOption{}
	for trs.Next() {
		var tid, pid int64
		var specG int64
		var min float64
		var max *float64
		var price float64
		if err := trs.Scan(&tid, &pid, &specG, &min, &max, &price); err != nil {
			return nil, err
		}
		tierMap[pid] = append(tierMap[pid], ProductTierOption{ID: tid, SpecG: specG, MinQty: min, MaxQty: max, UnitPrice: price})
	}
	if err := trs.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		out[i].Tiers = tierMap[out[i].ID]
	}
	return out, nil
}
