package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"strings"
	"testing"
)

func TestCapacityBatchEndpointsKeepProductionAndMasterDataPermissions(t *testing.T) {
	for _, route := range []struct{ method, path, permission string }{
		{http.MethodGet, "/api/production-plans/11", "production.read"},
		{http.MethodPost, "/api/production-plans/11/operation-splits/preview", "production.run"},
		{http.MethodPost, "/api/production-plans/11/operation-splits", "production.run"},
		{http.MethodPatch, "/api/production-plans/11/draft", "production.run"},
		{http.MethodPost, "/api/manufacturing-workstations", "bom.write"},
		{http.MethodPost, "/api/manufacturing-workstation-capacities", "bom.write"},
	} {
		for _, allowed := range []bool{false, true} {
			permissions := []string{"orders.read"}
			if allowed {
				permissions = append(permissions, route.permission)
			}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(719)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: permissions}}))
			called := false
			e.Add(route.method, route.path, func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"items":[{"planned_batch_count":6}]}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			want := http.StatusForbidden
			if allowed {
				want = http.StatusOK
			}
			if rec.Code != want || called != allowed {
				t.Errorf("%s %s allowed=%v: status=%d handler=%v", route.method, route.path, allowed, rec.Code, called)
			}
		}
	}
}
