package customerportal

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	supporthttp "orderapp/internal/interfaces/http/support"
)

type loginOptionsStore struct {
	value  string
	exists bool
	fail   bool
}

func (s loginOptionsStore) Get(context.Context, string) (string, bool, error) {
	if s.fail {
		return "", false, errors.New("unavailable")
	}
	return s.value, s.exists, nil
}
func (s loginOptionsStore) Set(context.Context, string, string, string) error { return nil }
func TestMiniLoginOptionsVisibility(t *testing.T) {
	for _, tc := range []struct {
		name              string
		store             loginOptionsStore
		disabled, visible bool
	}{
		{name: "default hidden"}, {name: "enabled", store: loginOptionsStore{value: "true", exists: true}, visible: true},
		{name: "global kill switch", store: loginOptionsStore{value: "true", exists: true}, disabled: true},
		{name: "invalid", store: loginOptionsStore{value: "invalid", exists: true}}, {name: "read failure", store: loginOptionsStore{fail: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			e.Use(supporthttp.BasicAuth("order", "secret", "public", nil))
			RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{}, EmployeeShareSettings: tc.store, DisableMiniPasswordLogin: tc.disabled})
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest("GET", "/api/mini/login-options", nil))
			want := `"show_password_login":false`
			if tc.visible {
				want = `"show_password_login":true`
			}
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("status=%d body=%s want=%s", rec.Code, rec.Body.String(), want)
			}
		})
	}
}
