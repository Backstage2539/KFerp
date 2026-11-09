package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	"testing"
)

func TestIssuedBillsFilterByShanghaiIssueDateAndSummarizeBeforePaging(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('出账日期测试') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		no     string
		issued string
		amount int
	}{{"BILL-8", "2026-11-08 23:30:00+08", 100}, {"BILL-9", "2026-11-09 00:30:00+08", 200}} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_batches(customer_id,settlement_no,period_from,period_to,status,total_amount,confirmed_at,created_at) VALUES($1,$2,'2026-09-01','2026-09-30','confirmed',$3,$4,$4)`, schema), customerID, row.no, row.amount, row.issued); err != nil {
			t.Fatal(err)
		}
	}
	all, err := repo.ListIssuedBills(ctx, app.BillingQuery{CustomerID: customerID, Page: 1, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 || len(all.Rows) != 1 || all.Summary.Count != 2 || all.Summary.TotalAmountCents != 30000 {
		t.Fatalf("all=%+v", all)
	}
	day, err := repo.ListIssuedBills(ctx, app.BillingQuery{CustomerID: customerID, DateFrom: "2026-11-09", DateTo: "2026-11-09", Page: 1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if day.Total != 1 || day.Rows[0].SettlementNo != "BILL-9" {
		t.Fatalf("day=%+v", day)
	}
}
