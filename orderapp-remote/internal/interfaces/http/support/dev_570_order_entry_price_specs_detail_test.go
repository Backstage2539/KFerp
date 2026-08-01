package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev570OrderEntryPriceSpecsDetailContracts(t *testing.T) {
	markers := []string{
		"PR-570-ORDER-ENTRY-PRICE-SPECS-ORDER-DETAIL",
		"DEV-570-PRICE-TABLE-SPEC-SCOPE",
		"DEV-570-ORDER-DETAIL-SCHEMA-COMPAT",
		"DEV-570-HISTORY-DOCS-DELIVERY",
	}
	for name, rel := range map[string]string{
		"requirement store": filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-08-01-order-entry-price-specs-order-detail.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", name, marker)
			}
		}
	}
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md")))
	if !strings.Contains(manual, "PR-570-ORDER-ENTRY-PRICE-SPECS-ORDER-DETAIL") {
		t.Fatal("order sales manual missing PR-570 marker")
	}

	orderappRoot := findAncestorForTest(t, "go.mod")
	workspaceRoot := filepath.Dir(orderappRoot)
	for _, rel := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		contents, err := os.ReadFile(filepath.Join(workspaceRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(contents), marker) {
				t.Fatalf("root %s missing %s", rel, marker)
			}
		}
	}
}
