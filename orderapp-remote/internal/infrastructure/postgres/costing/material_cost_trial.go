package costing

import (
	"context"
	"fmt"

	appcosting "orderapp/internal/application/costing"
)

func (r Repository) LoadMaterialCostTrialOptions(ctx context.Context, materialID int64) (appcosting.MaterialCostTrialOptions, error) {
	var mode string
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT CASE WHEN COALESCE(is_semi_finished,false) THEN 'manufacture' ELSE 'purchase' END FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, r.schema), materialID).Scan(&mode); err != nil {
		return appcosting.MaterialCostTrialOptions{}, err
	}
	result := appcosting.MaterialCostTrialOptions{MaterialID: materialID, SupplyMode: mode, BomVersions: []appcosting.MaterialCostTrialOption{}}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT pb.id, COALESCE(NULLIF(pb.name,''),'制造 BOM'), v.id,
		       COALESCE(NULLIF(v.version_no,''),v.id::text), v.status,
		       COALESCE(ob.is_default,false)
		FROM %s.production_boms pb
		JOIN %s.production_bom_versions v ON v.bom_id=pb.id
		LEFT JOIN %s.production_bom_output_bindings ob ON ob.bom_id=pb.id AND ob.bom_version_id=v.id AND ob.output_type='material' AND ob.output_id=$1
		WHERE pb.output_type='material' AND pb.output_material_id=$1
		  AND COALESCE(NULLIF(pb.status,''),'active')='active'
		ORDER BY CASE WHEN v.status='published' THEN 0 ELSE 1 END, v.id DESC`, r.schema, r.schema, r.schema), materialID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var row appcosting.MaterialCostTrialOption
		if err := rows.Scan(&row.BomID, &row.BomName, &row.VersionID, &row.VersionNo, &row.Status, &row.IsDefault); err != nil {
			return result, err
		}
		result.BomVersions = append(result.BomVersions, row)
	}
	return result, rows.Err()
}

func (r Repository) LoadMaterialCostTrial(ctx context.Context, cmd appcosting.MaterialCostTrialCommand) (appcosting.MaterialCostTrialResult, error) {
	var result appcosting.MaterialCostTrialResult
	var isSemi bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,code,name,COALESCE(is_semi_finished,false),COALESCE(unit,'kg') FROM %s.materials WHERE id=$1 AND deprecated_at IS NULL`, r.schema), cmd.MaterialID).
		Scan(&result.MaterialID, &result.MaterialCode, &result.MaterialName, &isSemi, &result.CostUnit); err != nil {
		return result, err
	}
	if isSemi {
		result.SupplyMode = "manufacture"
		return r.loadManufacturedMaterialTrial(ctx, cmd, result)
	}
	result.SupplyMode = "purchase"
	var weighted, purchase float64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		WITH valuation AS (
			SELECT SUM((CASE WHEN lower(COALESCE(NULLIF(m.unit,''),'kg')) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司') THEN l.qty_g::numeric ELSE l.qty_units::numeric END) * COALESCE(b.unit_cost,0)) /
			NULLIF(SUM(CASE WHEN lower(COALESCE(NULLIF(m.unit,''),'kg')) IN ('g','kg','lb','lbs','oz','克','千克','公斤','磅','盎司') THEN l.qty_g::numeric ELSE l.qty_units::numeric END),0) AS weighted
			FROM %s.material_batch_locations l JOIN %s.material_batches b ON b.id=l.material_batch_id JOIN %s.materials m ON m.id=l.material_id
			WHERE l.material_id=$1 AND (l.qty_g>0 OR l.qty_units>0) AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		)
		SELECT COALESCE((SELECT weighted FROM valuation),0), COALESCE((SELECT purchase_price FROM %s.materials WHERE id=$1),0)`, r.schema, r.schema, r.schema, r.schema), cmd.MaterialID).Scan(&weighted, &purchase); err != nil {
		return result, err
	}
	if weighted > 0 {
		result.UnitCost, result.PartialCost, result.CostStatus, result.CostSource = weighted, weighted, "complete", "weighted_batch_cost"
	} else if purchase > 0 {
		result.UnitCost, result.PartialCost, result.CostStatus, result.CostSource = purchase, purchase, "complete", "purchase_price"
	} else {
		result.CostStatus, result.CostSource = "incomplete", "missing_purchase_or_batch_cost"
		result.UnresolvedComponents = []appcosting.PricingRuleTrialCostIssue{{Code: "zero_material_cost", Reason: "外购物料采购价和有效批次单位成本均为 0，请维护采购价或批次成本", ComponentType: "material", ComponentID: cmd.MaterialID, ComponentMaterialID: cmd.MaterialID, ComponentName: result.MaterialName, CostUnit: result.CostUnit, UnitCost: 0, PurchasePrice: purchase, WeightedBatchUnitCost: weighted}}
	}
	return result, nil
}

func (r Repository) loadManufacturedMaterialTrial(ctx context.Context, cmd appcosting.MaterialCostTrialCommand, result appcosting.MaterialCostTrialResult) (appcosting.MaterialCostTrialResult, error) {
	all, err := r.loadResolvedProductionBomCostsTyped(ctx)
	if err != nil {
		return result, err
	}
	key := productionBomCostOutputKey("material", cmd.MaterialID)
	cost, ok := all[key]
	if !ok {
		result.CostStatus = "incomplete"
		result.CostSource = "missing_default_published_manufacturing_bom"
		result.UnresolvedComponents = []appcosting.PricingRuleTrialCostIssue{{Code: "bom_not_found", Reason: "自制物料没有默认已发布制造 BOM", ComponentType: "material", ComponentID: cmd.MaterialID, ComponentMaterialID: cmd.MaterialID, ComponentName: result.MaterialName, RootOutputType: "material", RootOutputID: cmd.MaterialID}}
		return result, nil
	}
	result.BomSnapshot = appcosting.PricingRuleTrialBomSnapshot{BomID: cost.BomID, BomName: cost.BomName, VersionID: cost.VersionID, VersionNo: cost.VersionNo, BomSpecID: cost.BomSpecID, BomVariantID: cost.BomVariantID, Status: "published", UsageMode: "manufacturing"}
	result.InputCost = cost.PartialInputCostPerOutputUnit
	result.OperationCost = cost.PartialOperationCostPerOutputUnit
	result.PartialCost = cost.PartialTotalCostPerOutputUnit
	result.CostStatus = cost.CostStatus
	result.CostSource = "manufacturing_bom"
	result.UnresolvedComponents = append([]appcosting.PricingRuleTrialCostIssue(nil), cost.UnresolvedIssues...)
	if cost.Resolved {
		result.UnitCost = cost.TotalCostPerOutputUnit
	}
	// A caller may request a version for diagnostics. The published graph is
	// still the only formal source; expose the selected identity in the
	// snapshot so the UI never silently presents another version.
	if cmd.BomVersionID > 0 && cost.VersionID != cmd.BomVersionID {
		result.CostStatus = "incomplete"
		result.CostSource = "requested_bom_version_not_default_published"
		result.UnresolvedComponents = append(result.UnresolvedComponents, appcosting.PricingRuleTrialCostIssue{Code: "bom_version_not_selected", Reason: fmt.Sprintf("请求的 BOM 版本 %d 不是默认已发布制造 BOM", cmd.BomVersionID), ComponentType: "material", ComponentID: cmd.MaterialID, ComponentMaterialID: cmd.MaterialID, ComponentName: result.MaterialName, BomID: cmd.BomVersionID})
	}
	return result, nil
}
