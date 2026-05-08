package customerfulfillment

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	customerapp "orderapp/internal/application/customer"
	app "orderapp/internal/application/customerfulfillment"
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

func TestCustomerOptionsAPIReturnsActiveCustomersForPicker(t *testing.T) {
	customers := &fakeCustomerDirectory{
		result: customerapp.ListResult{Rows: []customerapp.CustomerRow{
			{ID: 147, Name: "誉观山", CompanyName: "誉观山咖啡", Active: true},
			{ID: 148, Name: "停用客户", Active: false},
		}},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerFulfillment: &fakeCustomerFulfillmentService{}, Customers: customers})

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
	if strings.Contains(rec.Body.String(), "停用客户") {
		t.Fatalf("customer options must hide inactive customers: %s", rec.Body.String())
	}
	if customers.query.Query != "誉观山" || customers.query.Limit != 60 {
		t.Fatalf("customer query = %+v, want q 誉观山 limit 60", customers.query)
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
	for _, want := range []string{`"customer_name":"誉观山"`, `"item_name":"埃塞花魁"`, `"quantity_g":1000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("overview response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.overviewQuery.CustomerID != 147 {
		t.Fatalf("overview customer = %d, want 147", svc.overviewQuery.CustomerID)
	}
}

func TestCustomerPortalOverviewAPIDerivesCustomerFromEmployeeBinding(t *testing.T) {
	svc := &fakeCustomerFulfillmentService{
		customerOverviewResult: app.CustomerPortalOverview{
			CustomerID:   149,
			CustomerName: "誉观山咖啡",
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
	for _, want := range []string{`"customer_id":149`, `"customer_name":"誉观山咖啡"`, `"item_name":"埃塞花魁"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("portal overview response missing %s: %s", want, rec.Body.String())
		}
	}
	if svc.customerOverviewEmployeeID != 23 {
		t.Fatalf("portal overview employee id = %d, want 23", svc.customerOverviewEmployeeID)
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
		customerDirectShipResult: app.DirectShipOrderSummary{OrderNo: "CDS-20260508-0001", Status: "submitted", ItemCount: 1},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(23))
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{CustomerFulfillment: svc})

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

type fakeCustomerFulfillmentService struct {
	parseCmd                   app.ParseImportCommand
	parseFile                  string
	parseResult                app.ImportBatch
	applyCmd                   app.ApplyImportCommand
	applyResult                app.ApplyResult
	customerOverviewEmployeeID int64
	customerOverviewResult     app.CustomerPortalOverview
	customerProcessingCmd      app.SubmitCustomerProcessingWorkOrderCommand
	customerProcessingResult   app.ProcessingOrderSummary
	customerDirectShipCmd      app.SubmitCustomerDirectShipOrderCommand
	customerDirectShipResult   app.DirectShipOrderSummary
	custodyAdjustmentCmd       app.AdjustCustodyInventoryCommand
	custodyAdjustmentResult    app.CustodyBalance
	erpBindingCmd              app.UpsertCustomerERPBindingCommand
	erpBindingResult           app.CustomerERPBinding
	listERPBindingsCustomerID  int64
	listERPBindingsResult      []app.CustomerERPBinding
	importPreviewQuery         app.ImportPreviewQuery
	importPreviewResult        app.ImportPreview
	listImportRowsQuery        app.ListImportRowsQuery
	listImportRowsResult       []app.ImportRow
	settlementCmd              app.CreateSettlementCommand
	settlementResult           app.SettlementResult
	overviewQuery              app.OverviewQuery
	overviewResult             app.Overview
	listImportsQuery           app.ListImportsQuery
	listImportsResult          []app.ImportBatch
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
	return s.applyResult, nil
}

func (s *fakeCustomerFulfillmentService) CustomerPortalOverview(ctx context.Context, employeeID int64) (app.CustomerPortalOverview, error) {
	s.customerOverviewEmployeeID = employeeID
	return s.customerOverviewResult, nil
}

func (s *fakeCustomerFulfillmentService) SubmitCustomerProcessingWorkOrder(ctx context.Context, cmd app.SubmitCustomerProcessingWorkOrderCommand) (app.ProcessingOrderSummary, error) {
	s.customerProcessingCmd = cmd
	return s.customerProcessingResult, nil
}

func (s *fakeCustomerFulfillmentService) SubmitCustomerDirectShipOrder(ctx context.Context, cmd app.SubmitCustomerDirectShipOrderCommand) (app.DirectShipOrderSummary, error) {
	s.customerDirectShipCmd = cmd
	return s.customerDirectShipResult, nil
}

func (s *fakeCustomerFulfillmentService) AdjustCustodyInventory(ctx context.Context, cmd app.AdjustCustodyInventoryCommand) (app.CustodyBalance, error) {
	s.custodyAdjustmentCmd = cmd
	return s.custodyAdjustmentResult, nil
}

func (s *fakeCustomerFulfillmentService) UpsertCustomerERPBinding(ctx context.Context, cmd app.UpsertCustomerERPBindingCommand) (app.CustomerERPBinding, error) {
	s.erpBindingCmd = cmd
	return s.erpBindingResult, nil
}

func (s *fakeCustomerFulfillmentService) ListCustomerERPBindings(ctx context.Context, customerID int64) ([]app.CustomerERPBinding, error) {
	s.listERPBindingsCustomerID = customerID
	return s.listERPBindingsResult, nil
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

type fakeCustomerDirectory struct {
	query  customerapp.ListQuery
	result customerapp.ListResult
}

func (s *fakeCustomerDirectory) List(ctx context.Context, query customerapp.ListQuery) (customerapp.ListResult, error) {
	s.query = query
	return s.result, nil
}
