package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev461PriceListPickerTreePricingPopoverContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-461-PRICE-LIST-PICKER-TREE-PRICING-POPOVER",
			"DEV-461-PRICE-LIST-PICKER-TREE",
			"DEV-461-PRICE-LIST-PRICING-POPOVER",
			"DEV-461-PRICE-LIST-CATEGORY-TARGET",
			"DEV-461-PRICE-LIST-PREVIEW-PICKER-GROUPS",
			"DEV-461-DOCS-ACCEPTANCE",
			"REV-461-PRICE-LIST-PICKER-TREE-PRICING-POPOVER",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"productPickerCategoryStyle",
			"productPickerRowStyle",
			"toggleProductPickerCategoryCollapse",
			"price-list-pricing-popover",
			"openPriceListPricingPopover",
			"priceListPricingPopoverOptions",
			"按阶梯模板价计算",
			"按价格模板计算",
			"priceListCategoryTemplateTarget",
			"priceListCategoryTemplateSelection",
			"setPriceListCategoryTemplate",
			"buildBeanListPdfGroupsFromCategoryRows(categoryProductGroups.value",
			"visibleCategoryCodes: pdfVisibleCategoryCodes.value",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"buildBeanListPdfGroupsFromCategoryRows",
			"categoryCode: row.categoryCode",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-bean-list-version-ui.test.js"): {
			"product picker as an indented collapsible tree",
			"edits pricing in an anchored popover",
			"separates parent, subgroup and product overrides",
			"generate drawer should render from the same category rows as product picker",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.test.js"): {
			"explicit picker category rows",
			"1、意式拼配豆",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-461-PRICE-LIST-PICKER-TREE-PRICING-POPOVER",
			"父类、子类、商品逐级缩进",
			"点击分类或商品的 `计价`",
			"预览和生成 PDF",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-461-PRICE-LIST-PICKER-TREE-PRICING-POPOVER",
			"父类、子类和商品逐级缩进",
			"按钮附近弹出四项计价菜单",
			"同一选品分类下的商品不得被旧 bean-list 分类拆成多个预览分类",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-461-PRICE-LIST-PICKER-TREE-PRICING-POPOVER",
			"父类、子类、商品逐级缩进",
			"按钮附近修改计价模式",
			"下方预览和生成 PDF",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-price-list-picker-tree-pricing-popover.md"): {
			"PR-461 商品价格表选品树与计价弹出优化",
			"父类、子类、商品逐级缩进",
			"继承分类",
			"GREEN follow-up",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-461 marker %q", rel, want)
			}
		}
	}
}
