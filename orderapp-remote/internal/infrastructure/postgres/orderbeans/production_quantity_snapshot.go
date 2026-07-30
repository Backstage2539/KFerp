package orderbeans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ProductionQuantitySnapshot freezes the production-relevant quantity meaning
// of one sold SKU. Production planning consumes this snapshot instead of
// parsing display names such as "454g".
type ProductionQuantitySnapshot struct {
	SKUID                    int64   `json:"sku_id"`
	ParentProductID          int64   `json:"parent_product_id"`
	SpecLabel                string  `json:"spec_label"`
	SalesUnit                string  `json:"sales_unit"`
	InventoryUnit            string  `json:"inventory_unit"`
	InventoryQtyPerSalesUnit float64 `json:"inventory_qty_per_sales_unit"`
	ConversionSource         string  `json:"conversion_source"`
}

// BuildProductionQuantitySnapshot validates and freezes one concrete published
// SKU. The explicit published conversion graph is authoritative; net content
// is only a compatible fallback for weight-to-weight conversions.
func BuildProductionQuantitySnapshot(spec PublishedProductSpec) (ProductionQuantitySnapshot, error) {
	snapshot := ProductionQuantitySnapshot{
		SKUID:           spec.SKUID,
		ParentProductID: spec.ParentProductID,
		SpecLabel:       firstNonEmptyProductionString(spec.SpecLabel, spec.SpecName),
		SalesUnit:       strings.TrimSpace(spec.SalesUnit),
		InventoryUnit:   strings.TrimSpace(spec.InventoryUnit),
	}
	if snapshot.SKUID <= 0 || snapshot.ParentProductID <= 0 {
		return ProductionQuantitySnapshot{}, fmt.Errorf("具体 SKU 与父商品快照缺失")
	}
	if snapshot.SpecLabel == "" || snapshot.SalesUnit == "" || snapshot.InventoryUnit == "" {
		return ProductionQuantitySnapshot{}, fmt.Errorf("具体 SKU %d 缺少销售规格或库存单位换算字段", snapshot.SKUID)
	}
	if factor := publishedInventoryConversionFactor(spec.InventoryConversionJSON, snapshot.SalesUnit, snapshot.InventoryUnit); factor > 0 {
		snapshot.InventoryQtyPerSalesUnit = factor
		snapshot.ConversionSource = "published_inventory_conversion"
		if spec.CurrentCatalogAuthority {
			snapshot.ConversionSource = "current_catalog_inventory_conversion"
		}
		return snapshot, nil
	}
	if factor := publishedNetContentInventoryFactor(spec.NetContentQty, spec.NetContentUnit, snapshot.InventoryUnit); factor > 0 {
		snapshot.InventoryQtyPerSalesUnit = factor
		snapshot.ConversionSource = "effective_sales_spec_net_content"
		if spec.CurrentCatalogAuthority {
			snapshot.ConversionSource = "current_catalog_net_content"
		}
		return snapshot, nil
	}
	return ProductionQuantitySnapshot{}, fmt.Errorf(
		"具体 SKU %d（%s）缺少从销售单位 %s 到库存单位 %s 的有效库存单位换算",
		snapshot.SKUID,
		snapshot.SpecLabel,
		snapshot.SalesUnit,
		snapshot.InventoryUnit,
	)
}

// AttachProductionQuantitySnapshot adds the immutable production quantity
// snapshot to an order-line price source. Legacy publications remain readable;
// a concrete publication must contain the target SKU and a valid conversion.
func AttachProductionQuantitySnapshot(raw string, spec PublishedProductSpec) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(raw), &source); err != nil {
		return "", fmt.Errorf("解析订单价格来源快照失败: %w", err)
	}
	if !spec.ConcretePublication && !spec.CurrentCatalogAuthority {
		encoded, err := json.Marshal(source)
		if err != nil {
			return "", fmt.Errorf("生成订单价格来源快照失败: %w", err)
		}
		return string(encoded), nil
	}
	if !spec.ProductFound {
		return "", fmt.Errorf("具体 SKU %d 不在价格表发布快照中", spec.SKUID)
	}
	snapshot, err := BuildProductionQuantitySnapshot(spec)
	if err != nil {
		return "", err
	}
	source["production_quantity_snapshot"] = snapshot
	encoded, err := json.Marshal(source)
	if err != nil {
		return "", fmt.Errorf("生成订单生产数量快照失败: %w", err)
	}
	return string(encoded), nil
}

