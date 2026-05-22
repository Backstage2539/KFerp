package support

import (
	"os"
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
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"salesOrderSealMaxWidthMM",
		`:max="salesOrderSealMaxWidthMM"`,
		`@change="savePreviewSealSize"`,
		"公章大小已保存",
		"previewSealWidthMM",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("SalesOrderView missing seal size autosave marker %q", want)
		}
	}
	if strings.Contains(view, `max="80"`) {
		t.Fatal("SalesOrderView still caps seal size at 80mm")
	}
}

func TestDev161SalesOrderSealSizeManualDocumentsWhitePaddingAndAutosave(t *testing.T) {
	rels := []string{"docs/OP_MANUAL_ORDER_SALES.md"}
	if _, err := os.Stat(filepath.Join(findAncestorForTest(t, "go.mod"), "..", "OP_MANUAL_ORDER_SALES.md")); err == nil {
		rels = append(rels, filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"))
	}
	for _, rel := range rels {
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

func TestDev161SalesOrderPreviewUsesPageLevelSealCoordinates(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"PDFStampPreview",
		"salesOrderPreviewPlacements",
		"salesSealMMToPDFPlacement",
		"pdfPlacementToSalesSealMM",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("SalesOrderView missing page-level seal preview marker %q", want)
		}
	}

	helper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "sales-order-seal.js")))
	for _, want := range []string{
		"salesOrderPreviewDesignWidthPX = 1240",
		"salesOrderPreviewPageWidthMM = 210",
		"salesOrderPreviewDesignWidthPX / salesOrderPreviewPageWidthMM",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("sales-order-seal.js missing A4 preview scale marker %q", want)
		}
	}
}
