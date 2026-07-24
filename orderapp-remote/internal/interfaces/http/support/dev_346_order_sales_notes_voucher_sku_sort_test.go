package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev346OrderSalesNotesVoucherSkuSortRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
		"DEV-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
		"UT-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
		"API-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
		"REV-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-344 requirement seed missing %q", want)
		}
	}
}

func TestDev346OrderSalesNotesVoucherSkuSortWiring(t *testing.T) {
	checks := []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go"),
			markers: []string{
				"快递费备注",
				"销售单备注",
				"salesOrderFinancialRows",
				"salesOrderWrapCellText",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue"),
			markers: []string{
				"orderTotalPreviewValue",
				"orderTotalHintText",
				"paymentVoucherCollapsed",
				"voucher-preview-overlay",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "FinanceReportView.vue"),
			markers: []string{
				"payment_voucher_url",
				"收款凭证",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"),
			markers: []string{
				"sku-name-input",
				"sortRowsForCustomerSkuPriority",
			},
		},
		{
			rel: filepath.Join("frontend-vue-shell", "src", "lib", "bom.js"),
			markers: []string{
				"sortBomContextProducts",
				"order_usage_count",
			},
		},
	}
	for _, check := range checks {
		src := string(readOrderAppFileForTest(t, check.rel))
		for _, want := range check.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-341 wiring marker %q", check.rel, want)
			}
		}
	}
	pdfSource := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, forbidden := range []string{"salesOrderItemNoteSummary", `"订单明细备注"`} {
		if strings.Contains(pdfSource, forbidden) {
			t.Fatalf("sales order item notes must stay in item rows after PR-550; found obsolete marker %q", forbidden)
		}
	}
}

func TestDev346OrderSalesNotesVoucherSkuSortDocs(t *testing.T) {
	checks := []struct {
		rel     string
		markers []string
	}{
		{
			rel: filepath.Join("docs", "REQUIREMENTS.md"),
			markers: []string{
				"PR-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
				"收款凭证",
				"物流费用",
				"SKU",
				"BOM",
			},
		},
		{
			rel: filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
			markers: []string{
				"PR-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
				"收款凭证",
				"物流费用",
				"客户 SKU 和 BOM",
			},
		},
		{
			rel: filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
			markers: []string{
				"PR-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
				"快递费备注",
				"订单明细备注",
				"收款凭证上传后会默认收起",
			},
		},
		{
			rel: filepath.Join("docs", "OP_MANUAL_FINANCE.md"),
			markers: []string{
				"收款凭证入口",
				"来源明细可直接打开凭证",
			},
		},
		{
			rel: filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"),
			markers: []string{
				"商品名称不再在列表单元格内直接编辑",
				"搜索可匹配商品名称",
				"生产 BOM 制造主档",
			},
		},
		{
			rel: filepath.Join("docs", "acceptance", "2026-05-23-order-sales-notes-voucher-sku-sort.md"),
			markers: []string{
				"PR-346-ORDER-SALES-NOTES-VOUCHER-SKU-SORT",
				"收款凭证",
				"物流费用",
				"SKU",
				"BOM",
			},
		},
	}
	for _, check := range checks {
		src := string(readOrderAppFileForTest(t, check.rel))
		for _, want := range check.markers {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-344 documentation marker %q", check.rel, want)
			}
		}
	}
}
