package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceExpenseOrderCustomerMatchGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"ensureExpenseOrderCustomerMatch",
		"SELECT customer_id",
		"finance dimension customer does not match order",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("finance repository missing order/customer match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense",
		"finance dimension customer does not match order",
		"mismatched dimension expense rows",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing order/customer match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"客户维度必须与订单归属客户一致",
		"finance dimension customer does not match order",
		"finance_expenses",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing order/customer match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-246-FINANCE-EXPENSE-ORDER-CUSTOMER-MATCH-GUARD",
		"DEV-246-FINANCE-EXPENSE-ORDER-CUSTOMER-MATCH-GUARD",
		"TestFinanceExpenseAPIRejectsOrderCustomerMismatchWithoutWritingExpense",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing order/customer match guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing order/customer match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK",
		"missing_order_then_customer_mismatch_then_product_mismatch",
		"finance dimension customer does not match order",
		"UI客户不一致",
		"order_id=256/customer_id=19",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing order/customer match UI marker %q", want)
		}
	}
}
