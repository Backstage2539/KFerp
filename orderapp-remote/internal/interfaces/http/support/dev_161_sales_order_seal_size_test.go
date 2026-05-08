package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev161SalesOrderSealSizeRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-161",
		"DEV-161-01",
		"DEV-161-02",
		"UT-161-01",
		"API-161-01",
		"REV-161-01",
		"销售单公章上传后本体不能过小",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 161 sales order seal size seed missing %q", want)
		}
	}
}

func TestDev161SalesOrderSealSizeAutosavesAndAllowsLargerMax(t *testing.T) {
	settings := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, want := range []string{
		"salesOrderSealMaxWidthMM",
		`:max="salesOrderSealMaxWidthMM"`,
		`@change="saveSealPosition"`,
		"公章大小已保存",
		"上传公章时会自动裁掉图片白边",
	} {
		if !strings.Contains(settings, want) {
			t.Fatalf("SalesOrderSettingsView missing seal size autosave marker %q", want)
		}
	}
	if strings.Contains(settings, `max="80"`) {
		t.Fatal("SalesOrderSettingsView still caps seal size at 80mm")
	}
}

func TestDev161SalesOrderSealSizeManualDocumentsWhitePaddingAndAutosave(t *testing.T) {
	for _, rel := range []string{
		"docs/OP_MANUAL_ORDER_SALES.md",
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
	} {
		manual := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"上传公章时系统会自动裁掉图片白边",
			"调整公章大小后会自动保存",
			"重新生成 PDF 或图片",
		} {
			if !strings.Contains(manual, want) {
				t.Fatalf("%s missing seal size manual marker %q", rel, want)
			}
		}
	}
}
