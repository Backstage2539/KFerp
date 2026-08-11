package production

import (
	"math"
	"testing"
)

func TestCalculateRecursiveManufacturingStandardCostAppliesEachBOMLossOnce(t *testing.T) {
	product := ManufacturingItemRef{Type: "product", ID: 227, Name: "227g咖啡", Unit: "unit"}
	roasted := ManufacturingItemRef{Type: "material", ID: 10, Name: "熟豆", Unit: "g"}
	bag := ManufacturingItemRef{Type: "material", ID: 20, Name: "咖啡袋", Unit: "unit"}
	green := ManufacturingItemRef{Type: "material", ID: 30, Name: "生豆", Unit: "g"}
	boms := []ManufacturingBOM{
		{
			VersionID:     1001,
			Output:        product,
			OutputQty:     1,
			OperationCost: 1,
			Components: []ManufacturingBOMComponent{
				{Item: roasted, Qty: 227},
				// Fixed package-unit components never inherit the product BOM loss.
				{Item: bag, Qty: 1, Fixed: true, LossRate: 0.20},
			},
		},
		{
			VersionID:     1002,
			Output:        roasted,
			OutputQty:     1000,
			OperationCost: 5,
			Components: []ManufacturingBOMComponent{
				// 1,000g net green input with 20% gross-input loss requires 1,250g.
				{Item: green, Qty: 1000, LossRate: 0.20},
			},
		},
	}
	cost, err := CalculateRecursiveManufacturingStandardCost(product, 1, boms, map[string]float64{
		bag.Key():   0.50,
		green.Key(): 0.078,
	})
	if err != nil {
		t.Fatalf("CalculateRecursiveManufacturingStandardCost: %v", err)
	}
	// Roasted batch: 1,250g * 0.078 + 5 = 102.5 per 1,000g.
	// Product: 227g * 0.1025 + one 0.50 bag + 1 operation = 24.7675.
	if math.Abs(cost-24.7675) > 0.000001 {
		t.Fatalf("cost=%v, want 24.7675", cost)
	}
}

func TestCalculateRecursiveManufacturingStandardCostRejectsMissingLeafCost(t *testing.T) {
	product := ManufacturingItemRef{Type: "product", ID: 1, Name: "SKU", Unit: "unit"}
	material := ManufacturingItemRef{Type: "material", ID: 2, Name: "原料", Unit: "g"}
	_, err := CalculateRecursiveManufacturingStandardCost(product, 1, []ManufacturingBOM{{
		VersionID:  1,
		Output:     product,
		OutputQty:  1,
		Components: []ManufacturingBOMComponent{{Item: material, Qty: 1}},
	}}, nil)
	if err == nil {
		t.Fatal("expected missing leaf standard cost error")
	}
}
