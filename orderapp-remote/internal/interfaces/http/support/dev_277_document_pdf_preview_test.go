package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev277SalesAndDeliveryUsePDFStampPreview(t *testing.T) {
	checks := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "sales order",
			path: filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"),
			want: []string{
				"PDFStampPreview",
				"`/api/orders/${orderID.value}/sales-order-preview.pdf`",
				"PREVIEW 预览版",
				"@placement-commit",
				"pdfPlacementToSalesSealMM",
				"salesSealMMToPDFPlacement",
				"公章大小",
				"previewSealWidthMM",
				"savePreviewSealSize",
				"salesOrderSealMinWidthMM",
				"salesOrderSealMaxWidthMM",
			},
		},
		{
			name: "delivery note",
			path: filepath.Join("frontend-vue-shell", "src", "views", "DeliveryNoteView.vue"),
			want: []string{
				"PDFStampPreview",
				"`/api/orders/${orderID.value}/delivery-note-preview.pdf`",
				"PREVIEW 预览版",
				"@placement-commit",
				"pdfPlacementToSalesSealMM",
				"salesSealMMToPDFPlacement",
				"公章大小",
				"previewSealWidthMM",
				"savePreviewSealSize",
				"salesOrderSealMinWidthMM",
				"salesOrderSealMaxWidthMM",
			},
		},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			src := string(readOrderAppFileForTest(t, tc.path))
			for _, want := range tc.want {
				if !strings.Contains(src, want) {
					t.Fatalf("%s missing PDF preview marker %q", tc.path, want)
				}
			}
		})
	}
}

func TestDev277PDFStampPreviewPreservesSealAspectAndSizeControls(t *testing.T) {
	stamp := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "document-pdf-stamp.js")))
	for _, want := range []string{
		"normalizePDFStampAspectRatio",
		"scalePDFStampPlacement",
		"sealAspectRatio",
		"salesDocumentSealHeightRatio = 1",
	} {
		if !strings.Contains(stamp, want) {
			t.Fatalf("document-pdf-stamp.js missing aspect-safe marker %q", want)
		}
	}

	contractStamp := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "contract-stamp.js")))
	for _, want := range []string{
		"sealAspectRatio",
		"sealImage.height",
		"sealImage.width",
		"contractPDFDrawPlacement({ pageHeight: height, placement, sealAspectRatio })",
	} {
		if !strings.Contains(contractStamp, want) {
			t.Fatalf("contract-stamp.js missing aspect-safe marker %q", want)
		}
	}

	preview := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "components", "PDFStampPreview.vue")))
	for _, want := range []string{
		"max-width: 100%",
		"max-height: 100%",
		"height: auto",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("PDFStampPreview.vue missing non-stretch image CSS %q", want)
		}
	}

	contracts := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ContractsView.vue")))
	for _, want := range []string{
		"公章大小",
		"contractSealWidth",
		"resizeContractStamps",
		"sealAspectRatio",
		"loadContractSealAspectRatio",
	} {
		if !strings.Contains(contracts, want) {
			t.Fatalf("ContractsView.vue missing contract seal size marker %q", want)
		}
	}

	pdfRenderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	if !strings.Contains(pdfRenderer, "salesOrderSealHeightRatio    = 1") {
		t.Fatalf("sales_order_pdf.go must render sales and delivery seals from a non-elliptical square reference box")
	}
}

func TestDev277DocumentPreviewAndContractWorkspaceDocumentation(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-277-DOCUMENT-PDF-PREVIEW-STAMPING",
		"DEV-277-01",
		"DEV-277-02",
		"DEV-277-03",
		"UT-277-01",
		"API-277-01",
		"REV-277-01",
		"PR-278-CONTRACT-WORKSPACE-SAVE-DELETE",
		"DEV-278-01",
		"DEV-278-02",
		"UT-278-01",
		"API-278-01",
		"REV-278-01",
		"PR-279-DOCUMENT-SEAL-ASPECT-SCALE",
		"DEV-279-01",
		"DEV-279-02",
		"UT-279-01",
		"API-279-01",
		"REV-279-01",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing document preview workspace row %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
	} {
		requirements := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"PR-277-DOCUMENT-PDF-PREVIEW-STAMPING",
			"PREVIEW 预览版",
			"PR-278-CONTRACT-WORKSPACE-SAVE-DELETE",
			"合同标题和备注可保存",
			"PR-279-DOCUMENT-SEAL-ASPECT-SCALE",
			"公章原图比例",
		} {
			if !strings.Contains(requirements, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		checklist := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"PREVIEW 预览版",
			"确认生成 PDF",
			"保存合同",
			"删除合同",
			"公章大小滑轨",
			"不显示为椭圆",
		} {
			if !strings.Contains(checklist, want) {
				t.Fatalf("%s missing %q", path, want)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单/出库单预览显示“PREVIEW 预览版”",
			"合同标题和备注可保存",
			"删除合同会从列表隐藏",
			"公章大小滑轨",
			"圆章不被压成椭圆",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing document preview manual marker %q", path, want)
			}
		}
	}

	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-15-document-pdf-preview-and-contract-workspace.md")))
	for _, want := range []string{
		"SALES_ORDER_PREVIEW_PDF_OK",
		"DELIVERY_NOTE_PREVIEW_PDF_OK",
		"CONTRACT_METADATA_SAVE_DELETE_OK",
		"CONTRACT_WORKSPACE_UI_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("document preview acceptance evidence missing %q", want)
		}
	}
}
