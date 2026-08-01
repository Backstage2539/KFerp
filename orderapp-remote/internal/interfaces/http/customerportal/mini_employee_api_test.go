package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	customerapp "orderapp/internal/application/customer"
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
	draft          *salesapp.EmployeeOrderDraft
	draftSave      salesapp.SaveEmployeeOrderDraftCommand
	draftSaveError error
	draftDeleteID  int64
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
			Address: "上海市测试路1号", CompanyName: "客户A公司", ResponsibleEmployeeID: 7,
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

func (f *miniEmployeeSalesFake) GetEmployeeOrderDraft(_ context.Context, employeeID int64) (*salesapp.EmployeeOrderDraft, error) {
	if f.draft == nil || f.draft.EmployeeID != employeeID {
		return nil, nil
	}
	copyDraft := *f.draft
	copyDraft.Payload = append(json.RawMessage(nil), f.draft.Payload...)
	return &copyDraft, nil
}

func (f *miniEmployeeSalesFake) SaveEmployeeOrderDraft(_ context.Context, cmd salesapp.SaveEmployeeOrderDraftCommand) (salesapp.EmployeeOrderDraft, error) {
	f.draftSave = cmd
	if f.draftSaveError != nil {
		return salesapp.EmployeeOrderDraft{}, f.draftSaveError
	}
	return salesapp.EmployeeOrderDraft{ID: 12, EmployeeID: cmd.EmployeeID, Payload: cmd.Payload, UpdatedAt: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)}, nil
}

func (f *miniEmployeeSalesFake) DeleteEmployeeOrderDraft(_ context.Context, employeeID int64, _ string) (bool, error) {
	f.draftDeleteID = employeeID
	return true, nil
}

type miniEmployeeCustomersFake struct {
	listActor        customerapp.MaintenancePrincipal
	editorActor      customerapp.MaintenancePrincipal
	upsertActor      customerapp.MaintenancePrincipal
	upsertID         *int64
	upsertCmd        customerapp.UpsertCommand
	upsertCalls      int
	upsertCompleted  bool
	listResult       customerapp.ListResult
	editorData       *customerapp.EditorData
	afterUpsertError error
	error            error
}

func (f *miniEmployeeCustomersFake) ListManaged(_ context.Context, actor customerapp.MaintenancePrincipal, _ customerapp.ListQuery) (customerapp.ListResult, error) {
	f.listActor = actor
	return f.listResult, f.error
}

func (f *miniEmployeeCustomersFake) EditorManaged(_ context.Context, actor customerapp.MaintenancePrincipal, _ int64) (*customerapp.EditorData, error) {
	f.editorActor = actor
	if f.upsertCompleted && f.afterUpsertError != nil {
		return nil, f.afterUpsertError
	}
	return f.editorData, f.error
}

func (f *miniEmployeeCustomersFake) UpsertManaged(_ context.Context, actor customerapp.MaintenancePrincipal, id *int64, cmd customerapp.UpsertCommand) (int64, error) {
	f.upsertCalls++
	f.upsertActor = actor
	f.upsertID = id
	f.upsertCmd = cmd
	if f.error != nil {
		return 0, f.error
	}
	f.upsertCompleted = true
	if id != nil {
		return *id, nil
	}
	return 99, nil
}

