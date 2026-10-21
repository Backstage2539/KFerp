package customerfulfillment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAtomicFulfillmentSettingsAPIPreservesPolicyAndPathCustomer(t *testing.T) {
	s := &fulfillmentSettingsAPIStub{}
	e := echo.New()
	registerFulfillmentSettingsRoutes(e, s)
	body := `{"customer_id":999,"expected_revision":4,"modes":[{"mode":"on_demand","enabled":true,"fee_configured":true,"shipping_policy":{"template_id":8,"surcharge_cents":125,"free_shipping":false,"expected_revision":3}}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/customer-fulfillment/147/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		CustomerID int64 `json:"customer_id"`
		Modes      []struct {
			Policy map[string]any `json:"shipping_policy"`
		} `json:"modes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.CustomerID != 147 || s.saved.ExpectedRevision != 4 || len(result.Modes) != 1 || result.Modes[0].Policy["template_id"] != float64(8) || result.Modes[0].Policy["surcharge_cents"] != float64(125) {
		t.Fatalf("lost nested policy or customer scope: %s", rec.Body.String())
	}
}
