package main

import (
	"os"
	"strings"
	"testing"
)

func TestProductTierGramsModelInTemplatesAndRepository(t *testing.T) {
	tpl, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(tpl)
	for _, want := range []string{
		"function tierOptionsHtml(productId, specG)",
		"data-spec-g",
		"/件",
		"function matchTierPrice(pid, specG, units)",
		"低于最低档(${m.minTier}件)",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("order template missing %q", want)
		}
	}
	if strings.Contains(html, "/lb") || strings.Contains(html, "元/磅") {
		t.Fatalf("order template should present product tiers in grams/package units, not lb")
	}

	repo, err := os.ReadFile("sales_order_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(repo)
	for _, want := range []string{
		"COALESCE(NULLIF(spec_g,0),454)=$2",
		"COALESCE(min_qty_units, min_qty_lb) <= $3",
		"COALESCE(price_per_unit, price_per_lb)",
		"items[idx].lineTotal = price * float64(items[idx].units)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sales repository missing %q", want)
		}
	}
}
