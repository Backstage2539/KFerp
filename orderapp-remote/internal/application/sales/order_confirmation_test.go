package sales

import "testing"

func TestFulfillmentLaterShipmentStatusesStayLocked(t *testing.T) {
	for _, status := range []string{"部分发货", "已出库", "已签收", "已收货", "已完成"} {
		if result := EvaluateOrderEditability(OrderEditState{ShipStatus: status}); result.CanEdit {
			t.Errorf("status %s allowed edit", status)
		}
	}
}
