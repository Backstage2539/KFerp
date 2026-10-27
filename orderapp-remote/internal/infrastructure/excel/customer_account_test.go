package excel

import (
	"bytes"
	"testing"

	app "orderapp/internal/application/customerfulfillment"

	"github.com/xuri/excelize/v2"
)

func TestRenderCustomerAccountIncludesUnifiedSummarySourcesAndDisputes(t *testing.T) {
	data := app.AccountData{
		CustomerName: "测试客户", DateFrom: "2026-09-01", DateTo: "2026-09-30", AsOf: "2026-09-14 16:00:00",
		Summary: app.AccountSummary{GoodsCents: 12345, ProcessingCents: 2345, DirectShipServiceCents: 345, ShippingCents: 1234, PayableCents: 16269, PaidCents: 5000, DueCents: 11269},
		Fees:    []app.AccountFee{{ID: 7, FeeType: "shipping", FeeName: "快递费", AmountCents: 1234, SourceType: "order", SourceID: 9, IncludedInOrder: true}},
		Settlements: []app.AccountSettlement{{
			SettlementNo: "SET-1", ReconciliationStatus: "disputed", StatementRevision: "rev-1",
			Disputes: []app.AccountStatementDispute{{FeeItemID: 7, Reason: "运费不符", Status: "resolved", Reply: "已复核"}},
		}},
	}
	body, err := RenderCustomerAccount(data)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got := book.GetSheetList(); len(got) != 4 || got[0] != "订单账单" || got[1] != "费用明细" || got[3] != "账单异议" {
		t.Fatalf("sheets=%v", got)
	}
	expectations := map[string]map[string]string{
		"订单账单":  {"A2": "商品货款", "B2": "123.45", "A3": "应付"},
		"费用明细":  {"D3": "快递费", "J3": "是"},
		"正式结算单": {"H3": "有异议", "J3": "rev-1"},
		"账单异议":  {"D2": "运费不符", "F2": "已复核"},
	}
	for sheet, cells := range expectations {
		for cell, want := range cells {
			if got, err := book.GetCellValue(sheet, cell); err != nil || got != want {
				t.Fatalf("%s!%s=%q err=%v want=%q", sheet, cell, got, err, want)
			}
		}
	}
}

func TestRenderCustomerOrderFeeAccountExportsAllGroupedChargesAndNumericMoney(t *testing.T) {
	order := app.AccountOrder{ID: 9, OrderNo: "SO-9", OrderDate: "2026-10-22", ReceiverName: "客户", DiscountCents: 300, Items: []app.AccountOrderItem{{Name: "咖啡豆", Spec: "227g", Quantity: "2", Unit: "袋", UnitPrice: "20.00", LineTotal: "39.70"}}}
	feeLines := []app.AccountFeeLine{
		{AccountFee: app.AccountFee{OrderID: 9, OrderNo: "SO-9", FeeType: "product", FeeName: "商品货款", AmountCents: 3970, OccurredAt: "2026-10-22", SourceType: "order", SourceID: 9}},
		{AccountFee: app.AccountFee{OrderID: 9, OrderNo: "SO-9", FeeType: "shipping", FeeName: "寄件实际运费调整", AmountCents: 2020, OccurredAt: "2026-10-26", SourceType: "freight_actual", SourceID: 71, TrackingNo: "SF123"}},
		{AccountFee: app.AccountFee{OrderID: 9, OrderNo: "SO-9", FeeType: "direct_ship_service", FeeName: "代发服务费", AmountCents: 300, OccurredAt: "2026-10-22", SourceType: "order", SourceID: 9}},
	}
	data := app.AccountData{
		CustomerName: "测试客户", DateFrom: "2026-10-01", DateTo: "2026-10-31", AsOf: "2026-10-27 15:00:00",
		FeeSummary: app.AccountFeeSummary{GoodsCents: 3970, ShippingCents: 2020, DirectShipServiceCents: 300, PayableCents: 6290, PaidCents: 1000, DueCents: 5290},
		OrderFeeGroups: []app.AccountOrderFeeGroup{{Order: order, PayableCents: 6290, PaidCents: 1000, DueCents: 5290,
			FeeLines: feeLines,
			Shipments: []app.AccountShipment{
				{OrderID: 9, OrderNo: "SO-9", TrackingNo: "SF123", ShippedAt: "2026-10-26 10:00:00", SettlementWeightKG: "1.250", ShippingCents: 2020, Source: "快递揽件记录"},
				{OrderID: 9, OrderNo: "SO-9", TrackingNo: "SF456", ShippedAt: "2026-10-27 09:00:00", SettlementWeightKG: "0.750", Source: "快递揽件记录"},
			},
		}},
	}
	body, err := RenderCustomerOrderFeeAccount(data)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	if got := book.GetSheetList(); len(got) != 3 || got[0] != "账期汇总" || got[1] != "订单汇总" || got[2] != "费用明细" {
		t.Fatalf("sheets=%v", got)
	}
	if got, _ := book.GetCellValue("费用明细", "B2"); got != "SO-9" {
		t.Fatalf("order=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "J3"); got != "SF123" {
		t.Fatalf("tracking=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "H3"); got != "2026-10-26 10:00:00（快递揽件记录）" {
		t.Fatalf("shipping time for matched tracking=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "H4"); got != "2026-10-26 10:00:00（快递揽件记录）；2026-10-27 09:00:00（快递揽件记录）" {
		t.Fatalf("service shipments=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "I4"); got != "1.250；0.750" {
		t.Fatalf("service weights=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "J4"); got != "SF123, SF456" {
		t.Fatalf("service tracking=%q", got)
	}
	if got, _ := book.GetCellValue("费用明细", "F3"); got != "20.20" {
		t.Fatalf("freight amount=%q", got)
	}
	cell, _ := excelize.CoordinatesToCellName(6, 3)
	value, err := book.GetCellValue("费用明细", cell, excelize.Options{RawCellValue: true})
	if err != nil || value != "20.2" {
		t.Fatalf("amount cell=%q err=%v", value, err)
	}
}
