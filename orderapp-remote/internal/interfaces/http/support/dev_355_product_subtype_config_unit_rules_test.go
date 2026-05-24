package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev355ProductSubtypeConfigUnitRulesRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
		"DEV-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
		"UT-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
		"API-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
		"REV-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product subtype config/unit rule seed missing %q", want)
		}
	}
}

func TestDev355ProductSubtypeConfigUnitRulesWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"operation_template_id",
			"price_list_rule_json",
			"unit_conversion_json",
			"integer_unit",
			"gradient_template_id_override",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"OperationTemplateID",
			"PriceListRuleJSON",
			"UnitConversionJSON",
			"IntegerUnit",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildProductCategoryConfigPayload",
			"buildSkuConfigOverridePayload",
			"customer_id",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"子类型配置",
			"库存单位",
			"报价单位",
			"录单单位",
			"新增换算",
			"整数单位",
			"saveProductSubtypeConfig",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing subtype config/unit rule marker %q", rel, want)
			}
		}
	}
}

func TestDev355ProductSubtypeConfigUnitRulesDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
			"产品子类型",
			"单位换算",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
			"冻干速溶",
			"integer_unit",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"产品子类型配置",
			"库存单位",
			"报价单位",
			"录单单位",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-product-subtype-config-unit-rules.md"): {
			"PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES",
			"unit_conversion_json",
			"产品价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing subtype config/unit rule docs marker %q", rel, want)
			}
		}
	}
}
