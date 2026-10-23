package support

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"testing"
)

func TestBatchMaterialIssueRequiresBothProductionAndStockPermissions(t *testing.T) {
	for _, tc := range []struct {
		name, path  string
		permissions []string
		want        int
	}{
		{"preview production only", "preview", []string{"production.read"}, 403},
		{"preview stock only", "preview", []string{"stock.read"}, 403},
		{"preview both", "preview", []string{"production.read", "stock.read"}, 200},
		{"submit production only", "submit", []string{"production.run"}, 403},
		{"submit stock only", "submit", []string{"stock.write"}, 403},
		{"submit both", "submit", []string{"production.run", "stock.write"}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: tc.permissions}}))
			path := "/api/production/workstation/material-issue/" + tc.path
			e.POST(path, func(c echo.Context) error { return c.NoContent(200) })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
			if rec.Code != tc.want {
				t.Fatalf("status %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
