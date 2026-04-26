package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestVueShellMigratesOrdersAuditAndRequirementTables(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"import OrdersView from './views/OrdersView.vue'",
		"import AuditView from './views/AuditView.vue'",
		"import RequirementsView from './views/RequirementsView.vue'",
		"orders: OrdersView",
		"audit: AuditView",
		"reqProduct: RequirementsView",
		"reqDev: RequirementsView",
		"reqUnit: RequirementsView",
		"reqApi: RequirementsView",
		"reqReview: RequirementsView",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing migrated Vue wiring %q", want)
		}
	}
}

func TestLegacyListRoutesRedirectToVueShell(t *testing.T) {
	e := echo.New()
	registerOrderRoutes(e, nil, "public")
	registerCoreRoutes(e, nil, "public")
	registerRequirementPages(e, nil, "public")

	cases := []struct {
		path string
		want string
	}{
		{path: "/orders?q=SO-1", want: "/vue-shell?view=orders&q=SO-1"},
		{path: "/audit?type=order", want: "/vue-shell?view=audit&type=order"},
		{path: "/req/product?page=2", want: "/vue-shell?view=reqProduct&page=2"},
		{path: "/req/dev", want: "/vue-shell?view=reqDev"},
		{path: "/req/unit", want: "/vue-shell?view=reqUnit"},
		{path: "/req/api", want: "/vue-shell?view=reqApi"},
		{path: "/req/review", want: "/vue-shell?view=reqReview"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("GET %s status = %d, want %d body=%s", tc.path, rec.Code, http.StatusFound, rec.Body.String())
		}
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Fatalf("GET %s Location = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestRequirementAPIListsCreatesAndCascadesReview(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerRequirementAPIs(e, pool, schema)

	createBody := strings.NewReader(`{"code":"PR-VUE-001","title":"旧需求表迁入 Vue","status":"review","assignee":"Codex"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/api/req/product", createBody)
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("POST /api/req/product status = %d body=%s", createRec.Code, createRec.Body.String())
	}

	reviewBody := strings.NewReader(`{"code":"REV-VUE-001","pr_code":"PR-VUE-001","title":"验收旧需求表迁移","status":"todo","assignee":"Van"}`)
	reviewReq := httptest.NewRequest(http.MethodPost, "/api/req/review", reviewBody)
	reviewReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	reviewRec := httptest.NewRecorder()
	e.ServeHTTP(reviewRec, reviewReq)
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("POST /api/req/review status = %d body=%s", reviewRec.Code, reviewRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodPost, "/api/req/review/status", strings.NewReader(`{"code":"REV-VUE-001","status":"done"}`))
	statusReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	statusRec := httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("POST /api/req/review/status status = %d body=%s", statusRec.Code, statusRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/req/product?limit=5", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/req/product status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), `"code":"PR-VUE-001"`) || !strings.Contains(listRec.Body.String(), `"status":"done"`) {
		t.Fatalf("GET /api/req/product missing cascaded row: %s", listRec.Body.String())
	}

	var prStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.req_product WHERE code='PR-VUE-001'`, schema)).Scan(&prStatus); err != nil {
		t.Fatal(err)
	}
	if prStatus != "done" {
		t.Fatalf("req_product status = %q, want done", prStatus)
	}
}

func TestAuditAPIListsRows(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.audit_logs(actor, entity_type, action, field, old_value, new_value)
		VALUES ('tester','order','update','notes','old','new');
	`, schema))

	e := echo.New()
	registerCoreRoutes(e, pool, schema)

	req := httptest.NewRequest(http.MethodGet, "/api/audit?type=order&q=tester", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/audit status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"actor":"tester"`) || !strings.Contains(rec.Body.String(), `"summary"`) {
		t.Fatalf("GET /api/audit response missing audit row: %s", rec.Body.String())
	}
}

