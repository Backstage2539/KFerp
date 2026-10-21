package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"testing"
)

func TestVirtualWarehouseManagementIsInternalAndPermissionScoped(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		permission := "stock.write"
		if method == "GET" {
			permission = "stock.read"
		}
		for _, account := range []string{AccountTypeChannelCustomer, AccountTypeInternalEmployee} {
			for _, allowed := range []bool{false, true} {
				e := echo.New()
				actor := authzapp.Actor{AccountType: account}
				if allowed {
					actor.Permissions = []string{permission}
				}
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error { c.Set("employee_id", int64(4)); return next(c) }
				})
				e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: actor}))
				e.Any("/*", func(c echo.Context) error { return c.NoContent(200) })
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest(method, "/api/stock/virtual-warehouses/virtual_1", nil))
				want := http.StatusForbidden
				if account == AccountTypeInternalEmployee && allowed {
					want = 200
				}
				if rec.Code != want {
					t.Fatalf("%s %s allowed=%v: status=%d want=%d", method, account, allowed, rec.Code, want)
				}
			}
		}
	}
}
