package production

import "testing"

func TestCalculateSemiFinishedDemand(t *testing.T) {
	result, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{
		SpecCount: 100, NetContentQty: 227, NetContentUnit: "g",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DemandQty != 22700 || result.DemandUnit != "g" {
		t.Fatalf("100×227g = %v %s, want 22700 g", result.DemandQty, result.DemandUnit)
	}

	result2, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{
		SpecCount: 100, NetContentQty: 0.227, NetContentUnit: "kg",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.DemandQty != 22.7 || result2.DemandUnit != "kg" {
		t.Fatalf("100×0.227kg = %v %s, want 22.7 kg", result2.DemandQty, result2.DemandUnit)
	}

	result3, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{
		SpecCount: 50, NetContentQty: 454, NetContentUnit: "g",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result3.DemandQty != 22700 || result3.DemandUnit != "g" {
		t.Fatalf("50×454g = %v %s, want 22700 g", result3.DemandQty, result3.DemandUnit)
	}
}

func TestCalculateSemiFinishedDemandErrors(t *testing.T) {
	if _, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{SpecCount: 0, NetContentQty: 227, NetContentUnit: "g"}); err == nil {
		t.Fatal("should error on zero spec_count")
	}
	if _, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{SpecCount: 100, NetContentQty: 0, NetContentUnit: "g"}); err == nil {
		t.Fatal("should error on zero net_content_qty")
	}
	if _, err := CalculateSemiFinishedDemand(SemiFinishedDemandInput{SpecCount: 100, NetContentQty: 227, NetContentUnit: "袋"}); err == nil {
		t.Fatal("should error on non-weight unit")
	}
}

func TestDetermineTwoStagePlan(t *testing.T) {
	plan := DetermineTwoStagePlan(22.7, 30)
	if plan.NeedsSemiFinishedWO {
		t.Fatalf("sufficient stock should not need semi-finished WO")
	}
	if plan.PackagingQty != 22.7 {
		t.Fatalf("packaging qty = %v, want 22.7", plan.PackagingQty)
	}

	plan2 := DetermineTwoStagePlan(22.7, 10)
	if !plan2.NeedsSemiFinishedWO {
		t.Fatalf("insufficient stock should need semi-finished WO")
	}
	if plan2.SemiFinishedGap != 12.7 {
		t.Fatalf("gap = %v, want 12.7", plan2.SemiFinishedGap)
	}
	if plan2.PackagingQty != 22.7 {
		t.Fatalf("packaging qty = %v, want 22.7", plan2.PackagingQty)
	}

	plan3 := DetermineTwoStagePlan(22.7, 22.7)
	if plan3.NeedsSemiFinishedWO {
		t.Fatalf("exact stock should not need semi-finished WO")
	}
}
