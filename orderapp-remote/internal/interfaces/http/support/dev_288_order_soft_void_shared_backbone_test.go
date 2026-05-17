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
		`e.POST("/api/orders/:id/unvoid", h.unvoid)`,
		"sales.Void",
		"sales.Unvoid",
	} {
		if !strings.Contains(orderAPI, want) {
			t.Fatalf("order API missing shared soft void marker %q", want)
		}
	}
	for _, want := range []string{
		"失效",
		"恢复",
		"voidOrder(row)",
		"restoreOrder(row)",
		"`/api/orders/${id}/void`",
		"`/api/orders/${id}/unvoid`",
	} {
		if !strings.Contains(ordersView, want) {
			t.Fatalf("OrdersView.vue missing soft void marker %q", want)
		}
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
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("..", "OP_MANUAL_CUSTOMER_PORTAL.md"),
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
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order soft void doc marker %q", path, want)
			}
		}
	}
}
