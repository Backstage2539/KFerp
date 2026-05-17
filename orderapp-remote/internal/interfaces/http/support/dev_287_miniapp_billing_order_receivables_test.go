package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappBillingOrderReceivablesEvidenceExists(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-287-MINIAPP-BILLING-ORDER-RECEIVABLES",
		"DEV-287-MINIAPP-BILLING-ORDER-RECEIVABLES",
		"TestLoadSettlementServicePageIncludesOrderReceivablesForCurrentCustomer",
		"TestGetSettlementServicePageSummaryCountsOrderBills",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing miniapp billing order receivable marker %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"PR-287-MINIAPP-BILLING-ORDER-RECEIVABLES",
			"订单账单",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp billing order receivable marker %q", path, want)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"订单账单",
			"/api/mini/services/settlement",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp billing order receivable manual marker %q", path, want)
			}
		}
	}
}

func TestMiniappBillingOrderReceivablesSourceWiring(t *testing.T) {
	for path, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go"): {
			"ServiceKeySettlement",
			"page.Orders, err = r.listCustomerOrders(ctx, query, limit)",
		},
		filepath.Join("internal", "application", "customerportal", "service.go"): {
			"订单账单",
			"len(page.Orders)",
		},
		filepath.Join("..", "miniapp", "src", "utils", "servicePage.ts"): {
			"orderSectionTitle",
			"订单账单",
		},
		filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue"): {
			"orderSectionTitle(serviceKey.value)",
		},
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp billing order receivable source marker %q", path, want)
			}
		}
	}
}
