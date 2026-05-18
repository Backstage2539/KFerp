package sales

import (
	"math"
	"testing"
)

func TestApplyRoundToIntDisabled(t *testing.T) {
	grand, rounding := ApplyRoundToInt(12.34, false)
	if grand != 12.34 || rounding != 0 {
		t.Fatalf("expected unchanged total and zero rounding, got %.2f/%.2f", grand, rounding)
	}
}

func TestApplyRoundToIntEnabled(t *testing.T) {
	grand, rounding := ApplyRoundToInt(12.34, true)
	if grand != 12 {
		t.Fatalf("expected grand=12, got %.2f", grand)
	}
	if math.Abs(rounding-(-0.34)) > 0.000001 {
		t.Fatalf("expected rounding=-0.34, got %.2f", rounding)
	}
}

func TestRetailPackagePrice(t *testing.T) {
	cases := []struct {
		name   string
		retail float64
		specG  int64
		want   float64
	}{
		{name: "base 227g", retail: 38, specG: 227, want: 38},
		{name: "100g rounded up", retail: 38, specG: 100, want: 17},
		{name: "454g doubled", retail: 38, specG: 454, want: 76},
		{name: "250g rounded up", retail: 38, specG: 250, want: 42},
		{name: "zero retail", retail: 0, specG: 227, want: 0},
	}

	for _, tc := range cases {
		if got := RetailPackagePrice(tc.retail, tc.specG); got != tc.want {
			t.Fatalf("%s: RetailPackagePrice() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRetailPackagePriceForSpecPrefersExactSpec(t *testing.T) {
	prices := RetailSpecPrices{Price100G: 42, Price200G: 85, Price227G: 50, Price250G: 55}
	cases := []struct {
		spec int64
		want float64
	}{
		{spec: 100, want: 42},
		{spec: 200, want: 85},
		{spec: 227, want: 50},
		{spec: 250, want: 55},
		{spec: 454, want: 100},
	}
	for _, tc := range cases {
		if got := RetailPackagePriceForSpec(prices, tc.spec); got != tc.want {
			t.Fatalf("spec %dg: got %.2f want %.2f", tc.spec, got, tc.want)
		}
	}
}

func TestRetailPackagePriceForSpecFallsBackTo227GWhenSpecPriceMissing(t *testing.T) {
	prices := RetailSpecPrices{Price227G: 50}
	if got := RetailPackagePriceForSpec(prices, 250); got != 56 {
		t.Fatalf("spec 250g: got %.2f want 56.00", got)
	}
}

func TestRetailAvailableSpecs(t *testing.T) {
	prices := RetailSpecPrices{Price100G: 42, Price200G: 0, Price227G: 50, Price250G: 55}
	got := RetailAvailableSpecs(prices)
	want := []int64{100, 227, 250}
	if len(got) != len(want) {
		t.Fatalf("RetailAvailableSpecs len=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RetailAvailableSpecs[%d]=%d want=%d", i, got[i], want[i])
		}
	}
}

func TestDripBagLineUsesBagTier(t *testing.T) {
	tier := UnitPriceTier{
		ProductKind:  "drip_bag",
		SalesUnit:    "bag",
		MinQty:       100,
		PricePerUnit: 2.15,
		UnitBagCount: 1,
	}
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind:  "drip_bag",
		SalesUnit:    "bag",
		Quantity:     120,
		UnitBagCount: 1,
		Tiers:        []UnitPriceTier{tier},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.UnitPrice != 2.15 || got.LineTotal != 258 {
		t.Fatalf("got unit price %.2f total %.2f", got.UnitPrice, got.LineTotal)
	}
}

func TestDripBoxLineMatchesTierByConvertedBags(t *testing.T) {
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind:  "drip_bag",
		SalesUnit:    "box",
		Quantity:     12,
		UnitBagCount: 10,
		Tiers: []UnitPriceTier{{
			ProductKind:  "drip_bag",
			SalesUnit:    "bag",
			MinQty:       100,
			PricePerUnit: 2.15,
			UnitBagCount: 1,
		}},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.MatchedQtyForTier != 120 || got.UnitPrice != 21.50 || got.LineTotal != 258 {
		t.Fatalf("got matched %.0f unit price %.2f total %.2f", got.MatchedQtyForTier, got.UnitPrice, got.LineTotal)
	}
}

func TestDripBoxLinePrefersBagTierWhenBoxTierAlsoExists(t *testing.T) {
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind:  "drip_bag",
		SalesUnit:    "box",
		Quantity:     12,
		UnitBagCount: 10,
		Tiers: []UnitPriceTier{
			{
				ProductKind:  "drip_bag",
				SalesUnit:    "box",
				MinQty:       10,
				PricePerUnit: 30,
				UnitBagCount: 10,
			},
			{
				ProductKind:  "drip_bag",
				SalesUnit:    "bag",
				MinQty:       100,
				PricePerUnit: 2.15,
				UnitBagCount: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.MatchedQtyForTier != 120 || got.UnitPrice != 21.50 || got.LineTotal != 258 || got.Tier.SalesUnit != "bag" {
		t.Fatalf("got tier %+v matched %.0f unit price %.2f total %.2f", got.Tier, got.MatchedQtyForTier, got.UnitPrice, got.LineTotal)
	}
}

func TestDripBoxLineFallbackToBoxTierReportsBoxMatchedQuantity(t *testing.T) {
	got, err := CalculateUnitLineTotal(UnitLineInput{
		ProductKind:  "drip_bag",
		SalesUnit:    "box",
		Quantity:     12,
		UnitBagCount: 10,
		Tiers: []UnitPriceTier{{
			ProductKind:  "drip_bag",
			SalesUnit:    "box",
			MinQty:       10,
			PricePerUnit: 30,
			UnitBagCount: 10,
		}},
	})
	if err != nil {
		t.Fatalf("CalculateUnitLineTotal: %v", err)
	}
	if got.Tier.SalesUnit != "box" || got.MatchedQtyForTier != 12 || got.UnitPrice != 30 || got.LineTotal != 360 {
		t.Fatalf("got tier %+v matched %.0f unit price %.2f total %.2f", got.Tier, got.MatchedQtyForTier, got.UnitPrice, got.LineTotal)
	}
}
