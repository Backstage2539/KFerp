package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerSettlementReimportFeeIdempotencyEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"appliedSettlementFeeItemIDByExternalKeyTx",
		"r.external_key=$2",
		"r.target_type='customer_fee_item'",
		"JOIN %s.customer_fee_items f ON f.id=r.target_id",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing settlement reimport fee idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplySettlementImportReimportSameFeeDoesNotDuplicateFeeItems",
		"total fee cents after settlement reimport",
		"customer_fee_items",
		"want 8000",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing settlement reimport fee idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部结算费用行",
		"customer_fee_items",
		"不会被重传抬高",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing settlement reimport fee idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY",
		"DEV-250-CUSTOMER-SETTLEMENT-REIMPORT-FEE-IDEMPOTENCY",
		"TestApplySettlementImportReimportSameFeeDoesNotDuplicateFeeItems",
		"CUSTOMER_SETTLEMENT_FEE_REIMPORT_IDEMPOTENCY_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing settlement reimport fee idempotency marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing settlement reimport fee idempotency marker %q", want)
		}
	}
}
