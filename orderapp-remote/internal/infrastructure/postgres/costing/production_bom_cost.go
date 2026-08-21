package costing

import (
	"context"
	"fmt"
	"math"
	"strings"

	appcosting "orderapp/internal/application/costing"
)

type productionBomCostItem struct {
	ID                    int64
	ComponentType         string
	ComponentMaterialID   int64
	ComponentIsSemi       bool
	ComponentProductID    int64
	ComponentBomSpecID    int64
	ComponentSpecG        int64
	ConsumeUnit           string
	QtyPerUnit            float64
	RatioPct              float64
	MaterialLossRate      float64
	UnitCost              float64
	UnitCostUnit          string
	ComponentName         string
	ComponentMaterialName string
	ComponentProductName  string
	PurchasePrice         float64
	WeightedBatchUnitCost float64
	UnitCostSnapshot      float64
}

type productionBomCostNode struct {
	OutputType           string
	OutputID             int64
	ProductID            int64
	VersionID            int64
	VariantID            int64
	BomSpecID            int64
	MaterialLossRate     float64
	YieldRate            float64
	OutputQty            float64
	OutputUnit           string
	OperationCostPerUnit float64
	BomID                int64
	BomName              string
	VersionNo            string
	OutputName           string
	Items                []productionBomCostItem
}

type productionBomResolvedItemCost struct {
	UnitCost                  float64
	CostUnit                  string
	ContributionBeforeYield   float64
	ContributionPerOutputUnit float64
}

