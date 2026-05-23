package pdf

import (
	"bytes"
	salesdomain "orderapp/internal/domain/sales"
	"path/filepath"
	"strings"
	"testing"
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
