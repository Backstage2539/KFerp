package support

import (
	"strings"
	"testing"
)

func TestDev319BeanListPreviewStylePDFSeedDocsAndCode(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-319-BEAN-LIST-PREVIEW-STYLE-PDF",
		"DEV-319-BEAN-LIST-PREVIEW-STYLE-PDF",
		"UT-319-BEAN-LIST-PREVIEW-STYLE-PDF",
		"API-319-BEAN-LIST-PREVIEW-STYLE-PDF",
		"REV-319-BEAN-LIST-PREVIEW-STYLE-PDF",
		"docs/acceptance/2026-05-22-bean-list-preview-style-pdf.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-319 seed marker %q", want)
		}
	}
	markers := map[string][]string{
		"docs/REQUIREMENTS.md": {
			"PR-319-BEAN-LIST-PREVIEW-STYLE-PDF",
			"米色底、版本/标题/类型胶囊、分类条、商品卡、绿色/蓝色报价块",
			"bean-list-preview-style-v1",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"PR-319-BEAN-LIST-PREVIEW-STYLE-PDF",
			"不得调用 `window.print()`",
			"旧文本版缓存",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"PDF 使用生成抽屉预览的卡片样式",
			"不会打开系统打印窗口",
		},
		"docs/acceptance/2026-05-22-bean-list-preview-style-pdf.md": {
			"生成豆单抽屉",
			"预览卡片样式 PDF",
		},
		"internal/infrastructure/pdf/bean_list_pdf.go": {
			"UsePreviewStyle",
			"gofpdf.SizeType{Wd: 108, Ht: 192}",
			"beanListPreviewState",
		},
		"frontend-vue-shell/src/views/CostingView.vue": {
			"async function generateBeanListPdf()",
			"apiSend('/api/costing/bean-list/drafts'",
			"downloadBeanListPublicationPDF(document)",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-319 marker %q", rel, want)
			}
		}
	}
}
