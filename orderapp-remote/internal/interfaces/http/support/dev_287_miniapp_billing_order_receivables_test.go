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
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"已确认并推送",
			"不显示订单应收",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing replacement billing manual marker %q", path, want)
			}
		}
	}
}

func TestMiniappBillingOrderReceivablesSourceWiring(t *testing.T) {
	for path, wants := range map[string][]string{
		filepath.Join("internal", "application", "customerportal", "processing_bills.go"): {
			"ListCustomerBills",
			"GetCustomerBill",
			"CapabilitySettlement",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "processing_bills.go"): {
			"ListCustomerProcessingBills",
			"GetCustomerProcessingBill",
			"processing_billing_run_id",
		},
		filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue"): {
			"CustomerBillsPanel",
			"serviceKey.value === 'settlement'",
		},
		filepath.Join("..", "miniapp", "src", "components", "CustomerBillsPanel.vue"): {
			"只显示 ERP 已确认并推送的代加工账单",
			"fetchCustomerBillDetail",
		},
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing replacement miniapp billing source marker %q", path, want)
			}
		}
	}
}
