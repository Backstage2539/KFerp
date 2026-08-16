package customerfulfillment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"
	messagecenterapp "orderapp/internal/application/messagecenter"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestParseImportAPIAcceptsMultipartFile(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		parseResult: app.ImportBatch{
			ID:         77,
			CustomerID: 147,
			ImportType: app.ImportTypeDirectShipWorkbook,
			Summary:    app.ImportSummary{ValidRows: 3, DirectShipOrders: 1},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("import_type", string(app.ImportTypeDirectShipWorkbook)); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", " direct.xlsx ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("xlsx-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/147/imports/parse", &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("parse status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"batch_id":77`, `"valid_rows":3`, `"direct_ship_orders":1`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("parse response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.parseCmd.CustomerID != 147 || svc.parseCmd.ImportType != app.ImportTypeDirectShipWorkbook || svc.parseCmd.SourceFilename != "direct.xlsx" || svc.parseFile != "xlsx-bytes" {
		t.Fatalf("parse command = %+v file=%q", svc.parseCmd, svc.parseFile)
	}
}

func TestCustomerOptionsAPIReturnsBoundWholesaleCustomersForPicker(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 147, Name: "誉观山", CustomerType: "wholesale", CompanyName: "誉观山咖啡", Active: true},
			{ID: 148, Name: "停用客户", CustomerType: "wholesale", Active: false},
			{ID: 149, Name: "零售客户", CustomerType: "retail", Active: true},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		externalUsersByCustomer: map[int64][]app.CustomerExternalUser{
			147: {{CustomerID: 147, EmployeeID: 8, Phone: "13800138001", HasPassword: true, LoginEnabled: true, BindingStatus: "active"}},
		},
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			147: true,
			149: false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?q=誉观山&limit=60", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customers"`, `"id":147`, `"name":"誉观山"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("customer options response missing %s: %s", want, rec.Body.String())
		}
	}
	for _, unwanted := range []string{"停用客户", "零售客户"} {
		if strings.Contains(rec.Body.String(), unwanted) {
			t.Fatalf("customer options must hide %s: %s", unwanted, rec.Body.String())
		}
	}
	if customers.query.Query != "誉观山" || customers.query.Limit != 60 {
		t.Fatalf("customer query = %+v, want q 誉观山 limit 60", customers.query)
	}
}

func TestCustomerOptionsAPIUsesPortalCapabilityInsteadOfWholesaleType(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 301, Name: "渠道代发客户", CustomerType: "channel", Active: true},
			{ID: 302, Name: "未开通工作台客户", CustomerType: "wholesale", Active: true},
			{ID: 303, Name: "停用渠道客户", CustomerType: "channel", Active: false},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			301: true,
			302: false,
			303: true,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "渠道代发客户") {
		t.Fatalf("customer options should include channel customer with portal workbench capability: %s", rec.Body.String())
	}
	for _, unwanted := range []string{"未开通工作台客户", "停用渠道客户"} {
		if strings.Contains(rec.Body.String(), unwanted) {
			t.Fatalf("customer options should hide %s: %s", unwanted, rec.Body.String())
		}
	}
}

func TestCustomerOptionsAPIRequiresPortalWorkbenchCapability(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 201, Name: "无工作台模板客户", CustomerType: "wholesale", Active: true},
			{ID: 202, Name: "代加工工作台客户", CustomerType: "wholesale", Active: true},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		externalUsersByCustomer: map[int64][]app.CustomerExternalUser{
			201: {{CustomerID: 201, EmployeeID: 31, Phone: "13800138031", HasPassword: true, LoginEnabled: true, BindingStatus: "active"}},
			202: {{CustomerID: 202, EmployeeID: 32, Phone: "13800138032", HasPassword: true, LoginEnabled: true, BindingStatus: "active"}},
		},
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			201: false,
			202: true,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "无工作台模板客户") {
		t.Fatalf("customer options must hide customer without portal workbench capability: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "代加工工作台客户") {
		t.Fatalf("customer options must keep workbench customer: %s", rec.Body.String())
	}
}

func TestCustomerOptionsAPIIncludesPortalEnabledCustomerWithoutWorkbenchTemplate(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 211, Name: "karen", CustomerType: "wholesale", Active: true, PortalEnabled: true},
			{ID: 212, Name: "未开通门户客户", CustomerType: "wholesale", Active: true, PortalEnabled: false},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			211: false,
			212: false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "karen") {
		t.Fatalf("customer options should include portal-enabled customer even without workbench template: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "未开通门户客户") {
		t.Fatalf("customer options should hide customer without portal switch: %s", rec.Body.String())
	}
}

func TestCustomerOptionsAPISkipsCustomersWithoutPortalCapability(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 203, Name: "无密码客户", CustomerType: "wholesale", Active: true},
			{ID: 204, Name: "禁用登录客户", CustomerType: "wholesale", Active: true},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		externalUsersByCustomer: map[int64][]app.CustomerExternalUser{
			203: {{CustomerID: 203, EmployeeID: 33, Phone: "13800138033", HasPassword: false, LoginEnabled: true, BindingStatus: "active"}},
			204: {{CustomerID: 204, EmployeeID: 34, Phone: "13800138034", HasPassword: true, LoginEnabled: false, BindingStatus: "active"}},
		},
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			203: false,
			204: false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, unwanted := range []string{"无密码客户", "禁用登录客户"} {
		if strings.Contains(rec.Body.String(), unwanted) {
			t.Fatalf("customer options should skip non-capable portal customer %s: %s", unwanted, rec.Body.String())
		}
	}
}

