package production

import "fmt"

type SpecStandardCostInput struct {
	SemiFinishedUnitCost  float64
	NetContentQty         float64
	NetContentUnit        string
	PackagingMaterialCost float64
	PackagingProcessCost  float64
}

type SpecStandardCostResult struct {
	TotalCost            float64
	SemiFinishedCost     float64
	PackagingMaterialCost float64
	PackagingProcessCost  float64
}

func CalculateSpecStandardCost(input SpecStandardCostInput) (SpecStandardCostResult, error) {
	if input.NetContentQty <= 0 {
		return SpecStandardCostResult{}, fmt.Errorf("net_content_qty must be positive")
	}
	netContentKg := toKg(input.NetContentQty, input.NetContentUnit)
	if netContentKg <= 0 {
		return SpecStandardCostResult{}, fmt.Errorf("net_content_unit must be a weight unit (g or kg)")
	}
	semiFinishedCost := input.SemiFinishedUnitCost * netContentKg
	total := semiFinishedCost + input.PackagingMaterialCost + input.PackagingProcessCost
	return SpecStandardCostResult{
		TotalCost:            total,
		SemiFinishedCost:     semiFinishedCost,
		PackagingMaterialCost: input.PackagingMaterialCost,
		PackagingProcessCost:  input.PackagingProcessCost,
	}, nil
}

func toKg(qty float64, unit string) float64 {
	switch unit {
	case "kg":
		return qty
	case "g":
		return qty / 1000
	default:
		return 0
	}
}
