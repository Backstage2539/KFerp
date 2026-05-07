package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentUsesCustomerPickerInsteadOfManualID(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CustomerFulfillmentView.vue")))
	api := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "api", "customer-fulfillment.js")))
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))

	for _, want := range []string{
		"SearchableSelect",
		"fetchCustomerFulfillmentCustomers",
		"activeCustomerFulfillmentCustomers",
		"customerFulfillmentCustomerOptionLabel",
		"选择客户",
	} {
		if !strings.Contains(view, want) && !strings.Contains(api, want) {
			t.Fatalf("customer fulfillment customer picker missing %q", want)
		}
	}
	for _, forbidden := range []string{">客户 ID<", "例如 147"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("customer fulfillment view must not ask operators for raw IDs; found %q", forbidden)
		}
	}
	for _, want := range []string{"PR-159", "DEV-159-01", "UT-159-01", "API-159-01", "REV-159-01"} {
		if !strings.Contains(reqStore, want) {
			t.Fatalf("requirement seed missing %s", want)
		}
	}
}

func TestCustomerFulfillmentCustomerPickerPermission(t *testing.T) {
	if got := requiredPermissionForRequest("GET", "/api/customer-fulfillment/customers"); got != "stock.read" {
		t.Fatalf("GET /api/customer-fulfillment/customers permission = %q, want stock.read", got)
	}
}
