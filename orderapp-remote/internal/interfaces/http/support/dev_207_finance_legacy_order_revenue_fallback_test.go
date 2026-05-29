package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceLegacyOrderRevenueFallbackEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"financeOrderRevenueSQL",
		"COALESCE(%[1]sgrand_total,0) <> 0",
		"COALESCE(%[1]sdiscount_amount,0) <> 0",
		"COALESCE(%[1]sshipping_amount,0) <> 0",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("finance repository missing legacy revenue fallback marker %q", want)
		}
	}
	for _, want := range []string{
		"TestMonthlySourceTotalsUsesLegacyTotalAmountWhenGrandTotalWasDefaultZero",
		"SO-LEGACY-TOTAL",
		"SO-FULL-DISCOUNT",
		"SO-VOID",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("finance repository test missing legacy revenue fallback marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
		"历史订单只有 `total_amount` 且 `grand_total` 为默认 0",
		"TestMonthlySourceTotalsUsesLegacyTotalAmountWhenGrandTotalWasDefaultZero",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing legacy revenue fallback marker %q", want)
		}
	}
}

func TestFinanceLegacyOrderRevenueFallbackRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
		"DEV-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
		"UT-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
		"API-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
		"REV-207-FINANCE-LEGACY-ORDER-REVENUE-FALLBACK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestFinanceLegacyOrderRevenueFallbackManualsAndRequirementDocs(t *testing.T) {
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
			"历史订单只有 `total_amount` 且 `grand_total` 为默认 0",
			"全额折扣",
			"作废订单不计入收入",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing legacy revenue fallback marker %q", path, want)
			}
		}
	}
}
