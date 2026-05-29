package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev357SalesOrderPaymentOverlayCrossPageMarkers(t *testing.T) {
	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "document-pdf-stamp.js")))
	for _, want := range []string{
		"movePDFStampPlacementAcrossPages",
		"pdfPageHeight",
		"preferredPageNumber",
		"box.page_number",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("document-pdf-stamp.js missing PR-357 cross-page helper marker %q", want)
		}
	}

	preview := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "PDFStampPreview.vue")))
	for _, want := range []string{
		"movePDFStampPlacementAcrossPages",
		"state.original.cross_page_drag",
		"pages.value",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("PDFStampPreview.vue missing PR-357 cross-page drag marker %q", want)
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"cross_page_drag: true",
		"page_number: Number(placement.page_number || 1)",
		"payment_text_page_number: textBox.page_number",
		"payment_code_page_number: codeBox.page_number",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("SalesOrderView.vue missing PR-357 sales payment marker %q", want)
		}
	}

	settingsAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	for _, want := range []string{
		"PaymentTextPageNumber",
		"PaymentCodePageNumber",
		"payment_text_page_number",
		"payment_code_page_number",
	} {
		if !strings.Contains(settingsAPI, want) {
			t.Fatalf("sales_order_settings.go missing PR-357 page persistence marker %q", want)
		}
	}

	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go")))
	for _, want := range []string{
		"payment_text_page_number",
		"payment_code_page_number",
		"PageNumber: settings.PaymentTextPageNumber",
		"PageNumber: settings.PaymentCodePageNumber",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("sales_order_repository.go missing PR-357 page persistence marker %q", want)
		}
	}

	pdf := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"salesOrderLayoutBoxPageNumber",
		"renderSalesOrderPaymentTextOnPage",
		"renderSalesOrderPaymentCodesOnPage",
	} {
		if !strings.Contains(pdf, want) {
			t.Fatalf("sales_order_pdf.go missing PR-357 page rendering marker %q", want)
		}
	}
}

func TestDev357SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE",
		"DEV-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE",
		"UT-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE",
		"API-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE",
		"REV-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-357 seed %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-sales-order-payment-overlay-cross-page.md"),
		filepath.Join("docs", "acceptance", "2026-05-24-sales-order-payment-overlay-cross-page.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-357-SALES-ORDER-PAYMENT-OVERLAY-CROSS-PAGE", "跨页", "收款码"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-357 doc marker %q", rel, want)
			}
		}
	}
}
