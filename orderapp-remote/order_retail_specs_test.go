package main

import (
	"os"
	"strings"
	"testing"
)

func TestOrderRetailSpecsUseServerProvidedRetailSpecs(t *testing.T) {
	b, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(b)

	required := []string{
		"const DEFAULT_SPECS = [50, 100, 200, 227, 250, 454, 908, 1000];",
		"function specOptionsHtml(productId, currentSpecG){",
		"p?.retail_specs",
		"refreshSpecOptions(tr, isRetailOrder() ? '' :",
		"refreshRetailSpecsForAllRows();",
	}
	for _, want := range required {
		if !strings.Contains(html, want) {
			t.Fatalf("order retail specs missing %q", want)
		}
	}
}
