package finance

import (
	"context"
	"fmt"
	appfinance "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
		"employee_id BIGINT NULL",
		"order_id BIGINT NOT NULL DEFAULT 0",
		"customer_id BIGINT NOT NULL DEFAULT 0",
		"product_id BIGINT NOT NULL DEFAULT 0",
		"batch_no TEXT NOT NULL DEFAULT ''",
		"dimension_note TEXT NOT NULL DEFAULT ''",
		"REFERENCES %[1]s.company_employees(id)",
		"allocation TEXT NOT NULL DEFAULT 'period_expense'",
		"finance_expenses_employee_idx",
		"finance_tax_ledger",
		"invoice_no TEXT NOT NULL DEFAULT ''",
		"finance_monthly_reports",
		"snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"finance_adjustments",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("finance schema missing %q", want)
		}
	}
}

func TestFinanceSchemaBackfillsExpenseEmployeeColumnBeforeEmployeeIndex(t *testing.T) {
	b, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	addColumn := strings.Index(src, "ALTER TABLE %s.finance_expenses ADD COLUMN IF NOT EXISTS employee_id")
	createIndex := strings.Index(src, "CREATE INDEX IF NOT EXISTS finance_expenses_employee_idx")
	if addColumn < 0 || createIndex < 0 {
		t.Fatalf("employee schema migration pieces missing: addColumn=%d createIndex=%d", addColumn, createIndex)
	}
	if addColumn > createIndex {
		t.Fatalf("employee_id column must be added before finance_expenses_employee_idx is created")
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
		"financeOrderRevenueSQL",
		"COALESCE(%[1]sgrand_total,0) <> 0",
		"COALESCE(%[1]sdiscount_amount,0) <> 0",
		"COALESCE(%[1]sshipping_amount,0) <> 0",
		"COALESCE(is_void,false)=false",
		"FROM %s.production_batch_costs",
		"FROM %s.finance_expenses",
		"allocation='main_cost'",
		"allocation='period_expense'",
		"LEFT JOIN %s.company_employees",
		"FinanceSourceDetails",
		"source_type",
		"order_revenue",
		"production_cost",
		"finance_tax_ledger",
		"CreateTaxLedgerEntry",
		"WHERE fe.month=$1",
		"fe.employee_id=$%d",
		"fe.customer_id=$%d",
		"ListExpenseEmployees",
		"FROM %s.company_employees",
		"FROM %s.finance_adjustments",
		"postgresinfra.AuditInsertTx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("finance repository missing %q", want)
		}
	}
}

