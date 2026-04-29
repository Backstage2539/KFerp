package support

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestBasicAuthAllowsPublicBeanListWithoutCredentials(t *testing.T) {
	e := echo.New()
	e.Use(BasicAuth("order", "secret", "public", nil))
	e.GET("/public/bean-list/:list_type", func(c echo.Context) error {
		return c.String(http.StatusOK, "public bean list")
	})
	e.GET("/api/costing/bean-list", func(c echo.Context) error {
		return c.String(http.StatusOK, "private api")
	})

	req := httptest.NewRequest(http.MethodGet, "/public/bean-list/commercial", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "public bean list" {
		t.Fatalf("public route status = %d body = %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/costing/bean-list", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("private route status = %d, want 401", rec.Code)
	}
}

func TestLogoutEndpointRequiresAuthentication(t *testing.T) {
	if isAuthPublicPath("/api/auth/logout") {
		t.Fatal("/api/auth/logout must stay behind auth middleware so only the current session can be revoked")
	}
}

func TestVueShellCanLoadBeforeBearerAPIAuth(t *testing.T) {
	for _, path := range []string{"/vue-shell", "/vue-shell/assets/index.js"} {
		if !isPublicUnauthenticatedPath(path) {
			t.Fatalf("%s must be public so mobile login can load the Vue shell before API calls attach Bearer token", path)
		}
	}
}

func TestBearerTokenFromHeader(t *testing.T) {
	if got := bearerTokenFromHeader("Bearer abc123"); got != "abc123" {
		t.Fatalf("token=%q", got)
	}
	if got := bearerTokenFromHeader("bearer  abc123  "); got != "abc123" {
		t.Fatalf("lowercase token=%q", got)
	}
	if got := bearerTokenFromHeader("Basic abc123"); got != "" {
		t.Fatalf("basic token=%q, want empty", got)
	}
}
