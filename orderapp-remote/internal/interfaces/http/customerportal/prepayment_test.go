package customerportal

import (
	"encoding/json"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestMiniPrepaymentCommandAndLegacyEditCompatibility(t *testing.T) {
	var request miniEmployeeOrderRequest
	if err := json.Unmarshal([]byte(`{"customer_id":8,"order_date":"2026-09-07","pay_status_id":9,"payment_method":"微信支付","prepayment_amount":30,"items":[{"product_id":10,"qty":1,"unit_price":100}]}`), &request); err != nil {
		t.Fatal(err)
	}
	cmd, err := miniEmployeeSaveOrderCommand(request, "mini-employee:7", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.PrepaymentAmount == nil || *cmd.PrepaymentAmount != 30 || cmd.PayStatusID != 9 || cmd.PaymentMethod != "微信支付" {
		t.Fatalf("lost deposit: %+v", cmd)
	}
	existing := &salesapp.OrderEditData{CustomerID: 8, PayStatusID: 9, PaymentMethod: "微信支付", PrepaymentAmount: "30.00"}
	cmd.PayStatusID = 2
	if err := miniEmployeePreserveHiddenOrderFields(&cmd, existing); err != nil {
		t.Fatal(err)
	}
	if cmd.PayStatusID != 2 || *cmd.PrepaymentAmount != 30 {
		t.Fatal("explicit settlement overwritten")
	}
	cmd.PrepaymentAmount = nil
	cmd.PayStatusID = 1
	cmd.PaymentMethod = ""
	if err := miniEmployeePreserveHiddenOrderFields(&cmd, existing); err != nil {
		t.Fatal(err)
	}
	if cmd.PayStatusID != 9 || cmd.PaymentMethod != "微信支付" || cmd.PrepaymentAmount != nil {
		t.Fatal("legacy edit must preserve server payment")
	}
}
