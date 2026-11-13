package support

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
)

func TestWIPTopUpMiddlewareRequiresBothProductionAndStockPermissions(t *testing.T) {
	for _, prefix := range []string{"", "/app"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			production, stock := "production.read", "stock.read"
			if method == http.MethodPost {
				production, stock = "production.run", "stock.write"
			}
			cases := []struct {
				name        string
				permissions []string
				want        int
			}{
				{"neither", nil, http.StatusForbidden},
				{"only production", []string{production}, http.StatusForbidden},
				{"only stock", []string{stock}, http.StatusForbidden},
				{"both", []string{production, stock}, http.StatusOK},
			}
			for _, tc := range cases {
				t.Run(prefix+"/"+method+"/"+tc.name, func(t *testing.T) {
					path := prefix + "/api/job-cards/120/wip-top-up"
					e := echo.New()
					e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
						return func(c echo.Context) error { c.Set("employee_id", int64(3)); return next(c) }
					})
					e.Use(AuthorizationMiddleware(&fakeAuthzService{actor: authzapp.Actor{Permissions: tc.permissions}}))
					called := false
					e.Add(method, path, func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
					if rec.Code != tc.want || called != (tc.want == http.StatusOK) {
						t.Fatalf("method=%s path=%s status=%d called=%v want=%d body=%s", method, path, rec.Code, called, tc.want, rec.Body.String())
					}
				})
			}
		}
	}
}