func TestCustomerOptionsAPISkipsInactivePortalWorkbenchCustomer(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 205, Name: "历史停用绑定客户", CustomerType: "wholesale", Active: true},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		externalUsersByCustomer: map[int64][]app.CustomerExternalUser{
			205: {{CustomerID: 205, EmployeeID: 35, Phone: "13800138035", HasPassword: true, LoginEnabled: true, BindingStatus: "inactive"}},
		},
		erpWorkbenchAvailableByCustomer: map[int64]bool{
			205: false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Customers: customers})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/customers?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer options status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "历史停用绑定客户") {
		t.Fatalf("customer options should skip inactive portal workbench customer: %s", rec.Body.String())
	}
}

func TestApplyImportAPIReturnsApplySummary(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{applyResult: app.ApplyResult{BatchID: 55, AppliedRows: 8, DirectShipOrders: 1}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/imports/55/apply", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"batch_id":55`, `"applied_rows":8`, `"direct_ship_orders":1`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("apply response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.applyCmd.BatchID != 55 {
		t.Fatalf("apply batch id = %d, want 55", svc.applyCmd.BatchID)
	}
}

func TestApplyImportAPICapabilityUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{applyErr: errors.New("customer capability direct_ship unavailable")}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/imports/55/apply", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("capability unavailable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "customer capability direct_ship unavailable") {
		t.Fatalf("capability unavailable response = %s", rec.Body.String())
	}
	if svc.applyCmd.BatchID != 55 {
		t.Fatalf("apply batch id = %d, want 55", svc.applyCmd.BatchID)
	}
}

