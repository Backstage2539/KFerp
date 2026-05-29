package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev307SalesOrderPaymentLayoutControls(t *testing.T) {
	settingsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, want := range []string{
		"个性化说明会优先显示",
		"销售单预览中拖动",
		"selectSeal",
		"/api/settings/sales-order/seal/select",
	} {
		if !strings.Contains(settingsView, want) {
			t.Fatalf("SalesOrderSettingsView missing payment layout marker %q", want)
		}
	}
	for _, old := range []string{
		`form.payment_text_x_mm`,
		`form.payment_code_x_mm`,
		`saveSealPosition`,
		`seal-position-stage`,
	} {
		if strings.Contains(settingsView, old) {
			t.Fatalf("SalesOrderSettingsView should not expose coordinate/seal-position controls, still found %q", old)
		}
	}

	salesOrderView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"文字位置和大小",
		"收款码位置和大小",
		"salesLayoutBoxMMToPDFPreviewPlacement",
		"pdfPlacementToSalesLayoutBox",
		"/api/settings/sales-order/payment-layout",
		"savePDFPreviewLayoutBox",
		"resizable: true",
	} {
		if !strings.Contains(salesOrderView, want) {
			t.Fatalf("SalesOrderView missing draggable payment layout marker %q", want)
		}
	}

	pdfRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"salesOrderPaymentLayoutBoxes",
		"PaymentTextBox",
		"PaymentCodeBox",
		"pdf.SetPage(1)",
		"salesOrderPaymentCodeMetricsForBox",
		"salesOrderPaymentTextSections",
	} {
		if !strings.Contains(pdfRenderer, want) {
			t.Fatalf("sales_order_pdf.go missing payment layout marker %q", want)
		}
	}

	pngRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go")))
	for _, want := range []string{
		"salesOrderPNGMMToPX",
		"salesOrderPaymentLayoutBoxes",
		"paymentCodes(",
	} {
		if !strings.Contains(pngRenderer, want) {
			t.Fatalf("sales_order_png.go missing payment layout marker %q", want)
		}
	}

	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-307-SALES-ORDER-PAYMENT-LAYOUT",
		"DEV-307-SALES-ORDER-PAYMENT-LAYOUT",
		"UT-307-SALES-ORDER-PAYMENT-LAYOUT",
		"API-307-SALES-ORDER-PAYMENT-LAYOUT",
		"REV-307-SALES-ORDER-PAYMENT-LAYOUT",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing sales order payment layout seed %q", want)
		}
	}
}
