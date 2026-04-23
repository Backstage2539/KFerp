package main

import (
	"os"
	"strings"
	"testing"
)

func TestProductRetailPrices202604V302SQL(t *testing.T) {
	b, err := os.ReadFile("db/product_retail_prices_202604_v302.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(b)

	required := []string{
		"retail_price_100g",
		"retail_price_200g",
		"retail_price_227g",
		"retail_price_250g",
		"('橘皮乌龙', 0, 0, 50, 55)",
		"('金色山脉', 0, 0, 50, 55)",
		"('晚香玉', 42, 85, 0, 0)",
		"UPDATE p2rms15pepb5ciz.products p",
		"WHERE p.name = r.name",
	}
	for _, want := range required {
		if !strings.Contains(sql, want) {
			t.Fatalf("retail price SQL missing %q", want)
		}
	}
}
