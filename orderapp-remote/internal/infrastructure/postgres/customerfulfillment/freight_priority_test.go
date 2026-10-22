package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	freighthttp "orderapp/internal/interfaces/http/freight"
)

func TestFreightTemplatePriorityAPI(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	seedManualFreightWeight(t, pool, schema, 911, 1000)
	r := NewRepository(pool, schema)
	fr := freightpg.NewRepository(pool, schema)
	e := echo.New()
	freighthttp.RegisterRoutes(e, pool, schema, app.NewService(r))
	call := func(method, path string, body any, want int) domain.Policy {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		var out domain.Policy
		if want == 200 {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	endpoint := "/api/settings/logistics/default-shipping-policy"
	call("GET", endpoint, nil, 200)
	templates := []domain.Template{}
	for i := int64(1); i <= 3; i++ {
		tm, err := r.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Actor: "test", Template: domain.Template{Name: fmt.Sprintf("priority-%d", i), Active: true, Rules: []domain.LegacyRule{{Province: "*", FirstWeightG: 2000, FirstFeeCents: i * 100, AdditionalWeightG: 1000, AdditionalFeeCents: 100}}}})
		if err != nil {
			t.Fatal(err)
		}
		templates = append(templates, tm)
	}
	global := call("PUT", endpoint, map[string]any{"template_id": templates[0].ID, "surcharge_cents": 50, "expected_revision": 0}, 200)
	call("PUT", endpoint, map[string]any{"template_id": templates[1].ID, "expected_revision": 0}, 400)
	check := func(scene string, template int64, cents int64) domain.Snapshot {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		q, err := fr.QuoteTx(ctx, tx, 501, scene, domain.Destination{Province: "云南"}, 1000)
		if err != nil {
			t.Fatal(err)
		}
		if q.TemplateID != template || q.EstimatedTotalCents != cents || q.Policy.CustomerID != 501 || q.Policy.Scene != scene {
			t.Fatalf("scene %s: %+v", scene, q)
		}
		if scene == "sale" {
			ordinary, err := fr.QuoteOrderTx(ctx, tx, 501, 0, "", domain.Destination{Province: "云南"}, []freightpg.WeightItem{{ProductID: 911, Qty: 1}}, nil)
			if err != nil || ordinary.TemplateID != template {
				t.Fatalf("ordinary %+v %v", ordinary, err)
			}
		}
		return q
	}
	for _, scene := range []string{"sale", "stock", "on_demand"} {
		check(scene, templates[0].ID, 150)
	}
	customer := call("PUT", "/api/customer-fulfillment/501/shipping-policies/sale", map[string]any{"template_id": templates[1].ID, "expected_revision": 0}, 200)
	for _, scene := range []string{"sale", "stock", "on_demand"} {
		check(scene, templates[1].ID, 200)
	}
	stock := call("PUT", "/api/customer-fulfillment/501/shipping-policies/stock", map[string]any{"template_id": templates[2].ID, "expected_revision": 0}, 200)
	check("stock", templates[2].ID, 300)
	check("sale", templates[1].ID, 200)
	check("on_demand", templates[1].ID, 200)
	// Saving an empty override restores inheritance without deleting its revision.
	stock = call("PUT", "/api/customer-fulfillment/501/shipping-policies/stock", map[string]any{"expected_revision": stock.Revision}, 200)
	check("stock", templates[1].ID, 200)
	stock = call("PUT", "/api/customer-fulfillment/501/shipping-policies/stock", map[string]any{"free_shipping": true, "expected_revision": stock.Revision}, 200)
	check("stock", 0, 0)
	call("PUT", "/api/customer-fulfillment/501/shipping-policies/sale", map[string]any{"expected_revision": customer.Revision}, 200)
	check("sale", templates[0].ID, 150)
	check("on_demand", templates[0].ID, 150)
	check("stock", 0, 0)
	// Customer editors read their stored configuration, never a copied fallback.
	policies, err := fr.Policies(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if policies[0].TemplateID != 0 || policies[0].Revision != 2 {
		t.Fatal(policies)
	}

	// An enabled mode persists inheritance atomically with its service fee.
	zero := int64(0)
	_, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "test", Settings: app.FulfillmentSettings{CustomerID: 501, Modes: []app.FulfillmentModeSettings{{Mode: "on_demand", Enabled: true, FeeConfigured: true, FixedFeeCents: 300, ShippingPolicy: &app.FulfillmentShippingPolicy{ExpectedRevision: &zero}}}}})
	if err != nil {
		t.Fatal(err)
	}
	fees, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "on_demand", Province: "云南", Items: []app.FulfillmentFeeItem{{ProductID: 911, Qty: 1}}})
	if err != nil || fees.Freight == nil || fees.Freight.TemplateID != templates[0].ID || fees.ServiceFeeCents != 300 {
		t.Fatalf("inherited dropship %+v %v", fees, err)
	}
	// Other customers inherit only the system default.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q, err := fr.QuoteTx(ctx, tx, 502, "sale", domain.Destination{Province: "云南"}, 1000)
	tx.Rollback(ctx)
	if err != nil || q.TemplateID != templates[0].ID || q.Policy.CustomerID != 502 {
		t.Fatalf("other customer %+v %v", q, err)
	}
	call("PUT", endpoint, map[string]any{"template_id": 999999, "expected_revision": global.Revision}, 400)
	if call("GET", endpoint, nil, 200).Revision != global.Revision {
		t.Fatal("invalid save changed default")
	}
	var audits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='system_shipping_policy'`, schema)).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit %d %v", audits, err)
	}
}
