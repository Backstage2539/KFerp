package sales

import (
	"encoding/json"
	"testing"
)

func TestOtherFeeAmountValidation(t *testing.T) {
	for _, raw := range []string{`"0.01"`, `88.12`, `"0"`, `"9999999999.99"`} {
		var v OtherFeeInput
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{`-1`, `1.001`, `"1e2"`, `true`, `[]`, `"NaN"`, `10000000000`} {
		var v OtherFeeInput
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			t.Fatalf("invalid fee accepted: %s", raw)
		}
	}
}
func TestCustomerCannotSetOrClearOtherFee(t *testing.T) {
	c := SaveOrderCommand{PortalServiceCode: "product_order", DeliveryMode: "customer_stock"}
	if err := ValidateCustomerOrder(c, false); err != nil {
		t.Fatal(err)
	}
	zero := 0.0
	c.OtherFee = &zero
	if err := ValidateCustomerOrder(c, false); err == nil {
		t.Fatal("customer cleared fee")
	}
	c.OtherFee = nil
	note := "包装"
	c.OtherFeeNote = &note
	if err := ValidateCustomerOrder(c, false); err == nil {
		t.Fatal("customer changed fee note")
	}
}
