package production

import "fmt"

type SemiFinishedDemandInput struct {
	SpecCount      float64
	NetContentQty  float64
	NetContentUnit string
}

type SemiFinishedDemandResult struct {
	DemandQty  float64
	DemandUnit string
}

func CalculateSemiFinishedDemand(input SemiFinishedDemandInput) (SemiFinishedDemandResult, error) {
	if input.SpecCount <= 0 {
		return SemiFinishedDemandResult{}, fmt.Errorf("spec_count must be positive")
	}
	if input.NetContentQty <= 0 {
		return SemiFinishedDemandResult{}, fmt.Errorf("net_content_qty must be positive")
	}
	unit := normalizeWeightUnit(input.NetContentUnit)
	if unit == "" {
		return SemiFinishedDemandResult{}, fmt.Errorf("net_content_unit must be a weight unit (g or kg)")
	}
	demandQty := input.SpecCount * input.NetContentQty
	return SemiFinishedDemandResult{DemandQty: demandQty, DemandUnit: unit}, nil
}

func normalizeWeightUnit(unit string) string {
	switch unit {
	case "g", "kg":
		return unit
	default:
		return ""
	}
}

type TwoStagePlanDecision struct {
	NeedsSemiFinishedWO bool
	SemiFinishedGap     float64
	PackagingQty        float64
}

func DetermineTwoStagePlan(semiFinishedDemand float64, availableStock float64) TwoStagePlanDecision {
	if semiFinishedDemand <= availableStock {
		return TwoStagePlanDecision{
			NeedsSemiFinishedWO: false,
			SemiFinishedGap:     0,
			PackagingQty:        semiFinishedDemand,
		}
	}
	gap := semiFinishedDemand - availableStock
	return TwoStagePlanDecision{
		NeedsSemiFinishedWO: true,
		SemiFinishedGap:     gap,
		PackagingQty:        semiFinishedDemand,
	}
}
