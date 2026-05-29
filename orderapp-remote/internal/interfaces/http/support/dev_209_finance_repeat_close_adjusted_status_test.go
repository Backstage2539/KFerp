package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceRepeatCloseAdjustedStatusEvidenceExists(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service.go")))
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service_test.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"report.Status != domain.MonthStatusAdjusted",
		"report.Status = domain.MonthStatusClosed",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("finance service missing repeat-close adjusted marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCloseMonthPreservesAdjustedStatus",
		"repeat close does not downgrade post-close adjustments",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("finance service test missing repeat-close adjusted marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCloseMonthKeepsAdjustedStatusAfterAdjustment",
		"补记费用",
		"persisted report status",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("finance repository test missing repeat-close adjusted marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
		"重复结账",
		"TestCloseMonthKeepsAdjustedStatusAfterAdjustment",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing repeat-close adjusted marker %q", want)
		}
	}
}

func TestFinanceRepeatCloseAdjustedStatusRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
		"DEV-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
		"UT-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
		"API-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
		"REV-209-FINANCE-REPEAT-CLOSE-ADJUSTED-STATUS",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestFinanceRepeatCloseAdjustedStatusManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_FINANCE.md"),
		filepath.Join("docs", "OP_MANUAL_FINANCE.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"重复结账",
			"已调整",
			"不会降回已结账",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing repeat-close adjusted marker %q", path, want)
			}
		}
	}
}
