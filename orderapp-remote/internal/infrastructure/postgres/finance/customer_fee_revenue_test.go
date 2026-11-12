package finance

import (
	"context"
	"fmt"
	app "orderapp/internal/application/finance"
	"testing"
)

func TestIndependentFeeRevenueAndCreditUseTheirOwnPostingPeriods(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.customers(id,name,company_name)VALUES(1,'fee customer',''),(2,'other','');
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,total_amount,grand_total,is_void)VALUES(1,'FEE-ORDER','2026-10-01',1,90,100,false);
 CREATE TABLE %[1]s.customer_fee_items(id bigint PRIMARY KEY,customer_id bigint,source_type text,source_id bigint,fee_type text,amount numeric,status text,occurred_at timestamptz);
 INSERT INTO %[1]s.customer_fee_items VALUES(1,1,'order',1,'direct_ship_service',10,'settled','2026-10-01'),(2,1,'order',1,'shipping',10,'settled','2026-10-01'),(3,1,'order',1,'product',90,'settled','2026-10-01'),(4,1,'manual',0,'processing',99,'reversed','2026-10-01'),(5,2,'manual',0,'storage',3,'unsettled','2026-10-01');
 CREATE TABLE %[1]s.order_resolution_requests(id bigint PRIMARY KEY,applied boolean,command_json jsonb);
 CREATE TABLE %[1]s.order_resolution_fee_events(request_id bigint,source_id bigint,credit_cents bigint);
 INSERT INTO %[1]s.order_resolution_requests VALUES(1,true,'{"posting_date":"2026-11-07"}'),(2,false,'{"posting_date":"2026-11-07"}');
 INSERT INTO %[1]s.order_resolution_fee_events VALUES(1,1,500),(1,-1,1000),(2,1,500);
 `, schema))
	r := NewRepository(pool, schema)
	for _, sample := range []struct {
		month    string
		customer int64
		want     float64
	}{{"2026-10", 1, 110}, {"2026-11", 1, -5}, {"2026-10", 0, 113}, {"2026-11", 2, 0}} {
		filter := app.ReportFilter{Month: sample.month, CustomerID: sample.customer}
		totals, _, err := r.MonthlySourceTotals(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		if float64(totals.RevenueTaxInclusive) != sample.want {
			t.Fatalf("%s customer%d revenue %v want %v", sample.month, sample.customer, totals.RevenueTaxInclusive, sample.want)
		}
		details, err := r.FinanceSourceDetails(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		sum := float64(0)
		for _, row := range details {
			if row.Section == "revenue" {
				sum += float64(row.Amount)
			}
		}
		if sum != sample.want {
			t.Fatalf("details sum %v want %v: %+v", sum, sample.want, details)
		}
	}
}

func TestMonthlySourceTotalsIgnoresAppliedRequestsWithBlankPostingDates(t *testing.T) {
	pool, schema := newFinancePostgresTestDB(t)
	ctx := context.Background()
	mustExecFinanceSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.customers(id,name,company_name)VALUES(1,'fee customer','');
 CREATE TABLE %[1]s.customer_fee_items(id bigint PRIMARY KEY,customer_id bigint,source_type text,source_id bigint,fee_type text,amount numeric,status text,occurred_at timestamptz);
 INSERT INTO %[1]s.customer_fee_items VALUES(1,1,'manual',0,'processing',10,'reversed','2026-11-07');
 CREATE TABLE %[1]s.order_resolution_requests(id bigint PRIMARY KEY,applied boolean,command_json jsonb);
 INSERT INTO %[1]s.order_resolution_requests VALUES(1,true,'{"posting_date":""}');
 CREATE TABLE %[1]s.order_resolution_fee_events(request_id bigint,source_id bigint,credit_cents bigint);
 INSERT INTO %[1]s.order_resolution_fee_events VALUES(1,1,500);
 `, schema))

	repo := NewRepository(pool, schema)
	totals, _, err := repo.MonthlySourceTotals(ctx, app.ReportFilter{Month: "2026-11"})
	if err != nil {
		t.Fatalf("MonthlySourceTotals with a blank posting_date: %v", err)
	}
	if totals.RevenueTaxInclusive != 0 {
		t.Fatalf("RevenueTaxInclusive = %.2f, want 0.00", totals.RevenueTaxInclusive)
	}
}
