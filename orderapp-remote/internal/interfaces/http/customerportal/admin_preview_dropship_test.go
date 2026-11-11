package customerportal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	portal "orderapp/internal/application/customerportal"
)

type previewDropshipService struct {
	fakeService
	RecipientAddressService
	addressToken string
}

func (s *previewDropshipService) ListRecipientAddresses(_ context.Context, token string) ([]portal.CustomerRecipientAddress, error) {
	s.addressToken = token
	return []portal.CustomerRecipientAddress{{CustomerID: 74}}, nil
}

func TestPreviewDropshipReadScopeAndWriteDenial(t *testing.T) {
	s := &previewDropshipService{fakeService: fakeService{me: portal.CurrentContext{AccountType: "preview", CurrentCustomerID: 74, Capabilities: []portal.Capability{{Code: portal.CapabilityDirectShip, Enabled: true}}}}}
	fulfillment := &miniCustomerFulfillmentFake{}
	e := echo.New()
	e.Use(adminPreviewReadOnlyMiddleware(s))
	registerMiniCustomerFulfillmentAPI(e, s, fulfillment, nil, "")
	registerRecipientAddressAPI(e, s, nil)
	request := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer admin-preview.scoped")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	if rec := request(http.MethodGet, "/api/mini/direct-ship/catalog?customer_id=999&fulfillment_mode=stock"); rec.Code != http.StatusOK || fulfillment.catalogQuery.CustomerID != 74 || fulfillment.catalogQuery.FulfillmentMode != "stock" {
		t.Errorf("catalog: status=%d body=%s query=%+v", rec.Code, rec.Body, fulfillment.catalogQuery)
	}
	if rec := request(http.MethodGet, "/api/mini/recipient-addresses?customer_id=999"); rec.Code != http.StatusOK || s.addressToken != "admin-preview.scoped" {
		t.Errorf("addresses: status=%d body=%s token=%q", rec.Code, rec.Body, s.addressToken)
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/mini/direct-ship/preview"}, {http.MethodPost, "/api/mini/direct-ship/requests"},
		{http.MethodPost, "/api/mini/recipient-addresses"}, {http.MethodPut, "/api/mini/recipient-addresses/1"}, {http.MethodDelete, "/api/mini/recipient-addresses/1"},
	} {
		if rec := request(tc.method, tc.path); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: got %d", tc.method, tc.path, rec.Code)
		}
	}
	s.me.Capabilities = nil
	if rec := request(http.MethodGet, "/api/mini/direct-ship/catalog"); rec.Code != http.StatusForbidden {
		t.Errorf("disabled capability: got %d", rec.Code)
	}
	s.me.AccountType = "customer"
	if rec := request(http.MethodGet, "/api/mini/direct-ship/catalog"); rec.Code != http.StatusUnauthorized {
		t.Errorf("preview token with non-preview identity: got %d", rec.Code)
	}
	s.err = portal.ErrAdminPreviewDenied
	for _, path := range []string{"/api/mini/direct-ship/catalog", "/api/mini/recipient-addresses"} {
		if rec := request(http.MethodGet, path); rec.Code != http.StatusUnauthorized {
			t.Errorf("revoked preview %s: got %d", path, rec.Code)
		}
	}
}
