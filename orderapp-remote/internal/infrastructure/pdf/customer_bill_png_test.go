package pdf

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	app "orderapp/internal/application/customerfulfillment"
	"os/exec"
	"testing"
)

func TestRenderCustomerBillPNGUsesSavedSealAndAllPages(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm unavailable")
	}
	lines := make([]app.SettlementLineSummary, 40)
	for i := range lines {
		lines[i] = app.SettlementLineSummary{ChargeType: "shipping", Description: "实际运费", AmountCents: 1110, Detail: &app.BillingLineDetail{OrderNo: "SO-123", ChargeType: "shipping", InformationSource: "出账快照", Shipments: []app.AccountShipment{{TrackingNo: "SF123"}}}}
	}
	bill := app.BillingBill{SettlementResult: app.SettlementResult{SettlementNo: "CB-123", Lines: lines, TotalAmountCents: 44400}, CustomerName: "测试客户"}
	seal := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			seal.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, seal); err != nil {
		t.Fatal(err)
	}
	settings := app.BillingPDFSettings{SealAssetID: 1, Placements: []app.BillingPDFPlacement{{PageNumber: 2, X: 30, Y: 50, Width: 40, Height: 40}}}
	body, err := RenderCustomerBillPNG(context.Background(), bill, settings, b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds().Dy() < 2400 {
		t.Fatalf("image truncated: %v", got.Bounds())
	}
	r, g, bl, _ := got.At(80, 1191+120).RGBA()
	if r < 60000 || g > 1000 || bl > 1000 {
		t.Fatalf("stamp is not at page two position: %d,%d,%d", r, g, bl)
	}
}

func TestRenderCustomerBillPNGShowsInheritedDefaultSeal(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm unavailable")
	}
	seal := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			seal.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, seal); err != nil {
		t.Fatal(err)
	}
	settings := app.ApplyBillingSealDefault(app.BillingPDFSettings{SealAssetID: 9}, false)
	body, err := RenderCustomerBillPNG(context.Background(), app.BillingBill{CustomerName: "测试客户"}, settings, data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(1365, 230).RGBA()
	if r < 60000 || g > 1000 || b > 1000 {
		t.Fatalf("default seal missing at preview position: %d,%d,%d", r, g, b)
	}
}
