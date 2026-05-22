package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev329SalesOrderSettlementSummaryLayoutSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT",
		"DEV-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT",
		"UT-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT",
		"API-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT",
		"REV-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 329 requirement seed missing %q", want)
		}
	}
}

func TestDev329SalesOrderSettlementSummaryLayoutWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"),
			markers: []string{
				`item.UnitPrice`,
				`"订单备注"`,
				`"商品合计： " + snapshot.TotalAmount`,
				`salesOrderMoneyPositive(snapshot.Discount)`,
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go"),
			markers: []string{
				"salesOrderItemHeaders(hasDiscount)",
				"row.Cells",
				"订单备注",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 329 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev329SalesOrderSettlementSummaryLayoutDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-sales-order-settlement-summary-layout.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-329",
			"订单备注",
			"优惠合计",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 329 documentation marker %q", rel, want)
			}
		}
	}
}
