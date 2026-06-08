package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev376ProductConfigSpecialKVRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
		"DEV-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
		"UT-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
		"API-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
		"REV-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config special KV seed missing %q", want)
		}
	}
}

func TestDev376ProductConfigSpecialKVSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"price_rule_fixed_unit_price",
			"price_rule_cost_plus_percent",
			"商品配置模板",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"商品生产配置",
			"productionConfigPriceListFields",
			"show_in_price_list",
			"/api/product-production-configs",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"price-list-page-config",
			"attributeLines",
			"itemProductAttributeLines",
			"bean-attrs",
			"product-picker-row",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"SpecialAttrsSchemaJSON",
			"fixed_unit_price required",
			"cost_plus_rate required",
		},
		filepath.Join("internal", "domain", "costing", "engine.go"): {
			"ProductAttribute",
			"productAttributesFromSpecialAttrs",
			"show_in_price_list",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"findProductConfigTemplateBySourceTx",
			"source.SpecialAttrsSchemaJSON",
		},
		filepath.Join("internal", "interfaces", "http", "costing", "bean_list_pdf.go"): {
			"beanListPublicationPDFAttributeLines",
			"productAttributes",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config special KV marker %q", rel, want)
			}
		}
	}
}

func TestDev376ProductConfigSpecialKVDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"cost_plus_rate",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-376-PRODUCT-CONFIG-SPECIAL-KV-PRICE-LIST",
			"PR-389-BOM-GROUP-SPECIAL-ATTRS",
			"速溶咖啡产品价格表",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-389",
			"特殊属性",
			"固定单价",
		},
		filepath.Join("docs", "acceptance", "2026-05-26-product-config-special-kv-price-list.md"): {
			"PR-376",
			"浏览器验收",
			"烘焙度：中深烘",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config special KV doc marker %q", rel, want)
			}
		}
	}
}
