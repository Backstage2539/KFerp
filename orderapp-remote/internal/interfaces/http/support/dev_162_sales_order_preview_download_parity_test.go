package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev162SalesOrderPreviewDownloadParityRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-162",
		"DEV-162-01",
		"DEV-162-02",
		"UT-162-01",
		"API-162-01",
		"REV-162-01",
		"销售单预览和下载版式必须一致",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 162 preview/download parity seed missing %q", want)
		}
	}
}

func TestDev162SalesOrderManualDocumentsPreviewDownloadParity(t *testing.T) {
	rels := []string{"docs/OP_MANUAL_ORDER_SALES.md"}
	if _, err := os.Stat(filepath.Join(findAncestorForTest(t, "go.mod"), "..", "OP_MANUAL_ORDER_SALES.md")); err == nil {
		rels = append(rels, filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"))
	}
	for _, rel := range rels {
		manual := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"销售单预览按下载图片同一张 A4 页面比例显示",
			"如果预览区域横向滚动",
			"重新生成 PDF 或图片",
		} {
			if !strings.Contains(manual, want) {
				t.Fatalf("%s missing preview/download parity marker %q", rel, want)
			}
		}
	}
}
