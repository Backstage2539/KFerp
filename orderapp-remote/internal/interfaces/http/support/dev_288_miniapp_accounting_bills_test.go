package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniappAccountingBillsEvidenceExists(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-288-MINIAPP-ACCOUNTING-BILLS",
		"DEV-288-MINIAPP-ACCOUNTING-BILLS",
		"TestGetSettlementServicePageSummaryShowsReceivableLedger",
		"TestMiniSettlementServicePageAPIParsesBillingFilters",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing miniapp accounting bills marker %q", want)
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
			"PR-288-MINIAPP-ACCOUNTING-BILLS",
			"未付款金额",
			"订单号跳转订单页",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp accounting bills marker %q", path, want)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"费用中心",
			"关联真实工单",
			"不显示订单应收",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing replacement miniapp accounting bills manual marker %q", path, want)
			}
		}
	}
}

func TestMiniappAccountingBillsSourceWiring(t *testing.T) {
	for path, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go"): {
			"/api/mini/customer-bills",
			"ListCustomerBills",
			"GetCustomerBill",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "processing_bills.go"): {
			"processing_billing_run_id>0",
			"customer_id=$1",
			"status IN ('confirmed','settled','paid','reversed')",
		},
		filepath.Join("..", "miniapp", "src", "api", "customerPortal.ts"): {
			"buildCustomerBillsPath",
			"fetchCustomerBillDetail",
		},
		filepath.Join("..", "miniapp", "src", "components", "CustomerBillsPanel.vue"): {
			"关联工单",
			"费用项目",
			"计费依据",
		},
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing replacement miniapp accounting bills source marker %q", path, want)
			}
		}
	}
}
