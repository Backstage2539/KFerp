package support

import (
	"os"
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
		"销售单设置页公章下移后预览不能消失",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 163 settings seal preview visibility seed missing %q", want)
		}
	}
}

func TestDev163SalesOrderSettingsUsesScrollableA4SealCanvas(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, want := range []string{
		"sealStageViewportStyle",
		"sealStageCanvasStyle",
		"salesOrderPreviewDesignHeightPX",
		"salesOrderPreviewDesignWidthPX",
		"salesOrderSealSettingsViewportHeight",
		"seal-stage-canvas",
		".seal-position { display: grid; grid-template-columns: 1fr;",
		".seal-position-fields { order: -1;",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("SalesOrderSettingsView missing A4 seal canvas marker %q", want)
		}
	}
	for _, unwanted := range []string{
		"aspect-ratio: 2.5 / 1",
		"overflow: hidden",
	} {
		if strings.Contains(settings, unwanted) {
			t.Fatalf("SalesOrderSettingsView still clips lowered seal with %q", unwanted)
		}
	}
}

func TestDev163SalesOrderManualDocumentsSettingsSealPreviewVisibility(t *testing.T) {
	rels := []string{"docs/OP_MANUAL_ORDER_SALES.md"}
	if _, err := os.Stat(filepath.Join(findAncestorForTest(t, "go.mod"), "..", "OP_MANUAL_ORDER_SALES.md")); err == nil {
		rels = append(rels, filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"))
	}
	for _, rel := range rels {
		manual := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"销售单设置页公章预览使用可滚动的 A4 坐标画布",
			"公章下移后不会被短预览框裁掉",
			"重新生成 PDF 或图片",
		} {
			if !strings.Contains(manual, want) {
				t.Fatalf("%s missing settings seal preview visibility manual marker %q", rel, want)
			}
		}
	}
}
