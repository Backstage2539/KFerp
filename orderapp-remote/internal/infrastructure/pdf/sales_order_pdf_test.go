package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	salesdomain "orderapp/internal/domain/sales"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jung-kurt/gofpdf"
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

func TestSalesOrderHeaderMetaRowsShowsDocumentAndOrderDates(t *testing.T) {
	rows := salesOrderHeaderMetaRows(salesdomain.SalesOrderSnapshot{
		OrderNo:      "SO-20260523-0001",
		DocumentDate: "2026-05-23",
		OrderDate:    "2026-05-20",
		CustomerName: "某某咖啡馆",
	})
	flat := strings.Join(append(rows[0], rows[1]...), "\n")
	for _, want := range []string{"单据日期：2026-05-23", "订单日期：2026-05-20"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("sales order meta rows missing %q: %#v", want, rows)
		}
	}
}

func TestRenderSalesOrderPDFWrapsUTF8PaymentTextWithoutPanic(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.Render(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260522-0001",
		OrderDate:    "2026-05-22",
		CustomerName: "某某咖啡馆",
		CompanyName:  "浅焙作坊咖啡",
		PaymentText:  strings.Repeat("微信支付支付宝转账对公账户", 8),
		Note:         strings.Repeat("请密封避光保存并尽快使用", 8),
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		TotalAmount: "134.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "134.00",
		PaymentTextBox: salesdomain.SalesOrderLayoutBox{
			XMM: 16, YMM: 118, WidthMM: 62, HeightMM: 78,
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("PDF missing header: %q", b[:5])
	}
}

func TestSalesOrderItemRowsWrapLongNamesAndNotes(t *testing.T) {
	renderer := SalesOrderRenderer{}
	fontPath, err := renderer.resolveFontPath()
	if err != nil {
		t.Fatal(err)
	}
	pdf := newSalesOrderTestPDF(t, fontPath)
	widths := []float64{46, 22, 22, 26, 26, 34}
	shortItem := salesdomain.SalesOrderSnapshotItem{Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00"}
	longItem := salesdomain.SalesOrderSnapshotItem{
		Name:      "芬纳-曲奇定制（20%乌干达，15%云南厌氧日晒，65%云南水洗）-中深烘",
		Spec:      "1000g",
		Qty:       "20",
		Unit:      "件",
		UnitPrice: "117.00",
		LineTotal: "2100.00",
		Note:      "客户指定包装和发货说明也需要换行展示",
	}
	if got := salesOrderItemRowHeight(pdf, shortItem, widths, 6); got <= 6 || got > 13 {
		t.Fatalf("short item row height = %.2f, want one-line row with padding", got)
	}
	if got, short := salesOrderItemRowHeight(pdf, longItem, widths, 6), salesOrderItemRowHeight(pdf, shortItem, widths, 6); got <= short {
		t.Fatalf("long item row height = %.2f, short = %.2f; long product names must wrap and grow the row", got, short)
	}
}

func TestSalesOrderItemRowsShowSpecPerUnitDiscountAndFinalNote(t *testing.T) {
	item := salesdomain.SalesOrderSnapshotItem{
		Name:           "芬纳-曲奇定制",
		Spec:           "1000g",
		Qty:            "1",
		Unit:           "件",
		UnitPrice:      "115.00",
		LineTotal:      "93.35",
		DiscountAmount: "28.00",
		Note:           "20%乌干达，15%云南厌氧日晒，65%云南水洗",
	}
	wantHeaders := []string{"商品", "规格", "数量", "单价", "优惠折扣", "总价", "备注"}
	if got := salesOrderItemHeaders(true); strings.Join(got, "|") != strings.Join(wantHeaders, "|") {
		t.Fatalf("salesOrderItemHeaders()=%v want %v", got, wantHeaders)
	}
	wantCells := []string{"芬纳-曲奇定制", "1000g/件", "1件", "115.00", "￥-28元", "93.35", "20%乌干达，15%云南厌氧日晒，65%云南水洗"}
	if got := salesOrderItemCells(item, true); strings.Join(got, "|") != strings.Join(wantCells, "|") {
		t.Fatalf("salesOrderItemCells()=%v want %v", got, wantCells)
	}
	noDiscountHeaders := []string{"商品", "规格", "数量", "单价", "总价", "备注"}
	if got := salesOrderItemHeaders(false); strings.Join(got, "|") != strings.Join(noDiscountHeaders, "|") {
		t.Fatalf("salesOrderItemHeaders(false)=%v want %v", got, noDiscountHeaders)
	}
	noDiscountCells := []string{"芬纳-曲奇定制", "1000g/件", "1件", "115.00", "93.35", "20%乌干达，15%云南厌氧日晒，65%云南水洗"}
	if got := salesOrderItemCells(item, false); strings.Join(got, "|") != strings.Join(noDiscountCells, "|") {
		t.Fatalf("salesOrderItemCells(no discount)=%v want %v", got, noDiscountCells)
	}
}

func TestSalesOrderFinancialRowsHideDiscountTotalWhenNoDiscount(t *testing.T) {
	rows := salesOrderFinancialRows(salesdomain.SalesOrderSnapshot{
		TotalAmount:    "2455.00",
		Shipping:       "169.00",
		Discount:       "261.65",
		GrandTotal:     "2362.35",
		ExpressFee:     "顺丰到付前电话确认",
		SalesOrderNote: "客户要求周五前发出",
		Items: []salesdomain.SalesOrderSnapshotItem{
			{Name: "芬纳定制-红酒日晒-中深烘", Note: "磨粉"},
			{Name: "芬纳-曲奇定制", Note: "贴标"},
		},
		DiscountBreakdowns: []salesdomain.SalesOrderDiscountBreakdown{
			{Type: "unit_amount", Amount: "200.00"},
			{Type: "percent", Amount: "61.65"},
		},
	})
	want := []salesOrderFinancialRow{
		{Label: "快递费备注", Value: "顺丰到付前电话确认"},
		{Label: "订单明细备注", Value: "芬纳定制-红酒日晒-中深烘：磨粉；芬纳-曲奇定制：贴标"},
		{Label: "销售单备注", Value: "客户要求周五前发出"},
		{Cells: []string{"商品合计： 2455.00", "优惠合计： 261.65", "运费： 169.00", "应收： 2362.35"}, Bold: true},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows=%+v want %+v", rows, want)
	}
	for i := range want {
		if rows[i].Label != want[i].Label || rows[i].Value != want[i].Value || rows[i].Bold != want[i].Bold || strings.Join(rows[i].Cells, "|") != strings.Join(want[i].Cells, "|") {
			t.Fatalf("rows[%d]=%+v want %+v", i, rows[i], want[i])
		}
	}

	rows = salesOrderFinancialRows(salesdomain.SalesOrderSnapshot{
		TotalAmount:    "134.00",
		Shipping:       "0.00",
		Discount:       "0.00",
		GrandTotal:     "134.00",
		SalesOrderNote: "无优惠订单",
	})
	want = []salesOrderFinancialRow{
		{Label: "销售单备注", Value: "无优惠订单"},
		{Cells: []string{"商品合计： 134.00", "运费： 0.00", "应收： 134.00"}, Bold: true},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows without discount=%+v want %+v", rows, want)
	}
	for i := range want {
		if rows[i].Label != want[i].Label || rows[i].Value != want[i].Value || rows[i].Bold != want[i].Bold || strings.Join(rows[i].Cells, "|") != strings.Join(want[i].Cells, "|") {
			t.Fatalf("rows without discount[%d]=%+v want %+v", i, rows[i], want[i])
		}
	}
}

func TestCombinedSalesOrderHeaderMetaRowsShowCustomerDateAndRelatedOrdersOnly(t *testing.T) {
	snapshot := salesdomain.CombinedSalesOrderSnapshot{
		CombinedNo:             "CSO-SO-20260509-0004-SO-20260523-0003",
		CustomerName:           "岩师傅",
		CustomerCompanyName:    "岩师傅咖啡店",
		CustomerCompanyPhone:   "13900000000",
		CustomerCompanyAddress: "上海市徐汇区咖啡路 100 号",
		OrderNos:               []string{"SO-20260509-0004", "SO-20260523-0003"},
		CustomerID:             1,
		CompanyName:            "孟连口加农业科技有限公司",
		CombinationKey:         "1,2",
		OrderIDs:               []int64{1, 2},
		TotalAmount:            "960.00",
		Shipping:               "20.00",
		Discount:               "0.00",
		GrandTotal:             "980.00",
		Groups: func() []salesdomain.CombinedSalesOrderGroup {
			groups := testCombinedSalesOrderGroups(2)
			groups[1].DocumentDate = "2026-05-23"
			return groups
		}(),
	}
	rows := combinedSalesOrderHeaderMetaRows(snapshot)
	flat := strings.Join(flattenSalesOrderMetaRows(rows), "\n")
	for _, want := range []string{
		"客户：岩师傅",
		"客户公司：岩师傅咖啡店",
		"联系电话：13900000000",
		"单据日期：2026-05-23",
		"关联订单：SO-20260509-0004、SO-20260523-0003",
		"公司地址：上海市徐汇区咖啡路 100 号",
	} {
		if !strings.Contains(flat, want) {
			t.Fatalf("combined sales order header missing %q in %#v", want, rows)
		}
	}
	for _, forbidden := range []string{"组合单", "订单数", "订单日期："} {
		if strings.Contains(flat, forbidden) {
			t.Fatalf("combined sales order header should not expose %q: %#v", forbidden, rows)
		}
	}
}

func TestCombinedSalesOrderGroupHeaderShowsOrderDateInsteadOfOrderNo(t *testing.T) {
	group := salesdomain.CombinedSalesOrderGroup{
		OrderNo:      "SO-20260509-0004",
		DocumentDate: "2026-05-09",
		OrderDate:    "2026-05-07",
	}
	got := combinedSalesOrderGroupHeaderText(group)
	if got != "订单日期 2026-05-07" {
		t.Fatalf("combined group header = %q, want order date", got)
	}
	for _, forbidden := range []string{"SO-20260509-0004", "单据日期"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("combined group header should not repeat %q: %q", forbidden, got)
		}
	}
}

func TestRenderSalesOrderPDFAddsPaymentContinuationPageWhenPaymentLayoutExceedsPage(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.Render(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260524-0001",
		DocumentDate: "2026-05-24",
		OrderDate:    "2026-05-24",
		CustomerName: "岩师傅",
		CompanyName:  "孟连口加农业科技有限公司",
		PaymentText:  "微信或对公转账",
		Note:         "请在付款后备注订单号",
		Items: []salesdomain.SalesOrderSnapshotItem{{
			Name: "兰卡拼配", Spec: "1000g", Qty: "1", Unit: "件", UnitPrice: "82.00", LineTotal: "82.00",
		}},
		TotalAmount: "82.00",
		Shipping:    "0.00",
		Discount:    "0.00",
		GrandTotal:  "82.00",
		PaymentTextBox: salesdomain.SalesOrderLayoutBox{
			XMM: 16, YMM: 260, WidthMM: 104, HeightMM: 54,
		},
		PaymentCodeBox: salesdomain.SalesOrderLayoutBox{
			XMM: 126, YMM: 260, WidthMM: 72, HeightMM: 90,
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := pdfPageCount(b); got < 2 {
		t.Fatalf("PDF page count = %d, want a continuation page for out-of-page payment layout", got)
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

func TestRenderSalesOrderPNGUsesLongImageForLongOrders(t *testing.T) {
	items := make([]salesdomain.SalesOrderSnapshotItem, 0, 34)
	for i := 0; i < 34; i++ {
		items = append(items, salesdomain.SalesOrderSnapshotItem{
			Name:      "兰卡拼配熟豆长商品名需要在图片中完整展示",
			Spec:      "1000g",
			Qty:       "1",
			Unit:      "件",
			UnitPrice: "82.00",
			LineTotal: "82.00",
			Note:      "第" + strconv.Itoa(i+1) + "行备注",
		})
	}
	renderer := SalesOrderRenderer{}
	b, err := renderer.RenderPNG(salesdomain.SalesOrderSnapshot{
		OrderID:      1,
		OrderNo:      "SO-20260524-0002",
		DocumentDate: "2026-05-24",
		OrderDate:    "2026-05-24",
		CustomerName: "岩师傅",
		CompanyName:  "孟连口加农业科技有限公司",
		PaymentText:  "微信或对公转账",
		Note:         "长图不得分页或裁切说明与收款码",
		Items:        items,
		TotalAmount:  "2788.00",
		Shipping:     "0.00",
		Discount:     "0.00",
		GrandTotal:   "2788.00",
	})
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if img.Bounds().Dy() <= salesOrderPNGHeight {
		t.Fatalf("PNG height = %d, want long image taller than one A4 design page %d", img.Bounds().Dy(), salesOrderPNGHeight)
	}
}

func TestRenderCombinedSalesOrderPNGUsesLongImageForManyGroups(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.RenderCombinedSalesOrderPNG(salesdomain.CombinedSalesOrderSnapshot{
		CombinationKey: "1,2,3,4,5,6",
		CombinedNo:     "CSO-20260524-0001",
		CustomerID:     1,
		CustomerName:   "岩师傅",
		CompanyName:    "孟连口加农业科技有限公司",
		OrderIDs:       []int64{1, 2, 3, 4, 5, 6},
		OrderNos:       []string{"SO-20260524-0001", "SO-20260524-0002", "SO-20260524-0003", "SO-20260524-0004", "SO-20260524-0005", "SO-20260524-0006"},
		Groups:         testCombinedSalesOrderGroups(6),
		TotalAmount:    "492.00",
		Shipping:       "0.00",
		Discount:       "0.00",
		GrandTotal:     "492.00",
		PaymentText:    "微信或对公转账",
		Note:           "组合销售单图片用长图承载，不按 PDF 分页",
	})
	if err != nil {
		t.Fatalf("RenderCombinedSalesOrderPNG() error = %v", err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if img.Bounds().Dy() <= salesOrderPNGHeight {
		t.Fatalf("combined PNG height = %d, want long image taller than one A4 design page %d", img.Bounds().Dy(), salesOrderPNGHeight)
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

func TestSalesOrderPaymentLayoutDefaultsPutCodeOnFirstPageRight(t *testing.T) {
	textBox, codeBox := salesOrderPaymentLayoutBoxes(salesdomain.SalesOrderSnapshot{})
	if textBox.XMM < 15 || textBox.YMM > 130 || textBox.WidthMM < 95 || textBox.HeightMM < 60 {
		t.Fatalf("default text box should stay on page 1 left area, got %+v", textBox)
	}
	if codeBox.XMM < 120 || codeBox.YMM > 120 || codeBox.WidthMM < 70 || codeBox.HeightMM < 105 {
		t.Fatalf("default payment code box should stay on page 1 right area and be large, got %+v", codeBox)
	}
	if codeBox.XMM+codeBox.WidthMM > 200 || codeBox.YMM+codeBox.HeightMM > 260 {
		t.Fatalf("default payment code box should fit on the first A4 page, got %+v", codeBox)
	}
}

func newSalesOrderTestPDF(t *testing.T, fontPath string) *gofpdf.Fpdf {
	t.Helper()
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "mm",
		SizeStr:        "A4",
		FontDirStr:     filepath.Dir(fontPath),
	})
	pdf.SetMargins(16, 14, 16)
	pdf.AddUTF8Font("noto", "", filepath.Base(fontPath))
	pdf.AddUTF8Font("noto", "B", filepath.Base(fontPath))
	pdf.AddPage()
	pdf.SetFont("noto", "", 10)
	return pdf
}

func TestSalesOrderPaymentLayoutUsesConfiguredTextAndCodeBoxes(t *testing.T) {
	textBox, codeBox := salesOrderPaymentLayoutBoxes(salesdomain.SalesOrderSnapshot{
		PaymentTextBox: salesdomain.SalesOrderLayoutBox{XMM: 18, YMM: 142, WidthMM: 98, HeightMM: 54},
		PaymentCodeBox: salesdomain.SalesOrderLayoutBox{XMM: 126, YMM: 104, WidthMM: 76, HeightMM: 126},
	})
	if textBox != (salesdomain.SalesOrderLayoutBox{XMM: 18, YMM: 142, WidthMM: 98, HeightMM: 54}) {
		t.Fatalf("text layout = %+v", textBox)
	}
	if codeBox != (salesdomain.SalesOrderLayoutBox{XMM: 126, YMM: 104, WidthMM: 76, HeightMM: 126}) {
		t.Fatalf("code layout = %+v", codeBox)
	}

	metrics := salesOrderPaymentCodeMetricsForBox(1, codeBox)
	if metrics.ImageSize < 72 || metrics.CellWidth != codeBox.WidthMM {
		t.Fatalf("configured code metrics should use the bigger editable box, metrics=%+v box=%+v", metrics, codeBox)
	}
}

func TestSalesOrderPaymentTextSectionsPrioritizePersonalNote(t *testing.T) {
	sections := salesOrderPaymentTextSections(salesdomain.SalesOrderSnapshot{
		PaymentText:     "微信支付",
		Note:            "请送货前联系老板",
		BankAccountName: "棵凡咖啡",
		BankName:        "农业银行",
		BankAccountNo:   "123456",
	})
	if len(sections) != 3 {
		t.Fatalf("section count = %d", len(sections))
	}
	for i, want := range []string{"收款方式", "说明", "公账收款"} {
		if sections[i].title != want {
			t.Fatalf("section[%d].title = %q, want %q", i, sections[i].title, want)
		}
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

func TestRenderSalesOrderPNGUsesConfiguredPaymentCodeLayout(t *testing.T) {
	dir := t.TempDir()
	writeSolidPNG(t, filepath.Join(dir, "sales-order", "payment", "wechat.png"), color.RGBA{G: 0xf0, A: 0xff}, 64, 64)

	renderer := SalesOrderRenderer{AssetBaseDir: dir}
	b, err := renderer.RenderPNG(salesdomain.SalesOrderSnapshot{
		OrderID:        1,
		OrderNo:        "SO-20260430-0008",
		OrderDate:      "2026-04-30",
		CustomerName:   "某某咖啡馆",
		CompanyName:    "浅焙作坊咖啡",
		PaymentText:    "微信",
		PaymentCodeBox: salesdomain.SalesOrderLayoutBox{XMM: 122, YMM: 94, WidthMM: 78, HeightMM: 126},
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
	codeBounds := dominantGreenBounds(img)
	wantLeft := int(math.Round(122 * float64(salesOrderPNGDesignWidth) / 210.0 * salesOrderPNGScale))
	wantTop := int(math.Round((94 + 8) * float64(salesOrderPNGDesignWidth) / 210.0 * salesOrderPNGScale))
	if math.Abs(float64(codeBounds.Min.X-wantLeft)) > 80 || math.Abs(float64(codeBounds.Min.Y-wantTop)) > 80 {
		t.Fatalf("payment code bounds = %v, want near configured first-page box left=%d top=%d", codeBounds, wantLeft, wantTop)
	}
	if codeBounds.Dx() < 860 || codeBounds.Dy() < 860 {
		t.Fatalf("configured payment code bounds = %v, want larger than default QR image", codeBounds)
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

func flattenSalesOrderMetaRows(rows [][]string) []string {
	out := make([]string, 0, len(rows)*3)
	for _, row := range rows {
		out = append(out, row...)
	}
	return out
}

func testCombinedSalesOrderGroups(count int) []salesdomain.CombinedSalesOrderGroup {
	groups := make([]salesdomain.CombinedSalesOrderGroup, 0, count)
	for i := 0; i < count; i++ {
		no := fmt.Sprintf("SO-202605%02d-%04d", 9+i, i+1)
		groups = append(groups, salesdomain.CombinedSalesOrderGroup{
			OrderID:      int64(i + 1),
			OrderNo:      no,
			DocumentDate: "2026-05-09",
			OrderDate:    "2026-05-07",
			Items: []salesdomain.SalesOrderSnapshotItem{{
				Name:      "兰卡拼配熟豆",
				Spec:      "1000g",
				Qty:       "1",
				Unit:      "件",
				UnitPrice: "82.00",
				LineTotal: "82.00",
				Note:      "独立订单明细备注",
			}},
			TotalAmount: "82.00",
			Shipping:    "0.00",
			Discount:    "0.00",
			GrandTotal:  "82.00",
		})
	}
	return groups
}
