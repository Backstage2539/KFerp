package shippingreturn

import (
	"bytes"
	"github.com/xuri/excelize/v2"
	"testing"
)

func TestRecipient(t *testing.T) {
	for _, c := range []struct {
		a, b, p, q string
		want       bool
	}{{"张*", "张三", "138****1234", "13812341234", true}, {"张*", "张三丰", "138****1234", "+86 13812341234", true}, {"张三", "张四", "138****1234", "13812341234", false}, {"*", "张三", "138****1234", "13812341234", false}, {"张*", "张三", "138****1234", "13912341234", false}} {
		if got := RecipientMatches(c.a, c.p, c.b, c.q); got != c.want {
			t.Fatalf("%+v got %v", c, got)
		}
	}
}
func TestParse(t *testing.T) {
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "寄件列表")
	cfg := DefaultTemplate()
	for _, v := range cfg.Columns {
		f.SetCellValue(cfg.Sheet, v.Column+"1", v.Header)
	}
	for i := 2; i <= 10; i++ {
		for col, v := range map[string]string{"C": "SF0000000000001", "J": "张*", "K": "138****1234", "S": "12.000"} {
			cell, _ := excelize.CoordinatesToCellName(mustColumn(col), i)
			f.SetCellValue(cfg.Sheet, cell, v)
		}
	}
	for _, cell := range []string{"S7", "S9", "S10"} {
		f.SetCellValue(cfg.Sheet, cell, "")
	}
	f.SetCellValue(cfg.Sheet, "S2", "0.000")
	buf, _ := f.WriteToBuffer()
	rows, e := Parse(bytes.NewReader(buf.Bytes()), cfg)
	if e != nil {
		t.Fatal(e)
	}
	valid := 0
	for _, r := range rows {
		if r.Error == "" {
			valid++
		}
	}
	if valid != 6 || len(rows) != 9 || rows[0].ActualCents != 0 {
		t.Fatalf("%+v", rows)
	}
	cfg.Columns["amount"] = Column{"S", "付款金额"}
	if _, e = Parse(bytes.NewReader(buf.Bytes()), cfg); e == nil {
		t.Fatal("header mismatch accepted")
	}
}
func mustColumn(s string) int { n, _ := excelize.ColumnNameToNumber(s); return n }

func TestParseRejectsUnsafeTrackingNumbers(t *testing.T) {
	for _, value := range []any{"SF1,SF2", "SF1\nSF2", "1.23E+15", int64(1234567890123456)} {
		f := excelize.NewFile()
		cfg := DefaultTemplate()
		f.SetSheetName("Sheet1", cfg.Sheet)
		for _, c := range cfg.Columns {
			f.SetCellValue(cfg.Sheet, c.Column+"1", c.Header)
		}
		for col, v := range map[string]any{"C": value, "S": 12, "J": "张三", "K": "138****1234"} {
			f.SetCellValue(cfg.Sheet, col+"2", v)
		}
		buf, _ := f.WriteToBuffer()
		rows, err := Parse(bytes.NewReader(buf.Bytes()), cfg)
		f.Close()
		if err != nil || len(rows) != 1 || rows[0].Status != "skipped" {
			t.Fatalf("unsafe tracking %v: %+v %v", value, rows, err)
		}
	}
}
