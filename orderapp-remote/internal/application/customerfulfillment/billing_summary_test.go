package customerfulfillment

import "testing"

func TestIssuedBillSummaryUsesCurrentBalances(t *testing.T) {
	bills := []BillingBill{
		{SettlementResult: SettlementResult{TotalAmountCents: 10000}, ReceivedCents: 3000, DueAmountCents: 7000},
		{SettlementResult: SettlementResult{TotalAmountCents: 5000}, ReceivedCents: 5000, DueAmountCents: 0, AdjustmentCents: 100},
	}
	got := SummarizeIssuedBills(bills)
	if got.Count != 2 || got.UnpaidCount != 1 || got.DueAmountCents != 7000 || got.TotalAmountCents != 15000 || got.ReceivedCents != 8000 || got.AdjustmentCents != 100 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
