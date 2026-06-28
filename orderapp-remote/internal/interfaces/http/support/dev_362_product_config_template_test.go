package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev362ProductConfigTemplateRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-362-PRODUCT-CONFIG-TEMPLATE",
		"DEV-362-PRODUCT-CONFIG-TEMPLATE",
		"UT-362-PRODUCT-CONFIG-TEMPLATE",
		"API-362-PRODUCT-CONFIG-TEMPLATE",
		"REV-362-PRODUCT-CONFIG-TEMPLATE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("product config template seed missing %q", want)
		}
	}
}

func TestDev362ProductConfigTemplateUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"商品配置",
		"复制为客户配置",
		"product_config_template_id",
		"商品档案配置",
		"productConfigTemplates",
		"saveProductConfigTemplate",
		"deriveProductConfigTemplateForCustomer",
		"/api/product-settings/product-config-templates",
		"usePublicSkuInCategoryTree: false",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing product config template marker %q", want)
		}
	}
	for _, blocked := range []string{
		"客户产品规则",
		"客户规则模板",
		"客户专属覆盖",
		"纳入产品价格表",
		"usePublicSkuInCategoryTree: customerUsesPublicCategories.value",
	} {
		if strings.Contains(src, blocked) {
			t.Fatalf("ProductSettingsView.vue should not expose legacy product rule marker %q", blocked)
		}
	}
}

func TestDev362ProductConfigTemplateDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-362-PRODUCT-CONFIG-TEMPLATE",
			"商品配置",
			"客户只开启公共商品分类但未开启公共 SKU",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-362-PRODUCT-CONFIG-TEMPLATE",
			"盒装速溶配置",
			"分类树显示公共分类但不显示公共 SKU",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-392",
			"复制为客户配置",
			"227g袋装 / 袋 / 227 g",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-392",
			"商品配置",
			"是否进入产品价格表由产品价格表页面决定",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-product-config-template.md"): {
			"PR-362",
			"Product price table inclusion is controlled on 产品价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing product config template docs marker %q", rel, want)
			}
		}
	}
}
