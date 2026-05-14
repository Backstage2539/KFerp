package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPaymentCodeLabelGuardEvidenceExists(t *testing.T) {
	settingsAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"label := strings.TrimSpace(c.FormValue(\"label\"))",
		"label required",
		"saveUploadedSalesOrderAsset",
	} {
		if !strings.Contains(settingsAPI, want) {
			t.Fatalf("sales order settings API missing payment code label guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestSalesOrderPaymentCodeUploadRequiresLabelBeforeWritingAsset",
		"blank label payment code upload wrote orphan asset entries",
		"assertNoSalesOrderAssetRows",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("sales order API test missing payment code label guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
		"销售单收款码上传必须先填写标签",
		"TestSalesOrderPaymentCodeUploadRequiresLabelBeforeWritingAsset",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing payment code label guard marker %q", want)
		}
	}
}

func TestSalesOrderPaymentCodeLabelGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
		"DEV-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
		"UT-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
		"API-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
		"REV-201-SALES-ORDER-PAYMENT-CODE-LABEL-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestSalesOrderPaymentCodeLabelGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单收款码上传必须先填写标签",
			"标签为空时不能写入收款码资产",
			"补齐标签后再上传",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing payment code label guard marker %q", path, want)
			}
		}
	}
}
