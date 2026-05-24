package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev356SalesOrderPaymentOverlayMultipageMarkers(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"salesLayoutBoxMMToPDFPreviewPlacement",
		"salesLayoutBoxMMToPDFPreviewPlacement(snapshot.payment_text_box, previewPDFPages.value",
		"salesLayoutBoxMMToPDFPreviewPlacement(snapshot.payment_code_box, previewPDFPages.value",
		"PDFStampPreview",
		"placement-commit",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("SalesOrderView.vue missing PR-356 multipage overlay marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"salesLayoutBoxMMToPDFPlacement(snapshot.payment_text_box, page",
		"salesLayoutBoxMMToPDFPlacement(snapshot.payment_code_box, page",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("SalesOrderView.vue still locks payment overlay to first page: %q", forbidden)
		}
	}

	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "document-pdf-stamp.js")))
	for _, want := range []string{
		"salesLayoutBoxMMToPDFPreviewPlacement",
		"salesLayoutBoxPreviewPage",
		"fitSalesLayoutBoxWithinPDFPreviewPage",
		"salesDocumentPageBottomMarginMM",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("document-pdf-stamp.js missing PR-356 helper marker %q", want)
		}
	}
}

func TestDev356SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE",
		"DEV-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE",
		"UT-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE",
		"API-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE",
		"REV-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-356 seed %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-sales-order-payment-overlay-multipage.md"),
		filepath.Join("docs", "acceptance", "2026-05-24-sales-order-payment-overlay-multipage.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-356-SALES-ORDER-PAYMENT-OVERLAY-MULTIPAGE", "拖拽框", "收款码"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-356 doc marker %q", rel, want)
			}
		}
	}
}
