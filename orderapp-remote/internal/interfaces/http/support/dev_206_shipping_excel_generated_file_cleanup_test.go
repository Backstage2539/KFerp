package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShippingExcelGeneratedFileCleanupEvidenceExists(t *testing.T) {
	shippingExcel := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_shipping_excel.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"cleanupOrderShippingExportFile",
		"CreateOrderShipment",
		"SaveAs(path)",
	} {
		if !strings.Contains(shippingExcel, want) {
			t.Fatalf("order shipping excel API missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"TestOrdersShippingExcelAPICleansFileWhenShipmentSaveFails",
		"order_shipment_test_reject_generated",
		"failed shipment save left shipping export files",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("order shipping excel API test missing generated-file cleanup marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
		"快递录单 Excel 生成失败时必须清理刚写入的文件",
		"TestOrdersShippingExcelAPICleansFileWhenShipmentSaveFails",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing shipping excel generated-file cleanup marker %q", want)
		}
	}
}

func TestShippingExcelGeneratedFileCleanupRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
		"DEV-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
		"UT-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
		"API-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
		"REV-206-SHIPPING-EXCEL-GENERATED-FILE-CLEANUP",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestShippingExcelGeneratedFileCleanupManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"快递录单 Excel 生成失败时必须清理刚写入的文件",
			"不会留下公开孤儿快递录单文件",
			"重新生成快递录单 Excel",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing shipping excel generated-file cleanup marker %q", path, want)
			}
		}
	}
}
