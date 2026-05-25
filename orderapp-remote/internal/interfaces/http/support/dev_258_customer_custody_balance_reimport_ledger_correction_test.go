package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerCustodyBalanceReimportLedgerCorrectionEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	repositoryTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	requirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	acceptanceTests := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"upsertCustodyBalanceAdjustmentLedgerTx",
		"targetG-baseG",
		"targetUnits-baseUnits",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("customer fulfillment repository missing custody balance correction marker %q", want)
		}
	}
	for _, want := range []string{
		"TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta",
		"raw bean corrected balance/ledger sum",
		"packaging corrected balance/ledger sum",
	} {
		if !strings.Contains(repositoryTest, want) {
			t.Fatalf("customer fulfillment repository test missing custody balance correction marker %q", want)
		}
	}
	for _, want := range []string{
		"同一外部生豆余额或耗材余额重传并更正盘点数",
		"库存余额和台账汇总应保持一致",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("customer fulfillment manual missing custody balance correction marker %q", want)
		}
	}
	for _, want := range []string{
		"修正版代加工库存余额表",
		"库存余额与台账汇总一致",
	} {
		if !strings.Contains(requirements, want) {
			t.Fatalf("REQUIREMENTS.md missing custody balance correction marker %q", want)
		}
	}
	for _, want := range []string{
		"更正盘点数",
		"库存台账 delta 汇总也等于最新余额",
	} {
		if !strings.Contains(acceptanceTests, want) {
			t.Fatalf("ACCEPTANCE_TESTS.md missing custody balance correction marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION",
		"DEV-258-CUSTOMER-CUSTODY-BALANCE-REIMPORT-LEDGER-CORRECTION",
		"TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta",
		"CUSTOMER_CUSTODY_BALANCE_REIMPORT_LEDGER_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing custody balance correction marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing custody balance correction marker %q", want)
		}
	}
}
