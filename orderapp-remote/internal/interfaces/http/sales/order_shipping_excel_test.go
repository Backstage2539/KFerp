package sales

import (
	"path/filepath"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"

	"github.com/xuri/excelize/v2"
)

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
