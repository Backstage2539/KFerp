package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev352InstantCoffeeProductKindRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
		"DEV-352-INSTANT-COFFEE-PRODUCT-KIND",
		"UT-352-INSTANT-COFFEE-PRODUCT-KIND",
		"API-352-INSTANT-COFFEE-PRODUCT-KIND",
		"REV-352-INSTANT-COFFEE-PRODUCT-KIND",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("instant coffee product kind seed missing %q", want)
		}
	}
}

func TestDev352InstantCoffeeProductKindDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
			"速溶咖啡",
			"instant_coffee",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
			"录单",
			"速溶咖啡",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
			"速溶咖啡",
			"原料为速溶咖啡",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
			"速溶咖啡",
			"录单",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-instant-coffee-product-kind.md"): {
			"PR-352-INSTANT-COFFEE-PRODUCT-KIND",
			"go test ./internal/domain/catalog ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/production",
			"node --test src/lib/product-settings.test.js src/lib/order-entry.test.js",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing instant coffee documentation marker %q", rel, want)
			}
		}
	}
}

func TestDev352InstantCoffeeProductKindWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "domain", "catalog", "product_kind.go"): {
			"ProductKindInstantCoffee",
			"instant_coffee",
			"速溶咖啡",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"产品类型",
			"产品子类型",
			"inferProductKindFromProductTypeCategory",
			"速溶咖啡",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"): {
			"kind-instant",
			"速溶咖啡",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "plan_queries.go"): {
			"instant_coffee",
			"速溶咖啡",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing instant coffee wiring marker %q", rel, want)
			}
		}
	}
}