// ResolveOrderProductionProductSpec returns the authority that a newly written
// order line must freeze. Concrete publications keep their immutable published
// spec after validating the active SKU identity. Legacy publications and
// manual orders resolve the same active concrete SKU from current catalog
// master data; only already-existing historical order lines may omit this
// snapshot and use the production-side compatibility fallback.
func ResolveOrderProductionProductSpec(
	ctx context.Context,
	q rowQuerier,
	schema string,
	productID int64,
	published PublishedProductSpec,
) (PublishedProductSpec, error) {
	current, err := ResolveCurrentOrderProductionProductSpec(ctx, q, schema, productID)
	if err != nil {
		return PublishedProductSpec{}, err
	}
	if published.ConcretePublication {
		if !published.ProductFound {
			return PublishedProductSpec{}, fmt.Errorf("具体 SKU %d 不在价格表发布快照中", productID)
		}
		if published.SKUID != current.SKUID {
			return PublishedProductSpec{}, fmt.Errorf("价格表 SKU 与当前商品规格不一致")
		}
		if published.ParentProductID != current.ParentProductID {
			return PublishedProductSpec{}, fmt.Errorf("价格表 SKU 不属于当前商品，请重新选择规格")
		}
		if _, err := BuildProductionQuantitySnapshot(published); err != nil {
			return PublishedProductSpec{}, err
		}
		return published, nil
	}
	return current, nil
}

// ResolveCurrentOrderProductionProductSpec freezes the active concrete SKU
// catalog authority for order writes that do not originate from a price-list
// publication, such as mall-price orders and zero-price direct-ship imports.
// It intentionally performs no bean_list_publications lookup.
func ResolveCurrentOrderProductionProductSpec(
	ctx context.Context,
	q rowQuerier,
	schema string,
	productID int64,
) (PublishedProductSpec, error) {
	current, err := resolveCurrentProductionProductSpec(ctx, q, schema, productID)
	if err != nil {
		return PublishedProductSpec{}, err
	}
	if _, err := BuildProductionQuantitySnapshot(current); err != nil {
		return PublishedProductSpec{}, err
	}
	return current, nil
}

