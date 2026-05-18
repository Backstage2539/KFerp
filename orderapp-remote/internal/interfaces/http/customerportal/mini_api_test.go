package customerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type fakeService struct {
	login            customerportalapp.LoginResult
	loginCmd         *customerportalapp.LoginCommand
	passwordLoginCmd *customerportalapp.PasswordLoginCommand
	me               customerportalapp.CurrentContext
	service          customerportalapp.ServicePage
	filter           *customerportalapp.ServicePageFilter
	customers        []customerportalapp.PortalAdminCustomer
	detail           customerportalapp.PortalAdminDetail
	saveCmd          *customerportalapp.UpdatePortalVisibilityCommand
	templates        []customerportalapp.CapabilityTemplate
	templateSaveCmd  *customerportalapp.SaveCapabilityTemplateCommand
	templateCopyCmd  *customerportalapp.CopyCapabilityTemplateCommand
	templateCmd      *customerportalapp.ApplyCapabilityTemplateCommand
	erpBindingCmd    *customerportalapp.UpsertPortalERPBindingCommand
	mallRows         []customerportalapp.MallProduct
	mallOptions      []customerportalapp.MallProductOption
	mallSaveCmd      *customerportalapp.SaveMallProductCommand
	mallImageCmd     *customerportalapp.UpdateMallProductImageCommand
	mallImageErr     error
	mallPage         customerportalapp.MallPage
	mallOrder        customerportalapp.FulfillmentOrder
	mallOrderCmd     *customerportalapp.CreateMallOrderCommand
	directShip       customerportalapp.DirectShipBatch
	processing       customerportalapp.ProcessingRequest
	fulfillment      customerportalapp.FulfillmentOrder
	fulfillmentCmd   *customerportalapp.CreateFulfillmentOrderCommand
	beanList         customerportalapp.BeanListSummary
	orderAccessToken *string
	orderAccessID    *int64
	err              error
}

func (s fakeService) Login(_ context.Context, cmd customerportalapp.LoginCommand) (customerportalapp.LoginResult, error) {
	if s.err != nil {
		return customerportalapp.LoginResult{}, s.err
	}
	if s.loginCmd != nil {
		*s.loginCmd = cmd
	}
	return s.login, nil
}

func (s fakeService) LoginWithPassword(_ context.Context, cmd customerportalapp.PasswordLoginCommand) (customerportalapp.LoginResult, error) {
	if s.err != nil {
		return customerportalapp.LoginResult{}, s.err
	}
	if s.passwordLoginCmd != nil {
		*s.passwordLoginCmd = cmd
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

func (s fakeService) GetServicePage(ctx context.Context, token, key string, filter customerportalapp.ServicePageFilter) (customerportalapp.ServicePage, error) {
	if s.err != nil {
		return customerportalapp.ServicePage{}, s.err
	}
	if s.filter != nil {
		*s.filter = filter
	}
	return s.service, nil
}

func (s fakeService) GetBeanListPublication(context.Context, string, int64) (customerportalapp.BeanListSummary, error) {
	if s.err != nil {
		return customerportalapp.BeanListSummary{}, s.err
	}
	return s.beanList, nil
}

func (s fakeService) AcknowledgeBeanListPublication(context.Context, string, int64) error {
	return s.err
}

func (s fakeService) ListPortalAdminCustomers(context.Context, customerportalapp.PortalAdminCustomerQuery) ([]customerportalapp.PortalAdminCustomer, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.customers, nil
}

func (s fakeService) PortalAdminDetail(context.Context, int64) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	return s.detail, nil
}

func (s fakeService) UpdatePortalVisibility(_ context.Context, cmd customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	if s.saveCmd != nil {
		*s.saveCmd = cmd
	}
	return s.detail, nil
}

func (s fakeService) ListCapabilityTemplates(context.Context) ([]customerportalapp.CapabilityTemplate, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.templates != nil {
		return s.templates, nil
	}
	return customerportalapp.DefaultCapabilityTemplates(), nil
}

func (s fakeService) SaveCapabilityTemplate(_ context.Context, cmd customerportalapp.SaveCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error) {
	if s.err != nil {
		return customerportalapp.CapabilityTemplate{}, s.err
	}
	if s.templateSaveCmd != nil {
		*s.templateSaveCmd = cmd
	}
	return cmd.Template, nil
}

func (s fakeService) CopyCapabilityTemplate(_ context.Context, cmd customerportalapp.CopyCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error) {
	if s.err != nil {
		return customerportalapp.CapabilityTemplate{}, s.err
	}
	if s.templateCopyCmd != nil {
		*s.templateCopyCmd = cmd
	}
	source, ok := customerportalapp.CustomerCapabilityTemplateByKey(cmd.SourceKey)
	if !ok {
		source, _ = customerportalapp.CustomerCapabilityTemplateByKey(customerportalapp.CapabilityTemplatePublicSKUDirectShip)
	}
	source.Key = strings.TrimSpace(cmd.NewKey)
	source.ParentTemplateKey = strings.TrimSpace(cmd.SourceKey)
	source.Label = strings.TrimSpace(cmd.Label)
	source.Active = true
	return source, nil
}

func (s fakeService) ApplyCapabilityTemplate(_ context.Context, cmd customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	if s.templateCmd != nil {
		*s.templateCmd = cmd
	}
	return s.detail, nil
}

func (s fakeService) UpsertPortalERPBinding(_ context.Context, cmd customerportalapp.UpsertPortalERPBindingCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	if s.erpBindingCmd != nil {
		*s.erpBindingCmd = cmd
	}
	return s.detail, nil
}

func (s fakeService) ListMallProducts(context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.mallRows, s.mallOptions, nil
}

func (s fakeService) SaveMallProduct(_ context.Context, cmd customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error) {
	if s.err != nil {
		return customerportalapp.MallProduct{}, s.err
	}
	if s.mallSaveCmd != nil {
		*s.mallSaveCmd = cmd
	}
	return customerportalapp.MallProduct{ID: 12, ProductID: cmd.ProductID, Title: cmd.Title, SpecG: cmd.SpecG, UnitPrice: cmd.UnitPrice, TemplateKey: cmd.TemplateKey, Status: cmd.Status}, nil
}

func (s fakeService) UpdateMallProductImage(_ context.Context, cmd customerportalapp.UpdateMallProductImageCommand) (customerportalapp.MallProduct, error) {
	if s.err != nil {
		return customerportalapp.MallProduct{}, s.err
	}
	if s.mallImageErr != nil {
		return customerportalapp.MallProduct{}, s.mallImageErr
	}
	if s.mallImageCmd != nil {
		*s.mallImageCmd = cmd
	}
	return customerportalapp.MallProduct{ID: cmd.ID, ImageURL: cmd.ImageURL}, nil
}

func (s fakeService) GetMallPage(context.Context, string) (customerportalapp.MallPage, error) {
	if s.err != nil {
		return customerportalapp.MallPage{}, s.err
	}
	return s.mallPage, nil
}

func (s fakeService) CreateMallOrder(_ context.Context, token string, cmd customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	if s.err != nil {
		return customerportalapp.FulfillmentOrder{}, s.err
	}
	if s.mallOrderCmd != nil {
		*s.mallOrderCmd = cmd
	}
	return s.mallOrder, nil
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

func (s fakeService) CreateFulfillmentOrder(_ context.Context, _ string, cmd customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	if s.err != nil {
		return customerportalapp.FulfillmentOrder{}, s.err
	}
	if s.fulfillmentCmd != nil {
		*s.fulfillmentCmd = cmd
	}
	return s.fulfillment, nil
}

func (s fakeService) EnsureOrderAccess(_ context.Context, token string, orderID int64) error {
	if s.err != nil {
		return s.err
	}
	if s.orderAccessToken != nil {
		*s.orderAccessToken = token
	}
	if s.orderAccessID != nil {
		*s.orderAccessID = orderID
	}
	return nil
}

type fakeMiniSalesDocuments struct {
	salesPath       string
	deliveryPath    string
	salesOrderID    int64
	deliveryOrderID int64
	salesLatest     bool
	deliveryLatest  bool
}

func (f *fakeMiniSalesDocuments) LoadSalesOrderDocumentFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.SalesOrderDocumentFile, error) {
	f.salesOrderID = orderID
	f.salesLatest = latest
	return salesapp.SalesOrderDocumentFile{
		Document: salesapp.SalesOrderDocument{ID: documentID, OrderID: orderID, OrderNo: "SO-MINI", VersionNo: 1},
		Path:     f.salesPath,
		Filename: "SO-MINI-sales-order.pdf",
	}, nil
}

func (f *fakeMiniSalesDocuments) LoadDeliveryNoteDocumentFile(_ context.Context, orderID, documentID int64, latest bool) (salesapp.DeliveryNoteDocumentFile, error) {
	f.deliveryOrderID = orderID
	f.deliveryLatest = latest
	return salesapp.DeliveryNoteDocumentFile{
		Document: salesapp.DeliveryNoteDocument{ID: documentID, OrderID: orderID, OrderNo: "SO-MINI", VersionNo: 1},
		Path:     f.deliveryPath,
		Filename: "SO-MINI-delivery-note.pdf",
	}, nil
}

func TestMiniLoginAndMeAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login: customerportalapp.LoginResult{
			Token:             "mini-token",
			MiniUserID:        3,
			ThemeKey:          customerportalapp.PortalThemePremiumPartner,
			MiniappEntryMode:  customerportalapp.MiniappEntryModeMall,
			CurrentCustomerID: 8,
		},
		me: customerportalapp.CurrentContext{
			MiniUserID: 3, CurrentCustomerID: 8, CurrentCustomerName: "客户A",
			ThemeKey:         customerportalapp.PortalThemePremiumPartner,
			MiniappEntryMode: customerportalapp.MiniappEntryModeMall,
			Capabilities:     []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		},
	}})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code","phone":"13800138000"}`))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK ||
		!strings.Contains(loginRec.Body.String(), `"token":"mini-token"`) ||
		!strings.Contains(loginRec.Body.String(), `"theme_key":"premium_partner"`) ||
		!strings.Contains(loginRec.Body.String(), `"miniapp_entry_mode":"mall"`) {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	meReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK ||
		!strings.Contains(meRec.Body.String(), `"current_customer_name":"客户A"`) ||
		!strings.Contains(meRec.Body.String(), `"theme_key":"premium_partner"`) ||
		!strings.Contains(meRec.Body.String(), `"miniapp_entry_mode":"mall"`) ||
		!strings.Contains(meRec.Body.String(), customerportalapp.CapabilityDirectShip) {
		t.Fatalf("me status=%d body=%s", meRec.Code, meRec.Body.String())
	}
}

