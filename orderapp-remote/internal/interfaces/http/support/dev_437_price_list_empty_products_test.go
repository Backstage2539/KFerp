package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev437PriceListEmptyProductsSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-437-PRICE-LIST-EMPTY-PRODUCTS",
		"DEV-437-BEAN-LIST-EMPTY-CATALOG",
		"API-437-PRICE-LIST-EMPTY-PRODUCTS",
		"REV-437-PRICE-LIST-EMPTY-PRODUCTS",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-437 requirement seed missing %q", want)
		}
	}
}

func TestDev437PriceListEmptyProductsWiringAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "application", "costing", "service.go"): {
			"len(inputs) == 0",
			"Items: []domain.ProductResult{}",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestBeanListAllowsEmptyProductCatalog",
			"want empty response",
			"TestCalculateRejectsEmptyProducts",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-437-PRICE-LIST-EMPTY-PRODUCTS",
			"商品价格表",
			"products required",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-437-PRICE-LIST-EMPTY-PRODUCTS",
			"空状态",
			"products required",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-437-PRICE-LIST-EMPTY-PRODUCTS",
			"products required",
			"空状态",
		},
		filepath.Join("docs", "acceptance", "2026-06-06-price-list-empty-products.md"): {
			"PR-437-PRICE-LIST-EMPTY-PRODUCTS",
			"商品价格表",
			"products required",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-437 marker %q", rel, want)
			}
		}
	}
}
