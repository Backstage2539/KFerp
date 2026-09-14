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
