package pdf

import (
	"bytes"
	"fmt"
	"github.com/xuri/excelize/v2"
	app "orderapp/internal/application/customerfulfillment"
	excelinfra "orderapp/internal/infrastructure/excel"
	"os"
	"path/filepath"
	"testing"
)

func TestCustomerAccountExportsAllRowsAndChinesePDF(t *testing.T) {
	d := app.AccountData{CustomerName: "测试咖啡客户", DateFrom: "2026-09-01", DateTo: "2026-09-30", AsOf: "2026-09-09 18:00:00"}
	for i := 0; i < 205; i++ {
		d.Rows = append(d.Rows, app.AccountOrder{OrderNo: fmt.Sprintf("SO-TEST-%03d", i+1), OrderDate: "2026-09-09", GoodsCents: 9500, ShippingCents: 1000, DiscountCents: 500, TotalCents: 10000, PaidCents: 3000, DueCents: 7000, PaymentStatus: "partial", ShipStatus: "未发货"})
	}
	d.Summary = app.AccountSummary{Count: 205, TotalCents: 2050000, PaidCents: 615000, DueCents: 1435000}
	d.Fees = []app.AccountFee{{ID: 1, FeeType: "shipping", AmountCents: 2000, Currency: "CNY", OccurredAt: "2026-09-09 12:00", SettlementNo: "CS-TEST-1", SettlementStatus: "confirmed", PaymentStatus: "unpaid"}}
	d.Settlements = []app.AccountSettlement{{ID: 1, SettlementNo: "CS-TEST-1", PeriodFrom: "2026-09-01", PeriodTo: "2026-09-30", Status: "confirmed", TotalCents: 2000}}
	p, err := RenderCustomerAccount(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(p, []byte("%PDF")) {
		t.Fatal("invalid PDF")
	}
	x, err := excelinfra.RenderCustomerAccount(d)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(x))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("订单账单")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 208 || rows[206][0] != "SO-TEST-205" || rows[207][5] != "20500" {
		t.Fatalf("truncated/incorrect workbook rows %d last %+v", len(rows), rows[len(rows)-1])
	}
	if dir := os.Getenv("ACCOUNT_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for ext, data := range map[string][]byte{"pdf": p, "xlsx": x} {
			if err := os.WriteFile(filepath.Join(dir, "statement-205-orders."+ext), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
