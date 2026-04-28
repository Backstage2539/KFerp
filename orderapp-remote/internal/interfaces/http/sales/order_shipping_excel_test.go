package sales

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"

	"github.com/xuri/excelize/v2"
)

func TestParseShipmentTrackingExcelUsesWaybillAndRemarkOrderNo(t *testing.T) {
	wb := excelize.NewFile()
	sheet := wb.GetSheetName(0)
	headers := []string{
		"用户平台订单号", "货单号", "运单号", "子单号", "快递公司", "寄付人", "寄件人手机", "寄件人地址",
		"寄付公司", "收件人", "收件人手机", "收件人地址", "收件人公司", "托寄物", "下单重量", "结算重量",
		"付款金额", "补价金额", "应收金额", "主运费", "保价费用", "保鲜费用", "包装费用", "特安服务费",
		"下单类型", "结算类型", "订单状态", "签收时间", "支付状态", "支付时间", "支付方式", "下单时间",
		"揽件时间", "揽收工号", "备注", "下单人", "一级代理名称", "二级代理名称", "转寄退回-新单", "创建时间",
	}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := wb.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	if err := wb.SetCellValue(sheet, "C2", " SF5199040648127 "); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(sheet, "AI2", "SO-20260428-0001；橘皮乌龙 227g x1件"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(sheet, "C3", "SF0222363353152"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetCellValue(sheet, "AI3", "备注没有订单号"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := wb.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if err := wb.Close(); err != nil {
		t.Fatal(err)
	}

	items, err := parseShipmentTrackingExcel(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1: %+v", len(items), items)
	}
	if items[0].OrderNo != "SO-20260428-0001" || items[0].TrackingNo != "SF5199040648127" {
		t.Fatalf("parsed item = %+v", items[0])
	}
}

func TestBuildOrderShippingWorkbookFillsTemplateDefaultsAndRemark(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	tmpl := excelize.NewFile()
	sheet := tmpl.GetSheetName(0)
	headers := []string{"收件人", "收件人手机/电话", "收件地址", "寄件人", "寄件人手机/电话", "寄件地址", "收件公司", "包裹件数", "托寄物", "重量", "长", "宽", "高", "备注(选填)", "寄件公司", "业务类型", "包装服务费"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := tmpl.SetCellValue(sheet, cell, header); err != nil {
			t.Fatal(err)
		}
	}
	if err := tmpl.SaveAs(templatePath); err != nil {
		t.Fatal(err)
	}
	if err := tmpl.Close(); err != nil {
		t.Fatal(err)
	}

	wb, err := buildOrderShippingWorkbook(templatePath, salesapp.SenderProfile{
		Name:    "寄件人",
		Phone:   "13900000000",
		Addr:    "上海市测试路",
		Company: "寄件公司",
		Goods:   "",
		BizType: "标快",
	}, salesapp.OrderShippingExportData{
		OrderID:      7,
		OrderNo:      "SO20260427001",
		OrderDate:    "2026-04-27",
		CustomerName: "测试客户",
		RecvName:     "收件人",
		RecvPhone:    "13800000000",
		RecvAddr:     "杭州市测试路",
		Items: []salesapp.OrderShippingExportItem{{
			Name:      "橘皮乌龙",
			Spec:      "227g",
			Qty:       "2",
			Unit:      "件",
			UnitPrice: "49.00",
			LineTotal: "98.00",
		}},
	})
	if err != nil {
		t.Fatalf("buildOrderShippingWorkbook() error = %v", err)
	}
	defer wb.Close()

	got := func(cell string) string {
		v, err := wb.GetCellValue(sheet, cell)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got("A2") != "收件人" || got("D2") != "寄件人" || got("I2") != "茶叶" || got("J2") != "0.1" {
		t.Fatalf("filled cells A2=%q D2=%q I2=%q J2=%q", got("A2"), got("D2"), got("I2"), got("J2"))
	}
	if !strings.Contains(got("N2"), "SO20260427001") || !strings.Contains(got("N2"), "橘皮乌龙 227g x2件") {
		t.Fatalf("remark N2 = %q", got("N2"))
	}
	if strings.Contains(got("N2"), "单价") || strings.Contains(got("N2"), "小计") || strings.Contains(got("N2"), "49.00") || strings.Contains(got("N2"), "98.00") {
		t.Fatalf("remark N2 should not include price or subtotal: %q", got("N2"))
	}
}

func TestBuildOrdersShippingWorkbookFillsSelectedOrdersOnSeparateRows(t *testing.T) {
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "ship_temp.xlsx")
	tmpl := excelize.NewFile()
	if err := tmpl.SaveAs(templatePath); err != nil {
		t.Fatal(err)
	}
	if err := tmpl.Close(); err != nil {
		t.Fatal(err)
	}

	wb, err := buildOrdersShippingWorkbook(templatePath, salesapp.SenderProfile{
		Name:  "寄件人",
		Phone: "13900000000",
		Addr:  "上海市测试路",
		Goods: "茶叶",
	}, []salesapp.OrderShippingExportData{
		{OrderNo: "SO-1", RecvName: "客户一", RecvPhone: "13800000001", RecvAddr: "地址一"},
		{OrderNo: "SO-2", RecvName: "客户二", RecvPhone: "13800000002", RecvAddr: "地址二"},
	})
	if err != nil {
		t.Fatalf("buildOrdersShippingWorkbook() error = %v", err)
	}
	defer wb.Close()
	sheet := wb.GetSheetName(0)
	got := func(cell string) string {
		v, err := wb.GetCellValue(sheet, cell)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got("A2") != "客户一" || got("A3") != "客户二" || got("N3") != "SO-2" {
		t.Fatalf("batch rows A2=%q A3=%q N3=%q", got("A2"), got("A3"), got("N3"))
	}
}

func TestOrderShippingFilenameIncludesDateCustomerAndOrderNo(t *testing.T) {
	got := orderShippingFilename(salesapp.OrderShippingExportData{
		OrderID:      7,
		OrderNo:      "SO/2026:001",
		OrderDate:    "2026-04-27",
		CustomerName: "测试 客户",
	})
	if got != "ship_20260427_测试_客户_SO_2026_001.xlsx" {
		t.Fatalf("filename = %q", got)
	}
}
