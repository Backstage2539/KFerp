package production

import (
	"fmt"
	"math"
)

// CalculateRecursiveManufacturingStandardCost calculates cost from the
// default, frozen BOM graph supplied by the caller. Each BOM applies only its
// own component loss and operation cost, so an upstream loss is never counted
// again by a downstream BOM.
func CalculateRecursiveManufacturingStandardCost(output ManufacturingItemRef, qty float64, boms []ManufacturingBOM, leafUnitCosts map[string]float64) (float64, error) {
	if err := output.validate(); err != nil {
		return 0, err
	}
	if qty <= 0 {
		return 0, fmt.Errorf("positive standard-cost quantity required: %s", output.Key())
	}
	byOutput := make(map[string]ManufacturingBOM, len(boms))
	for _, bom := range boms {
		if err := bom.Output.validate(); err != nil {
			return 0, err
		}
		if bom.OutputQty <= 0 || bom.VersionID <= 0 {
			return 0, fmt.Errorf("published BOM version and positive output quantity required: %s", bom.Output.Key())
		}
		if _, exists := byOutput[bom.Output.Key()]; exists {
			return 0, fmt.Errorf("multiple default BOMs for %s", bom.Output.Key())
		}
		byOutput[bom.Output.Key()] = bom
	}
	return recursiveManufacturingStandardCost(output, qty, byOutput, leafUnitCosts, map[string]bool{})
}

func recursiveManufacturingStandardCost(output ManufacturingItemRef, qty float64, boms map[string]ManufacturingBOM, leafUnitCosts map[string]float64, path map[string]bool) (float64, error) {
	key := output.Key()
	bom, hasBOM := boms[key]
	if !hasBOM {
		unitCost, ok := leafUnitCosts[key]
		if !ok {
			return 0, fmt.Errorf("standard cost missing for leaf %s", key)
		}
		if unitCost < 0 {
			return 0, fmt.Errorf("standard cost cannot be negative for %s", key)
		}
		return normalizeManufacturingCost(qty * unitCost), nil
	}
	if path[key] {
		return 0, fmt.Errorf("manufacturing BOM cycle detected at %s", key)
	}
	path[key] = true
	defer delete(path, key)

	factor := qty / bom.OutputQty
	total := bom.OperationCost * factor
	for _, component := range bom.Components {
		required := manufacturingComponentGrossQty(component) * factor
		componentCost, err := recursiveManufacturingStandardCost(component.Item, required, boms, leafUnitCosts, path)
		if err != nil {
			return 0, err
		}
		total += componentCost
	}
	return normalizeManufacturingCost(total), nil
}

func normalizeManufacturingCost(value float64) float64 {
	return math.Round(value*100000000) / 100000000
}
