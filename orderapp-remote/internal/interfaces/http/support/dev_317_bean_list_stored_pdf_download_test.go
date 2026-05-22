package support

import (
	"strings"
	"testing"
)

func TestDev317BeanListStoredPDFDownloadSeed(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
		"DEV-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
		"UT-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
		"API-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
		"REV-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
		"docs/acceptance/2026-05-22-bean-list-stored-pdf-download.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-314 seed marker %q", want)
		}
	}
}

func TestDev317BeanListStoredPDFDownloadDocsAndUI(t *testing.T) {
	markers := map[string][]string{
		"docs/REQUIREMENTS.md": {
			"PR-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
			"不得调用浏览器打印",
			"bean_list_publication_assets",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"PR-317-BEAN-LIST-STORED-PDF-DOWNLOAD",
			"不弹出打印窗口",
			"`/api/costing/bean-list/publications/:id/pdf`",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"服务器按该版本快照生成并保存 PDF",
			"不再调用浏览器打印窗口",
		},
		"docs/acceptance/2026-05-22-bean-list-stored-pdf-download.md": {
			"POST /api/costing/bean-list/publications/:id/pdf",
			"bean_list_publication_assets(publication_id, asset_type='pdf')",
		},
		"frontend-vue-shell/src/views/CostingView.vue": {
			"apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`",
			"apiFetch(document.download_url)",
			"URL.createObjectURL(blob)",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing stored pdf marker %q", rel, want)
			}
		}
	}
}

func TestDev317BeanListPDFDownloadUsesReadPermission(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		if got := requiredPermissionForRequest(method, "/api/costing/bean-list/publications/7/pdf"); got != "costing.read" {
			t.Fatalf("%s bean-list pdf permission = %q, want costing.read", method, got)
		}
	}
	if got := requiredPermissionForRequest("POST", "/api/costing/bean-list/publications"); got != "auth.manage" {
		t.Fatalf("publish permission = %q, want auth.manage", got)
	}
}
