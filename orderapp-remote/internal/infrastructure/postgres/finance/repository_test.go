package finance

import (
	"os"
	"strings"
	"testing"
)

func TestFinanceSchemaDefinesSettingsExpensesReportsAndAdjustments(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"finance_settings",
		"company_type TEXT NOT NULL DEFAULT 'coffee_roaster'",
		"taxpayer_type TEXT NOT NULL DEFAULT 'small_scale'",
		"closing_mode TEXT NOT NULL DEFAULT 'strong_lock'",
		"close_mode_admin_users JSONB NOT NULL DEFAULT '[]'::jsonb",
		"finance_expenses",
		"allocation TEXT NOT NULL DEFAULT 'period_expense'",
		"finance_monthly_reports",
		"snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"finance_adjustments",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("finance schema missing %q", want)
		}
	}
}

func TestFinanceRepositoryAggregatesOrdersCostsExpensesAndAdjustments(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"FROM %s.orders",
		"COALESCE(grand_total,total_amount,0)",
		"COALESCE(is_void,false)=false",
		"FROM %s.production_batch_costs",
		"FROM %s.finance_expenses",
		"allocation='main_cost'",
		"allocation='period_expense'",
		"FROM %s.finance_adjustments",
		"postgresinfra.AuditInsertTx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("finance repository missing %q", want)
		}
	}
}
