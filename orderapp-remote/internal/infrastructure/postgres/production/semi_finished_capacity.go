package production

import (
	"fmt"
	app "orderapp/internal/application/production"
)

// replanSemiFinishedCapacity preserves existing workstation choices and the
// relative distribution of their loads. An unknown unit or old target must be
// reviewed by a planner; silently guessing would produce an invalid schedule.
func replanSemiFinishedCapacity(before, after app.ProductionPlanItem, splits []app.ProductionPlanOperationSplit) ([]app.ProductionPlanOperationSplit, error) {
	result := make([]app.ProductionPlanOperationSplit, 0, len(splits))
	for _, split := range splits {
		oldTarget, newTarget := float64(0), float64(0)
		switch productionCapacityUnitKind(split.BatchSizeUnit) {
		case "weight":
			oldTarget, newTarget = float64(productionPlanItemTargetG(before)), float64(productionPlanItemTargetG(after))
		case "count":
			oldTarget, newTarget = before.SalesSpecCount, after.SalesSpecCount
			if oldTarget <= 0 && before.SpecG > 0 {
				oldTarget = float64(productionPlanItemTargetG(before)) / float64(before.SpecG)
			}
			if newTarget <= 0 && after.SpecG > 0 {
				newTarget = float64(productionPlanItemTargetG(after)) / float64(after.SpecG)
			}
		default:
			return nil, fmt.Errorf("工序 %s 产能单位无法换算，请手动重新安排", split.Operation)
		}
		if newTarget <= 0 {
			continue
		}
		if oldTarget <= 0 {
			return nil, fmt.Errorf("工序 %s 原目标数量不明，请手动重新安排", split.Operation)
		}
		if split.PlannedQty <= 0 && split.SchedulingMode != "quantity" && split.PlannedBatchCount > 0 && split.BatchSizeQty > 0 {
			split.PlannedQty = float64(split.PlannedBatchCount) * split.BatchSizeQty
		}
		split.ProductionPlanItemID = after.ID
		split.PlannedQty = roundProductionPlanQuantity(split.PlannedQty * newTarget / oldTarget)
		if split.PlannedQty <= 0 {
			continue
		}
		split.PlannedBatchCount, split.PlannedQtyG, split.PlannedMinutes = 0, 0, 0
		split.PlannedSeconds, split.PlannedOperationCost = nil, 0
		result = append(result, plannedCapacitySplitMetricsWithBasis(split, after.SpecG, after.SalesSpecCount, productionPlanItemOutputTargetG(after)))
	}
	return result, nil
}
