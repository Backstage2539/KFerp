package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPaymentCodeUploadGuardEvidenceExists(t *testing.T) {
	settingsAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_settings.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"maxSalesOrderSettingsAssetUploadBytes",
		"isAllowedSalesOrderSettingsImage",
		"image file required",
		"image file too large",
	} {
		if !strings.Contains(settingsAPI, want) {
			t.Fatalf("sales order settings API missing payment code upload guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestSalesOrderPaymentCodeUploadRejectsNonImageAsset",
		"TestSalesOrderPaymentCodeUploadRejectsOversizedAsset",
		"pay.html",
		"huge.jpg",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("sales order API test missing payment code upload guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
		"销售单收款码上传只接受图片文件",
		"TestSalesOrderPaymentCodeUploadRejectsNonImageAsset",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing payment code upload guard marker %q", want)
		}
	}
}

func TestSalesOrderPaymentCodeUploadGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
		"DEV-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
		"UT-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
		"API-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
		"REV-195-SALES-ORDER-PAYMENT-CODE-UPLOAD-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestSalesOrderPaymentCodeUploadGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单收款码上传只接受图片文件",
			"不能上传 HTML 或脚本文件作为收款码",
			"收款码图片超过 8MB 时必须拒绝",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing payment code upload guard marker %q", path, want)
			}
		}
	}
}
