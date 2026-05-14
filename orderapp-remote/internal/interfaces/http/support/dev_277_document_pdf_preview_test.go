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
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing document preview workspace row %q", want)
		}
	}

	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	for _, want := range []string{
		"PR-277-DOCUMENT-PDF-PREVIEW-STAMPING",
		"PREVIEW 预览版",
		"PR-278-CONTRACT-WORKSPACE-SAVE-DELETE",
		"合同标题和备注可保存",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing %q", want)
		}
	}

	checklist := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	for _, want := range []string{
		"PREVIEW 预览版",
		"确认生成 PDF",
		"保存合同",
		"删除合同",
	} {
		if !strings.Contains(checklist, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"销售单/出库单预览显示“PREVIEW 预览版”",
			"合同标题和备注可保存",
			"删除合同会从列表隐藏",
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
