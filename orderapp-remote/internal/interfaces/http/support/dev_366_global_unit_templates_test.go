package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev366GlobalUnitTemplatesRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-366-GLOBAL-UNIT-TEMPLATES",
		"DEV-366-GLOBAL-UNIT-TEMPLATES",
		"UT-366-GLOBAL-UNIT-TEMPLATES",
		"API-366-GLOBAL-UNIT-TEMPLATES",
		"REV-366-GLOBAL-UNIT-TEMPLATES",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("global unit template seed missing %q", want)
		}
	}
}

func TestDev366GlobalUnitTemplatesSchemaAndAPI(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"product_unit_definitions",
			"product_unit_templates",
			"ALTER TABLE %[1]s.product_config_templates ADD COLUMN IF NOT EXISTS unit_template_id",
			"ALTER TABLE %[1]s.pricing_gradient_templates ADD COLUMN IF NOT EXISTS unit_template_id",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"/api/product-settings/units",
			"/api/product-settings/unit-templates",
			"saveProductUnitDefinitionAPI",
			"saveProductUnitTemplateAPI",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"ProductUnitDefinition",
			"ProductUnitTemplate",
			"UnitTemplateID",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing global unit template marker %q", rel, want)
			}
		}
	}
}

func TestDev366GlobalUnitTemplatesUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	for _, want := range []string{
		"unit-template-pane",
		"productUnitDefinitions",
		"productUnitTemplates",
		"saveProductUnitTemplate",
		"productConfigTemplateForm.unit_template_id",
		"销售规格模板",
		"销售规格明细",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing global unit template marker %q", want)
		}
	}
	for _, want := range []string{
		"全局单位字典",
		"saveGlobalUnitDefinition",
		"/api/product-settings/units",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("UISettingsView.vue missing global unit dictionary marker %q", want)
		}
	}
	if strings.Contains(src, `<div class="field-group-title">单位规则</div>`) && strings.Contains(src, "productConfigTemplateForm.unit_conversion_rows") {
		t.Fatal("product config template should choose a unit template instead of editing unit conversion rows directly")
	}
}

func TestDev366GlobalUnitTemplatesDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-366-GLOBAL-UNIT-TEMPLATES",
			"单位字典是全局基础资料",
			"商品配置模板只引用单位模板",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-366-GLOBAL-UNIT-TEMPLATES",
			"新增单位：盒",
			"盒装200g",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-366-GLOBAL-UNIT-TEMPLATES",
			"基础单位",
			"单位模板",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-global-unit-templates.md"): {
			"PR-366",
			"全局单位字典",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing global unit template doc marker %q", rel, want)
			}
		}
	}
}
