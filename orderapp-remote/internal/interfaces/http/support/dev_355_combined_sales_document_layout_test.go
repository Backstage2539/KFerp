package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev355CombinedSalesDocumentLayoutRendererMarkers(t *testing.T) {
	pdfRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"combinedSalesOrderHeaderMetaRows",
		"combinedSalesOrderGroupHeaderText",
		"renderSalesOrderPaymentInfoSectionWithPageBreak",
		"salesOrderPaymentSectionNeedsNewPage",
		"renderSalesOrderPageNumber",
		"销售单 SALES ORDER",
	} {
		if !strings.Contains(pdfRenderer, want) {
			t.Fatalf("sales_order_pdf.go missing PR-355 renderer marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"组合销售单 COMBINED SALES ORDER",
		"组合单号：",
		"订单数：",
		"订单 %s    单据日期",
	} {
		if strings.Contains(pdfRenderer, forbidden) {
			t.Fatalf("sales_order_pdf.go should not keep old combined header marker %q", forbidden)
		}
	}

	pngRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go")))
	for _, want := range []string{
		"salesOrderPNGDocumentHeight",
		"combinedSalesOrderPNGDocumentHeight",
		"paymentInfoBottom",
		"combinedSalesOrderHeaderMetaRows",
		"combinedSalesOrderGroupHeaderText",
	} {
		if !strings.Contains(pngRenderer, want) {
			t.Fatalf("sales_order_png.go missing PR-355 long-image marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"组合销售单 COMBINED SALES ORDER",
		"组合单号：",
		"订单数：",
		"单据日期：\"+firstNonEmpty(group.DocumentDate",
	} {
		if strings.Contains(pngRenderer, forbidden) {
			t.Fatalf("sales_order_png.go should not keep old combined header marker %q", forbidden)
		}
	}
}

func TestDev355SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION",
		"DEV-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION",
		"UT-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION",
		"API-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION",
		"REV-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-355 seed %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-combined-sales-document-layout-pagination.md"),
		filepath.Join("docs", "acceptance", "2026-05-24-combined-sales-document-layout-pagination.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-355-COMBINED-SALES-DOCUMENT-LAYOUT-PAGINATION", "组合销售单", "单据日期", "长图"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-355 doc marker %q", rel, want)
			}
		}
	}
}
