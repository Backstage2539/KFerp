package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementReimportFeeNameCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"appliedSettlementFeeItemIDByFeeLineTx",
		"r.sheet_name=$2",
		"r.row_no=$3",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing settlement fee-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem",
		"仓储费旧名称",
		"corrected settlement fee name",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing settlement fee-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"费用类型或费用名称修正导致完整外部键变化",
		"不会生成重复结算费用",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing settlement fee-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"费用类型或费用名称更正导致完整外部键变化",
		"重复费用",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing settlement fee-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"更正费用名称",
		"重复结算费用",
	} {
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing settlement fee-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION",
		"DEV-264-CUSTOMER-SETTLEMENT-REIMPORT-FEE-NAME-CORRECTION",
		"TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem",
		"CUSTOMER_SETTLEMENT_FEE_NAME_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing settlement fee-name correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement fee-name correction marker %q", want)
		}
	}
}
