package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev363SkuSettingsWorkbenchLayoutRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
		"DEV-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
		"UT-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
		"API-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
		"REV-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU settings workbench layout seed missing %q", want)
		}
	}
}

func TestDev363SkuSettingsWorkbenchLayoutUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"sectionMode",
		"sku-master-workspace",
		"sku-template-workspace",
		"master-data-layout",
		"template-workspace-stack",
		"商品档案",
		"商品配置模板",
		"currentSettingsSection",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing workbench layout marker %q", want)
		}
	}
	if strings.Index(src, `class="panel product-panel"`) > strings.Index(src, `class="sku-template-workspace"`) {
		t.Fatal("SKU list must stay before template workspace in ProductSettingsView.vue")
	}
}

func TestDev363SkuSettingsWorkbenchLayoutDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
			"商品资料",
			"商品配置",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
			"商品分类和客户SKU列表在商品资料工作区",
			"阶梯价模板和商品配置在商品配置工作区",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-363-SKU-SETTINGS-WORKBENCH-LAYOUT",
			"商品档案",
			"商品配置",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-settings-workbench-layout.md"): {
			"PR-363",
			"SKU设置页面排版",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU settings workbench layout marker %q", rel, want)
			}
		}
	}
}
