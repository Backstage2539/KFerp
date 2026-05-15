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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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

func TestCustomerOptionsAPIDoesNotRequireWorkbenchTemplate(t *testing.T) {
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
	if !strings.Contains(rec.Body.String(), "无工作台模板客户") {
		t.Fatalf("customer options must keep customer with valid external user even without workbench template: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "代加工工作台客户") {
		t.Fatalf("customer options must keep workbench customer: %s", rec.Body.String())
	}
}

func TestCustomerOptionsAPISkipsLegacyBindingWithoutLoginOrPassword(t *testing.T) {
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
			t.Fatalf("customer options should skip weak external-user customer %s: %s", unwanted, rec.Body.String())
		}
	}
}

func TestCustomerOptionsAPISkipsInactiveExternalUserBinding(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 205, Name: "历史停用绑定客户", CustomerType: "wholesale", Active: true},
		}},
	}
	svc := &fakeCustomerFulfillmentService{
		externalUsersByCustomer: map[int64][]app.CustomerExternalUser{
			205: {{CustomerID: 205, EmployeeID: 35, Phone: "13800138035", HasPassword: true, LoginEnabled: true, BindingStatus: "inactive"}},
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
		t.Fatalf("customer options should skip inactive external-user binding customer: %s", rec.Body.String())
	}
}

func TestApplyImportAPIReturnsApplySummary(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{applyResult: app.ApplyResult{BatchID: 55, AppliedRows: 8, DirectShipOrders: 1}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

	body := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","shipping_amount":12,"items":[{"product_id":12,"product_name":"岩师傅冷萃豆","spec":"100g","quantity_units":2},{"product_id":12,"product_name":"岩师傅冷萃豆","spec_g":227,"quantity_units":1}],"note":"门卫代收"}`
	req := httptest.NewRequest(http.MethodPost, "/api/customer-processing/portal/direct-ship-orders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("customer direct ship submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(svc.customerDirectShipCmd.Items) != 2 || svc.customerDirectShipCmd.Items[1].SpecG != 227 || svc.customerDirectShipCmd.ShippingAmount != 12 {
		t.Fatalf("direct ship multi-line cmd = %+v", svc.customerDirectShipCmd)
	}
}

func TestInternalProcessingAndDirectShipSubmitAPIsUseExplicitCustomer(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerProcessingResult: app.ProcessingOrderSummary{WorkOrderNo: "CP-20260509-0001", Status: "submitted", ProductName: "誉观山冷萃豆", QuantityG: 5000, Units: 50},
		customerDirectShipResult: app.DirectShipOrderSummary{OrderNo: "CDS-20260509-0001", Status: "submitted", ItemCount: 1},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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

	directShipBody := `{"receiver_name":"张三","receiver_phone":"13800000000","receiver_address":"浙江杭州","product_name":"誉观山冷萃豆","spec":"100g","quantity_units":2,"note":"管理员提交"}`
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
}

func TestInternalSubmitAPICapabilityUnavailableMapsToBadRequest(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerProcessingErr: errors.New("customer capability processing unavailable"),
		customerDirectShipErr: errors.New("customer capability direct_ship unavailable"),
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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

func TestCreateSettlementAPIRequiresPeriod(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		settlementResult: app.SettlementResult{BatchID: 88, CustomerID: 147, PeriodFrom: "2026-03-01", PeriodTo: "2026-03-31", FeeItems: 4, TotalAmountCents: 16300},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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