func resolveCurrentProductionProductSpec(ctx context.Context, q rowQuerier, schema string, productID int64) (PublishedProductSpec, error) {
	if q == nil || strings.TrimSpace(schema) == "" || productID <= 0 {
		return PublishedProductSpec{}, fmt.Errorf("商品规格已停用或不存在，请重新选择")
	}
	var (
		spec        PublishedProductSpec
		autoDerived bool
		conversion  string
	)
	sql := fmt.Sprintf(`
		SELECT p.id,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN p.parent_product_id ELSE p.id END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.derived_spec_key,''),'')
		         ELSE COALESCE(NULLIF(default_spec.spec_key,''),NULLIF(p.derived_spec_key,''),'')
		       END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.derived_spec_name,''),NULLIF(p.spec_label,''),NULLIF(p.sku_name,''),NULLIF(p.derived_sales_unit,''),'')
		         ELSE COALESCE(NULLIF(default_spec.spec_name,''),NULLIF(p.derived_spec_name,''),NULLIF(p.spec_label,''),NULLIF(p.sku_name,''),p.name,'')
		       END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.spec_label,''),NULLIF(p.derived_spec_name,''),NULLIF(p.sku_name,''),NULLIF(p.derived_sales_unit,''),'')
		         ELSE COALESCE(NULLIF(default_spec.spec_label,''),NULLIF(default_spec.spec_name,''),NULLIF(p.spec_label,''),NULLIF(p.sku_name,''),p.name,'')
		       END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(NULLIF(p.derived_sales_unit,''),NULLIF(p.spec_label,''),NULLIF(p.sku_name,''),'')
		         ELSE COALESCE(
		           NULLIF(p.unit_rule_override_json->>'default_sales_unit',''),
		           NULLIF(p.unit_rule_override_json->>'order_unit',''),
		           NULLIF(p.unit_rule_override_json->>'quote_unit',''),
		           NULLIF(default_spec.sales_unit,''),
		           NULLIF(product_unit_template.order_unit,''),
		           NULLIF(product_unit_template.quote_unit,''),
		           NULLIF(p.derived_sales_unit,''),
		           NULLIF(p.spec_label,''),
		           NULLIF(p.sku_name,'')
		         )
		       END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(p.net_content_qty,0)::float8
		         ELSE COALESCE(NULLIF(p.net_content_qty,0),default_spec.net_content_qty,0)::float8
		       END,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0
		         THEN COALESCE(p.net_content_unit,'')
		         ELSE COALESCE(NULLIF(p.net_content_unit,''),default_spec.net_content_unit,'')
		       END,
		       COALESCE(NULLIF(p.product_kind,''),'roasted_bean'),
		       COALESCE(p.drip_box_bag_count,0),
		       COALESCE(p.drip_bag_grams,0)::float8,
		       CASE WHEN COALESCE(p.parent_product_id,0)>0 THEN COALESCE(
		         NULLIF(parent_product.unit_rule_override_json->>'inventory_unit',''),
		         NULLIF(parent_unit_template.inventory_unit,''),
		         NULLIF(parent_config.inventory_unit,''),
		         NULLIF(parent_category.inventory_unit,''),
		         NULLIF(parent_parent_category.inventory_unit,'')
		       ) ELSE COALESCE(
		         NULLIF(p.unit_rule_override_json->>'inventory_unit',''),
		         NULLIF(product_unit_template.inventory_unit,''),
		         NULLIF(product_config.inventory_unit,''),
		         NULLIF(product_category.inventory_unit,''),
		         NULLIF(parent_category_self.inventory_unit,'')
		       ) END,
		       COALESCE(
		         NULLIF(p.unit_rule_override_json->>'unit_conversion_json',''),
		         NULLIF(p.unit_rule_override_json->>'conversion_json',''),
		         NULLIF(product_unit_template.unit_conversion_json::text,'{}'),
		         NULLIF(product_config.unit_conversion_json::text,'{}'),
		         NULLIF(product_category.unit_conversion_json::text,'{}'),
		         NULLIF(parent_category_self.unit_conversion_json::text,'{}'),
		         '{}'
		       ),
		       COALESCE(p.auto_derived_sku,false)
		FROM %[1]s.products p
		LEFT JOIN %[1]s.products parent_product
		  ON parent_product.id=p.parent_product_id AND COALESCE(parent_product.active,true)=true
		LEFT JOIN %[1]s.product_unit_templates product_unit_template
		  ON product_unit_template.id=p.unit_template_id AND COALESCE(product_unit_template.active,true)=true
		LEFT JOIN LATERAL (
		  SELECT NULLIF(spec.row->>'spec_key','') AS spec_key,
		         NULLIF(spec.row->>'spec_name','') AS spec_name,
		         COALESCE(NULLIF(spec.row->>'spec_label',''),NULLIF(spec.row->>'spec_name','')) AS spec_label,
		         COALESCE(NULLIF(spec.row->>'sales_unit',''),NULLIF(spec.row->>'spec_name','')) AS sales_unit,
		         COALESCE(NULLIF(spec.row->>'net_content_qty','')::numeric,0)::float8 AS net_content_qty,
		         NULLIF(spec.row->>'net_content_unit','') AS net_content_unit
		  FROM jsonb_array_elements(COALESCE(product_unit_template.sales_specs_json,'[]'::jsonb)) WITH ORDINALITY AS spec(row,ord)
		  WHERE COALESCE(spec.row->>'active','true')<>'false'
		  ORDER BY CASE WHEN COALESCE(spec.row->>'default','false')='true' THEN 0 ELSE 1 END,spec.ord
		  LIMIT 1
		) default_spec ON true
		LEFT JOIN %[1]s.product_config_templates product_config
		  ON product_config.id=p.product_config_template_id AND COALESCE(product_config.active,true)=true
		LEFT JOIN %[1]s.product_categories product_category
		  ON product_category.id=p.product_category_id AND COALESCE(product_category.active,true)=true
		LEFT JOIN %[1]s.product_categories parent_category_self
		  ON parent_category_self.id=product_category.parent_id AND COALESCE(parent_category_self.active,true)=true
		LEFT JOIN %[1]s.product_unit_templates parent_unit_template
		  ON parent_unit_template.id=parent_product.unit_template_id AND COALESCE(parent_unit_template.active,true)=true
		LEFT JOIN %[1]s.product_config_templates parent_config
		  ON parent_config.id=parent_product.product_config_template_id AND COALESCE(parent_config.active,true)=true
		LEFT JOIN %[1]s.product_categories parent_category
		  ON parent_category.id=parent_product.product_category_id AND COALESCE(parent_category.active,true)=true
		LEFT JOIN %[1]s.product_categories parent_parent_category
		  ON parent_parent_category.id=parent_category.parent_id AND COALESCE(parent_parent_category.active,true)=true
		WHERE p.id=$1
		  AND COALESCE(p.active,true)=true
		  AND (NOT COALESCE(p.auto_derived_sku,false) OR COALESCE(NULLIF(p.derived_spec_status,''),'active')='active')
		  AND (COALESCE(p.parent_product_id,0)=0 OR parent_product.id IS NOT NULL)
	`, schema)
	if err := q.QueryRow(ctx, sql, productID).Scan(
		&spec.SKUID,
		&spec.ParentProductID,
		&spec.SpecKey,
		&spec.SpecName,
		&spec.SpecLabel,
		&spec.SalesUnit,
		&spec.NetContentQty,
		&spec.NetContentUnit,
		&spec.ProductKind,
		&spec.UnitBagCount,
		&spec.UnitBeanG,
		&spec.InventoryUnit,
		&conversion,
		&autoDerived,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublishedProductSpec{}, fmt.Errorf("商品规格已停用或不存在，请重新选择")
		}
		return PublishedProductSpec{}, err
	}
	spec.ProductFound = true
	spec.CurrentCatalogAuthority = true
	spec.QuantityBasis = "sales_spec_count"
	spec.InventoryConversionJSON = strings.TrimSpace(conversion)
	if autoDerived {
		factor := publishedNetContentInventoryFactor(spec.NetContentQty, spec.NetContentUnit, spec.InventoryUnit)
		if factor <= 0 && publishedUnitsEquivalent(spec.SalesUnit, spec.InventoryUnit) {
			factor = 1
		}
		// Generated SKU sales specs such as a drip bag or box are already
		// inventory-count units when no weight conversion applies. This mirrors
		// the catalog/costing authority used to derive those SKUs.
		if factor <= 0 {
			_, salesWeight := publishedWeightUnitGrams(spec.SalesUnit)
			_, inventoryWeight := publishedWeightUnitGrams(spec.InventoryUnit)
			if !salesWeight && !inventoryWeight {
				factor = 1
			}
		}
		if factor > 0 {
			spec.InventoryConversionJSON = productionInventoryConversionJSON(spec.SalesUnit, spec.InventoryUnit, factor)
		}
	}
	return spec, nil
}

