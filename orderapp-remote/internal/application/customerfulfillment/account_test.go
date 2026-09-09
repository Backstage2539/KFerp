package customerfulfillment

import "testing"

func TestAccountQueryPeriodAndPayment(t *testing.T) {
	q, err := NormalizeAccountQuery(AccountQuery{Period: "week", Anchor: "2026-09-09"})
	if err != nil || q.DateFrom != "2026-09-07" || q.DateTo != "2026-09-13" {
		t.Fatalf("week: %+v %v", q, err)
	}
	q, err = NormalizeAccountQuery(AccountQuery{Period: "month", Anchor: "2026-02-12"})
	if err != nil || q.DateFrom != "2026-02-01" || q.DateTo != "2026-02-28" {
		t.Fatalf("month: %+v %v", q, err)
	}
	if _, err = NormalizeAccountQuery(AccountQuery{DateFrom: "2026-09-10", DateTo: "2026-09-01"}); err == nil {
		t.Fatal("reversed period accepted")
	}
	row := AccountOrder{TotalCents: 10000, PrepaymentCents: 3000, PayStatus: "预付款（付款未完成）"}
	row.SetPayment()
	if row.PaidCents != 3000 || row.DueCents != 7000 || row.PaymentStatus != "partial" {
		t.Fatalf("partial: %+v", row)
	}
	row.PayStatus = "已付款"
	row.SetPayment()
	if row.PaidCents != 10000 || row.DueCents != 0 {
		t.Fatalf("paid: %+v", row)
	}
}

func TestAccountPaginationClampsLargePage(t *testing.T) {
	d := AccountData{Rows: []AccountOrder{{ID: 1}}}
	d.Paginate(int(^uint(0)>>1), 20)
	if d.Page != 1 || len(d.Rows) != 1 {
		t.Fatalf("bad page: %+v", d)
	}
}
