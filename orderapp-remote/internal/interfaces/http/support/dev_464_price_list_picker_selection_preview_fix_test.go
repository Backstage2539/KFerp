package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev464PriceListPickerSelectionPreviewFixContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
			"DEV-464-PRICE-LIST-SELECTION-KEY",
			"DEV-464-PRICE-LIST-PICKER-SCOPE",
			"DEV-464-PRICE-LIST-CASCADE-SELECTION-COLLAPSE",
			"REV-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-types.js"): {
			"priceListSelectionStateKey",
			"product-catalog:",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-selection.js"): {
			"priceListVisibleCategoryRows",
			"priceListCategoryProductIDs",
			"priceListCategoryCodesForSelectedProducts",
			"priceListCategoryHiddenByCollapsedAncestor",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"priceListSelectionKey",
			"priceListVisibleCategoryRows",
			"priceListCategoryProductIDs(categoryProductGroups.value",
			"priceListCategoryHiddenByCollapsedAncestor(categoryProductGroups.value",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
			"默认全选该类型下的商品",
			"不展示其他商品类型的空分类",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
			"下方预览立即有内容",
			"挂耳咖啡",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
			"勾选或取消勾选分类/商品会立即更新下方预览",
			"父类收起时子类一起隐藏",
		},
		filepath.Join("docs", "acceptance", "2026-06-10-price-list-picker-selection-preview-fix.md"): {
			"PR-464-PRICE-LIST-PICKER-SELECTION-PREVIEW-FIX",
			"选择分类和产品",
			"预览动态",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-464 marker %q", rel, want)
			}
		}
	}
}
