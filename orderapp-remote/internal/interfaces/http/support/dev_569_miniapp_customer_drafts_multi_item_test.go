package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev569MiniappCustomerDraftsMultiItemDocumentationContracts(t *testing.T) {
	markers := []string{
		"PR-569-MINIAPP-CUSTOMER-DRAFTS-MULTI-ITEM",
		"DEV-569-CUSTOMER-PERMISSION",
		"DEV-569-ORDER-CUSTOMER-QUICK-EDIT",
		"DEV-569-ORDER-DRAFT",
		"DEV-569-MULTI-ITEM",
		"DEV-569-AUDIT-DOCS-DELIVERY",
	}

	for name, rel := range map[string]string{
		"requirement store": filepath.Join("internal", "interfaces", "http", "support", "req_store.go"),
		"requirements":      filepath.Join("docs", "REQUIREMENTS.md"),
		"acceptance":        filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		"evidence":          filepath.Join("docs", "acceptance", "2026-08-01-miniapp-customer-drafts-multi-item.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s missing %s", name, marker)
			}
		}
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

	for name, rel := range map[string]string{
		"miniapp employee manual": filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"),
		"order sales manual":      filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		"customer manual":         filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
	} {
		contents := string(readOrderAppFileForTest(t, rel))
		if !strings.Contains(contents, "PR-569-MINIAPP-CUSTOMER-DRAFTS-MULTI-ITEM") {
			t.Fatalf("%s missing PR-569 marker", name)
		}
	}

	index := string(readOrderAppFileForTest(t, filepath.Join("docs", "OPERATION_MANUALS.md")))
	for _, marker := range []string{
		"OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md",
		"客户维护",
		"多商品明细",
		"订单草稿",
	} {
		if !strings.Contains(index, marker) {
			t.Fatalf("operation manual index missing %q", marker)
		}
	}
}
