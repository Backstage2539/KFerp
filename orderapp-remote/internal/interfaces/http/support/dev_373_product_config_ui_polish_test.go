package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev373ProductConfigUIPolishRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-373-PRODUCT-CONFIG-UI-POLISH",
		"DEV-373-PRODUCT-CONFIG-UI-POLISH",
		"UT-373-PRODUCT-CONFIG-UI-POLISH",
		"API-373-PRODUCT-CONFIG-UI-POLISH",
		"REV-373-PRODUCT-CONFIG-UI-POLISH",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config UI polish seed missing %q", want)
		}
	}
}

func TestDev373ProductConfigUIPolishUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"product-config-row",
		"product-config-row-title",
		"template-state-pill",
		"template-meta-chips",
		"price-rule-grid",
		"rule-config-field",
		"grid-template-rows: 22px auto",
		`type="button" class="field-help-icon"`,
		"min-height: 16px",
		"field-help-tooltip",
		`role="tooltip"`,
		"productConfigUnitChips(config.unit_template_id)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing UI polish marker %q", want)
		}
	}
	if strings.Contains(src, "<small>默认继承单位模板") {
		t.Fatal("price rule inline help should use the exclamation tooltip instead of an inline small note")
	}
}

func TestDev373ProductConfigUIPolishDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-373-PRODUCT-CONFIG-UI-POLISH",
			"商品配置模板列表",
			"感叹号提示",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-373-PRODUCT-CONFIG-UI-POLISH",
			"三列对齐",
			"弹出提示",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-373-PRODUCT-CONFIG-UI-POLISH",
			"商品配置模板列表",
			"感叹号",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-product-config-ui-polish.md"): {
			"PR-373",
			"product-config-row",
			"field-help-tooltip",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing UI polish doc marker %q", rel, want)
			}
		}
	}
}
