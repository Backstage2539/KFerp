package customerportal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customerfulfillmentapp "orderapp/internal/application/customerfulfillment"
	customerportalapp "orderapp/internal/application/customerportal"

	"github.com/labstack/echo/v4"
)

type miniCustomerFulfillmentFake struct {
	calls             int
	catalogQuery      customerfulfillmentapp.MiniDirectShipCatalogQuery
	previewCmd        customerfulfillmentapp.MiniDirectShipCommand
	submitCmd         customerfulfillmentapp.MiniDirectShipCommand
	listQuery         customerfulfillmentapp.MiniDirectShipListQuery
	listResult        customerfulfillmentapp.MiniDirectShipListResult
	listErr           error
	detailCustomerID  int64
	detailRequestID   int64
	cancelCustomerID  int64
	cancelRequestID   int64
	cancelActor       string
	inventoryQuery    customerfulfillmentapp.CustomerInventoryListQuery
	inventoryResult   customerfulfillmentapp.CustomerInventoryListResult
	batchCustomerID   int64
	batchProductID    int64
	batchBomSpecID    int64
	batchBomVariantID int64
	batchSpecG        int64
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

func (f *miniCustomerFulfillmentFake) ListMiniDirectShipRequests(_ context.Context, query customerfulfillmentapp.MiniDirectShipListQuery) (customerfulfillmentapp.MiniDirectShipListResult, error) {
	f.calls++
	f.listQuery = query
	if f.listErr != nil {
		return customerfulfillmentapp.MiniDirectShipListResult{}, f.listErr
	}
	if f.listResult.Rows != nil {
		return f.listResult, nil
	}
	return customerfulfillmentapp.MiniDirectShipListResult{
		Rows:  []customerfulfillmentapp.MiniDirectShipRequest{{ID: 71, RequestNo: "DSR-71", Status: "reserved"}},
		Total: 1, Page: query.Page, Limit: query.Limit, TotalPages: 1,
	}, nil
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

func (f *miniCustomerFulfillmentFake) ListCustomerCentralInventory(_ context.Context, query customerfulfillmentapp.CustomerInventoryListQuery) (customerfulfillmentapp.CustomerInventoryListResult, error) {
	f.calls++
	f.inventoryQuery = query
	if f.inventoryResult.Rows != nil {
		return f.inventoryResult, nil
	}
	return customerfulfillmentapp.CustomerInventoryListResult{
		Rows:       []customerfulfillmentapp.CustomerInventorySummary{{ProductID: 911, SpecG: 1000, AvailableQty: 3}},
		Total:      1,
		Page:       1,
		Limit:      1,
		TotalPages: 1,
	}, nil
}

func (f *miniCustomerFulfillmentFake) ListCustomerCentralInventoryBatches(_ context.Context, query customerfulfillmentapp.CustomerInventoryBatchQuery) ([]customerfulfillmentapp.CustomerInventoryBatch, error) {
	f.calls++
	f.batchCustomerID, f.batchProductID = query.CustomerID, query.ProductID
	f.batchBomSpecID, f.batchBomVariantID, f.batchSpecG = query.BomSpecID, query.BomVariantID, query.SpecG
	return []customerfulfillmentapp.CustomerInventoryBatch{{BatchID: 9, ProductID: query.ProductID, BomSpecID: query.BomSpecID, BomVariantID: query.BomVariantID, SpecG: query.SpecG}}, nil
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

func TestMiniDirectShipSubmitForwardsCanonicalBOMSpecIdentity(t *testing.T) {
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
		"items":[{"product_id":911,"bom_spec_id":801,"bom_variant_id":901,"inventory_unit":"袋","qty":2}]
	}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	req.Header.Set("Idempotency-Key", "mini-ds-bom-spec")
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(fulfillment.submitCmd.Items) != 1 {
		t.Fatalf("items=%#v", fulfillment.submitCmd.Items)
	}
	got := fulfillment.submitCmd.Items[0]
	if got.ProductID != 911 || got.BomSpecID != 801 || got.BomVariantID != 901 || got.SpecG != 0 || got.InventoryUnit != "袋" {
		t.Fatalf("canonical item=%#v", got)
	}
}

func TestMiniDirectShipListBindsFiltersPaginationAndCurrentCustomer(t *testing.T) {
	fulfillment := &miniCustomerFulfillmentFake{listResult: customerfulfillmentapp.MiniDirectShipListResult{
		Rows:  []customerfulfillmentapp.MiniDirectShipRequest{{ID: 71, RequestNo: "DSR-71", Status: "shipped"}},
		Total: 37, Page: 3, Limit: 10, TotalPages: 4, HasNext: true,
	}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityDirectShip, Enabled: true}},
		}},
		CustomerFulfillment: fulfillment,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/direct-ship/requests?customer_id=999&q=%E7%94%B2%E5%BA%97&shipped_from=2026-08-01&shipped_to=2026-08-07&page=3&limit=10", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fulfillment.listQuery.CustomerID != 9 {
		t.Fatalf("list customer=%d, want token-bound customer 9", fulfillment.listQuery.CustomerID)
	}
	if fulfillment.listQuery.Q != "甲店" || fulfillment.listQuery.ShippedFrom != "2026-08-01" || fulfillment.listQuery.ShippedTo != "2026-08-07" {
		t.Fatalf("list filters = %#v", fulfillment.listQuery)
	}
	if fulfillment.listQuery.Page != 3 || fulfillment.listQuery.Limit != 10 {
		t.Fatalf("list pagination = %#v", fulfillment.listQuery)
	}
	var body customerfulfillmentapp.MiniDirectShipListResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}
	if body.Total != 37 || body.TotalPages != 4 || body.Page != 3 || body.Limit != 10 || !body.HasNext || len(body.Rows) != 1 {
		t.Fatalf("list body = %#v", body)
	}
}

