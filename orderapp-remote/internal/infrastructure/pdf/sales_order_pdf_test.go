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

	"golang.org/x/image/font/opentype"
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

func TestRenderSalesOrderPNG(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.RenderPNG(salesdomain.SalesOrderSnapshot{
		OrderID:                1,
		OrderNo:                "SO-20260430-0008",
		OrderDate:              "2026-04-30",
		CustomerName:           "某某咖啡馆",
		CompanyName:            "浅焙作坊咖啡",
		CustomerCompanyName:    "某某咖啡贸易公司",
		CustomerCompanyAddress: "上海市徐汇区长地址一二三四五六七八九十",
		CustomerCompanyPhone:   "021-12345678",
		PaymentText:            "微信或对公转账",
		BankAccountName:        "孟连口加农业科技有限公司",
		BankName:               "中国农业银行孟连支行",
		BankAccountNo:          "6222000000000000",
		TaxpayerID:             "91530827MACGJ29D6J",
		CompanyAddress:         "云南省普洱市孟连县",
		Note:                   "第一行\n第二行",
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		TotalAmount: "134.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "134.00",
	})
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("PNG missing header: %q", b[:8])
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if img.Bounds().Dx() < 1000 || img.Bounds().Dy() < 1400 {
		t.Fatalf("PNG bounds = %v, want A4-like document image", img.Bounds())
	}
}

func TestSalesOrderPNGTextMetricsFitConfiguredLineHeights(t *testing.T) {
	renderer := SalesOrderRenderer{}
	fontPath, err := renderer.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	fontBytes, err := os.ReadFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	parsedFont, err := opentype.Parse(fontBytes)
	if err != nil {
		t.Fatal(err)
	}
	canvas := salesOrderPNGCanvas{font: parsedFont}
	for _, tc := range []struct {
		name       string
		size       float64
		lineHeight int
	}{
		{name: "meta", size: 20, lineHeight: 28},
		{name: "text block", size: 20, lineHeight: 30},
		{name: "payment code description", size: 16, lineHeight: 24},
	} {
		height := canvas.face(tc.size).Metrics().Height.Ceil()
		if height > tc.lineHeight {
			t.Fatalf("%s font height %d exceeds line height %d at size %.0f", tc.name, height, tc.lineHeight, tc.size)
		}
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
	if pos.XMM <= 16 || pos.YMM < 4 || pos.WidthMM <= 0 {
		t.Fatalf("default seal position should stamp near company header without leaving the page, got %+v", pos)
	}
	if math.Abs(pos.HeightMM-pos.WidthMM) > 0.001 {
		t.Fatalf("default seal box should keep a round seal square, got %+v", pos)
	}
	custom := salesOrderSealPosition(42, 21, 38)
	if custom.XMM != 42 || custom.YMM != 21 || custom.WidthMM != 38 || custom.HeightMM != 38 {
		t.Fatalf("custom seal position = %+v", custom)
	}
	oldDefault := salesOrderSealPosition(32, 22, 42)
	if oldDefault.XMM != salesOrderSealDefaultXMM || oldDefault.YMM != salesOrderSealDefaultYMM || oldDefault.WidthMM != salesOrderSealDefaultWidthMM {
		t.Fatalf("legacy default seal should be normalized to current default: pos=%+v", oldDefault)
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
	if single.ImageSize < 62 {
		t.Fatalf("single payment code image too small for scanning: %+v", single)
	}
	if multiple.ImageSize < 50 {
		t.Fatalf("multiple payment code image too small for scanning: %+v", multiple)
	}
	if !multiple.Stacked {
		t.Fatalf("multiple payment codes should stack vertically to fill the payment area: %+v", multiple)
	}
}

func TestRenderSalesOrderPNGUsesHighResolutionCanvasAndLargePaymentCode(t *testing.T) {
	dir := t.TempDir()
	writeSolidPNG(t, filepath.Join(dir, "sales-order", "payment", "wechat.png"), color.RGBA{G: 0xf0, A: 0xff}, 64, 64)

	renderer := SalesOrderRenderer{AssetBaseDir: dir}
	b, err := renderer.RenderPNG(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260430-0008",
		OrderDate:    "2026-04-30",
		CustomerName: "某某咖啡馆",
		CompanyName:  "浅焙作坊咖啡",
		PaymentText:  "微信",
		Note:         "请密封避光保存",
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		TotalAmount: "134.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "134.00",
		PaymentCodes: []salesdomain.SalesOrderAssetRef{{
			Label: "微信", ObjectKey: "sales-order/payment/wechat.png", ContentType: "image/png",
		}},
	})
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if img.Bounds().Dx() < 2480 || img.Bounds().Dy() < 3508 {
		t.Fatalf("PNG bounds = %v, want 2x A4-like export for WeChat sharing", img.Bounds())
	}
	codeBounds := dominantGreenBounds(img)
	if codeBounds.Empty() || codeBounds.Dx() < 620 || codeBounds.Dy() < 620 {
		t.Fatalf("payment code bounds = %v, want at least 620px square for scanning", codeBounds)
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

func writeSolidPNG(t *testing.T, path string, col color.RGBA, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, col)
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func dominantGreenBounds(img image.Image) image.Rectangle {
	minX, minY := img.Bounds().Max.X, img.Bounds().Max.Y
	maxX, maxY := img.Bounds().Min.X, img.Bounds().Min.Y
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0x8000 && g > 0xc000 && r < 0x4000 && b < 0x4000 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x+1 > maxX {
					maxX = x + 1
				}
				if y+1 > maxY {
					maxY = y + 1
				}
			}
		}
	}
	if maxX <= minX || maxY <= minY {
		return image.Rectangle{}
	}
	return image.Rect(minX, minY, maxX, maxY)
}