func productionInventoryConversionJSON(salesUnit, inventoryUnit string, factor float64) string {
	if strings.TrimSpace(salesUnit) == "" || strings.TrimSpace(inventoryUnit) == "" || factor <= 0 {
		return "{}"
	}
	value := map[string]map[string]float64{
		strings.TrimSpace(salesUnit): {
			strings.TrimSpace(inventoryUnit): normalizePublishedInventoryFactor(factor),
		},
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func publishedInventoryConversionFactor(raw, salesUnit, inventoryUnit string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return 0
	}
	var graph map[string]any
	if json.Unmarshal([]byte(raw), &graph) != nil {
		return 0
	}
	from, ok := publishedUnitJSONValue(graph, salesUnit)
	if !ok {
		return 0
	}
	switch value := from.(type) {
	case float64:
		return normalizePublishedInventoryFactor(value)
	case json.Number:
		factor, _ := value.Float64()
		return normalizePublishedInventoryFactor(factor)
	case map[string]any:
		target, ok := publishedUnitJSONValue(value, inventoryUnit)
		if !ok {
			return 0
		}
		switch factor := target.(type) {
		case float64:
			return normalizePublishedInventoryFactor(factor)
		case json.Number:
			value, _ := factor.Float64()
			return normalizePublishedInventoryFactor(value)
		}
	}
	return 0
}

func publishedUnitJSONValue(values map[string]any, unit string) (any, bool) {
	for key, value := range values {
		if publishedUnitsEquivalent(key, unit) {
			return value, true
		}
	}
	return nil, false
}

func publishedNetContentInventoryFactor(qty float64, netContentUnit, inventoryUnit string) float64 {
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) {
		return 0
	}
	sourceGrams, sourceWeight := publishedWeightUnitGrams(netContentUnit)
	targetGrams, targetWeight := publishedWeightUnitGrams(inventoryUnit)
	if sourceWeight && targetWeight && targetGrams > 0 {
		return normalizePublishedInventoryFactor(qty * sourceGrams / targetGrams)
	}
	if publishedUnitsEquivalent(netContentUnit, inventoryUnit) {
		return normalizePublishedInventoryFactor(qty)
	}
	return 0
}

func publishedUnitsEquivalent(left, right string) bool {
	leftGrams, leftWeight := publishedWeightUnitGrams(left)
	rightGrams, rightWeight := publishedWeightUnitGrams(right)
	if leftWeight || rightWeight {
		return leftWeight && rightWeight && leftGrams == rightGrams
	}
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func publishedWeightUnitGrams(unit string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "gram", "克":
		return 1, true
	case "kg", "kilogram", "千克", "公斤":
		return 1000, true
	case "lb", "lbs", "磅":
		return 453.59237, true
	default:
		return 0, false
	}
}

func normalizePublishedInventoryFactor(value float64) float64 {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*1e12) / 1e12
}

func firstNonEmptyProductionString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
