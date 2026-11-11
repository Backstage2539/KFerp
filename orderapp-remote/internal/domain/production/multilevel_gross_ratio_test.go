package production

import (
	"math"
	"testing"
)

func TestManufacturingGrossRatioUsesStableRecipeOrderAcrossMixedUnits(t *testing.T) {
	bom := ManufacturingBOM{GrossRatioGrams: true, OutputGramFactor: 1000, OutputBasisGrams: 1000, GrossLossFactor: 1 / .805, Components: []ManufacturingBOMComponent{
		{Item: ManufacturingItemRef{Type: "material", ID: 1, Unit: "kg"}, Qty: .15, GramFactor: 1000, RatioWeight: 15},
		{Item: ManufacturingItemRef{Type: "material", ID: 2, Unit: "g"}, Qty: 150, GramFactor: 1, RatioWeight: 15},
		{Item: ManufacturingItemRef{Type: "material", ID: 3, Unit: "kg"}, Qty: .2, GramFactor: 1000, RatioWeight: 20},
		{Item: ManufacturingItemRef{Type: "material", ID: 4, Unit: "kg"}, Qty: .5, GramFactor: 1000, RatioWeight: 50},
	}}
	got, err := manufacturingGrossRatioAllocations(bom, 10.006)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{1.865, 1864, 2.486, 6.215} {
		if math.Abs(got[i]-want) > 1e-9 {
			t.Fatalf("mixed unit allocation: %v", got)
		}
	}
}
