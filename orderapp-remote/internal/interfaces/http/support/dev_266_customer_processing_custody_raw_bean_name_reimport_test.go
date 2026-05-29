package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerProcessingCustodyRawBeanNameReimportCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"previousItemID",
		"r.sheet_name=$3",
		"r.row_no=$4",
		"FOR UPDATE OF l",
		"-previousDeltaG",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing raw bean name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance",
		"BEAN-NAME-CORRECT",
		"corrected raw bean name balances/ledger",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing raw bean name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一 Excel 行号",
		"旧生豆余额回退",
		"新生豆余额",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing raw bean name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION",
		"DEV-266-CUSTOMER-PROCESSING-CUSTODY-RAW-BEAN-NAME-REIMPORT-CORRECTION",
		"TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance",
		"CUSTOMER_PROCESSING_CUSTODY_RAW_BEAN_NAME_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing raw bean name correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing raw bean name correction marker %q", want)
		}
	}
}
