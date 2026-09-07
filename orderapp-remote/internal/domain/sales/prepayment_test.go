package sales

import (
	"math"
	"testing"
)

func TestPrepaymentValidation(t *testing.T) {
	for _, tc := range []struct {
		status      string
		paid, total float64
		valid       bool
	}{
		{"预付款（付款未完成）", 30, 100, true}, {"预付款（付款未完成）", 0, 100, false},
		{"预付款（付款未完成）", 100, 100, false}, {"预付款（付款未完成）", 101, 100, false},
		{"预付款（付款未完成）", -1, 100, false}, {"预付款（付款未完成）", math.NaN(), 100, false},
		{"预付款（付款未完成）", 1.001, 100, false}, {"已付款", 30, 100, true}, {"未付款", 30, 100, false}, {"未付款", 0, 100, true},
	} {
		if err := ValidatePrepayment(tc.status, tc.paid, tc.total); (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
}
func TestPaymentBalance(t *testing.T) {
	paid, due := OrderPaymentAmounts("预付款（付款未完成）", 30, 100)
	if paid != "30.00" || due != "70.00" {
		t.Fatalf("%s/%s", paid, due)
	}
	paid, due = OrderPaymentAmounts("已付款", 30, 100)
	if paid != "100.00" || due != "0.00" {
		t.Fatalf("%s/%s", paid, due)
	}
}
