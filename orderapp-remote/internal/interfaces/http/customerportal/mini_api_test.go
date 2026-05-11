package customerportal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type fakeService struct {
	login           customerportalapp.LoginResult
	me              customerportalapp.CurrentContext
	service         customerportalapp.ServicePage
	filter          *customerportalapp.ServicePageFilter
	customers       []customerportalapp.PortalAdminCustomer
	detail          customerportalapp.PortalAdminDetail
	saveCmd         *customerportalapp.UpdatePortalVisibilityCommand
	templates       []customerportalapp.CapabilityTemplate
	templateSaveCmd *customerportalapp.SaveCapabilityTemplateCommand
	templateCmd     *customerportalapp.ApplyCapabilityTemplateCommand
	mallRows        []customerportalapp.MallProduct
	mallOptions     []customerportalapp.MallProductOption
	mallSaveCmd     *customerportalapp.SaveMallProductCommand
	mallImageCmd    *customerportalapp.UpdateMallProductImageCommand
	mallPage        customerportalapp.MallPage
	mallOrder       customerportalapp.FulfillmentOrder
	mallOrderCmd    *customerportalapp.CreateMallOrderCommand
	directShip      customerportalapp.DirectShipBatch
	processing      customerportalapp.ProcessingRequest
	fulfillment     customerportalapp.FulfillmentOrder
	beanList        customerportalapp.BeanListSummary
	err             error
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

func (s fakeService) ApplyCapabilityTemplate(_ context.Context, cmd customerportalapp.ApplyCapabilityTemplateCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	if s.templateCmd != nil {
		*s.templateCmd = cmd
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

func (s fakeService) CreateFulfillmentOrder(context.Context, string, customerportalapp.CreateFulfillmentOrderCommand) (customerportalapp.FulfillmentOrder, error) {
	if s.err != nil {
		return customerportalapp.FulfillmentOrder{}, s.err
	}
	return s.fulfillment, nil
}

func TestMiniLoginAndMeAPI(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		login: customerportalapp.LoginResult{
			Token:             "mini-token",
			MiniUserID:        3,
			ThemeKey:          customerportalapp.PortalThemePremiumPartner,
			CurrentCustomerID: 8,
		},
		me: customerportalapp.CurrentContext{
			MiniUserID: 3, CurrentCustomerID: 8, CurrentCustomerName: "客户A",
			ThemeKey:     customerportalapp.PortalThemePremiumPartner,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		},
	}})

	loginReq := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"code":"wx-code","phone":"13800138000"}`))
	loginReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK ||
		!strings.Contains(loginRec.Body.String(), `"token":"mini-token"`) ||
		!strings.Contains(loginRec.Body.String(), `"theme_key":"premium_partner"`) {
		t.Fatalf("login status=%d body=%s", loginRec.Code, loginRec.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
	meReq.Header.Set(echo.HeaderAuthorization, "Bearer mini-token")
	meRec := httptest.NewRecorder()
	e.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK ||
		!strings.Contains(meRec.Body.String(), `"current_customer_name":"客户A"`) ||
		!strings.Contains(meRec.Body.String(), `"theme_key":"premium_partner"`) ||
		!strings.Contains(meRec.Body.String(), customerportalapp.CapabilityDirectShip) {
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
		ThemeKey:            customerportalapp.PortalThemeCleanOps,
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
		!strings.Contains(listRec.Body.String(), `"threshold_lb":14`) ||
		!strings.Contains(listRec.Body.String(), `"erp_role_codes":["customer_direct_ship_customer"]`) {
		t.Fatalf("template list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	saveBody := strings.NewReader(`{"label":"公共 SKU 小批量代发","theme_key":"clean_ops","miniapp_entry_mode":"services","erp_role_codes":["customer_direct_ship_customer"],"erp_permissions":["customer_processing.read"],"erp_view_keys":["customerProcessingPortal"],"capabilities":[{"code":"direct_ship","enabled":true,"config":{"public_sku_aliases":true,"small_batch_price_rule":{"enabled":true,"threshold_lb":14,"tier_min_lb":15,"tier_max_lb":28}}}]}`)
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
		!strings.Contains(mallRec.Body.String(), `"unit_price":68`) {
		t.Fatalf("mini mall status=%d body=%s", mallRec.Code, mallRec.Body.String())
	}

	orderReq := httptest.NewRequest(http.MethodPost, "/api/mini/mall/orders", strings.NewReader(`{"recipient_name":"张三","recipient_phone":"13800138000","recipient_address":"上海市","items":[{"mall_product_id":11,"qty":2}]}`))
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
}

func TestMallAdminImageUploadStoresAndServesPublicAsset(t *testing.T) {
	var imageCmd customerportalapp.UpdateMallProductImageCommand
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{mallImageCmd: &imageCmd},
		AssetDir:       t.TempDir(),
	})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "hero.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("fake-image")); err != nil {
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
	if assetRec.Code != http.StatusOK || assetRec.Body.String() != "fake-image" {
		t.Fatalf("asset status=%d body=%q", assetRec.Code, assetRec.Body.String())
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
		directShip:  customerportalapp.DirectShipBatch{ID: 5, BatchNo: "DS-20260503-0005", Status: "submitted", TotalRows: 100},
		processing:  customerportalapp.ProcessingRequest{ID: 7, RequestNo: "PJ-20260503-0007", Status: "submitted"},
		fulfillment: customerportalapp.FulfillmentOrder{OrderID: 9, OrderNo: "SO-20260504-0009", PortalServiceCode: customerportalapp.PortalServiceProcessingShipment, SourceWarehouse: "cust_147_processing"},
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
