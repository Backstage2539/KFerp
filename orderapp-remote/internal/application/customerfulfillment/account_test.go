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

func TestOrderFeeAccountGroupingAndCategoryPagination(t *testing.T) {
	orders := []AccountOrder{
		{ID: 1, OrderNo: "SO-1", OrderDate: "2026-10-22", TotalCents: 39700, ShippingCents: 0, PaidCents: 10000, InPeriod: true},
		{ID: 2, OrderNo: "SO-2", OrderDate: "2026-09-30", TotalCents: 5000, InPeriod: false},
	}
	fees := []AccountFee{
		{ID: 1, OrderID: 1, OrderNo: "SO-1", FeeType: "shipping", FeeName: "寄件实际运费调整", AmountCents: 2020, OccurredAt: "2026-10-26", SourceType: "freight_actual", SourceID: 101, PaymentStatus: "unpaid"},
		{ID: 2, OrderID: 1, OrderNo: "SO-1", FeeType: "direct_ship_service", AmountCents: 600, OccurredAt: "2026-10-22", PaymentStatus: "paid"},
		{ID: 3, OrderID: 0, OrderNo: "", FeeType: "adjustment", FeeName: "无法关联费用", AmountCents: 300, OccurredAt: "2026-10-25", SourceType: "import", SourceID: 303},
		{ID: 4, OrderID: 2, OrderNo: "SO-2", FeeType: "shipping", AmountCents: 900, OccurredAt: "2026-10-27", SourceType: "freight_actual", SourceID: 102},
	}
	groups, summary := BuildOrderFeeGroups(orders, fees)
	if len(groups) != 3 {
		t.Fatalf("groups=%+v", groups)
	}
	if summary.GoodsCents != 39700 || summary.ShippingCents != 2920 || summary.DirectShipServiceCents != 600 || summary.OtherCents != 300 || summary.PayableCents != 43520 || summary.PaidCents != 10600 || summary.DueCents != 32920 {
		t.Fatalf("summary=%+v", summary)
	}
	filtered := FilterOrderFeeGroups(groups, "shipping")
	if len(filtered) != 2 || filtered[0].Order.OrderNo != "SO-1" || filtered[1].Order.OrderNo != "SO-2" {
		t.Fatalf("shipping filter=%+v", filtered)
	}
	paged, total, pages := PaginateOrderFeeGroups(filtered, 2, 1)
	if len(paged) != 1 || paged[0].Order.OrderNo != "SO-2" || total != 2 || pages != 2 {
		t.Fatalf("page=%+v total=%d pages=%d", paged, total, pages)
	}
}

func TestAccountOrderFeeQueryValidation(t *testing.T) {
	q, err := NormalizeAccountQuery(AccountQuery{View: "order_fees", ChargeType: "shipping"})
	if err != nil || q.View != "order_fees" || q.ChargeType != "shipping" {
		t.Fatalf("query=%+v err=%v", q, err)
	}
	if _, err = NormalizeAccountQuery(AccountQuery{View: "other"}); err == nil {
		t.Fatal("unknown account view accepted")
	}
	if _, err = NormalizeAccountQuery(AccountQuery{View: "order_fees", ChargeType: "unknown"}); err == nil {
		t.Fatal("unknown charge type accepted")
	}
}

func TestOrderFreightUsesFeeOccurrenceDateInsteadOfOrderDate(t *testing.T) {
	order := AccountOrder{ID: 3, OrderNo: "SO-3", OrderDate: "2026-10-22", TotalCents: 12000, ShippingCents: 2000, PaidCents: 12000, InPeriod: true, HasOrderShippingFee: true}
	groups, summary := BuildOrderFeeGroups([]AccountOrder{order}, nil)
	if len(groups) != 1 || summary.GoodsCents != 10000 || summary.ShippingCents != 0 || summary.PayableCents != 10000 || groups[0].PaidCents != 10000 {
		t.Fatalf("order-date-only account=%+v %+v", groups, summary)
	}
	freight := AccountFee{OrderID: 3, OrderNo: "SO-3", FeeType: "shipping", FeeName: "订单运费", AmountCents: 2000, OccurredAt: "2026-10-28", SourceType: "order", SourceID: 3, IncludedInOrder: true}
	groups, summary = BuildOrderFeeGroups([]AccountOrder{order}, []AccountFee{freight})
	if summary.GoodsCents != 10000 || summary.ShippingCents != 2000 || summary.PayableCents != 12000 || groups[0].PaidCents != 12000 {
		t.Fatalf("fee-date freight account=%+v %+v", groups, summary)
	}
}

func TestOrderGoodsSourceDoesNotDuplicateFrozenOrderMerchandise(t *testing.T) {
	order := AccountOrder{ID: 4, OrderNo: "SO-4", TotalCents: 10000, InPeriod: true}
	fee := AccountFee{OrderID: 4, OrderNo: "SO-4", FeeType: "order_goods", AmountCents: 10000, IncludedInOrder: true}
	groups, summary := BuildOrderFeeGroups([]AccountOrder{order}, []AccountFee{fee})
	if len(groups) != 1 || len(groups[0].FeeLines) != 1 || groups[0].FeeLines[0].Category != "product" || groups[0].PayableCents != 10000 || summary.GoodsCents != 10000 || summary.PayableCents != 10000 {
		t.Fatalf("duplicate order goods source: groups=%+v summary=%+v", groups, summary)
	}
}

func TestAccountPaginationClampsLargePage(t *testing.T) {
	d := AccountData{Rows: []AccountOrder{{ID: 1}}}
	d.Paginate(int(^uint(0)>>1), 20)
	if d.Page != 1 || len(d.Rows) != 1 {
		t.Fatalf("bad page: %+v", d)
	}
}
