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
		{"draft production only", "draft", []string{"production.run"}, 403},
		{"draft stock only", "draft", []string{"stock.write"}, 403},
		{"draft both", "draft", []string{"production.run", "stock.write"}, 200},
		{"confirm production only", "confirm", []string{"production.run"}, 403},
		{"confirm stock only", "confirm", []string{"stock.write"}, 403},
		{"confirm both", "confirm", []string{"production.run", "stock.write"}, 200},
		{"cancel production only", "cancel", []string{"production.run"}, 403},
		{"cancel stock only", "cancel", []string{"stock.write"}, 403},
		{"cancel both", "cancel", []string{"production.run", "stock.write"}, 200},
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

func TestReplenishmentRecoveryReadDoesNotRequireWrite(t *testing.T) {
	for _, tc := range []struct {
		name        string
		permissions []string
		want        int
	}{{"both reads", []string{"production.read", "stock.read"}, 200}, {"missing stock", []string{"production.read"}, 403}, {"missing production", []string{"stock.read"}, 403}} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
			})
			e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: tc.permissions}}))
			path := "/api/production/workstation/material-issue/draft"
			e.GET(path, func(c echo.Context) error { return c.NoContent(200) })
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != tc.want {
				t.Fatalf("status %d want %d", rec.Code, tc.want)
			}
		})
	}
}
