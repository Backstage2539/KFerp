package customerportal

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	fulfillment "orderapp/internal/application/customerfulfillment"
	portal "orderapp/internal/application/customerportal"
	"strings"
	"testing"
)

type feeTestFulfillment struct {
	miniCustomerFulfillmentFake
	query fulfillment.AccountQuery
}

func (f *feeTestFulfillment) CustomerAccount(_ context.Context, q fulfillment.AccountQuery) (fulfillment.AccountData, error) {
	f.query = q
	return fulfillment.AccountData{OrderFeeGroups: []fulfillment.AccountOrderFeeGroup{{Order: fulfillment.AccountOrder{ID: 117}, PayableCents: 1410, PaidCents: 0, DueCents: 1410, FeeLines: []fulfillment.AccountFeeLine{{AccountFee: fulfillment.AccountFee{AmountCents: 1110}, Category: "shipping"}, {AccountFee: fulfillment.AccountFee{AmountCents: 300}, Category: "direct_ship_service"}}}}}, nil
}
func TestMiniOrderPageIncludesConfirmedFeesForCurrentCustomer(t *testing.T) {
	f := &feeTestFulfillment{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: portal.CurrentContext{CurrentCustomerID: 9}, service: portal.ServicePage{CurrentCustomerID: 9, Orders: []portal.CustomerOrderSummary{{ID: 117, GrandTotal: "0.00"}}}}, CustomerFulfillment: f})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/services/orders?customer_id=999", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Orders []struct {
			FeeSummary struct {
				Payable  int64 `json:"payable_cents"`
				Shipping int64 `json:"shipping_cents"`
				Service  int64 `json:"service_cents"`
			} `json:"fee_summary"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Orders) != 1 || body.Orders[0].FeeSummary.Payable != 1410 || body.Orders[0].FeeSummary.Shipping != 1110 || body.Orders[0].FeeSummary.Service != 300 {
		t.Fatalf("wrong fees: %s", rec.Body.String())
	}
	if f.query.CustomerID != 9 || f.query.View != "order_fees" || f.query.DateFrom != "" || f.query.DateTo != "" {
		t.Fatalf("wrong account query: %+v", f.query)
	}
}

func TestStockDirectShipMayPreviewPurchasePrices(t *testing.T) {
	f := &miniCustomerFulfillmentFake{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: portal.CurrentContext{CurrentCustomerID: 9, Capabilities: []portal.Capability{{Code: portal.CapabilityDirectShip, Enabled: true}}}}, CustomerFulfillment: f})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/order-price-tables/preview?usage_code=product_order", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if f.catalogQuery.CustomerID != 9 || f.catalogQuery.FulfillmentMode != "stock" {
		t.Fatalf("stock was not verified: %+v", f.catalogQuery)
	}
}

func TestPhoneLoginWithoutCustomerBindingReturnsActionableForbidden(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{err: portal.ErrCustomerBindingNotFound}})
	req := httptest.NewRequest(http.MethodPost, "/api/mini/login", strings.NewReader(`{"mode":"phone_verify","code":"wx","phone_code":"phone"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
