package customerportal

import (
	sales "orderapp/internal/application/sales"
	d "orderapp/internal/domain/orderprogress"
	"testing"
)

func TestMiniEmployeeProgressDTOCarriesSharedReceiptStatus(t *testing.T) {
	p := d.Compute(d.Input{DeliveryMode: "customer_stock", DeliveryStatus: "received", ProcessStatus: "生产完成"})
	v := miniEmployeeOrderDetail(sales.OrderRow{ID: 113, DeliveryMode: "customer_stock", FulfillmentProgress: &p}, sales.OrderFormData{EditData: &sales.OrderEditData{ID: 113}})
	if v.DeliveryMode != "customer_stock" || v.FulfillmentProgress == nil || v.FulfillmentProgress.Label != "生产完成 · 已入库" || v.FulfillmentProgress.CanShip {
		t.Fatalf("progress %+v", v)
	}
}
