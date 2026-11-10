package customerportal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	portal "orderapp/internal/application/customerportal"
)

type previewMiddlewareService struct{ allowed bool }

func (s *previewMiddlewareService) Me(_ context.Context, _ string) (portal.CurrentContext, error) {
	if !s.allowed {
		return portal.CurrentContext{}, portal.ErrAdminPreviewDenied
	}
	return portal.CurrentContext{AccountType: "preview", CurrentCustomerID: 74}, nil
}

type previewHTTPService struct {
	fakeService
	started   int64
	exited    bool
	loggedOut bool
}

func (s *previewHTTPService) Me(_ context.Context, token string) (portal.CurrentContext, error) {
	if token == "admin-preview.test" {
		return portal.CurrentContext{AccountType: "preview", CurrentCustomerID: 74}, nil
	}
	if token == "employee" {
		return portal.CurrentContext{AccountType: "employee", EmployeeID: 15, Roles: []string{"admin"}}, nil
	}
	return portal.CurrentContext{}, portal.ErrMiniSessionNotFound
}
func (s *previewHTTPService) StartAdminPreview(_ context.Context, token string, id int64) (portal.AdminPreviewSession, error) {
	if token != "employee" {
		return portal.AdminPreviewSession{}, portal.ErrAdminPreviewDenied
	}
	s.started = id
	return portal.AdminPreviewSession{Token: "admin-preview.test"}, nil
}
func (s *previewHTTPService) EndAdminPreview(_ context.Context, token string) error {
	if token != "admin-preview.test" {
		return portal.ErrAdminPreviewDenied
	}
	s.exited = true
	return nil
}
func (s *previewHTTPService) LogoutPreviewEmployee(_ context.Context, token string) error {
	if token != "employee" {
		return portal.ErrAdminPreviewDenied
	}
	s.loggedOut = true
	return nil
}

func TestPreviewHTTPStartExitAndGlobalGuard(t *testing.T) {
	e := echo.New()
	svc := &previewHTTPService{}
	registerAdminPreviewAPI(e, svc, nil)
	e.POST("/api/mini/product-orders", func(c echo.Context) error { return c.NoContent(201) })
	start := httptest.NewRequest("POST", "/api/mini/employee/customer-preview", strings.NewReader(`{"customer_id":74}`))
	start.Header.Set("Content-Type", "application/json")
	start.Header.Set("Authorization", "Bearer employee")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, start)
	if rec.Code != 200 || svc.started != 74 || !strings.Contains(rec.Body.String(), "admin-preview.test") {
		t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
	}
	write := httptest.NewRequest("POST", "/api/mini/product-orders", nil)
	write.Header.Set("Authorization", "Bearer admin-preview.test")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, write)
	if rec.Code != 403 {
		t.Fatalf("write not blocked: %d", rec.Code)
	}
	exit := httptest.NewRequest("POST", "/api/mini/customer-preview/exit", nil)
	exit.Header.Set("Authorization", "Bearer admin-preview.test")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, exit)
	if rec.Code != 204 || !svc.exited {
		t.Fatalf("exit: %d", rec.Code)
	}
	logout := httptest.NewRequest("POST", "/api/mini/employee/logout", nil)
	logout.Header.Set("Authorization", "Bearer employee")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, logout)
	if rec.Code != 204 || !svc.loggedOut {
		t.Fatalf("employee logout: %d", rec.Code)
	}
	logout = httptest.NewRequest("POST", "/api/mini/employee/logout", nil)
	logout.Header.Set("Authorization", "Bearer admin-preview.test")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, logout)
	if rec.Code != 403 {
		t.Fatalf("preview attempted employee logout: %d", rec.Code)
	}
}

func TestPreviewMiddlewareRejectsEveryUnlistedReadAndWrite(t *testing.T) {
	e := echo.New()
	svc := &previewMiddlewareService{allowed: true}
	e.Use(adminPreviewReadOnlyMiddleware(svc))
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/mini/me"}, {"GET", "/api/mini/services/:key"}, {"GET", "/api/mini/customer-issued-bills/:id"},
		{"POST", "/api/mini/product-orders"}, {"POST", "/api/mini/customer-preview/exit"}, {"GET", "/api/mini/employee/orders"}, {"GET", "/api/customers"},
	} {
		e.Add(route.method, route.path, func(c echo.Context) error { return c.NoContent(204) })
	}
	for _, tc := range []struct {
		method, path string
		expected     int
	}{
		{"GET", "/api/mini/me", 204}, {"GET", "/api/mini/services/orders", 204}, {"GET", "/api/mini/services/product_order", 403},
		{"GET", "/api/mini/customer-issued-bills/1.pdf", 204}, {"GET", "/api/mini/customer-issued-bills/anything", 403},
		{"POST", "/api/mini/product-orders", 403}, {"GET", "/api/mini/employee/orders", 403}, {"GET", "/api/customers", 403},
		{"POST", "/api/mini/customer-preview/exit", 204},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Authorization", "Bearer admin-preview.opaque")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.expected {
			t.Errorf("%s %s: %d want %d", tc.method, tc.path, rec.Code, tc.expected)
		}
	}
	svc.allowed = false
	req := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	req.Header.Set("Authorization", "Bearer admin-preview.opaque")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("revoked preview status %d", rec.Code)
	}
}
