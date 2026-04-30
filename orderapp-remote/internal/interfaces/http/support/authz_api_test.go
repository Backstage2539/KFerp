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
