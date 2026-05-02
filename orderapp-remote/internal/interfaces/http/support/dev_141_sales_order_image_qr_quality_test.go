package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev141SalesOrderImageQRQualityRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-141",
		"DEV-141-01",
		"DEV-141-02",
		"UT-141-01",
		"API-141-01",
		"REV-141-01",
		"二维码和分享图片质量提升",
		"2480x3508",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 141 requirement seed missing %q", want)
		}
	}
}

func TestDev141SalesOrderPNGAndPDFUseLargerReadableAssets(t *testing.T) {
	pngSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go")))
	for _, want := range []string{
		"salesOrderPNGScale",
		"salesOrderPNGTextWeightOffsetPixels",
		"salesOrderPNGPaymentCodeMetrics",
		"ImageSize: 330",
		"assetImageSharp",
		"xdraw.NearestNeighbor.Scale",
	} {
		if !strings.Contains(pngSrc, want) {
			t.Fatalf("sales_order_png.go missing QR/image quality marker %q", want)
		}
	}

	pdfSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"ImageSize: 64",
		"ImageSize: 52",
	} {
		if !strings.Contains(pdfSrc, want) {
			t.Fatalf("sales_order_pdf.go missing larger payment code marker %q", want)
		}
	}

	testSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf_test.go")))
	for _, want := range []string{
		"TestRenderSalesOrderPNGUsesHighResolutionCanvasAndLargePaymentCode",
		"dominantGreenBounds",
		"2480",
		"620",
	} {
		if !strings.Contains(testSrc, want) {
			t.Fatalf("sales_order_pdf_test.go missing high-resolution QR regression %q", want)
		}
	}
}
