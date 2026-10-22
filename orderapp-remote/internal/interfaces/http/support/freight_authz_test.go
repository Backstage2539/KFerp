package support

import (
	"net/http"
	"testing"
)

func TestFreightAuthorizationSeparatesPortalAndInternalPolicies(t *testing.T) {
	for _, path := range []string{"/api/customer-processing/portal/freight/preview", "/api/mini/employee/freight-preview", "/api/mini/shipping-regions"} {
		if isInternalCustomerStockRequest(path) {
			t.Fatalf("customer or token route misclassified internal: %s", path)
		}
	}
	for _, path := range []string{"/api/customer-fulfillment/501/shipping-policies", "/api/customer-fulfillment/501/freight/preview", "/api/settings/logistics/freight-imports/preview"} {
		if !isInternalCustomerStockRequest(path) {
			t.Fatalf("internal route missing guard: %s", path)
		}
	}
	if p := requiredPermissionForRequest(http.MethodPut, "/api/customer-fulfillment/501/shipping-policies/sale"); p != "customers.write" {
		t.Fatal(p)
	}
	if p := requiredPermissionForRequest(http.MethodPost, "/api/customer-fulfillment/501/freight/preview"); p != "customers.read" {
		t.Fatal(p)
	}
}

func TestOrderFreightUsesOrderWritePermission(t *testing.T) {
	for _, path := range []string{"/api/order/freight/preview", "/api/order/freight/regions", "/api/order/shipping-destination/resolve"} {
		if requiredPermissionForRequest(http.MethodPost, path) != "orders.write" {
			t.Fatal(path)
		}
	}
}

func TestSystemFreightDefaultRequiresSettingsPermission(t *testing.T) {
	path := "/api/settings/logistics/default-shipping-policy"
	if !isInternalCustomerStockRequest(path) {
		t.Fatal("customer portal must not access system defaults")
	}
	if requiredPermissionForRequest(http.MethodPut, path) != "settings.write" {
		t.Fatal("system default must require settings.write")
	}
}
