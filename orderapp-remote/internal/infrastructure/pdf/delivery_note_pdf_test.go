package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	salesdomain "orderapp/internal/domain/sales"
)

func TestRenderDeliveryNotePDFEmbedsConfiguredSeal(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "sales-order", "seal", "seal.png"))

	renderer := DeliveryNoteRenderer{AssetBaseDir: dir}
	b, err := renderer.Render(salesdomain.DeliveryNoteSnapshot{
		OrderID:         1,
		OrderNo:         "SO-20260503-0001",
		DeliveryNoteNo:  "DN-SO-20260503-0001",
		PostingDate:     "2026-05-03",
		CompanyName:     "棵凡咖啡",
		CustomerName:    "测试客户",
		SourceWarehouse: "finished_goods",
		DeliveryMethod:  "顺丰",
		TrackingNo:      "SF-DRAWER-001",
		Items: []salesdomain.DeliveryNoteSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", Warehouse: "finished_goods",
		}},
		Seal: &salesdomain.SalesOrderAssetRef{
			Label: "公章", ObjectKey: "sales-order/seal/seal.png", ContentType: "image/png", XMM: 48, YMM: 9, WidthMM: 30,
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("PDF missing header: %q", b[:5])
	}
	if got := bytes.Count(b, []byte("/Subtype /Image")); got < 1 {
		t.Fatalf("embedded image count = %d, want >= 1", got)
	}
}

func TestDeliveryNoteHeaderMetaRowsShowsDocumentOrderAndPostingDates(t *testing.T) {
	rows := deliveryNoteHeaderMetaRows(salesdomain.DeliveryNoteSnapshot{
		OrderNo:        "SO-20260523-0001",
		DeliveryNoteNo: "DN-SO-20260523-0001",
		DocumentDate:   "2026-05-23",
		OrderDate:      "2026-05-20",
		PostingDate:    "2026-05-21",
		CustomerName:   "测试客户",
		TrackingNo:     "SF123",
	})
	flat := strings.Join(append(append(rows[0], rows[1]...), rows[2]...), "\n")
	for _, want := range []string{"单据日期：2026-05-23", "订单日期：2026-05-20", "出库日期：2026-05-21"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("delivery note meta rows missing %q: %#v", want, rows)
		}
	}
}

func TestRenderDeliveryNotePNGProducesHighResolutionLongImageWithoutClipping(t *testing.T) {
	items := make([]salesdomain.DeliveryNoteSnapshotItem, 0, 36)
	for i := 0; i < 36; i++ {
		items = append(items, salesdomain.DeliveryNoteSnapshotItem{
			Name:          "金色山脉精品咖啡豆",
			Spec:          "227g 袋装",
			Qty:           "12",
			Unit:          "袋",
			Warehouse:     "finished_goods",
			WarehouseName: "成品仓",
			Note:          "发货前再次检查包装、烘焙日期与客户标签，破损包装不得装箱",
		})
	}
	snapshot := salesdomain.DeliveryNoteSnapshot{
		OrderID:                1,
		OrderNo:                "SO-20260801-0001",
		DeliveryNoteNo:         "DN-SO-20260801-0001",
		DocumentDate:           "2026-08-01",
		OrderDate:              "2026-07-31",
		PostingDate:            "2026-08-01",
		CompanyName:            "棵凡咖啡",
		CustomerName:           "测试客户",
		CustomerCompanyName:    "测试客户咖啡有限公司",
		CustomerCompanyAddress: "云南省普洱市孟连县测试路一百二十三号测试园区二栋",
		CustomerCompanyPhone:   "0879-1234567",
		ReceiverName:           "张三",
		ReceiverPhone:          "13800000000",
		ReceiverAddress:        "上海市徐汇区测试路一百二十三号测试园区二栋一单元八零八室",
		SourceWarehouse:        "finished_goods",
		SourceWarehouseName:    "成品仓",
		DeliveryMethod:         "顺丰标快",
		TrackingNo:             "SF1234567890 / SF0987654321",
		Note:                   "整单备注：到货后请先核对箱数，再检查每袋标签；如有异常请拍照联系销售人员。",
		Items:                  items,
	}

	b, err := (DeliveryNoteRenderer{}).RenderPNG(snapshot)
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("PNG missing signature: %q", b[:8])
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != salesOrderPNGWidth {
		t.Fatalf("PNG width = %d, want %d", bounds.Dx(), salesOrderPNGWidth)
	}
	if bounds.Dy() <= salesOrderPNGHeight {
		t.Fatalf("long PNG height = %d, want > %d", bounds.Dy(), salesOrderPNGHeight)
	}
	lastInkY := lastNonWhitePixelRow(img)
	if lastInkY < salesOrderPNGHeight || lastInkY >= bounds.Max.Y-40 {
		t.Fatalf("last rendered row = %d for height %d; content should extend beyond A4 and retain bottom padding", lastInkY, bounds.Dy())
	}
	if output := strings.TrimSpace(os.Getenv("DELIVERY_NOTE_PNG_OUTPUT")); output != "" {
		if err := os.WriteFile(output, b, 0644); err != nil {
			t.Fatalf("write representative PNG: %v", err)
		}
		t.Logf("representative PNG: %s (%dx%d)", output, bounds.Dx(), bounds.Dy())
	}
}

func lastNonWhitePixelRow(img image.Image) int {
	bounds := img.Bounds()
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) != white {
				return y
			}
		}
	}
	return -1
}
