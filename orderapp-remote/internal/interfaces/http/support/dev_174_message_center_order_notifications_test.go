package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMessageCenterOrderNotificationWiring(t *testing.T) {
	messageSchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "messagecenter", "schema.go")))
	messageAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "messagecenter", "api.go")))
	messageModule := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "messagecenter", "module.go")))
	salesOrderAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	miniAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerportal", "mini_api.go")))
	appRoutes := string(readOrderAppFileForTest(t, filepath.Join("internal", "appmain", "app_routes.go")))
	authz := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "authz_middleware.go")))
	appVue := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "App.vue")))
	ordersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	customerFulfillmentAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "customerfulfillment", "api.go")))
	customerFulfillmentRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go")))
	orderQueries := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_queries.go")))

	for _, want := range []string{
		"message_events",
		"message_deliveries",
		"message_reads",
		"channel TEXT NOT NULL",
	} {
		if !strings.Contains(messageSchema, want) {
			t.Fatalf("messagecenter schema missing %q", want)
		}
	}
	for _, want := range []string{
		"/api/message-center/notifications",
		"/api/message-center/notifications/:id/read",
	} {
		if !strings.Contains(messageModule, want) {
			t.Fatalf("message center module missing route %q", want)
		}
	}
	for _, want := range []string{
		"CurrentEmployeeID",
	} {
		if !strings.Contains(messageAPI, want) {
			t.Fatalf("message center API missing %q", want)
		}
	}
	for _, want := range []string{
		"messagecenterapp.NewService",
		"messagecenterhttp.RegisterRoutes",
		"MessageCenter:",
	} {
		if !strings.Contains(appRoutes, want) {
			t.Fatalf("app route wiring missing %q", want)
		}
	}
	if !strings.Contains(authz, `"/api/message-center"`) || !strings.Contains(authz, `"orders.read"`) {
		t.Fatal("message center API must require orders.read")
	}
	if !strings.Contains(salesOrderAPI, "publishOrderCreated") {
		t.Fatal("sales order API must publish order created messages")
	}
	if !strings.Contains(miniAPI, "publishMiniOrderCreated") {
		t.Fatal("mini order API must publish order created messages")
	}
	for _, want := range []string{
		"order.created",
		"orders_scope",
		"highlight_order_id",
	} {
		if !strings.Contains(salesOrderAPI, want) || !strings.Contains(miniAPI, want) {
			t.Fatalf("sales and mini order APIs must share order notification marker %q", want)
		}
	}
	for _, want := range []string{
		"global-notification",
		"fetchERPNotifications",
		"markNotificationRead",
		"openNotification",
		"highlight_order_id",
	} {
		if !strings.Contains(appVue, want) {
			t.Fatalf("App.vue missing global notification behavior %q", want)
		}
	}
	for _, want := range []string{
		"scope-tabs",
		"履约客户订单",
		"highlight-new",
		"state-unproduced",
		"state-unshipped",
		"state-unpaid",
	} {
		if !strings.Contains(ordersView, want) {
			t.Fatalf("OrdersView.vue missing order scope/highlight marker %q", want)
		}
	}
	for _, want := range []string{
		`case "mine"`,
		`case "fulfillment"`,
		"customer_erp_user_bindings",
		"portal_service_code",
		"customer_type",
	} {
		if !strings.Contains(orderQueries, want) {
			t.Fatalf("order queries missing fulfillment scope marker %q", want)
		}
	}
	for _, want := range []string{
		`row.CustomerType == "wholesale"`,
		"hasActiveERPBinding",
	} {
		if !strings.Contains(customerFulfillmentAPI, want) {
			t.Fatalf("customer fulfillment API missing picker filter %q", want)
		}
	}
	if !strings.Contains(customerFulfillmentRepo, "portal_service_code IN ('direct_ship','processing_ship')") {
		t.Fatal("customer fulfillment overview must include customer portal direct ship orders")
	}
}

func TestMessageCenterOrderNotificationRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-174-MESSAGE-CENTER-ORDER-NOTIFICATIONS",
		"DEV-174-MESSAGE-CENTER-ORDER-NOTIFICATIONS",
		"UT-174-MESSAGE-CENTER-ORDER-NOTIFICATIONS",
		"API-174-MESSAGE-CENTER-ORDER-NOTIFICATIONS",
		"REV-174-MESSAGE-CENTER-ORDER-NOTIFICATIONS",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
