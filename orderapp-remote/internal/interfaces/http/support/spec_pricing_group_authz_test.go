package support

import (
	"net/http"
	"testing"
)

func TestSpecPricingGroupPermissions(t *testing.T) {
	for _, path := range []string{"/api/spec-pricing-groups", "/api/spec-pricing-groups/options", "/api/spec-pricing-groups/12"} {
		if !isProductCatalogFeatureSelectionReadRequest(http.MethodGet, path) {
			t.Errorf("%s: expected product or costing read permission", path)
		}
		if got := requiredPermissionForRequest(http.MethodPut, path); got != "products.write" {
			t.Errorf("%s: PUT permission = %q", path, got)
		}
	}
}