type productionBomResolvedCost struct {
	OutputType                        string
	OutputID                          int64
	ProductID                         int64
	BomID                             int64
	BomName                           string
	VersionID                         int64
	VersionNo                         string
	BomSpecID                         int64
	BomVariantID                      int64
	OutputName                        string
	OutputUnit                        string
	InputCostPerOutputUnit            float64
	OperationCostPerOutputUnit        float64
	TotalCostPerOutputUnit            float64
	HasProductComponent               bool
	HasManufacturedMaterialComponent  bool
	Resolved                          bool
	ItemCosts                         map[int64]productionBomResolvedItemCost
	CostStatus                        string
	PartialInputCostPerOutputUnit     float64
	PartialOperationCostPerOutputUnit float64
	PartialTotalCostPerOutputUnit     float64
	UnresolvedIssues                  []appcosting.PricingRuleTrialCostIssue
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

	pathNode := func(node productionBomCostNode) appcosting.PricingRuleTrialCostPathNode {
		return appcosting.PricingRuleTrialCostPathNode{
			OutputType: node.OutputType,
			OutputID:   node.OutputID,
			ProductID:  node.ProductID,
			BomID:      node.BomID,
			BomName:    node.BomName,
			VersionID:  node.VersionID,
			VersionNo:  node.VersionNo,
			OutputName: node.OutputName,
		}
	}
	issueForItem := func(node productionBomCostNode, item productionBomCostItem, code, reason string, path []appcosting.PricingRuleTrialCostPathNode) appcosting.PricingRuleTrialCostIssue {
		return appcosting.PricingRuleTrialCostIssue{
			Code:                  code,
			Reason:                reason,
			ComponentType:         normalizeProductionBomComponentType(item.ComponentType),
			ComponentID:           item.ID,
			ComponentMaterialID:   item.ComponentMaterialID,
			ComponentProductID:    item.ComponentProductID,
			ComponentBomSpecID:    item.ComponentBomSpecID,
			ComponentName:         item.ComponentName,
			ComponentMaterialName: item.ComponentMaterialName,
			ComponentProductName:  item.ComponentProductName,
			IsSemiFinished:        item.ComponentIsSemi,
			ConsumeUnit:           item.ConsumeUnit,
			CostUnit:              item.UnitCostUnit,
			Quantity:              item.QtyPerUnit,
			UnitCost:              item.UnitCost,
			PurchasePrice:         item.PurchasePrice,
			WeightedBatchUnitCost: item.WeightedBatchUnitCost,
			UnitCostSnapshot:      item.UnitCostSnapshot,
			RootOutputType:        node.OutputType,
			RootOutputID:          node.OutputID,
			RootProductID:         node.ProductID,
			BomID:                 node.BomID,
			BomName:               node.BomName,
			VersionID:             node.VersionID,
			VersionNo:             node.VersionNo,
			BomSpecID:             node.BomSpecID,
			BomVariantID:          node.VariantID,
			Path:                  append([]appcosting.PricingRuleTrialCostPathNode(nil), path...),
		}
	}

	var resolve func(string, []appcosting.PricingRuleTrialCostPathNode) productionBomResolvedCost
	resolve = func(key string, path []appcosting.PricingRuleTrialCostPathNode) productionBomResolvedCost {
		if state[key] == 2 {
			return resolved[key]
		}
		node, ok := nodes[key]
		if state[key] == 1 {
			result := productionBomResolvedCost{OutputType: node.OutputType, OutputID: node.OutputID, ProductID: node.ProductID, BomID: node.BomID, BomName: node.BomName, VersionID: node.VersionID, VersionNo: node.VersionNo, BomSpecID: node.BomSpecID, BomVariantID: node.VariantID, OutputName: node.OutputName, CostStatus: "incomplete"}
			result.UnresolvedIssues = []appcosting.PricingRuleTrialCostIssue{{
				Code:           "bom_cycle",
				Reason:         "检测到 BOM 递归循环，无法解析组件成本",
				ComponentType:  node.OutputType,
				ComponentID:    node.OutputID,
				ComponentName:  node.OutputName,
				BomID:          node.BomID,
				BomName:        node.BomName,
				VersionID:      node.VersionID,
				VersionNo:      node.VersionNo,
				RootOutputType: node.OutputType,
				RootOutputID:   node.OutputID,
				RootProductID:  node.ProductID,
				Path:           append([]appcosting.PricingRuleTrialCostPathNode(nil), path...),
			}}
			return result
		}
		outputType := normalizeProductionBomCostOutputType(node.OutputType)
		outputID := node.OutputID
		if outputID <= 0 && outputType == "product" {
			outputID = node.ProductID
		}
		if !ok || outputID <= 0 || node.VersionID <= 0 {
			result := productionBomResolvedCost{OutputType: node.OutputType, OutputID: outputID, ProductID: node.ProductID, BomID: node.BomID, BomName: node.BomName, VersionID: node.VersionID, VersionNo: node.VersionNo, BomSpecID: node.BomSpecID, BomVariantID: node.VariantID, OutputName: node.OutputName, OutputUnit: normalizeProductionBomCostUnit(node.OutputUnit), CostStatus: "incomplete"}
			result.UnresolvedIssues = []appcosting.PricingRuleTrialCostIssue{{
				Code:           "bom_not_found",
				Reason:         "组件没有可用的已发布制造 BOM",
				ComponentType:  node.OutputType,
				ComponentID:    outputID,
				ComponentName:  node.OutputName,
				BomID:          node.BomID,
				BomName:        node.BomName,
				VersionID:      node.VersionID,
				VersionNo:      node.VersionNo,
				RootOutputType: node.OutputType,
				RootOutputID:   outputID,
				RootProductID:  node.ProductID,
				Path:           append([]appcosting.PricingRuleTrialCostPathNode(nil), path...),
			}}
			resolved[key] = result
			state[key] = 2
			return result
		}

		state[key] = 1
		result := productionBomResolvedCost{
			OutputType:   node.OutputType,
			OutputID:     node.OutputID,
			ProductID:    node.ProductID,
			BomID:        node.BomID,
			BomName:      node.BomName,
			VersionID:    node.VersionID,
			VersionNo:    node.VersionNo,
			BomSpecID:    node.BomSpecID,
			BomVariantID: node.VariantID,
			OutputName:   node.OutputName,
			OutputUnit:   normalizeProductionBomCostUnit(node.OutputUnit),
			ItemCosts:    make(map[int64]productionBomResolvedItemCost, len(node.Items)),
			CostStatus:   "incomplete",
		}
		outputQty, outputQtyOK := normalizedProductionBomOutputQty(node.OutputQty)
		inputCost := 0.0
		valid := outputQtyOK && finiteNonNegative(node.OperationCostPerUnit)
		currentPath := append(append([]appcosting.PricingRuleTrialCostPathNode(nil), path...), pathNode(node))
		if !outputQtyOK {
			result.UnresolvedIssues = append(result.UnresolvedIssues, issueForItem(node, productionBomCostItem{}, "invalid_output_qty", "BOM产出数量必须大于 0", currentPath))
		}
		if !finiteNonNegative(node.OperationCostPerUnit) {
			result.UnresolvedIssues = append(result.UnresolvedIssues, issueForItem(node, productionBomCostItem{}, "invalid_operation_cost", "BOM工序成本无效", currentPath))
		}
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
				if item.ComponentBomSpecID > 0 {
					componentKey = productionBomCostOutputKey("product_spec", item.ComponentBomSpecID)
				} else {
					componentKey = productionBomCostOutputKey("product", item.ComponentProductID)
				}
			} else if componentType == "material" && item.ComponentMaterialID > 0 {
				candidate := productionBomCostOutputKey("material", item.ComponentMaterialID)
				if _, manufactured := nodes[candidate]; manufactured || item.ComponentIsSemi {
					componentKey = candidate
					result.HasManufacturedMaterialComponent = true
				}
			}
			if componentKey != "" {
				component := resolve(componentKey, currentPath)
				if !component.Resolved {
					valid = false
					if len(component.UnresolvedIssues) == 0 {
						result.UnresolvedIssues = append(result.UnresolvedIssues, issueForItem(node, item, "component_cost_unresolved", "组件商品没有可用且可完整解析的已发布生产 BOM 成本", currentPath))
					} else {
						for _, issue := range component.UnresolvedIssues {
							if len(issue.Path) == 0 {
								issue.Path = append([]appcosting.PricingRuleTrialCostPathNode(nil), currentPath...)
							}
							issue.RootOutputType = node.OutputType
							issue.RootOutputID = node.OutputID
							issue.RootProductID = node.ProductID
							issue.BomID = node.BomID
							issue.BomName = node.BomName
							issue.VersionID = node.VersionID
							issue.VersionNo = node.VersionNo
							issue.BomSpecID = node.BomSpecID
							issue.BomVariantID = node.VariantID
							result.UnresolvedIssues = append(result.UnresolvedIssues, issue)
						}
					}
					continue
				}
				unitCost = component.TotalCostPerOutputUnit
				costUnit = component.OutputUnit
			}
			amount, ok := productionBomItemCost(item, componentType, unitCost, costUnit, result.OutputUnit)
			if !ok {
				valid = false
				code, reason := "component_cost_unresolved", "BOM组件成本无法解析"
				if unitCost <= 0 {
					code, reason = "zero_component_cost", "BOM组件单价为 0：请维护物料采购价；半成品物料需绑定默认已发布的制造 BOM"
				}
				result.UnresolvedIssues = append(result.UnresolvedIssues, issueForItem(node, item, code, reason, currentPath))
				continue
			}
			amountPerOutputUnit, ok := productionBomItemCostPerOutputUnit(item, amount, outputQty)
			if !ok {
				valid = false
				result.UnresolvedIssues = append(result.UnresolvedIssues, issueForItem(node, item, "component_cost_unresolved", "BOM组件成本无法按产出基准折算", currentPath))
				continue
			}
			inputCost += amountPerOutputUnit
			result.ItemCosts[item.ID] = productionBomResolvedItemCost{
				UnitCost:                  unitCost,
				CostUnit:                  costUnit,
				ContributionBeforeYield:   amount,
				ContributionPerOutputUnit: amountPerOutputUnit,
			}
		}
		result.PartialInputCostPerOutputUnit = inputCost
		result.PartialOperationCostPerOutputUnit = node.OperationCostPerUnit
		result.PartialTotalCostPerOutputUnit = inputCost + node.OperationCostPerUnit
		if valid && len(result.UnresolvedIssues) == 0 {
			result.InputCostPerOutputUnit = inputCost
			result.OperationCostPerOutputUnit = node.OperationCostPerUnit
			result.TotalCostPerOutputUnit = inputCost + node.OperationCostPerUnit
			result.Resolved = finiteNonNegative(result.TotalCostPerOutputUnit)
			if result.Resolved {
				result.CostStatus = "complete"
			}
		}
		resolved[key] = result
		state[key] = 2
		return result
	}

	for key := range nodes {
		resolve(key, nil)
	}
	return resolved
}

