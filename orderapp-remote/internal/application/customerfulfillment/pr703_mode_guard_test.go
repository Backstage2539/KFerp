package customerfulfillment

import (
	"testing"
)

func TestPR703OrderAndDropshipModesCannotBeInterchanged(t *testing.T) {
	base := MiniDirectShipCommand{CustomerID: 1, RecipientName: "收件人", RecipientPhone: "13800000000", DetailAddress: "大理市", Items: []MiniDirectShipItemCommand{{ProductID: 1, BomSpecID: 2, Qty: 1}}}
	for _, c := range []struct{ name, usage, mode, delivery, warehouse string }{
		{"free product order", "product_order", "stock", "", ""},
		{"priced product order hijack", "product_order", "on_demand", "", ""},
		{"dropship retained delivery", "direct_ship", "stock", "customer_stock", "CUSTOMER-1"},
		{"unknown mode", "direct_ship", "invalid", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := base
			cmd.UsageCode = c.usage
			cmd.FulfillmentMode = c.mode
			cmd.DeliveryMode = c.delivery
			cmd.DeliveryWarehouseCode = c.warehouse
			if _, e := normalizeMiniDirectShipCommand(cmd, false); e == nil {
				t.Fatal("invalid entry/mode combination accepted")
			}
		})
	}
	valid := base
	valid.UsageCode = "product_order"
	valid.DeliveryMode = "customer_stock"
	valid.DeliveryWarehouseCode = "CUSTOMER-1"
	if _, e := normalizeMiniDirectShipCommand(valid, false); e != nil {
		t.Fatalf("normal paid retained purchase rejected before warehouse authority validation: %v", e)
	}
}
