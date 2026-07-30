package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev547PriceListPageToolbarUXContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-547-PRICE-LIST-PAGE-TOOLBAR-UX",
		"DEV-547-PRICE-LIST-LABEL-CLEANUP",
		"DEV-547-PUBLISHED-LIST-TOOLBAR",
		"DEV-547-DOCS-ACCEPTANCE",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-547 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-547-PRICE-LIST-PAGE-TOOLBAR-UX", "计价规则", "上下双箭头",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-547-PRICE-LIST-PAGE-TOOLBAR-UX", "刷新版本", "商品类型",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-547", "计价规则", "已发布价格表",
		},
		filepath.Join("docs", "acceptance", "2026-07-22-price-list-page-toolbar-ux.md"): {
			"PR-547", "RED", "GREEN",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-547 marker %q", rel, want)
			}
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue")))
	for _, want := range []string{
		"publication-list-collapse-toggle",
		"展开已发布价格表",
		"收起已发布价格表",
		"⇊",
		"⇈",
		"<strong>计价规则</strong>",
		".version-controls input, .version-controls select",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CostingView.vue missing PR-547 marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"<span>模型</span>",
		"<strong>Price List / Item Price 生成规则</strong>",
		"查看当前范围下的已发布价格表、生成新版、撤回和归档。",
		"refreshBeanListVersionList",
		"刷新版本",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("CostingView.vue still exposes removed PR-547 marker %q", forbidden)
		}
	}
}
