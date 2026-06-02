package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev370UnitTemplateDrawerSkuCategoryLayoutRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
		"DEV-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
		"UT-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
		"API-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
		"REV-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("unit template drawer/category layout seed missing %q", want)
		}
	}
}

func TestDev370UnitTemplateDrawerAndCategoryLayoutUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"unit-template-list-panel",
		"unit-template-editor-panel",
		"globalUnitDrawerOpen",
		"global-unit-dictionary-drawer",
			"openGlobalUnitDictionaryDrawer",
			"saveGlobalUnitDefinitionFromDrawer",
			"buildProductUnitDefinitionPayload",
			"classification-template-list",
			"classification-category-editor",
		} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing drawer/layout marker %q", want)
		}
	}
	unitListPos := strings.Index(src, "unit-template-list-panel")
	unitEditorPos := strings.Index(src, "unit-template-editor-panel")
	if unitListPos < 0 || unitEditorPos < 0 || unitListPos > unitEditorPos {
		t.Fatal("unit template list should be before editor for left-list/right-editor layout")
	}
	listPos := strings.Index(src, `classification-template-list`)
	editorPos := strings.Index(src, `classification-category-editor`)
	if listPos < 0 || editorPos < 0 || listPos > editorPos {
		t.Fatal("classification template list should render before category editor")
	}
}

func TestDev370UnitTemplateDrawerSkuCategoryLayoutDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
			"单位模板页提供“全局单位字典”按钮",
			"大类标题和上下排序按钮放在左侧",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
			"右侧抽屉维护全局单位字典",
			"折叠/展开按钮在右侧",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-370-UNIT-TEMPLATE-DRAWER-SKU-CATEGORY-LAYOUT",
			"单位模板左侧是模板列表，右侧是新增或编辑表单",
			"产品类型标题和上下排序按钮在左侧",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-unit-template-drawer-sku-category-layout.md"): {
			"PR-370",
			"全局单位字典抽屉",
			"左列表右编辑",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing drawer/category layout doc marker %q", rel, want)
			}
		}
	}
}
