package costing

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type productionBomCostItem struct {
	ID                  int64
	ComponentType       string
	ComponentMaterialID int64
	ComponentProductID  int64
	ComponentSpecG      int64
	ConsumeUnit         string
	QtyPerUnit          float64
	RatioPct            float64
	MaterialLossRate    float64
	UnitCost            float64
	UnitCostUnit        string
}

type productionBomCostNode struct {
	OutputType           string
	OutputID             int64
	ProductID            int64
	VersionID            int64
	YieldRate            float64
	OutputQty            float64
	OutputUnit           string
	OperationCostPerUnit float64
	Items                []productionBomCostItem
}

type productionBomResolvedItemCost struct {
	UnitCost                  float64
	CostUnit                  string
	ContributionBeforeYield   float64
	ContributionPerOutputUnit float64
}

type productionBomResolvedCost struct {
	ProductID                        int64
	VersionID                        int64
	OutputUnit                       string
	InputCostPerOutputUnit           float64
	OperationCostPerOutputUnit       float64
	TotalCostPerOutputUnit           float64
	HasProductComponent              bool
	HasManufacturedMaterialComponent bool
	Resolved                         bool
	ItemCosts                        map[int64]productionBomResolvedItemCost
}

func resolveProductionBomCosts(nodes map[int64]productionBomCostNode) map[int64]productionBomResolvedCost {
	typed := make(map[string]productionBomCostNode, len(nodes))
	for productID, node := range nodes {
		if node.ProductID <= 0 {
			node.ProductID = productID
		}
		node.OutputType = "product"
		node.OutputID = node.ProductID
		typed[productionBomCostOutputKey(node.OutputType, node.OutputID)] = node
	}
	all := resolveTypedProductionBomCosts(typed)
	resolved := make(map[int64]productionBomResolvedCost, len(nodes))
	for productID := range nodes {
		resolved[productID] = all[productionBomCostOutputKey("product", productID)]
	}
	return resolved
}

func resolveTypedProductionBomCosts(nodes map[string]productionBomCostNode) map[string]productionBomResolvedCost {
	resolved := make(map[string]productionBomResolvedCost, len(nodes))
	state := make(map[string]uint8, len(nodes))

	var resolve func(string) productionBomResolvedCost
	resolve = func(key string) productionBomResolvedCost {
		if state[key] == 2 {
			return resolved[key]
		}
		node, ok := nodes[key]
		if state[key] == 1 {
			return productionBomResolvedCost{ProductID: node.ProductID}
		}
		outputType := normalizeProductionBomCostOutputType(node.OutputType)
		outputID := node.OutputID
		if outputID <= 0 && outputType == "product" {
			outputID = node.ProductID
		}
		if !ok || outputID <= 0 || node.VersionID <= 0 {
			result := productionBomResolvedCost{ProductID: node.ProductID, VersionID: node.VersionID, OutputUnit: normalizeProductionBomCostUnit(node.OutputUnit)}
			resolved[key] = result
			state[key] = 2
			return result
		}

		state[key] = 1
		result := productionBomResolvedCost{
			ProductID:  node.ProductID,
			VersionID:  node.VersionID,
			OutputUnit: normalizeProductionBomCostUnit(node.OutputUnit),
			ItemCosts:  make(map[int64]productionBomResolvedItemCost, len(node.Items)),
		}
		outputQty, outputQtyOK := normalizedProductionBomOutputQty(node.OutputQty)
		inputCost := 0.0
		valid := outputQtyOK && finiteNonNegative(node.OperationCostPerUnit)
		for _, item := range node.Items {
			if normalizeProductionBomComponentType(item.ComponentType) == "product" {
				result.HasProductComponent = true
			}
		}
		for _, item := range node.Items {
			componentType := normalizeProductionBomComponentType(item.ComponentType)
			unitCost := item.UnitCost
			costUnit := strings.TrimSpace(item.UnitCostUnit)
			componentKey := ""
			if componentType == "product" && item.ComponentProductID > 0 {
				componentKey = productionBomCostOutputKey("product", item.ComponentProductID)
			} else if componentType == "material" && item.ComponentMaterialID > 0 {
				candidate := productionBomCostOutputKey("material", item.ComponentMaterialID)
				if _, manufactured := nodes[candidate]; manufactured {
					componentKey = candidate
					result.HasManufacturedMaterialComponent = true
				}
			}
			if componentKey != "" {
				component := resolve(componentKey)
				if !component.Resolved {
					valid = false
					break
				}
				unitCost = component.TotalCostPerOutputUnit
				costUnit = component.OutputUnit
			}
			amount, ok := productionBomItemCost(item, componentType, unitCost, costUnit, result.OutputUnit)
			if !ok {
				valid = false
				break
			}
			amountPerOutputUnit, ok := productionBomItemCostPerOutputUnit(item, amount, outputQty)
			if !ok {
				valid = false
				break
			}
			inputCost += amountPerOutputUnit
			result.ItemCosts[item.ID] = productionBomResolvedItemCost{
				UnitCost:                  unitCost,
				CostUnit:                  costUnit,
				ContributionBeforeYield:   amount,
				ContributionPerOutputUnit: amountPerOutputUnit,
			}
		}
		if valid {
			result.InputCostPerOutputUnit = inputCost
			result.OperationCostPerOutputUnit = node.OperationCostPerUnit
			result.TotalCostPerOutputUnit = inputCost + node.OperationCostPerUnit
			result.Resolved = finiteNonNegative(result.TotalCostPerOutputUnit)
		}
		if !result.Resolved {
			result.InputCostPerOutputUnit = 0
			result.OperationCostPerOutputUnit = 0
			result.TotalCostPerOutputUnit = 0
			result.ItemCosts = nil
		}
		resolved[key] = result
		state[key] = 2
		return result
	}

	for key := range nodes {
		resolve(key)
	}
	return resolved
}

