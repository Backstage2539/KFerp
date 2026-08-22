package costing

import (
	"context"
	"fmt"

	appcosting "orderapp/internal/application/costing"
	domain "orderapp/internal/domain/costing"
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
	result.MaterialUnitCost = result.PartialCost
	result.BomCostTotal = result.PartialCost
	result.StandardManufacturingUnitCost = result.PartialCost
	result.BaseCostDetails = []appcosting.PricingRuleTrialBaseCostDetail{{
		Key: "material:source", Type: "material", TypeLabel: "物料", Name: result.MaterialName,
		ComponentID: result.MaterialID, Quantity: 1, ConsumeUnit: result.CostUnit,
		UnitCost: result.PartialCost, CostUnitCost: result.PartialCost, CostUnit: result.CostUnit,
		CostSource: result.CostSource, Amount: result.PartialCost, Unit: result.CostUnit,
		Description: materialDirectCostDescription(result.CostSource, weighted, purchase),
	}}
	result.FormulaExpression, result.FormulaExpressionLines, result.Steps = materialCostTrialFormula(result)
	return result, nil
}

func (r Repository) loadManufacturedMaterialTrial(ctx context.Context, cmd appcosting.MaterialCostTrialCommand, result appcosting.MaterialCostTrialResult) (appcosting.MaterialCostTrialResult, error) {
	all, err := r.loadResolvedProductionBomCostsTyped(ctx)
	if cmd.BomVersionID > 0 {
		all, err = r.loadResolvedProductionBomCostsTypedForMaterial(ctx, cmd.MaterialID, cmd.BomVersionID)
	}
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
	result.BomSnapshot = appcosting.PricingRuleTrialBomSnapshot{BomID: cost.BomID, BomName: cost.BomName, VersionID: cost.VersionID, VersionNo: cost.VersionNo, BomSpecID: cost.BomSpecID, BomVariantID: cost.BomVariantID, Status: cost.BomStatus, UsageMode: "manufacturing"}
	result.BomVersionID = cost.VersionID
	result.BomVersionNo = cost.VersionNo
	result.BomStatus = cost.BomStatus
	result.BomUsageMode = "manufacturing"
	result.InputCost = cost.PartialInputCostPerOutputUnit
	result.OperationCost = cost.PartialOperationCostPerOutputUnit
	result.BomCostTotal = cost.PartialInputCostPerOutputUnit
	result.OperationCostTotal = cost.PartialOperationCostPerOutputUnit
	result.MaterialUnitCost = cost.PartialInputCostPerOutputUnit
	result.OperationUnitCost = cost.PartialOperationCostPerOutputUnit
	result.PartialCost = cost.PartialTotalCostPerOutputUnit
	result.CostStatus = cost.CostStatus
	result.CostSource = "manufacturing_bom"
	if cost.BomStatus != "published" {
		result.CostSource = "manufacturing_bom_draft_diagnostic"
	}
	result.UnresolvedComponents = append([]appcosting.PricingRuleTrialCostIssue(nil), cost.UnresolvedIssues...)
	result.BaseCostDetails = append([]appcosting.PricingRuleTrialBaseCostDetail(nil), cost.BaseCostDetails...)
	operationDetails, operationErr := r.loadMaterialCostTrialOperationDetails(ctx, cost.VersionID, cost.BomVariantID, cost.OutputUnit)
	if operationErr != nil {
		return result, operationErr
	}
	result.BaseCostDetails = append(result.BaseCostDetails, operationDetails...)
	if cost.Resolved {
		result.UnitCost = cost.TotalCostPerOutputUnit
		result.StandardManufacturingUnitCost = cost.TotalCostPerOutputUnit
	} else {
		result.StandardManufacturingUnitCost = cost.PartialTotalCostPerOutputUnit
	}
	// Build the workstation snapshot after the aggregate costs are assigned so
	// its summary mirrors the waterfall shown by both product and material
	// trials.  Building it before this point would leave the standard cost at 0
	// even though the detail rows were resolved correctly.
	result.WorkstationCostSnapshot = materialWorkstationSnapshot(result, operationDetails)
	result.FormulaExpression, result.FormulaExpressionLines, result.Steps = materialCostTrialFormula(result)
	return result, nil
}

func materialDirectCostDescription(source string, weighted, purchase float64) string {
	switch source {
	case "weighted_batch_cost":
		return fmt.Sprintf("有效批次加权成本 %.4f", weighted)
	case "purchase_price":
		return fmt.Sprintf("采购价 %.4f", purchase)
	case "missing_purchase_or_batch_cost":
		return "有效批次加权成本和采购价均为 0"
	default:
		return "物料直接成本来源"
	}
}

