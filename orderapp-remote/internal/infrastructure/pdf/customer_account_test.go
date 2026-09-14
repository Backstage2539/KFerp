package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestRenderCustomerAccountBuildsUnifiedStatementPDF(t *testing.T) {
	body, err := RenderCustomerAccount(app.AccountData{
		CustomerName: "测试客户", DateFrom: "2026-09-01", DateTo: "2026-09-30", AsOf: "2026-09-14 16:00:00",
		Summary:     app.AccountSummary{GoodsCents: 10000, ProcessingCents: 1200, DirectShipServiceCents: 300, ShippingCents: 800, PayableCents: 12300, PaidCents: 5000, DueCents: 7300},
		Rows:        []app.AccountOrder{{OrderNo: "SO-20260914-001", OrderDate: "2026-09-14", GoodsCents: 10000, ShippingCents: 800, TotalCents: 10800, PaidCents: 5000, DueCents: 5800, PaymentStatus: "partial", ShipStatus: "部分发货"}},
		Fees:        []app.AccountFee{{ID: 7, FeeType: "processing", FeeName: "代加工费", AmountCents: 1200, SourceType: "processing_request", SourceID: 21}},
		Settlements: []app.AccountSettlement{{SettlementNo: "SET-1", PeriodFrom: "2026-09-01", PeriodTo: "2026-09-30", TotalCents: 12300, Status: "confirmed", ReconciliationStatus: "disputed", Disputes: []app.AccountStatementDispute{{Reason: "运费金额与约定不符", Status: "resolved", Reply: "已复核并通过 ERP 调整", RepliedBy: "财务"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) < 1000 || !bytes.HasPrefix(body, []byte("%PDF")) {
		t.Fatalf("unexpected PDF output: %d bytes", len(body))
	}
	if dir := os.Getenv("KFERP_PDF_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "customer-account.pdf"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