func productionBomCostOutputKey(outputType string, outputID int64) string {
	return fmt.Sprintf("%s:%d", normalizeProductionBomCostOutputType(outputType), outputID)
}

func normalizeProductionBomCostOutputType(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "material") {
		return "material"
	}
	return "product"
}

func (r Repository) loadResolvedProductionBomCosts(ctx context.Context) (map[int64]productionBomResolvedCost, error) {
	if r.pool == nil {
		return map[int64]productionBomResolvedCost{}, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT lower(COALESCE(NULLIF(binding.output_type,''),'product')) AS output_type,
		       binding.output_id,
		       CASE WHEN lower(COALESCE(NULLIF(binding.output_type,''),'product'))='product' THEN binding.output_id ELSE 0 END AS product_id,
		       version.id AS version_id,
		       COALESCE(version.yield_rate,0)::float8 AS yield_rate,
		       COALESCE(NULLIF(version.output_qty,0),1)::float8 AS output_qty,
		       COALESCE(NULLIF(version.output_unit,''),'unit') AS output_unit,
		       COALESCE(SUM(oc.operation_unit_cost),0)::float8 AS operation_cost_per_unit
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_boms bom
		  ON bom.id=binding.bom_id
		 AND lower(COALESCE(NULLIF(bom.output_type,''),'product'))=lower(COALESCE(NULLIF(binding.output_type,''),'product'))
		 AND CASE WHEN lower(COALESCE(NULLIF(bom.output_type,''),'product'))='material'
		          THEN bom.output_material_id ELSE bom.output_product_id END=binding.output_id
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=bom.id
		 AND version.status='published'
		LEFT JOIN %[1]s.production_bom_version_operation_costs oc
		  ON oc.version_id=version.id
		 AND COALESCE(NULLIF(oc.cost_method,''),'time')='time'
		 AND (NULLIF(oc.operation_cost_unit,'') IS NULL OR lower(oc.operation_cost_unit)=lower(version.output_unit))
		WHERE binding.is_default=true
		  AND COALESCE(NULLIF(bom.status,''),'active')='active'
		GROUP BY binding.output_type, binding.output_id, version.id, version.yield_rate, version.output_qty, version.output_unit
		ORDER BY output_type, binding.output_id
	`, r.schema))
	if err != nil {
		return nil, err
	}
	nodes := map[string]productionBomCostNode{}
	versionOutputKeys := map[int64]string{}
	versionIDs := make([]int64, 0)
	for rows.Next() {
		var node productionBomCostNode
		if err := rows.Scan(&node.OutputType, &node.OutputID, &node.ProductID, &node.VersionID, &node.YieldRate, &node.OutputQty, &node.OutputUnit, &node.OperationCostPerUnit); err != nil {
			rows.Close()
			return nil, err
		}
		key := productionBomCostOutputKey(node.OutputType, node.OutputID)
		nodes[key] = node
		versionOutputKeys[node.VersionID] = key
		versionIDs = append(versionIDs, node.VersionID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(versionIDs) == 0 {
		return map[int64]productionBomResolvedCost{}, nil
	}

	itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH material_valuation AS (
			SELECT l.material_id,
			       SUM((CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.cost_unit,''), NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END) * COALESCE(b.unit_cost,0))
			       / NULLIF(SUM(CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.cost_unit,''), NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END),0) AS weighted_unit_cost
			FROM %[1]s.material_batch_locations l
			JOIN %[1]s.material_batches b ON b.id=l.material_batch_id
			JOIN %[1]s.materials m ON m.id=l.material_id
			WHERE (l.qty_g > 0 OR l.qty_units > 0)
			  AND b.status='active'
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			GROUP BY l.material_id
		)
		SELECT i.version_id,
		       i.id,
		       COALESCE(NULLIF(i.component_type,''),'material') AS component_type,
		       COALESCE(i.material_id,0),
		       COALESCE(i.component_product_id,0),
		       COALESCE(i.component_spec_g,0),
		       COALESCE(NULLIF(i.consume_unit,''),'ratio_pct') AS consume_unit,
		       COALESCE(i.qty_per_unit,0)::float8,
		       COALESCE(i.ratio_pct,0)::float8,
		       COALESCE(i.material_loss_rate,0)::float8,
		       COALESCE(NULLIF(mv.weighted_unit_cost,0), NULLIF(m.purchase_price,0), NULLIF(i.unit_cost_snapshot,0), 0)::float8 AS unit_cost,
		       COALESCE(NULLIF(m.cost_unit,''), 'kg') AS unit_cost_unit
		FROM %[1]s.production_bom_version_items i
		LEFT JOIN %[1]s.materials m ON m.id=i.material_id
		LEFT JOIN material_valuation mv ON mv.material_id=i.material_id
		WHERE i.version_id=ANY($1)
		ORDER BY i.version_id, i.id
	`, r.schema), versionIDs)
	if err != nil {
		return nil, err
	}
	for itemRows.Next() {
		var versionID int64
		var item productionBomCostItem
		if err := itemRows.Scan(
			&versionID,
			&item.ID,
			&item.ComponentType,
			&item.ComponentMaterialID,
			&item.ComponentProductID,
			&item.ComponentSpecG,
			&item.ConsumeUnit,
			&item.QtyPerUnit,
			&item.RatioPct,
			&item.MaterialLossRate,
			&item.UnitCost,
			&item.UnitCostUnit,
		); err != nil {
			itemRows.Close()
			return nil, err
		}
		outputKey := versionOutputKeys[versionID]
		node := nodes[outputKey]
		node.Items = append(node.Items, item)
		nodes[outputKey] = node
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return nil, err
	}
	itemRows.Close()
	all := resolveTypedProductionBomCosts(nodes)
	productCosts := make(map[int64]productionBomResolvedCost)
	for key, node := range nodes {
		if normalizeProductionBomCostOutputType(node.OutputType) == "product" && node.OutputID > 0 {
			productCosts[node.OutputID] = all[key]
		}
	}
	return productCosts, nil
}

