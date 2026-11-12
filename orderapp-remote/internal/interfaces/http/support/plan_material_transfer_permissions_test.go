package support

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
)

func TestPlanMaterialTransferRequiresProductionAndStockPermissions(t *testing.T) {
	for _, tc := range []struct{ method, path, production, stock string }{
		{http.MethodGet, "/api/production-plans/25/material-transfer-options", "production.read", "stock.read"},
		{http.MethodPost, "/api/production-plans/25/material-transfers/preview", "production.read", "stock.read"},
		{http.MethodPost, "/api/production-plans/25/material-transfers", "production.run", "stock.write"},
		{http.MethodPost, "/app/api/production-plans/25/material-transfers", "production.run", "stock.write"},
		{http.MethodPost, "/api/production-plans/25/semi-finished-stock/transfer-to-wip", "production.run", "stock.write"},
		{http.MethodPost, "/app/api/production-plans/25/semi-finished-stock/transfer-to-wip", "production.run", "stock.write"},
		{http.MethodPost, "/api/production-plans/25/submit", "production.run", ""},
	} {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.production {
			t.Errorf("%s production permission = %q, want %q", tc.path, got, tc.production)
		}
		if got := operationCompletionStockPermission(tc.method, tc.path); got != tc.stock {
			t.Errorf("%s stock permission = %q, want %q", tc.path, got, tc.stock)
		}
	}
}

func TestPlanMaterialTransferHTTPEnforcesBothPermissions(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		permissions        []string
		want               int
	}{
		{"inventory requires both read", http.MethodGet, "/api/production-plans/25/material-transfer-options", []string{"production.read"}, 403},
		{"preview requires stock read", http.MethodPost, "/api/production-plans/25/material-transfers/preview", []string{"production.read"}, 403},
		{"preview allows both reads", http.MethodPost, "/api/production-plans/25/material-transfers/preview", []string{"production.read", "stock.read"}, 200},
		{"create requires stock write", http.MethodPost, "/api/production-plans/25/material-transfers", []string{"production.run", "stock.read"}, 403},
		{"create requires production run", http.MethodPost, "/api/production-plans/25/material-transfers", []string{"production.read", "stock.write"}, 403},
		{"create allows both writes", http.MethodPost, "/api/production-plans/25/material-transfers", []string{"production.run", "stock.write"}, 200},
		{"direct requires stock write", http.MethodPost, "/api/production-plans/25/semi-finished-stock/transfer-to-wip", []string{"production.run", "stock.read"}, 403},
		{"direct requires production run", http.MethodPost, "/api/production-plans/25/semi-finished-stock/transfer-to-wip", []string{"production.read", "stock.write"}, 403},
		{"direct allows both writes", http.MethodPost, "/api/production-plans/25/semi-finished-stock/transfer-to-wip", []string{"production.run", "stock.write"}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(3)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: tc.permissions}}))
			called := false
			e.Add(tc.method, tc.path, func(c echo.Context) error { called = true; return c.NoContent(200) })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.want || called != (tc.want == 200) {
				t.Fatalf("status=%d called=%v, want=%d", rec.Code, called, tc.want)
			}
		})
	}
}

func TestPlanPackagingRemainderPermissions(t *testing.T) {
	for _, path := range []string{"/api/packaging-remainders", "/app/api/packaging-remainders"} {
		if got := requiredPermissionForRequest("GET", path); got != "production.read" {
			t.Errorf("%s requires production.read, got %s", path, got)
		}
	}
	for _, row := range []struct{ method, path, stock string }{
		{"GET", "/api/packaging-remainders", "stock.read"},
		{"GET", "/api/production-plans/25/packaging-remainders", "stock.read"},
		{"POST", "/api/production-plans/25/packaging-remainders", "stock.write"},
		{"POST", "/api/production-plans/25/packaging-remainders/recheck", "stock.write"},
	} {
		if got := operationCompletionStockPermission(row.method, row.path); got != row.stock {
			t.Errorf("%s %s = %s, want %s", row.method, row.path, got, row.stock)
		}
	}
}
