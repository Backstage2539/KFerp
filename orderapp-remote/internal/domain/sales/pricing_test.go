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
