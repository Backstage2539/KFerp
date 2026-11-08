package messagecenter

import (
	"fmt"
	"strings"
)

// OrderState contains only notification-safe state, never receipts or addresses.
type OrderState struct {
	Payment          string `json:"payment"`
	Production       string `json:"production"`
	Shipping         string `json:"shipping"`
	Void             bool   `json:"void"`
	SettlementReason string `json:"settlement_reason"`
}

func OrderMilestones(before, after OrderState, created bool) []string {
	if after.Void {
		if !before.Void {
			return []string{"cancelled"}
		}
		return nil
	}
	var result []string
	if created {
		result = append(result, "created")
	}
	paid := func(s string) bool { return s == "已付款" || s == "已收款" || s == "已结清" }
	started := func(s string) bool { return s == "生产中" || s == "部分完成" || s == "部分生产完成" }
	finished := func(s string) bool { return s == "生产完成" || s == "已生产完成" }
	shipped := func(s string) int {
		switch s {
		case "部分发货":
			return 1
		case "已发货", "已出库", "已签收", "已收货", "已完成":
			return 2
		}
		return 0
	}
	if paid(after.Payment) && !paid(before.Payment) && after.SettlementReason != "free_sample" {
		result = append(result, "paid")
	}
	if started(after.Production) && !started(before.Production) && !finished(before.Production) {
		result = append(result, "production_started")
	}
	if finished(after.Production) && !finished(before.Production) {
		result = append(result, "production_finished")
	}
	if shipped(after.Shipping) > shipped(before.Shipping) {
		result = append(result, "shipped")
	}
	return result
}

var MilestoneLabels = map[string]string{"created": "下单成功", "paid": "付款成功", "production_started": "开始生产", "production_finished": "生产完成", "shipped": "已发货", "cancelled": "订单已取消/失效"}

func NotifyProduction(events []string) bool {
	for _, e := range events {
		if e == "created" || e == "production_started" || e == "production_finished" || e == "cancelled" {
			return true
		}
	}
	return false
}

type OrderNotice struct {
	ChangeID        int64      `json:"change_id"`
	OrderID         int64      `json:"order_id"`
	CustomerID      int64      `json:"customer_id"`
	OrderNo         string     `json:"order_no"`
	CustomerName    string     `json:"customer_name"`
	ResponsibleID   int64      `json:"responsible_id"`
	ResponsibleName string     `json:"responsible_name"`
	Items           string     `json:"items"`
	DueDate         string     `json:"due_date"`
	Milestones      []string   `json:"milestones"`
	State           OrderState `json:"state"`
	OccurredAt      string     `json:"occurred_at"`
	Test            bool       `json:"test"`
}

func (n OrderNotice) StatusText() string {
	var parts []string
	for _, e := range n.Milestones {
		label := MilestoneLabels[e]
		if e == "shipped" && n.State.Shipping != "" {
			label = n.State.Shipping
		}
		if label != "" {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, "、")
}
func (n OrderNotice) MiniPath(employee bool) string {
	if n.Test {
		return "pages/order-notifications/order-notifications"
	}
	if employee {
		return fmt.Sprintf("pages/employee-order-detail/employee-order-detail?id=%d", n.OrderID)
	}
	return fmt.Sprintf("pages/service/service?key=orders&source=notice&order_id=%d&customer_id=%d", n.OrderID, n.CustomerID)
}
func (n OrderNotice) Values() map[string]string {
	return map[string]string{"order_no": n.OrderNo, "customer_name": n.CustomerName, "status": n.StatusText(), "occurred_at": n.OccurredAt, "items": n.Items, "due_date": n.DueDate, "responsible_name": n.ResponsibleName}
}
func (n OrderNotice) GroupText() string {
	prefix := "【订单通知】"
	if n.Test {
		prefix = "【测试通知·虚拟订单】"
	}
	return fmt.Sprintf("%s%s\n订单：%s\n客户：%s\n商品：%s\n交期：%s\n负责人：%s\n时间：%s\n通知号：N%d", prefix, n.StatusText(), n.OrderNo, n.CustomerName, n.Items, n.DueDate, n.ResponsibleName, n.OccurredAt, n.ChangeID)
}
