package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev330SalesOrderDiscountLineLayoutSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT",
		"DEV-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT",
		"UT-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT",
		"API-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT",
		"REV-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 330 requirement seed missing %q", want)
		}
	}
}

func TestDev330SalesOrderDiscountLineLayoutWiring(t *testing.T) {
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "domain", "sales", "sales_order.go"),
			markers: []string{
				"DiscountAmount string",
				`json:"discount_amount"`,
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "postgres", "sales", "sales_order_repository.go"),
			markers: []string{
				"COALESCE(oi.discount_amount,0)::float8",
				"item.DiscountAmount",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"),
			markers: []string{
				`"优惠折扣"`,
				`"总价"`,
				"salesOrderItemSpec",
				"salesOrderItemQuantity",
				"salesOrderDiscountCell",
				"salesOrderMoneyPositive(snapshot.Discount)",
			},
		},
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_png.go"),
			markers: []string{
				"salesOrderPNGItemColumnWidths",
				"salesOrderItemCells(item, hasDiscount)",
			},
		},
	} {
		src := string(readOrderAppFileForTest(t, tc.rel))
		for _, want := range tc.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 330 marker %q", tc.rel, want)
			}
		}
	}
}

func TestDev330SalesOrderDiscountLineLayoutDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-sales-order-discount-line-layout.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-330",
			"1000g/件",
			"优惠折扣",
			"优惠合计",
			"总价",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 330 documentation marker %q", rel, want)
			}
		}
	}
}
