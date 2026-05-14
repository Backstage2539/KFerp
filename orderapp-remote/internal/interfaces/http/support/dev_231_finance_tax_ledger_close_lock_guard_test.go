package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceTaxLedgerCloseLockGuardEvidenceExists(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service.go")))
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"CanEditSourceDocument(settings.Settings, status)",
		"month is closed by strong lock",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("finance service missing tax ledger close lock guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateTaxLedgerRespectsStrongLockForClosedMonth",
		"domain.ClosingModeLightConfirmation",
		"closed strong-lock month should reject new tax ledger source document",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("finance service test missing tax ledger close lock guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceTaxLedgerAPIReturnsBadRequestWhenServiceRejectsClosedMonth",
		"month is closed by strong lock",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing tax ledger close lock guard marker %q", want)
		}
	}
	for _, want := range []string{
		"强锁账月份继续新增同月费用或票税台账会被后端拒绝",
		"默认强锁账",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing tax ledger close lock guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-231-FINANCE-TAX-LEDGER-CLOSE-LOCK-GUARD",
		"DEV-231-FINANCE-TAX-LEDGER-CLOSE-LOCK-GUARD",
		"TestCreateTaxLedgerRespectsStrongLockForClosedMonth",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing tax ledger close lock guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing tax ledger close lock guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_TAX_LEDGER_CLOSE_LOCK_UI_CLICK_OK",
		"LOCK-UI-001",
		"close_2026_05_then_tax_ledger_save",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing tax ledger close lock UI marker %q", want)
		}
	}
}
