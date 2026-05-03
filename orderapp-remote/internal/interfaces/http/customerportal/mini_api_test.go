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
	login      customerportalapp.LoginResult
	me         customerportalapp.CurrentContext
	service    customerportalapp.ServicePage
	directShip customerportalapp.DirectShipBatch
	processing customerportalapp.ProcessingRequest
	err        error
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

func (s fakeService) GetServicePage(context.Context, string, string) (customerportalapp.ServicePage, error) {
	if s.err != nil {
		return customerportalapp.ServicePage{}, s.err
	}
	return s.service, nil
}

func (s fakeService) CreateDirectShipBatch(context.Context, string, customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error) {
	if s.err != nil {
		return customerportalapp.DirectShipBatch{}, s.err
	}
	return s.directShip, nil
}

func (s fakeService) CreateProcessingRequest(context.Context, string, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	if s.err != nil {
		return customerportalapp.ProcessingRequest{}, s.err
	}
	return s.processing, nil
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

func TestMiniServicePageAPIRequiresTokenAndReturnsScopedData(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key:                 customerportalapp.ServiceKeyShipping,
		Title:               "物流查询",
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		Orders:              []customerportalapp.CustomerOrderSummary{{OrderNo: "SO-1", ShipTrackingNo: "SF123"}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/shipping", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ship_tracking_no":"SF123"`) || !strings.Contains(rec.Body.String(), `"current_customer_id":8`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	noToken := httptest.NewRequest(http.MethodGet, "/api/mini/services/shipping", nil)
	noTokenRec := httptest.NewRecorder()
	e.ServeHTTP(noTokenRec, noToken)
	if noTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status=%d body=%s", noTokenRec.Code, noTokenRec.Body.String())
	}
}

func TestMiniServicePageCapabilityDeniedMapsToForbidden(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrCapabilityNotEnabled}})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/directShip", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"error":"capability not enabled"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniDirectShipAndProcessingSubmitAPIs(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		directShip: customerportalapp.DirectShipBatch{ID: 5, BatchNo: "DS-20260503-0005", Status: "submitted", TotalRows: 100},
		processing: customerportalapp.ProcessingRequest{ID: 7, RequestNo: "PJ-20260503-0007", Status: "submitted"},
	}})

	directReq := httptest.NewRequest(http.MethodPost, "/api/mini/direct-ship/batches", strings.NewReader(`{"source_name":"5月直播订单","total_rows":100,"note":"客户一次发来100单"}`))
	directReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	directReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	directRec := httptest.NewRecorder()
	e.ServeHTTP(directRec, directReq)
	if directRec.Code != http.StatusOK || !strings.Contains(directRec.Body.String(), `"batch_no":"DS-20260503-0005"`) {
		t.Fatalf("direct status=%d body=%s", directRec.Code, directRec.Body.String())
	}

	processingReq := httptest.NewRequest(http.MethodPost, "/api/mini/processing-requests", strings.NewReader(`{"input_material_id":4,"input_qty_g":30000,"target_product_id":5,"target_spec_g":454,"target_qty":50,"note":"代加工申请"}`))
	processingReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	processingReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	processingRec := httptest.NewRecorder()
	e.ServeHTTP(processingRec, processingReq)
	if processingRec.Code != http.StatusOK || !strings.Contains(processingRec.Body.String(), `"request_no":"PJ-20260503-0007"`) {
		t.Fatalf("processing status=%d body=%s", processingRec.Code, processingRec.Body.String())
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

func TestMiniLoginDisabledUserMapsToForbidden(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrMiniUserDisabled}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"error":"mini user disabled"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
