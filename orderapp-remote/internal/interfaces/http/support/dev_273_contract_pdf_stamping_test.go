package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev273ContractPDFStampingRequirementSeeds(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-275-CONTRACT-PDF-STAMPING",
		"DEV-275-01",
		"DEV-275-02",
		"DEV-275-03",
		"UT-275-01",
		"API-275-01",
		"REV-275-01",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestDev273ContractPDFStampingVueShellWiring(t *testing.T) {
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	for _, want := range []string{
		"{ key: 'contracts', label: '合同盖章', title: '合同盖章' }",
		"contractPDF",
		"合同PDF",
	} {
		if !strings.Contains(menu, want) {
			t.Fatalf("menu-ia.js missing %q", want)
		}
	}
	app := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	for _, want := range []string{
		"ContractsView",
		"contracts: ContractsView",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ContractsView.vue")))
	if !strings.Contains(view, "下载已盖章PDF") {
		t.Fatal("ContractsView.vue missing stamped PDF download action")
	}
}

func TestDev273ContractPDFStampingManualsDocumentWorkflow(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"合同盖章",
			"DOCX 合同上传后先转换为 PDF",
			"多页拖动公章位置",
			"保存盖章后的 PDF",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing contract stamping manual marker %q", path, want)
			}
		}
	}
}

func TestDev273ContractPDFStampingAcceptanceEvidence(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-14-contract-pdf-stamping.md")))
	for _, want := range []string{
		"CONTRACT_PDF_UPLOAD_OK",
		"CONTRACT_DOCX_CONVERT_OK",
		"CONTRACT_MULTI_PAGE_SEAL_DRAG_OK",
		"CONTRACT_STAMPED_PDF_SAVE_OK",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("contract acceptance evidence missing %q", want)
		}
	}
}

func TestDev273ContractPDFStampingRuntimeIncludesDocxConverter(t *testing.T) {
	dockerfile := string(readOrderAppFileForTest(t, filepath.Join("Dockerfile")))
	for _, want := range []string{
		"libreoffice",
		"DOCX_CONVERTER_CMD=soffice",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing DOCX converter marker %q", want)
		}
	}
}
