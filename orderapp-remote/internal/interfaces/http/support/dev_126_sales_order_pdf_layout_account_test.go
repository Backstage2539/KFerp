package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSalesOrderPDFLayoutAccountRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-126",
		"DEV-126-01",
		"DEV-126-02",
		"DEV-126-03",
		"UT-126-01",
		"API-126-01",
		"REV-126-01",
		"PDF 公章等比渲染",
		"预览和导出 PDF 版式一致",
		"公账收款设置",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order PDF layout/account requirement seed missing %q", want)
		}
	}
}

func TestSalesOrderPDFRendererUsesPreviewStylePaymentLayout(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"renderSalesOrderHeader",
		"renderSalesOrderItemsTable",
		"renderSalesOrderTotals",
		"renderSalesOrderPaymentInfoSection",
		"renderSalesOrderAccountLines",
		"fitSalesOrderImageInBox",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("sales order PDF renderer missing preview-style layout marker %q", want)
		}
	}
	for _, unwanted := range []string{
		`pdf.CellFormat(0, 12, "销售单", "", 1, "C"`,
		`"应收合计："`,
	} {
		if strings.Contains(src, unwanted) {
			t.Fatalf("sales order PDF renderer still contains old layout marker %q", unwanted)
		}
	}
}

func TestSalesOrderPreviewUsesPDFRendererForAccountAndSharedPaymentLayout(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"PDFStampPreview",
		"salesOrderPreviewPDFUrl",
		"preview-label=\"PREVIEW 预览版\"",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("SalesOrderView missing shared payment/account layout marker %q", want)
		}
	}
	pdf := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"renderSalesOrderPaymentInfoSection",
		"renderSalesOrderAccountLines",
		"BankAccountName",
		"BankName",
		"BankAccountNo",
	} {
		if !strings.Contains(pdf, want) {
			t.Fatalf("sales order PDF renderer missing shared payment/account marker %q", want)
		}
	}
}

func TestSalesOrderSettingsNoLongerOwnsBankAccountSettings(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderSettingsView.vue")))
	for _, forbidden := range []string{
		"公账收款设置",
		"bank_account_name",
		"bank_name",
		"bank_account_no",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("SalesOrderSettingsView should not keep company-owned bank account marker %q", forbidden)
		}
	}
}
