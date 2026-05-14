package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderInvoiceSaveFailureCleanupEvidenceExists(t *testing.T) {
	invoiceAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_invoice.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_invoice_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupUploadedOrderInvoiceFile",
		"SaveOrderInvoiceFile",
		"sales_order_assets",
	} {
		if !strings.Contains(invoiceAPI, want) {
			t.Fatalf("order invoice API missing save-failure cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestOrderInvoiceAPIUploadCleansFileWhenInvoiceSaveFails",
		"order_invoice_test_reject_uploaded",
		"assertAssetDirEmpty",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("order invoice API test missing save-failure cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
		"发票文件保存失败时必须清理刚写入的发票资产文件",
		"TestOrderInvoiceAPIUploadCleansFileWhenInvoiceSaveFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing invoice cleanup marker %q", want)
		}
	}
}

func TestOrderInvoiceSaveFailureCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
		"DEV-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
		"UT-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
		"API-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
		"REV-203-ORDER-INVOICE-UPLOAD-SAVE-FAILURE-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestOrderInvoiceSaveFailureCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"发票文件保存失败时必须清理刚写入的发票资产文件",
			"不会留下公开孤儿发票文件",
			"重新上传发票",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing invoice cleanup marker %q", path, want)
			}
		}
	}
}
