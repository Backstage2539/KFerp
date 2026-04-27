package production

import "testing"

func TestDefaultInputGramsUsesYieldRate(t *testing.T) {
	got := DefaultInputGrams(800, 0.8)
	if got != 1000 {
		t.Fatalf("DefaultInputGrams() = %d, want 1000", got)
	}
}

func TestDefaultInputGramsFallsBackToPointEight(t *testing.T) {
	got := DefaultInputGrams(2270, 0)
	if got != 2838 {
		t.Fatalf("DefaultInputGrams() = %d, want 2838", got)
	}
}

func TestActualYieldRateRoundsToFourDecimals(t *testing.T) {
	got, err := ActualYieldRate(227, 3, 19, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.7000 {
		t.Fatalf("ActualYieldRate() = %.4f, want 0.7000", got)
	}
}

func TestRunningInventoryPlanPrefersInputAndYield(t *testing.T) {
	got := RunningInventoryPlan(1000, 1000, 2000, 0.8)
	if got.Units != 1 || got.LooseG != 600 {
		t.Fatalf("RunningInventoryPlan() = %d units + %dg, want 1 unit + 600g", got.Units, got.LooseG)
	}
}
