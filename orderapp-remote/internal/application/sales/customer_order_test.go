package sales

import (
	"testing"
	"time"
)

func TestCustomerOrderPolicy(t *testing.T) {
	id := int64(7)
	base := SaveOrderCommand{CustomerID: 14, OrderDate: time.Now(), PortalServiceCode: "direct_ship", Items: []OrderItemCommand{{ProductID: &id, SpecG: 227, Units: 2}}}
	if err := ValidateCustomerOrder(base, false); err == nil {
		t.Fatal("ordinary direct ship must require a recipient")
	}
	if err := ValidateCustomerOrder(base, true); err != nil {
		t.Fatal(err)
	}
	base.ReceiverName = "收件人"
	base.ReceiverPhone = "13800000000"
	base.ReceiverAddress = "测试地址"
	if err := ValidateCustomerOrder(base, false); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*SaveOrderCommand){
		func(c *SaveOrderCommand) { c.EditID = 42 },
		func(c *SaveOrderCommand) { c.DiscountAmount = 1 },
		func(c *SaveOrderCommand) { c.ShippingAmount = 1 },
		func(c *SaveOrderCommand) { c.PayStatusID = 3 },
		func(c *SaveOrderCommand) { c.ShipStatusID = 3 },
		func(c *SaveOrderCommand) {
			c.Items = append([]OrderItemCommand{}, c.Items...)
			p := 1.0
			c.Items[0].ManualPrice = &p
		},
	} {
		c := base
		change(&c)
		if ValidateCustomerOrder(c, false) == nil {
			t.Fatal("customer accepted a restricted write")
		}
	}
}
