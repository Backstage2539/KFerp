package customerportal

import "testing"

func TestPrepaymentSettlementSummaryCountsOnlyUnpaidBalance(t *testing.T) {
	rows := settlementAccountingSummary([]CustomerOrderSummary{{GrandTotal: "100.00", PayStatus: "预付款（付款未完成）", PrepaymentAmount: "30.00"}, {GrandTotal: "80.00", PayStatus: "已付款", PrepaymentAmount: "20.00"}, {GrandTotal: "10.00", PayStatus: "未付款"}})
	values := map[string]string{}
	for _, r := range rows {
		values[r.Label] = r.Value
	}
	for k, want := range map[string]string{"应收总额": "190.00", "待结算金额": "80.00", "已付款金额": "110.00", "未付款订单": "2"} {
		if values[k] != want {
			t.Fatalf("%s=%s want %s", k, values[k], want)
		}
	}
}
