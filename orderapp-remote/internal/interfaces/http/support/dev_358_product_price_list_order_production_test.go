package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev358ProductPriceListOrderProductionRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
		"DEV-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
		"UT-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
		"API-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
		"REV-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product price list order/production seed missing %q", want)
		}
	}
}

func TestDev358ProductPriceListOrderProductionWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"): {
			"ProductTypeCategoryID",
			"product_type_category_id",
			"ProductTypeName",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"): {
			"latestProductPriceListVersionOption",
			"product_type_category_id",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"): {
			"ProductSubtypeCategoryID",
			"OperationTemplateID",
			"ProductTypeName",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product price list order/production marker %q", rel, want)
			}
		}
	}
}

func TestDev358ProductPriceListOrderProductionDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
			"录单",
			"生产",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
			"产品类型",
			"速溶咖啡",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-product-price-list-order-production.md"): {
			"PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION",
			"录单",
			"生产",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product price list order/production docs marker %q", rel, want)
			}
		}
	}
}
