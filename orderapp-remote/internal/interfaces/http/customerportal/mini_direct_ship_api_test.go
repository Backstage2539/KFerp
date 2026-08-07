package customerportal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniCustomerFulfillmentFake struct {
	calls               int
	catalogQuery        customerfulfillmentapp.MiniDirectShipCatalogQuery
	previewCmd          customerfulfillmentapp.MiniDirectShipCommand
	submitCmd           customerfulfillmentapp.MiniDirectShipCommand
	listCustomerID      int64
	detailCustomerID    int64
	detailRequestID     int64
	cancelCustomerID    int64
	cancelRequestID     int64
	cancelActor         string
	inventoryCustomerID int64
	batchCustomerID     int64
	batchProductID      int64
	batchSpecG          int64
}

func (f *miniCustomerFulfillmentFake) MiniDirectShipCatalog(_ context.Context, query customerfulfillmentapp.MiniDirectShipCatalogQuery) (customerfulfillmentapp.MiniDirectShipCatalog, error) {
	f.calls++
	f.catalogQuery = query
	return customerfulfillmentapp.MiniDirectShipCatalog{CurrentCustomerID: query.CustomerID, ProductFamilies: []map[string]any{}}, nil
}

func (f *miniCustomerFulfillmentFake) PreviewMiniDirectShip(_ context.Context, cmd customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipPreview, error) {
	f.calls++
	f.previewCmd = cmd
	return customerfulfillmentapp.MiniDirectShipPreview{CanSubmit: true, Warehouses: []customerfulfillmentapp.MiniDirectShipPreviewWarehouse{}}, nil
}

func (f *miniCustomerFulfillmentFake) SubmitMiniDirectShip(_ context.Context, cmd customerfulfillmentapp.MiniDirectShipCommand) (customerfulfillmentapp.MiniDirectShipRequest, error) {
	f.calls++
	f.submitCmd = cmd
	return customerfulfillmentapp.MiniDirectShipRequest{ID: 71, RequestNo: "DSR-71", Status: "reserved", Items: cmd.Items}, nil
}

func (f *miniCustomerFulfillmentFake) ListMiniDirectShipRequests(_ context.Context, customerID int64, _ int) ([]customerfulfillmentapp.MiniDirectShipRequest, error) {
	f.calls++
	f.listCustomerID = customerID
	return []customerfulfillmentapp.MiniDirectShipRequest{{ID: 71, RequestNo: "DSR-71", Status: "reserved"}}, nil
}

func (f *miniCustomerFulfillmentFake) GetMiniDirectShipRequest(_ context.Context, customerID, requestID int64) (customerfulfillmentapp.MiniDirectShipRequest, error) {
	f.calls++
	f.detailCustomerID, f.detailRequestID = customerID, requestID
	return customerfulfillmentapp.MiniDirectShipRequest{ID: requestID, RequestNo: "DSR-71"}, nil
}

func (f *miniCustomerFulfillmentFake) CancelMiniDirectShipRequest(_ context.Context, customerID, requestID int64, actor string) (customerfulfillmentapp.MiniDirectShipRequest, error) {
	f.calls++
	f.cancelCustomerID, f.cancelRequestID, f.cancelActor = customerID, requestID, actor
	return customerfulfillmentapp.MiniDirectShipRequest{ID: requestID, Status: "cancelled"}, nil
}

func (f *miniCustomerFulfillmentFake) ListCustomerCentralInventory(_ context.Context, customerID int64) ([]customerfulfillmentapp.CustomerInventorySummary, error) {
	f.calls++
	f.inventoryCustomerID = customerID
	return []customerfulfillmentapp.CustomerInventorySummary{{ProductID: 911, SpecG: 1000, AvailableQty: 3}}, nil
}

func (f *miniCustomerFulfillmentFake) ListCustomerCentralInventoryBatches(_ context.Context, customerID, productID, specG int64) ([]customerfulfillmentapp.CustomerInventoryBatch, error) {
	f.calls++
	f.batchCustomerID, f.batchProductID, f.batchSpecG = customerID, productID, specG
	return []customerfulfillmentapp.CustomerInventoryBatch{{BatchID: 9, ProductID: productID, SpecG: specG}}, nil
}

