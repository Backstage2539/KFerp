package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderSealDragImageRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-136",
		"DEV-136-01",
		"DEV-136-02",
		"UT-136-01",
		"API-136-01",
		"REV-136-01",
		"销售单公章拖动",
		"图片使用最新公章位置",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order seal drag/image requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderSettingsSealDragDoesNotJumpAndSavesOnRelease(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, want := range []string{
		"beginSalesOrderSealDrag",
		"moveSalesOrderSealDrag",
		"saveSealPosition",
		"/api/settings/sales-order/seal-position",
		"松手自动保存",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("SalesOrderSettingsView missing seal drag autosave marker %q", want)
		}
	}
	if strings.Contains(settings, "(clientX - rect.left) * scaleX") {
		t.Fatal("SalesOrderSettingsView should not snap the seal top-left to the clicked point")
	}
}

func TestSalesOrderViewExplainsRegeneratingImageAfterSealMove(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"公章位置已保存，请重新生成图片或 PDF 后下载",
		"已生成图片",
		"下载最新版图片",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing seal/image guidance marker %q", want)
		}
	}
}