func TestImportRowsAPIReturnsInvalidRowsForTroubleshooting(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		listImportRowsResult: []app.ImportRow{{
			BatchID:   55,
			SheetName: "生产工单",
			RowNo:     8,
			RowType:   "processing_work_order",
			Status:    "invalid",
			Error:     "投豆量无效",
		}},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/imports/55/rows?status=invalid&limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("rows status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"rows"`, `"sheet_name":"生产工单"`, `"row_no":8`, `"error":"投豆量无效"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("rows response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.listImportRowsQuery.BatchID != 55 || svc.listImportRowsQuery.Status != "invalid" || svc.listImportRowsQuery.Limit != 80 {
		t.Fatalf("rows query = %+v", svc.listImportRowsQuery)
	}
}

func TestImportPreviewAPIReturnsNonMutatingSummary(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		importPreviewResult: app.ImportPreview{
			Batch: app.ImportBatch{ID: 55, CustomerID: 147, ImportType: app.ImportTypeProcessingWorkbook, Status: "parsed"},
			Effects: []app.ImportPreviewEffect{
				{Label: "将应用有效行", Value: 44},
				{Label: "加工工单", Value: 136},
			},
			Warning: "预览不写入业务数据",
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/imports/55/preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"batch"`, `"id":55`, `"effects"`, `"label":"加工工单"`, `"warning":"预览不写入业务数据"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("preview response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.importPreviewQuery.BatchID != 55 {
		t.Fatalf("preview query batch = %d, want 55", svc.importPreviewQuery.BatchID)
	}
}

func TestOverviewAPIReturnsCustomerFulfillmentData(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		overviewResult: app.Overview{
			CustomerID:   147,
			CustomerName: "誉观山",
			Capabilities: []string{"direct_ship", "settlement"},
			CustodyBalances: []app.CustodyBalance{{
				ItemType:  "raw_bean",
				ItemName:  "埃塞花魁",
				QuantityG: 1000,
			}},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/147/overview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("overview status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_name":"誉观山"`, `"capabilities":["direct_ship","settlement"]`, `"item_name":"埃塞花魁"`, `"quantity_g":1000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("overview response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.overviewQuery.CustomerID != 147 {
		t.Fatalf("overview customer = %d, want 147", svc.overviewQuery.CustomerID)
	}
}

func TestCustomerFulfillmentOptionsAPIReturnsPickerData(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		optionsResult: app.CustomerFulfillmentOptions{
			CustomerSKUs: []app.CustomerSKUOption{{ProductID: 88, ProductName: "誉观山冷萃豆", Spec: "100g"}},
			CustodyItems: []app.CustodyItemOption{{ItemID: 19, ItemType: "raw_bean", ItemName: "埃塞花魁", QuantityG: 12000}},
			Employees:    []app.EmployeeOption{{ID: 23, Name: "誉观山客户", Active: true}},
			Recipients:   []app.RecipientOption{{ReceiverName: "张三", ReceiverPhone: "13800000000", ReceiverAddress: "浙江杭州"}},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/options", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("options status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_skus"`, `"product_name":"誉观山冷萃豆"`, `"custody_items"`, `"item_name":"埃塞花魁"`, `"employees"`, `"employee_id":23`, `"recipients"`, `"receiver_phone":"13800000000"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("options response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.optionsCustomerID != 149 {
		t.Fatalf("options customer id = %d, want 149", svc.optionsCustomerID)
	}
}

func TestCustomerFulfillmentOptionsAPIReturnsDripUnitPricingFields(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		optionsResult: app.CustomerFulfillmentOptions{
			CustomerSKUs: []app.CustomerSKUOption{{
				ProductID:       88,
				ProductName:     "誉观山挂耳",
				ProductKind:     "drip_bag",
				SalesUnits:      []string{"bag", "box"},
				DripBagGrams:    10,
				DripBoxBagCount: 10,
				Tiers: []app.CustomerSKUPriceTier{{
					ID:           7,
					ProductKind:  "drip_bag",
					SalesUnit:    "bag",
					Min:          1,
					UnitPrice:    6.5,
					UnitBagCount: 1,
					PriceSource:  "published_unit_price",
				}},
			}},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/options", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("options status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"product_kind":"drip_bag"`,
		`"sales_units":["bag","box"]`,
		`"drip_bag_grams":10`,
		`"drip_box_bag_count":10`,
		`"sales_unit":"bag"`,
		`"unit_bag_count":1`,
		`"price_source":"published_unit_price"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("drip options response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestCustomerPortalOverviewAPIDerivesCustomerFromEmployeeBinding(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerOverviewResult: app.CustomerPortalOverview{
			CustomerID:   149,
			CustomerName: "誉观山咖啡",
			Capabilities: []string{"direct_ship", "settlement"},
			CustodyBalances: []app.CustodyBalance{{
				ItemType:  "raw_bean",
				ItemName:  "埃塞花魁",
				QuantityG: 12000,
			}},
		},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-processing/portal/overview?customer_id=999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer portal overview status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_id":149`, `"customer_name":"誉观山咖啡"`, `"capabilities":["direct_ship","settlement"]`, `"item_name":"埃塞花魁"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("portal overview response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.customerOverviewEmployeeID != 23 {
		t.Fatalf("portal overview employee id = %d, want 23", svc.customerOverviewEmployeeID)
	}
}

func TestCustomerPortalOverviewAPILegacyWorkbenchBindingMapsToForbidden(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerOverviewErr: app.ErrCustomerERPBindingNotFound,
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-processing/portal/overview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("legacy workbench binding status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), app.ErrCustomerERPBindingNotFound.Error()) {
		t.Fatalf("legacy workbench binding response = %s", rec.Body.String())
	}
	if svc.customerOverviewEmployeeID != 23 {
		t.Fatalf("portal overview employee id = %d, want 23", svc.customerOverviewEmployeeID)
	}
}

func TestCustomerPortalOptionsAPIDerivesCustomerFromEmployeeBinding(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		portalOptionsResult: app.CustomerFulfillmentOptions{
			CustomerSKUs: []app.CustomerSKUOption{{ProductID: 7, ProductName: "客户A专属名", Source: "public_sku_alias"}},
			Recipients:   []app.RecipientOption{{ReceiverName: "张三", ReceiverPhone: "13800000000", ReceiverAddress: "浙江杭州"}},
		},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-processing/portal/options", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer portal options status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_skus"`, `"product_name":"客户A专属名"`, `"recipients"`, `"receiver_name":"张三"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("portal options response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.portalOptionsEmployeeID != 23 {
		t.Fatalf("portal options employee id = %d, want 23", svc.portalOptionsEmployeeID)
	}
}

func TestCustomerPortalProcessingSubmitAPIDerivesEmployeeAndIgnoresCustomerID(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerProcessingResult: app.ProcessingOrderSummary{WorkOrderNo: "CP-20260508-0001", Status: "submitted", ProductName: "誉观山冷萃豆", QuantityG: 5000, Units: 50},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"customer_id":999,"product_name":"誉观山冷萃豆","raw_bean_name":"埃塞花魁","input_quantity_g":5000,"planned_output_units":50,"expected_date":"2026-05-20","note":"急单"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/work-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer processing submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"work_order_no":"CP-20260508-0001"`, `"status":"submitted"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("processing submit response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.customerProcessingCmd.EmployeeID != 23 || svc.customerProcessingCmd.CustomerID != 0 || svc.customerProcessingCmd.ProductName != "誉观山冷萃豆" {
		t.Fatalf("processing submit cmd = %+v", svc.customerProcessingCmd)
	}
}

func TestCustomerPortalDirectShipSubmitAPIDerivesEmployee(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerDirectShipResult: app.DirectShipOrderSummary{OrderID: 98, OrderNo: "CDS-20260508-0001", Status: "submitted", ItemCount: 1},
	}
	messages := &fakeCustomerFulfillmentMessagePublisher{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, MessageCenter: messages})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","product_name":"誉观山冷萃豆","spec":"100g","quantity_units":2,"note":"门卫代收"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"order_no":"CDS-20260508-0001"`) {
		t.Fatalf("direct ship submit response = %s", rec.Body.String())
	}
	if svc.customerDirectShipCmd.EmployeeID != 23 || svc.customerDirectShipCmd.ReceiverName != "张三" || svc.customerDirectShipCmd.ProductName != "誉观山冷萃豆" {
		t.Fatalf("direct ship submit cmd = %+v", svc.customerDirectShipCmd)
	}
	if messages.cmd.EventType != "order.created" || messages.cmd.SourceID != 98 || messages.cmd.Payload["orders_scope"] != "fulfillment" {
		t.Fatalf("message publish cmd = %#v", messages.cmd)
	}
}

func TestCustomerPortalDirectShipSubmitAPIForwardsDripSalesUnit(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerDirectShipResult: app.DirectShipOrderSummary{OrderID: 98, OrderNo: "CDS-20260508-0001", Status: "submitted", ItemCount: 1},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","items":[{"product_id":88,"product_name":"誉观山挂耳","spec_g":10,"quantity_units":3,"sales_unit":"box"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.customerDirectShipCmd.Items) != 1 {
		t.Fatalf("direct ship items = %+v", svc.customerDirectShipCmd.Items)
	}
	if got := svc.customerDirectShipCmd.Items[0].SalesUnit; got != "box" {
		t.Fatalf("sales unit = %q, want box", got)
	}
}

func TestCustomerPortalDirectShipSubmitAPIForwardsCanonicalBOMSpecIdentity(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerDirectShipResult: app.DirectShipOrderSummary{OrderID: 98, OrderNo: "CDS-20260817-0001", Status: "submitted", ItemCount: 1},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","items":[{"product_id":88,"bom_spec_id":801,"bom_variant_id":901,"spec":"227g袋","sales_unit":"袋","quantity_units":3}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("canonical direct ship status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.customerDirectShipCmd.Items) != 1 {
		t.Fatalf("items=%#v", svc.customerDirectShipCmd.Items)
	}
	got := svc.customerDirectShipCmd.Items[0]
	if got.ProductID != 88 || got.BomSpecID != 801 || got.BomVariantID != 901 || got.SpecG != 0 || got.SalesUnit != "袋" {
		t.Fatalf("canonical item=%#v", got)
	}
}

func TestCustomerPortalDirectShipSubmitAPIAcceptsMultiLineItems(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerDirectShipResult: app.DirectShipOrderSummary{OrderID: 108, OrderNo: "CDS-20260508-0108", Status: "submitted", ItemCount: 2},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","shipping_amount":12,"items":[{"product_id":12,"product_name":"岩师傅冷萃豆","spec":"100g","quantity_units":2,"discount_type":"percent","discount_value":80},{"product_id":12,"product_name":"岩师傅冷萃豆","spec_g":227,"quantity_units":1,"discount_type":"amount","discount_value":10}],"note":"门卫代收"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.customerDirectShipCmd.Items) != 2 || svc.customerDirectShipCmd.Items[1].SpecG != 227 {
		t.Fatalf("direct ship multi-line cmd = %+v", svc.customerDirectShipCmd)
	}
	if svc.customerDirectShipCmd.ShippingAmount != 0 {
		t.Fatalf("customer portal direct ship shipping amount = %v, want 0", svc.customerDirectShipCmd.ShippingAmount)
	}
	if svc.customerDirectShipCmd.Items[0].DiscountType != "" || svc.customerDirectShipCmd.Items[0].DiscountValue != 0 {
		t.Fatalf("customer portal first item discount = %#v, want stripped customer-side discount", svc.customerDirectShipCmd.Items[0])
	}
	if svc.customerDirectShipCmd.Items[1].DiscountType != "" || svc.customerDirectShipCmd.Items[1].DiscountValue != 0 {
		t.Fatalf("customer portal second item discount = %#v, want stripped customer-side discount", svc.customerDirectShipCmd.Items[1])
	}
}

