package pdf

import (
	"bytes"
	"strings"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestRenderCustomerBillProducesPDFWithLongFeeDetails(t *testing.T) {
	lines := make([]app.SettlementLineSummary, 0, 70)
	for i := 0; i < 70; i++ {
		lines = append(lines, app.SettlementLineSummary{ChargeType: "shipping", OccurredAt: "2026-10-22", SourceNo: "SF5144416745654", Description: "超长中文费用名称：" + strings.Repeat("客户运费结算明细说明", 5), ReceivableCents: 1234, AmountCents: 1234})
	}
	body, err := RenderCustomerBill(app.BillingBill{SettlementResult: app.SettlementResult{SettlementNo: "CB-488-001", PeriodFrom: "2026-10-01", PeriodTo: "2026-10-31", TotalAmountCents: 86380, Lines: lines}, CustomerName: "吴国宏"})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < 1000 || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("invalid PDF output: %d bytes", len(body))
	}
}
