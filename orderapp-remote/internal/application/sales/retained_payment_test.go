package sales

import "testing"

func TestValidateNewRetainedPayment(t *testing.T) {
	amount := 1.0
	tests := []struct {
		name  string
		cmd   SaveOrderCommand
		valid bool
	}{
		{"new unpaid", SaveOrderCommand{DeliveryMode: "customer_stock"}, true},
		{"prepayment", SaveOrderCommand{DeliveryMode: "customer_stock", PrepaymentAmount: &amount}, false},
		{"receipt amount", SaveOrderCommand{DeliveryMode: "customer_stock", PaymentGoodsAmount: 1}, false},
		{"receipt method", SaveOrderCommand{DeliveryMode: "customer_stock", PaymentMethod: "现金"}, false},
		{"voucher", SaveOrderCommand{DeliveryMode: "customer_stock", PaymentVoucherAssetID: 1}, false},
		{"existing retained", SaveOrderCommand{EditID: 1, DeliveryMode: "customer_stock", PaymentGoodsAmount: 1}, true},
		{"normal delivery", SaveOrderCommand{DeliveryMode: "ship", PaymentGoodsAmount: 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateNewRetainedPayment(tt.cmd); (got == nil) != tt.valid {
				t.Fatalf("got %v", got)
			}
		})
	}
}