func TestInternalProcessingAndDirectShipSubmitAPIsUseExplicitCustomer(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerProcessingResult: app.ProcessingOrderSummary{WorkOrderNo: "CP-20260509-0001", Status: "submitted", ProductName: "誉观山冷萃豆", QuantityG: 5000, Units: 50},
		customerDirectShipResult: app.DirectShipOrderSummary{OrderNo: "CDS-20260509-0001", Status: "submitted", ItemCount: 1},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	processingBody := `{"product_name":"誉观山冷萃豆","raw_bean_name":"埃塞花魁","input_quantity_g":5000,"planned_output_units":50,"expected_date":"2026-05-20","note":"管理员提交"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/work-orders", strings.NewReader(processingBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("internal processing submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerProcessingCmd.CustomerID != 149 || svc.customerProcessingCmd.EmployeeID != 0 || svc.customerProcessingCmd.ProductName != "誉观山冷萃豆" {
		t.Fatalf("internal processing cmd = %+v", svc.customerProcessingCmd)
	}

	directShipBody := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","shipping_amount":12,"items":[{"product_id":12,"product_name":"誉观山冷萃豆","spec_g":227,"quantity_units":2,"discount_type":"percent","discount_value":85}],"note":"管理员提交"}`
	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/direct-ship-orders", strings.NewReader(directShipBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("internal direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerDirectShipCmd.CustomerID != 149 || svc.customerDirectShipCmd.EmployeeID != 0 || svc.customerDirectShipCmd.ReceiverName != "张三" {
		t.Fatalf("internal direct ship cmd = %+v", svc.customerDirectShipCmd)
	}
	if svc.customerDirectShipCmd.ShippingAmount != 12 {
		t.Fatalf("internal direct ship shipping amount = %v, want preserved ERP-side value", svc.customerDirectShipCmd.ShippingAmount)
	}
	if len(svc.customerDirectShipCmd.Items) != 1 || svc.customerDirectShipCmd.Items[0].DiscountType != "percent" || svc.customerDirectShipCmd.Items[0].DiscountValue != 85 {
		t.Fatalf("internal direct ship item discount = %#v, want preserved ERP-side discount", svc.customerDirectShipCmd.Items)
	}
}