func TestMiniDirectShipListReturnsSpecificShipmentDateValidationMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "invalid from", err: errors.New("shipped_from invalid"), want: "发货开始日期格式不正确，请使用 YYYY-MM-DD"},
		{name: "invalid to", err: errors.New("shipped_to invalid"), want: "发货结束日期格式不正确，请使用 YYYY-MM-DD"},
		{name: "reversed", err: errors.New("shipment date range invalid"), want: "发货开始日期不能晚于结束日期"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fulfillment := &miniCustomerFulfillmentFake{listErr: tc.err}
			e := echo.New()
			RegisterRoutes(e, Dependencies{
				CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
					MiniUserID: 17, CurrentCustomerID: 9,
					Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
				}},
				CustomerFulfillment: fulfillment,
			})
			req := httptest.NewRequest(http.MethodGet, "/api/mini/direct-ship/requests", nil)
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] != tc.want {
				t.Fatalf("body=%s err=%v want=%q", rec.Body.String(), err, tc.want)
			}
		})
	}
}

func TestMiniCustomerInventoryBindsTokenCustomerFiltersPaginationAndLegacyMode(t *testing.T) {
	fulfillment := &miniCustomerFulfillmentFake{inventoryResult: customerfulfillmentapp.CustomerInventoryListResult{
		Rows:       []customerfulfillmentapp.CustomerInventorySummary{{ProductID: 552, ProductName: "乌拉嘎 454g", SpecG: 454}},
		Total:      2,
		Page:       2,
		Limit:      1,
		TotalPages: 2,
	}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		CustomerFulfillment: fulfillment,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/customer-inventory?customer_id=999&q=%E4%B9%8C%E6%8B%89%E5%98%8E&page=2&limit=1", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fulfillment.inventoryQuery.CustomerID != 9 || fulfillment.inventoryQuery.Q != "乌拉嘎" || fulfillment.inventoryQuery.Page != 2 || fulfillment.inventoryQuery.Limit != 1 || fulfillment.inventoryQuery.LegacyAll {
		t.Fatalf("inventory query = %#v", fulfillment.inventoryQuery)
	}
	var body customerfulfillmentapp.CustomerInventoryListResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Total != 2 || body.TotalPages != 2 || body.Page != 2 || body.Limit != 1 || len(body.Rows) != 1 {
		t.Fatalf("body=%s err=%v", rec.Body.String(), err)
	}

	legacy := &miniCustomerFulfillmentFake{}
	e = echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		CustomerFulfillment: legacy,
	})
	req = httptest.NewRequest(http.MethodGet, "/api/mini/customer-inventory?customer_id=999", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || legacy.inventoryQuery.CustomerID != 9 || !legacy.inventoryQuery.LegacyAll {
		t.Fatalf("legacy status=%d query=%#v body=%s", rec.Code, legacy.inventoryQuery, rec.Body.String())
	}
}

func TestMiniCustomerInventoryBatchesPassesCanonicalBOMSpecIdentity(t *testing.T) {
	fulfillment := &miniCustomerFulfillmentFake{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{
		CustomerPortal: fakeService{me: customerportalapp.CurrentContext{
			MiniUserID: 17, CurrentCustomerID: 9,
			Capabilities: []customerportalapp.Capability{{Code: customerportalapp.CapabilityProcessing, Enabled: true}},
		}},
		CustomerFulfillment: fulfillment,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/customer-inventory/550/batches?bom_spec_id=91&bom_variant_id=191", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if fulfillment.batchCustomerID != 9 || fulfillment.batchProductID != 550 || fulfillment.batchBomSpecID != 91 || fulfillment.batchBomVariantID != 191 || fulfillment.batchSpecG != 0 {
		t.Fatalf("canonical batch identity customer/product/spec/variant/spec_g = %d/%d/%d/%d/%d", fulfillment.batchCustomerID, fulfillment.batchProductID, fulfillment.batchBomSpecID, fulfillment.batchBomVariantID, fulfillment.batchSpecG)
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
	if fulfillment.listQuery.CustomerID != 9 || fulfillment.detailCustomerID != 9 || fulfillment.inventoryQuery.CustomerID != 9 || fulfillment.batchCustomerID != 9 {
		t.Fatalf("customer scopes = list:%d detail:%d inventory:%d batch:%d", fulfillment.listQuery.CustomerID, fulfillment.detailCustomerID, fulfillment.inventoryQuery.CustomerID, fulfillment.batchCustomerID)
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
