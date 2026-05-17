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
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"账期筛选",
			"本周、本月、本年",
			"订单号跳转订单页",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp accounting bills manual marker %q", path, want)
			}
		}
	}
}

func TestMiniappAccountingBillsSourceWiring(t *testing.T) {
	for path, wants := range map[string][]string{
		filepath.Join("internal", "application", "customerportal", "service.go"): {
			"settlementAccountingSummary",
			"应收总额",
			"未付款金额",
			"PaymentMethod",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go"): {
			"COALESCE(o.payment_method,'')",
			"listCustomerOrders(ctx, query, limit, false)",
		},
		filepath.Join("..", "miniapp", "src", "utils", "orderFilters.ts"): {
			"'week'",
			"'year'",
		},
		filepath.Join("..", "miniapp", "src", "pages", "service", "service.vue"): {
			"账期筛选",
			"openOrderFromBill",
			"serviceKey !== 'settlement' && page?.orders?.length",
			"paymentMethodText",
		},
	} {
		body := string(readOrderAppFileForTest(t, path))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing miniapp accounting bills source marker %q", path, want)
			}
		}
	}
}
