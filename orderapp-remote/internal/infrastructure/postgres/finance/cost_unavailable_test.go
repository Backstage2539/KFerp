package finance

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/finance"
	"strings"
	"testing"
	"time"
)

func TestMonthlyReportUnknownCostAndHistoricalSnapshot(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.production_batch_costs ADD COLUMN cost_unavailable boolean NOT NULL DEFAULT false; ALTER TABLE %[1]s.production_batch_costs ADD COLUMN cost_updated_at timestamptz;
 INSERT INTO %[1]s.production_batch_costs(product_name,total_cost,created_at,cost_unavailable) VALUES('unknown',12,'2026-11-03',true),('known',30,'2026-12-03',false);
 CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec text,qty numeric,item_name text,unit text);
 INSERT INTO %[1]s.orders(id,order_date,customer_id) VALUES(1,'2026-11-03',1);
 INSERT INTO %[1]s.order_items VALUES(1,1,42,1,1,'',1,'未核算订单商品','件');`, schema))
	repo := NewRepository(pool, schema)
	svc := app.NewService(repo)
	report, err := svc.DraftReport(ctx, app.ReportFilter{Month: "2026-11"})
	if err != nil || !report.CostUnavailable {
		t.Fatalf("unknown %+v %v", report, err)
	}
	if _, err = svc.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-11", Actor: "test"}); err == nil || !strings.Contains(err.Error(), "成本待核算") {
		t.Fatalf("unknown close %v", err)
	}
	known, err := svc.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-12", Actor: "test"})
	if err != nil || known.CostUnavailable || known.Status != "closed" {
		t.Fatalf("known close %+v %v", known, err)
	}
	historical := domain.MonthlyReport{Month: "2026-11", Status: "closed", MainBusinessCost: 777, GrossProfit: 123}
	raw, _ := json.Marshal(historical)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_monthly_reports(month,status,snapshot_json) VALUES('2026-11','closed',$1::jsonb)`, schema), string(raw)); err != nil {
		t.Fatal(err)
	}
	got, err := svc.DraftReport(ctx, app.ReportFilter{Month: "2026-11"})
	if err != nil || got.CostUnavailable || got.MainBusinessCost != 777 || got.GrossProfit != 123 {
		t.Fatalf("historical %+v %v", got, err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.finance_monthly_reports SET closed_at='2026-11-04' WHERE month='2026-11';UPDATE %[1]s.production_batch_costs SET cost_updated_at='2026-11-05' WHERE product_name='unknown'`, schema))
	dashboard, err := svc.Dashboard(ctx, "2026-11")
	if err != nil || dashboard.Report.MainBusinessCost != 777 {
		t.Fatalf("late report %+v %v", dashboard.Report, err)
	}
	found := false
	for _, exception := range dashboard.Exceptions {
		if exception.Code == "production_cost_after_close" {
			found = true
		}
	}
	if !found {
		t.Fatal("late cost change was not reported")
	}
}

func TestMonthCloseSerializesWithCostWritesAndWarnsForLateCost(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.production_batch_costs ADD COLUMN cost_unavailable boolean NOT NULL DEFAULT false;ALTER TABLE %[1]s.production_batch_costs ADD COLUMN cost_updated_at timestamptz;
 CREATE TABLE %[1]s.order_items(id bigint PRIMARY KEY,order_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec text,qty numeric,item_name text,unit text)`, schema))
	svc := app.NewService(NewRepository(pool, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_batch_costs(product_name,total_cost,created_at,cost_unavailable,cost_updated_at) VALUES('pending',5,'2026-11-03',true,now());
 INSERT INTO %[1]s.orders(id,order_date,customer_id) VALUES(1,'2026-11-03',1);
 INSERT INTO %[1]s.order_items VALUES(1,1,42,1,1,'',1,'未核算订单商品','件')`, schema)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, e := svc.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-11", Actor: "test"}); done <- e }()
	select {
	case err = <-done:
		t.Fatalf("close did not wait for production transaction: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil || !strings.Contains(err.Error(), "成本待核算") {
			t.Fatalf("close err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("close did not finish")
	}
	closed, err := svc.CloseMonth(ctx, app.CloseMonthCommand{Month: "2026-12", Actor: "test"})
	if err != nil || closed.Status != "closed" {
		t.Fatalf("close known: %+v %v", closed, err)
	}
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx2.Rollback(ctx)
	if _, err = tx2.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, schema+":finance-period"); err != nil {
		t.Fatal(err)
	}
	if _, err = tx2.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_batch_costs(product_name,total_cost,created_at,cost_unavailable,cost_updated_at) VALUES('late',33,'2026-12-03',true,now()+interval '1 second')`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = tx2.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	dashboard, err := svc.Dashboard(ctx, "2026-12")
	if err != nil || dashboard.Report.CostUnavailable || dashboard.Report.MainBusinessCost != closed.MainBusinessCost {
		t.Fatalf("closed snapshot changed %+v %v", dashboard.Report, err)
	}
	found := false
	for _, ex := range dashboard.Exceptions {
		if ex.Code == "production_cost_after_close" {
			found = true
		}
	}
	if !found {
		t.Fatal("late write warning missing")
	}
}

func TestPartialProductionEventsBlockEachMonthWithoutDuplicatingBatchCost(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.production_batch_costs ADD COLUMN IF NOT EXISTS cost_unavailable boolean NOT NULL DEFAULT false; ALTER TABLE %[1]s.production_batch_costs ADD COLUMN IF NOT EXISTS cost_updated_at timestamptz;
 ALTER TABLE %[1]s.production_batch_costs ADD COLUMN running_item_id bigint;
 CREATE TABLE %[1]s.work_orders(id bigint primary key,running_item_id bigint not null);
 CREATE TABLE %[1]s.production_operation_events(id bigserial primary key,work_order_id bigint not null,kind text not null,status text not null,payload jsonb not null,created_at timestamptz not null);
 INSERT INTO %[1]s.work_orders(id,running_item_id) VALUES(123,456);
 INSERT INTO %[1]s.production_operation_events(work_order_id,kind,status,payload,created_at) VALUES(123,'complete','posted','{"cost_unavailable":true}','2026-11-06'),(123,'complete','posted','{"cost_unavailable":true}','2026-11-07'),(123,'complete','posting','{"cost_unavailable":true}','2026-10-07');`, schema))
	svc := app.NewService(NewRepository(pool, schema))
	first, err := svc.DraftReport(ctx, app.ReportFilter{Month: "2026-11"})
	if err != nil || first.CostUnavailable {
		t.Fatalf("unsold partial production should not affect cost: %+v %v", first, err)
	}
	filtered, _, err := NewRepository(pool, schema).MonthlySourceTotals(ctx, app.ReportFilter{Month: "2026-11", CustomerID: 42})
	if err != nil || filtered.CostUnavailable || filtered.MainBusinessCost != 0 {
		t.Fatalf("unsold partial production should not affect customer cost: %+v %v", filtered, err)
	}
	old, err := svc.DraftReport(ctx, app.ReportFilter{Month: "2026-10"})
	if err != nil || old.CostUnavailable {
		t.Fatalf("unposted event affected report: %+v %v", old, err)
	}
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_batch_costs(running_item_id,product_name,total_cost,created_at,cost_unavailable) VALUES(456,'finished',42,'2026-12-01',true); INSERT INTO %[1]s.production_operation_events(work_order_id,kind,status,payload,created_at) VALUES(123,'complete','posted','{"cost_unavailable":true}','2026-12-01');`, schema))
	first, err = svc.DraftReport(ctx, app.ReportFilter{Month: "2026-11"})
	if err != nil || first.CostUnavailable {
		t.Fatalf("unsold later output affected earlier month: %+v %v", first, err)
	}
	last, err := svc.DraftReport(ctx, app.ReportFilter{Month: "2026-12"})
	if err != nil || last.CostUnavailable || last.MainBusinessCost != 0 {
		t.Fatalf("unsold output must not be expensed: %+v %v", last, err)
	}
	_, exceptions, err := NewRepository(pool, schema).MonthlySourceTotals(ctx, app.ReportFilter{Month: "2026-12"})
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range exceptions {
		if ex.Code == "production_cost_unavailable" && ex.Count != 1 {
			t.Fatalf("same work order double counted: %+v", ex)
		}
	}
	snapshot, _ := json.Marshal(domain.MonthlyReport{Month: "2026-11", Status: "closed", MainBusinessCost: 777})
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_monthly_reports(month,status,snapshot_json,closed_at) VALUES('2026-11','closed',$1::jsonb,'2026-11-05')`, schema), string(snapshot)); err != nil {
		t.Fatal(err)
	}
	dashboard, err := svc.Dashboard(ctx, "2026-11")
	if err != nil || dashboard.Report.CostUnavailable || dashboard.Report.MainBusinessCost != 777 {
		t.Fatalf("closed snapshot changed: %+v %v", dashboard.Report, err)
	}
	foundLate := false
	for _, ex := range dashboard.Exceptions {
		if ex.Code == "production_cost_after_close" {
			foundLate = true
		}
	}
	if !foundLate {
		t.Fatal("partial event after close requires a late cost warning")
	}
}
