package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev635SalesOrderPaymentAmountColorContracts(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-635-SALES-ORDER-PAYMENT-AMOUNT-COLORS",
			"DEV-635-DOCUMENT-PAYMENT-COLORS",
			"DEV-635-DUAL-EXPORT-CONTRACT",
		},
		filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"): {
			"salesOrderPaymentRows",
			"已付金额",
			"未付金额",
		},
		filepath.Join("internal", "interfaces", "http", "sales", "sales_order_documents.go"): {
			"GenerateSalesOrderDocument",
			"GenerateSalesOrderImage",
		},
		filepath.Join("internal", "interfaces", "http", "customerportal", "mini_employee_api.go"): {
			"/api/mini/employee/orders/:id/documents/sales-order.pdf",
			"/api/mini/employee/orders/:id/documents/sales-order.png",
			"GenerateSalesOrderDocument",
			"GenerateSalesOrderImage",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"# PR-635-SALES-ORDER-PAYMENT-AMOUNT-COLORS",
			"完全未付",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"## PR-635 销售单付款金额颜色",
			"网页 ERP",
			"员工小程序",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"已付金额",
			"未付金额",
		},
		filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"): {
			"已付金额",
			"未付金额",
		},
		filepath.Join("docs", "acceptance", "2026-09-07-sales-order-payment-amount-colors.md"): {
			"# PR-635",
			"RED",
			"GREEN",
		},
	}
	for path, markers := range checks {
		body := string(readOrderAppFileForTest(t, path))
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Errorf("%s missing %q", path, marker)
			}
		}
	}
}
