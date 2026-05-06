package customerfulfillment

import (
	"bytes"
	"slices"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseProcessingWorkbookExtractsCustodyWorkOrdersAndSKU(t *testing.T) {
	wb := excelize.NewFile()
	mustSetRows(t, wb, "生豆入库表", [][]any{
		{"日期", "入库单号", "生豆名称", "入库重量", "仓库"},
		{"2026/03/04", "IN-001", "埃塞花魁", "1.5kg", "誉观山托管仓"},
	})
	mustSetRows(t, wb, "生豆库存表", [][]any{
		{"生豆名称", "库存重量"},
		{"埃塞花魁", "1250g"},
	})
	mustSetRows(t, wb, "生产工单", [][]any{
		{"日期", "工单编号", "产品名称", "生豆名称", "投豆量", "计划产量", "状态"},
		{"2026-03-05", "WO-001", "誉观山花魁227g", "埃塞花魁", "1000g", "4", "已完成"},
	})
	mustSetRows(t, wb, "生产子工单-包装", [][]any{
		{"工单编号", "产品名称", "包装耗材", "数量"},
		{"WO-001", "誉观山花魁227g", "227g袋", "4"},
	})
	mustSetRows(t, wb, "SKU", [][]any{
		{"SKU编码", "产品名称", "规格", "烘焙度"},
		{"YGS-HK-227", "誉观山花魁227g", "227g", "浅烘"},
	})
	mustSetRows(t, wb, "耗材库存（预估）", [][]any{
		{"耗材名称", "库存数量"},
		{"227g袋", "96"},
	})

	parsed, err := ParseWorkbook(ImportTypeProcessingWorkbook, bytes.NewReader(mustWorkbookBytes(t, wb)))
	if err != nil {
		t.Fatal(err)
	}

	gotTypes := rowTypes(parsed.Rows)
	for _, want := range []string{
		"raw_bean_receipt",
		"raw_bean_balance",
		"processing_work_order",
		"packaging_job",
		"customer_sku",
		"packaging_balance",
	} {
		if !slices.Contains(gotTypes, want) {
			t.Fatalf("missing row type %s in %#v", want, gotTypes)
		}
	}
	if parsed.Summary.RawBeanReceipts != 1 || parsed.Summary.RawBeanBalances != 1 ||
		parsed.Summary.ProcessingOrders != 1 || parsed.Summary.PackagingJobs != 1 ||
		parsed.Summary.CustomerSKUs != 1 || parsed.Summary.PackagingBalances != 1 {
		t.Fatalf("unexpected summary: %#v", parsed.Summary)
	}

	receipt := firstRowOfType(t, parsed.Rows, "raw_bean_receipt")
	if receipt.ExternalKey != "raw_bean_receipt:IN-001:埃塞花魁" {
		t.Fatalf("unexpected receipt key %q", receipt.ExternalKey)
	}
	if got := receipt.Payload["quantity_g"]; got != int64(1500) {
		t.Fatalf("quantity_g = %#v, want 1500", got)
	}
}

func TestParseDirectShipWorkbookCarriesForwardOrderHeaderRows(t *testing.T) {
	wb := excelize.NewFile()
	mustSetRows(t, wb, "代发信息", [][]any{
		{"时间", "序号", "订单编号", "收货地址", "商品标题", "属性", "商品规格", "数量", "磨粉服务", "备注", "运单号", "发货日期", "状态"},
		{"2026-03-04", "1", "YGS20260304001", "张三 13800000000 浙江杭州", "誉观山花魁", "浅烘", "100g", "1", "不磨粉", "加急", "", "", "待发货"},
		{"", "", "", "", "誉观山拼配", "中烘", "227g", "2", "磨粉", "", "", "", ""},
	})

	parsed, err := ParseWorkbook(ImportTypeDirectShipWorkbook, bytes.NewReader(mustWorkbookBytes(t, wb)))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Summary.DirectShipOrders != 1 || parsed.Summary.DirectShipItems != 2 {
		t.Fatalf("unexpected direct ship summary: %#v", parsed.Summary)
	}

	items := rowsOfType(parsed.Rows, "direct_ship_item")
	if len(items) != 2 {
		t.Fatalf("direct ship item rows = %d, want 2", len(items))
	}
	if got := items[1].Payload["order_no"]; got != "YGS20260304001" {
		t.Fatalf("second item order_no = %#v, want carried header", got)
	}
	if got := items[1].Payload["receiver_address"]; got != "张三 13800000000 浙江杭州" {
		t.Fatalf("second item receiver_address = %#v, want carried header", got)
	}
	if got := items[1].Payload["quantity_units"]; got != int64(2) {
		t.Fatalf("second item quantity_units = %#v, want 2", got)
	}
}

func TestParseSettlementWorkbookExtractsFeeLines(t *testing.T) {
	wb := excelize.NewFile()
	mustSetRows(t, wb, "结算单", [][]any{
		{"烘焙"},
		{"项目", "数量", "单价", "金额"},
		{"烘焙费", "1000g", "0.08", "80"},
		{"代发、仓储费用"},
		{"代发费", "3单", "3", "9"},
		{"生豆仓储费", "5kg", "10", "50"},
		{"物流费用"},
		{"物流费", "2票", "", "24"},
	})

	parsed, err := ParseWorkbook(ImportTypeSettlementWorkbook, bytes.NewReader(mustWorkbookBytes(t, wb)))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Summary.FeeItems != 4 {
		t.Fatalf("fee items = %d, want 4", parsed.Summary.FeeItems)
	}
	for _, want := range []string{"roasting", "direct_ship_service", "storage", "shipping"} {
		if !slices.Contains(feeTypes(parsed.Rows), want) {
			t.Fatalf("missing fee type %s in %#v", want, feeTypes(parsed.Rows))
		}
	}
	shipping := firstFeeOfType(t, parsed.Rows, "shipping")
	if got := shipping.Payload["amount_cents"]; got != int64(2400) {
		t.Fatalf("shipping amount_cents = %#v, want 2400", got)
	}
}

func TestParseQuantityAndExcelDateHelpers(t *testing.T) {
	gCases := map[string]int64{
		"1.5kg":  1500,
		"250g":   250,
		"2,000":  2000,
		" 1 KG ": 1000,
	}
	for in, want := range gCases {
		got, ok := parseQtyG(in)
		if !ok || got != want {
			t.Fatalf("parseQtyG(%q) = %d,%v want %d,true", in, got, ok, want)
		}
	}
	unitCases := map[string]int64{
		"3单": 3,
		"2票": 2,
		"12": 12,
	}
	for in, want := range unitCases {
		got, ok := parseQtyUnits(in)
		if !ok || got != want {
			t.Fatalf("parseQtyUnits(%q) = %d,%v want %d,true", in, got, ok, want)
		}
	}
	for in, want := range map[string]string{
		"2026/3/4":            "2026-03-04",
		"2026-03-04 00:00:00": "2026-03-04",
	} {
		if got := parseExcelDateText(in); got != want {
			t.Fatalf("parseExcelDateText(%q) = %q, want %q", in, got, want)
		}
	}
	if got := normalizedCell("  誉观山\n花魁\t "); got != "誉观山 花魁" {
		t.Fatalf("normalizedCell collapsed whitespace to %q", got)
	}
}

func mustSetRows(t *testing.T, wb *excelize.File, sheet string, rows [][]any) {
	t.Helper()
	idx, err := wb.GetSheetIndex(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if idx < 0 {
		if _, err := wb.NewSheet(sheet); err != nil {
			t.Fatal(err)
		}
	}
	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := wb.SetCellValue(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func mustWorkbookBytes(t *testing.T, wb *excelize.File) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := wb.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func rowTypes(rows []ParsedRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.RowType)
	}
	return out
}

func rowsOfType(rows []ParsedRow, rowType string) []ParsedRow {
	out := make([]ParsedRow, 0)
	for _, row := range rows {
		if row.RowType == rowType {
			out = append(out, row)
		}
	}
	return out
}

func firstRowOfType(t *testing.T, rows []ParsedRow, rowType string) ParsedRow {
	t.Helper()
	for _, row := range rows {
		if row.RowType == rowType {
			return row
		}
	}
	t.Fatalf("row type %s not found", rowType)
	return ParsedRow{}
}

func feeTypes(rows []ParsedRow) []string {
	out := make([]string, 0)
	for _, row := range rows {
		if row.RowType == "fee_item" {
			if feeType, ok := row.Payload["fee_type"].(string); ok {
				out = append(out, feeType)
			}
		}
	}
	return out
}

func firstFeeOfType(t *testing.T, rows []ParsedRow, feeType string) ParsedRow {
	t.Helper()
	for _, row := range rows {
		if row.RowType == "fee_item" && row.Payload["fee_type"] == feeType {
			return row
		}
	}
	t.Fatalf("fee type %s not found", feeType)
	return ParsedRow{}
}
