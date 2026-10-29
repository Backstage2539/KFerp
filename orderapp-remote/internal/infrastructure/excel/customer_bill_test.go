package excel

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	app "orderapp/internal/application/customerfulfillment"
)

func TestRenderCustomerBillIncludesSnapshotAndCurrentBalances(t *testing.T) {
	body, err := RenderCustomerBill(app.BillingBill{
		SettlementResult: app.SettlementResult{SettlementNo: "CB-488-001", PeriodFrom: "2026-10-01", PeriodTo: "2026-10-31", TotalAmountCents: 1800, ReceivedCents: 500, DueAmountCents: 1300,
			Lines: []app.SettlementLineSummary{{ChargeType: "shipping", OccurredAt: "2026-10-22", SourceNo: "SF001", Description: "顺丰运费", ReceivableCents: 2000, AmountCents: 1800, IssuedReceivedCents: 200, ReceivedCents: 500, DueCents: 1300}}},
		CustomerName: "吴国宏",
	})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for cell, want := range map[string]string{"B1": "吴国宏", "D1": "CB-488-001", "A5": "费用类型分类小计", "A6": "运费", "B6": "18", "A7": "费用类型", "D8": "顺丰运费", "E8": "20", "F8": "18", "G8": "2", "H8": "5", "I8": "13"} {
		got, err := book.GetCellValue("费用明细", cell)
		if err != nil || got != want {
			t.Fatalf("%s=%q err=%v want %q", cell, got, err, want)
		}
	}
}

func TestRenderCustomerBillIncludesOrderAndWaybillDetails(t *testing.T) {
	bill := app.BillingBill{SettlementResult: app.SettlementResult{SettlementNo: "CB-1", Lines: []app.SettlementLineSummary{{ChargeType: "shipping", AmountCents: 1110, Detail: &app.BillingLineDetail{OrderNo: "SO-117", ChargeType: "shipping", InformationSource: "出账快照", Shipments: []app.AccountShipment{{TrackingNo: "SF-123", ShippedAt: "2026-10-26", SettlementWeightKG: "1.2"}}}}}}}
	body, err := RenderCustomerBill(bill)
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	detail, err := book.GetCellValue("费用明细", "J8")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SO-117", "SF-123", "1.2", "出账快照"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail %q missing %q", detail, want)
		}
	}
	amount, _ := book.GetCellValue("费用明细", "F8")
	if amount != "11.1" {
		t.Fatalf("amount=%q", amount)
	}
	for cell, want := range map[string]string{"A2": "SO-117", "C2": "11.1", "M2": "SF-123", "N2": "1.2"} {
		got, err := book.GetCellValue("订单明细", cell)
		if err != nil || got != want {
			t.Fatalf("订单明细 %s=%q err=%v, want %q", cell, got, err, want)
		}
	}
}
