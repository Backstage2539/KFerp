package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev374ComposableProductPricingRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-374-COMPOSABLE-PRODUCT-PRICING",
		"DEV-374-COMPOSABLE-PRODUCT-PRICING",
		"UT-374-COMPOSABLE-PRODUCT-PRICING",
		"API-374-COMPOSABLE-PRODUCT-PRICING",
		"REV-374-COMPOSABLE-PRODUCT-PRICING",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("composable product pricing seed missing %q", want)
		}
	}
}

func TestDev374ComposableProductPricingSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("internal", "domain", "costing", "engine.go"): {
			"BomCostPerUnit",
			"PriceListRuleJSON",
			"buildComposableProductCommercialTiers",
			"physicalSpecGForDisplayUnit",
			"PriceUnit",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"bom_unit_cost",
			"unit_per_box",
			"unit_per_bag",
			"g_per_bag",
			"price_list_rule_json",
			"&input.BomCostPerUnit",
			"&input.PriceListRuleJSON",
		},
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_form_queries.go"): {
			"PriceUnit               string",
			"published_price_snapshot",
			"inventory_conversion_json",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"const displayUnit = String(tier.display_unit || '').trim()",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js"): {
			"`元/${displayUnit}`",
			"priceUnitForDisplayUnit(tier?.display_unit, tier?.spec_g)",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing composable pricing marker %q", rel, want)
			}
		}
	}
}

func TestDev374ComposableProductPricingDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-374-COMPOSABLE-PRODUCT-PRICING",
			"cost_model",
			"unit_per_box",
			"元/盒",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-374-COMPOSABLE-PRODUCT-PRICING",
			"速溶盒装",
			"display_unit=盒",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"商品价格管理",
			"库存换算",
			"价格表快照",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"商品价格管理",
			"库存换算",
			"全局单位字典",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-composable-product-pricing.md"): {
			"PR-374",
			"元/盒",
			"浏览器验收",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing composable pricing doc marker %q", rel, want)
			}
		}
	}
}
