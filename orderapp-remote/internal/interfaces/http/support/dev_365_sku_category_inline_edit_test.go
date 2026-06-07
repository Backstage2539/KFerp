package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev365SkuCategoryInlineEditRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-365-SKU-CATEGORY-INLINE-EDIT",
		"DEV-365-SKU-CATEGORY-INLINE-EDIT",
		"UT-365-SKU-CATEGORY-INLINE-EDIT",
		"API-365-SKU-CATEGORY-INLINE-EDIT",
		"REV-365-SKU-CATEGORY-INLINE-EDIT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SKU category inline edit seed missing %q", want)
		}
	}
}

func TestDev365SkuCategoryInlineEditUI(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	template := strings.Split(src, "<script setup>")[0]
	for _, want := range []string{
		"sku-category-management-workspace",
		"createPrimaryCategoryInline",
		"createSecondaryCategoryInline",
		"saveProductCatalogBusinessGroupItem",
		"moveProductCatalogBusinessGroupItem",
		"deleteCategory",
		"/api/business-group-items",
		"新增大类",
		"停用大类",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing business group edit marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"classification-template-list",
		"classification-category-editor",
		"openClassificationTemplateCreateDrawer",
		"分类模板只定义分类结构",
		"category-editor-drawer",
		"openCategoryDrawer",
		">编辑产品类型<",
		">改名<",
	} {
		if strings.Contains(template, forbidden) {
			t.Fatalf("ProductSettingsView.vue should not render legacy category edit marker %q", forbidden)
		}
	}
}

func TestDev365SkuCategoryInlineEditDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-365-SKU-CATEGORY-INLINE-EDIT",
			"点击产品类型或产品子类型名称直接改名",
			"大类右侧箭头",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-365-SKU-CATEGORY-INLINE-EDIT",
			"加号",
			"红色减号",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-365-SKU-CATEGORY-INLINE-EDIT",
			"点击名称直接改名",
			"子类型仍按原拖动方式排序",
		},
		filepath.Join("docs", "acceptance", "2026-05-25-sku-category-inline-edit.md"): {
			"PR-365",
			"产品类型列表内直接操作",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing inline category edit marker %q", rel, want)
			}
		}
	}
}
