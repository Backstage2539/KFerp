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
	Active                      bool
	CustomerID                  int64
	BaseProductID               int64
	Visibility                  string
	CustomType                  string
	MarginRateOverride          *float64
	GradientTemplateIDOverride  int64
	OperationTemplateIDOverride int64
	UnitRuleOverrideJSON        string
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
	sqlstr := fmt.Sprintf(`SELECT p.id, p.name, COALESCE(p.remark,''), COALESCE(p.roast_level,''), COALESCE(p.special_attrs_json::text,'{}'), p.default_price,
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
		COALESCE(p.active,true),
		COALESCE(p.customer_id, 0),
		COALESCE(p.base_product_id, 0),
		COALESCE(NULLIF(p.visibility,''), 'public'),
		COALESCE(p.custom_type, ''),
		p.margin_rate_override::float8,
		COALESCE(p.gradient_template_id_override,0),
		COALESCE(p.operation_template_id_override,0),
		COALESCE(p.unit_rule_override_json::text,'{}'),
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
		if err := rows.Scan(&p.ID, &p.Name, &p.Remark, &p.RoastLevel, &p.SpecialAttrsJSON, &p.DefaultPrice, &p.ProductKind, &p.GreenBeanType, &p.GreenBeanBomProductID, &p.DripBagGrams, &p.DripBoxBagCount, &p.AllowFulfillmentOrder, &p.AllowMallOrder, &p.RetailPrice100G, &p.RetailPrice200G, &p.RetailPrice227G, &p.RetailPrice250G, &p.YieldRate, &p.ExpectedLossRate, &p.ProcessRouteID, &p.ProductionConfigNote, &p.ProductCategoryID, &p.ProductCategoryPosition, &p.Active, &p.CustomerID, &p.BaseProductID, &p.Visibility, &p.CustomType, &p.MarginRateOverride, &p.GradientTemplateIDOverride, &p.OperationTemplateIDOverride, &p.UnitRuleOverrideJSON, &p.BomItemCount, &p.BomStatus, &p.OrderUsageCount, &p.BomSourceType, &p.EffectiveProductID, &p.EffectiveBomVersionID, &p.SourceProductID, &p.SourceProductCode, &p.SourceProductName, &p.SourceBomVersionID, &p.SourceBomVersionNo, &p.DerivedFromLabel, &p.CanEditBOM, &p.ProductionBomID, &p.ProductionBomCode, &p.ProductionBomName, &p.ProductionBomVersionID, &p.ProductionBomVersionNo, &p.LatestBomVersionID, &p.LatestBomVersionNo, &p.IsLatestBomVersion, &p.ProductionBomGroupID, &p.ProductionBomGroupName); err != nil {
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
