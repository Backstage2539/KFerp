package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev367SkuCategoryActionPolishRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-367-SKU-CATEGORY-ACTION-POLISH",
		"DEV-367-SKU-CATEGORY-ACTION-POLISH",
		"UT-367-SKU-CATEGORY-ACTION-POLISH",
		"API-367-SKU-CATEGORY-ACTION-POLISH",
		"REV-367-SKU-CATEGORY-ACTION-POLISH",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU category action polish seed missing %q", want)
		}
	}
}

func TestDev367SkuCategoryActionPolishUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"classification-category-row",
		"moveClassificationCategory(category, -1)",
		"moveClassificationCategory(category, 1)",
		"deleteClassificationCategory(category)",
		"danger-text",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing classification action marker %q", want)
		}
	}
	if strings.Contains(src, `class="icon-action`) {
		t.Fatal("classification template controls should not use the old square icon-action buttons")
	}
	deleteStart := strings.Index(src, "async function deleteClassificationCategory(category)")
	if deleteStart < 0 {
		t.Fatal("deleteClassificationCategory function missing")
	}
	deleteEnd := strings.Index(src[deleteStart:], "async function saveProductClassificationTemplateUsage")
	if deleteEnd < 0 {
		t.Fatal("deleteClassificationCategory function end marker missing")
	}
	deleteFunction := src[deleteStart : deleteStart+deleteEnd]
	if !strings.Contains(deleteFunction, "product-classification-template-categories") {
		t.Fatal("deleteClassificationCategory should call classification template category API")
	}
}

func TestDev367SkuCategoryActionPolishDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-367-SKU-CATEGORY-ACTION-POLISH",
			"紧凑胶囊式操作控件",
			"红色删除减号显示在对应分类行右侧",
			"不再弹出二次确认",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-367-SKU-CATEGORY-ACTION-POLISH",
			"不再是大方块按钮",
			"不弹二次确认",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-367-SKU-CATEGORY-ACTION-POLISH",
			"紧凑胶囊按钮",
			"不再二次弹窗确认",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-category-action-polish.md"): {
			"PR-367",
			"红色删除按钮都在右侧",
			"删除分类不再需要二次确认",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing SKU category action polish doc marker %q", rel, want)
			}
		}
	}
}
