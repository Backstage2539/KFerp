package sales

import (
	"encoding/json"
	d "orderapp/internal/domain/orderprogress"
	"strings"
	"testing"
)

func TestOrderDetailIncludesSharedFulfillmentProgress(t *testing.T) {
	p := d.Compute(d.Input{DeliveryMode: "customer_stock", DeliveryStatus: "received", ProcessStatus: "生产完成"})
	data := editDataForAPI(&OrderEditData{FulfillmentProgress: &p, DeliveryMode: "customer_stock", DeliveryStatus: "received"})
	raw, e := json.Marshal(data)
	if e != nil || !strings.Contains(string(raw), `"label":"生产完成 · 已入库"`) || !strings.Contains(string(raw), `"can_ship":false`) {
		t.Fatalf("detail %s %v", raw, e)
	}
}
