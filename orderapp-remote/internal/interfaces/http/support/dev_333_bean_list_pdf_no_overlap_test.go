package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev333BeanListPDFNoOverlapSeedsDocsAndCode(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-333-BEAN-LIST-PDF-NO-OVERLAP",
		"DEV-333-BEAN-LIST-PDF-NO-OVERLAP",
		"UT-333-BEAN-LIST-PDF-NO-OVERLAP",
		"API-333-BEAN-LIST-PDF-NO-OVERLAP",
		"REV-333-BEAN-LIST-PDF-NO-OVERLAP",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing PR-333 seed marker %q", want)
		}
	}

	renderer := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "bean_list_pdf.go")))
	for _, want := range []string{
		"groupStartMinHeight",
		"keepAfter",
		"bean-list-preview-style-v4",
	} {
		if !strings.Contains(renderer, want) && want == "bean-list-preview-style-v4" {
			service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "costing", "service.go")))
			if !strings.Contains(service, want) {
				t.Fatalf("renderer/service missing PR-333 marker %q", want)
			}
			continue
		}
		if !strings.Contains(renderer, want) {
			t.Fatalf("bean_list_pdf.go missing PR-333 marker %q", want)
		}
	}

	pdfTests := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "bean_list_pdf_test.go")))
	for _, want := range []string{
		"TestCardRowLayoutDoesNotSqueezeBelowRenderableHeight",
		"TestRenderGroupKeepsTitleWithFirstCardRow",
		"TestRenderBeanListPDFCompactsCardRowsBeforeAddingBlankPage",
	} {
		if !strings.Contains(pdfTests, want) {
			t.Fatalf("bean_list_pdf_test.go missing PR-333 regression %q", want)
		}
	}

	docs := strings.Join([]string{
		string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md"))),
		string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md"))),
		string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_COSTING.md"))),
		string(readOrderAppFileForTest(t, filepath.Join("docs", "acceptance", "2026-05-23-bean-list-pdf-no-overlap.md"))),
	}, "\n")
	for _, want := range []string{
		"PR-333-BEAN-LIST-PDF-NO-OVERLAP",
		"分类标题",
		"报价块",
		"bean-list-preview-style-v4",
	} {
		if !strings.Contains(docs, want) {
			t.Fatalf("PR-333 docs missing %q", want)
		}
	}
}
