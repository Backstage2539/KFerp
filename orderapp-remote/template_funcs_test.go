package main

import (
	"reflect"
	"testing"
)

func TestRetailPriceLinesOnlyReturnsConfiguredSpecs(t *testing.T) {
	got := retailPriceLines(42, 0, 50, 0)
	want := []string{"100g 42.00", "227g 50.00"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retailPriceLines() = %#v, want %#v", got, want)
	}
}
