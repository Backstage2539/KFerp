package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev550SalesSpecOrderOutputFixContracts(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		`code: "PR-550-SALES-SPEC-ORDER-OUTPUT-FIX"`,
		`code: "DEV-550-SALES-SPEC-CONNECTION"`,
		`code: "DEV-550-ORDER-SPEC-QUANTITY"`,
		`code: "DEV-550-LINE-NOTE"`,
		`code: "DEV-550-PDF-CACHE-DOCS"`,
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-550 seed %q", want)
		}
	}

	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-550-SALES-SPEC-ORDER-OUTPUT-FIX", "conn busy", "sales_spec_count",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K92", "1Kg/1Kg", "301Kg", "2.5Kg袋装，共12袋",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"PR-550", "纯规格", "纯件数", "商品行备注",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-550", "完整读取并关闭", "conn busy",
		},
		filepath.Join("docs", "acceptance", "2026-07-24-sales-spec-order-output-fix.md"): {
			"PR-550", "RED", "GREEN", "视觉证据",
		},
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-550 marker %q", rel, want)
			}
		}
	}
}
