package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderInvoiceUploadMissingOrderGuardEvidenceExists(t *testing.T) {
	api := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_invoice.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_invoice_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"LoadOrderInvoice",
		"saveUploadedOrderInvoiceFile",
		"order not found",
	} {
		if !strings.Contains(api, want) {
			t.Fatalf("order invoice API missing missing-order upload guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestOrderInvoiceAPIRejectsMissingOrderWithoutWritingAsset",
		"TestOrderInvoiceAPIRejectsMissingOrderBeforeReadingFile",
		"missing-order.pdf",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("order invoice API test missing missing-order upload guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
		"发票文件上传必须先确认订单存在",
		"TestOrderInvoiceAPIRejectsMissingOrderWithoutWritingAsset",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing invoice missing-order upload guard marker %q", want)
		}
	}
}

func TestOrderInvoiceUploadMissingOrderGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
		"DEV-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
		"UT-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
		"API-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
		"REV-198-ORDER-INVOICE-UPLOAD-MISSING-ORDER-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestOrderInvoiceUploadMissingOrderGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"发票文件上传必须先确认订单存在",
			"订单不存在时返回 order not found",
			"不会写入发票资产文件",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing invoice missing-order upload guard marker %q", path, want)
			}
		}
	}
}