func materialCostTrialFormula(result appcosting.MaterialCostTrialResult) (string, []string, []domain.PriceExplanationStep) {
	unit := result.CostUnit
	if unit == "" {
		unit = "kg"
	}
	materialCost := result.MaterialUnitCost
	operationCost := result.OperationUnitCost
	standardCost := result.StandardManufacturingUnitCost
	expression := fmt.Sprintf("标准制造成本 = 物料成本 %.4f/%s + 标准工序成本 %.4f/%s = %.4f/%s", materialCost, unit, operationCost, unit, standardCost, unit)
	lines := []string{
		fmt.Sprintf("物料成本 = Σ(组件用量 × 成本单价) = %.4f/%s", materialCost, unit),
		fmt.Sprintf("标准工序成本 = BOM 工序成本快照合计 = %.4f/%s", operationCost, unit),
		expression,
	}
	steps := []domain.PriceExplanationStep{
		{Key: "material_unit_cost", Label: "物料成本", Source: result.CostSource, Value: materialCost, Unit: unit},
		{Key: "operation_unit_cost", Label: "标准工序成本", Source: "bom_operation_snapshot", Value: operationCost, Unit: unit},
		{Key: "standard_manufacturing_unit_cost", Label: "标准制造成本", Source: "formula", Value: standardCost, Unit: unit},
	}
	return expression, lines, steps
}

func materialWorkstationSnapshot(result appcosting.MaterialCostTrialResult, operationDetails []appcosting.PricingRuleTrialBaseCostDetail) appcosting.PricingRuleTrialWorkstationCostSnapshot {
	snapshot := appcosting.PricingRuleTrialWorkstationCostSnapshot{
		MaterialUnitCost:              result.MaterialUnitCost,
		OperationUnitCost:             result.OperationUnitCost,
		StandardManufacturingUnitCost: result.StandardManufacturingUnitCost,
		OperationRows:                 make([]appcosting.PricingRuleTrialWorkstationCostSnapshotRow, 0, len(operationDetails)),
	}
	for _, row := range operationDetails {
		snapshot.OperationRows = append(snapshot.OperationRows, appcosting.PricingRuleTrialWorkstationCostSnapshotRow{
			OperationName: row.Name, WorkstationName: row.WorkstationName, CapacityName: row.CapacityName,
			CostMethod: row.CostMethod, PieceRate: row.PieceRate, RateUnit: row.RateUnit,
			HourlyRate: row.HourlyRate, StandardMinutes: row.StandardMinutes,
			StandardOutputQty: row.StandardOutputQty, StandardOutputUnit: row.StandardOutputUnit,
			UnitCost: row.UnitCost, Unit: row.Unit,
		})
	}
	return snapshot
}

func (r Repository) loadMaterialCostTrialOperationDetails(ctx context.Context, versionID, variantID int64, outputUnit string) ([]appcosting.PricingRuleTrialBaseCostDetail, error) {
	if versionID <= 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,
		       COALESCE(NULLIF(operation_name,''),'工序'),
		       COALESCE(NULLIF(workstation_name,''),''),
		       COALESCE(NULLIF(capacity_name,''),''),
		       COALESCE(hourly_rate_snapshot,0)::float8,
		       COALESCE(standard_minutes_snapshot,0)::float8,
		       COALESCE(batch_size_qty_snapshot,0)::float8,
		       COALESCE(batch_size_unit_snapshot,''),
		       COALESCE(NULLIF(cost_method,''),'time'),
		       COALESCE(piece_rate_snapshot,0)::float8,
		       COALESCE(rate_unit_snapshot,''),
		       COALESCE(operation_unit_cost,0)::float8,
		       COALESCE(NULLIF(operation_cost_unit,''),$2)
		FROM %s.production_bom_version_operation_costs
		WHERE version_id=$1
		ORDER BY sort_order,id
	`, r.schema), versionID, outputUnit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]appcosting.PricingRuleTrialBaseCostDetail, 0)
	for rows.Next() {
		var row appcosting.PricingRuleTrialBaseCostDetail
		var id int64
		if err := rows.Scan(&id, &row.Name, &row.WorkstationName, &row.CapacityName, &row.HourlyRate, &row.StandardMinutes, &row.StandardOutputQty, &row.StandardOutputUnit, &row.CostMethod, &row.PieceRate, &row.RateUnit, &row.UnitCost, &row.Unit); err != nil {
			return nil, err
		}
		row.Key = fmt.Sprintf("operation:bom_snapshot:%d", id)
		row.Type = "operation"
		row.TypeLabel = "标准工序"
		row.ConsumeUnit = "per_inventory_unit"
		row.Quantity = 1
		row.CostUnit = row.Unit
		row.CostUnitCost = row.UnitCost
		row.Amount = row.UnitCost
		row.CostSource = "bom_operation_snapshot"
		row.CapacitySelectionSource = "bom_operation_snapshot"
		row.Description = fmt.Sprintf("标准工序成本来自 BOM 工序成本快照：%s · %s · %.4f/%s", row.WorkstationName, row.CapacityName, row.UnitCost, row.Unit)
		result = append(result, row)
	}
	return result, rows.Err()
}
