package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev545OrderPriceListCatalogScopeContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-545-ORDER-PRICE-LIST-CATALOG-SCOPE",
		"DEV-545-CLASSIFICATION-VERSION-IDENTITY",
		"DEV-545-ACTIVE-PUBLICATION-FILTER",
		"DEV-545-HISTORY-COMPAT",
		"DEV-545-DOCS-DEPLOY",
		"REV-545-ORDER-PRICE-LIST-CATALOG-SCOPE",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-545 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-545-ORDER-PRICE-LIST-CATALOG-SCOPE", "分类模板", "旧价格表",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-545-ORDER-PRICE-LIST-CATALOG-SCOPE", "V3.0.19", "V3.0.18",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-545", "历史兼容价格表", "默认不参与新订单",
		},
		filepath.Join("docs", "acceptance", "2026-07-22-order-price-list-catalog-scope.md"): {
			"PR-545", "RED", "GREEN", "历史订单",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-545 marker %q", rel, want)
			}
		}
	}
}
