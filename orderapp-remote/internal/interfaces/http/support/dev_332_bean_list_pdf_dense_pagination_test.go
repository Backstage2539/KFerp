package support

import (
	"strings"
	"testing"
)

func TestDev332BeanListPDFDensePaginationSeedDocsAndCode(t *testing.T) {
	store := string(readOrderAppFileForTest(t, "internal/interfaces/http/support/req_store.go"))
	for _, want := range []string{
		"PR-332-BEAN-LIST-PDF-DENSE-PAGINATION",
		"DEV-332-BEAN-LIST-PDF-DENSE-PAGINATION",
		"UT-332-BEAN-LIST-PDF-DENSE-PAGINATION",
		"API-332-BEAN-LIST-PDF-DENSE-PAGINATION",
		"REV-332-BEAN-LIST-PDF-DENSE-PAGINATION",
		"docs/acceptance/2026-05-23-bean-list-pdf-dense-pagination.md",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-332 seed marker %q", want)
		}
	}
	markers := map[string][]string{
		"docs/REQUIREMENTS.md": {
			"PR-332-BEAN-LIST-PDF-DENSE-PAGINATION",
			"normal/compact/dense",
			"bean-list-preview-style-v4",
		},
		"docs/ACCEPTANCE_TESTS.md": {
			"PR-332-BEAN-LIST-PDF-DENSE-PAGINATION",
			"工厂量单 1 个卡片 + 庄园精品豆 2 个卡片",
			"bean-list-preview-style-v4",
		},
		"docs/OP_MANUAL_COSTING.md": {
			"PR-332",
			"normal/compact/dense",
			"bean-list-preview-style-v4",
		},
		"docs/acceptance/2026-05-23-bean-list-pdf-dense-pagination.md": {
			"三档密度",
			"TestRenderBeanListPDFCompactsCardRowsBeforeAddingBlankPage",
		},
		"internal/infrastructure/pdf/bean_list_pdf.go": {
			"beanListCardDensities",
			"name:           \"compact\"",
			"name:           \"dense\"",
		},
		"internal/application/costing/service.go": {
			"bean-list-preview-style-v4",
		},
	}
	for rel, wants := range markers {
		body := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing PR-332 marker %q", rel, want)
			}
		}
	}
}
