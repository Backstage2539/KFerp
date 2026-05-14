package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceExpenseOrderProductMatchGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"ensureExpenseOrderProductMatch",
		"FROM %s.order_items",
		"finance dimension product does not match order",
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("finance repository missing order/product match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense",
		"finance dimension product does not match order",
		"mismatched product dimension expense rows",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing order/product match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"商品维度必须属于该订单明细",
		"finance dimension product does not match order",
		"finance_expenses",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing order/product match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-247-FINANCE-EXPENSE-ORDER-PRODUCT-MATCH-GUARD",
		"DEV-247-FINANCE-EXPENSE-ORDER-PRODUCT-MATCH-GUARD",
		"TestFinanceExpenseAPIRejectsOrderProductMismatchWithoutWritingExpense",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing order/product match guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing order/product match guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK",
		"missing_order_then_customer_mismatch_then_product_mismatch",
		"finance dimension product does not match order",
		"UI商品不一致",
		"order_id=256/product_id=10",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing order/product match UI marker %q", want)
		}
	}
}
