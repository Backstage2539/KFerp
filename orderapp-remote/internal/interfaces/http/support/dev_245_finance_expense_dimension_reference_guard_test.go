package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFinanceExpenseDimensionReferenceGuardEvidenceExists(t *testing.T) {
	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "finance", "repository.go")))
	apiTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "finance", "finance_api_test.go")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_FINANCE.md")))
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"validateExpenseDimensionRefs",
		"ensureExpenseDimensionRefExists",
		"finance dimension %s not found",
		`"orders", cmd.OrderID, "order"`,
		`"customers", cmd.CustomerID, "customer"`,
		`"products", cmd.ProductID, "product"`,
	} {
		if !strings.Contains(repository, want) {
			t.Fatalf("finance repository missing dimension reference guard marker %q", want)
		}
	}
	for _, want := range []string{
		"TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense",
		"finance dimension order not found",
		"finance dimension customer not found",
		"finance dimension product not found",
		"missing dimension expense rows",
	} {
		if !strings.Contains(apiTest, want) {
			t.Fatalf("finance API test missing dimension reference guard marker %q", want)
		}
	}
	for _, want := range []string{
		"订单、客户、商品维度必须能在系统中找到",
		"finance dimension order not found",
		"finance_expenses",
	} {
		if !strings.Contains(manual, want) {
			t.Fatalf("finance manual missing dimension reference guard marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-245-FINANCE-EXPENSE-DIMENSION-REFERENCE-GUARD",
		"DEV-245-FINANCE-EXPENSE-DIMENSION-REFERENCE-GUARD",
		"TestFinanceExpenseAPIRejectsMissingDimensionReferencesWithoutWritingExpense",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing dimension reference guard marker %q", want)
		}
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing dimension reference guard marker %q", want)
		}
	}
	for _, want := range []string{
		"FINANCE_EXPENSE_DIMENSION_UI_CLICK_OK",
		"missing_order_then_customer_mismatch_then_product_mismatch",
		"finance dimension order not found",
		"UI维度缺失",
		"费用管理页三次填写并点击保存",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing dimension reference UI marker %q", want)
		}
	}
}
