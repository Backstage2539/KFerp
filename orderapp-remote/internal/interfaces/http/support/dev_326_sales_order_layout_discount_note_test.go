package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev326SalesOrderLayoutDiscountNoteSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE",
		"DEV-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE",
		"UT-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE",
		"API-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE",
		"REV-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 326 requirement seed missing %q", want)
		}
	}
}

func TestDev326SalesOrderLayoutDiscountNoteWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"),
			markers: []string{
				"salesOrderItemRowHeight",
				"salesOrderFinancialRows",
				"salesOrderDiscountTypeLabel",
				"SalesOrderNote",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go"),
			markers: []string{
				"salesOrderPNGItemRowHeight",
				"salesOrderFinancialRows",
				"备注",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue"),
			markers: []string{
				"销售单备注",
				"/sales-order-note",
				"只显示在销售单最后一行",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 326 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev326SalesOrderLayoutDiscountNoteDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-22-sales-order-layout-discount-note.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-326",
			"销售单备注",
			"优惠",
			"换行",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 326 documentation marker %q", rel, want)
			}
		}
	}
}
