package samples

import (
	"math"
	"testing"
	"time"
)

func TestSampleFollowupSchedule(t *testing.T) {
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	for _, status := range []string{"pending", "following", "won", "abandoned"} {
		next, err := NextFollowup(status, "已联系客户", now)
		if err != nil {
			t.Fatal(err)
		}
		if Terminal(status) {
			if next != nil {
				t.Fatal("closed sample still scheduled")
			}
		} else if next == nil || !next.Equal(now.Add(7*24*time.Hour)) {
			t.Fatal("wrong next followup")
		}
	}
	if _, err := NextFollowup("following", "  ", now); err == nil {
		t.Fatal("empty activity resets timer")
	}
	if _, err := NextFollowup("unknown", "x", now); err == nil {
		t.Fatal("unknown status accepted")
	}
}
func TestSampleActorIsolation(t *testing.T) {
	req := Request{MiniUserID: 2, AssigneeID: 8}
	for _, tc := range []struct {
		p           Principal
		read, write bool
	}{
		{Principal{MiniUserID: 2, Registered: true}, true, false},
		{Principal{MiniUserID: 3, Registered: true}, false, false},
		{Principal{EmployeeID: 8, Sales: true}, true, true},
		{Principal{EmployeeID: 9, Sales: true}, false, false},
		{Principal{EmployeeID: 8}, false, false},
		{Principal{EmployeeID: 9, Admin: true}, true, true},
	} {
		if CanRead(tc.p, req) != tc.read || CanWork(tc.p, req) != tc.write {
			t.Fatalf("isolation %+v", tc)
		}
	}
}
func TestSampleConfirmValidation(t *testing.T) {
	cmd := ConfirmCommand{CustomerID: 2, Receiver: Receiver{Name: "甲", Phone: "13800000000", Address: "上海测试路"}, Items: []ConfirmItem{{ProductID: 3, BomSpecID: 4, Qty: 1, UnitPrice: 0}}}
	if err := ValidateConfirm(cmd); err != nil {
		t.Fatal(err)
	}
	for _, price := range []float64{-1, math.NaN(), math.Inf(1)} {
		bad := cmd
		bad.Items = append([]ConfirmItem(nil), cmd.Items...)
		bad.Items[0].UnitPrice = price
		if ValidateConfirm(bad) == nil {
			t.Fatal("invalid price accepted")
		}
	}
	bad := cmd
	bad.Items = nil
	if ValidateConfirm(bad) == nil {
		t.Fatal("empty items")
	}
	bad = cmd
	bad.Receiver.Address = ""
	if ValidateConfirm(bad) == nil {
		t.Fatal("missing receiver address")
	}
	bad = cmd
	bad.Items = append([]ConfirmItem(nil), cmd.Items...)
	bad.Items[0].BomSpecID = 0
	if ValidateConfirm(bad) == nil {
		t.Fatal("free form weight allowed")
	}
}

func TestSampleConfirmationUsesWholeCents(t *testing.T) {
	cmd := ConfirmCommand{CustomerID: 2, Receiver: Receiver{Name: "甲", Phone: "13800000000", Address: "测试地址"}, Items: []ConfirmItem{{ProductID: 3, BomSpecID: 4, Qty: 1, UnitPrice: 1.001}}}
	if ValidateConfirm(cmd) == nil {
		t.Fatal("fractional cent unit price accepted")
	}
	cmd.Items[0].UnitPrice = 0
	cmd.ShippingAmount = 0.001
	if ValidateConfirm(cmd) == nil {
		t.Fatal("fractional cent freight accepted")
	}
	cmd.ShippingAmount = 0.1 + 0.2
	if err := ValidateConfirm(cmd); err != nil {
		t.Fatal(err)
	}
}
