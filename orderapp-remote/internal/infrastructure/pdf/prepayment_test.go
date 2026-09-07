package pdf

import (
	"bytes"
	"image/color"
	"image/png"
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

func TestSalesOrderPaymentRowsShowPaidAndUnpaidWithoutPrepayment(t *testing.T) {
	tests := []struct {
		name     string
		snapshot salesdomain.SalesOrderSnapshot
		label    string
		value    string
		tone     string
	}{
		{name: "fully paid", snapshot: salesdomain.SalesOrderSnapshot{PaidAmount: "100.00", UnpaidAmount: "0.00"}, label: "已付金额", value: "100.00", tone: "paid"},
		{name: "fully unpaid", snapshot: salesdomain.SalesOrderSnapshot{PaidAmount: "0.00", UnpaidAmount: "100.00"}, label: "未付金额", value: "100.00", tone: "unpaid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, row := range salesOrderFinancialRows(tc.snapshot) {
				if row.Label == tc.label && row.Value == tc.value && row.Tone == tc.tone {
					return
				}
			}
			t.Fatalf("missing %s row in %+v", tc.tone, salesOrderFinancialRows(tc.snapshot))
		})
	}
}

func TestSalesOrderPaymentStatePNGColors(t *testing.T) {
	tests := []struct {
		name     string
		paid     string
		unpaid   string
		wantFill color.RGBA
	}{
		{name: "paid", paid: "100.00", unpaid: "0.00", wantFill: color.RGBA{R: 220, G: 252, B: 231, A: 255}},
		{name: "unpaid", paid: "0.00", unpaid: "100.00", wantFill: color.RGBA{R: 254, G: 226, B: 226, A: 255}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := salesdomain.SalesOrderSnapshot{
				OrderID: 1, OrderNo: "PR635-付款状态示例", OrderDate: "2026-09-07", CustomerName: "验收客户", CompanyName: "浅焙作坊咖啡",
				TotalAmount: "100.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "100.00", PaidAmount: tc.paid, UnpaidAmount: tc.unpaid,
				Items: []salesdomain.SalesOrderSnapshotItem{{Name: "销售商品", Spec: "1袋", Qty: "1", Unit: "袋", UnitPrice: "100.00", LineTotal: "100.00"}},
			}
			renderer := SalesOrderRenderer{}
			pdfBody, err := renderer.Render(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			body, err := renderer.RenderPNG(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if dir := os.Getenv("PR635_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string][]byte{tc.name + ".pdf": pdfBody, tc.name + ".png": body} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
			img, err := png.Decode(bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			bounds := img.Bounds()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) == tc.wantFill {
						return
					}
				}
			}
			t.Fatalf("rendered PNG does not contain expected %s fill color", tc.name)
		})
	}
}

func TestCombinedSalesOrderPaymentStateColors(t *testing.T) {
	item := salesdomain.SalesOrderSnapshotItem{Name: "销售商品", Spec: "1袋", Qty: "1", Unit: "袋", UnitPrice: "100.00", LineTotal: "100.00"}
	snapshot := salesdomain.CombinedSalesOrderSnapshot{
		CombinationKey: "1,2", CombinedNo: "PR635-COMBINED", CustomerID: 1, CustomerName: "验收客户", CompanyName: "浅焙作坊咖啡",
		OrderIDs: []int64{1, 2}, OrderNos: []string{"PR635-PAID", "PR635-UNPAID"}, TotalAmount: "200.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "200.00",
		Groups: []salesdomain.CombinedSalesOrderGroup{
			{OrderID: 1, OrderNo: "PR635-PAID", OrderDate: "2026-09-07", Items: []salesdomain.SalesOrderSnapshotItem{item}, TotalAmount: "100.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "100.00", PaidAmount: "100.00", UnpaidAmount: "0.00"},
			{OrderID: 2, OrderNo: "PR635-UNPAID", OrderDate: "2026-09-07", Items: []salesdomain.SalesOrderSnapshotItem{item}, TotalAmount: "100.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "100.00", PaidAmount: "0.00", UnpaidAmount: "100.00"},
		},
	}
	renderer := SalesOrderRenderer{}
	pdfBody, err := renderer.RenderCombinedSalesOrder(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	pngBody, err := renderer.RenderCombinedSalesOrderPNG(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(pngBody))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]color.RGBA{
		"paid":   {R: 220, G: 252, B: 231, A: 255},
		"unpaid": {R: 254, G: 226, B: 226, A: 255},
	} {
		found := false
		bounds := img.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y && !found; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) == want {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("combined sales order PNG missing %s fill color", name)
		}
	}
	if dir := os.Getenv("PR635_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"combined.pdf": pdfBody, "combined.png": pngBody} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
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
