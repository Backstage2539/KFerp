package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev288OrderSoftVoidSharedBackboneSources(t *testing.T) {
	orderAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	ordersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	orderQueries := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_queries.go")))
	customerPortalRepo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerportal", "business_repository.go")))
	fulfillmentAPI := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js")))

	for _, want := range []string{
		`e.POST("/api/orders/:id/void", h.void)`,
		`e.POST("/api/orders/void", h.voidMany)`,
		"sales.Void",
		"sales.VoidMany",
	} {
		if !strings.Contains(orderAPI, want) {
			t.Fatalf("order API missing shared soft void marker %q", want)
		}
	}
	for _, forbidden := range []string{
		`/api/orders/:id/unvoid`,
		"func (h orderAPIHandler) unvoid",
		"sales.Unvoid",
	} {
		if strings.Contains(orderAPI, forbidden) {
			t.Fatalf("order API must not expose restore marker %q", forbidden)
		}
	}
	for _, want := range []string{
		"失效",
		"批量失效",
		"复制",
		"voidOrder(row)",
		"voidSelectedOrders",
		"togglePageOrderSelection",
		"pageSelectionState",
		"selectedVoidableOrderIDs",
		"当前页正常订单全选",
		"copyOrder(row)",
		"`/api/orders/${id}/void`",
		"`/api/orders/void`",
		"失效后不可恢复",
	} {
		if !strings.Contains(ordersView, want) {
			t.Fatalf("OrdersView.vue missing soft void marker %q", want)
		}
	}
	for _, forbidden := range []string{"restoreOrder", "unvoid"} {
		if strings.Contains(ordersView, forbidden) {
			t.Fatalf("OrdersView.vue must not expose restore marker %q", forbidden)
		}
	}
	if strings.Contains(ordersView, "选择本页正常订单") {
		t.Fatalf("OrdersView.vue must use header checkbox instead of the old select-current-page button")
	}
	for _, want := range []string{
		"o.is_void = true",
		"o.is_void = false",
	} {
		if !strings.Contains(orderQueries, want) {
			t.Fatalf("shared order query missing void filter marker %q", want)
		}
	}
	for _, want := range []string{
		`where := []string{"o.customer_id=$1", "o.is_void=false"}`,
		"FROM %s.orders o",
		"WHERE id=$1 AND customer_id=$2 AND is_void=false",
	} {
		if !strings.Contains(customerPortalRepo, want) {
			t.Fatalf("customer portal order backbone missing marker %q", want)
		}
	}
	for _, want := range []string{
		"fetchCustomerFulfillmentOrders",
		"`/api/orders?${params.toString()}`",
		"scope', 'fulfillment'",
	} {
		if !strings.Contains(fulfillmentAPI, want) {
			t.Fatalf("customer fulfillment order API missing shared order marker %q", want)
		}
	}
}

func TestDev288OrderSoftVoidSharedBackboneReqAndDocs(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-288-ORDER-SOFT-VOID-SHARED-BACKBONE",
		"DEV-288-ORDER-SOFT-VOID-SHARED-BACKBONE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}

	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"订单失效",
			"履约客户订单",
			"小程序订单",
			"已失效",
			"不可恢复",
			"复制",
			"批量失效",
			"当前页",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order soft void doc marker %q", path, want)
			}
		}
	}
}
