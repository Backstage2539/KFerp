package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev546OrderProductCategoryFilterContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-546-ORDER-PRODUCT-CATEGORY-FILTER",
		"DEV-546-PRODUCT-CATEGORY-FILTER",
		"DEV-546-DROPDOWN-OUTSIDE-DISMISS",
		"DEV-546-DOCS-DELIVERY",
		"REV-546-ORDER-PRODUCT-CATEGORY-FILTER",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-546 seed %q", want)
		}
	}
	for _, want := range []string{
		`code: "PR-546-ORDER-PRODUCT-CATEGORY-FILTER", title: "录单商品下拉支持当前价格表分类过滤并在点击外部时自动收起", status: "done"`,
		`code: "REV-546-ORDER-PRODUCT-CATEGORY-FILTER", prCode: "PR-546-ORDER-PRODUCT-CATEGORY-FILTER", title: "验收：录单商品按分类搜索且点击下拉外任意位置自动收起", status: "done"`,
		"Van manual acceptance 2026-07-22",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-546 closure marker %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-546-ORDER-PRODUCT-CATEGORY-FILTER", "分类过滤", "点击下拉框外",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-546-ORDER-PRODUCT-CATEGORY-FILTER", "全部", "自动收起",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-546", "熟豆", "点击商品下拉外部",
		},
		filepath.Join("docs", "acceptance", "2026-07-22-order-product-category-filter.md"): {
			"PR-546", "RED", "GREEN", "未部署",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-546 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"product-kind-filter",
		"productKindFilterOptions(row)",
		"handleOrderProductPointerDown",
		"document.addEventListener('pointerdown'",
		"document.removeEventListener('pointerdown'",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("OrderEntryView.vue missing PR-546 marker %q", want)
		}
	}
}
