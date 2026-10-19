package freight

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	conversion "orderapp/internal/domain/specconversion"
	"strconv"
	"strings"
)

type WeightItem struct{ ProductID, BomSpecID, BomVariantID, Qty int64 }

func payloadInt64(m map[string]any, k string) int64   { return int64(floatFromAny(m[k])) }
func payloadString(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func floatFromAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case json.Number:
		f, _ := n.Float64()
		return f
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}
func relationExists(ctx context.Context, q Queryer, name string) bool {
	var ok bool
	return q.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok) == nil && ok
}

var _ pgx.Tx

func weightInGrams(qty float64, unit string) int64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
	case "kg", "千克", "公斤":
		qty *= 1000
	case "lb", "lbs", "磅":
		qty *= 454
	default:
		return 0
	}
	if qty <= 0 || math.IsNaN(qty) || math.IsInf(qty, 0) || qty > 1000000000 {
		return 0
	}
	return int64(math.Ceil(qty))
}
func (r *Repository) ItemNetWeightG(ctx context.Context, q Queryer, customerID int64, item WeightItem) (int64, error) {
	if item.BomSpecID > 0 {
		if grams, err := r.structuredBOMSpecWeightG(ctx, q, customerID, item); err != nil || grams > 0 {
			return grams, err
		}
		// A BOM name is presentation, never a source of shipping weight. Use the frozen published specification.
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT p.content_json FROM %[1]s.customer_order_price_table_bindings b JOIN %[1]s.bean_list_publications p ON p.id=b.publication_id WHERE b.customer_id=$1 AND b.usage_code IN ('direct_ship','product_order') AND p.status='published' AND p.deleted_at IS NULL`, r.schema), customerID)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		found := int64(0)
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				return 0, err
			}
			var content struct {
				Rows []map[string]any `json:"price_rows"`
			}
			if err = json.Unmarshal(raw, &content); err != nil {
				return 0, err
			}
			for _, row := range content.Rows {
				spec, _ := row["effective_sales_spec"].(map[string]any)
				id := payloadInt64(row, "bom_spec_id")
				if id == 0 {
					id = payloadInt64(spec, "bom_spec_id")
				}
				product := payloadInt64(row, "parent_product_id")
				if product == 0 {
					product = payloadInt64(row, "product_id")
				}
				if product == 0 {
					product = payloadInt64(spec, "parent_product_id")
					if product == 0 {
						product = payloadInt64(spec, "product_id")
					}
				}
				if id != item.BomSpecID || product != item.ProductID {
					continue
				}
				n := floatFromAny(spec["net_content_qty"])
				u := payloadString(spec, "net_content_unit")
				if n <= 0 {
					n = floatFromAny(row["net_content_qty"])
					u = payloadString(row, "net_content_unit")
				}
				if n <= 0 {
					sku, _ := row["sku_snapshot"].(map[string]any)
					n = floatFromAny(sku["net_content_qty"])
					u = payloadString(sku, "net_content_unit")
				}
				g := weightInGrams(n, u)
				if g > 0 {
					if found > 0 && found != g {
						return 0, fmt.Errorf("已发布规格净重存在冲突，请核对价格表")
					}
					found = g
				}
			}
		}
		if err = rows.Err(); err != nil {
			return 0, err
		}
		if found > 0 {
			return found, nil
		}
		return 0, fmt.Errorf("商品规格未配置已发布净重，无法计算运费")
	}
	var qty float64
	var unit string
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT net_content_qty::float8,net_content_unit FROM %s.products WHERE id=$1 AND active=true AND COALESCE(customer_id,0) IN (0,$2)`, r.schema), item.ProductID, customerID).Scan(&qty, &unit)
	if err != nil {
		return 0, fmt.Errorf("商品不存在或不可用")
	}
	g := weightInGrams(qty, unit)
	if g <= 0 {
		return 0, fmt.Errorf("商品未配置净重，无法计算运费")
	}
	return g, nil
}

func (r *Repository) structuredBOMSpecWeightG(ctx context.Context, q Queryer, customerID int64, item WeightItem) (int64, error) {
	var identity conversion.Identity
	var currentVariant int64
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT spec.id,COALESCE(spec.source_spec_template_id,0),COALESCE(spec.source_spec_template_key,''),variant.inventory_unit,variant.id
 FROM %[1]s.production_bom_output_bindings binding JOIN %[1]s.production_bom_versions version ON version.id=binding.bom_version_id AND version.status='published'
 JOIN %[1]s.production_bom_specs spec ON spec.bom_id=binding.bom_id AND spec.id=$2
 JOIN %[1]s.production_bom_version_variants variant ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
 WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true AND ($3::bigint=0 OR variant.id=$3)`, r.schema), item.ProductID, item.BomSpecID, item.BomVariantID).Scan(&identity.SpecID, &identity.TemplateID, &identity.TemplateKey, &identity.InventoryUnit, &currentVariant)
	if err != nil {
		return 0, fmt.Errorf("商品规格不存在或未发布，无法计算运费")
	}
	if grams := weightInGrams(1, identity.InventoryUnit); grams > 0 {
		return grams, nil
	}
	if relationExists(ctx, q, r.schema+".spec_conversion_groups") {
		rows, e := q.Query(ctx, fmt.Sprintf(`SELECT id,name,base_unit,revision,members_json FROM %s.spec_conversion_groups WHERE active=true ORDER BY id`, r.schema))
		if e != nil {
			return 0, e
		}
		found := int64(0)
		for rows.Next() {
			var g conversion.Group
			var raw []byte
			if e = rows.Scan(&g.ID, &g.Name, &g.BaseUnit, &g.Revision, &raw); e != nil {
				rows.Close()
				return 0, e
			}
			if e = json.Unmarshal(raw, &g.Members); e != nil {
				rows.Close()
				return 0, e
			}
			member, e := g.Match(identity)
			if e != nil {
				continue
			}
			qty, e := strconv.ParseFloat(member.BaseQuantity, 64)
			if e != nil {
				rows.Close()
				return 0, fmt.Errorf("规格重量换算配置无效")
			}
			grams := weightInGrams(qty, g.BaseUnit)
			if grams > 0 {
				if found > 0 && found != grams {
					rows.Close()
					return 0, fmt.Errorf("规格存在冲突的净重换算关系")
				}
				found = grams
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return 0, err
		}
		if found > 0 {
			return found, nil
		}
	}
	// A retained sale freezes the exact specification weight independently of a current dropship price list.
	rows, err := q.Query(ctx, fmt.Sprintf(`SELECT item.price_source_json FROM %[1]s.order_items item JOIN %[1]s.orders sale ON sale.id=item.order_id WHERE sale.customer_id=$1 AND COALESCE(sale.is_void,false)=false AND COALESCE(to_jsonb(sale)->>'delivery_mode','')='customer_stock' AND item.product_id=$2 AND item.bom_spec_id=$3 ORDER BY sale.id DESC,item.id DESC`, r.schema), customerID, item.ProductID, item.BomSpecID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return 0, err
		}
		var source map[string]any
		if json.Unmarshal(raw, &source) != nil {
			continue
		}
		spec, _ := source["effective_sales_spec"].(map[string]any)
		if grams := weightInGrams(floatFromAny(spec["net_content_qty"]), payloadString(spec, "net_content_unit")); grams > 0 {
			return grams, nil
		}
		frozen, _ := source["production_quantity_snapshot"].(map[string]any)
		if grams := weightInGrams(floatFromAny(frozen["inventory_qty_per_sales_unit"]), payloadString(frozen, "inventory_unit")); grams > 0 {
			return grams, nil
		}
	}
	return 0, rows.Err()
}
