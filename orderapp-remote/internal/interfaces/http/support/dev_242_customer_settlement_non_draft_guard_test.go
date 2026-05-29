package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementNonDraftGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"FOR UPDATE",
		"settlement batch is not draft",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing non-draft settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateSettlementRejectsNonDraftExistingBatchWithoutChangingFees",
		"settlement batch is not draft",
		"customer_settlement_batches SET status='settled'",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing non-draft settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"已确认或已结算的月结批次不能通过重复生成同周期月结改动",
		"settlement batch is not draft",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing non-draft settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-242-CUSTOMER-SETTLEMENT-NON-DRAFT-IMMUTABLE",
		"DEV-242-CUSTOMER-SETTLEMENT-NON-DRAFT-IMMUTABLE",
		"TestCreateSettlementRejectsNonDraftExistingBatchWithoutChangingFees",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing non-draft settlement marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing non-draft settlement marker %q", want)
		}
	}
	for _, want := range []string{
		"CUSTOMER_SETTLEMENT_NON_DRAFT_UI_CLICK_OK",
		"db=batch_settled_104_extra_fee_unsettled_5",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing non-draft settlement UI marker %q", want)
		}
	}
}
