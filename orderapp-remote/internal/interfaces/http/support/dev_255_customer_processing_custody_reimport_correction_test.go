package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerProcessingCustodyReimportCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"upsertCustodyMovementLedgerTx",
		"deltaG - oldDeltaG",
		"appliedDeltaG",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing custody correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta",
		"raw bean balance after corrected reimport",
		"corrected ledger deltas",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing custody correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部生豆入库或出库流水重传并更正数量",
		"按新旧数量差额调整余额",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing custody correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION",
		"DEV-255-CUSTOMER-PROCESSING-CUSTODY-REIMPORT-CORRECTION",
		"TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta",
		"CUSTOMER_PROCESSING_CUSTODY_REIMPORT_DELTA_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing custody correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing custody correction marker %q", want)
		}
	}
}
