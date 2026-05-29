package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev364SkuSettingsCompactDrawersRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-364-SKU-SETTINGS-COMPACT-DRAWERS",
		"DEV-364-SKU-SETTINGS-COMPACT-DRAWERS",
		"UT-364-SKU-SETTINGS-COMPACT-DRAWERS",
		"API-364-SKU-SETTINGS-COMPACT-DRAWERS",
		"REV-364-SKU-SETTINGS-COMPACT-DRAWERS",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU settings compact drawers seed missing %q", want)
		}
	}
}

func TestDev364SkuSettingsCompactDrawersUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"categorySearchQuery",
		"visibleCategoryTreeForSkuContext",
		"category-scroll-list",
		"product-editor-drawer",
		"activeConfigTemplateSection",
		"product-config-template-pane",
		"gradient-template-pane",
		"商品配置模板",
		"阶梯价模板",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing compact drawer marker %q", want)
		}
	}
	if strings.Contains(src, `class="panel public-product-panel"`) {
		t.Fatal("new SKU form must move out of the main master-data panel into a drawer")
	}
}

func TestDev364SkuSettingsCompactDrawersDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-364-SKU-SETTINGS-COMPACT-DRAWERS",
			"商品分类列表必须是固定高度滚动窗",
			"商品配置模板",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-364-SKU-SETTINGS-COMPACT-DRAWERS",
			"搜索商品分类",
			"新增SKU",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-364-SKU-SETTINGS-COMPACT-DRAWERS",
			"新增SKU",
			"固定高度滚动窗",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-settings-compact-drawers.md"): {
			"PR-364",
			"SKU设置紧凑抽屉",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing compact drawer marker %q", rel, want)
			}
		}
	}
}
