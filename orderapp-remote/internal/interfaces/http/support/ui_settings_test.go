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

type fakeUISettingsStore struct {
	value    string
	hasValue bool
	setActor string
	setKey   string
	setValue string
}

func (f *fakeUISettingsStore) Get(ctx context.Context, key string) (string, bool, error) {
	return f.value, f.hasValue, nil
}

func (f *fakeUISettingsStore) Set(ctx context.Context, actor, key, value string) error {
	f.setActor = actor
	f.setKey = key
	f.setValue = value
	f.value = value
	f.hasValue = true
	return nil
}

func TestUISettingsAPIReturnsDefaultCustomerAccountFulfillmentHidden(t *testing.T) {
	e := echo.New()
	authz := &fakeAuthzService{actor: authzapp.Actor{Name: "客户账号", Permissions: []string{"customer_processing.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	registerUISettingsAPI(e, &fakeUISettingsStore{}, authz)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ui-settings", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Settings UISettings `json:"settings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Settings.HideCustomerAccountFulfillment {
		t.Fatalf("settings=%+v, want hide_customer_account_fulfillment default true", body.Settings)
	}
}

func TestUISettingsAPISavesCustomerAccountFulfillmentVisibility(t *testing.T) {
	e := echo.New()
	store := &fakeUISettingsStore{value: "true", hasValue: true}
	authz := &fakeAuthzService{actor: authzapp.Actor{Name: "管理员", Permissions: []string{"settings.write"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(9))
			return next(c)
		}
	})
	registerUISettingsAPI(e, store, authz)

	req := httptest.NewRequest(http.MethodPut, "/api/ui-settings", strings.NewReader(`{"hide_customer_account_fulfillment":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.setActor != "管理员" || store.setKey != keyHideCustomerAccountFulfillment || store.setValue != "false" {
		t.Fatalf("set actor/key/value = %q/%q/%q", store.setActor, store.setKey, store.setValue)
	}
	if !strings.Contains(rec.Body.String(), `"hide_customer_account_fulfillment":false`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestUISettingsAPIRejectsSaveWithoutSettingsPermission(t *testing.T) {
	e := echo.New()
	store := &fakeUISettingsStore{}
	authz := &fakeAuthzService{actor: authzapp.Actor{Name: "客户账号", Permissions: []string{"customer_processing.read"}}}
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(7))
			return next(c)
		}
	})
	registerUISettingsAPI(e, store, authz)

	req := httptest.NewRequest(http.MethodPut, "/api/ui-settings", strings.NewReader(`{"hide_customer_account_fulfillment":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if store.setKey != "" {
		t.Fatalf("unauthorized save reached store: %+v", store)
	}
}
