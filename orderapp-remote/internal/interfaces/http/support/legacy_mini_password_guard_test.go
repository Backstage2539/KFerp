package support

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestLegacyMiniPasswordGuard(t *testing.T) {
	for _, tc := range []struct {
		name, path, auth, cookie string
		legacy, fail             bool
		status                   int
		checked                  bool
	}{
		{"mini old", "/api/mini/me", "Bearer old", "", true, false, 401, true},
		{"price bypass", "/api/pages/document", "Bearer old", "", true, false, 401, true},
		{"sample bypass", "/api/mini/sample-requests", "Bearer old", "", true, false, 401, true},
		{"recipient parse old mini token", "/api/customer-recipient/parse", "Bearer old", "", true, false, 401, true},
		{"recipient parse wechat token", "/api/customer-recipient/parse", "Bearer wechat", "", false, false, 200, true},
		{"new wechat", "/api/mini/me", "Bearer wechat", "", false, false, 200, true},
		{"erp bearer", "/api/orders", "Bearer erp", "", false, false, 200, false},
		{"cookie web", "/api/page-auth/me", "", "session=x", false, false, 200, false},
		{"reauthorize stale bearer", "/api/mini/login", "Bearer old", "", true, false, 401, true},
		{"registration old bearer", "/api/mini/registration", "Bearer old", "", true, false, 401, true},
		{"registration anonymous", "/api/mini/registration", "", "", false, false, 200, false},
		{"database failure", "/api/mini/me", "Bearer unknown", "", false, true, 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			check := func(ctx context.Context, token string) (bool, error) {
				called = true
				if tc.fail {
					return false, errors.New("db down")
				}
				return tc.legacy, nil
			}
			e := echo.New()
			e.Use(LegacyMiniPasswordGuard(check))
			e.Any("/*", func(c echo.Context) error { return c.NoContent(200) })
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", tc.auth)
			req.Header.Set("Cookie", tc.cookie)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || called != tc.checked {
				t.Fatalf("got status=%d checked=%v, want %d %v", rec.Code, called, tc.status, tc.checked)
			}
		})
	}
}

func TestLegacyMiniPasswordCheckerUsesActualSessionIdentity(t *testing.T) {
	ctx := context.Background()
	pool, schema := newERPLoginGateTestDB(t)
	for _, item := range []struct {
		token, openid string
		legacy        bool
	}{{"old-customer", "erp-employee:1", true}, {"old-staff", "erp-internal-employee:2", true}, {"new-staff", "wechat-internal-employee:2:hash", false}, {"real-wechat", "wx-openid", false}} {
		var id int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid) VALUES($1) RETURNING id`, schema), item.openid).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_sessions(token,mini_user_id,expire_at) VALUES($1,$2,'infinity')`, schema), item.token, id); err != nil {
			t.Fatal(err)
		}
		got, err := NewLegacyMiniPasswordChecker(pool, schema)(ctx, item.token)
		if err != nil || got != item.legacy {
			t.Fatalf("token classification=%v want=%v err=%v", got, item.legacy, err)
		}
	}
	got, err := NewLegacyMiniPasswordChecker(pool, schema)(ctx, "ordinary-erp-token")
	if err != nil || got {
		t.Fatalf("ERP token must not be classified as mini: %v %v", got, err)
	}
}

func TestLegacyMiniPasswordGuardAllowsNonMiniERPBearer(t *testing.T) {
	called := false
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
	})
	e.Use(LegacyMiniPasswordGuard(func(context.Context, string) (bool, error) {
		called = true
		return false, nil
	}))
	e.GET("/api/pages/:key", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/pages/test", nil)
	req.Header.Set("Authorization", "Bearer erp-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || !called {
		t.Fatalf("authenticated ERP request blocked: status=%d checker=%v", rec.Code, called)
	}
	e.POST("/api/customer-recipient/parse", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req = httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", nil)
	req.Header.Set("Authorization", "Bearer erp-token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 || !called {
		t.Fatalf("authenticated ERP recipient parse blocked: status=%d checker=%v", rec.Code, called)
	}
}

func TestLegacyGuardRegistrationAllMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			e := echo.New()
			e.Use(LegacyMiniPasswordGuard(func(context.Context, string) (bool, error) { return true, nil }))
			e.Any("/api/mini/registration", func(c echo.Context) error { return c.NoContent(200) })
			req := httptest.NewRequest(method, "/api/mini/registration", nil)
			req.Header.Set("Authorization", "Bearer old")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != 401 {
				t.Fatalf("legacy %s registration status=%d", method, rec.Code)
			}
		})
	}
}

func TestLegacyGuardRecipientParsePost(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
		want   int
	}{{"old password", true, 401}, {"wechat", false, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(LegacyMiniPasswordGuard(func(context.Context, string) (bool, error) { return tc.legacy, nil }))
			e.POST("/api/customer-recipient/parse", func(c echo.Context) error { return c.NoContent(200) })
			req := httptest.NewRequest(http.MethodPost, "/api/customer-recipient/parse", nil)
			req.Header.Set("Authorization", "Bearer example")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("recipient parse status=%d want=%d", rec.Code, tc.want)
			}
		})
	}
}

func TestLegacyGuardChecksBearerEvenWithEmployeeContext(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
		want   int
	}{{"old mini bearer", true, 401}, {"erp bearer", false, 200}} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(7)); return next(c) }
			})
			e.Use(LegacyMiniPasswordGuard(func(context.Context, string) (bool, error) { called = true; return tc.legacy, nil }))
			e.GET("/api/mini/me", func(c echo.Context) error { return c.NoContent(200) })
			req := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want || !called {
				t.Fatalf("got status=%d checked=%v", rec.Code, called)
			}
		})
	}
}
