package customerportal

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	cfapp "orderapp/internal/application/customerfulfillment"
	app "orderapp/internal/application/customerportal"
	"testing"
)

func TestMiniInventoryAssetsReturnsAuthoritativeScopeEvenWhenEmpty(t *testing.T) {
	e := echo.New()
	fulfillment := &miniCustomerFulfillmentFake{assetRows: []cfapp.CustomerAssetInventory{}}
	RegisterRoutes(e, Dependencies{CustomerPortal: fakeService{me: app.CurrentContext{MiniUserID: 17, CurrentCustomerID: 9, Capabilities: []app.Capability{{Code: app.CapabilityInventoryCustody, Enabled: true}}}}, CustomerFulfillment: fulfillment})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/customer-inventory/assets?customer_id=999", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"inventory_types", "warehouses"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("inventory response lacks authoritative %s scope: %s", key, rec.Body.String())
		}
	}
	if fulfillment.scopeCustomerID != 9 {
		t.Fatalf("scope trusted client customer_id: %d", fulfillment.scopeCustomerID)
	}
	var rows []cfapp.CustomerAssetInventory
	if err := json.Unmarshal(body["rows"], &rows); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("empty authorized inventory response=%s err=%v", rec.Body.String(), err)
	}
}
