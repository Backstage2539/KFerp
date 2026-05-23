package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev342OrderEntryCommonProductsSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
		"DEV-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
		"UT-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
		"API-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
		"REV-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-342 requirement seed missing %q", want)
		}
	}
}

func TestDev342OrderEntryCommonProductsWiring(t *testing.T) {
	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"customerProductUsages = ref([])",
		"customerProductUsages.value = data.customer_product_usages || []",
		"sortProductsByCustomerUsage",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("OrderEntryView.vue missing common product sort marker %q", want)
		}
	}
	orderLib := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"export function sortProductsByCustomerUsage",
		"last_order_date",
	} {
		if !strings.Contains(orderLib, want) {
			t.Fatalf("order-entry.js missing common product sort marker %q", want)
		}
	}
	orderAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api.go")))
	if !strings.Contains(orderAPI, "customer_product_usages") {
		t.Fatal("order form API must return customer_product_usages")
	}
}

func TestDev342OrderEntryCommonProductsDocs(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "acceptance", "2026-05-23-order-entry-common-product-sort.md"),
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"PR-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT",
			"常用商品",
			"历史订单",
		} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-342 documentation marker %q", rel, want)
			}
		}
	}
}
