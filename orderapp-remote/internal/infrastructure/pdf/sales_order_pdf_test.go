package pdf

import (
	"bytes"
	salesdomain "orderapp/internal/domain/sales"
	"testing"
)

func TestRenderSalesOrderPDF(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.Render(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260430-0008",
		OrderDate:    "2026-04-30",
		CustomerName: "某某咖啡馆",
		CompanyName:  "浅焙作坊咖啡",
		PaymentText:  "微信或对公转账",
		Note:         "请密封避光保存",
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		TotalAmount: "134.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "134.00",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		head := b
		if len(head) > 5 {
			head = head[:5]
		}
		t.Fatalf("PDF missing header: %q", head)
	}
	if len(b) < 1000 {
		t.Fatalf("PDF size = %d, want >= 1000", len(b))
	}
}
