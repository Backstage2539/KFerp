package sales

import (
	"strings"
	"testing"
)

func TestOrderCopyDetailsTextSavedFields(t *testing.T) {
	source := OrderCopySnapshot{PayStatus: "预付款（付款未完成）", Order: OrderEditData{
		ID: 42, OrderNo: "SO-42", OrderDate: "2026-10-18", DocumentDate: "2026-10-19", ReceiverName: "张先生", ReceiverPhone: "13800000000", ReceiverAddress: "上海市测试路1号", Notes: "不要复制备注", GrandTotal: "999.99",
		Items: []OrderEditItem{
			{Product: "已改名商品", ProductNameSnapshot: "下单品名", CustomerProductDisplayNameSnapshot: "客户拼配", Spec: "454", Qty: "10.500", Unit: "袋", UnitPrice: "99.99"},
			{ProductNameSnapshot: "挂耳", ProductKind: "drip_bag", Spec: "10袋/盒", SalesUnit: "box", UnitBagCount: 10, Qty: "5", Unit: "盒"},
			{Product: "曲奇", BomSpecName: "2Kg袋装", Spec: "2000", Qty: "1", Unit: "袋"},
			{Product: "历史换算", BomSpecName: "旧来源", Spec: "454", PriceSourceJSON: `{"spec_conversion":{"target_spec_name":"250g 袋装","target_unit":"袋"}}`, Qty: "2", Unit: "袋"},
		}}}
	got := BuildOrderDetailsText(source)
	want := "订单号：SO-42\n订单日期：2026-10-18\n付款状态：预付款（付款未完成）\n\n订单详情：\n1. 客户拼配｜规格：454g｜数量：10.5 袋\n2. 挂耳｜规格：10袋/盒｜数量：5 盒\n3. 曲奇｜规格：2Kg袋装｜数量：1 袋\n4. 历史换算｜规格：250g 袋装｜数量：2 袋\n\n收件人：张先生\n电话：13800000000\n收件地址：上海市测试路1号"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, bad := range []string{"999.99", "99.99", "不要复制备注", "已改名商品", "2026-10-19"} {
		if strings.Contains(got, bad) {
			t.Fatalf("leak %s", bad)
		}
	}
}
func TestOrderCopyDetailsTextMissingAndVoid(t *testing.T) {
	source := OrderCopySnapshot{PayStatus: "无需收款", Order: OrderEditData{IsVoid: true, Items: []OrderEditItem{{Product: "挂耳", ProductKind: "drip_bag", SalesUnit: "bag", UnitBeanG: "10", Qty: "0", Unit: "袋"}, {Product: "杯", Spec: "大杯", Qty: "1.125", Unit: "个"}}}}
	got := BuildOrderDetailsText(source)
	for _, s := range []string{"【已作废订单】\n", "付款状态：无需收款", "收件人：未填写", "电话：未填写", "收件地址：未填写", "规格：10g/袋｜数量：0 袋", "规格：大杯｜数量：1.125 个"} {
		if !strings.Contains(got, s) {
			t.Fatalf("missing %s in %s", s, got)
		}
	}
}
