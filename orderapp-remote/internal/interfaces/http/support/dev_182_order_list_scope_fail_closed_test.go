package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderListScopeFailClosedEvidenceExists(t *testing.T) {
	orderAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	orderAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	orderScopeLib := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-scope.js")))
	orderScopeTest := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-scope.test.js")))
	ordersView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"validOrderListScope",
		`case "", "all", "mine", "fulfillment"`,
		`"error": "invalid scope"`,
	} {
		if !strings.Contains(orderAPI, want) {
			t.Fatalf("order API missing fail-closed scope marker %q", want)
		}
	}
	for _, want := range []string{
		"TestOrderAPIListRejectsInvalidScope",
		"fulfillment_typo",
		"orders API must reject invalid scope before querying repository",
	} {
		if !strings.Contains(orderAPITest, want) {
			t.Fatalf("order API test missing fail-closed scope marker %q", want)
		}
	}
	for _, want := range []string{
		"validOrderListScopes",
		"return normalized",
	} {
		if !strings.Contains(orderScopeLib, want) {
			t.Fatalf("order scope frontend helper missing marker %q", want)
		}
	}
	for _, want := range []string{
		"orderListScopeForRequest('fulfillment_typo')",
		"preserves invalid route values",
	} {
		if !strings.Contains(orderScopeTest, want) {
			t.Fatalf("order scope frontend test missing marker %q", want)
		}
	}
	if !strings.Contains(ordersView, "orderListScopeForRequest") || strings.Contains(ordersView, "normalizeScope") {
		t.Fatal("OrdersView must preserve invalid route scope through orderListScopeForRequest instead of normalizing it to all")
	}
	for _, want := range []string{
		"PR-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
		"/api/orders?scope=fulfillment_typo",
		`400 {"error":"invalid scope"}`,
		"orderListScopeForRequest",
		"ORDER_SCOPE_BROWSER_FAIL_CLOSED_OK",
		"当前结论：未完成",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestOrderListScopeFailClosedRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
		"DEV-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
		"UT-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
		"API-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
		"REV-182-ORDER-LIST-SCOPE-FAIL-CLOSED",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}

func TestOrderListScopeFailClosedManualsAndRequirementDocs(t *testing.T) {
	for _, path := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, path))
		for _, want := range []string{
			"invalid scope",
			"fulfillment",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order scope fail-closed marker %q", path, want)
			}
		}
	}
}
