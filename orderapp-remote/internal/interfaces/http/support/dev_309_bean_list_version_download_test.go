package support

import (
	"strings"
	"testing"
)

func TestDev309BeanListVersionDownloadSeed(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-309-BEAN-LIST-VERSION-DOWNLOAD",
		"DEV-309-BEAN-LIST-VERSION-DOWNLOAD",
		"UT-309-BEAN-LIST-VERSION-DOWNLOAD",
		"API-309-BEAN-LIST-VERSION-DOWNLOAD",
		"REV-309-BEAN-LIST-VERSION-DOWNLOAD",
		"docs/acceptance/2026-05-22-bean-list-version-download.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-309 seed marker %q", want)
		}
	}
}

func TestDev309BeanListVersionDownloadDocsAndWiring(t *testing.T) {
	markers := map[string][]string{
		"frontend-vue-shell/src/views/CostingView.vue": {
			"下载 PDF",
			"downloadBeanListPublication(row)",
			"downloadSourcePublication.value?.content?.groups",
			"beanListPublicationHasContent(row)",
			"apiSend(`/api/costing/bean-list/publications/${row.id}/pdf?${params.toString()}`",
			"downloadBeanListPublicationPDF(document)",
		},
		"frontend-vue-shell/src/lib/bean-list-pdf.js": {
			"export function beanListPublicationPdfOptions",
			"publication.config",
			"publication.version",
			"publication.changelog",
			"showCategoryNumbers",
		},
		"docs/REQUIREMENTS.md": {
			"PR-309-BEAN-LIST-VERSION-DOWNLOAD",
			"`content` 和 `config` 快照",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"PR-309-BEAN-LIST-VERSION-DOWNLOAD",
			"每行都展示“下载 PDF”",
			"不能因当前商品价格",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"在版本列表点击“下载 PDF”",
			"已锁定内容和样式",
		},
		"docs/acceptance/2026-05-22-bean-list-version-download.md": {
			"下载必须使用该行版本保存时锁定的 `content` 和 `config` 快照",
			"公共豆单、客户豆单、草稿、已发布和已撤回历史版本",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing bean-list version download marker %q", rel, want)
			}
		}
	}
}
