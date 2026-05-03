package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type fakeService struct {
	login customerportalapp.LoginResult
	me    customerportalapp.CurrentContext
	err   error
}

func (s fakeService) Login(context.Context, customerportalapp.LoginCommand) (customerportalapp.LoginResult, error) {
	if s.err != nil {
		return customerportalapp.LoginResult{}, s.err
	}
	return s.login, nil
}

func (s fakeService) Me(context.Context, string) (customerportalapp.CurrentContext, error) {
	if s.err != nil {
		return customerportalapp.CurrentContext{}, s.err
	}
	return s.me, nil
}

func (s fakeService) SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error) {
	if s.err != nil {
		return customerportalapp.CurrentContext{}, s.err
	}
	return s.me, nil
}

func TestMiniLoginAndMeAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login: customerportalapp.LoginResult{Token: "mini-token", MiniUserID: 3},
		me: customerportalapp.CurrentContext{
			MiniUserID: 3, CurrentCustomerID: 8, CurrentCustomerName: "客户A",
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		},
	}})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code","phone":"13800138000"}`))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK || !strings.Contains(loginRec.Body.String(), `"token":"mini-token"`) {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	meReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK || !strings.Contains(meRec.Body.String(), `"current_customer_name":"客户A"`) || !strings.Contains(meRec.Body.String(), customerportalapp.CapabilityDirectShip) {
		t.Fatalf("me status=%d body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestMiniMeRequiresToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{}})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/mini/me", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniCurrentCustomerPayload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: customerportalapp.CurrentContext{CurrentCustomerID: 9}}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/current-customer", strings.NewReader(`{"customer_id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got["current_customer_id"].(float64) != 9 {
		t.Fatalf("response=%v err=%v", got, err)
	}
}

func TestMiniAPINilServiceReturnsGenericInternalError(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code"}`)))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"error":"internal error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniMeInvalidTokenUsesGenericUnauthorizedError(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrMiniSessionNotFound}})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer bad-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"error":"invalid or expired mini token"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniUnexpectedServiceErrorDoesNotLeakRawMessage(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("database password leaked")}})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "database password leaked") || !strings.Contains(rec.Body.String(), `"error":"internal error"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniCurrentCustomerUnauthorizedBindingMapsToForbidden(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrCustomerBindingNotFound}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/current-customer", strings.NewReader(`{"customer_id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"error":"customer binding not found"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniCurrentCustomerInvalidPayloadMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/current-customer", strings.NewReader(`{"customer_id":0}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"customer required"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDisabledIdentityProviderReturnsLoginDisabled(t *testing.T) {
	_, err := (DisabledIdentityProvider{}).Resolve(context.Background(), "wx-code")
	if !errors.Is(err, customerportalapp.ErrMiniLoginDisabled) {
		t.Fatalf("Resolve() err=%v, want ErrMiniLoginDisabled", err)
	}
}

func TestMiniLoginDisabledMapsToUnavailable(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrMiniLoginDisabled}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"error":"mini login disabled"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
