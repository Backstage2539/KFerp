package freight

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	"testing"
)

type orderTestQuoter struct {
	seen app.FulfillmentFeeQuoteCommand
}

func (q *orderTestQuoter) QuoteFulfillmentFees(ctx context.Context, c app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error) {
	q.seen = c
	f, e := domain.QuoteOrder(domain.Policy{CustomerID: c.CustomerID, Scene: c.FulfillmentMode}, domain.Template{}, domain.Destination{}, 0, c.FreightOverride, c.EmployeeAdjustment)
	return app.FulfillmentFeeQuote{Freight: &f}, e
}
func TestOrderFreightAPIUsesExplicitCentsAndDirectory(t *testing.T) {
	e := echo.New()
	q := &orderTestQuoter{}
	RegisterRoutes(e, nil, "unused", q)
	call := func(path, body string) []byte {
		t.Helper()
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatal(path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	raw := call("/api/order/freight/preview", `{"customer_id":501,"freight_protocol":"order_freight_v2","freight_override":{"manual_amount_cents":2000},"items":[{"product_id":1,"qty":2}]}`)
	var f domain.Snapshot
	if er := json.Unmarshal(raw, &f); er != nil || f.CurrentReceivableCents != 2000 || q.seen.CustomerID != 501 || !q.seen.EmployeeAdjustment {
		t.Fatal(string(raw), q.seen, er)
	}
	raw = call("/api/order/shipping-destination/resolve", `{"address":"云南省普洱市孟连县测试路"}`)
	var result domain.AddressResolution
	json.Unmarshal(raw, &result)
	if result.Status != "resolved" || result.Destination.DistrictCode != "530827" {
		t.Fatal(string(raw))
	}
}
