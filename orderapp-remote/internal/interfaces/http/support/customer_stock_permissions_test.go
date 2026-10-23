package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"testing"
)

func TestCustomerStockConfigurationAndEmployeeDirectShipPermissions(t *testing.T) {
	cases := []struct{ method, path, permission string }{
		{"GET", "/api/customer-fulfillment/74/settings", "customers.read"},
		{"GET", "/api/customer-fulfillment/74/overview", "stock.read"},
		{"PUT", "/api/customer-fulfillment/74/settings", "customers.write"},
		{"POST", "/api/customer-fulfillment/74/fees/preview", "customers.read"},
		{"PUT", "/api/customer-fulfillment/74/orders/5/fees", "customers.write"},
		{"GET", "/api/settings/logistics/shipping-templates", "settings.write"},
		{"GET", "/api/settings/logistics/fulfillment-defaults", "settings.write"},
		{"PUT", "/api/settings/logistics/fulfillment-defaults", "settings.write"},
		{"POST", "/api/settings/logistics/shipping-templates", "settings.write"},
		{"GET", "/api/customer-fulfillment/74/direct-ship/catalog", "stock.write"},
		{"POST", "/api/customer-fulfillment/74/direct-ship/preview", "stock.write"},
	}
	for _, tc := range cases {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.permission {
			t.Errorf("%s %s permission=%s want=%s", tc.method, tc.path, got, tc.permission)
		}
		for _, account := range []string{AccountTypeChannelCustomer, AccountTypeInternalEmployee} {
			e := echo.New()
			az := &fakeAuthzService{actor: authzapp.Actor{AccountType: account, Permissions: []string{tc.permission}}}
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(3)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(az))
			e.Any("/*", func(c echo.Context) error { return c.NoContent(200) })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			want := http.StatusOK
			if account == AccountTypeChannelCustomer {
				want = http.StatusForbidden
			}
			if rec.Code != want {
				t.Errorf("account=%s path=%s status=%d want=%d", account, tc.path, rec.Code, want)
			}
		}
	}
}
