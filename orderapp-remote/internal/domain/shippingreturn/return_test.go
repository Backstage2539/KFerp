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

func TestParseOptionalShipmentSettlementMetadataAndLegacyTemplate(t *testing.T) {
	cfg := DefaultTemplate()
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", cfg.Sheet)
	for key, column := range cfg.Columns {
		if key == "settlement_weight" || key == "pickup_time" {
			continue
		}
		f.SetCellValue(cfg.Sheet, column.Column+"1", column.Header)
	}
	f.SetCellValue(cfg.Sheet, "C2", "SF0000000000001")
	f.SetCellValue(cfg.Sheet, "J2", "张三")
	f.SetCellValue(cfg.Sheet, "K2", "138****1234")
	f.SetCellValue(cfg.Sheet, "S2", "20.20")
	buf, _ := f.WriteToBuffer()
	rows, err := Parse(bytes.NewReader(buf.Bytes()), cfg)
	f.Close()
	if err != nil || len(rows) != 1 || rows[0].SettlementWeightKG != "" || rows[0].PickupTime != "" || rows[0].Error != "" {
		t.Fatalf("legacy template rows=%+v err=%v", rows, err)
	}

	f = excelize.NewFile()
	cfg.WeightUnit = "g"
	f.SetSheetName("Sheet1", cfg.Sheet)
	for _, column := range cfg.Columns {
		f.SetCellValue(cfg.Sheet, column.Column+"1", column.Header)
	}
	f.SetCellValue(cfg.Sheet, "C2", "SF0000000000002")
	f.SetCellValue(cfg.Sheet, "J2", "张三")
	f.SetCellValue(cfg.Sheet, "K2", "138****1234")
	f.SetCellValue(cfg.Sheet, "S2", "20.20")
	f.SetCellValue(cfg.Sheet, "P2", "1250")
	f.SetCellValue(cfg.Sheet, "AG2", "2026-10-26 10:30:00")
	buf, _ = f.WriteToBuffer()
	rows, err = Parse(bytes.NewReader(buf.Bytes()), cfg)
	f.Close()
	if err != nil || len(rows) != 1 || rows[0].SettlementWeightKG != "1.250" || rows[0].PickupTime != "2026-10-26 10:30:00" || rows[0].Error != "" {
		t.Fatalf("metadata rows=%+v err=%v", rows, err)
	}
	if got, err := normalizeWeightKG("1.25", "kg"); err != nil || got != "1.250" {
		t.Fatalf("kg weight=%q err=%v", got, err)
	}
}

func TestParseRejectsInvalidSettlementWeight(t *testing.T) {
	cfg := DefaultTemplate()
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", cfg.Sheet)
	for _, column := range cfg.Columns {
		f.SetCellValue(cfg.Sheet, column.Column+"1", column.Header)
	}
	for cell, value := range map[string]string{"C2": "SF0000000000003", "J2": "张三", "K2": "138****1234", "S2": "20.20", "P2": "-1"} {
		f.SetCellValue(cfg.Sheet, cell, value)
	}
	buf, _ := f.WriteToBuffer()
	rows, err := Parse(bytes.NewReader(buf.Bytes()), cfg)
	f.Close()
	if err != nil || len(rows) != 1 || rows[0].Status != "skipped" || rows[0].Error == "" {
		t.Fatalf("invalid weight rows=%+v err=%v", rows, err)
	}
}

func TestParseSettlementWeightWithActualCarrierHeaders(t *testing.T) {
	for _, tc := range []struct {
		name, configured, actual, column, value, want string
	}{
		{"carrier P header without unit", "结算重量（kg）", "结算重量", "P", "7.000", "7.000"},
		{"old configured header with fullwidth unit", "结算重量", "结算重量（kg）", "P", "1.25", "1.250"},
		{"halfwidth unit", "结算重量（kg）", "结算重量(kg)", "P", "1.25", "1.250"},
		{"moved optional weight column", "结算重量（kg）", "结算重量", "AH", "2.5", "2.500"},
		{"unrelated header not interpreted as weight", "结算重量（kg）", "下单重量", "P", "9", ""},
		{"custom template not broadened", "客户重量", "结算重量", "P", "7", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultTemplate()
			cfg.Columns["settlement_weight"] = Column{"P", tc.configured}
			f := excelize.NewFile()
			defer f.Close()
			f.SetSheetName("Sheet1", cfg.Sheet)
			for key, col := range cfg.Columns {
				if key != "settlement_weight" {
					f.SetCellValue(cfg.Sheet, col.Column+"1", col.Header)
				}
			}
			for cell, value := range map[string]string{"C2": "SF1224903286779", "J2": "张三", "K2": "138****1234", "S2": "20.20", tc.column + "1": tc.actual, tc.column + "2": tc.value} {
				f.SetCellValue(cfg.Sheet, cell, value)
			}
			buf, err := f.WriteToBuffer()
			if err != nil {
				t.Fatal(err)
			}
			rows, err := Parse(bytes.NewReader(buf.Bytes()), cfg)
			if err != nil || len(rows) != 1 || rows[0].Error != "" || rows[0].SettlementWeightKG != tc.want {
				t.Fatalf("rows=%+v err=%v, want weight %q", rows, err, tc.want)
			}
		})
	}
}

func TestParseRejectsFormulaInRelocatedSettlementWeight(t *testing.T) {
	cfg := DefaultTemplate()
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", cfg.Sheet)
	for key, col := range cfg.Columns {
		if key != "settlement_weight" {
			f.SetCellValue(cfg.Sheet, col.Column+"1", col.Header)
		}
	}
	for cell, value := range map[string]string{"C2": "SF1224903286779", "J2": "张三", "K2": "138****1234", "S2": "20.20", "AH1": "结算重量"} {
		f.SetCellValue(cfg.Sheet, cell, value)
	}
	f.SetCellFormula(cfg.Sheet, "AH2", "1+1")
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Parse(bytes.NewReader(buf.Bytes()), cfg)
	if err != nil || len(rows) != 1 || rows[0].Status != "skipped" {
		t.Fatalf("formula rows=%+v err=%v", rows, err)
	}
}