func TestInternalDirectShipSubmitAPIUsesSingleAtomicWriter(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerDirectShipResult: app.DirectShipOrderSummary{
			OrderID:         321,
			OrderNo:         "CDS-20260806-0321",
			OrderDate:       "2026-08-06",
			ReceiverAddress: "张三 13800000000 浙江杭州",
			Status:          "submitted",
			ItemCount:       1,
		},
	}
	sales := &recordingSalesSaver{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			c.Set("operator_employee", "认证管理员")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: sales})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","items":[{"product_id":12,"product_name":"誉观山冷萃豆","spec_g":227,"quantity_units":2}],"note":"管理员提交"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("internal direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerDirectShipCalls != 1 {
		t.Errorf("SubmitCustomerDirectShipOrder calls=%d, want exactly 1", svc.customerDirectShipCalls)
	}
	if sales.calls != 0 {
		t.Errorf("SaveOrder calls=%d, want 0 because the customer fulfillment submit owns the transaction", sales.calls)
	}
	if svc.customerDirectShipCmd.Actor != "认证管理员" {
		t.Errorf("direct ship audit actor=%q, want authenticated operator", svc.customerDirectShipCmd.Actor)
	}
	for _, want := range []string{
		`"order_id":321`,
		`"order_no":"CDS-20260806-0321"`,
		`"order_date":"2026-08-06"`,
		`"receiver_address":"张三 13800000000 浙江杭州"`,
		`"status":"submitted"`,
		`"item_count":1`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("internal direct ship response missing %s: %s", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), `"cds_order_no"`) {
		t.Errorf("internal direct ship response exposes duplicate-order compatibility field: %s", rec.Body.String())
	}
}

func TestInternalDirectShipSubmitAPIFailureDoesNotFallThroughToSecondWriter(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{customerDirectShipErr: errors.New("atomic direct ship write failed")}
	sales := &recordingSalesSaver{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: sales})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","items":[{"product_id":12,"product_name":"誉观山冷萃豆","spec_g":227,"quantity_units":2}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "atomic direct ship write failed") {
		t.Fatalf("internal direct ship failure status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerDirectShipCalls != 1 {
		t.Errorf("SubmitCustomerDirectShipOrder calls=%d, want exactly 1", svc.customerDirectShipCalls)
	}
	if sales.calls != 0 {
		t.Errorf("SaveOrder calls=%d after failed unified submit, want 0", sales.calls)
	}
}

func TestInternalSubmitAPICapabilityUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerProcessingErr: errors.New("customer capability processing unavailable"),
		customerDirectShipErr: errors.New("customer capability direct_ship unavailable"),
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	processingBody := `{"product_name":"越权加工","input_quantity_g":5000,"planned_output_units":50}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/work-orders", strings.NewReader(processingBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "customer capability processing unavailable") {
		t.Fatalf("internal processing capability status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerProcessingCmd.CustomerID != 149 {
		t.Fatalf("internal processing cmd = %+v", svc.customerProcessingCmd)
	}

	directShipBody := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","product_name":"越权代发","quantity_units":1}`
	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/direct-ship-orders", strings.NewReader(directShipBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "customer capability direct_ship unavailable") {
		t.Fatalf("internal direct ship capability status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.customerDirectShipCmd.CustomerID != 149 {
		t.Fatalf("internal direct ship cmd = %+v", svc.customerDirectShipCmd)
	}
}

func TestInternalCustodyAdjustmentAPIUsesExplicitCustomer(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		custodyAdjustmentResult: app.CustodyBalance{ItemType: "raw_bean", ItemName: "埃塞花魁", QuantityG: 12000},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"item_type":"raw_bean","item_name":"埃塞花魁","quantity_g_delta":1000,"note":"手工补录"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/custody-adjustments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("custody adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"quantity_g":12000`) {
		t.Fatalf("custody adjustment response = %s", rec.Body.String())
	}
	if svc.custodyAdjustmentCmd.CustomerID != 149 || svc.custodyAdjustmentCmd.ItemName != "埃塞花魁" {
		t.Fatalf("custody adjustment cmd = %+v", svc.custodyAdjustmentCmd)
	}
}

func TestCustodyAdjustmentAPICapabilityUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		custodyAdjustmentErr: errors.New("customer capability inventory_custody unavailable"),
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	body := `{"item_type":"raw_bean","item_name":"越权生豆","quantity_g_delta":1000,"note":"越权补录"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/custody-adjustments", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("capability unavailable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "customer capability inventory_custody unavailable") {
		t.Fatalf("capability unavailable response = %s", rec.Body.String())
	}
	if svc.custodyAdjustmentCmd.CustomerID != 149 {
		t.Fatalf("custody adjustment command = %+v, want customer 149", svc.custodyAdjustmentCmd)
	}
}

func TestInternalERPBindingAPIUpsertsCustomerEmployeeBinding(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		erpBindingResult: app.CustomerERPBinding{CustomerID: 149, EmployeeID: 23, EmployeeName: "誉观山客户", Role: "customer", Status: "active"},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/erp-bindings", strings.NewReader(`{"employee_id":23,"status":"active"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("erp binding status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"customer_id":149`, `"employee_id":23`, `"status":"active"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("erp binding response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.erpBindingCmd.CustomerID != 149 || svc.erpBindingCmd.EmployeeID != 23 || svc.erpBindingCmd.Status != "active" {
		t.Fatalf("erp binding cmd = %+v", svc.erpBindingCmd)
	}
}

func TestInternalERPBindingAPIWorkbenchUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		erpBindingErr: errors.New("ERP workbench unavailable for capability template"),
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/erp-bindings", strings.NewReader(`{"employee_id":23,"status":"active"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ERP binding workbench status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ERP workbench unavailable for capability template") {
		t.Fatalf("ERP binding workbench response = %s", rec.Body.String())
	}
	if svc.erpBindingCmd.CustomerID != 149 || svc.erpBindingCmd.EmployeeID != 23 || svc.erpBindingCmd.Status != "active" {
		t.Fatalf("ERP binding command = %+v", svc.erpBindingCmd)
	}
}

