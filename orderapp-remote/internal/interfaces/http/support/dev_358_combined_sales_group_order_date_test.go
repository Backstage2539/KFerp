package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev358CombinedSalesGroupOrderDateRendererMarkers(t *testing.T) {
	pdfRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"combinedSalesOrderGroupHeaderText",
		`return "订单日期 " + firstNonEmpty(group.OrderDate, group.DocumentDate)`,
	} {
		if !strings.Contains(pdfRenderer, want) {
			t.Fatalf("sales_order_pdf.go missing PR-358 group order date marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`return "订单 " + group.OrderNo`,
		`return "订单号 " + group.OrderNo`,
	} {
		if strings.Contains(pdfRenderer, forbidden) {
			t.Fatalf("sales_order_pdf.go should not keep PR-358 forbidden group order number marker %q", forbidden)
		}
	}

	pngRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go")))
	if !strings.Contains(pngRenderer, "combinedSalesOrderGroupHeaderText(group)") {
		t.Fatalf("sales_order_png.go must reuse combinedSalesOrderGroupHeaderText for PR-358")
	}

	pdfTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf_test.go")))
	for _, want := range []string{
		"TestCombinedSalesOrderGroupHeaderShowsOrderDateInsteadOfOrderNo",
		"订单日期 2026-05-07",
		"SO-20260509-0004",
	} {
		if !strings.Contains(pdfTest, want) {
			t.Fatalf("sales_order_pdf_test.go missing PR-358 test marker %q", want)
		}
	}
}

func TestDev358SeedsManualAndAcceptanceDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"DEV-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"UT-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"API-358-COMBINED-SALES-GROUP-ORDER-DATE",
		"REV-358-COMBINED-SALES-GROUP-ORDER-DATE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-358 seed %q", want)
		}
	}

	for _, rel := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "docs", "acceptance", "2026-05-24-combined-sales-group-order-date.md"),
		filepath.Join("docs", "acceptance", "2026-05-24-combined-sales-group-order-date.md"),
	} {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"PR-358-COMBINED-SALES-GROUP-ORDER-DATE", "订单日期", "订单号"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-358 doc marker %q", rel, want)
			}
		}
	}
}
