package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementReimportFeeTypeCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"SheetName   string",
		"RowNo       int",
		"SELECT id, sheet_name, row_no, row_type, external_key, payload",
		"r.sheet_name=$2",
		"r.row_no=$3",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing settlement fee-type correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplySettlementImportReimportCorrectedFeeTypeUpdatesExistingFeeItem",
		`parsedFee("storage", "仓储费", 8000)`,
		"corrected settlement fee type",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing settlement fee-type correction marker %q", want)
		}
	}
	for _, want := range []string{
		"费用类型或费用名称修正导致完整外部键变化",
		"同一结算表和同一 Excel 行号",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing settlement fee-type correction marker %q", want)
		}
	}
	for _, want := range []string{
		"费用类型或费用名称更正导致完整外部键变化",
		"同一结算表和 Excel 行号",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing settlement fee-type correction marker %q", want)
		}
	}
	for _, want := range []string{
		"更正费用类型",
		"费用类型、费用名称和金额显示最新 Excel 值",
	} {
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing settlement fee-type correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION",
		"DEV-265-CUSTOMER-SETTLEMENT-REIMPORT-FEE-TYPE-CORRECTION",
		"TestApplySettlementImportReimportCorrectedFeeTypeUpdatesExistingFeeItem",
		"CUSTOMER_SETTLEMENT_FEE_TYPE_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing settlement fee-type correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement fee-type correction marker %q", want)
		}
	}
}
