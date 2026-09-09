package support

import (
	"path/filepath"
	"strings"
	"testing"
)

// PR-643 supersedes the date-only group label. Behavioral PDF/PNG and API tests
// cover order identity, recipient snapshots, amounts and historical files.
func TestDev358SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"DEV-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"UT-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"API-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"REV-358-COMBINED-SALES-GROUP-ORDER-DATE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-358 seed %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-combined-sales-group-order-date.md"),
		filepath.Join("docs", "acceptance", "2026-05-24-combined-sales-group-order-date.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-358-COMBINED-SALES-GROUP-ORDER-DATE", "订单日期", "订单号"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-358 doc marker %q", rel, want)
			}
		}
	}
}
