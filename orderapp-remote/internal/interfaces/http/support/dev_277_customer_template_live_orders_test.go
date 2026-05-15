package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev277CustomerTemplateLiveOrdersEvidenceExists(t *testing.T) {
	processingPortal := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerProcessingPortalView.vue")))
	templateView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerCapabilityTemplatesView.vue")))
	settingsView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue")))
	customerPortalService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "customerportal", "service.go")))
	miniAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go")))
	customerPortalManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md")))
	customerFulfillmentManual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md")))
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	for _, want := range []string{
		"履约客户订单",
		"fetchCustomerFulfillmentOrders",
		"fetchCustomerFulfillmentOrderDetail",
		"customerFulfillmentOrderFees",
		"SalesOrderView",
		"DeliveryNoteView",
	} {
		if !strings.Contains(processingPortal, want) {
			t.Fatalf("CustomerProcessingPortalView.vue missing %q", want)
		}
	}
	if strings.Contains(processingPortal, "overview.direct_ship_orders") {
		t.Fatal("customer-side processing portal should not use the old direct_ship_orders table")
	}

	for _, want := range []string{
		"复制模板",
		"模板失效",
		"parent_template_key",
		"expandedTemplateKey",
		"visibleTemplateEditors",
		"flattenTemplateEditorsForTree",
		"请输入新模板名称",
		"inactive-template-badge",
	} {
		if !strings.Contains(templateView, want) {
			t.Fatalf("CustomerCapabilityTemplatesView.vue missing %q", want)
		}
	}
	if strings.Contains(templateView, "请输入新模板 key") {
		t.Fatal("template copy should not ask operators to type a technical template key")
	}
	if strings.Contains(templateView, ".template-panel.inactive { background: #f8fafc; opacity") {
		t.Fatal("inactive templates should not dim the whole panel or save button")
	}

	for _, want := range []string{
		"activeTemplates",
		"模板已失效",
		"inactiveTemplateKey",
		"template.active !== false",
	} {
		if !strings.Contains(settingsView, want) {
			t.Fatalf("CustomerPortalSettingsView.vue missing %q", want)
		}
	}

	for _, want := range []string{
		"ErrCapabilityTemplateInvalid",
		"ParentTemplateKey",
		"SortOrder",
		"CopyCapabilityTemplate",
		"nextCapabilityTemplateCopyKey",
	} {
		if !strings.Contains(customerPortalService, want) {
			t.Fatalf("customerportal service missing %q", want)
		}
	}
	for _, want := range []string{
		"miniCustomerConfigUpdatedMessage",
		"客户配置已更新，请联系管理员处理",
		"miniCustomerConfigUpdatedError",
	} {
		if !strings.Contains(miniAPI, want) {
			t.Fatalf("mini api missing %q", want)
		}
	}

	for _, want := range []string{
		"能力模板是实时引用",
		"手动点击“复制模板”",
		"模板已失效",
		"capability template invalid",
		"客户配置已更新，请联系管理员处理",
	} {
		if !strings.Contains(customerPortalManual, want) {
			t.Fatalf("customer portal manual missing %q", want)
		}
	}
	for _, want := range []string{
		"底部也使用同一套“履约客户订单”列表",
		"旧的“代发订单”小表",
		"同一套订单数据源",
	} {
		if !strings.Contains(customerFulfillmentManual, want) {
			t.Fatalf("customer fulfillment manual missing %q", want)
		}
	}
	for _, want := range []string{
		"PR-277-CUSTOMER-TEMPLATE-LIVE-ORDERS",
		"DEV-277-01",
		"UT-277-01",
		"API-277-01",
		"REV-277-01",
		"PR-279-CUSTOMER-TEMPLATE-COPY-ACCORDION",
		"DEV-279-01",
		"UT-279-01",
		"API-279-01",
		"REV-279-01",
		"PR-280-CUSTOMER-TEMPLATE-INACTIVE-MINIAPP-MESSAGE",
		"DEV-280-01",
		"UT-280-01",
		"API-280-01",
		"REV-280-01",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store missing %q", want)
		}
	}
}
