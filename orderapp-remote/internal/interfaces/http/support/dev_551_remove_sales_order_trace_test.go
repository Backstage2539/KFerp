package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev551RemoveSalesOrderTraceContracts(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		`code: "PR-551-REMOVE-SALES-ORDER-TRACE"`,
		`code: "DEV-551-REMOVE-TRACE-UI"`,
		`code: "DEV-551-REMOVE-TRACE-REQUEST"`,
		`code: "DEV-551-DOCS-DELIVERY"`,
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-551 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-551-REMOVE-SALES-ORDER-TRACE", "订单详情继续保留", "不再为了追溯额外请求",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K93", "销售单初始化不再请求订单详情追溯接口", "订单详情仍显示报价来源和生产来源",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-551", "只在订单详情只读展示", "销售单页面不再重复展示或刷新追溯",
		},
		filepath.Join("docs", "acceptance", "2026-07-24-remove-sales-order-trace.md"): {
			"PR-551", "RED", "GREEN", "订单详情继续保留",
		},
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-551 marker %q", rel, want)
			}
		}
	}
}
