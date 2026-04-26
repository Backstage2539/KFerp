package production

import (
	inventorydomain "orderapp/internal/domain/inventory"
	productiondomain "orderapp/internal/domain/production"
)

type InvQty = inventorydomain.Quantity

func invTotalG(specG int64, q InvQty) (int64, error) {
	return inventorydomain.TotalGrams(specG, q)
}

func invNormalize(specG int64, q InvQty) (InvQty, error) {
	return inventorydomain.Normalize(specG, q)
}

func invDeduct(specG int64, q InvQty, needG int64) (remain InvQty, deductedG int64, gapG int64, err error) {
	return inventorydomain.Deduct(specG, q, needG)
}

func normalizeYieldRate(rate float64) float64 {
	return productiondomain.NormalizeYieldRate(rate)
}

func defaultProductionInputG(needG int64, yieldRate float64) int64 {
	return productiondomain.DefaultInputGrams(needG, yieldRate)
}

func finishedTotalG(specG, units, looseG int64) int64 {
	return productiondomain.FinishedTotalGrams(specG, units, looseG)
}

func actualYieldRate(specG, units, looseG, inputG int64) (float64, error) {
	return productiondomain.ActualYieldRate(specG, units, looseG, inputG)
}

func plannedFinishedInventoryAddition(specG, needG int64) InvQty {
	return productiondomain.PlannedFinishedInventoryAddition(specG, needG)
}

func plannedFinishedInventoryByInput(specG, inputG int64, yieldRate float64) InvQty {
	return productiondomain.PlannedFinishedInventoryByInput(specG, inputG, yieldRate)
}

func runningInventoryPlan(specG, needG, inputG int64, yieldRate float64) InvQty {
	return productiondomain.RunningInventoryPlan(specG, needG, inputG, yieldRate)
}

func normalizeFinishedInventoryAddition(specG, units, looseG int64) (InvQty, error) {
	return productiondomain.NormalizeFinishedInventoryAddition(specG, units, looseG)
}

func restoreAllocatedInventory(specG int64, current InvQty, deductedG int64) (InvQty, error) {
	return productiondomain.RestoreAllocatedInventory(specG, current, deductedG)
}
