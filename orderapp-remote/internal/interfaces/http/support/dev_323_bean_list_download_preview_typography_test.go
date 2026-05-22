package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev323BeanListDownloadMatchesPreviewTypographySeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY",
		"DEV-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY",
		"UT-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY",
		"API-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY",
		"REV-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY",
		"bean-list-preview-style-v2",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 323 requirement seed missing %q", want)
		}
	}
}

func TestDev323BeanListDownloadMatchesPreviewTypographyWiring(t *testing.T) {
	pdfSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "bean_list_pdf.go")))
	for _, want := range []string{
		"beanListFontBold",
		"previewTitleFontSize   = 19.5",
		"previewNameFontSize    = 15.0",
		"previewPriceFontSize   = 11.25",
		"func (s *beanListPreviewState) drawText",
		"pdf.AddUTF8Font(\"noto\", \"B\"",
	} {
		if !strings.Contains(pdfSrc, want) {
			t.Fatalf("dev 323 pdf renderer missing %q", want)
		}
	}
	serviceSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "costing", "service.go")))
	if !strings.Contains(serviceSrc, "bean-list-preview-style-v2") {
		t.Fatalf("costing service must bump bean-list preview cache key to v2")
	}
}

func TestDev323BeanListDownloadMatchesPreviewTypographyDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_COSTING.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-bean-list-download-preview-typography-match.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-323",
			"字体",
			"粗字重",
			"bean-list-preview-style-v2",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 323 documentation marker %q", rel, want)
			}
		}
	}
}
