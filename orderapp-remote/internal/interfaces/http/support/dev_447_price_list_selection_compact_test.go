package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev447PriceListSelectionCompactContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-447-PRICE-LIST-SELECTION-COMPACT",
			"DEV-447-PRICE-LIST-CATEGORY-COMPACT",
			"DEV-447-PRICE-LIST-PRODUCT-COMPACT",
			"REV-447-PRICE-LIST-SELECTION-COMPACT",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"category-pricing-summary",
			"product-compact-status",
			"priceListCategoryPricingSummary",
			"priceListProductDisplaySummary",
			"openPriceListCategoryPricingDialog",
			"openPriceListProductPricingDialog",
			"openPriceListProductDisplayDialog",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-447-PRICE-LIST-SELECTION-COMPACT",
			"默认只显示勾选、名称、款数、计价状态和展示状态",
			"不新增 API、数据库字段、Pricing Rule 字段、阶梯模板字段或发布快照字段",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-447-PRICE-LIST-SELECTION-COMPACT",
			"分类计价、商品行计价、标签和标红词不常驻展开",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-447",
			"默认只显示分类/商品选择和计价、展示摘要",
			"点击分类或商品行的 `计价 / 展示` 摘要后",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-price-list-selection-compact.md"): {
			"PR-447 商品价格表选品区降噪",
			"默认不常驻显示分类计价、商品行计价、标签和标红词配置",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-447 contract marker %q", rel, want)
			}
		}
	}
}
