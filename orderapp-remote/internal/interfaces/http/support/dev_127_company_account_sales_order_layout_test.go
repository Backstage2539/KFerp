package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyAccountSalesOrderLayoutRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-127",
		"DEV-127-01",
		"DEV-127-02",
		"DEV-127-03",
		"UT-127-01",
		"API-127-01",
		"REV-127-01",
		"公账收款信息移入公司设置",
		"纳税人识别号",
		"客户地址不截断",
		"收款码自适应填充",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("company account sales order layout requirement seed missing %q", want)
		}
	}
}

func TestCompanyProfileOwnsPublicAccountSettings(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CompanyProfileView.vue")))
	for _, want := range []string{
		"公账收款设置",
		"taxpayer_id",
		"bank_account_name",
		"bank_name",
		"bank_account_no",
		"copyAccountInfo",
		"一键复制公账收款信息",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("CompanyProfileView missing public account setting marker %q", want)
		}
	}
}

func TestSalesOrderLayoutWrapsCustomerAddressAndAdaptsPaymentCodes(t *testing.T) {
	pdfSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "pdf", "sales_order_pdf.go")))
	for _, want := range []string{
		"writeSalesOrderMetaRow",
		"SplitLines",
		"salesOrderPaymentCodeMetrics",
		"Stacked",
	} {
		if !strings.Contains(pdfSrc, want) {
			t.Fatalf("sales order PDF renderer missing wrapping/adaptive layout marker %q", want)
		}
	}
	if strings.Contains(pdfSrc, `writeSalesOrderMetaCell(pdf, colW, "公司地址："+snapshot.CustomerCompanyAddress)`) {
		t.Fatalf("sales order PDF renderer still writes customer company address as a clipped single-line cell")
	}

	domainSrc := string(readOrderAppFileForTest(t, filepath.Join("internal", "domain", "sales", "sales_order.go")))
	for _, want := range []string{"taxpayer_id", "company_address"} {
		if !strings.Contains(domainSrc, want) {
			t.Fatalf("SalesOrderSnapshot missing account identity JSON field %q", want)
		}
	}

	vueSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "SalesOrderView.vue")))
	for _, want := range []string{
		"single-payment-code",
		"payment-code-stack",
		"taxpayer_id",
		"company_address",
	} {
		if !strings.Contains(vueSrc, want) {
			t.Fatalf("SalesOrderView missing adaptive payment/account preview marker %q", want)
		}
	}
}
