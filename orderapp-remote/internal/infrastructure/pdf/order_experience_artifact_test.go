package pdf

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	salesdomain "orderapp/internal/domain/sales"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderExperienceDocumentArtifacts(t *testing.T) {
	assetDir := t.TempDir()
	writeSolidPNG(t, filepath.Join(assetDir, "code.png"), color.RGBA{0, 230, 0, 255}, 160, 160)
	renderer := SalesOrderRenderer{AssetBaseDir: assetDir}
	single := salesdomain.SalesOrderSnapshot{
		OrderID: 1, OrderNo: "SO-20260908-0002", OrderDate: "2026-09-08", CompanyName: "销售单测试公司", CustomerName: "测试客户",
		CustomerCompanyAddress: strings.Repeat("测试地区中文客户地址", 5),
		OrderNote:              "原始订单备注：到店前电话确认\n第二行备注：请保持包装完整", SalesOrderNote: "销售单备注：附送样品",
		Note: "个性化说明：请密封避光保存", PaymentText: "请在付款时备注订单号",
		TotalAmount: "1035.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "1035.00", PaidAmount: "500.00", UnpaidAmount: "535.00",
		PaymentTextBox: salesdomain.SalesOrderLayoutBox{PageNumber: 1}, PaymentCodeBox: salesdomain.SalesOrderLayoutBox{PageNumber: 1},
		PaymentCodes: []salesdomain.SalesOrderAssetRef{{ObjectKey: "code.png", ContentType: "image/png", Label: "测试收款码", Description: "付款后备注订单号"}},
	}
	for _, name := range []string{"墨照啡石", "菠浪清甜", "酒心可可"} {
		single.Items = append(single.Items, salesdomain.SalesOrderSnapshotItem{Name: name, Spec: "454g袋装", Qty: "5", Unit: "袋", QuantityBasis: "sales_spec_count", UnitPrice: "69.00", LineTotal: "345.00"})
	}
	combined := salesdomain.CombinedSalesOrderSnapshot{CombinationKey: "test", CombinedNo: "CSO-TEST", CustomerID: 1, CustomerName: single.CustomerName, CompanyName: single.CompanyName,
		CustomerCompanyAddress: single.CustomerCompanyAddress, TotalAmount: "10350.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "10350.00",
		Note: single.Note, PaymentText: single.PaymentText, PaymentCodes: single.PaymentCodes, PaymentTextBox: single.PaymentTextBox, PaymentCodeBox: single.PaymentCodeBox}
	for i := 1; i <= 10; i++ {
		no := fmt.Sprintf("SO-20260908-%04d", i)
		combined.OrderIDs = append(combined.OrderIDs, int64(i))
		combined.OrderNos = append(combined.OrderNos, no)
		combined.Groups = append(combined.Groups, salesdomain.CombinedSalesOrderGroup{OrderID: int64(i), OrderNo: no, OrderDate: "2026-09-08", Items: single.Items, OrderNote: fmt.Sprintf("第%d单备注：到店前联系\n请保持包装完整", i), SalesOrderNote: single.SalesOrderNote, TotalAmount: single.TotalAmount, Shipping: "0.00", Discount: "0.00", GrandTotal: single.GrandTotal, PaidAmount: single.PaidAmount, UnpaidAmount: single.UnpaidAmount})
	}
	files := map[string][]byte{}
	var err error
	files["single.pdf"], err = renderer.RenderPreview(single)
	if err != nil {
		t.Fatal(err)
	}
	files["single.png"], err = renderer.RenderPNG(single)
	if err != nil {
		t.Fatal(err)
	}
	files["combined.pdf"], err = renderer.RenderCombinedSalesOrderPreview(combined)
	if err != nil {
		t.Fatal(err)
	}
	files["combined.png"], err = renderer.RenderCombinedSalesOrderPNG(combined)
	if err != nil {
		t.Fatal(err)
	}
	if pdfPageCount(files["combined.pdf"]) < 3 {
		t.Fatal("fixture should exercise multiple pages")
	}
	for _, name := range []string{"single.png", "combined.png"} {
		img, err := png.Decode(bytes.NewReader(files[name]))
		if err != nil {
			t.Fatal(err)
		}
		redPixels := 0
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r == 254*257 && g == 226*257 && b == 226*257 {
					redPixels++
					if x < img.Bounds().Dx()/2 {
						t.Fatalf("%s payment background stretches into left half", name)
					}
				}
			}
		}
		if redPixels == 0 {
			t.Fatalf("%s missing payment badge", name)
		}
	}
	if dir := os.Getenv("PR640_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestOrderExperiencePaymentCapacityAndMultipleCodes(t *testing.T) {
	r := SalesOrderRenderer{}
	snapshot := salesdomain.SalesOrderSnapshot{Note: strings.Repeat("不能丢失的说明文字", 1000)}
	if _, err := r.PrepareSalesOrderLayout(snapshot); err == nil {
		t.Fatal("oversized text must return a capacity error instead of clipping")
	}
	snapshot.Note = "付款说明"
	for i := 0; i < 3; i++ {
		snapshot.PaymentCodes = append(snapshot.PaymentCodes, salesdomain.SalesOrderAssetRef{Label: "收款码", Description: "付款请备注"})
	}
	fitted, err := r.PrepareSalesOrderLayout(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	metrics := salesOrderPaymentCodeMetricsForBox(3, fitted.PaymentCodeBox)
	if metrics.ImageSize > metrics.CellHeight-18 {
		t.Fatalf("QR image exceeds cell: %+v", metrics)
	}
}

func TestOrderExperiencePNGPaymentDescriptionsFitCells(t *testing.T) {
	metrics := salesOrderPNGPaymentCodeMetricsForText(3, 425, 900, 180)
	if metrics.ImageSize+180 > metrics.CellH {
		t.Fatalf("description would overlap the next code: %+v", metrics)
	}
}

func TestOrderExperienceLegacyPaymentPositionStaysOnPaper(t *testing.T) {
	r := SalesOrderRenderer{}
	snapshot, err := r.PrepareSalesOrderLayout(salesdomain.SalesOrderSnapshot{PaymentText: "微信", Note: "必须完整显示的说明", PaymentTextBox: salesdomain.SalesOrderLayoutBox{XMM: 232, YMM: 143, WidthMM: 81, HeightMM: 136}})
	if err != nil {
		t.Fatal(err)
	}
	box := snapshot.PaymentTextBox
	if box.XMM < 8 || box.XMM+box.WidthMM > 202.01 {
		t.Fatalf("legacy payment text is outside paper: %+v", box)
	}
}