func TestOrdersAPIListsRowsForVue(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id, order_no, order_date, customer_id, order_type_id, pay_status_id, ship_status_id, process_status_id, grand_total, is_void)
		VALUES (10, 'SO-VUE-001', '2026-04-26', 3, 2, 1, 1, 1, 188.50, false);
		INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value)
		VALUES (10, '测试员', 'created', '', 'SO-VUE-001');
	`, schema, schema))

	e := newOrderAPITestEcho(pool, schema)
	req := httptest.NewRequest(http.MethodGet, "/api/orders?q=SO-VUE-001", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/orders status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Rows []OrderRow `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 || payload.Rows[0].OrderNo != "SO-VUE-001" || payload.Rows[0].CreatedByEmployee != "测试员" {
		t.Fatalf("GET /api/orders rows = %+v, want SO-VUE-001 by 测试员", payload.Rows)
	}
}

func TestVueShellMigratesCatalogAndSettingsPages(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"import CustomersView from './views/CustomersView.vue'",
		"import ProductsView from './views/ProductsView.vue'",
		"import BomView from './views/BomView.vue'",
		"import CompanyStaffView from './views/CompanyStaffView.vue'",
		"import InventoryView from './views/InventoryView.vue'",
		"import MachinesView from './views/MachinesView.vue'",
		"import SenderSettingsView from './views/SenderSettingsView.vue'",
		"customers: CustomersView",
		"products: ProductsView",
		"bom: BomView",
		"departments: CompanyStaffView",
		"employees: CompanyStaffView",
		"inventory: InventoryView",
		"quotePrint: ProductsView",
		"machines: MachinesView",
		"senderSettings: SenderSettingsView",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing catalog/settings Vue wiring %q", want)
		}
	}
	if strings.Contains(content, "BOM_REACT_URL") {
		t.Fatal("App.vue should not route BOM through the React fallback")
	}
}

