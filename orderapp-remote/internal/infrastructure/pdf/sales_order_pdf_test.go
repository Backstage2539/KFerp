package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	salesdomain "orderapp/internal/domain/sales"
	"os"
	"path/filepath"
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

func TestRenderSalesOrderPDFEmbedsPaymentCodeAndSealImages(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "sales-order", "payment", "wechat.png"))
	writeTestPNG(t, filepath.Join(dir, "sales-order", "seal", "seal.png"))

	renderer := SalesOrderRenderer{AssetBaseDir: dir}
	b, err := renderer.Render(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260430-0008",
		OrderDate:    "2026-04-30",
		CustomerName: "某某咖啡馆",
		CompanyName:  "浅焙作坊咖啡",
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		TotalAmount: "134.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "134.00",
		PaymentCodes: []salesdomain.SalesOrderAssetRef{{
			Label: "微信", Description: "扫码付款", ObjectKey: "sales-order/payment/wechat.png", ContentType: "image/png",
		}},
		Seal: &salesdomain.SalesOrderAssetRef{
			Label: "公章", ObjectKey: "sales-order/seal/seal.png", ContentType: "image/png",
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := bytes.Count(b, []byte("/Subtype /Image")); got < 2 {
		t.Fatalf("embedded image count = %d, want >= 2", got)
	}
}

func TestSalesOrderPDFMultilineTextAndSealPositionHelpers(t *testing.T) {
	lines := salesOrderMultilineLines("说明", "第一行\r\n第二行\n\n第四行")
	wantLines := []string{"说明：第一行", "第二行", "", "第四行"}
	if len(lines) != len(wantLines) {
		t.Fatalf("lines=%+v want %+v", lines, wantLines)
	}
	for i := range wantLines {
		if lines[i] != wantLines[i] {
			t.Fatalf("lines[%d]=%q want %q", i, lines[i], wantLines[i])
		}
	}

	pos := salesOrderSealPosition(0, 0, 0)
	if pos.XMM <= 16 || pos.YMM <= 14 || pos.WidthMM <= 0 {
		t.Fatalf("default seal position should stamp near company header, got %+v", pos)
	}
	custom := salesOrderSealPosition(42, 21, 38)
	if custom.XMM != 42 || custom.YMM != 21 || custom.WidthMM != 38 {
		t.Fatalf("custom seal position = %+v", custom)
	}
}

func TestSalesOrderPDFSealImageFitPreservesAspectRatio(t *testing.T) {
	box := salesOrderSealPosition(32, 22, 42)
	got := fitSalesOrderImageInBox(100, 100, box.XMM, box.YMM, box.WidthMM, box.HeightMM)
	if math.Abs(got.WidthMM-box.HeightMM) > 0.001 || math.Abs(got.HeightMM-box.HeightMM) > 0.001 {
		t.Fatalf("square seal should fit by height without stretching, got %+v in %+v", got, box)
	}
	wantX := box.XMM + (box.WidthMM-box.HeightMM)/2
	if math.Abs(got.XMM-wantX) > 0.001 || math.Abs(got.YMM-box.YMM) > 0.001 {
		t.Fatalf("square seal should be centered in stamp box, got %+v want x=%v y=%v", got, wantX, box.YMM)
	}

	wide := fitSalesOrderImageInBox(200, 100, 10, 20, 40, 20)
	if math.Abs(wide.WidthMM-40) > 0.001 || math.Abs(wide.HeightMM-20) > 0.001 {
		t.Fatalf("wide image should fill matching wide box without stretching, got %+v", wide)
	}
}

func TestSalesOrderPDFPaymentCodeSizingAdaptsToCount(t *testing.T) {
	single := salesOrderPaymentCodeMetrics(1)
	multiple := salesOrderPaymentCodeMetrics(2)
	if single.ImageSize <= multiple.ImageSize {
		t.Fatalf("single payment code image should be larger than multiple layout: single=%+v multiple=%+v", single, multiple)
	}
	if !multiple.Stacked {
		t.Fatalf("multiple payment codes should stack vertically to fill the payment area: %+v", multiple)
	}
}

func TestSalesOrderImageTypeDetectsJPEGMagicForUnknownExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wechat.pic")
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := salesOrderImageType("image/pict", path); got != "JPG" {
		t.Fatalf("salesOrderImageType(.pic JPEG) = %q, want JPG", got)
	}
}

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	base := color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff}
	if filepath.Base(path) == "seal.png" {
		base = color.RGBA{R: 0xcc, G: 0x44, B: 0x22, A: 0xff}
	}
	img.Set(0, 0, base)
	img.Set(1, 0, color.RGBA{R: base.R + 0x11, G: base.G + 0x11, B: base.B + 0x11, A: 0xff})
	img.Set(0, 1, color.RGBA{R: base.R + 0x22, G: base.G + 0x22, B: base.B + 0x22, A: 0xff})
	img.Set(1, 1, color.RGBA{R: base.R + 0x33, G: base.G + 0x33, B: base.B + 0x33, A: 0xff})
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
