package customerportal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	portal "orderapp/internal/application/customerportal"
)

func TestPreviewProductCatalogIsCustomerScopedAndReadOnly(t *testing.T) {
	s := &previewDropshipService{fakeService: fakeService{me: portal.CurrentContext{
		AccountType: "preview", CurrentCustomerID: 74,
		Capabilities: []portal.Capability{{Code: portal.CapabilityProductOrder, Enabled: true}},
	}}}
	fulfillment := &miniCustomerFulfillmentFake{}
	e := echo.New()
	e.Use(adminPreviewReadOnlyMiddleware(s))
	registerMiniCustomerFulfillmentAPI(e, s, fulfillment, nil, "")
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer admin-preview.scoped")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(http.MethodGet, "/api/mini/product-orders/catalog?customer_id=999"); rec.Code != http.StatusOK || fulfillment.catalogQuery.CustomerID != 74 {
		t.Fatalf("catalog: status=%d body=%s query=%+v", rec.Code, rec.Body, fulfillment.catalogQuery)
	}
	s.me.CurrentCustomerID = 88
	if rec := request(http.MethodGet, "/api/mini/product-orders/catalog?customer_id=74"); rec.Code != http.StatusOK || fulfillment.catalogQuery.CustomerID != 88 {
		t.Fatalf("switched customer catalog: status=%d body=%s query=%+v", rec.Code, rec.Body, fulfillment.catalogQuery)
	}
	for _, path := range []string{"/api/mini/product-orders/preview", "/api/mini/product-orders"} {
		before := fulfillment.calls
		if rec := request(http.MethodPost, path); rec.Code != http.StatusForbidden || fulfillment.calls != before {
			t.Errorf("write %s: status=%d calls=%d before=%d", path, rec.Code, fulfillment.calls, before)
		}
	}
	s.me.Capabilities = nil
	before := fulfillment.calls
	if rec := request(http.MethodGet, "/api/mini/product-orders/catalog"); rec.Code != http.StatusForbidden || fulfillment.calls != before {
		t.Errorf("disabled capability: status=%d body=%s", rec.Code, rec.Body)
	}
	s.err = portal.ErrAdminPreviewDenied
	if rec := request(http.MethodGet, "/api/mini/product-orders/catalog"); rec.Code != http.StatusUnauthorized || fulfillment.calls != before {
		t.Errorf("revoked preview: status=%d body=%s", rec.Code, rec.Body)
	}
}
