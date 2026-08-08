package production

import (
	"fmt"
	"math"

	inventorydomain "orderapp/internal/domain/inventory"
)

type Quantity = inventorydomain.Quantity

type JobCardActual struct {
	ActualInputQty  float64
	ActualOutputQty float64
	ActualLossQty   float64
	ActualLossRate  float64
}

func NormalizeYieldRate(rate float64) float64 {
	if rate <= 0 || rate > 1 {
		return 0.8
	}
	return rate
}

func DefaultInputGrams(needG int64, yieldRate float64) int64 {
	if needG <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(needG) / NormalizeYieldRate(yieldRate)))
}

func PlannedInputGramsFromMaterialLoss(needG int64, lossRate float64) int64 {
	if needG <= 0 {
		return 0
	}
	if math.IsNaN(lossRate) || math.IsInf(lossRate, 0) || lossRate <= 0 || lossRate >= 1 {
		return needG
	}
	return int64(math.Ceil(float64(needG) * (1 + lossRate)))
}

func SalesSpecCountToInventoryQuantity(salesSpecCount, inventoryQtyPerSalesUnit float64) float64 {
	if salesSpecCount <= 0 || inventoryQtyPerSalesUnit <= 0 {
		return 0
	}
	return math.Round(salesSpecCount*inventoryQtyPerSalesUnit*1_000_000_000) / 1_000_000_000
}

func InventoryQuantityToLegacyGrams(quantity float64, unit string) int64 {
	if quantity <= 0 {
		return 0
	}
	switch normalizeProductionUnit(unit) {
	case "g":
		return int64(math.Round(quantity))
	case "kg":
		return int64(math.Round(quantity * 1000))
	case "lb":
		return int64(math.Round(quantity * 453.59237))
	default:
		return 0
	}
}

func normalizeProductionUnit(unit string) string {
	switch unit {
	case "G", "g", "克":
		return "g"
	case "KG", "Kg", "kg", "千克", "公斤":
		return "kg"
	case "LB", "Lb", "lb", "磅":
		return "lb"
	default:
		return unit
	}
}

func FinishedTotalGrams(specG, units, looseG int64) int64 {
	if specG <= 0 || units < 0 || looseG < 0 {
		return 0
	}
	return units*specG + looseG
}

func ActualYieldRate(specG, units, looseG, inputG int64) (float64, error) {
	if inputG <= 0 {
		return 0, fmt.Errorf("input_g must be greater than 0")
	}
	total := FinishedTotalGrams(specG, units, looseG)
	rate := float64(total) / float64(inputG)
	return math.Round(rate*10000) / 10000, nil
}

func ExpectedLossRate(yieldRate float64) float64 {
	rate := NormalizeYieldRate(yieldRate)
	return math.Round((1-rate)*10000) / 10000
}

func YieldRateFromExpectedLossRate(lossRate float64) (float64, error) {
	if lossRate < 0 || lossRate >= 1 {
		return 0, fmt.Errorf("expected_loss_rate must be >= 0 and < 1")
	}
	return math.Round((1-lossRate)*10000) / 10000, nil
}

func ActualLossMetrics(inputQty, outputQty float64) (float64, float64, error) {
	if inputQty <= 0 {
		return 0, 0, fmt.Errorf("actual_input_qty must be greater than 0")
	}
	if outputQty < 0 {
		return 0, 0, fmt.Errorf("actual_output_qty cannot be negative")
	}
	lossQty := inputQty - outputQty
	if lossQty < 0 {
		lossQty = 0
	}
	lossQty = math.Round(lossQty*1000) / 1000
	lossRate := math.Round((lossQty/inputQty)*10000) / 10000
	return lossQty, lossRate, nil
}

func AggregateJobCardActuals(rows []JobCardActual) JobCardActual {
	out := JobCardActual{}
	for _, row := range rows {
		out.ActualInputQty += row.ActualInputQty
		out.ActualOutputQty += row.ActualOutputQty
		out.ActualLossQty += row.ActualLossQty
	}
	out.ActualInputQty = math.Round(out.ActualInputQty*1000) / 1000
	out.ActualOutputQty = math.Round(out.ActualOutputQty*1000) / 1000
	out.ActualLossQty = math.Round(out.ActualLossQty*1000) / 1000
	if out.ActualInputQty > 0 {
		out.ActualLossRate = math.Round((out.ActualLossQty/out.ActualInputQty)*10000) / 10000
	}
	return out
}

func PlannedFinishedInventoryAddition(specG, needG int64) Quantity {
	if specG <= 0 || needG <= 0 {
		return Quantity{}
	}
	return Quantity{Units: needG / specG, LooseG: needG % specG}
}

func PlannedFinishedInventoryByInput(specG, inputG int64, yieldRate float64) Quantity {
	if specG <= 0 || inputG <= 0 {
		return Quantity{}
	}
	totalG := int64(math.Floor(float64(inputG)*NormalizeYieldRate(yieldRate) + 1e-9))
	return PlannedFinishedInventoryAddition(specG, totalG)
}

func RunningInventoryPlan(specG, needG, inputG int64, yieldRate float64) Quantity {
	plan := PlannedFinishedInventoryByInput(specG, inputG, yieldRate)
	if plan.Units > 0 || plan.LooseG > 0 {
		return plan
	}
	return PlannedFinishedInventoryAddition(specG, needG)
}

func NormalizeFinishedInventoryAddition(specG, units, looseG int64) (Quantity, error) {
	if units < 0 || looseG < 0 {
		return Quantity{}, fmt.Errorf("完成件数和散装余量不能为负数")
	}
	return inventorydomain.Normalize(specG, Quantity{Units: units, LooseG: looseG})
}

func RestoreAllocatedInventory(specG int64, current Quantity, deductedG int64) (Quantity, error) {
	if deductedG < 0 {
		return Quantity{}, fmt.Errorf("restored grams cannot be negative")
	}
	return inventorydomain.Normalize(specG, Quantity{Units: current.Units, LooseG: current.LooseG + deductedG})
}