func employeePortalService() fakeService {
	return fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
		Roles: []string{"sales"}, Permissions: []string{"orders.read", "orders.write", "customers.read", "customers.write"},
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
		`"can_maintain":true`,
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

func TestMiniEmployeeOrderFormHidesMaintainActionWithoutCustomerPermissions(t *testing.T) {
	e := echo.New()
	portal := fakeService{me: customerportalapp.CurrentContext{
		AccountType: "employee", EmployeeID: 7, Roles: []string{"sales"}, Permissions: []string{"orders.write"},
	}}
	registerMiniEmployeeAPI(e, portal, &miniEmployeeSalesFake{})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"can_maintain":false`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMiniEmployeeOrderCreateLeavesResponsibilityToCustomerArchive(t *testing.T) {
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
	if sales.save.ResponsibleID != 0 || sales.save.ResponsibleType != "" || sales.save.DraftEmployeeID != 7 || !sales.save.OrderDate.Equal(time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("save=%+v", sales.save)
	}
}

func TestMiniEmployeeOrderCreateMapsEverySubmittedItem(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)
	body := `{"order_date":"2026-08-01","customer_id":8,"items":[{"product_id":3,"customer_product_alias_id":201,"name":"咖啡豆A","qty":2,"spec_g":227,"unit_price":68},{"product_id":4,"customer_product_alias_id":202,"name":"咖啡豆B","qty":3,"spec_g":454,"unit_price":88}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if sales.saveCalls != 1 || len(sales.save.Items) != 2 || sales.save.Items[0].Units != 2 || sales.save.Items[1].Units != 3 || sales.save.Items[0].CustomerProductAliasID != 201 || sales.save.Items[1].CustomerProductAliasID != 202 || sales.save.Items[0].ManualPrice == nil || *sales.save.Items[0].ManualPrice != 68 || sales.save.Items[1].ManualPrice == nil || *sales.save.Items[1].ManualPrice != 88 {
		t.Fatalf("save calls=%d items=%+v", sales.saveCalls, sales.save.Items)
	}
}

func TestMiniEmployeeCustomerMaintenanceContractsAndPrincipal(t *testing.T) {
	e := echo.New()
	customers := &miniEmployeeCustomersFake{
		listResult: customerapp.ListResult{
			Rows:                []customerapp.CustomerRow{{ID: 8, Name: "客户A", Active: true}},
			Sources:             []customerapp.Option{{ID: 1, Name: "小程序"}},
			OrderTypes:          []customerapp.Option{{ID: 2, Name: "零售"}},
			Employees:           []customerapp.Option{{ID: 7, Name: "销售甲"}},
			CustomerTypeOptions: customerapp.DefaultCustomerTypeOptions(),
			Total:               1,
		},
		editorData: &customerapp.EditorData{Customer: customerapp.CustomerEditData{
			ID: 8, Name: "客户A", CustomerType: customerapp.CustomerTypeRetail,
			DefaultSourceID: "1", DefaultOrderTypeID: "2", ResponsibleEmployeeID: "7",
			ResponsibleEmployeeName: "销售甲", Active: true,
		}},
	}
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)

	for _, request := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{method: http.MethodGet, path: "/api/mini/employee/customers", status: http.StatusOK},
		{method: http.MethodGet, path: "/api/mini/employee/customers/8", status: http.StatusOK},
		{method: http.MethodPut, path: "/api/mini/employee/customers/8", body: `{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2,"responsible_employee_id":999,"active":false,"portal_enabled":true}`, status: http.StatusOK},
	} {
		req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != request.status {
			t.Fatalf("%s %s status=%d body=%s", request.method, request.path, rec.Code, rec.Body.String())
		}
		if request.path == "/api/mini/employee/customers" {
			for _, want := range []string{`"rows"`, `"sources":[{"id":1,"name":"小程序"}]`, `"order_types":[{"id":2,"name":"零售"}]`, `"employees":[{"id":7,"name":"销售甲"}]`, `"customer_type_options"`, `"is_admin":false`, `"total":1`, `"has_next":false`} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Fatalf("list body=%s missing %s", rec.Body.String(), want)
				}
			}
		} else if !strings.Contains(rec.Body.String(), `"customer"`) {
			t.Fatalf("detail/upsert body=%s", rec.Body.String())
		}
	}
	if customers.listActor.EmployeeID != 7 || customers.listActor.IsAdmin || customers.editorActor.EmployeeID != 7 || customers.upsertActor.EmployeeID != 7 || customers.upsertID == nil || *customers.upsertID != 8 {
		t.Fatalf("principals list=%+v editor=%+v upsert=%+v id=%v", customers.listActor, customers.editorActor, customers.upsertActor, customers.upsertID)
	}
}

func TestMiniEmployeeCustomerMaintenanceMapsForbiddenAndNotFound(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{err: customerapp.ErrCustomerMaintenanceForbidden, status: http.StatusForbidden},
		{err: customerapp.ErrCustomerNotFound, status: http.StatusNotFound},
	} {
		e := echo.New()
		customers := &miniEmployeeCustomersFake{error: tc.err}
		registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
		req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/customers/88", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("error=%v status=%d body=%s", tc.err, rec.Code, rec.Body.String())
		}
	}
}

func TestMiniEmployeeCustomerWritesRequireReadAndWritePermissions(t *testing.T) {
	for _, permissions := range [][]string{
		{"customers.read"},
		{"customers.write"},
	} {
		e := echo.New()
		portal := fakeService{me: customerportalapp.CurrentContext{
			AccountType: "employee", EmployeeID: 7, EmployeeName: "销售甲",
			Roles: []string{"sales"}, Permissions: permissions,
		}}
		customers := &miniEmployeeCustomersFake{}
		registerMiniEmployeeAPI(e, portal, &miniEmployeeSalesFake{}, customers)
		req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(`{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2}`))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || customers.upsertActor.EmployeeID != 0 {
			t.Fatalf("permissions=%v status=%d upsert actor=%+v body=%s", permissions, rec.Code, customers.upsertActor, rec.Body.String())
		}
	}
}

