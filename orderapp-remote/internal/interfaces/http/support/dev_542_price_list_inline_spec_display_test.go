package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev542PriceListInlineSpecDisplayContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-542-PRICE-LIST-INLINE-SPEC-DISPLAY",
		"DEV-542-INLINE-SPEC-PICKER",
		"DEV-542-NAME-SPEC-SNAPSHOT",
		"DEV-542-DOCS-DEPLOY",
		"REV-542-PRICE-LIST-INLINE-SPEC-DISPLAY",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-542 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"product-spec-options", "parent-product-fixed-prices", "priceListFlatRowSpecDescription",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-price-list-selection.js"): {
			"__price_list_display_name", "__price_list_product_name", "__price_list_sales_spec_label",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"sales_spec", "规格",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "costing-price-list-workflow.js"): {
			"priceListFlatRowSpecDescription", "priceListFlatRowContextLabel",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-542-PRICE-LIST-INLINE-SPEC-DISPLAY", "横向", "规格属性", "历史发布",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-542-PRICE-LIST-INLINE-SPEC-DISPLAY", "白月光瑰夏", "227g", "454g",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"横向排列", "商品名", "规格属性", "不重新发布",
		},
		filepath.Join("docs", "acceptance", "2026-07-20-price-list-inline-spec-display.md"): {
			"PR-542", "RED", "GREEN", "生产环境未部署",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-542 marker %q", rel, want)
			}
		}
	}
}
