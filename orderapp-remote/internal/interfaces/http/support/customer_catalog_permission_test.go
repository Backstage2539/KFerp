package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"testing"
)

func TestCustomerCatalogPermissions(t *testing.T) {
	for _, p := range []string{"/api/product-settings/customer-catalog/copy", "/api/product-settings/customer-catalog/nodes/1", "/api/product-customer-references/1"} {
		if got := requiredPermissionForRequest(http.MethodPost, p); got != "products.write" {
			t.Fatalf("%s: %s", p, got)
		}
	}
	if !isProductCatalogFeatureSelectionReadRequest(http.MethodGet, "/api/product-settings/customer-catalog") {
		t.Fatal("catalog read must allow costing or products read")
	}
	if got := requiredPermissionForRequest(http.MethodPost, "/api/product-settings/customer-catalog/migration"); got != "auth.manage" {
		t.Fatal(got)
	}
}

func TestCustomerCatalogMiddlewareDeniesCustomersAndEnforcesStaffPermissions(t *testing.T) {
	for _, tc := range []struct {
		account, permission, method, path string
		status                            int
	}{
		{AccountTypeChannelCustomer, "products.write", http.MethodPost, "/api/product-settings/customer-catalog/copy", 403},
		{AccountTypeChannelCustomer, "products.read", http.MethodGet, "/api/product-settings/customer-catalog?customer_id=43", 403},
		{"employee", "products.read", http.MethodPost, "/api/product-settings/customer-catalog/copy", 403},
		{"employee", "costing.read", http.MethodGet, "/api/product-settings/customer-catalog?customer_id=42", 200},
		{"employee", "costing.read", http.MethodGet, "/api/product-customer-references?customer_id=42", 200},
		{"employee", "products.write", http.MethodPost, "/api/product-settings/customer-catalog/copy", 200},
		{"employee", "products.write", http.MethodPost, "/api/product-settings/customer-catalog/migration", 403},
	} {
		t.Run(tc.account+tc.permission+tc.path, func(t *testing.T) {
			e := echo.New()
			authz := &fakeAuthzService{actor: authzapp.Actor{AccountType: tc.account, Permissions: []string{tc.permission}}}
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(3)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(authz))
			e.Any("/*", func(c echo.Context) error { return c.NoContent(200) })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}