func productionBomCostOutputKey(outputType string, outputID int64) string {
	return fmt.Sprintf("%s:%d", normalizeProductionBomCostOutputType(outputType), outputID)
}

func normalizeProductionBomCostOutputType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "material":
		return "material"
	case "product_spec":
		return "product_spec"
	default:
		return "product"
	}
}

func (r Repository) loadResolvedProductionBomCosts(ctx context.Context) (map[int64]productionBomResolvedCost, error) {
	if r.pool == nil {
		return map[int64]productionBomResolvedCost{}, nil
	}
	hasBomName, err := r.costingColumnExists(ctx, "production_boms", "name")
	if err != nil {
		return nil, err
	}
	hasVersionNo, err := r.costingColumnExists(ctx, "production_bom_versions", "version_no")
	if err != nil {
		return nil, err
	}
	bomNameExpr := "''::text"
	bomNameGroup := ""
	if hasBomName {
		bomNameExpr = "COALESCE(NULLIF(bom.name,''),'制造 BOM')"
		bomNameGroup = ", bom.name"
	}
	versionNoExpr := "version.id::text"
	versionNoGroup := ""
	if hasVersionNo {
		versionNoExpr = "COALESCE(NULLIF(version.version_no,''),version.id::text)"
		versionNoGroup = ", version.version_no"
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT lower(COALESCE(NULLIF(binding.output_type,''),'product')) AS output_type,
		       binding.output_id,
		       CASE WHEN lower(COALESCE(NULLIF(binding.output_type,''),'product'))='product' THEN binding.output_id ELSE 0 END AS product_id,
		       bom.id AS bom_id,
		       %s AS bom_name,
		       version.id AS version_id,
		       %s AS version_no,
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
		GROUP BY binding.output_type, binding.output_id, bom.id, version.id, version.yield_rate, version.output_qty, version.output_unit%s%s
		ORDER BY output_type, binding.output_id
	`, r.schema, bomNameExpr, versionNoExpr, bomNameGroup, versionNoGroup))
	if err != nil {
		return nil, err
	}
	nodes := map[string]productionBomCostNode{}
	versionOutputKeys := map[int64]string{}
	versionIDs := make([]int64, 0)
	for rows.Next() {
		var node productionBomCostNode
		if err := rows.Scan(&node.OutputType, &node.OutputID, &node.ProductID, &node.BomID, &node.BomName, &node.VersionID, &node.VersionNo, &node.YieldRate, &node.OutputQty, &node.OutputUnit, &node.OperationCostPerUnit); err != nil {
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

	// A PR-600 product BOM version owns one independently costed output node per
	// published specification. Legacy product BOMs without variants retain the
	// historical product:<product_id> node, while manufactured materials retain
	// material:<material_id>.
	hasVariants, err := r.costingRelationExists(ctx, "production_bom_version_variants")
	if err != nil {
		return nil, err
	}
	hasSpecs, err := r.costingRelationExists(ctx, "production_bom_specs")
	if err != nil {
		return nil, err
	}
	hasItemVariant, err := r.costingColumnExists(ctx, "production_bom_version_items", "variant_id")
	if err != nil {
		return nil, err
	}
	hasComponentBomSpec, err := r.costingColumnExists(ctx, "production_bom_version_items", "component_bom_spec_id")
	if err != nil {
		return nil, err
	}
	variantAware := hasVariants && hasSpecs && hasItemVariant && hasComponentBomSpec
	variantOutputKeys := map[int64]string{}
	versionHasVariants := map[int64]bool{}
	versionDefaultSpecKeys := map[int64]string{}
	if variantAware {
		hasRouteOperations, err := r.costingRelationExists(ctx, "process_route_operations")
		if err != nil {
			return nil, err
		}
		routeCostExpr := "0::float8"
		if hasRouteOperations {
			routeCostExpr = fmt.Sprintf(`COALESCE((SELECT SUM(operation.planned_operation_cost) FROM %s.process_route_operations operation WHERE operation.route_id=variant.process_route_id),0)::float8`, r.schema)
		}
		variantRows, err := r.pool.Query(ctx, fmt.Sprintf(`
			SELECT variant.id,
			       variant.version_id,
			       variant.bom_spec_id,
			       COALESCE(NULLIF(variant.inventory_unit,''),'unit'),
			       COALESCE(variant.material_loss_rate,0)::float8,
			       COALESCE(variant.is_default,false),
			       %s AS operation_cost_per_unit
			FROM %s.production_bom_version_variants variant
			WHERE variant.version_id=ANY($1)
			ORDER BY variant.version_id,variant.sort_order,variant.id
		`, routeCostExpr, r.schema), versionIDs)
		if err != nil {
			return nil, err
		}
		for variantRows.Next() {
			var variantID, versionID, bomSpecID int64
			var inventoryUnit string
			var materialLossRate, operationCost float64
			var isDefaultSpec bool
			if err := variantRows.Scan(&variantID, &versionID, &bomSpecID, &inventoryUnit, &materialLossRate, &isDefaultSpec, &operationCost); err != nil {
				variantRows.Close()
				return nil, err
			}
			parentKey := versionOutputKeys[versionID]
			parent, ok := nodes[parentKey]
			if !ok || normalizeProductionBomCostOutputType(parent.OutputType) != "product" || bomSpecID <= 0 {
				continue
			}
			key := productionBomCostOutputKey("product_spec", bomSpecID)
			nodes[key] = productionBomCostNode{
				OutputType:           "product_spec",
				OutputID:             bomSpecID,
				ProductID:            parent.ProductID,
				VersionID:            versionID,
				VariantID:            variantID,
				BomSpecID:            bomSpecID,
				MaterialLossRate:     materialLossRate,
				YieldRate:            1,
				OutputQty:            1,
				OutputUnit:           inventoryUnit,
				OperationCostPerUnit: operationCost,
				BomID:                parent.BomID,
				BomName:              parent.BomName,
				VersionNo:            parent.VersionNo,
				OutputName:           parent.OutputName,
			}
			variantOutputKeys[variantID] = key
			versionHasVariants[versionID] = true
			if isDefaultSpec && bomSpecID > 0 {
				versionDefaultSpecKeys[versionID] = key
			}
		}
		if err := variantRows.Err(); err != nil {
			variantRows.Close()
			return nil, err
		}
		variantRows.Close()
		for versionID := range versionHasVariants {
			delete(nodes, versionOutputKeys[versionID])
		}
	}

	variantSelect := "0::bigint AS variant_id, 0::bigint AS component_bom_spec_id,"
	if variantAware {
		variantSelect = "COALESCE(i.variant_id,0) AS variant_id, COALESCE(i.component_bom_spec_id,0) AS component_bom_spec_id,"
	}
	itemRows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH material_valuation AS (
			SELECT l.material_id,
			       SUM((CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
			         THEN l.qty_g::numeric
			         ELSE l.qty_units::numeric
			       END) * COALESCE(b.unit_cost,0))
			       / NULLIF(SUM(CASE
			         WHEN lower(btrim(COALESCE(NULLIF(m.unit,''), 'kg'))) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司')
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
		       %s
		       i.id,
		       COALESCE(NULLIF(i.component_type,''),'material') AS component_type,
		       COALESCE(i.material_id,0),
		       COALESCE(m.is_semi_finished,false),
		       COALESCE(i.component_product_id,0),
		       COALESCE(NULLIF(m.name,''),'') AS component_material_name,
		       COALESCE(i.component_spec_g,0),
		       COALESCE(NULLIF(i.consume_unit,''),'ratio_pct') AS consume_unit,
		       COALESCE(i.qty_per_unit,0)::float8,
		       COALESCE(i.ratio_pct,0)::float8,
		       COALESCE(i.material_loss_rate,0)::float8,
		       CASE WHEN COALESCE(m.is_semi_finished,false) THEN 0 ELSE COALESCE(
		           NULLIF(mv.weighted_unit_cost,0),
		           NULLIF(m.purchase_price,0),
		           NULLIF(i.unit_cost_snapshot,0),
		           0
		       ) END::float8 AS unit_cost,
		       COALESCE(NULLIF(m.unit,''), 'kg') AS unit_cost_unit,
		       COALESCE(m.purchase_price,0)::float8 AS purchase_price,
		       COALESCE(mv.weighted_unit_cost,0)::float8 AS weighted_batch_unit_cost,
		       COALESCE(i.unit_cost_snapshot,0)::float8 AS unit_cost_snapshot
		FROM %[1]s.production_bom_version_items i
		LEFT JOIN %[1]s.materials m ON m.id=i.material_id
		LEFT JOIN material_valuation mv ON mv.material_id=i.material_id
		WHERE i.version_id=ANY($1)
		ORDER BY i.version_id, i.id
	`, r.schema, variantSelect), versionIDs)
	if err != nil {
		return nil, err
	}
	for itemRows.Next() {
		var versionID, variantID int64
		var item productionBomCostItem
		if err := itemRows.Scan(
			&versionID,
			&variantID,
			&item.ComponentBomSpecID,
			&item.ID,
			&item.ComponentType,
			&item.ComponentMaterialID,
			&item.ComponentIsSemi,
			&item.ComponentProductID,
			&item.ComponentMaterialName,
			&item.ComponentSpecG,
			&item.ConsumeUnit,
			&item.QtyPerUnit,
			&item.RatioPct,
			&item.MaterialLossRate,
			&item.UnitCost,
			&item.UnitCostUnit,
			&item.PurchasePrice,
			&item.WeightedBatchUnitCost,
			&item.UnitCostSnapshot,
		); err != nil {
			itemRows.Close()
			return nil, err
		}
		item.ComponentName = strings.TrimSpace(item.ComponentMaterialName)
		if item.ComponentName == "" {
			item.ComponentName = strings.TrimSpace(item.ComponentProductName)
		}
		outputKey := versionOutputKeys[versionID]
		if versionHasVariants[versionID] {
			outputKey = variantOutputKeys[variantID]
			if outputKey == "" {
				// A published specification version must not absorb unscoped or a
				// sibling variant's items into another specification cost.
				continue
			}
		}
		node := nodes[outputKey]
		if node.VariantID > 0 {
			item.MaterialLossRate = node.MaterialLossRate
		}
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
		switch normalizeProductionBomCostOutputType(node.OutputType) {
		case "product":
			if node.OutputID <= 0 {
				continue
			}
			productCosts[node.OutputID] = all[key]
		case "product_spec":
			if node.BomSpecID <= 0 {
				continue
			}
			productCosts[productionBomSpecCostMapKey(node.BomSpecID)] = all[key]
		}
	}
	// Variant-only versions drop their product:<id> node; expose the default
	// specification cost under the product key so legacy callers (pricing
	// trial, production cost) without a BomSpecID still resolve through the
	// default specification instead of falling back to per-item trial costs.
	for versionID, key := range versionDefaultSpecKeys {
		if !versionHasVariants[versionID] {
			continue
		}
		node, ok := nodes[key]
		if !ok || node.ProductID <= 0 {
			continue
		}
		if _, exists := productCosts[node.ProductID]; exists {
			continue
		}
		productCosts[node.ProductID] = all[key]
	}
	return productCosts, nil
}

