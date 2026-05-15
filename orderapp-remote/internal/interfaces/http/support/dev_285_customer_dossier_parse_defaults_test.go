package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerDossierParseDefaultsSourceGuards(t *testing.T) {
	customersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	orderEntryView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))

	for _, want := range []string{
		"parseRecipientText",
		"粘贴收件信息",
		"地址解析",
		"<span>联系电话</span>",
		"<span>来源</span>",
		"<span>订单类型</span>",
		"defaultSourceID",
		"defaultOrderTypeID",
	} {
		if !strings.Contains(customersView, want) {
			t.Fatalf("CustomersView.vue missing marker %q", want)
		}
	}

	for _, forbidden := range []string{
		"原始名称",
		"<span>电话</span>",
		"默认来源",
		"默认订单类型",
		"公司地址",
	} {
		if strings.Contains(customersView, forbidden) {
			t.Fatalf("CustomersView.vue should not contain %q", forbidden)
		}
	}

	for _, want := range []string{
		"<span>联系电话</span>",
		"<span>来源</span>",
		"<span>订单类型</span>",
		"company_phone: customerForm.phone",
		"地址解析",
	} {
		if !strings.Contains(orderEntryView, want) {
			t.Fatalf("OrderEntryView.vue missing marker %q", want)
		}
	}
}
