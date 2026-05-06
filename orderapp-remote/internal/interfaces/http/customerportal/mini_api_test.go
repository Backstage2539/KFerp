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
	login       customerportalapp.LoginResult
	me          customerportalapp.CurrentContext
	service     customerportalapp.ServicePage
	filter      *customerportalapp.ServicePageFilter
	customers   []customerportalapp.PortalAdminCustomer
	detail      customerportalapp.PortalAdminDetail
	directShip  customerportalapp.DirectShipBatch
	processing  customerportalapp.ProcessingRequest
	fulfillment customerportalapp.FulfillmentOrder
	beanList    customerportalapp.BeanListSummary
	err         error
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

func (s fakeService) UpdatePortalVisibility(context.Context, customerportalapp.UpdatePortalVisibilityCommand) (customerportalapp.PortalAdminDetail, error) {
	if s.err != nil {
		return customerportalapp.PortalAdminDetail{}, s.err
	}
	return s.detail, nil
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
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{
		customers: []customerportalapp.PortalAdminCustomer{{ID: 147, Name: "13800138075", PortalEnabled: true, BindingCount: 1}},
		detail: customerportalapp.PortalAdminDetail{
			Customer: customerportalapp.PortalAdminCustomer{ID: 147, Name: "13800138075", DisplayName: "测试客户", PortalEnabled: true},
			Bindings: []customerportalapp.PortalUserBinding{{MiniUserID: 1, Phone: "13800138075", Role: "owner", Status: "approved"}},
			Capabilities: []customerportalapp.CapabilityOption{
				{Code: customerportalapp.CapabilityBeanList, Label: "我的豆单", Enabled: true},
				{Code: customerportalapp.CapabilityDirectShip, Label: "一件代发", Enabled: true},
			},
		},
	}})

	listReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers?q=13800138075", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), `"name":"13800138075"`) || !strings.Contains(listRec.Body.String(), `"binding_count":1`) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	detailReq := httptest.NewRequest(http.MethodGet, "/api/customer-portal/admin/customers/147", nil)
	detailRec := httptest.NewRecorder()
	e.ServeHTTP(detailRec, detailReq)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), `"bindings":[`) || !strings.Contains(detailRec.Body.String(), `"capabilities":[`) {
		t.Fatalf("detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}

	body := strings.NewReader(`{"display_name":"测试客户","enabled":true,"capabilities":[{"code":"bean_list","enabled":true},{"code":"direct_ship","enabled":false}]}`)
	saveReq := httptest.NewRequest(http.MethodPut, "/api/customer-portal/admin/customers/147/visibility", body)
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK || !strings.Contains(saveRec.Body.String(), `"display_name":"测试客户"`) {
		t.Fatalf("save status=%d body=%s", saveRec.Code, saveRec.Body.String())
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
