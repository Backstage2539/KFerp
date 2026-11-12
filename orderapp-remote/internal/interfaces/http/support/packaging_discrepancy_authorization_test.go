package support

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
)

func TestPackagingDiscrepancyResolutionRequiresBothPermissions(t *testing.T) {
	for _, prefix := range []string{"", "/app"} {
		for _, tc := range []struct {
			name        string
			permissions []string
			want        int
		}{
			{"no permissions", nil, 403},
			{"only production", []string{"production.run"}, 403},
			{"only warehouse", []string{"stock.write"}, 403},
			{"both", []string{"production.run", "stock.write"}, 200},
		} {
			t.Run(prefix+"/"+tc.name, func(t *testing.T) {
				path := prefix + "/api/production-plans/25/packaging-remainders/resolve-discrepancies"
				e := echo.New()
				e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error { c.Set("employee_id", int64(3)); return next(c) }
				})
				e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: tc.permissions}}))
				called := false
				e.POST(path, func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
				if rec.Code != tc.want || called != (tc.want == 200) {
					t.Fatalf("path=%s status=%d called=%v want=%d", path, rec.Code, called, tc.want)
				}
			})
		}
	}
}