func (r Repository) costingRelationExists(ctx context.Context, relation string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", r.schema, relation)).Scan(&exists)
	return exists, err
}

func (r Repository) costingColumnExists(ctx context.Context, relation string, column string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, r.schema, relation, column).Scan(&exists)
	return exists, err
}

func productionBomSpecCostMapKey(bomSpecID int64) int64 {
	if bomSpecID <= 0 {
		return 0
	}
	return -bomSpecID
}

func productionBomCostForProduct(costs map[int64]productionBomResolvedCost, productID int64, parentProductID int64, bomSpecID int64) (productionBomResolvedCost, bool) {
	if bomSpecID > 0 {
		cost, ok := costs[productionBomSpecCostMapKey(bomSpecID)]
		expectedParentID := parentProductID
		if expectedParentID <= 0 {
			expectedParentID = productID
		}
		if ok && cost.ProductID > 0 && expectedParentID > 0 && cost.ProductID != expectedParentID {
			return productionBomResolvedCost{}, false
		}
		return cost, ok
	}
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
		componentKey := item.ComponentProductID
		if item.ComponentBomSpecID > 0 {
			componentKey = productionBomSpecCostMapKey(item.ComponentBomSpecID)
		}
		componentCost, ok := costs[componentKey]
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
		if unitCost <= 0 {
			return productionBomResolvedItemCost{}, false, "BOM组件单价为 0：请维护物料采购价；半成品物料需绑定默认已发布的制造 BOM"
		}
		return productionBomResolvedItemCost{}, false, fmt.Sprintf("BOM组件成本单位无法换算：消耗单位 %s 与成本单位 %s 不匹配", strings.TrimSpace(item.ConsumeUnit), strings.TrimSpace(costUnit))
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
