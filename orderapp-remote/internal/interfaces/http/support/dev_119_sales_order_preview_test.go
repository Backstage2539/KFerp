package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPreviewDownloadRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-119",
		"DEV-119-01",
		"DEV-119-02",
		"UT-119-01",
		"API-119-01",
		"REV-119-01",
		"销售单预览",
		"历史版本下载",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order preview/download requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderVueRequiresPreviewBeforeGenerate(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"/sales-order-preview",
		"销售单预览",
		"确认生成 PDF",
		":disabled=\"generating || !orderID || !preview\"",
		"PDFStampPreview",
		"salesOrderPreviewPDFUrl",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing preview-before-generate marker %q", want)
		}
	}
}

func TestSalesOrderDownloadRouteParsesPDFDocumentID(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "sales_order_documents.go")))
	for _, want := range []string{
		"parseSalesOrderDocumentID",
		"strings.TrimSuffix(raw, \".pdf\")",
		"path.Base",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order document route missing robust id parser marker %q", want)
		}
	}
}
