package support

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	for _, path := range []string{"/vue-shell", "/vue-shell/assets/index.js", "/assets/sales_order_assets/payment_code/qr.pic", "/assets/mall_products/12/hero.png"} {
		if !isPublicUnauthenticatedPath(path) {
			t.Fatalf("%s must be public so mobile login can load the Vue shell before API calls attach Bearer token", path)
		}
	}
}

func TestMiniAPIBypassesBasicAuthForMiniTokenHandlers(t *testing.T) {
	if !isPublicUnauthenticatedPath("/api/mini/me") || !isPublicUnauthenticatedPath("/api/mini/login") {
		t.Fatal("/api/mini/* must bypass BasicAuth so mini handlers can enforce mini token auth")
	}
}

func TestExternalSharePagesArePublicButShareCreationRequiresOrderWrite(t *testing.T) {
	for _, path := range []string{"/share/abc123", "/share/abc123/file"} {
		if !isPublicUnauthenticatedPath(path) {
			t.Fatalf("%s must be public so WeChat recipients can open shared resources", path)
		}
	}
	if isPublicUnauthenticatedPath("/api/share-resources") {
		t.Fatal("/api/share-resources must stay authenticated because it creates external share links")
	}
	if got := requiredPermissionForRequest(http.MethodPost, "/api/share-resources"); got != "orders.write" {
		t.Fatalf("POST /api/share-resources permission=%q, want orders.write", got)
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

func TestBasicAuthChallengeIsNotAdvertisedForAPIOrBearerFailures(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		requestPath string
		authz       string
		want        bool
	}{
		{name: "page request can trigger outer browser auth", path: "/orders", requestPath: "/app/orders", want: true},
		{name: "api path returns JSON unauthorized", path: "/api/orders", requestPath: "/api/orders", want: false},
		{name: "app prefixed api path returns JSON unauthorized", path: "/api/auth/me", requestPath: "/app/api/auth/me", want: false},
		{name: "expired bearer never triggers browser basic auth prompt", path: "/api/auth/me", requestPath: "/app/api/auth/me", authz: "Bearer stale-token", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAdvertiseBasicAuthChallenge(tc.path, tc.requestPath, tc.authz)
			if got != tc.want {
				t.Fatalf("shouldAdvertiseBasicAuthChallenge=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestPasswordLoginIdentifierSupportsUsernameOrPhone(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  loginReq
		want string
	}{
		{name: "legacy phone field", req: loginReq{Mode: "password", Phone: "13800138075"}, want: "13800138075"},
		{name: "login field username", req: loginReq{Mode: "password", Login: "Van"}, want: "Van"},
		{name: "username fallback", req: loginReq{Mode: "password", Username: "管理员"}, want: "管理员"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := passwordLoginIdentifier(tc.req)
			if got != tc.want {
				t.Fatalf("identifier=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoginPageSupportsUsernamePasswordAndDoesNotRequirePhoneForPassword(t *testing.T) {
	src := readSupportTestFile(t, "templates/login.html")
	for _, want := range []string{
		"<title>系统登录</title>",
		"用户名/手机号+密码",
		"getLoginIdentifier",
		"validatePasswordLogin",
		"body.login",
		"/vue-shell?fresh_login=1",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("login page missing %q", want)
		}
	}
}

func TestLoginPageKeepsAppPrefixDuringMobileLogin(t *testing.T) {
	src := readSupportTestFile(t, "templates/login.html")
	for _, want := range []string{
		"function appPath(path)",
		"fetch(appPath('/api/auth/sms/send')",
		"fetch(appPath('/api/auth/login')",
		"window.location.href=appPath('/vue-shell?fresh_login=1')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("login page must keep /app prefix, missing %q", want)
		}
	}
	for _, bad := range []string{
		"fetch('/api/auth/sms/send'",
		"fetch('/api/auth/login'",
		"window.location.href='/vue-shell?fresh_login=1'",
	} {
		if strings.Contains(src, bad) {
			t.Fatalf("login page still uses root-scoped mobile login path %q", bad)
		}
	}
}

func TestCoreRedirectsUseRelativeLocationsForAppPrefix(t *testing.T) {
	e := echo.New()
	registerCoreRoutes(e, nil, "public")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "orders" {
		t.Fatalf("Location=%q, want relative orders", got)
	}
}

func TestVueShellRedirectUsesRelativeLocationForAppPrefix(t *testing.T) {
	e := echo.New()
	e.GET("/orders", func(c echo.Context) error {
		return VueShellRedirect(c, "orders")
	})

	req := httptest.NewRequest(http.MethodGet, "/orders?q=SO-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != "vue-shell?view=orders&q=SO-1" {
		t.Fatalf("Location=%q, want relative vue-shell redirect", got)
	}
}

func TestVueShellRedirectsToLoginWithoutStoredToken(t *testing.T) {
	src := readSupportTestFile(t, "frontend-vue-shell/src/App.vue")
	for _, want := range []string{
		"hasStoredAuthToken()",
		"redirectToLogin()",
		"clearStoredAuthToken()",
		"appURL('/login')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
}

func TestVueShellUsesDefaultMenuExpansionAfterFreshLogin(t *testing.T) {
	src := readSupportTestFile(t, "frontend-vue-shell/src/App.vue")
	for _, want := range []string{
		"fresh_login",
		"defaultExpandedGroups(availableMenuGroups.value, currentKey.value)",
		"searchParams.delete('fresh_login')",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
}

func TestUserPermissionsDistinguishesSetAndResetPasswordLabels(t *testing.T) {
	src := readSupportTestFile(t, "frontend-vue-shell/src/views/UserPermissionsView.vue")
	for _, want := range []string{
		"passwordActionLabel(employee.id)",
		"has_password ? '重置密码' : '设置密码'",
		"passwordPlaceholder(employee.id)",
		"savePassword(employee.id)",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("UserPermissionsView.vue missing %q", want)
		}
	}
}

func readSupportTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
