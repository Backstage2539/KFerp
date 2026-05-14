package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShippingTrackingExcelUploadSizeGuardEvidenceExists(t *testing.T) {
	orderShippingAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_shipping_excel.go")))
	legacyShippingAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "ship_sf_small_export.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"maxShippingTrackingExcelUploadBytes",
		"readShipmentTrackingExcelUpload",
		"file too large",
	} {
		if !strings.Contains(orderShippingAPI, want) {
			t.Fatalf("order shipping API missing tracking Excel size guard marker %q", want)
		}
	}
	for _, want := range []string{
		"readShipmentTrackingExcelUpload",
		"bytes.NewReader(data)",
	} {
		if !strings.Contains(legacyShippingAPI, want) {
			t.Fatalf("legacy shipping API missing tracking Excel size guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestOrdersShippingTrackingExcelAPIRejectsOversizedUpload",
		"TestLegacyShippingTrackingFillRejectsOversizedUpload",
		"(20<<20)+1",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("sales order API test missing tracking Excel size guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
		"物流单号 Excel 上传超过 20MB 时必须拒绝",
		"TestOrdersShippingTrackingExcelAPIRejectsOversizedUpload",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing tracking Excel size guard marker %q", want)
		}
	}
}

func TestShippingTrackingExcelUploadSizeGuardRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
		"DEV-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
		"UT-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
		"API-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
		"REV-196-SHIPPING-TRACKING-EXCEL-UPLOAD-SIZE-GUARD",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestShippingTrackingExcelUploadSizeGuardManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"物流单号 Excel 上传超过 20MB 时必须拒绝",
			"不能解析超大物流回传 Excel",
			"file too large",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing tracking Excel size guard marker %q", path, want)
			}
		}
	}
}
