package customerportal

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	fulfillment "orderapp/internal/application/customerfulfillment"
	portal "orderapp/internal/application/customerportal"
	"testing"
)

type issuedBillFake struct {
	*miniCustomerFulfillmentFake
	query      fulfillment.BillingQuery
	customerID int64
	id         int64
}

func (f *issuedBillFake) ListIssuedBills(_ context.Context, q fulfillment.BillingQuery) (fulfillment.BillingBillsPage, error) {
	f.query = q
	return fulfillment.BillingBillsPage{Summary: fulfillment.IssuedBillSummary{UnpaidCount: 2, DueAmountCents: 236610}}, nil
}
func (f *issuedBillFake) GetIssuedBill(_ context.Context, customerID, id int64) (fulfillment.BillingBill, error) {
	f.customerID = customerID
	f.id = id
	return fulfillment.BillingBill{SettlementResult: fulfillment.SettlementResult{BatchID: id}}, nil
}
func (f *issuedBillFake) GetBillingPDFSettings(_ context.Context, _, _ int64) (fulfillment.BillingPDFSettings, error) {
	return fulfillment.BillingPDFSettings{}, nil
}
func TestMiniIssuedBillScopesToSessionAndValidatesDates(t *testing.T) {
	fake := &issuedBillFake{miniCustomerFulfillmentFake: &miniCustomerFulfillmentFake{}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: portal.CurrentContext{MiniUserID: 17, CurrentCustomerID: 74, Capabilities: []portal.Capability{{Code: portal.CapabilitySettlement, Enabled: true}}}}, CustomerFulfillment: fake})
	request := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer token")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	rec := request("/api/mini/customer-issued-bills?customer_id=999&date_from=2026-11-08&date_to=2026-11-09&status=unpaid&page=2&limit=5")
	if rec.Code != 200 || fake.query.CustomerID != 74 || fake.query.DateFrom != "2026-11-08" || fake.query.DateTo != "2026-11-09" || fake.query.Page != 2 || fake.query.Status != "unpaid" {
		t.Fatalf("response=%d %s query=%+v", rec.Code, rec.Body, fake.query)
	}
	if rec = request("/api/mini/customer-issued-bills/4"); rec.Code != 200 || fake.customerID != 74 || fake.id != 4 {
		t.Fatalf("detail=%d customer=%d id=%d", rec.Code, fake.customerID, fake.id)
	}
	if rec = request("/api/mini/customer-issued-bills?date_from=2026-11-09&date_to=2026-11-08"); rec.Code != 400 {
		t.Fatalf("expected invalid range, got %d", rec.Code)
	}
}
