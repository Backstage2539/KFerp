package customerfulfillment

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	"strings"
	"testing"
)

type fulfillmentSettingsAPIStub struct {
	Service
	app.FulfillmentSettingsRepository
	saved    app.SaveFulfillmentSettingsCommand
	quote    app.FulfillmentFeeQuoteCommand
	template app.SaveShippingFeeTemplateCommand
}

func (s *fulfillmentSettingsAPIStub) ListShippingFeeTemplates(context.Context) ([]app.ShippingFeeTemplate, error) {
	return []app.ShippingFeeTemplate{}, nil
}

func (s *fulfillmentSettingsAPIStub) LoadFulfillmentSettings(_ context.Context, id int64) (app.FulfillmentSettings, error) {
	return app.FulfillmentSettings{CustomerID: id, Revision: 2, Modes: []app.FulfillmentModeSettings{}}, nil
}
func (s *fulfillmentSettingsAPIStub) SaveFulfillmentSettings(_ context.Context, c app.SaveFulfillmentSettingsCommand) (app.FulfillmentSettings, error) {
	s.saved = c
	return c.Settings, nil
}
func (s *fulfillmentSettingsAPIStub) QuoteFulfillmentFees(_ context.Context, c app.FulfillmentFeeQuoteCommand) (app.FulfillmentFeeQuote, error) {
	s.quote = c
	return app.FulfillmentFeeQuote{CustomerID: c.CustomerID, ServiceFeeCents: 300, ShippingFeeCents: 800, FeeQuoteToken: "quote"}, nil
}
func (s *fulfillmentSettingsAPIStub) SaveShippingFeeTemplate(_ context.Context, c app.SaveShippingFeeTemplateCommand) (app.ShippingFeeTemplate, error) {
	s.template = c
	return c.Template, nil
}

func TestFulfillmentSettingsAPIScopesCustomerAndReturnsFeeBreakdown(t *testing.T) {
	s := &fulfillmentSettingsAPIStub{}
	e := echo.New()
	registerFulfillmentSettingsRoutes(e, s)
	for _, tc := range []struct{ method, path, body string }{{http.MethodGet, "/api/customer-fulfillment/147/settings", ""}, {http.MethodPut, "/api/customer-fulfillment/147/settings", `{"customer_id":999,"expected_revision":2,"modes":[]}`}, {http.MethodPost, "/api/customer-fulfillment/147/fees/preview", `{"customer_id":999,"fulfillment_mode":"stock","province":"云南省","items":[{"product_id":7,"qty":12}]}`}, {http.MethodPut, "/api/settings/logistics/shipping-templates/8", `{"id":999,"name":"云南","expected_revision":3}`}} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s status=%d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if s.saved.Settings.CustomerID != 147 || s.saved.ExpectedRevision != 2 || s.quote.CustomerID != 147 || s.template.Template.ID != 8 {
		t.Fatalf("scope=%+v %+v %+v", s.saved, s.quote, s.template)
	}
}
