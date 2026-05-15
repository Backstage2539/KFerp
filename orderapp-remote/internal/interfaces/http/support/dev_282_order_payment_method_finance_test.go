package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev282OrderPaymentMethodFinanceEvidenceExists(t *testing.T) {
	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	ordersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	financeReport := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "FinanceReportView.vue")))
	salesRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	financeRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	handoffExcel := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "excel", "finance_accountant_handoff_excel.go")))
	orderManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md")))
	financeManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_FINANCE.md")))
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	for _, want := range []string{
		"payment_method",
		"orderReceiptMethodOptions",
		"requiresOrderPaymentMethod",
		"请选择收款方式",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing %q", want)
		}
	}
	if !strings.Contains(ordersView, "row.payment_method") {
		t.Fatal("OrdersView.vue should show payment_method in order status")
	}
	if !strings.Contains(financeReport, "row.payment_method") {
		t.Fatal("FinanceReportView.vue should show payment_method in drilldown rows")
	}

	for _, want := range []string{
		"normalizeOrderPaymentMethodForStatusTx",
		"payment_method required",
		"payment_method=$7",
	} {
		if !strings.Contains(salesRepo, want) {
			t.Fatalf("sales repository missing %q", want)
		}
	}
	for _, want := range []string{
		"COALESCE(o.payment_method,'') AS payment_method",
		"&row.PaymentMethod",
	} {
		if !strings.Contains(financeRepo, want) {
			t.Fatalf("finance repository missing %q", want)
		}
	}
	if !strings.Contains(handoffExcel, "Payment method") {
		t.Fatal("accountant handoff Excel should expose Payment method")
	}

	for _, want := range []string{
		"收款方式",
		"已付款",
		"财务管理",
	} {
		if !strings.Contains(orderManual, want) {
			t.Fatalf("order manual missing %q", want)
		}
	}
	for _, want := range []string{
		"收款方式",
		"来源明细",
		"代账交接",
	} {
		if !strings.Contains(financeManual, want) {
			t.Fatalf("finance manual missing %q", want)
		}
	}
	for _, want := range []string{
		"PR-282-ORDER-PAYMENT-METHOD-FINANCE",
		"DEV-282-01",
		"DEV-282-02",
		"DEV-282-03",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store missing %q", want)
		}
	}
}