func TestMiniLoginAcceptsPhoneVerifyPayload(t *testing.T) {
	e := echo.New()
	var loginCmd customerportalapp.LoginCommand
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login:    customerportalapp.LoginResult{Token: "mini-token"},
		loginCmd: &loginCmd,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"mode":"phone_verify","code":"wx-code","phone_code":"phone-code","nickname":"客户A"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if loginCmd.Mode != "phone_verify" || loginCmd.Code != "wx-code" || loginCmd.PhoneCode != "phone-code" || loginCmd.Nickname != "客户A" {
		t.Fatalf("login cmd=%+v", loginCmd)
	}
}

func TestMiniPasswordLoginAPI(t *testing.T) {
	var cmd customerportalapp.PasswordLoginCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login: customerportalapp.LoginResult{
			Token:             "mini-token",
			MiniUserID:        3,
			ThemeKey:          customerportalapp.PortalThemePremiumPartner,
			MiniappEntryMode:  customerportalapp.MiniappEntryModeMall,
			CurrentCustomerID: 8,
		},
		passwordLoginCmd: &cmd,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/login/password", strings.NewReader(`{"login":"13800138075","password":"secret"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"token":"mini-token"`) ||
		!strings.Contains(rec.Body.String(), `"theme_key":"premium_partner"`) ||
		!strings.Contains(rec.Body.String(), `"miniapp_entry_mode":"mall"`) {
		t.Fatalf("password login status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cmd.Login != "13800138075" || cmd.Password != "secret" {
		t.Fatalf("LoginWithPassword cmd=%+v", cmd)
	}
}

func TestMiniPasswordLoginRejectsMissingCredentials(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/login/password", strings.NewReader(`{"login":"13800138075"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing password status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniPasswordLoginErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
		body string
	}{
		{name: "invalid login", err: customerportalapp.ErrMiniInvalidLogin, want: http.StatusUnauthorized, body: "invalid login"},
		{name: "disabled", err: customerportalapp.ErrMiniAccountLoginDisabled, want: http.StatusForbidden, body: "login disabled"},
		{name: "binding", err: customerportalapp.ErrCustomerBindingNotFound, want: http.StatusForbidden, body: "customer binding not found"},
		{name: "invalid template", err: customerportalapp.ErrCapabilityTemplateInvalid, want: http.StatusConflict, body: "客户配置已更新，请联系管理员处理"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: tc.err}})
			req := httptest.NewRequest(http.MethodPost, "/api/mini/login/password", strings.NewReader(`{"login":"13800138075","password":"secret"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("%s status=%d body=%s, want %d containing %q", tc.name, rec.Code, rec.Body.String(), tc.want, tc.body)
			}
		})
	}
}

func TestMiniMeCapabilityTemplateInvalidShowsCustomerConfigMessage(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrCapabilityTemplateInvalid}})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "客户配置已更新，请联系管理员处理") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
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

func TestMiniCurrentCustomerInactiveBindingMapsToForbidden(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrCustomerBindingNotFound}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/current-customer", strings.NewReader(`{"customer_id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `"error":"customer binding not found"`) {
		t.Fatalf("inactive binding switch status=%d body=%s", rec.Code, rec.Body.String())
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
		ThemeKey:            customerportalapp.PortalThemeCleanOps,
		MiniappEntryMode:    customerportalapp.MiniappEntryModeMall,
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		Orders: []customerportalapp.CustomerOrderSummary{{
			OrderNo: "SO-1", ShipTrackingNo: "SF123", GrandTotal: "137.00",
			Items: []customerportalapp.CustomerOrderItemSummary{{ItemName: "乌拉嘎", Spec: "454g", Qty: "2", UnitPrice: "68.50", LineTotal: "137.00"}},
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/shipping", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"ship_tracking_no":"SF123"`) ||
		!strings.Contains(rec.Body.String(), `"grand_total":"137.00"`) ||
		!strings.Contains(rec.Body.String(), `"items":[`) ||
		!strings.Contains(rec.Body.String(), `"theme_key":"clean_ops"`) ||
		!strings.Contains(rec.Body.String(), `"miniapp_entry_mode":"mall"`) ||
		!strings.Contains(rec.Body.String(), `"current_customer_id":8`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	noToken := httptest.NewRequest(http.MethodGet, "/api/mini/services/shipping", nil)
	noTokenRec := httptest.NewRecorder()
	e.ServeHTTP(noTokenRec, noToken)
	if noTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status=%d body=%s", noTokenRec.Code, noTokenRec.Body.String())
	}
}

func TestMiniOrdersServicePageAPIReturnsDirectOrderPayload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key:                 customerportalapp.ServiceKeyOrders,
		Title:               "我的订单",
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		Orders: []customerportalapp.CustomerOrderSummary{{
			OrderNo: "SO-DIRECT", ProcessStatus: "生产中", PayStatus: "已收款", ShipStatus: "待发货", GrandTotal: "258.00",
			Items: []customerportalapp.CustomerOrderItemSummary{{ItemName: "乌拉嘎", Spec: "454g", Qty: "1", UnitPrice: "258.00", LineTotal: "258.00"}},
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"key":"orders"`) ||
		!strings.Contains(rec.Body.String(), `"title":"我的订单"`) ||
		!strings.Contains(rec.Body.String(), `"order_no":"SO-DIRECT"`) ||
		!strings.Contains(rec.Body.String(), `"grand_total":"258.00"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniServicePageAPIReturnsDripProductPayload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key: customerportalapp.ServiceKeyProductOrder,
		Products: []customerportalapp.ProductSummary{{
			ID:                 88,
			Name:               "誉观山挂耳",
			ProductKind:        "drip_bag",
			SalesUnits:         []string{"bag", "box"},
			DripBagGrams:       10,
			DripBoxBagCount:    10,
			DripPriceGradients: []customerportalapp.UnitPriceGradient{{SalesUnit: "bag", MinQty: 1, UnitPrice: 6.5, UnitBagCount: 1, PriceSource: "published_unit_price"}},
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/productOrder", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"product_kind":"drip_bag"`,
		`"sales_units":["bag","box"]`,
		`"drip_bag_grams":10`,
		`"drip_box_bag_count":10`,
		`"drip_price_gradients":[`,
		`"sales_unit":"bag"`,
		`"price_source":"published_unit_price"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("drip service payload missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestMiniOrdersServicePageAPIReturnsDocumentURLs(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key:                 customerportalapp.ServiceKeyOrders,
		Title:               "我的订单",
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		Orders: []customerportalapp.CustomerOrderSummary{{
			ID: 88, OrderNo: "SO-DOC", SalesOrderURL: "/api/mini/orders/88/sales-order-latest.pdf", DeliveryNoteURL: "/api/mini/orders/88/delivery-note-latest.pdf",
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"sales_order_url":"/api/mini/orders/88/sales-order-latest.pdf"`) ||
		!strings.Contains(rec.Body.String(), `"delivery_note_url":"/api/mini/orders/88/delivery-note-latest.pdf"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniOrderDocumentsRequireTokenAndCheckOrderAccess(t *testing.T) {
	tmp := t.TempDir()
	salesPath := tmp + "/sales.pdf"
	deliveryPath := tmp + "/delivery.pdf"
	if err := os.WriteFile(salesPath, []byte("%PDF-sales"), 0o600); err != nil {
		t.Fatalf("write sales pdf: %v", err)
	}
	if err := os.WriteFile(deliveryPath, []byte("%PDF-delivery"), 0o600); err != nil {
		t.Fatalf("write delivery pdf: %v", err)
	}
	var accessToken string
	var accessOrderID int64
	docs := &fakeMiniSalesDocuments{salesPath: salesPath, deliveryPath: deliveryPath}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{orderAccessToken: &accessToken, orderAccessID: &accessOrderID},
		SalesDocuments: docs,
	})

	noToken := httptest.NewRequest(http.MethodGet, "/api/mini/orders/88/sales-order-latest.pdf", nil)
	noTokenRec := httptest.NewRecorder()
	e.ServeHTTP(noTokenRec, noToken)
	if noTokenRec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status=%d body=%s", noTokenRec.Code, noTokenRec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/mini/orders/88/sales-order-latest.pdf", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		rec.Header().Get(echo.HeaderContentType) != "application/pdf" ||
		!strings.Contains(rec.Body.String(), "%PDF-sales") {
		t.Fatalf("sales status=%d content-type=%s body=%s", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.String())
	}
	if accessToken != "mini-token" || accessOrderID != 88 || docs.salesOrderID != 88 || !docs.salesLatest {
		t.Fatalf("sales access token=%q accessOrderID=%d docs=%+v", accessToken, accessOrderID, docs)
	}

	deliveryReq := httptest.NewRequest(http.MethodGet, "/api/mini/orders/88/delivery-note-latest.pdf", nil)
	deliveryReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	deliveryRec := httptest.NewRecorder()
	e.ServeHTTP(deliveryRec, deliveryReq)
	if deliveryRec.Code != http.StatusOK ||
		deliveryRec.Header().Get(echo.HeaderContentType) != "application/pdf" ||
		!strings.Contains(deliveryRec.Body.String(), "%PDF-delivery") {
		t.Fatalf("delivery status=%d content-type=%s body=%s", deliveryRec.Code, deliveryRec.Header().Get(echo.HeaderContentType), deliveryRec.Body.String())
	}
	if docs.deliveryOrderID != 88 || !docs.deliveryLatest {
		t.Fatalf("delivery docs=%+v", docs)
	}
}

func TestMiniSettlementServicePageAPIReturnsFinancePayload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key:                 customerportalapp.ServiceKeySettlement,
		Title:               "结算中心",
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		FeeItems: []customerportalapp.FeeItem{{
			ID: 1, SourceType: "manual_adjustment", SourceID: 101, FeeType: "shipping",
			Amount: "12.34", Currency: "CNY", OccurredAt: "2026-05-13 09:00",
			Status: "unsettled", Note: "客户A费用",
		}},
		Orders: []customerportalapp.CustomerOrderSummary{{
			ID: 88, OrderNo: "SO-YAN-BILL", OrderDate: "2026-05-17", PayStatus: "未付款", PaymentMethod: "", GrandTotal: "4559.00",
		}},
		SettlementBatches: []customerportalapp.SettlementBatch{{
			ID: 2, SettlementNo: "客户A结算单", PeriodFrom: "2026-05-01", PeriodTo: "2026-05-31",
			Status: "confirmed", TotalAmount: "12.34", ConfirmedAt: "2026-05-31 10:00",
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/settlement", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"fee_items":[`) ||
		!strings.Contains(rec.Body.String(), `"orders":[`) ||
		!strings.Contains(rec.Body.String(), `"settlement_batches":[`) ||
		!strings.Contains(rec.Body.String(), `"order_no":"SO-YAN-BILL"`) ||
		!strings.Contains(rec.Body.String(), `"payment_method":""`) ||
		!strings.Contains(rec.Body.String(), `"note":"客户A费用"`) ||
		!strings.Contains(rec.Body.String(), `"settlement_no":"客户A结算单"`) ||
		strings.Contains(rec.Body.String(), `"item_name":`) ||
		strings.Contains(rec.Body.String(), "客户B不应泄露") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniSettlementServicePageAPIParsesBillingFilters(t *testing.T) {
	var gotFilter customerportalapp.ServicePageFilter
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		filter: &gotFilter,
		service: customerportalapp.ServicePage{
			Key:                 customerportalapp.ServiceKeySettlement,
			Title:               "结算中心",
			CurrentCustomerID:   8,
			CurrentCustomerName: "客户A",
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/settlement?q=SO-YAN&date_from=2026-05-01&date_to=2026-05-31&pay_status=%E6%9C%AA%E4%BB%98%E6%AC%BE", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotFilter.Query != "SO-YAN" || gotFilter.DateFrom != "2026-05-01" || gotFilter.DateTo != "2026-05-31" || gotFilter.PayStatus != "未付款" {
		t.Fatalf("billing filter=%+v", gotFilter)
	}
}

func TestMiniOrdersServicePageAPIParsesKeywordAndDateFilters(t *testing.T) {
	var gotFilter customerportalapp.ServicePageFilter
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		filter: &gotFilter,
		service: customerportalapp.ServicePage{
			Key:                 customerportalapp.ServiceKeyOrders,
			Title:               "我的订单",
			CurrentCustomerID:   8,
			CurrentCustomerName: "客户A",
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/orders?q=%E4%B9%8C%E6%8B%89%E5%98%8E&date_from=2026-05-01&date_to=2026-05-03&process_status=%E7%94%9F%E4%BA%A7%E4%B8%AD&pay_status=%E5%B7%B2%E6%94%B6%E6%AC%BE&ship_status=%E5%BE%85%E5%8F%91%E8%B4%A7", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotFilter.Query != "乌拉嘎" ||
		gotFilter.DateFrom != "2026-05-01" ||
		gotFilter.DateTo != "2026-05-03" ||
		gotFilter.ProcessStatus != "生产中" ||
		gotFilter.PayStatus != "已收款" ||
		gotFilter.ShipStatus != "待发货" {
		t.Fatalf("filter=%+v", gotFilter)
	}
}

func TestMiniBeanListServicePageAPIReturnsDisplayContent(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{service: customerportalapp.ServicePage{
		Key:                 customerportalapp.ServiceKeyBeanList,
		Title:               "我的豆单",
		CurrentCustomerID:   8,
		CurrentCustomerName: "客户A",
		BeanLists: []customerportalapp.BeanListSummary{{
			ID: 1, ListType: "commercial", VersionNo: "V3.0.5", Status: "published",
			CacheKey:            "bean-list:1:V3.0.5",
			Title:               "棵凡咖啡批发豆单",
			Subtitle:            "报价不含税、不含运",
			ListTypeLabel:       "商用",
			BrandName:           "棵凡咖啡",
			BrandIntro:          "源头工厂现烘现发",
			LayoutStyle:         "card",
			CardsPerRow:         2,
			ShowVersion:         true,
			ShowChangelog:       true,
			ShowCategoryNumbers: true,
			BackgroundColor:     "#f8f1e5",
			FontColor:           "#171717",
			Groups: []customerportalapp.BeanListGroupSummary{{
				Category:     "原产地精选豆",
				ShowCategory: true,
				Items: []customerportalapp.BeanListProductSummary{{
					Code: "5.2", Name: "乌拉嘎", Badge: "new", BadgeLabel: "NEW", Flavor: "柑橘/莓果",
					BeanListQuality: customerportalapp.BeanListQualitySummary{
						FactoryFlavorDescription: "茉莉花、柑橘",
						Moisture:                 "10.8%",
						Density:                  "780g/L",
						InspectionCreatedAt:      "2026-05-18 09:30",
					},
					HighlightTerms: []string{"乌拉嘎"},
					Prices:         []customerportalapp.BeanListPriceSummary{{Label: "454g", Value: "118/包", Red: true}},
				}},
			}},
		}},
	}}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/beanList", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"groups":[`) ||
		!strings.Contains(rec.Body.String(), `"cache_key":"bean-list:1:V3.0.5"`) ||
		!strings.Contains(rec.Body.String(), `"title":"棵凡咖啡批发豆单"`) ||
		!strings.Contains(rec.Body.String(), `"layout_style":"card"`) ||
		!strings.Contains(rec.Body.String(), `"cards_per_row":2`) ||
		!strings.Contains(rec.Body.String(), `"background_color":"#f8f1e5"`) ||
		!strings.Contains(rec.Body.String(), `"show_category":true`) ||
		!strings.Contains(rec.Body.String(), `"bean_list_quality":{"factory_flavor_description":"茉莉花、柑橘","moisture":"10.8%","density":"780g/L","inspection_created_at":"2026-05-18 09:30"}`) ||
		!strings.Contains(rec.Body.String(), `"highlight_terms":["乌拉嘎"]`) ||
		!strings.Contains(rec.Body.String(), `"name":"乌拉嘎"`) ||
		!strings.Contains(rec.Body.String(), `"prices":[`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniBeanListPDFAPIReturnsPDFDownload(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		beanList: customerportalapp.BeanListSummary{
			ID: 1, ListType: "commercial", VersionNo: "V3.0.5", Status: "published", PublishedAt: "2026-05-03 12:00",
			Groups: []customerportalapp.BeanListGroupSummary{{
				Category: "原产地精选豆",
				Items: []customerportalapp.BeanListProductSummary{{
					Code: "5.2", Name: "乌拉嘎", Flavor: "柑橘/莓果",
					Prices: []customerportalapp.BeanListPriceSummary{{Label: "454g", Value: "¥118/包"}},
				}},
			}},
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/bean-lists/1.pdf", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get(echo.HeaderContentType) != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF") {
		t.Fatalf("status=%d content-type=%q body=%q", rec.Code, rec.Header().Get(echo.HeaderContentType), rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get(echo.HeaderContentDisposition), "bean-list-commercial-V3.0.5.pdf") {
		t.Fatalf("content-disposition=%q", rec.Header().Get(echo.HeaderContentDisposition))
	}
}

func TestMiniBeanListPDFPublicationNotFoundMapsToNotFound(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrBeanListPublicationNotFound}})

	req := httptest.NewRequest(http.MethodGet, "/api/mini/bean-lists/404.pdf", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"error":"bean list publication not found"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPortalAdminVisibilityAPIsExposeAndSaveCustomerCapabilities(t *testing.T) {
	var saveCmd customerportalapp.UpdatePortalVisibilityCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		saveCmd:   &saveCmd,
		customers: []customerportalapp.PortalAdminCustomer{{ID: 147, Name: "13800138075", PortalEnabled: true, BindingCount: 1, ThemeKey: customerportalapp.PortalThemeCoffeeFactory, MiniappEntryMode: customerportalapp.MiniappEntryModeServices}},
		detail: customerportalapp.PortalAdminDetail{
			Customer: customerportalapp.PortalAdminCustomer{ID: 147, Name: "13800138075", DisplayName: "测试客户", PortalEnabled: true, ThemeKey: customerportalapp.PortalThemePremiumPartner, MiniappEntryMode: customerportalapp.MiniappEntryModeMall},
			Bindings: []customerportalapp.PortalUserBinding{{MiniUserID: 1, Phone: "13800138075", Role: "owner", Status: "approved"}},
			Capabilities: []customerportalapp.CapabilityOption{
				{Code: customerportalapp.CapabilityBeanList, Label: "我的豆单", Enabled: true},
				{Code: customerportalapp.CapabilityMall, Label: "商城下单", Enabled: true},
				{Code: customerportalapp.CapabilityDirectShip, Label: "一件代发", Enabled: true},
			},
		},
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers?q=13800138075", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK ||
		!strings.Contains(listRec.Body.String(), `"name":"13800138075"`) ||
		!strings.Contains(listRec.Body.String(), `"theme_key":"coffee_factory"`) ||
		!strings.Contains(listRec.Body.String(), `"miniapp_entry_mode":"services"`) ||
		!strings.Contains(listRec.Body.String(), `"binding_count":1`) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers/147", nil)
	detailRec := httptest.NewRecorder()
	e.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK ||
		!strings.Contains(detailRec.Body.String(), `"theme_key":"premium_partner"`) ||
		!strings.Contains(detailRec.Body.String(), `"miniapp_entry_mode":"mall"`) ||
		!strings.Contains(detailRec.Body.String(), `"bindings":[`) ||
		!strings.Contains(detailRec.Body.String(), `"capabilities":[`) {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}

	body := strings.NewReader(`{"display_name":"测试客户","enabled":true,"capability_template_key":"public_sku_direct_ship"}`)
	saveReq := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/customers/147/visibility", body)
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK || !strings.Contains(saveRec.Body.String(), `"display_name":"测试客户"`) {
		t.Fatalf("save status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}
	if saveCmd.CapabilityTemplateKey != customerportalapp.CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("save capability_template_key=%q, want public_sku_direct_ship", saveCmd.CapabilityTemplateKey)
	}
	if len(saveCmd.Capabilities) != 0 || saveCmd.ThemeKey != "" || saveCmd.MiniappEntryMode != "" {
		t.Fatalf("customer portal config should reference templates without inline capability/theme payload: %+v", saveCmd)
	}
}

func TestPortalAdminVisibilityTemplateInvalidMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("capability template invalid")}})

	body := strings.NewReader(`{"display_name":"测试客户","enabled":true,"capability_template_key":"legacy_unknown_template"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/customers/147/visibility", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"capability template invalid"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPortalAdminAPIPreservesUnknownTemplateKeyForCorrection(t *testing.T) {
	repo := &templateContractRepository{
		adminCustomers: []customerportalapp.PortalAdminCustomer{
			{ID: 147, Name: "未知模板客户", CapabilityTemplateKey: " legacy_unknown_template "},
		},
		adminDetail: customerportalapp.PortalAdminDetail{
			Customer: customerportalapp.PortalAdminCustomer{ID: 147, Name: "未知模板客户", CapabilityTemplateKey: " legacy_unknown_template "},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: customerportalapp.NewService(repo, staticMiniIdentityProvider{})})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"capability_template_key":"legacy_unknown_template"`) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers/147", nil)
	detailRec := httptest.NewRecorder()
	e.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), `"capability_template_key":"legacy_unknown_template"`) {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}
}

func TestPortalAdminCapabilityTemplateAPIsExposeAndApply(t *testing.T) {
	var templateCmd customerportalapp.ApplyCapabilityTemplateCommand
	var templateSaveCmd customerportalapp.SaveCapabilityTemplateCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		templateCmd:     &templateCmd,
		templateSaveCmd: &templateSaveCmd,
		detail: customerportalapp.PortalAdminDetail{
			Customer: customerportalapp.PortalAdminCustomer{ID: 147, Name: "客户A", CapabilityTemplateKey: customerportalapp.CapabilityTemplatePublicSKUDirectShip},
			Capabilities: []customerportalapp.CapabilityOption{
				{Code: customerportalapp.CapabilityDirectShip, Enabled: true},
			},
		},
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/capability-templates", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK ||
		!strings.Contains(listRec.Body.String(), `"key":"public_sku_direct_ship"`) ||
		!strings.Contains(listRec.Body.String(), `"key":"retail_mall"`) ||
		!strings.Contains(listRec.Body.String(), `"threshold_lb":14`) ||
		!strings.Contains(listRec.Body.String(), `"miniapp_entry_mode":"mall"`) ||
		!strings.Contains(listRec.Body.String(), `"erp_role_codes":[]`) {
		t.Fatalf("template list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	saveBody := strings.NewReader(`{"label":"公共 SKU 小批量代发","theme_key":"clean_ops","miniapp_entry_mode":"services","erp_role_codes":[],"erp_permissions":["customer_processing.read"],"erp_view_keys":["customerProcessingPortal"],"capabilities":[{"code":"direct_ship","enabled":true,"config":{"public_sku_aliases":true,"small_batch_price_rule":{"enabled":true,"threshold_lb":14,"tier_min_lb":15,"tier_max_lb":28}}}]}`)
	saveReq := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/capability-templates/public_sku_direct_ship", saveBody)
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK || !strings.Contains(saveRec.Body.String(), `"key":"public_sku_direct_ship"`) {
		t.Fatalf("save template status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}
	if templateSaveCmd.Template.Key != customerportalapp.CapabilityTemplatePublicSKUDirectShip || templateSaveCmd.Template.ThemeKey != customerportalapp.PortalThemeCleanOps {
		t.Fatalf("template save command=%+v", templateSaveCmd)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/customers/147/capability-template", strings.NewReader(`{"template_key":"public_sku_direct_ship"}`))
	applyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	applyRec := httptest.NewRecorder()
	e.ServeHTTP(applyRec, applyReq)
	if applyRec.Code != http.StatusOK || !strings.Contains(applyRec.Body.String(), `"capability_template_key":"public_sku_direct_ship"`) {
		t.Fatalf("apply status=%d body=%s", applyRec.Code, applyRec.Body.String())
	}
	if templateCmd.CustomerID != 147 || templateCmd.TemplateKey != customerportalapp.CapabilityTemplatePublicSKUDirectShip {
		t.Fatalf("template command=%+v", templateCmd)
	}
}

func TestPortalAdminCapabilityTemplateAPIsExposeTreeCopyAndInactiveState(t *testing.T) {
	child, _ := customerportalapp.CustomerCapabilityTemplateByKey(customerportalapp.CapabilityTemplatePublicSKUDirectShip)
	child.Key = "public_sku_direct_ship_b"
	child.ParentTemplateKey = customerportalapp.CapabilityTemplatePublicSKUDirectShip
	child.Label = "模板 B"
	child.Active = false
	child.SortOrder = 20
	var templateSaveCmd customerportalapp.SaveCapabilityTemplateCommand
	var templateCopyCmd customerportalapp.CopyCapabilityTemplateCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		templates:       []customerportalapp.CapabilityTemplate{child},
		templateSaveCmd: &templateSaveCmd,
		templateCopyCmd: &templateCopyCmd,
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/capability-templates", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	for _, want := range []string{
		`"key":"public_sku_direct_ship_b"`,
		`"parent_template_key":"public_sku_direct_ship"`,
		`"active":false`,
		`"sort_order":20`,
	} {
		if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), want) {
			t.Fatalf("template list missing %s: status=%d body=%s", want, listRec.Code, listRec.Body.String())
		}
	}

	saveReq := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/capability-templates/public_sku_direct_ship_b", strings.NewReader(`{"label":"模板 B 已停用","parent_template_key":"public_sku_direct_ship","active":false,"sort_order":21,"theme_key":"clean_ops","miniapp_entry_mode":"services","erp_permissions":["customer_processing.read"],"erp_view_keys":["customerProcessingPortal"],"capabilities":[{"code":"direct_ship","enabled":true}]}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save inactive status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}
	if templateSaveCmd.Template.ParentTemplateKey != customerportalapp.CapabilityTemplatePublicSKUDirectShip || templateSaveCmd.Template.Active || templateSaveCmd.Template.SortOrder != 21 {
		t.Fatalf("template save command missing tree/inactive fields=%+v", templateSaveCmd)
	}

	copyReq := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/capability-templates/public_sku_direct_ship/copy", strings.NewReader(`{"new_key":"public_sku_direct_ship_c","label":"模板 C"}`))
	copyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	copyRec := httptest.NewRecorder()
	e.ServeHTTP(copyRec, copyReq)
	if copyRec.Code != http.StatusOK || !strings.Contains(copyRec.Body.String(), `"parent_template_key":"public_sku_direct_ship"`) {
		t.Fatalf("copy status=%d body=%s", copyRec.Code, copyRec.Body.String())
	}
	if templateCopyCmd.SourceKey != customerportalapp.CapabilityTemplatePublicSKUDirectShip || templateCopyCmd.NewKey != "public_sku_direct_ship_c" || templateCopyCmd.Label != "模板 C" {
		t.Fatalf("template copy command=%+v", templateCopyCmd)
	}

	copyNameReq := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/capability-templates/public_sku_direct_ship/copy", strings.NewReader(`{"label":"岩师傅模板"}`))
	copyNameReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	copyNameRec := httptest.NewRecorder()
	e.ServeHTTP(copyNameRec, copyNameReq)
	if copyNameRec.Code != http.StatusOK {
		t.Fatalf("copy with label-only status=%d body=%s", copyNameRec.Code, copyNameRec.Body.String())
	}
	if templateCopyCmd.SourceKey != customerportalapp.CapabilityTemplatePublicSKUDirectShip || templateCopyCmd.NewKey != "" || templateCopyCmd.Label != "岩师傅模板" {
		t.Fatalf("template copy label-only command=%+v", templateCopyCmd)
	}
}

func TestPortalAdminCapabilityTemplateERPWorkbenchUnavailableMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable}})

	req := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/capability-templates/retail_mall", strings.NewReader(`{"erp_permissions":["customer_processing.read"],"erp_view_keys":["customerProcessingPortal"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), customerportalapp.ErrCapabilityTemplateERPWorkbenchUnavailable.Error()) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPortalAdminERPBindingRejectsTemplatesWithoutWorkbench(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		err: errors.New("ERP workbench unavailable for capability template"),
	}})

	req := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/customers/147/erp-binding", strings.NewReader(`{"employee_id":23,"status":"active"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "ERP workbench") {
		t.Fatalf("erp binding status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPortalAdminMallProductUnavailableMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("mall product unavailable")}})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products", strings.NewReader(`{"product_id":7,"title":"客户专属商品","spec_g":250,"unit_price":88,"status":"published"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMallAdminAndMiniAPIs(t *testing.T) {
	var saveCmd customerportalapp.SaveMallProductCommand
	var orderCmd customerportalapp.CreateMallOrderCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		mallSaveCmd:  &saveCmd,
		mallOrderCmd: &orderCmd,
		mallRows: []customerportalapp.MallProduct{{
			ID: 11, ProductID: 7, Title: "乌拉嘎", Subtitle: "柑橘莓果", ImageURL: "/assets/mall_products/ulagaa.png",
			SpecG: 250, UnitPrice: 68, TemplateKey: customerportalapp.MallTemplateHero, Status: customerportalapp.MallProductStatusPublished,
		}},
		mallOptions: []customerportalapp.MallProductOption{{ID: 7, Name: "乌拉嘎", DefaultPrice: 88}},
		mallPage: customerportalapp.MallPage{
			ThemeKey:            customerportalapp.PortalThemeCleanOps,
			MiniappEntryMode:    customerportalapp.MiniappEntryModeMall,
			CurrentCustomerID:   147,
			CurrentCustomerName: "测试客户",
			Products: []customerportalapp.MallProduct{{
				ID: 11, ProductID: 7, Title: "乌拉嘎", Subtitle: "柑橘莓果", SpecG: 250, UnitPrice: 68,
				TemplateKey: customerportalapp.MallTemplateHero, Status: customerportalapp.MallProductStatusPublished,
			}, {
				ID: 12, ProductID: 8, Title: "誉观山挂耳", UnitPrice: 0, ProductKind: "drip_bag",
				SalesUnits: []string{"bag", "box"}, DripBagGrams: 10, DripBoxBagCount: 10,
			}},
		},
		mallOrder: customerportalapp.FulfillmentOrder{OrderID: 99, OrderNo: "SO-20260507-0099", PortalServiceCode: customerportalapp.PortalServiceMall, SourceWarehouse: "finished_goods"},
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/mall-products", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK ||
		!strings.Contains(listRec.Body.String(), `"rows":[`) ||
		!strings.Contains(listRec.Body.String(), `"product_options":[`) ||
		!strings.Contains(listRec.Body.String(), `"title":"乌拉嘎"`) ||
		!strings.Contains(listRec.Body.String(), `"template_key":"hero"`) {
		t.Fatalf("mall admin list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	saveReq := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products", strings.NewReader(`{"product_id":7,"title":"乌拉嘎","subtitle":"柑橘莓果","spec_g":250,"unit_price":68,"template_key":"hero","status":"published","sort_order":3}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK || !strings.Contains(saveRec.Body.String(), `"id":12`) {
		t.Fatalf("mall admin save status=%d body=%s", saveRec.Code, saveRec.Body.String())
	}
	if saveCmd.ProductID != 7 || saveCmd.Title != "乌拉嘎" || saveCmd.TemplateKey != customerportalapp.MallTemplateHero || saveCmd.Status != customerportalapp.MallProductStatusPublished {
		t.Fatalf("mall save command=%+v", saveCmd)
	}

	mallReq := httptest.NewRequest(http.MethodGet, "/api/mini/mall", nil)
	mallReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	mallRec := httptest.NewRecorder()
	e.ServeHTTP(mallRec, mallReq)
	if mallRec.Code != http.StatusOK ||
		!strings.Contains(mallRec.Body.String(), `"miniapp_entry_mode":"mall"`) ||
		!strings.Contains(mallRec.Body.String(), `"products":[`) ||
		!strings.Contains(mallRec.Body.String(), `"unit_price":68`) ||
		!strings.Contains(mallRec.Body.String(), `"product_kind":"drip_bag"`) ||
		!strings.Contains(mallRec.Body.String(), `"unit_price":0`) {
		t.Fatalf("mini mall status=%d body=%s", mallRec.Code, mallRec.Body.String())
	}

	orderReq := httptest.NewRequest(http.MethodPost, "/api/mini/mall/orders", strings.NewReader(`{"recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","shipping_amount":12,"items":[{"mall_product_id":11,"qty":2}]}`))
	orderReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	orderReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	orderRec := httptest.NewRecorder()
	e.ServeHTTP(orderRec, orderReq)
	if orderRec.Code != http.StatusOK ||
		!strings.Contains(orderRec.Body.String(), `"order_no":"SO-20260507-0099"`) ||
		!strings.Contains(orderRec.Body.String(), `"portal_service_code":"mall"`) {
		t.Fatalf("mini mall order status=%d body=%s", orderRec.Code, orderRec.Body.String())
	}
	if orderCmd.RecipientName != "张三" || len(orderCmd.Items) != 1 || orderCmd.Items[0].MallProductID != 11 || orderCmd.Items[0].Qty != 2 {
		t.Fatalf("mall order command=%+v", orderCmd)
	}
	if orderCmd.ShippingAmount != 0 {
		t.Fatalf("mall order shipping amount=%v, want 0 for customer-side order submit", orderCmd.ShippingAmount)
	}
}

func TestMallAdminImageUploadStoresAndServesPublicAsset(t *testing.T) {
	var imageCmd customerportalapp.UpdateMallProductImageCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{
			mallRows:     []customerportalapp.MallProduct{{ID: 12}},
			mallImageCmd: &imageCmd,
		},
		AssetDir: t.TempDir(),
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hero.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNGForMallProductUploadTest()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products/12/image", &body)
	uploadReq.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	uploadRec := httptest.NewRecorder()
	e.ServeHTTP(uploadRec, uploadReq)
	if uploadRec.Code != http.StatusOK ||
		imageCmd.ID != 12 ||
		!strings.HasPrefix(imageCmd.ImageURL, "/assets/mall_products/12/") ||
		!strings.Contains(uploadRec.Body.String(), `"image_url":"`) {
		t.Fatalf("upload status=%d cmd=%+v body=%s", uploadRec.Code, imageCmd, uploadRec.Body.String())
	}

	assetReq := httptest.NewRequest(http.MethodGet, imageCmd.ImageURL, nil)
	assetRec := httptest.NewRecorder()
	e.ServeHTTP(assetRec, assetReq)
	if assetRec.Code != http.StatusOK || !strings.HasPrefix(assetRec.Header().Get(echo.HeaderContentType), "image/png") || !bytes.HasPrefix(assetRec.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("asset status=%d content-type=%q body-prefix=%x", assetRec.Code, assetRec.Header().Get(echo.HeaderContentType), assetRec.Body.Bytes())
	}
}

func TestMallAdminImageUploadRejectsNonImageAsset(t *testing.T) {
	var imageCmd customerportalapp.UpdateMallProductImageCommand
	assetDir := t.TempDir()
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{mallImageCmd: &imageCmd},
		AssetDir:       assetDir,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "promo.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(`<script>alert("xss")</script>`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products/12/image", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "image file required") {
		t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, imageCmd, rec.Body.String())
	}
	if imageCmd.ID != 0 || imageCmd.ImageURL != "" {
		t.Fatalf("image command should not be called for non-image upload: %+v", imageCmd)
	}
}

func TestMallAdminImageUploadRejectsOversizedAsset(t *testing.T) {
	var imageCmd customerportalapp.UpdateMallProductImageCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{mallImageCmd: &imageCmd},
		AssetDir:       t.TempDir(),
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "huge.png")
	if err != nil {
		t.Fatal(err)
	}
	oversized := append(tinyPNGForMallProductUploadTest(), bytes.Repeat([]byte{0}, 8<<20)...)
	if _, err := part.Write(oversized); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products/12/image", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "image file too large") {
		t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, imageCmd, rec.Body.String())
	}
	if imageCmd.ID != 0 || imageCmd.ImageURL != "" {
		t.Fatalf("image command should not be called for oversized upload: %+v", imageCmd)
	}
}

