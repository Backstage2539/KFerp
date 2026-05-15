package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIMNeutralNotificationRulesFrameworkWiring(t *testing.T) {
	service := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "messagecenter", "service.go")))
	schema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "messagecenter", "schema.go")))
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "messagecenter", "repository.go")))
	api := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "messagecenter", "api.go")))
	module := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "messagecenter", "module.go")))
	salesShipping := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_shipping_excel.go")))
	productionFlow := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "production_flow_routes.go")))
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "NotificationSettingsView.vue")))
	menu := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js")))
	manual := string(readOrderAppFileForTest(t, filepath.Join("docs", "OP_MANUAL_NOTIFICATIONS.md")))
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	for _, want := range []string{"ChannelAdapter", "ExternalDelivery", "PayloadMatch", "order_customer", "order_responsible", "ChannelExternalIM"} {
		if !strings.Contains(service, want) {
			t.Fatalf("messagecenter service missing IM-neutral marker %q", want)
		}
	}
	for _, want := range []string{"message_notification_rules", "message_channel_identities", "template_key", "adapter_key", "order_shipped_customer_external_im"} {
		if !strings.Contains(schema, want) {
			t.Fatalf("messagecenter schema missing framework marker %q", want)
		}
	}
	for _, want := range []string{"ListActiveRules", "SaveRule", "employee_roles", "customer_erp_user_bindings"} {
		if !strings.Contains(repo, want) {
			t.Fatalf("messagecenter repository missing recipient/rule marker %q", want)
		}
	}
	for _, want := range []string{"/api/message-center/rules", "listRules", "saveRule"} {
		if !strings.Contains(api+module, want) {
			t.Fatalf("messagecenter API missing rule route marker %q", want)
		}
	}
	for _, want := range []string{"order.shipped", "publishOrdersShipped"} {
		if !strings.Contains(salesShipping, want) {
			t.Fatalf("sales shipping missing status event marker %q", want)
		}
	}
	for _, want := range []string{"order.production_finished", "publishProductionFinished"} {
		if !strings.Contains(productionFlow, want) {
			t.Fatalf("production flow missing status event marker %q", want)
		}
	}
	for _, want := range []string{"通知配置", "external_im", "wechat_service_account", "enterprise_wechat", "payload_match"} {
		if !strings.Contains(view+menu, want) {
			t.Fatalf("notification settings UI missing marker %q", want)
		}
	}
	for _, want := range []string{"不是微信专用", "外部 IM", "adapter", "OP_MANUAL_NOTIFICATIONS"} {
		if !strings.Contains(manual+"\n"+reqStore, want) {
			t.Fatalf("manual/requirements seed missing marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-280-IM-NEUTRAL-NOTIFICATION-RULES",
		"DEV-280-IM-NEUTRAL-NOTIFICATION-RULES",
		"UT-280-IM-NEUTRAL-NOTIFICATION-RULES",
		"API-280-IM-NEUTRAL-NOTIFICATION-RULES",
		"REV-280-IM-NEUTRAL-NOTIFICATION-RULES",
	} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
