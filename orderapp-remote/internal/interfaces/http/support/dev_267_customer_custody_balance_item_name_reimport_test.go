package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCustodyBalanceItemNameReimportCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"target_type='customer_custody_balance'",
		"bal.item_type=$6",
		"newDeltaG := targetG - currentG",
		"return r.setCustodyBalanceTx",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing balance item-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportCorrectedCustodyBalanceItemNameMovesLedgerAndBalance",
		"corrected raw balance item",
		"corrected packaging balance item",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing balance item-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"库存余额表",
		"同一 Excel 行号",
		"旧物料余额回退",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing balance item-name correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION",
		"DEV-267-CUSTOMER-CUSTODY-BALANCE-ITEM-NAME-REIMPORT-CORRECTION",
		"TestApplyProcessingImportReimportCorrectedCustodyBalanceItemNameMovesLedgerAndBalance",
		"CUSTOMER_CUSTODY_BALANCE_ITEM_NAME_REIMPORT_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing balance item-name correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing balance item-name correction marker %q", want)
		}
	}
}