func TestMallAdminImageUploadRejectsMissingMallProductWithoutWritingAsset(t *testing.T) {
	var imageCmd customerportalapp.UpdateMallProductImageCommand
	assetDir := t.TempDir()
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{
			mallImageCmd: &imageCmd,
			mallImageErr: errors.New("mall product unavailable"),
		},
		AssetDir: assetDir,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hero.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNGForMallProductUploadTest()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products/12/image", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d cmd=%+v body=%s", rec.Code, imageCmd, rec.Body.String())
	}
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing mall product image upload wrote orphan asset entries: %+v", entries)
	}
}

func TestMallAdminImageUploadCleansAssetWhenImageUpdateFails(t *testing.T) {
	assetDir := t.TempDir()
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{
			mallRows:     []customerportalapp.MallProduct{{ID: 12}},
			mallImageErr: errors.New("mall product unavailable"),
		},
		AssetDir: assetDir,
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hero.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(tinyPNGForMallProductUploadTest()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/customer-portal/admin/mall-products/12/image", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed mall product image update left orphan asset entries: %+v", entries)
	}
}

func tinyPNGForMallProductUploadTest() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
		0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0c, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x03, 0x01, 0x01, 0x00,
		0x18, 0xdd, 0x8d, 0xb0,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D',
		0xae, 0x42, 0x60, 0x82,
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
	var cmd customerportalapp.CreateFulfillmentOrderCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		directShip:     customerportalapp.DirectShipBatch{ID: 5, BatchNo: "DS-20260503-0005", Status: "submitted", TotalRows: 100},
		processing:     customerportalapp.ProcessingRequest{ID: 7, RequestNo: "PJ-20260503-0007", Status: "submitted"},
		fulfillment:    customerportalapp.FulfillmentOrder{OrderID: 9, OrderNo: "SO-20260504-0009", PortalServiceCode: customerportalapp.PortalServiceProcessingShipment, SourceWarehouse: "cust_147_processing"},
		fulfillmentCmd: &cmd,
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

	orderReq := httptest.NewRequest(http.MethodPost, "/api/mini/fulfillment-orders", strings.NewReader(`{"service_code":"processing_ship","recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","product_id":5,"spec_g":454,"qty":2,"shipping_amount":12}`))
	orderReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	orderReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	orderRec := httptest.NewRecorder()
	e.ServeHTTP(orderRec, orderReq)
	if orderRec.Code != http.StatusOK || !strings.Contains(orderRec.Body.String(), `"order_no":"SO-20260504-0009"`) || !strings.Contains(orderRec.Body.String(), `"source_warehouse":"cust_147_processing"`) {
		t.Fatalf("fulfillment status=%d body=%s", orderRec.Code, orderRec.Body.String())
	}
	if cmd.ShippingAmount != 0 {
		t.Fatalf("fulfillment shipping amount=%v, want 0 for customer-side order submit", cmd.ShippingAmount)
	}
}

