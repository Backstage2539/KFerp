package messagecenter

import (
	"reflect"
	"testing"
)

func TestLifecycleSixMilestones(t *testing.T) {
	cases := []struct {
		name          string
		before, after OrderState
		created       bool
		want          []string
	}{
		{"created", OrderState{}, OrderState{}, true, []string{"created"}},
		{"paid", OrderState{Payment: "未付款"}, OrderState{Payment: "已付款"}, false, []string{"paid"}},
		{"received alias", OrderState{Payment: "未付款"}, OrderState{Payment: "已收款"}, false, []string{"paid"}},
		{"start", OrderState{Production: "待生产"}, OrderState{Production: "生产中"}, false, []string{"production_started"}},
		{"finish", OrderState{Production: "生产中"}, OrderState{Production: "生产完成"}, false, []string{"production_finished"}},
		{"partial shipment", OrderState{Shipping: "未发货"}, OrderState{Shipping: "部分发货"}, false, []string{"shipped"}},
		{"full shipment", OrderState{Shipping: "部分发货"}, OrderState{Shipping: "已发货"}, false, []string{"shipped"}},
		{"void", OrderState{}, OrderState{Void: true}, false, []string{"cancelled"}},
		{"unchanged", OrderState{Payment: "已付款"}, OrderState{Payment: "已付款"}, false, nil},
		{"paid alias unchanged", OrderState{Payment: "已付款"}, OrderState{Payment: "已收款"}, false, nil},
		{"deposit", OrderState{}, OrderState{Payment: "预付款（付款未完成）"}, false, nil},
		{"stock isn't produced", OrderState{}, OrderState{Production: "库存待发货"}, false, nil},
		{"partial production", OrderState{Production: "生产中"}, OrderState{Production: "部分完成"}, false, nil},
		{"rollback", OrderState{Production: "生产完成"}, OrderState{Production: "待生产"}, false, nil},
		{"cancel wins", OrderState{}, OrderState{Void: true, Payment: "已付款", Shipping: "已发货"}, false, []string{"cancelled"}},
		{"one operation", OrderState{}, OrderState{Payment: "已付款", Production: "生产中"}, false, []string{"paid", "production_started"}},
		{"free sample", OrderState{}, OrderState{Payment: "已付款", SettlementReason: "free_sample"}, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := OrderMilestones(c.before, c.after, c.created); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestLifecycleRecipientPolicy(t *testing.T) {
	for _, event := range []string{"created", "paid", "production_started", "production_finished", "shipped", "cancelled"} {
		if !NotifyProduction([]string{event}) && event != "paid" && event != "shipped" {
			t.Fatal(event)
		}
	}
	if NotifyProduction([]string{"paid", "shipped"}) {
		t.Fatal("production must not receive finance-only notices")
	}
}

func TestLifecycleMessageUsesSnapshotAndPreciseTarget(t *testing.T) {
	n := OrderNotice{ChangeID: 9, OrderID: 42, CustomerID: 8, OrderNo: "SO-42", CustomerName: "测试客户", Milestones: []string{"shipped"}, State: OrderState{Shipping: "部分发货"}, OccurredAt: "2026-11-08 10:00:00", Items: "测试商品 250g × 2", Test: true}
	if n.MiniPath(false) != "pages/order-notifications/order-notifications" {
		t.Fatal("virtual test must not open a real order")
	}
	n.Test = false
	if got := n.MiniPath(false); got != "pages/service/service?key=orders&source=notice&order_id=42&customer_id=8" {
		t.Fatal(got)
	}
	if got := n.MiniPath(true); got != "pages/employee-order-detail/employee-order-detail?id=42" {
		t.Fatal(got)
	}
	if n.StatusText() != "部分发货" {
		t.Fatal(n.StatusText())
	}
	if n.GroupText() == "" || n.Values()["order_no"] != "SO-42" {
		t.Fatal("missing snapshot")
	}
}
