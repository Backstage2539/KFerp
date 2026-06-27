package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev361SkuSubtypeParkingStructuredRulesRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
		"DEV-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
		"UT-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
		"API-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
		"REV-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU subtype parking/structured rule seed missing %q", want)
		}
	}
}

func TestDev361SkuSubtypeParkingStructuredRulesUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"产品子类型",
		"product_subtype_category_id",
		"assignCreatedSkuToSelectedProductSubtype",
		"停车场",
		"新增销售单位",
		"价格表生成规则",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing structured subtype UI marker %q", want)
		}
	}
	for _, blocked := range []string{
		"价格表规则 JSON",
		"单位换算 JSON",
		"单位规则 JSON",
		"const categoryID = Number(form?.product_type_category_id",
	} {
		if strings.Contains(src, blocked) {
			t.Fatalf("ProductSettingsView.vue should not expose legacy raw config marker %q", blocked)
		}
	}
}

func TestDev361SkuSubtypeParkingStructuredRulesDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
			"未选产品子类型",
			"停车场",
			"结构化",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-361-SKU-SUBTYPE-PARKING-STRUCTURED-RULES",
			"不进入产品价格表生成",
			"新增销售单位",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"停车场",
			"产品子类型",
			"新增销售单位",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU subtype parking/structured rule docs marker %q", rel, want)
			}
		}
	}
}
