package support

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	authzapp "orderapp/internal/application/authz"
	"strings"
	"testing"
)

type passwordSettingsStore map[string]string

func (s passwordSettingsStore) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := s[key]
	return v, ok, nil
}
func (s passwordSettingsStore) Set(_ context.Context, actor, key, value string) error {
	s[key] = value
	return nil
}
func TestMiniPasswordSettingAdminAndCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		admin  bool
		body   string
		status int
		want   string
	}{
		{"admin enable", true, `{"show_miniapp_password_login":true}`, 200, "true"},
		{"nonadmin forbidden", false, `{"show_miniapp_password_login":true}`, 403, "false"},
		{"legacy update preserves switch", false, `{"hide_customer_account_fulfillment":false}`, 200, "false"},
		{"reject simultaneous fields", true, `{"hide_customer_account_fulfillment":false,"show_miniapp_password_login":true}`, 400, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := passwordSettingsStore{"miniapp.login.show_password": "false"}
			actor := authzapp.Actor{Name: "test", Permissions: []string{"settings.write"}, BasicAuthAdmin: tc.admin}
			e := echo.New()
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(c echo.Context) error { c.Set("employee_id", int64(1)); return next(c) }
			})
			registerUISettingsAPI(e, s, &fakeAuthzService{actor: actor})
			req := httptest.NewRequest("PUT", "/api/ui-settings", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || s["miniapp.login.show_password"] != tc.want {
				t.Fatalf("status=%d setting=%q body=%s", rec.Code, s["miniapp.login.show_password"], rec.Body.String())
			}
		})
	}
}
