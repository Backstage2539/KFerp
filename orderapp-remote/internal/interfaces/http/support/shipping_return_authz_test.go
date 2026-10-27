package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"testing"
)

func TestShippingReturnPermissions(t *testing.T) {
	for _, c := range []struct{ method, path, want string }{{http.MethodPost, "/api/orders/shipping-return-rematch", "orders.write"}, {http.MethodGet, "/api/orders/shipping-return-candidates", "orders.read"}, {http.MethodPost, "/api/orders/1/shipping-return-selection", "orders.write"}, {http.MethodPut, "/api/settings/logistics/shipping-return-templates", "settings.write"}} {
		if got := requiredPermissionForRequest(c.method, c.path); got != c.want {
			t.Fatalf("%+v got %s", c, got)
		}
	}
}

func TestShippingReturnRejectsCustomerEvenWithOrderPermissions(t *testing.T) {
	for _, customer := range []bool{true, false} {
		account := "internal_employee"
		if customer {
			account = AccountTypeChannelCustomer
		}
		e := echo.New()
		authz := &fakeAuthzService{actor: authzapp.Actor{AccountType: account, Permissions: []string{"orders.read", "orders.write", "settings.write"}}}
		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
		})
		e.Use(AuthorizationMiddleware(authz))
		for _, item := range []struct{ method, path string }{{"POST", "/api/orders/shipping-return-rematch"}, {"GET", "/api/orders/shipping-return-candidates"}, {"GET", "/api/orders/shipping-return-templates"}, {"POST", "/api/orders/1/shipping-return-selection"}, {"POST", "/api/orders/shipping-tracking-excel"}, {"PUT", "/api/settings/logistics/shipping-return-templates"}} {
			e.Add(item.method, item.path, func(c echo.Context) error { return c.NoContent(200) })
			out := httptest.NewRecorder()
			e.ServeHTTP(out, httptest.NewRequest(item.method, item.path, nil))
			want := 200
			if customer {
				want = 403
			}
			if out.Code != want {
				t.Fatalf("customer=%v %s status=%d want=%d", customer, item.path, out.Code, want)
			}
		}
	}
}
