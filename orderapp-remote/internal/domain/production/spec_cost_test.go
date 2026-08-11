package production

import (
	"math"
	"testing"
)

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func TestCalculateSpecStandardCost(t *testing.T) {
	result, err := CalculateSpecStandardCost(SpecStandardCostInput{
		SemiFinishedUnitCost:  80.0,
		NetContentQty:         227,
		NetContentUnit:        "g",
		PackagingMaterialCost: 1.5,
		PackagingProcessCost:  0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(result.SemiFinishedCost, 18.16) {
		t.Fatalf("semi-finished cost = %v, want 18.16 (80×0.227)", result.SemiFinishedCost)
	}
	if !floatEq(result.TotalCost, 20.16) {
		t.Fatalf("total cost = %v, want 20.16 (18.16+1.5+0.5)", result.TotalCost)
	}

	result2, err := CalculateSpecStandardCost(SpecStandardCostInput{
		SemiFinishedUnitCost:  80.0,
		NetContentQty:         0.454,
		NetContentUnit:        "kg",
		PackagingMaterialCost: 2.0,
		PackagingProcessCost:  1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(result2.SemiFinishedCost, 36.32) {
		t.Fatalf("semi-finished cost = %v, want 36.32 (80×0.454)", result2.SemiFinishedCost)
	}
	if !floatEq(result2.TotalCost, 39.32) {
		t.Fatalf("total cost = %v, want 39.32", result2.TotalCost)
	}

	result3, err := CalculateSpecStandardCost(SpecStandardCostInput{
		SemiFinishedUnitCost:  100.0,
		NetContentQty:         2.5,
		NetContentUnit:        "kg",
		PackagingMaterialCost: 5.0,
		PackagingProcessCost:  2.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(result3.SemiFinishedCost, 250.0) {
		t.Fatalf("semi-finished cost = %v, want 250.0 (100×2.5)", result3.SemiFinishedCost)
	}
	if !floatEq(result3.TotalCost, 257.0) {
		t.Fatalf("total cost = %v, want 257.0", result3.TotalCost)
	}
}

func TestCalculateSpecStandardCostNoLossInPackaging(t *testing.T) {
	result, err := CalculateSpecStandardCost(SpecStandardCostInput{
		SemiFinishedUnitCost:  80.0,
		NetContentQty:         227,
		NetContentUnit:        "g",
		PackagingMaterialCost: 1.5,
		PackagingProcessCost:  0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(result.PackagingMaterialCost, 1.5) {
		t.Fatalf("packaging material cost should not include loss, got %v", result.PackagingMaterialCost)
	}
}

func TestCalculateSpecStandardCostErrors(t *testing.T) {
	if _, err := CalculateSpecStandardCost(SpecStandardCostInput{NetContentQty: 0, NetContentUnit: "g"}); err == nil {
		t.Fatal("should error on zero net_content_qty")
	}
	if _, err := CalculateSpecStandardCost(SpecStandardCostInput{NetContentQty: 227, NetContentUnit: "袋"}); err == nil {
		t.Fatal("should error on non-weight unit")
	}
}
