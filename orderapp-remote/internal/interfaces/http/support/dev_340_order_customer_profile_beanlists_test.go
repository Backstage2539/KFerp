package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev340OrderCustomerProfileBeanListSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
		"DEV-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
		"UT-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
		"API-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
		"REV-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-340 requirement seed missing %q", want)
		}
	}
}

func TestDev340OrderCustomerProfileBeanListWiring(t *testing.T) {
	orderSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	orderInfoStart := strings.Index(orderSrc, `<section class="panel order-fields"`)
	lineItemStart := strings.Index(orderSrc, `<section class="panel" :class`)
	if orderInfoStart < 0 || lineItemStart <= orderInfoStart {
		t.Fatalf("OrderEntryView.vue missing expected order info or line item sections")
	}
	orderInfoBlock := orderSrc[orderInfoStart:lineItemStart]
	for _, forbidden := range []string{
		`v-model.number="form.source_id"`,
		`v-model.number="form.order_type_id"`,
		`showBeanListVersionPickerByType`,
	} {
		if strings.Contains(orderInfoBlock, forbidden) {
			t.Fatalf("order info block should not contain %q", forbidden)
		}
	}
	for _, want := range []string{
		"selectedCustomerProfileSummary",
		"syncOrderHeaderFromCustomer",
		"selectedCustomerMissingProfileLabels",
		"openBeanListDrawer",
		"bean-list-drawer",
		"bean-list-picker-list",
		"选择豆单",
		"请选择客户类型",
		"请选择客户来源",
		"请选择客户订单类型",
	} {
		if !strings.Contains(orderSrc, want) {
			t.Fatalf("OrderEntryView.vue missing PR-340 marker %q", want)
		}
	}

	customerSrc := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue")))
	for _, want := range []string{
		`<select v-model="form.customer_type" required>`,
		`<select v-model.number="form.default_source_id" required>`,
		`<select v-model.number="form.default_order_type_id" required>`,
		"请选择客户类型",
		"请选择客户来源",
		"请选择客户订单类型",
	} {
		if !strings.Contains(customerSrc, want) {
			t.Fatalf("CustomersView.vue missing PR-340 marker %q", want)
		}
	}
	if strings.Contains(customerSrc, `<option :value="0">未设置</option>`) {
		t.Fatalf("CustomersView.vue should not offer unset source/order type options for required defaults")
	}
}

func TestDev340OrderCustomerProfileBeanListBackendGuards(t *testing.T) {
	customerService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customer", "service.go")))
	for _, want := range []string{
		"validateRequiredCustomerProfileDefaults",
		"客户类型",
		"来源",
		"订单类型",
	} {
		if !strings.Contains(customerService, want) {
			t.Fatalf("customer service missing PR-340 guard marker %q", want)
		}
	}

	salesRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		"requiredOrderCustomerProfileTx",
		"cmd.SourceID = customerProfile.sourceID",
		"cmd.OrderTypeID = customerProfile.orderTypeID",
		"客户资料缺少",
	} {
		if !strings.Contains(salesRepo, want) {
			t.Fatalf("sales repository missing PR-340 guard marker %q", want)
		}
	}
}

func TestDev340OrderCustomerProfileBeanListDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-customer-profile-beanlist-drawer.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-340-ORDER-CUSTOMER-PROFILE-BEANLIST-DRAWER",
			"客户类型",
			"选择豆单",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-340 documentation marker %q", rel, want)
			}
		}
	}
}
