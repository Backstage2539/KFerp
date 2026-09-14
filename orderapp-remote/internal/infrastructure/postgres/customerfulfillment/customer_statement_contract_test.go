package customerfulfillment

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerStatementsReuseOrdersFeesAndKeepReconciliationSeparateFromPayment(t *testing.T) {
	schema, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	account, err := os.ReadFile("account.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"customer_statement_reconciliations", "customer_statement_disputes"} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("schema missing %q", want)
		}
	}
	for _, want := range []string{"ConfirmCustomerStatement", "CreateCustomerStatementDispute", "ReplyCustomerStatementDispute", "IncludedInOrder"} {
		if !strings.Contains(string(account), want) {
			t.Fatalf("account source missing %q", want)
		}
	}
}
