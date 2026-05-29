package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev357ProductPriceListGeneralizationRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-357-PRODUCT-PRICE-LIST-GENERALIZATION",
		"DEV-357-PRODUCT-PRICE-LIST-GENERALIZATION",
		"UT-357-PRODUCT-PRICE-LIST-GENERALIZATION",
		"API-357-PRODUCT-PRICE-LIST-GENERALIZATION",
		"REV-357-PRODUCT-PRICE-LIST-GENERALIZATION",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product price list generalization seed missing %q", want)
		}
	}
}

func TestDev357ProductPriceListGeneralizationWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "costing", "schema.go"): {
			"product_type_category_id",
			"product_type_name",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"ProductTypeCategoryID",
			"ProductTypeName",
			"LegacyBeanListTypeProductTypeName",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"产品价格表",
			"发布价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product price list marker %q", rel, want)
			}
		}
	}
}

func TestDev357ProductPriceListGeneralizationDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-357-PRODUCT-PRICE-LIST-GENERALIZATION",
			"产品价格表",
			"产品豆单",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-357-PRODUCT-PRICE-LIST-GENERALIZATION",
			"发布快照",
			"产品类型",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-product-price-list-generalization.md"): {
			"PR-357-PRODUCT-PRICE-LIST-GENERALIZATION",
			"产品价格表",
			"bean_list_publications",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product price list docs marker %q", rel, want)
			}
		}
	}
}
