package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceExpenseInactiveEmployeeGuardEvidenceExists(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service.go")))
	serviceTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "finance", "service_test.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"ensureActiveExpenseEmployee",
		"employee inactive",
		"employee not found",
		"ListExpenseEmployees",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("finance service missing inactive employee guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestCreateExpenseRejectsInactiveEmployee",
		"employee inactive",
		"len(repo.expenses) != 0",
	} {
		if !strings.Contains(serviceTest, want) {
			t.Fatalf("finance service test missing inactive employee guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense",
		"employee inactive",
		"finance_expenses",
		"active BOOLEAN NOT NULL DEFAULT true",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing inactive employee guard marker %q", want)
		}
	}
	for _, want := range []string{
		"停用员工不能作为新费用经办人",
		"employee inactive",
		"finance_expenses",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing inactive employee guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-244-FINANCE-EXPENSE-INACTIVE-EMPLOYEE-GUARD",
		"DEV-244-FINANCE-EXPENSE-INACTIVE-EMPLOYEE-GUARD",
		"TestFinanceExpenseAPIRejectsInactiveEmployeeWithoutWritingExpense",
		"FINANCE_EXPENSE_INACTIVE_EMPLOYEE_UI_CLICK_OK",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing inactive employee guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing inactive employee guard marker %q", want)
		}
	}
}
