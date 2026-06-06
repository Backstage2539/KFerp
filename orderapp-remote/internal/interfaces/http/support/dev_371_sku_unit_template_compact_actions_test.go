package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev371SkuUnitTemplateCompactActionsRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
		"DEV-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
		"UT-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
		"API-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
		"REV-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU unit template compact action seed missing %q", want)
		}
	}
}

func TestDev371SkuUnitTemplateCompactActionsUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"sku-page-summary",
		"商品档案只维护商品资料",
		"kferp:notify",
		"新增单位模板",
		"成品库存单位",
		"globalUnitEditingCode",
		"新增基础单位",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing compact action marker %q", want)
		}
	}
	if !strings.Contains(src, "productUnitTemplateForm.id ? '保存' : '新增'") {
		t.Fatal("unit template submit button should switch between 新增 and 保存 by edit state")
	}
	if !strings.Contains(src, "globalUnitEditingCode ? '保存' : '新增'") {
		t.Fatal("global unit dictionary submit button should switch between 新增 and 保存 by edit state")
	}
}

func TestDev371SkuUnitTemplateCompactActionsDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
			"SKU归属区域必须压缩高度",
			"成品库存单位",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
			"右下角按钮显示“新增”",
			"右下角按钮显示“保存”",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-371-SKU-UNIT-TEMPLATE-COMPACT-ACTIONS",
			"新增单位模板",
			"成品库存单位",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-unit-template-compact-actions.md"): {
			"PR-371",
			"顶部区域压缩",
			"新增/保存",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing compact action doc marker %q", rel, want)
			}
		}
	}
}
