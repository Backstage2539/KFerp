package freight

import (
	"bytes"
	"github.com/xuri/excelize/v2"
	"testing"
)

func TestActualWorkbookMappedTextWaybillAndMoney(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetRow("Sheet1", "A1", &[]any{"最终费", "运单"})
	f.SetSheetRow("Sheet1", "A2", &[]any{"12.00", "00123456789012345678"})
	f.SetSheetRow("Sheet1", "A3", &[]any{"-1", "SF2"})
	b, e := f.WriteToBuffer()
	if e != nil {
		t.Fatal(e)
	}
	rows, e := parseWorkbook(bytes.NewReader(b.Bytes()), "Sheet1", 2, 1)
	if e != nil || len(rows) != 2 || rows[0].TrackingNo != "00123456789012345678" || rows[0].ActualCents != 1200 || rows[1].Error == "" {
		t.Fatal(rows, e)
	}
}
