package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerProcessingCustodyReimportIdempotencyEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"upsertCustodyMovementLedgerTx",
		"appliedDeltaG == 0 && appliedDeltaUnits == 0",
		"return itemID, nil",
		"addCustodyBalanceTx",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing custody reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance",
		"raw bean balance after reimport",
		"customer_custody_ledger_entries",
		"source_external_key='raw_bean_receipt:IN-REIMPORT:埃塞花魁'",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing custody reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部生豆入库或出库流水",
		"不会重复加减",
		"只保留一条库存台账",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing custody reimport idempotency marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY",
		"DEV-249-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-IDEMPOTENCY",
		"TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance",
		"CUSTOMER_PROCESSING_CUSTODY_REIMPORT_IDEMPOTENCY_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing custody reimport idempotency marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing custody reimport idempotency marker %q", want)
		}
	}
}