func TestCatalogAndSettingsRoutesRedirectToVueShell(t *testing.T) {
	e := echo.New()
	registerCustomerRoutes(e, nil, "public", "")
	registerProductRoutes(e, nil, "public")
	registerBomPages(e, nil, "public")
	registerCompanyStaffPages(e, nil, "public")
	registerFinishedInventoryPages(e, nil, "public")
	registerMachineCapacityPages(e, nil, "public")
	registerSenderSettingsPage(e, nil, "public")

	cases := []struct {
		path string
		want string
	}{
		{path: "/customers?q=Karen", want: "/vue-shell?view=customers&q=Karen"},
		{path: "/customers/new", want: "/vue-shell?view=customers&mode=new"},
		{path: "/customers/new?from=order", want: "/vue-shell?view=customers&from=order&mode=new"},
		{path: "/customers/7", want: "/vue-shell?view=customers&edit_id=7"},
		{path: "/products", want: "/vue-shell?view=products"},
		{path: "/products/7", want: "/vue-shell?view=products&edit_id=7"},
		{path: "/products/print", want: "/vue-shell?view=quotePrint"},
		{path: "/bom?product_id=7", want: "/vue-shell?view=bom&product_id=7"},
		{path: "/company/departments", want: "/vue-shell?view=departments"},
		{path: "/company/employees?department_id=1", want: "/vue-shell?view=employees&department_id=1"},
		{path: "/products/inventory", want: "/vue-shell?view=inventory"},
		{path: "/produce/machines", want: "/vue-shell?view=machines"},
		{path: "/settings/sender", want: "/vue-shell?view=senderSettings"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("GET %s status = %d, want %d body=%s", tc.path, rec.Code, http.StatusFound, rec.Body.String())
		}
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Fatalf("GET %s Location = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestCatalogDetailAPIsSupportVueEditors(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS raw_name TEXT;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS contact TEXT;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS phone TEXT;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS address TEXT;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS default_source_id INTEGER;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS default_order_type_id INTEGER;
		ALTER TABLE %s.customers ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
		CREATE TABLE IF NOT EXISTS %s.customer_assets (
			id BIGSERIAL PRIMARY KEY,
			customer_id BIGINT,
			kind TEXT,
			object_key TEXT,
			content_type TEXT,
			bytes BIGINT,
			sha256 TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT
		);
		CREATE TABLE IF NOT EXISTS %s.product_bom (
			product_id BIGINT PRIMARY KEY,
			yield_rate NUMERIC NOT NULL DEFAULT 0.8,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		INSERT INTO %s.customers(id,name,raw_name,contact,phone,address,active,default_source_id,default_order_type_id)
		VALUES (42,'Vue 客户','Vue 客户原名','联系人','13900000000','地址',true,1,2);
		INSERT INTO %s.product_price_tiers(product_id,spec_g,min_qty_units,max_qty_units,price_per_unit,active)
		VALUES (7,227,1,5,48,true);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerCustomerRoutes(e, pool, schema, t.TempDir())
	registerProductRoutes(e, pool, schema)

	customerDetailReq := httptest.NewRequest(http.MethodGet, "/api/customers/42", nil)
	customerDetailRec := httptest.NewRecorder()
	e.ServeHTTP(customerDetailRec, customerDetailReq)
	if customerDetailRec.Code != http.StatusOK {
		t.Fatalf("GET /api/customers/42 status = %d body=%s", customerDetailRec.Code, customerDetailRec.Body.String())
	}
	if !strings.Contains(customerDetailRec.Body.String(), `"name":"Vue 客户"`) || !strings.Contains(customerDetailRec.Body.String(), `"sources"`) {
		t.Fatalf("GET /api/customers/42 missing editor payload: %s", customerDetailRec.Body.String())
	}

	customerUpdate := strings.NewReader(`{"name":"Vue 客户更新","raw_name":"Vue 客户更新","contact":"新联系人","phone":"13911111111","address":"新地址","default_source_id":1,"default_order_type_id":2,"active":true}`)
	customerUpdateReq := httptest.NewRequest(http.MethodPut, "/api/customers/42", customerUpdate)
	customerUpdateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	customerUpdateRec := httptest.NewRecorder()
	e.ServeHTTP(customerUpdateRec, customerUpdateReq)
	if customerUpdateRec.Code != http.StatusOK {
		t.Fatalf("PUT /api/customers/42 status = %d body=%s", customerUpdateRec.Code, customerUpdateRec.Body.String())
	}
	if !strings.Contains(customerUpdateRec.Body.String(), `"name":"Vue 客户更新"`) {
		t.Fatalf("PUT /api/customers/42 missing updated customer: %s", customerUpdateRec.Body.String())
	}

	productDetailReq := httptest.NewRequest(http.MethodGet, "/api/products/7", nil)
	productDetailRec := httptest.NewRecorder()
	e.ServeHTTP(productDetailRec, productDetailReq)
	if productDetailRec.Code != http.StatusOK {
		t.Fatalf("GET /api/products/7 status = %d body=%s", productDetailRec.Code, productDetailRec.Body.String())
	}
	if !strings.Contains(productDetailRec.Body.String(), `"name":"橘皮乌龙"`) || !strings.Contains(productDetailRec.Body.String(), `"tiers"`) {
		t.Fatalf("GET /api/products/7 missing editor payload: %s", productDetailRec.Body.String())
	}

	productUpdate := strings.NewReader(`{"roast_level":"深烘","retail_price_100g":28,"retail_price_200g":50,"retail_price_227g":54,"retail_price_250g":58,"tiers":[{"spec_g":227,"min_qty":1,"max_qty":10,"unit_price":53}]}`)
	productUpdateReq := httptest.NewRequest(http.MethodPut, "/api/products/7", productUpdate)
	productUpdateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	productUpdateRec := httptest.NewRecorder()
	e.ServeHTTP(productUpdateRec, productUpdateReq)
	if productUpdateRec.Code != http.StatusOK {
		t.Fatalf("PUT /api/products/7 status = %d body=%s", productUpdateRec.Code, productUpdateRec.Body.String())
	}
	if !strings.Contains(productUpdateRec.Body.String(), `"roast_level":"深烘"`) || !strings.Contains(productUpdateRec.Body.String(), `"unit_price":53`) {
		t.Fatalf("PUT /api/products/7 missing updated product: %s", productUpdateRec.Body.String())
	}
}
