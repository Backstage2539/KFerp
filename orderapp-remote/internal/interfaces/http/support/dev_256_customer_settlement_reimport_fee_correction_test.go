package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementReimportFeeCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"refreshImportedSettlementFeeItemTx",
		"status='unsettled'",
		"settlement_batch_id=0",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing settlement correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplySettlementImportReimportCorrectedFeeUpdatesUnsettledFeeItem",
		"total fee cents after corrected settlement reimport",
		"want 9500",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing settlement correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部结算费用行重传并更正未结费用金额",
		"未结且未绑定月结批次的原费用明细",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing settlement correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION",
		"DEV-256-CUSTOMER-SETTLEMENT-REIMPORT-FEE-CORRECTION",
		"TestApplySettlementImportReimportCorrectedFeeUpdatesUnsettledFeeItem",
		"CUSTOMER_SETTLEMENT_FEE_REIMPORT_AMOUNT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing settlement correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement correction marker %q", want)
		}
	}
}
