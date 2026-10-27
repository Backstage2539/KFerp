package orderprogress

import "testing"

func TestProgressSeparatesReceiptAndShipping(t *testing.T) {
	for _, tc := range []struct {
		in    Input
		label string
		ship  bool
	}{
		{Input{DeliveryMode: "customer_stock", DeliveryStatus: "received", ProcessStatus: "生产完成", ShipStatus: "待发货"}, "生产完成 · 已入库", false},
		{Input{DeliveryMode: "customer_stock", DeliveryStatus: "partially_received", ProcessStatus: "生产中"}, "生产中 · 部分入库", false},
		{Input{FulfillmentMode: "stock", StockReady: true, ProcessStatus: "库存待发货"}, "库存充足 · 可以发货", true},
		{Input{FulfillmentMode: "stock", StockReady: false, ProcessStatus: "库存待发货"}, "库存不足 · 待入库", false},
		{Input{FulfillmentMode: "stock", StockReady: true, ShipStatus: "已发货"}, "已发货", false},
		{Input{FulfillmentMode: "stock", StockReady: true, IsVoid: true}, "已作废", false},
		{Input{FulfillmentMode: "stock", StockReady: true, ConfirmationStatus: "pending"}, "待确认", false},
	} {
		got := Compute(tc.in)
		if got.Label != tc.label || got.CanShip != tc.ship {
			t.Errorf("%+v => %+v", tc.in, got)
		}
	}
}