func productionBomCostForProduct(costs map[int64]productionBomResolvedCost, productID int64, parentProductID int64) (productionBomResolvedCost, bool) {
	if cost, ok := costs[productID]; ok {
		return cost, true
	}
	if parentProductID > 0 {
		if cost, ok := costs[parentProductID]; ok {
			return cost, true
		}
	}
	return productionBomResolvedCost{}, false
}

func resolveProductionBomTrialItemCost(item productionBomCostItem, materialUnitCost float64, materialCostUnit string, _ float64, bomOutputQty float64, bomOutputUnit string, costs map[int64]productionBomResolvedCost) (productionBomResolvedItemCost, bool, string) {
	componentType := normalizeProductionBomComponentType(item.ComponentType)
	unitCost := materialUnitCost
	costUnit := strings.TrimSpace(materialCostUnit)
	if componentType == "product" {
		componentCost, ok := costs[item.ComponentProductID]
		if !ok || !componentCost.Resolved {
			return productionBomResolvedItemCost{}, false, "组件商品没有可用且可完整解析的已发布生产 BOM 成本"
		}
		unitCost = componentCost.TotalCostPerOutputUnit
		costUnit = componentCost.OutputUnit
	}
	outputQty, ok := normalizedProductionBomOutputQty(bomOutputQty)
	if !ok {
		return productionBomResolvedItemCost{}, false, "BOM产出数量必须大于 0"
	}
	amount, ok := productionBomItemCost(item, componentType, unitCost, costUnit, bomOutputUnit)
	if !ok {
		return productionBomResolvedItemCost{}, false, "BOM组件成本单位无法换算"
	}
	amountPerOutputUnit, ok := productionBomItemCostPerOutputUnit(item, amount, outputQty)
	if !ok {
		return productionBomResolvedItemCost{}, false, "BOM组件成本无法按产出基准折算"
	}
	return productionBomResolvedItemCost{
		UnitCost:                  unitCost,
		CostUnit:                  costUnit,
		ContributionBeforeYield:   amount,
		ContributionPerOutputUnit: amountPerOutputUnit,
	}, true, ""
}

func normalizeProductionBomComponentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "product", "finished_product":
		return "product"
	default:
		return "material"
	}
}

func productionBomItemCost(item productionBomCostItem, componentType string, unitCost float64, costUnit string, outputUnit string) (float64, bool) {
	if !finiteNonNegative(unitCost) {
		return 0, false
	}
	consumeUnit := strings.ToLower(strings.TrimSpace(item.ConsumeUnit))
	if consumeUnit == "" {
		consumeUnit = "ratio_pct"
	}
	qty := item.QtyPerUnit
	if qty <= 0 && item.ComponentSpecG > 0 {
		qty = float64(item.ComponentSpecG)
	}
	hasUsage := (consumeUnit == "ratio_pct" && item.RatioPct > 0) || (consumeUnit != "ratio_pct" && qty > 0)
	if hasUsage && unitCost <= 0 {
		return 0, false
	}
	switch consumeUnit {
	case "ratio_pct":
		if componentType == "product" || item.RatioPct < 0 || item.RatioPct > 100 {
			return 0, false
		}
		sourceMassKg := productionBomCostMassKgFactor(costUnit)
		outputMassKg := productionBomCostMassKgFactor(outputUnit)
		if sourceMassKg > 0 && outputMassKg > 0 {
			unitCost = unitCost * outputMassKg / sourceMassKg
		}
		lossRate := item.MaterialLossRate
		if lossRate < 0 || lossRate >= 1 {
			return 0, false
		}
		return unitCost * item.RatioPct / 100 / (1 - lossRate), true
	case "g", "g_per_bag":
		perKg, ok := productionBomCostPerKg(unitCost, costUnit)
		if !ok || qty < 0 {
			return 0, false
		}
		return qty / 1000 * perKg, true
	case "kg":
		perKg, ok := productionBomCostPerKg(unitCost, costUnit)
		if !ok || qty < 0 {
			return 0, false
		}
		return qty * perKg, true
	case "lb", "lbs", "磅":
		perKg, ok := productionBomCostPerKg(unitCost, costUnit)
		if !ok || qty < 0 {
			return 0, false
		}
		return qty * 0.45359237 * perKg, true
	case "unit_per_bag", "unit_per_box", "fixed_qty", "unit", "length", "area":
		if qty < 0 {
			return 0, false
		}
		return qty * unitCost, true
	default:
		if qty < 0 || strings.TrimSpace(costUnit) == "" || !strings.EqualFold(strings.TrimSpace(costUnit), strings.TrimSpace(item.ConsumeUnit)) {
			return 0, false
		}
		return qty * unitCost, true
	}
}

func productionBomItemCostPerOutputUnit(item productionBomCostItem, amount float64, outputQty float64) (float64, bool) {
	if !finiteNonNegative(amount) || outputQty <= 0 || math.IsNaN(outputQty) || math.IsInf(outputQty, 0) {
		return 0, false
	}
	switch strings.ToLower(strings.TrimSpace(item.ConsumeUnit)) {
	case "", "ratio_pct":
		return amount, true
	case "g_per_bag", "unit_per_bag", "unit_per_box":
		return amount, true
	default:
		return amount / outputQty, true
	}
}

func normalizedProductionBomOutputQty(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, false
	}
	if value == 0 {
		return 1, true
	}
	return value, true
}

func productionBomCostPerKg(unitCost float64, unit string) (float64, bool) {
	factor := productionBomCostMassKgFactor(unit)
	if !finiteNonNegative(unitCost) || factor <= 0 {
		return 0, false
	}
	return unitCost / factor, true
}

func productionBomCostMassKgFactor(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "公斤", "千克":
		return 1
	case "g", "克":
		return 0.001
	case "lb", "lbs", "磅":
		return 0.45359237
	default:
		return 0
	}
}

func normalizeProductionBomCostUnit(unit string) string {
	unit = strings.TrimSpace(unit)
	if unit == "" {
		return "unit"
	}
	return unit
}

func finiteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
