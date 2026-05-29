package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceTaxLedgerDuplicateInvoiceGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"ensureTaxLedgerInvoiceUniqueTx",
		"pg_advisory_xact_lock(hashtext($1)::bigint)",
		"lower(invoice_no)=lower($2)",
		"tax ledger invoice already exists",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("finance repository missing tax ledger duplicate invoice guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger",
		"PINV-DUP-001",
		"duplicate invoice ledger rows",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing tax ledger duplicate invoice guard marker %q", want)
		}
	}
	for _, want := range []string{
		"同类型非空发票号或凭证号不能重复录入",
		"tax ledger invoice already exists",
		"finance_tax_ledger",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing tax ledger duplicate invoice guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD",
		"DEV-251-FINANCE-TAX-LEDGER-DUPLICATE-INVOICE-GUARD",
		"TestFinanceTaxLedgerAPIRejectsDuplicateInvoiceNoWithoutWritingLedger",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing tax ledger duplicate invoice guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing tax ledger duplicate invoice guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_TAX_LEDGER_DUPLICATE_UI_CLICK_OK",
		"PINV-UI-DUP-001",
		"save_2026_05_then_duplicate_2026_06",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing tax ledger duplicate invoice UI marker %q", want)
		}
	}
}