func TestMonthlySourceTotalsUsesLegacyTotalAmountWhenGrandTotalWasDefaultZero(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()

	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id, name, company_name) VALUES (1, '历史客户', ''), (2, '折扣客户', ''), (3, '正常客户', '');
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void)
		VALUES (1, 'SO-LEGACY-TOTAL', '2026-05-02', 1, 120, 0, false);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, discount_amount, grand_total, is_void)
		VALUES (2, 'SO-FULL-DISCOUNT', '2026-05-03', 2, 88, 88, 0, false);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void)
		VALUES (3, 'SO-GRAND-TOTAL', '2026-05-04', 3, 100, 110, false);
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, is_void)
		VALUES (4, 'SO-VOID', '2026-05-05', 3, 999, 999, true);
	`, schema, schema, schema, schema, schema))

	repo := NewRepository(pool, schema)
	totals, _, err := repo.MonthlySourceTotals(ctx, appfinance.ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatalf("MonthlySourceTotals: %v", err)
	}
	if totals.RevenueTaxInclusive != 230 {
		t.Fatalf("RevenueTaxInclusive = %.2f, want 230.00", totals.RevenueTaxInclusive)
	}

	details, err := repo.FinanceSourceDetails(ctx, appfinance.ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatalf("FinanceSourceDetails: %v", err)
	}
	got := map[string]float64{}
	for _, row := range details {
		if row.Section == "revenue" {
			got[row.Name] = float64(row.Amount)
		}
	}
	if got["SO-LEGACY-TOTAL"] != 120 {
		t.Fatalf("legacy order detail amount = %.2f, want 120.00; details=%#v", got["SO-LEGACY-TOTAL"], got)
	}
	if got["SO-FULL-DISCOUNT"] != 0 {
		t.Fatalf("full discount detail amount = %.2f, want 0.00; details=%#v", got["SO-FULL-DISCOUNT"], got)
	}
	if _, ok := got["SO-VOID"]; ok {
		t.Fatalf("void order should not appear in revenue details: %#v", got)
	}
}

func TestFinanceSourceDetailsIncludesOrderPaymentMethod(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()

	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.customers(id, name, company_name) VALUES (1, '门店客户', '');
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, grand_total, payment_method, is_void)
		VALUES (11, 'SO-PAID-BANK', '2026-05-15', 1, 680, 680, '银行转账', false);
	`, schema, schema))

	repo := NewRepository(pool, schema)
	details, err := repo.FinanceSourceDetails(ctx, appfinance.ReportFilter{Month: "2026-05"})
	if err != nil {
		t.Fatalf("FinanceSourceDetails: %v", err)
	}
	for _, row := range details {
		if row.SourceType == "order_revenue" && row.SourceID == 11 {
			if row.PaymentMethod != "银行转账" {
				t.Fatalf("payment method = %q, want 银行转账; row=%#v", row.PaymentMethod, row)
			}
			return
		}
	}
	t.Fatalf("missing SO-PAID-BANK revenue detail: %#v", details)
}

func TestCloseMonthKeepsAdjustedStatusAfterAdjustment(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	svc := appfinance.NewService(NewRepository(pool, schema))

	if _, err := svc.CloseMonth(ctx, appfinance.CloseMonthCommand{Month: "2026-05", Actor: "Van"}); err != nil {
		t.Fatalf("first CloseMonth: %v", err)
	}
	if _, err := svc.CreateAdjustment(ctx, appfinance.CreateAdjustmentCommand{
		Month:  "2026-05",
		Type:   domain.AdjustmentExpense,
		Amount: 100,
		Reason: "补记费用",
		Actor:  "Van",
	}); err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
	report, err := svc.CloseMonth(ctx, appfinance.CloseMonthCommand{Month: "2026-05", Actor: "Van"})
	if err != nil {
		t.Fatalf("repeat CloseMonth: %v", err)
	}
	if report.Status != domain.MonthStatusAdjusted {
		t.Fatalf("repeat close report status = %q, want adjusted", report.Status)
	}

	var status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.finance_monthly_reports WHERE month='2026-05'`, schema)).Scan(&status); err != nil {
		t.Fatalf("query report status: %v", err)
	}
	if status != domain.MonthStatusAdjusted {
		t.Fatalf("persisted report status = %q, want adjusted", status)
	}
}

func newFinancePostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for finance postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_finance_%d", time.Now().UnixNano())
	mustExecFinanceSQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})
	mustExecFinanceSQL(t, ctx, pool, financePostgresTestDDL(schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return pool, schema
}

func financePostgresTestDDL(schema string) string {
	return fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	company_name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.orders (
	id BIGINT PRIMARY KEY,
	order_no TEXT NOT NULL DEFAULT '',
	order_date DATE NOT NULL,
	customer_id BIGINT NOT NULL DEFAULT 0,
	total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
	payment_method TEXT NOT NULL DEFAULT '',
	grand_total NUMERIC(12,2) NOT NULL DEFAULT 0,
	is_void BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE %s.production_batch_costs (
	id BIGSERIAL PRIMARY KEY,
	product_name TEXT NOT NULL DEFAULT '',
	total_cost NUMERIC(12,2) NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.company_employees (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT NULL,
	action TEXT NOT NULL DEFAULT '',
	field TEXT NULL,
	old_value TEXT NULL,
	new_value TEXT NULL,
	meta JSONB NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
	`, schema, schema, schema, schema, schema)
}

func mustExecFinanceSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v\n%s", err, sql)
	}
}
