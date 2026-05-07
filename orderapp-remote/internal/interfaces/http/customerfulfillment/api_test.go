package customerfulfillment

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	parseCmd          app.ParseImportCommand
	parseFile         string
	parseResult       app.ImportBatch
	applyCmd          app.ApplyImportCommand
	applyResult       app.ApplyResult
	settlementCmd     app.CreateSettlementCommand
	settlementResult  app.SettlementResult
	overviewQuery     app.OverviewQuery
	overviewResult    app.Overview
	listImportsQuery  app.ListImportsQuery
	listImportsResult []app.ImportBatch
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
