package production

import (
	"fmt"
	"math"

	inventorydomain "orderapp/internal/domain/inventory"
)

type Quantity = inventorydomain.Quantity

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

