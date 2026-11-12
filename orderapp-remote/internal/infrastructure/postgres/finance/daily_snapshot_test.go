package finance

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/finance"
)

func TestClosingFreezesDailyLedgerAndLegacySnapshotIsUnavailable(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.finance_expenses(expense_date,month,category,amount,allocation) VALUES('2026-05-02','2026-05','房租',50,'period_expense');`, schema))
	repo := NewRepository(pool, schema)
	service := app.NewService(repo)
	if _, err := service.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-05", Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	// Simulate external drift in an isolated test schema after closing.
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.finance_expenses DISABLE TRIGGER USER; UPDATE %[1]s.finance_expenses SET amount=500 WHERE month='2026-05'; ALTER TABLE %[1]s.finance_expenses ENABLE TRIGGER USER; INSERT INTO %[1]s.finance_monthly_reports(month,status,snapshot_json) VALUES('2026-04','closed','{"month":"2026-04","status":"closed"}');`, schema))
	ledger, err := service.DailyLedger(ctx, app.DailyFilter{Month: "2026-05", Date: "2026-05-02"})
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.Available || ledger.Total != 1 || ledger.Rows[0].Amount != 50 {
		t.Fatalf("ledger=%+v", ledger)
	}
	if _, err = service.CreateAdjustment(ctx, app.CreateAdjustmentCommand{Month: "2026-05", Type: "expense", Amount: 10, Reason: "补充费用", Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	dashboard, err := service.Dashboard(ctx, "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	if !dashboard.Daily.HasMonthlyAdjustments || dashboard.Report.AdjustedPeriodExpenses != 60 {
		t.Fatalf("dashboard=%+v", dashboard)
	}
	repeated, err := service.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-05", Actor: "tester"})
	if err != nil || repeated.AdjustedPeriodExpenses != 60 || repeated.Status != "adjusted" {
		t.Fatalf("repeat close must include existing adjustments: report=%+v error=%v", repeated, err)
	}
	// Rebuild is the existing approved correction path and must recalculate
	// the monthly snapshot and daily snapshot together in its transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RebuildClosedMonthTx(ctx, tx, "2026-05", "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	dashboard, err = service.Dashboard(ctx, "2026-05")
	if err != nil {
		t.Fatal(err)
	}
	ledger, err = service.DailyLedger(ctx, app.DailyFilter{Month: "2026-05", Date: "2026-05-02"})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Report.AdjustedPeriodExpenses != 510 || ledger.Rows[0].Amount != 500 {
		t.Fatalf("rebuilt report=%+v ledger=%+v", dashboard.Report, ledger)
	}
	old, err := service.DailyLedger(ctx, app.DailyFilter{Month: "2026-04"})
	if err != nil || old.Available || old.Total != 0 {
		t.Fatalf("legacy=%+v error=%v", old, err)
	}
}

func TestCustomerReportsUseScopedFrozenSnapshots(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户一'),(2,'客户二');
 INSERT INTO %[1]s.finance_expenses(expense_date,month,category,amount,allocation,customer_id)
 VALUES('2026-05-02','2026-05','客户一费用',10,'period_expense',1),('2026-05-03','2026-05','客户二费用',20,'period_expense',2);`, schema))
	repo := NewRepository(pool, schema)
	service := app.NewService(repo)
	if _, err := service.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-05", Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(3,'结账后新增客户'); ALTER TABLE %[1]s.finance_expenses DISABLE TRIGGER USER; UPDATE %[1]s.finance_expenses SET amount=amount*10 WHERE month='2026-05'; ALTER TABLE %[1]s.finance_expenses ENABLE TRIGGER USER;`, schema))
	if _, err := service.CreateAdjustment(ctx, app.CreateAdjustmentCommand{Month: "2026-05", Type: "expense", Amount: 8, Reason: "全局调整", Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id       int64
		expected float64
		name     string
	}{{1, 10, "客户一费用"}, {2, 20, "客户二费用"}} {
		filter := app.ReportFilter{Month: "2026-05", CustomerID: tc.id}
		report, err := service.DraftReport(ctx, filter)
		if err != nil || float64(report.PeriodExpenses) != tc.expected || float64(report.AdjustedPeriodExpenses) != tc.expected {
			t.Fatalf("customer %d frozen report=%+v err=%v", tc.id, report, err)
		}
		details, err := service.ReportDrilldown(ctx, filter)
		if err != nil || len(details.Sections) != 1 || len(details.Sections[0].Rows) != 1 || details.Sections[0].Rows[0].Name != tc.name {
			t.Fatalf("customer %d frozen details=%+v err=%v", tc.id, details, err)
		}
		ledger, err := service.DailyLedger(ctx, app.DailyFilter{Month: "2026-05", CustomerID: tc.id})
		if err != nil || !ledger.Available || ledger.Total != 1 || ledger.Rows[0].Name != tc.name {
			t.Fatalf("customer %d frozen ledger=%+v err=%v", tc.id, ledger, err)
		}
	}
	if _, err := service.DraftReport(ctx, app.ReportFilter{Month: "2026-05", CustomerID: 3}); err == nil {
		t.Fatal("a customer absent from the closing snapshot must not be assigned a fabricated zero report")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RebuildClosedMonthTx(ctx, tx, "2026-05", "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	report, err := service.DraftReport(ctx, app.ReportFilter{Month: "2026-05", CustomerID: 1})
	if err != nil || report.PeriodExpenses != 100 {
		t.Fatalf("rebuild must atomically update customer snapshot: %+v %v", report, err)
	}
}