func TestExternalUsersAPIManagesCustomerAccounts(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		externalUsersResult:        []app.CustomerExternalUser{{CustomerID: 149, EmployeeID: 23, Name: "誉观山账号", Phone: "13800138075", LoginEnabled: true, HasPassword: true, BindingStatus: "active"}},
		createExternalUserResult:   app.CustomerExternalUser{CustomerID: 149, EmployeeID: 24, Name: "誉观山新账号", Phone: "13800138076", LoginEnabled: true, HasPassword: true, BindingStatus: "active"},
		resetExternalUserResult:    app.CustomerExternalUser{CustomerID: 149, EmployeeID: 23, Name: "誉观山账号", Phone: "13800138075", LoginEnabled: true, HasPassword: true, BindingStatus: "active"},
		setExternalUserLoginResult: app.CustomerExternalUser{CustomerID: 149, EmployeeID: 23, Name: "誉观山账号", Phone: "13800138075", LoginEnabled: false, HasPassword: true, BindingStatus: "active"},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodGet, "/api/customer-fulfillment/149/external-users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"users"`) || !strings.Contains(rec.Body.String(), "誉观山账号") {
		t.Fatalf("list external users status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.externalUsersCustomerID != 149 {
		t.Fatalf("external users customer id = %d, want 149", svc.externalUsersCustomerID)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users", strings.NewReader(`{"name":"誉观山新账号","phone":"13800138076","password":"secret123"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"employee_id":24`) {
		t.Fatalf("create external user status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.createExternalUserCmd.CustomerID != 149 || svc.createExternalUserCmd.Phone != "13800138076" || svc.createExternalUserCmd.Password != "secret123" {
		t.Fatalf("create external user cmd = %+v", svc.createExternalUserCmd)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/password/reset", strings.NewReader(`{"password":"secret456"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"employee_id":23`) {
		t.Fatalf("reset external user status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.resetExternalUserCmd.CustomerID != 149 || svc.resetExternalUserCmd.EmployeeID != 23 || svc.resetExternalUserCmd.Password != "secret456" {
		t.Fatalf("reset external user cmd = %+v", svc.resetExternalUserCmd)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/external-users/23/login-enabled", strings.NewReader(`{"login_enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"login_enabled":false`) {
		t.Fatalf("toggle external user login status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.setExternalUserLoginCmd.CustomerID != 149 || svc.setExternalUserLoginCmd.EmployeeID != 23 || svc.setExternalUserLoginCmd.LoginEnabled {
		t.Fatalf("set external user login cmd = %+v", svc.setExternalUserLoginCmd)
	}
}

func TestExternalUserWriteAPIsUseAuthenticatedActorAndIgnoreXUser(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		createExternalUserResult:   app.CustomerExternalUser{CustomerID: 149, EmployeeID: 24},
		resetExternalUserResult:    app.CustomerExternalUser{CustomerID: 149, EmployeeID: 24},
		setExternalUserLoginResult: app.CustomerExternalUser{CustomerID: 149, EmployeeID: 24, LoginEnabled: true},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("operator_employee", "认证管理员")
			c.Set("actor", "认证回退账号")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	request := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set("X-User", "伪造管理员")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
		return rec
	}

	request("/api/customer-fulfillment/149/external-users", `{"name":"门户账号","phone":"13800138076","password":"secret123"}`)
	if svc.createExternalUserCmd.Actor != "认证管理员" {
		t.Fatalf("create actor=%q, want authenticated employee and X-User ignored", svc.createExternalUserCmd.Actor)
	}

	request("/api/customer-fulfillment/149/external-users/24/password/reset", `{"password":"secret456"}`)
	if svc.resetExternalUserCmd.Actor != "认证管理员" {
		t.Fatalf("reset actor=%q, want authenticated employee and X-User ignored", svc.resetExternalUserCmd.Actor)
	}

	request("/api/customer-fulfillment/149/external-users/24/login-enabled", `{"login_enabled":true}`)
	if svc.setExternalUserLoginCmd.Actor != "认证管理员" {
		t.Fatalf("login actor=%q, want authenticated employee and X-User ignored", svc.setExternalUserLoginCmd.Actor)
	}
}

func TestCreateSettlementAPIRequiresPeriod(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		settlementResult: app.SettlementResult{BatchID: 88, CustomerID: 147, PeriodFrom: "2026-03-01", PeriodTo: "2026-03-31", FeeItems: 4, TotalAmountCents: 16300},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/147/settlements", bytes.NewBufferString(`{"period_from":"","period_to":"2026-03-31"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing period status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/147/settlements", bytes.NewBufferString(`{"period_from":"2026-03-01","period_to":"2026-03-31"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create settlement status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"batch_id":88`, `"fee_items":4`, `"total_amount_cents":16300`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("settlement response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.settlementCmd.CustomerID != 147 || svc.settlementCmd.PeriodFrom != "2026-03-01" {
		t.Fatalf("settlement command = %+v", svc.settlementCmd)
	}
}

func TestCreateSettlementAPICapabilityUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		settlementErr: errors.New("customer capability settlement unavailable"),
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc, Sales: testSalesSaver})

	req := httptest.NewRequest(http.MethodPost, "/api/customer-fulfillment/149/settlements", bytes.NewBufferString(`{"period_from":"2026-03-01","period_to":"2026-03-31"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("capability unavailable status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "customer capability settlement unavailable") {
		t.Fatalf("capability unavailable response = %s", rec.Body.String())
	}
	if svc.settlementCmd.CustomerID != 149 {
		t.Fatalf("settlement command = %+v, want customer 149", svc.settlementCmd)
	}
}

type fakeCustomerFulfillmentService struct {
	parseCmd                        app.ParseImportCommand
	parseFile                       string
	parseResult                     app.ImportBatch
	applyCmd                        app.ApplyImportCommand
	applyResult                     app.ApplyResult
	applyErr                        error
	customerOverviewEmployeeID      int64
	customerOverviewResult          app.CustomerPortalOverview
	customerOverviewErr             error
	portalOptionsEmployeeID         int64
	portalOptionsResult             app.CustomerFulfillmentOptions
	customerProcessingCmd           app.SubmitCustomerProcessingWorkOrderCommand
	customerProcessingResult        app.ProcessingOrderSummary
	customerProcessingErr           error
	customerDirectShipCmd           app.SubmitCustomerDirectShipOrderCommand
	customerDirectShipResult        app.DirectShipOrderSummary
	customerDirectShipErr           error
	customerDirectShipCalls         int
	custodyAdjustmentCmd            app.AdjustCustodyInventoryCommand
	custodyAdjustmentResult         app.CustodyBalance
	custodyAdjustmentErr            error
	erpBindingCmd                   app.UpsertCustomerERPBindingCommand
	erpBindingResult                app.CustomerERPBinding
	erpBindingErr                   error
	listERPBindingsCustomerID       int64
	listERPBindingsResult           []app.CustomerERPBinding
	listERPBindingsByCustomer       map[int64][]app.CustomerERPBinding
	erpWorkbenchAvailableByCustomer map[int64]bool
	erpWorkbenchAvailableCustomerID int64
	erpWorkbenchAvailableErr        error
	externalUsersCustomerID         int64
	externalUsersByCustomer         map[int64][]app.CustomerExternalUser
	externalUsersResult             []app.CustomerExternalUser
	createExternalUserCmd           app.CreateExternalUserCommand
	createExternalUserResult        app.CustomerExternalUser
	resetExternalUserCmd            app.ResetExternalUserPasswordCommand
	resetExternalUserResult         app.CustomerExternalUser
	setExternalUserLoginCmd         app.SetExternalUserLoginEnabledCommand
	setExternalUserLoginResult      app.CustomerExternalUser
	optionsCustomerID               int64
	optionsResult                   app.CustomerFulfillmentOptions
	importPreviewQuery              app.ImportPreviewQuery
	importPreviewResult             app.ImportPreview
	listImportRowsQuery             app.ListImportRowsQuery
	listImportRowsResult            []app.ImportRow
	settlementCmd                   app.CreateSettlementCommand
	settlementResult                app.SettlementResult
	settlementErr                   error
	overviewQuery                   app.OverviewQuery
	overviewResult                  app.Overview
	listImportsQuery                app.ListImportsQuery
	listImportsResult               []app.ImportBatch
}

type fakeSalesSaver struct{}

func (s *fakeSalesSaver) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	return salesapp.SaveOrderResult{OrderID: 999, OrderNo: "SO-TEST-999"}, nil
}

var testSalesSaver = &fakeSalesSaver{}

type recordingSalesSaver struct {
	calls int
}

func (s *recordingSalesSaver) SaveOrder(ctx context.Context, cmd salesapp.SaveOrderCommand) (salesapp.SaveOrderResult, error) {
	s.calls++
	return salesapp.SaveOrderResult{OrderID: 999, OrderNo: "SO-TEST-999"}, nil
}

func (s *fakeCustomerFulfillmentService) ParseImport(ctx context.Context, cmd app.ParseImportCommand) (app.ImportBatch, error) {
	s.parseCmd = cmd
	if cmd.Reader != nil {
		b, _ := io.ReadAll(cmd.Reader)
		s.parseFile = string(b)
	}
	return s.parseResult, nil
}

func (s *fakeCustomerFulfillmentService) ApplyImport(ctx context.Context, cmd app.ApplyImportCommand) (app.ApplyResult, error) {
	s.applyCmd = cmd
	if s.applyErr != nil {
		return app.ApplyResult{}, s.applyErr
	}
	return s.applyResult, nil
}

func (s *fakeCustomerFulfillmentService) CustomerPortalOverview(ctx context.Context, employeeID int64) (app.CustomerPortalOverview, error) {
	s.customerOverviewEmployeeID = employeeID
	if s.customerOverviewErr != nil {
		return app.CustomerPortalOverview{}, s.customerOverviewErr
	}
	return s.customerOverviewResult, nil
}

func (s *fakeCustomerFulfillmentService) InternalCustomerPortalOverview(ctx context.Context, customerID int64) (app.CustomerPortalOverview, error) {
	return s.customerOverviewResult, nil
}

func (s *fakeCustomerFulfillmentService) InternalCustomerPortalOptions(ctx context.Context, customerID int64) (app.CustomerFulfillmentOptions, error) {
	return s.portalOptionsResult, nil
}

func (s *fakeCustomerFulfillmentService) CustomerPortalOptions(ctx context.Context, employeeID int64) (app.CustomerFulfillmentOptions, error) {
	s.portalOptionsEmployeeID = employeeID
	return s.portalOptionsResult, nil
}

func (s *fakeCustomerFulfillmentService) SubmitCustomerProcessingWorkOrder(ctx context.Context, cmd app.SubmitCustomerProcessingWorkOrderCommand) (app.ProcessingOrderSummary, error) {
	s.customerProcessingCmd = cmd
	if s.customerProcessingErr != nil {
		return app.ProcessingOrderSummary{}, s.customerProcessingErr
	}
	return s.customerProcessingResult, nil
}

func (s *fakeCustomerFulfillmentService) SubmitCustomerDirectShipOrder(ctx context.Context, cmd app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error) {
	s.customerDirectShipCalls++
	s.customerDirectShipCmd = cmd
	if s.customerDirectShipErr != nil {
		return app.DirectShipOrderSummary{}, s.customerDirectShipErr
	}
	return s.customerDirectShipResult, nil
}

func (s *fakeCustomerFulfillmentService) AdjustCustodyInventory(ctx context.Context, cmd app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error) {
	s.custodyAdjustmentCmd = cmd
	if s.custodyAdjustmentErr != nil {
		return app.CustodyBalance{}, s.custodyAdjustmentErr
	}
	return s.custodyAdjustmentResult, nil
}

func (s *fakeCustomerFulfillmentService) UpsertCustomerERPBinding(ctx context.Context, cmd app.UpsertCustomerERPBindingCommand) (app.CustomerERPBinding, error) {
	s.erpBindingCmd = cmd
	if s.erpBindingErr != nil {
		return app.CustomerERPBinding{}, s.erpBindingErr
	}
	return s.erpBindingResult, nil
}

func (s *fakeCustomerFulfillmentService) ListCustomerERPBindings(ctx context.Context, customerID int64) ([]app.CustomerERPBinding, error) {
	s.listERPBindingsCustomerID = customerID
	if s.listERPBindingsByCustomer != nil {
		return s.listERPBindingsByCustomer[customerID], nil
	}
	return s.listERPBindingsResult, nil
}

func (s *fakeCustomerFulfillmentService) CustomerERPWorkbenchAvailable(ctx context.Context, customerID int64) (bool, error) {
	s.erpWorkbenchAvailableCustomerID = customerID
	if s.erpWorkbenchAvailableErr != nil {
		return false, s.erpWorkbenchAvailableErr
	}
	if s.erpWorkbenchAvailableByCustomer != nil {
		return s.erpWorkbenchAvailableByCustomer[customerID], nil
	}
	return true, nil
}

func (s *fakeCustomerFulfillmentService) ListExternalUsers(ctx context.Context, customerID int64) ([]app.CustomerExternalUser, error) {
	s.externalUsersCustomerID = customerID
	if s.externalUsersByCustomer != nil {
		return s.externalUsersByCustomer[customerID], nil
	}
	return s.externalUsersResult, nil
}

func (s *fakeCustomerFulfillmentService) CreateExternalUser(ctx context.Context, cmd app.CreateExternalUserCommand) (app.CustomerExternalUser, error) {
	s.createExternalUserCmd = cmd
	return s.createExternalUserResult, nil
}

func (s *fakeCustomerFulfillmentService) ResetExternalUserPassword(ctx context.Context, cmd app.ResetExternalUserPasswordCommand) (app.CustomerExternalUser, error) {
	s.resetExternalUserCmd = cmd
	return s.resetExternalUserResult, nil
}

func (s *fakeCustomerFulfillmentService) SetExternalUserLoginEnabled(ctx context.Context, cmd app.SetExternalUserLoginEnabledCommand) (app.CustomerExternalUser, error) {
	s.setExternalUserLoginCmd = cmd
	return s.setExternalUserLoginResult, nil
}

func (s *fakeCustomerFulfillmentService) CustomerFulfillmentOptions(ctx context.Context, customerID int64) (app.CustomerFulfillmentOptions, error) {
	s.optionsCustomerID = customerID
	return s.optionsResult, nil
}

func (s *fakeCustomerFulfillmentService) ImportPreview(ctx context.Context, query app.ImportPreviewQuery) (app.ImportPreview, error) {
	s.importPreviewQuery = query
	return s.importPreviewResult, nil
}

func (s *fakeCustomerFulfillmentService) ListImportRows(ctx context.Context, query app.ListImportRowsQuery) ([]app.ImportRow, error) {
	s.listImportRowsQuery = query
	return s.listImportRowsResult, nil
}

func (s *fakeCustomerFulfillmentService) CreateSettlement(ctx context.Context, cmd app.CreateSettlementCommand) (app.SettlementResult, error) {
	s.settlementCmd = cmd
	if s.settlementErr != nil {
		return app.SettlementResult{}, s.settlementErr
	}
	return s.settlementResult, nil
}

func (s *fakeCustomerFulfillmentService) Overview(ctx context.Context, query app.OverviewQuery) (app.Overview, error) {
	s.overviewQuery = query
	return s.overviewResult, nil
}

func (s *fakeCustomerFulfillmentService) ListImports(ctx context.Context, query app.ListImportsQuery) ([]app.ImportBatch, error) {
	s.listImportsQuery = query
	return s.listImportsResult, nil
}

type fakeCustomerFulfillmentMessagePublisher struct {
	cmd messagecenterapp.PublishCommand
}

func (p *fakeCustomerFulfillmentMessagePublisher) Publish(ctx context.Context, cmd messagecenterapp.PublishCommand) (int64, error) {
	p.cmd = cmd
	return 1, nil
}

type fakeCustomerDirectory struct {
	query  customerapp.ListQuery
	result customerapp.ListResult
}

func (s *fakeCustomerDirectory) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	s.query = query
	return s.result, nil
}
