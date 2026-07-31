package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	customerportalapp "orderapp/internal/application/customerportal"
	salesapp "orderapp/internal/application/sales"

	"github.com/labstack/echo/v4"
)

type miniEmployeeSalesFake struct {
	listQuery      salesapp.OrderListQuery
	save           salesapp.SaveOrderCommand
	orderFormCalls int
	listCalls      int
	saveCalls      int
}

func (f *miniEmployeeSalesFake) ListOrders(_ context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	f.listCalls++
	f.listQuery = query
	return salesapp.OrderListResult{Rows: []salesapp.OrderRow{{ID: 1, OrderNo: "SO-1"}}}, nil
}
func (f *miniEmployeeSalesFake) OrderForm(context.Context, int64) (salesapp.OrderFormData, error) {
	f.orderFormCalls++
	return salesapp.OrderFormData{
		Today: "2026-07-30",
		Customers: []salesapp.CustomerOption{{
			ID: 8, Name: "客户A", Contact: "收货人A", Phone: "13800000000",
			Address: "上海市测试路1号", CompanyName: "客户A公司",
		}},
		Products: []salesapp.ProductOption{{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎",
			Name: "乌拉嘎 227g", SKUName: "227g袋装", SKUCode: "WLG-227", SpecLabel: "227g",
			NetContentQty: 227, NetContentUnit: "g", ProductKind: "roasted_bean",
			Tiers: []salesapp.ProductTierOption{{
				ID: 11, UnitPrice: 68, PublicationID: 901, QuantityBasis: "sales_spec_count",
				EffectiveSalesSpec: map[string]any{"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋"},
			}},
		}},
	}, nil
}
func (f *miniEmployeeSalesFake) SaveOrder(_ context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	f.saveCalls++
	f.save = cmd
	return salesapp.SaveOrderResult{OrderID: 9, OrderNo: "SO-9"}, nil
}

func employeePortalService() fakeService {
	return fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
		Roles: []string{"sales"}, Permissions: []string{"orders.read", "orders.write"},
	}}
}

func TestMiniEmployeeOrderListScopesSalesToOwnOrders(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders?q=SO", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"order_no":"SO-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.listQuery.Scope != "mine" || sales.listQuery.EmployeeID != 7 || sales.listQuery.Q != "SO" {
		t.Fatalf("query=%+v", sales.listQuery)
	}
}

func TestMiniEmployeeOrderListKeepsAdministratorScopeUnrestricted(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 1, Roles: []string{"admin"}, Permissions: []string{"orders.read"},
	}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || sales.listQuery.Scope != "" || sales.listQuery.EmployeeID != 0 {
		t.Fatalf("status=%d query=%+v body=%s", rec.Code, sales.listQuery, rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormSeparatesProductAndSpecsAndReturnsShippingDefaults(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	for _, want := range []string{
		`"name":"乌拉嘎"`,
		`"specs":[`,
		`"spec_label":"227g"`,
		`"sku_code":"WLG-227"`,
		`"products":[`,
		`"receiver_name":"收货人A"`,
		`"receiver_phone":"13800000000"`,
		`"receiver_address":"上海市测试路1号"`,
		`"receiver_company":"客户A公司"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("status=%d body=%s missing %s", rec.Code, body, want)
		}
	}
	if strings.Contains(body, `"name":"乌拉嘎 227g"`) {
		var response struct {
			ProductFamilies []struct {
				Name string `json:"name"`
			} `json:"product_families"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		for _, family := range response.ProductFamilies {
			if family.Name == "乌拉嘎 227g" {
				t.Fatalf("family product name must not carry specification: %s", body)
			}
		}
	}
	var response struct {
		Customers []struct {
			Py  string `json:"py"`
			Pyi string `json:"pyi"`
		} `json:"customers"`
		ProductFamilies []struct {
			Py    string `json:"py"`
			Pyi   string `json:"pyi"`
			Specs []struct {
				Py  string `json:"py"`
				Pyi string `json:"pyi"`
			} `json:"specs"`
		} `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Customers) != 1 || response.Customers[0].Py == "" || response.Customers[0].Pyi == "" {
		t.Fatalf("customer search fields missing: %#v", response.Customers)
	}
	if len(response.ProductFamilies) != 1 || response.ProductFamilies[0].Py == "" || response.ProductFamilies[0].Pyi == "" || len(response.ProductFamilies[0].Specs) != 1 || response.ProductFamilies[0].Specs[0].Py == "" || response.ProductFamilies[0].Specs[0].Pyi == "" {
		t.Fatalf("product search fields missing: %#v", response.ProductFamilies)
	}
}

func TestMiniEmployeeProductFamiliesFallBackToParentAndSKUFields(t *testing.T) {
	families := salesapp.BuildOrderProductFamilies([]salesapp.ProductOption{
		{ID: 550, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎"},
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 227g", SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g"},
		{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 454g", SpecLabel: "454g", NetContentQty: 454, NetContentUnit: "g"},
	})
	if len(families) != 1 || families[0]["name"] != "乌拉嘎" {
		t.Fatalf("families=%#v", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 2 || specs[0]["spec_label"] != "227g" || specs[1]["spec_label"] != "454g" {
		t.Fatalf("specs=%#v", specs)
	}
}

func TestMiniEmployeeOrderFormWithoutTokenDoesNotCallSalesOrLeakMasterData(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("unauthorized response must be written exactly once: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "客户A") || strings.Contains(rec.Body.String(), "乌拉嘎") || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("unauthorized response leaked order form data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderListAndCreateWithoutTokenStopBeforeSales(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/mini/employee/orders"},
		{method: http.MethodPost, path: "/api/mini/employee/orders", body: `{}`},
	} {
		req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, rec.Code, rec.Body.String())
		}
	}
	if sales.listCalls != 0 || sales.saveCalls != 0 || sales.orderFormCalls != 0 {
		t.Fatalf("sales called after auth failure: form=%d list=%d save=%d", sales.orderFormCalls, sales.listCalls, sales.saveCalls)
	}
}

func TestMiniEmployeeOrderFormWithExpiredTokenDoesNotCallSalesOrLeakMasterData(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, fakeService{err: customerportalapp.ErrMiniSessionNotFound}, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer expired")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("expired-token response must be written exactly once: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "客户A") || strings.Contains(rec.Body.String(), "乌拉嘎") || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("expired-token response leaked order form data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormWithoutPermissionDoesNotCallSales(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{AccountType: "employee", Roles: []string{"sales"}}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || sales.orderFormCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) || strings.Contains(rec.Body.String(), "product_families") {
		t.Fatalf("forbidden response must be one error object without master data: %s", rec.Body.String())
	}
}

func TestMiniEmployeeOrderFormAllowsAdministratorWithPermission(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", Roles: []string{"admin"}, Permissions: []string{"orders.write"},
	}}
	registerMiniEmployeeAPI(e, portal, sales)
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || sales.orderFormCalls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, sales.orderFormCalls, rec.Body.String())
	}
}

func TestMiniEmployeeOrderCreateWritesResponsibleEmployee(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-07-30","customer_id":8,"items":[{"product_id":3,"name":"咖啡豆","qty":2,"spec_g":454,"unit_price":68}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"order_no":"SO-9"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.save.ResponsibleID != 7 || sales.save.ResponsibleType != "employee" || !sales.save.OrderDate.Equal(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("save=%+v", sales.save)
	}
}

func TestMiniEmployeeAPIRejectsCustomerAccount(t *testing.T) {
	e := echo.New()
	registerMiniEmployeeAPI(e, fakeService{me: customerportalapp.CurrentContext{AccountType: "customer"}}, &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/orders", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
