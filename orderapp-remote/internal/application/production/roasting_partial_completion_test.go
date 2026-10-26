package production

import (
	"math"
	"testing"
)

func TestRoastingPartialCompletionConsumesEachIncrementalInputOnce(t *testing.T) {
	for _, inputs := range [][2]float64{{1, 5}, {3, 3}} {
		materials := []OperationMaterialUse{{Name: "生豆", Unit: "kg", RatioPct: 100}}
		consumed := 0.0
		for _, input := range inputs {
			materials[0].ConsumedQty = consumed
			uses, err := ResolveRoastingMaterialUses(materials, input, "kg")
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(uses[0].Qty-input) > 1e-9 {
				t.Errorf("inputs %v: current input %g after consuming %g must consume %g, got %g", inputs, input, consumed, input, uses[0].Qty)
			}
			consumed += uses[0].Qty
		}
		if math.Abs(consumed-6) > 1e-9 {
			t.Errorf("inputs %v: total consumption = %g, want 6 kg", inputs, consumed)
		}
	}
}

func TestRoastingPartialCompletionKeepsWholeGramRounding(t *testing.T) {
	uses, err := ResolveRoastingMaterialUses([]OperationMaterialUse{
		{Name: "配料甲", Unit: "kg", RatioPct: 25, ConsumedQty: 1},
		{Name: "配料乙", Unit: "g", RatioPct: 75, ConsumedQty: 3000},
	}, 1.003, "kg")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(uses[0].Qty-.251) > 1e-9 || uses[1].Qty != 752 {
		t.Fatalf("current 1003 g should deduct 251 g + 752 g regardless of prior consumption: %+v", uses)
	}
}
