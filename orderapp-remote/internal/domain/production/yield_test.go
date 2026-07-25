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

func TestInventoryQuantityProjectionKeepsFractionalKgAndDoesNotInflateLosslessOutput(t *testing.T) {
	inventoryQty := SalesSpecCountToInventoryQuantity(4, 0.454)
	if inventoryQty != 1.816 {
		t.Fatalf("inventory quantity = %.9f, want 1.816", inventoryQty)
	}
	if got := InventoryQuantityToLegacyGrams(inventoryQty, "kg"); got != 1816 {
		t.Fatalf("legacy grams = %d, want 1816", got)
	}
	if got := DefaultInputGrams(1816, 1); got != 1816 {
		t.Fatalf("lossless input = %d, want 1816", got)
	}
}

func TestSalesSpecCountToInventoryQuantityRejectsMissingAndNegativeInputs(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		salesSpecCount           float64
		inventoryQtyPerSalesUnit float64
	}{
		{name: "missing count", salesSpecCount: 0, inventoryQtyPerSalesUnit: 0.454},
		{name: "negative count", salesSpecCount: -4, inventoryQtyPerSalesUnit: 0.454},
		{name: "missing conversion", salesSpecCount: 4, inventoryQtyPerSalesUnit: 0},
		{name: "negative conversion", salesSpecCount: 4, inventoryQtyPerSalesUnit: -0.454},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SalesSpecCountToInventoryQuantity(tc.salesSpecCount, tc.inventoryQtyPerSalesUnit); got != 0 {
				t.Fatalf("inventory quantity = %.9f, want 0", got)
			}
		})
	}
}

func TestInventoryQuantityToLegacyGramsRejectsMissingAndNonWeightUnits(t *testing.T) {
	for _, unit := range []string{"", "件"} {
		t.Run(unit, func(t *testing.T) {
			if got := InventoryQuantityToLegacyGrams(1.816, unit); got != 0 {
				t.Fatalf("legacy grams = %d, want 0", got)
			}
		})
	}
}

func TestInventoryQuantityToLegacyGramsUsesExactPoundConversion(t *testing.T) {
	if got := InventoryQuantityToLegacyGrams(10, "lb"); got != 4536 {
		t.Fatalf("legacy grams = %d, want 4536", got)
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

func TestExpectedLossRateIsDerivedFromYieldRate(t *testing.T) {
	got := ExpectedLossRate(0.82)
	if got != 0.18 {
		t.Fatalf("ExpectedLossRate() = %.4f, want 0.1800", got)
	}
}

func TestYieldRateFromExpectedLossRateRejectsInvalidValues(t *testing.T) {
	for _, loss := range []float64{-0.01, 1, 1.1} {
		if _, err := YieldRateFromExpectedLossRate(loss); err == nil {
			t.Fatalf("YieldRateFromExpectedLossRate(%v) error = nil, want error", loss)
		}
	}
}

func TestYieldRateFromExpectedLossRateConvertsToYield(t *testing.T) {
	got, err := YieldRateFromExpectedLossRate(0.1855)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.8145 {
		t.Fatalf("YieldRateFromExpectedLossRate() = %.4f, want 0.8145", got)
	}
}

func TestActualLossMetricsDeriveLossQtyAndRate(t *testing.T) {
	lossQty, lossRate, err := ActualLossMetrics(1000, 815)
	if err != nil {
		t.Fatal(err)
	}
	if lossQty != 185 {
		t.Fatalf("loss qty = %.3f, want 185", lossQty)
	}
	if lossRate != 0.185 {
		t.Fatalf("loss rate = %.4f, want 0.1850", lossRate)
	}
}

func TestAggregateJobCardActualsSumsLossAcrossOperations(t *testing.T) {
	got := AggregateJobCardActuals([]JobCardActual{
		{ActualInputQty: 1000, ActualOutputQty: 820, ActualLossQty: 180},
		{ActualInputQty: 820, ActualOutputQty: 800, ActualLossQty: 20},
	})
	if got.ActualInputQty != 1820 {
		t.Fatalf("actual input = %.3f, want 1820", got.ActualInputQty)
	}
	if got.ActualOutputQty != 1620 {
		t.Fatalf("actual output = %.3f, want 1620", got.ActualOutputQty)
	}
	if got.ActualLossQty != 200 {
		t.Fatalf("actual loss = %.3f, want 200", got.ActualLossQty)
	}
	if got.ActualLossRate != 0.1099 {
		t.Fatalf("actual loss rate = %.4f, want 0.1099", got.ActualLossRate)
	}
}

func TestRunningInventoryPlanPrefersInputAndYield(t *testing.T) {
	got := RunningInventoryPlan(1000, 1000, 2000, 0.8)
	if got.Units != 1 || got.LooseG != 600 {
		t.Fatalf("RunningInventoryPlan() = %d units + %dg, want 1 unit + 600g", got.Units, got.LooseG)
	}
}
