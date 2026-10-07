package officialaccount

import (
	portal "orderapp/internal/application/customerportal"
	"strings"
	"testing"
)

func TestEntryPolicyNeverLeaksPrivateOrWithdrawnPrice(t *testing.T) {
	e := Entry{Enabled: true, Visibility: "public", OwnerType: "customer", OwnerKey: "7", Status: "published"}
	if CheckEntry(e, nil) == nil {
		t.Fatal("private publication became public")
	}
	e.Visibility = "authenticated"
	c := portal.CurrentContext{CurrentCustomerID: 8}
	if CheckEntry(e, &c) == nil {
		t.Fatal("cross customer allowed")
	}
	e.OwnerType = "official"
	e.Visibility = "public"
	if err := CheckEntry(e, nil); err != nil {
		t.Fatal(err)
	}
	e.Status = "withdrawn"
	if CheckEntry(e, nil) == nil {
		t.Fatal("withdrawn entry allowed")
	}
}

func TestRecentSummaryIsBoundedAndKeepsAllOrderHeaders(t *testing.T) {
	rows := []portal.CustomerOrderSummary{}
	for _, n := range []string{"SO-1", "SO-2", "SO-3"} {
		rows = append(rows, portal.CustomerOrderSummary{OrderNo: n, OrderDate: "2026-10-08", GrandTotal: "88.00", ProcessStatus: "生产中", PayStatus: "未付款", ShipStatus: "待发货", Items: []portal.CustomerOrderItemSummary{{ItemName: strings.Repeat("咖啡豆", 1000), Qty: "2"}}})
	}
	got := OrderSummary("测试客户", rows)
	if len([]byte(got)) > 1900 {
		t.Fatalf("oversized reply: %d", len([]byte(got)))
	}
	for _, s := range []string{"SO-1", "SO-2", "SO-3", "88.00", "生产中", "测试客户"} {
		if !strings.Contains(got, s) {
			t.Fatalf("missing %s", s)
		}
	}
}

func TestMenuOnlyAllowsRegisteredActionsAndSafePaths(t *testing.T) {
	m := Menu{Buttons: []Button{{Name: "最近一次", Type: "click", Key: "ORDERS_RECENT_1"}}}
	if err := ValidateMenu(m, "wx-mini"); err != nil {
		t.Fatal(err)
	}
	m.Buttons[0].Key = "FREE_FORM_SQL"
	if ValidateMenu(m, "wx-mini") == nil {
		t.Fatal("unknown command allowed")
	}
	m.Buttons = []Button{{Name: "订单", Type: "miniprogram", AppID: "wx-mini", URL: "https://erp.qacoohee.com/app/", PagePath: "pages/service/service?key=orders"}}
	if err := ValidateMenu(m, "wx-mini"); err != nil {
		t.Fatal(err)
	}
	m.Buttons[0].PagePath = "pages/employee-orders/employee-orders"
	if ValidateMenu(m, "wx-mini") == nil {
		t.Fatal("employee path allowed")
	}
}

func TestSummaryFitsWeChatByteLimitWithLongFieldsAndManyItems(t *testing.T) {
	rows := []portal.CustomerOrderSummary{}
	for _, number := range []string{"SO-A", "SO-B", "SO-C"} {
		item := portal.CustomerOrderItemSummary{ItemName: strings.Repeat("豆", 100), Spec: strings.Repeat("规", 100), Qty: strings.Repeat("9", 30), Unit: strings.Repeat("袋", 30)}
		rows = append(rows, portal.CustomerOrderSummary{OrderNo: number + strings.Repeat("N", 100), OrderDate: "2026-10-08", GrandTotal: strings.Repeat("9", 30), PayStatus: strings.Repeat("款", 30), ProcessStatus: strings.Repeat("产", 30), ShipStatus: strings.Repeat("发", 30), ShipTrackingNo: strings.Repeat("W", 100), Items: []portal.CustomerOrderItemSummary{item, item, item}})
	}
	text := OrderSummary(strings.Repeat("客", 100), rows)
	if len(text) > 2048 {
		t.Fatalf("WeChat 2048-byte limit exceeded: %d", len(text))
	}
	for _, part := range []string{"SO-A", "SO-B", "SO-C", "完整明细请点击"} {
		if !strings.Contains(text, part) {
			t.Fatalf("lost header or truncation hint: %s", part)
		}
	}
}
