package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev449PriceListSelectionFeedbackContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-449-PRICE-LIST-SELECTION-FEEDBACK",
			"DEV-449-PRICE-LIST-SUMMARY-DIALOG",
			"DEV-449-PRICE-LIST-PREVIEW-CURRENT-SELECTION",
			"DEV-449-PRICE-LIST-PAGE-CONFIG",
			"REV-449-PRICE-LIST-SELECTION-FEEDBACK",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"price-list-page-config",
			"价格表配置",
			"计价模式规则",
			"priceListRulesDialogOpen",
			"price-list-config-dialog",
			"openPriceListCategoryPricingDialog",
			"openPriceListProductPricingDialog",
			"openPriceListProductDisplayDialog",
			"return '继承分类'",
			"downloadSourcePublication.value?.content?.groups",
			"pdfCategoryCodesForVisibleSelection",
			"visibleCategoryCodes: pdfVisiblePreviewCategoryCodes.value",
			"v-if=\"priceListFlatRows.length\" class=\"pdf-picker flat-price-row-editor\"",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-449-PRICE-LIST-SELECTION-FEEDBACK",
			"继承状态统一显示 `继承分类`",
			"生成抽屉预览必须按当前勾选商品生成",
			"主页面直接展示 `Price List / Item Price 生成规则`",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-449-PRICE-LIST-SELECTION-FEEDBACK",
			"不能因当前已发布版本为空而空白",
			"`计价模式规则` 按钮弹窗展示",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-449",
			"点击分类或商品行的 `计价 / 展示` 摘要后，在弹窗中修改配置",
			"主页面配置区直接展示 `Price List / Item Price 生成规则`",
			"没有生成价格行时不显示平铺价格行块",
		},
		filepath.Join("docs", "acceptance", "2026-06-08-price-list-selection-feedback.md"): {
			"PR-449 商品价格表选品区验收反馈",
			"价格表配置",
			"预览按当前勾选商品生成",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-449 contract marker %q", rel, want)
			}
		}
	}
}
