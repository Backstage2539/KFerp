package pdf

import (
	salesdomain "orderapp/internal/domain/sales"
	"os"
	"path/filepath"
	"testing"
)

func TestSalesOrderPrepaymentRows(t *testing.T) {
	rows := salesOrderFinancialRows(salesdomain.SalesOrderSnapshot{GrandTotal: "100.00", PrepaymentAmount: "30.00", PaidAmount: "30.00", UnpaidAmount: "70.00"})
	foundPaid, foundDue := false, false
	for _, r := range rows {
		if r.Label == "已支付预付款" && r.Value == "30.00" && r.Tone == "paid" {
			foundPaid = true
		}
		if r.Label == "未支付尾款" && r.Value == "70.00" && r.Tone == "unpaid" {
			foundDue = true
		}
	}
	if !foundPaid || !foundDue {
		t.Fatalf("missing payment colors: %+v", rows)
	}
}

func TestPrepaymentArtifacts(t *testing.T) {
	renderer := SalesOrderRenderer{}
	snapshot := salesdomain.SalesOrderSnapshot{OrderID: 1, OrderNo: "PR632-预付款示例", OrderDate: "2026-09-07", CustomerName: "验收客户", CompanyName: "浅焙作坊咖啡", TotalAmount: "88.00", Shipping: "12.00", Discount: "0.00", GrandTotal: "100.00", PrepaymentAmount: "30.00", PaidAmount: "30.00", UnpaidAmount: "70.00", Items: []salesdomain.SalesOrderSnapshotItem{{Name: "橘皮乌龙", Spec: "454g 袋装", Qty: "1", Unit: "袋", UnitPrice: "88.00", LineTotal: "88.00"}}}
	pdf, err := renderer.Render(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	png, err := renderer.RenderPNG(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("PR632_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"prepayment.pdf": pdf, "prepayment.png": png} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