func TestMiniEmployeeCustomerWritesExposeOnlySafeValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		want       string
		notContain string
	}{
		{name: "validation", err: customerapp.NewMaintenanceValidationError("来源不存在，请重新选择"), status: http.StatusBadRequest, want: "来源不存在，请重新选择"},
		{name: "repository failure", err: errors.New("pq: secret_schema.audit_logs unavailable"), status: http.StatusInternalServerError, notContain: "secret_schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			customers := &miniEmployeeCustomersFake{error: tc.err}
			registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
			req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(`{"name":"客户A","customer_type":"retail","default_source_id":1,"default_order_type_id":2}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || (tc.want != "" && !strings.Contains(rec.Body.String(), tc.want)) || (tc.notContain != "" && strings.Contains(rec.Body.String(), tc.notContain)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiniEmployeeCustomerCreateDoesNotReportFailureAfterCommittedPostReadRace(t *testing.T) {
	e := echo.New()
	customers := &miniEmployeeCustomersFake{afterUpsertError: customerapp.ErrCustomerMaintenanceForbidden}
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{}, customers)
	body := `{"name":"客户A","customer_type":"retail","company_name":"客户A公司","contact":"收货人A","phone":"021-12345678-801","address":"上海市测试路1号","default_source_id":1,"default_order_type_id":2,"responsible_employee_id":999,"active":false,"portal_enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/mini/employee/customers", strings.NewReader(body))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated || customers.upsertCalls != 1 {
		t.Fatalf("status=%d upsert calls=%d body=%s", rec.Code, customers.upsertCalls, rec.Body.String())
	}
	for _, want := range []string{
		`"id":99`, `"name":"客户A"`, `"responsible_employee_id":7`, `"active":true`, `"portal_enabled":false`,
		`"receiver_name":"收货人A"`, `"receiver_phone":"021-12345678-801"`, `"receiver_address":"上海市测试路1号"`, `"receiver_company":"客户A公司"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body=%s missing %s", rec.Body.String(), want)
		}
	}
}

func TestMiniEmployeeOrderDraftContractsUseTokenEmployeeOnly(t *testing.T) {
	e := echo.New()
	sales := &miniEmployeeSalesFake{draft: &salesapp.EmployeeOrderDraft{
		ID: 5, EmployeeID: 7, Payload: json.RawMessage(`{"customer_id":8}`), UpdatedAt: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC),
	}}
	registerMiniEmployeeAPI(e, employeePortalService(), sales)

	requests := []struct {
		method string
		body   string
		want   string
	}{
		{method: http.MethodGet, want: `"draft":{"id":5`},
		{method: http.MethodPut, body: `{"payload":{"customer_id":9,"items":[]}}`, want: `"draft":{"id":12`},
		{method: http.MethodDelete, want: `"deleted":true`},
	}
	for _, request := range requests {
		req := httptest.NewRequest(request.method, "/api/mini/employee/order-draft", strings.NewReader(request.body))
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), request.want) {
			t.Fatalf("%s status=%d body=%s want=%s", request.method, rec.Code, rec.Body.String(), request.want)
		}
	}
	if sales.draftSave.EmployeeID != 7 || sales.draftDeleteID != 7 || strings.Contains(string(sales.draftSave.Payload), "employee_id") {
		t.Fatalf("draft save=%+v delete employee=%d", sales.draftSave, sales.draftDeleteID)
	}
}

func TestMiniEmployeeOrderDraftSaveExposesOnlySafeValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		status     int
		want       string
		notContain string
	}{
		{name: "validation", err: salesapp.NewEmployeeOrderDraftValidationError("草稿内容不正确"), status: http.StatusBadRequest, want: "草稿内容不正确"},
		{name: "repository failure", err: errors.New("pq: secret_schema.employee_order_drafts unavailable"), status: http.StatusInternalServerError, want: `"error":"internal error"`, notContain: "secret_schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			sales := &miniEmployeeSalesFake{draftSaveError: tc.err}
			registerMiniEmployeeAPI(e, employeePortalService(), sales)
			req := httptest.NewRequest(http.MethodPut, "/api/mini/employee/order-draft", strings.NewReader(`{"payload":{"customer_id":9,"items":[]}}`))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) || (tc.notContain != "" && strings.Contains(rec.Body.String(), tc.notContain)) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
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