func TestMiniDirectShipSubmitBindsCurrentCustomerAndIdempotencyHeader(t *testing.T) {
	fulfillment := &miniCustomerFulfillmentFake{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, EmployeeID: 19, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		}},
		CustomerFulfillment: fulfillment,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/direct-ship/requests", strings.NewReader(`{
		"recipient_name":"张三","recipient_phone":"13800138000","detail_address":"咖啡路 8 号",
		"items":[{"product_id":911,"spec_g":1000,"qty":2}]
	}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set("Idempotency-Key", "mini-ds-unique")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fulfillment.submitCmd.CustomerID != 9 || fulfillment.submitCmd.EmployeeID != 19 || fulfillment.submitCmd.MiniUserID != 17 {
		t.Fatalf("principal = %#v", fulfillment.submitCmd)
	}
	if fulfillment.submitCmd.IdempotencyKey != "mini-ds-unique" || fulfillment.submitCmd.Actor != "mini_employee:19" {
		t.Fatalf("idempotency/actor = %q/%q", fulfillment.submitCmd.IdempotencyKey, fulfillment.submitCmd.Actor)
	}
	var body customerfulfillmentapp.MiniDirectShipRequest
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.ID != 71 {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
}

func TestMiniCustomerFulfillmentRoutesWithoutTokenStopBeforeFulfillment(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "catalog", method: http.MethodGet, path: "/api/mini/direct-ship/catalog"},
		{name: "preview", method: http.MethodPost, path: "/api/mini/direct-ship/preview", body: `{}`},
		{name: "submit", method: http.MethodPost, path: "/api/mini/direct-ship/requests", body: `{}`},
		{name: "list requests", method: http.MethodGet, path: "/api/mini/direct-ship/requests"},
		{name: "request detail", method: http.MethodGet, path: "/api/mini/direct-ship/requests/71"},
		{name: "cancel request", method: http.MethodPost, path: "/api/mini/direct-ship/requests/71/cancel", body: `{}`},
		{name: "inventory", method: http.MethodGet, path: "/api/mini/customer-inventory"},
		{name: "inventory batches", method: http.MethodGet, path: "/api/mini/customer-inventory/911/batches?spec_g=1000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fulfillment := &miniCustomerFulfillmentFake{}
			e := echo.New()
			RegisterRoutes(e, Dependencies{
				CustomerPortal:      fakeService{},
				CustomerFulfillment: fulfillment,
			})
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response must be one valid JSON document: body=%q err=%v", rec.Body.String(), err)
			}
			if body["error"] != "mini token required" {
				t.Fatalf("body=%v", body)
			}
			if fulfillment.calls != 0 {
				t.Fatalf("unauthenticated request reached fulfillment %d time(s)", fulfillment.calls)
			}
		})
	}
}

func TestMiniDirectShipReadAllowsProcessingButWriteRequiresDirectShip(t *testing.T) {
	fulfillment := &miniCustomerFulfillmentFake{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		CustomerFulfillment: fulfillment,
	})
	for _, path := range []string{"/api/mini/direct-ship/requests", "/api/mini/direct-ship/requests/71", "/api/mini/customer-inventory", "/api/mini/customer-inventory/911/batches?spec_g=1000"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	if fulfillment.listCustomerID != 9 || fulfillment.detailCustomerID != 9 || fulfillment.inventoryCustomerID != 9 || fulfillment.batchCustomerID != 9 {
		t.Fatalf("customer scopes = list:%d detail:%d inventory:%d batch:%d", fulfillment.listCustomerID, fulfillment.detailCustomerID, fulfillment.inventoryCustomerID, fulfillment.batchCustomerID)
	}
	callsBeforeDeniedPreview := fulfillment.calls
	req := httptest.NewRequest(http.MethodPost, "/api/mini/direct-ship/preview", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("processing-only preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var deniedBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &deniedBody); err != nil {
		t.Fatalf("denied response must be one valid JSON document: body=%q err=%v", rec.Body.String(), err)
	}
	if fulfillment.calls != callsBeforeDeniedPreview {
		t.Fatalf("capability-denied request reached fulfillment")
	}
}

func TestMiniClosedLoopRetiresLegacyMiniDirectShipWrites(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal:      fakeService{me: customerportalapp.CurrentContext{CurrentCustomerID: 9}},
		CustomerFulfillment: &miniCustomerFulfillmentFake{},
	})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/mini/direct-ship/batches", strings.NewReader(`{"source_name":"old","total_rows":2}`))
	batchRec := httptest.NewRecorder()
	e.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != http.StatusGone {
		t.Fatalf("legacy batch status=%d body=%s", batchRec.Code, batchRec.Body.String())
	}
	orderReq := httptest.NewRequest(http.MethodPost, "/api/mini/fulfillment-orders", strings.NewReader(`{"service_code":"direct_ship"}`))
	orderReq.Header.Set(echo.HeaderAuthorization, "Bearer token")
	orderReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	orderRec := httptest.NewRecorder()
	e.ServeHTTP(orderRec, orderReq)
	if orderRec.Code != http.StatusGone {
		t.Fatalf("legacy direct order status=%d body=%s", orderRec.Code, orderRec.Body.String())
	}
}
