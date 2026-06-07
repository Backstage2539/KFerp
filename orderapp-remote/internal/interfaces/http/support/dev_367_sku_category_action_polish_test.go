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
	template := strings.Split(src, "<script setup>")[0]
	for _, want := range []string{
		"category-action-button",
		"movePrimaryCategory(primary, -1)",
		"movePrimaryCategory(primary, 1)",
		"deleteCategory(primary)",
		"deleteCategory(secondary)",
		"danger-toggle",
		"danger-text",
		"/api/business-group-items",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing business group action marker %q", want)
		}
	}
	if strings.Contains(template, `class="icon-action`) {
		t.Fatal("business group controls should not use the old square icon-action buttons")
	}
	for _, forbidden := range []string{
		"classification-category-row",
		"moveClassificationCategory(category, -1)",
		"moveClassificationCategory(category, 1)",
		"deleteClassificationCategory(category)",
	} {
		if strings.Contains(template, forbidden) {
			t.Fatalf("ProductSettingsView.vue should not render legacy classification action marker %q", forbidden)
		}
	}
	deleteStart := strings.Index(src, "async function deleteCategory(category)")
	if deleteStart < 0 {
		t.Fatal("deleteCategory function missing")
	}
	deleteEnd := strings.Index(src[deleteStart:], "function startCategoryDrag")
	if deleteEnd < 0 {
		t.Fatal("deleteCategory function end marker missing")
	}
	deleteFunction := src[deleteStart : deleteStart+deleteEnd]
	if !strings.Contains(deleteFunction, "/api/business-group-items") {
		t.Fatal("deleteCategory should call business group item API")
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
