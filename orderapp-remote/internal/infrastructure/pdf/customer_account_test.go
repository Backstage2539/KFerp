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

func TestRenderCustomerOrderFeeAccountPaginatesLongGroupedDetail(t *testing.T) {
	lines := make([]app.AccountFeeLine, 0, 70)
	for i := 0; i < 70; i++ {
		lines = append(lines, app.AccountFeeLine{AccountFee: app.AccountFee{ID: int64(i + 1), OrderID: 9, OrderNo: "SO-9", FeeType: "shipping", FeeName: "寄件实际运费调整费用名称较长用于验证重复表头和分页", AmountCents: 1234, OccurredAt: "2026-10-26", TrackingNo: "SF5144416745654"}, Category: "shipping"})
	}
	body, err := RenderCustomerOrderFeeAccount(app.AccountData{
		CustomerName: "测试客户", DateFrom: "2026-10-01", DateTo: "2026-10-31", AsOf: "2026-10-27 15:00:00",
		FeeSummary:     app.AccountFeeSummary{ShippingCents: 70 * 1234, PayableCents: 70 * 1234},
		OrderFeeGroups: []app.AccountOrderFeeGroup{{Order: app.AccountOrder{ID: 9, OrderNo: "SO-9", OrderDate: "2026-10-22"}, PayableCents: 70 * 1234, FeeLines: lines}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) || bytes.Count(body, []byte("/Type /Page")) < 2 {
		t.Fatalf("expected multipage fee PDF, bytes=%d page objects=%d", len(body), bytes.Count(body, []byte("/Type /Page")))
	}
	if dir := os.Getenv("KFERP_PDF_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "customer-order-fees.pdf"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderCustomerUnpaidFeeAccount(t *testing.T) {
	lines := make([]app.AccountFeeLine, 35)
	for i := range lines {
		lines[i] = app.AccountFeeLine{AccountFee: app.AccountFee{ID: int64(i + 1), FeeType: "shipping", FeeName: "寄件费用长名称用于检验未付明细及多页展示", AmountCents: 910, OccurredAt: "2026-10-26"}, Category: "shipping", ReceivableCents: 1000, ReceivedCents: 90}
	}
	body, err := RenderCustomerOrderFeeAccount(app.AccountData{CustomerName: "测试客户", UnpaidOnly: true, AsOf: "2026-10-28 12:00:00", FeeSummary: app.AccountFeeSummary{ShippingCents: 31850, PayableCents: 31850, DueCents: 31850}, OrderFeeGroups: []app.AccountOrderFeeGroup{{Order: app.AccountOrder{ID: 1, OrderNo: "SO-1"}, PayableCents: 31850, DueCents: 31850, FeeLines: lines}}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) || bytes.Count(body, []byte("/Type /Page")) < 2 {
		t.Fatalf("invalid or single-page PDF, bytes=%d", len(body))
	}
	if dir := os.Getenv("KFERP_PDF_QA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "customer-unpaid-fees.pdf"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
