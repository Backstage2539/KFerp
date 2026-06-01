package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev368SkuCategoryCollapseAndGlobalUnitRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
		"DEV-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
		"UT-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
		"API-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
		"REV-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU category collapse/global unit seed missing %q", want)
		}
	}
}

func TestDev368SkuCategoryCollapseAndFocusUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"collapsedPrimaryCategoryIds",
		"collapsedSecondaryCategoryIds",
		"togglePrimaryCategoryCollapse",
		"toggleSecondaryCategoryCollapse",
		"focusCategoryAfterCreate",
		"scrollIntoView",
		"category-collapse-button",
		"基础单位在“全局设置”维护",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing collapse/focus marker %q", want)
		}
	}
	for _, blocked := range []string{
		">更换商品配置<",
		"startProductSubtypeConfigEdit",
		"saveProductSubtypeConfig",
		`@submit.prevent="saveProductUnitDefinition"`,
	} {
		if strings.Contains(src, blocked) {
			t.Fatalf("ProductSettingsView.vue should not keep redundant marker %q", blocked)
		}
	}
}

func TestDev368GlobalUnitDictionarySettingsUI(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "UISettingsView.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	for _, want := range []string{
		"全局设置",
		"全局单位字典",
		"productUnitDefinitions",
		"saveGlobalUnitDefinition",
		"/api/product-settings/units",
		"unit-definition-form",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("UISettingsView.vue missing global unit dictionary marker %q", want)
		}
	}
	if !strings.Contains(menu, "label: '全局设置'") {
		t.Fatal("settings menu should expose UISettingsView as 全局设置")
	}
}

func TestDev368SkuCategoryCollapseGlobalUnitDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
			"大类和小类都支持折叠",
			"全局单位字典放到全局设置",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
			"定位到新加类目的位置",
			"产品子类型行不再绑定商品配置模板",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-368-SKU-CATEGORY-COLLAPSE-GLOBAL-UNIT",
			"基础单位在 设置 → 全局设置",
			"商品分类的大类和小类都可以折叠",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-category-collapse-global-unit.md"): {
			"PR-368",
			"全局单位字典",
			"折叠",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing collapse/global unit doc marker %q", rel, want)
			}
		}
	}
}
