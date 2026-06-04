package support

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authzapp "orderapp/internal/application/authz"

	"github.com/labstack/echo/v4"
)

type fakeAuthzService struct {
	actor       authzapp.Actor
	roles       []authzapp.Role
	assignments map[int64][]string
	assigned    authzapp.AssignmentCommand
}

func (f *fakeAuthzService) ActorByEmployeeID(ctx context.Context, employeeID int64) (authzapp.Actor, error) {
	f.actor.EmployeeID = employeeID
	return f.actor, nil
}

func (f *fakeAuthzService) ListRoles(ctx context.Context) ([]authzapp.Role, error) {
	return f.roles, nil
}

func (f *fakeAuthzService) AssignEmployeeRoles(ctx context.Context, cmd authzapp.AssignmentCommand) error {
	f.assigned = cmd
	return nil
}

func (f *fakeAuthzService) ListEmployeeRoles(ctx context.Context) (map[int64][]string, error) {
	return f.assignments, nil
}

func TestAuthMeReturnsCurrentActorPermissionsAndViews(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{
		Name:            "销售",
		Permissions:     []string{"orders.read"},
		AllowedViewKeys: []string{"orders"},
	}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	registerAuthzAPI(e, authz)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "销售" {
		t.Fatalf("name=%v", body["name"])
	}
}

func TestAuthMeTreatsBasicAuthAsAdminFallback(t *testing.T) {
	e := echo.New()
	e.Use(BasicAuth("boss", "secret", "public", nil))
	registerAuthzAPI(e, &fakeAuthzService{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.SetBasicAuth("boss", "secret")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"basic_auth_admin":true`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestAuthorizationMiddlewareDeniesMissingPermission(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"orders.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(3))
			return next(c)
		}
	})
	e.Use(AuthorizationMiddleware(authz))
	e.GET("/api/settings/sender", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/sender", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rec.Code)
	}
}

func TestAuthorizationMiddlewareStopsDeniedRequestBeforeHandler(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"orders.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(3))
			return next(c)
		}
	})
	e.Use(AuthorizationMiddleware(authz))
	called := false
	e.GET("/api/settings/sender", func(c echo.Context) error {
		called = true
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/sender", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403, body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("authorization middleware called the protected handler after denying permission")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("denied response must be one valid JSON object, got %q: %v", rec.Body.String(), err)
	}
	if body["error"] != "permission denied" {
		t.Fatalf("denied response body = %#v", body)
	}
}

func TestAuthorizationMiddlewareAllowsFulfillmentOrderListForCustomerWorkbench(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"customer_processing.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	e.Use(AuthorizationMiddleware(authz))
	e.GET("/api/orders", func(c echo.Context) error {
		if !CustomerFulfillmentOrderScopeLimited(c) {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "scope marker missing"})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/orders?scope=fulfillment&customer_id=152", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizationMiddlewareAllowsFulfillmentOrderDetailForCustomerWorkbench(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"customer_processing.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	e.Use(AuthorizationMiddleware(authz))
	e.GET("/api/orders/:id/detail", func(c echo.Context) error {
		if !CustomerFulfillmentOrderScopeLimited(c) {
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "scope marker missing"})
		}
		return c.JSON(http.StatusOK, map[string]any{"ok": true})
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/orders/88/detail", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAuthorizationMiddlewareAllowsMatchingPermission(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"settings.write"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(3))
			return next(c)
		}
	})
	e.Use(AuthorizationMiddleware(authz))
	e.GET("/api/settings/sender", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/sender", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCustomerPortalAdminAPIRequiresCustomerPermissions(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/customer-portal/admin/customers", "customers.read"},
		{http.MethodGet, "/api/customer-portal/admin/customers/147", "customers.read"},
		{http.MethodPut, "/api/customer-portal/admin/customers/147/visibility", "customers.write"},
	}
	for _, tc := range cases {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.want {
			t.Fatalf("%s %s permission = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestMessageCenterAPIRequiresOrderReadPermission(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/message-center/notifications"); got != "orders.read" {
		t.Fatalf("message center GET permission = %q, want orders.read", got)
	}
	if got := requiredPermissionForRequest(http.MethodPost, "/api/message-center/notifications/11/read"); got != "orders.read" {
		t.Fatalf("message center read permission = %q, want orders.read", got)
	}
	if got := requiredPermissionForRequest(http.MethodGet, "/api/message-center/rules"); got != "settings.write" {
		t.Fatalf("message center rules permission = %q, want settings.write", got)
	}
}

func TestRequiredPermissionForInternalAccounts(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/auth/internal-accounts"); got != "" {
		t.Fatalf("GET /api/auth/internal-accounts middleware permission = %q, want empty", got)
	}
}

func TestBeanListPublicationPermissionsSeparatePublishAndDraft(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/api/costing/bean-list/publications", "auth.manage"},
		{http.MethodPost, "/api/costing/bean-list/publications/8/withdraw", "auth.manage"},
		{http.MethodPost, "/api/costing/bean-list/drafts", "costing.read"},
		{http.MethodGet, "/api/costing/bean-list/publications", "costing.read"},
	}
	for _, tc := range cases {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.want {
			t.Fatalf("%s %s permission = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/drip-price-templates"},
		{http.MethodPost, "/api/drip-price-templates"},
		{http.MethodPut, "/api/drip-price-templates/8"},
		{http.MethodPost, "/api/drip-price-templates/8/deactivate"},
	} {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != "" {
			t.Fatalf("%s %s permission = %q, want empty because route is removed", tc.method, tc.path, got)
		}
	}
}

func TestContractStampingAPIRequiresOrderPermissions(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodGet, "/api/contracts", "orders.read"},
		{http.MethodPost, "/api/contracts", "orders.write"},
		{http.MethodPost, "/api/contracts/7/stamped", "orders.write"},
		{http.MethodGet, "/api/settings/sales-order/seals", "orders.write"},
	}
	for _, tc := range cases {
		if got := requiredPermissionForRequest(tc.method, tc.path); got != tc.want {
			t.Fatalf("%s %s permission = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestMiniAPIHasNoEmployeePermissionBoundary(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodGet, "/api/mini/me"); got != "" {
		t.Fatalf("GET /api/mini/me permission = %q, want empty", got)
	}
	if got := requiredPermissionForRequest(http.MethodGet, "/api/customers"); got != "customers.read" {
		t.Fatalf("GET /api/customers permission = %q, want customers.read", got)
	}
}

func TestCompanyProfileAPIRequiresSettingsPermission(t *testing.T) {
	if got := requiredPermissionForRequest(http.MethodPost, "/api/company/profile"); got != "settings.write" {
		t.Fatalf("company profile permission = %q, want settings.write", got)
	}
}

func TestAssignEmployeeRolesAPIRequiresAuthManage(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Permissions: []string{"auth.manage"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			return next(c)
		}
	})
	registerAuthzAPI(e, authz)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/employee-roles", strings.NewReader(`{"employee_id":9,"role_codes":["sales","production"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if authz.assigned.EmployeeID != 9 || len(authz.assigned.RoleCodes) != 2 {
		t.Fatalf("assigned=%+v", authz.assigned)
	}
}

func TestListEmployeeRolesAPIRequiresAuthManage(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{
		actor:       authzapp.Actor{Permissions: []string{"auth.manage"}},
		assignments: map[int64][]string{9: {"sales"}},
	}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			return next(c)
		}
	})
	registerAuthzAPI(e, authz)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/employee-roles", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"9":["sales"]`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}
