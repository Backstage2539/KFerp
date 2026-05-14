package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSettlementServicePageIsolationEvidenceExists(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	repoTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "repository_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"listFeeItems(ctx context.Context, customerID int64",
		"listSettlementBatches(ctx context.Context, customerID int64",
		"WHERE customer_id=$1",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("customerportal repository missing settlement isolation marker %q", want)
		}
	}
	for _, want := range []string{
		"TestLoadSettlementServicePageFiltersFinanceRowsByCustomer",
		"客户A结算单",
		"客户B不应泄露",
	} {
		if !strings.Contains(repoTest, want) {
			t.Fatalf("customerportal repository test missing settlement isolation marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
		"结算服务页客户隔离",
		"TestLoadSettlementServicePageFiltersFinanceRowsByCustomer",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement isolation marker %q", want)
		}
	}
}

func TestSettlementServicePageIsolationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
		"DEV-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
		"UT-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
		"API-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
		"REV-185-SETTLEMENT-SERVICE-PAGE-ISOLATION",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestSettlementServicePageIsolationManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"结算服务页",
			"费用明细",
			"不能显示其他客户",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing settlement service page isolation marker %q", path, want)
			}
		}
	}
}