func TestMiniFulfillmentOrderAPIIgnoresClientUnitPrice(t *testing.T) {
	var cmd customerportalapp.CreateFulfillmentOrderCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		fulfillment:    customerportalapp.FulfillmentOrder{OrderID: 9, OrderNo: "SO-20260504-0009", PortalServiceCode: customerportalapp.PortalServiceProductOrder, SourceWarehouse: "finished_goods"},
		fulfillmentCmd: &cmd,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/fulfillment-orders", strings.NewReader(`{"service_code":"product_order","recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","product_id":5,"spec_g":454,"qty":2,"unit_price":0.01,"shipping_amount":12}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cmd.UnitPrice != 0 {
		t.Fatalf("service command UnitPrice=%.2f, want 0 so backend pricing stays authoritative", cmd.UnitPrice)
	}
	if cmd.ShippingAmount != 0 {
		t.Fatalf("service command ShippingAmount=%.2f, want 0 so customer-side freight stays authoritative", cmd.ShippingAmount)
	}
}

func TestMiniFulfillmentOrderAPIForwardsDripSalesUnit(t *testing.T) {
	var cmd customerportalapp.CreateFulfillmentOrderCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		fulfillment:    customerportalapp.FulfillmentOrder{OrderID: 9, OrderNo: "SO-20260504-0009", PortalServiceCode: customerportalapp.PortalServiceProductOrder, SourceWarehouse: "finished_goods"},
		fulfillmentCmd: &cmd,
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/fulfillment-orders", strings.NewReader(`{"service_code":"product_order","recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","product_id":88,"spec_g":10,"qty":3,"sales_unit":"box"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cmd.SalesUnit != "box" {
		t.Fatalf("service command SalesUnit=%q, want box", cmd.SalesUnit)
	}
}

func TestMiniMallOrderUnavailableMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("mall product unavailable")}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/mall/orders", strings.NewReader(validMiniMallOrderJSON()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniDirectShipBatchEmptyRowsMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("total_rows invalid")}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/direct-ship/batches", strings.NewReader(`{"source_name":"空代发批次","total_rows":0}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniProcessingRequestUnavailableInputsMapToBadRequest(t *testing.T) {
	for _, errText := range []string{"input material unavailable", "target product unavailable"} {
		t.Run(errText, func(t *testing.T) {
			e := echo.New()
			RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New(errText)}})

			req := httptest.NewRequest(http.MethodPost, "/api/mini/processing-requests", strings.NewReader(validMiniProcessingRequestJSON()))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiniFulfillmentOrderProductUnavailableMapsToBadRequest(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: errors.New("product unavailable")}})

	req := httptest.NewRequest(http.MethodPost, "/api/mini/fulfillment-orders", strings.NewReader(validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProductOrder)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error":"invalid request"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniAPITemplateBusinessContract(t *testing.T) {
	type routeAccess struct {
		name    string
		method  string
		path    string
		body    string
		allowed bool
	}
	tests := []struct {
		templateKey string
		routes      []routeAccess
	}{
		{
			templateKey: customerportalapp.CapabilityTemplateProcessingFulfillment,
			routes: []routeAccess{
				{name: "orders", method: http.MethodGet, path: "/api/mini/services/orders", allowed: true},
				{name: "direct ship service", method: http.MethodGet, path: "/api/mini/services/directShip", allowed: true},
				{name: "processing service", method: http.MethodGet, path: "/api/mini/services/processing", allowed: true},
				{name: "inventory service", method: http.MethodGet, path: "/api/mini/services/inventory", allowed: true},
				{name: "settlement service", method: http.MethodGet, path: "/api/mini/services/settlement", allowed: true},
				{name: "product order service", method: http.MethodGet, path: "/api/mini/services/productOrder"},
				{name: "mall page", method: http.MethodGet, path: "/api/mini/mall"},
				{name: "mall order", method: http.MethodPost, path: "/api/mini/mall/orders", body: validMiniMallOrderJSON()},
				{name: "direct ship batch", method: http.MethodPost, path: "/api/mini/direct-ship/batches", body: validMiniDirectShipBatchJSON(), allowed: true},
				{name: "direct ship order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceDirectShip), allowed: true},
				{name: "processing request", method: http.MethodPost, path: "/api/mini/processing-requests", body: validMiniProcessingRequestJSON(), allowed: true},
				{name: "processing order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProcessingShipment), allowed: true},
				{name: "product order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProductOrder)},
			},
		},
		{
			templateKey: customerportalapp.CapabilityTemplatePublicSKUDirectShip,
			routes: []routeAccess{
				{name: "orders", method: http.MethodGet, path: "/api/mini/services/orders", allowed: true},
				{name: "direct ship service", method: http.MethodGet, path: "/api/mini/services/directShip", allowed: true},
				{name: "product order service", method: http.MethodGet, path: "/api/mini/services/productOrder", allowed: true},
				{name: "settlement service", method: http.MethodGet, path: "/api/mini/services/settlement", allowed: true},
				{name: "processing service", method: http.MethodGet, path: "/api/mini/services/processing"},
				{name: "inventory service", method: http.MethodGet, path: "/api/mini/services/inventory"},
				{name: "mall page", method: http.MethodGet, path: "/api/mini/mall"},
				{name: "mall order", method: http.MethodPost, path: "/api/mini/mall/orders", body: validMiniMallOrderJSON()},
				{name: "direct ship batch", method: http.MethodPost, path: "/api/mini/direct-ship/batches", body: validMiniDirectShipBatchJSON(), allowed: true},
				{name: "direct ship order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceDirectShip), allowed: true},
				{name: "product order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProductOrder), allowed: true},
				{name: "processing request", method: http.MethodPost, path: "/api/mini/processing-requests", body: validMiniProcessingRequestJSON()},
				{name: "processing order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProcessingShipment)},
			},
		},
		{
			templateKey: customerportalapp.CapabilityTemplateRetailMall,
			routes: []routeAccess{
				{name: "orders", method: http.MethodGet, path: "/api/mini/services/orders", allowed: true},
				{name: "mall page", method: http.MethodGet, path: "/api/mini/mall", allowed: true},
				{name: "mall order", method: http.MethodPost, path: "/api/mini/mall/orders", body: validMiniMallOrderJSON(), allowed: true},
				{name: "product order service", method: http.MethodGet, path: "/api/mini/services/productOrder"},
				{name: "direct ship service", method: http.MethodGet, path: "/api/mini/services/directShip"},
				{name: "processing service", method: http.MethodGet, path: "/api/mini/services/processing"},
				{name: "inventory service", method: http.MethodGet, path: "/api/mini/services/inventory"},
				{name: "settlement service", method: http.MethodGet, path: "/api/mini/services/settlement"},
				{name: "direct ship batch", method: http.MethodPost, path: "/api/mini/direct-ship/batches", body: validMiniDirectShipBatchJSON()},
				{name: "direct ship order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceDirectShip)},
				{name: "product order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProductOrder)},
				{name: "processing request", method: http.MethodPost, path: "/api/mini/processing-requests", body: validMiniProcessingRequestJSON()},
				{name: "processing order", method: http.MethodPost, path: "/api/mini/fulfillment-orders", body: validMiniFulfillmentOrderJSON(customerportalapp.PortalServiceProcessingShipment)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.templateKey, func(t *testing.T) {
			repo := &templateContractRepository{current: currentMiniContextForTemplate(t, tt.templateKey)}
			e := echo.New()
			RegisterRoutes(e, Dependencies{CustomerPortal: customerportalapp.NewService(repo, staticMiniIdentityProvider{})})

			for _, route := range tt.routes {
				t.Run(route.name, func(t *testing.T) {
					req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
					req.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
					if route.body != "" {
						req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					}
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, req)
					wantStatus := http.StatusForbidden
					if route.allowed {
						wantStatus = http.StatusOK
					}
					if rec.Code != wantStatus {
						t.Fatalf("%s %s status=%d body=%s, want %d", route.method, route.path, rec.Code, rec.Body.String(), wantStatus)
					}
				})
			}
		})
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

type staticMiniIdentityProvider struct{}

func (staticMiniIdentityProvider) Resolve(context.Context, string) (customerportalapp.MiniIdentity, error) {
	return customerportalapp.MiniIdentity{OpenID: "openid"}, nil
}

type templateContractRepository struct {
	current        customerportalapp.CurrentContext
	adminCustomers []customerportalapp.PortalAdminCustomer
	adminDetail    customerportalapp.PortalAdminDetail
}

func (r *templateContractRepository) CreateLoginSession(context.Context, customerportalapp.CreateLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{Token: "mini-token", MiniUserID: r.current.MiniUserID, CurrentCustomerID: r.current.CurrentCustomerID, ThemeKey: r.current.ThemeKey, MiniappEntryMode: r.current.MiniappEntryMode, Capabilities: r.current.Capabilities}, nil
}

func (r *templateContractRepository) CreatePhoneVerifiedLoginSession(context.Context, customerportalapp.CreatePhoneVerifiedLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{Token: "mini-token", MiniUserID: r.current.MiniUserID, CurrentCustomerID: r.current.CurrentCustomerID, ThemeKey: r.current.ThemeKey, MiniappEntryMode: r.current.MiniappEntryMode, Capabilities: r.current.Capabilities}, nil
}

func (r *templateContractRepository) CreatePasswordLoginSession(context.Context, customerportalapp.CreatePasswordLoginSessionCommand) (customerportalapp.LoginResult, error) {
	return customerportalapp.LoginResult{Token: "mini-token", MiniUserID: r.current.MiniUserID, CurrentCustomerID: r.current.CurrentCustomerID, ThemeKey: r.current.ThemeKey, MiniappEntryMode: r.current.MiniappEntryMode, Capabilities: r.current.Capabilities}, nil
}

func (r *templateContractRepository) CurrentContextByToken(context.Context, string) (customerportalapp.CurrentContext, error) {
	return r.current, nil
}

func (r *templateContractRepository) SwitchCurrentCustomer(context.Context, string, int64) (customerportalapp.CurrentContext, error) {
	return r.current, nil
}

func (r *templateContractRepository) LoadServicePage(_ context.Context, query customerportalapp.ServicePageQuery) (customerportalapp.ServicePage, error) {
	return customerportalapp.ServicePage{
		Key:    query.Key,
		Orders: []customerportalapp.CustomerOrderSummary{{OrderNo: "SO-HISTORY", GrandTotal: "128.00"}},
	}, nil
}

func (r *templateContractRepository) LoadBeanListPublication(context.Context, int64, int64) (customerportalapp.BeanListSummary, error) {
	return customerportalapp.BeanListSummary{}, nil
}

func (r *templateContractRepository) LoadBeanListPublicationAsset(context.Context, int64, string) (customerportalapp.BeanListPublicationAsset, error) {
	return customerportalapp.BeanListPublicationAsset{}, customerportalapp.ErrBeanListPublicationNotFound
}

func (r *templateContractRepository) SaveBeanListPublicationAsset(_ context.Context, asset customerportalapp.BeanListPublicationAsset, _ string) (customerportalapp.BeanListPublicationAsset, error) {
	return asset, nil
}

func (r *templateContractRepository) AcknowledgeBeanListPublication(context.Context, int64, int64, string) error {
	return nil
}

func (r *templateContractRepository) ListPortalAdminCustomers(context.Context, customerportalapp.PortalAdminCustomerQuery) ([]customerportalapp.PortalAdminCustomer, error) {
	return r.adminCustomers, nil
}

func (r *templateContractRepository) PortalAdminDetail(context.Context, int64) (customerportalapp.PortalAdminDetail, error) {
	return r.adminDetail, nil
}

func (r *templateContractRepository) UpdatePortalVisibility(context.Context, customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	return customerportalapp.PortalAdminDetail{}, nil
}

func (r *templateContractRepository) ListCapabilityTemplates(context.Context) ([]customerportalapp.CapabilityTemplate, error) {
	return customerportalapp.DefaultCapabilityTemplates(), nil
}

func (r *templateContractRepository) SaveCapabilityTemplate(context.Context, customerportalapp.SaveCapabilityTemplateCommand) (customerportalapp.CapabilityTemplate, error) {
	return customerportalapp.CapabilityTemplate{}, nil
}

func (r *templateContractRepository) ApplyCapabilityTemplate(context.Context, customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error) {
	return customerportalapp.PortalAdminDetail{}, nil
}

func (r *templateContractRepository) UpsertPortalERPBinding(context.Context, customerportalapp.UpsertPortalERPBindingCommand) (customerportalapp.PortalAdminDetail, error) {
	return customerportalapp.PortalAdminDetail{}, nil
}

func (r *templateContractRepository) ListMallProducts(context.Context) ([]customerportalapp.MallProduct, []customerportalapp.MallProductOption, error) {
	return nil, nil, nil
}

func (r *templateContractRepository) SaveMallProduct(context.Context, customerportalapp.SaveMallProductCommand) (customerportalapp.MallProduct, error) {
	return customerportalapp.MallProduct{}, nil
}

func (r *templateContractRepository) UpdateMallProductImage(context.Context, customerportalapp.UpdateMallProductImageCommand) (customerportalapp.MallProduct, error) {
	return customerportalapp.MallProduct{}, nil
}

func (r *templateContractRepository) LoadMallPage(context.Context, int64) (customerportalapp.MallPage, error) {
	return customerportalapp.MallPage{
		ThemeKey:         r.current.ThemeKey,
		MiniappEntryMode: r.current.MiniappEntryMode,
		Products: []customerportalapp.MallProduct{{
			ID: 11, ProductID: 8, Title: "乌拉嘎", SpecG: 250, UnitPrice: 68, Status: customerportalapp.MallProductStatusPublished,
		}},
	}, nil
}

func (r *templateContractRepository) CustomerOwnsOrder(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (r *templateContractRepository) CreateMallOrder(context.Context, customerportalapp.CreateMallOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	return customerportalapp.FulfillmentOrder{OrderID: 101, OrderNo: "SO-MALL", PortalServiceCode: customerportalapp.PortalServiceMall}, nil
}

func (r *templateContractRepository) CreateDirectShipBatch(context.Context, customerportalapp.CreateDirectShipBatchCommand) (customerportalapp.DirectShipBatch, error) {
	return customerportalapp.DirectShipBatch{ID: 201, BatchNo: "DS-201", Status: "submitted"}, nil
}

func (r *templateContractRepository) CreateProcessingRequest(context.Context, customerportalapp.CreateProcessingRequestCommand) (customerportalapp.ProcessingRequest, error) {
	return customerportalapp.ProcessingRequest{ID: 301, RequestNo: "PJ-301", Status: "submitted"}, nil
}

func (r *templateContractRepository) CreateFulfillmentOrder(_ context.Context, cmd customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	return customerportalapp.FulfillmentOrder{OrderID: 401, OrderNo: "SO-FULFILLMENT", PortalServiceCode: cmd.PortalServiceCode}, nil
}

func currentMiniContextForTemplate(t *testing.T, templateKey string) customerportalapp.CurrentContext {
	t.Helper()
	template, ok := customerportalapp.CustomerCapabilityTemplateByKey(templateKey)
	if !ok {
		t.Fatalf("template %s missing", templateKey)
	}
	capabilities := make([]customerportalapp.Capability, 0, len(template.Capabilities))
	for _, capability := range template.Capabilities {
		capabilities = append(capabilities, customerportalapp.Capability{
			Code:    capability.Code,
			Enabled: capability.Enabled,
			Config:  capability.Config,
		})
	}
	return customerportalapp.CurrentContext{
		MiniUserID:          3,
		CurrentCustomerID:   7,
		CurrentCustomerName: template.Label,
		ThemeKey:            template.ThemeKey,
		MiniappEntryMode:    template.MiniappEntryMode,
		Capabilities:        capabilities,
	}
}

func validMiniMallOrderJSON() string {
	return `{"recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","items":[{"mall_product_id":11,"qty":2}]}`
}

func validMiniDirectShipBatchJSON() string {
	return `{"source_name":"客户批量代发","total_rows":12}`
}

func validMiniProcessingRequestJSON() string {
	return `{"input_material_id":4,"input_qty_g":30000,"target_product_id":5,"target_spec_g":454,"target_qty":50}`
}

func validMiniFulfillmentOrderJSON(serviceCode string) string {
	return `{"service_code":"` + serviceCode + `","recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","product_id":8,"product_name":"乌拉嘎","spec_g":250,"qty":2}`
}
