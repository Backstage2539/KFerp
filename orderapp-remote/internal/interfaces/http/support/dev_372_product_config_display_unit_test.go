package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev372ProductConfigDisplayUnitRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-372-PRODUCT-CONFIG-DISPLAY-UNIT",
		"DEV-372-PRODUCT-CONFIG-DISPLAY-UNIT",
		"UT-372-PRODUCT-CONFIG-DISPLAY-UNIT",
		"API-372-PRODUCT-CONFIG-DISPLAY-UNIT",
		"REV-372-PRODUCT-CONFIG-DISPLAY-UNIT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config display unit seed missing %q", want)
		}
	}
}

func TestDev372ProductConfigDisplayUnitUI(t *testing.T) {
	vue := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js")))

	for _, want := range []string{
		"价格表展示单位",
		"price_rule_display_unit",
		"priceListRuleDisplayUnitOptions(activeProductUnitDefinitions)",
	} {
		if !strings.Contains(vue, want) {
			t.Fatalf("ProductSettingsView.vue missing display unit marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"盒装/箱装展示",
		"按重量展示",
		"priceListRuleDisplayModeOptions",
		"price_rule_display_mode",
		">展示方式<",
	} {
		if strings.Contains(vue, unwanted) {
			t.Fatalf("ProductSettingsView.vue still contains fixed display mode marker %q", unwanted)
		}
	}
	for _, want := range []string{
		"PRICE_LIST_RULE_DISPLAY_UNIT_INHERIT",
		"继承报价单位",
		"display_unit",
		"display_mode",
		"boxed",
		"weight",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("product-settings.js missing compatibility marker %q", want)
		}
	}
}

func TestDev372ProductConfigDisplayUnitDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-372-PRODUCT-CONFIG-DISPLAY-UNIT",
			"价格表展示单位",
			"display_unit",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-372-PRODUCT-CONFIG-DISPLAY-UNIT",
			"继承报价单位",
			"display_mode=boxed/weight/by_quote_unit",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-372-PRODUCT-CONFIG-DISPLAY-UNIT",
			"价格表展示单位",
			"单位模板和单位换算",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-product-config-display-unit.md"): {
			"PR-372",
			"display_unit",
			"盒装/箱装展示",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing display unit doc marker %q", rel, want)
			}
		}
	}
}
