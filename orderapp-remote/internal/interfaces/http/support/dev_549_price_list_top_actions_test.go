package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev549PriceListTopActionsContracts(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		`code: "PR-549-PRICE-LIST-TOP-ACTIONS"`,
		`code: "DEV-549-TOP-ACTION-GROUP"`,
		`code: "DEV-549-EQUAL-HEIGHT-SUMMARY"`,
		`code: "DEV-549-REMOVE-REFRESH"`,
		`code: "DEV-549-DOCS-DELIVERY"`,
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-549 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-549-PRICE-LIST-TOP-ACTIONS", "三个入口", "等高", "刷新",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K91", "管理阶梯模板", "计价模式规则", "价格表配置",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-549", "顶部", "三个按钮",
		},
		filepath.Join("docs", "acceptance", "2026-07-23-price-list-top-actions.md"): {
			"PR-549", "RED", "GREEN",
		},
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-549 marker %q", rel, want)
			}
		}
	}
}
