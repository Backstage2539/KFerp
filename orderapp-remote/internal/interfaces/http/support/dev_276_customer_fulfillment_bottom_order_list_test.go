package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev276CustomerFulfillmentBottomOrderListRequirementSeeds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("internal", "interfaces", "http", "support", "req_store.go"))
	if err != nil {
		t.Fatalf("read req_store.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PR-276-CUSTOMER-FULFILLMENT-BOTTOM-ORDER-LIST",
		"DEV-276-01",
		"DEV-276-02",
		"UT-276-01",
		"API-276-01",
		"REV-276-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer fulfillment bottom order list seed missing %q", want)
		}
	}
}

func TestDev276CustomerFulfillmentBottomOrderListSourceWiring(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	for _, want := range []string{
		"履约客户订单",
		"订单费用",
		"fetchCustomerFulfillmentOrders",
		"fetchCustomerFulfillmentOrderDetail",
		"SalesOrderView",
		"DeliveryNoteView",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CustomerFulfillmentView missing %q", want)
		}
	}

	api := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js")))
	for _, want := range []string{
		"/api/orders",
		"scope",
		"customer_id",
		"/api/orders/${Number(orderId)}/detail",
	} {
		if !strings.Contains(api, want) {
			t.Fatalf("customer-fulfillment API missing %q", want)
		}
	}

	orderQueries := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_queries.go")))
	for _, want := range []string{
		"total_amount",
		"shipping_amount",
		"discount_amount",
	} {
		if !strings.Contains(orderQueries, want) {
			t.Fatalf("order_queries.go missing %q", want)
		}
	}
}

func TestDev276CustomerFulfillmentBottomOrderListDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range []string{
			"履约客户订单",
			"订单费用",
			"微信分享",
			"销售单",
			"出库单",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing customer fulfillment order list marker %q", path, want)
			}
		}
	}

	for _, path := range []string{
		filepath.Join("..", "REQUIREMENTS.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("..", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		for _, want := range []string{
			"履约客户订单",
			"订单费用",
			"微信分享销售单",
			"微信分享出库单",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing customer fulfillment order list doc marker %q", path, want)
			}
		}
	}
}
