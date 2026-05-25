package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev163SalesOrderSettingsSealPreviewVisibilityRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-163",
		"DEV-163-01",
		"UT-163-01",
		"API-163-01",
		"REV-163-01",
		"销售单预览页公章下移后不能消失",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 163 settings seal preview visibility seed missing %q", want)
		}
	}
}

func TestDev163SalesOrderPreviewUsesScrollablePDFCanvas(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	preview := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "PDFStampPreview.vue")))
	for _, want := range []string{
		"PDFStampPreview",
		"pdf-stamp-pages",
		"overflow-x: auto",
		"salesOrderPreviewPlacements",
		"previewSealWidthMM",
	} {
		if !strings.Contains(view, want) && !strings.Contains(preview, want) {
			t.Fatalf("SalesOrderView/PDFStampPreview missing scrollable PDF seal canvas marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"aspect-ratio: 2.5 / 1",
		"overflow: hidden",
	} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("SalesOrderView still clips lowered seal with %q", unwanted)
		}
	}
}

func TestDev163SalesOrderManualDocumentsSettingsSealPreviewVisibility(t *testing.T) {
	rels := []string{"docs/OP_MANUAL_ORDER_SALES.md"}
	for _, rel := range rels {
		manual := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"销售单预览显示 PDF 页面",
			"销售单和出库单预览页可直接拖动公章位置",
			"重新生成 PDF 或图片",
		} {
			if !strings.Contains(manual, want) {
				t.Fatalf("%s missing settings seal preview visibility manual marker %q", rel, want)
			}
		}
	}
}
