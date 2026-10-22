package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/customerfulfillment"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
	"strings"
	"testing"
	"time"
)

func TestSharedFulfillmentDefaultsPreserveLegacyAndInvalidateNewQuotes(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	seedManualFreightWeight(t, pool, schema, 911, 1000)
	r := NewRepository(pool, schema)
	legacy, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.SaveFulfillmentDefaults(ctx, app.SaveFulfillmentDefaultsCommand{Actor: "PR718", Defaults: app.FulfillmentDefaults{Modes: []app.FulfillmentModeSettings{{Mode: "stock", FeeConfigured: true, FixedFeeCents: 300, PerPackageFeeCents: 20, ShippingFree: true}, {Mode: "on_demand", FeeConfigured: true, ShippingFree: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range legacy.Modes {
		u := unchanged.Modes[i]
		if m.Enabled != u.Enabled || m.FeeConfigured != u.FeeConfigured || m.FixedFeeCents != u.FixedFeeCents || m.ShippingFree != u.ShippingFree {
			t.Fatalf("legacy changed: %+v -> %+v", m, u)
		}
	}
	missing, err := r.LoadFulfillmentSettings(ctx, 502)
	if err != nil || missing.Modes[0].Enabled || missing.Modes[0].FeeConfigured {
		t.Fatalf("missing config silently enabled: %+v %v", missing, err)
	}
	m := unchanged.Modes[0]
	m.Enabled = true
	m.WarehouseCode = "DEV-E2E-A1"
	m.FeeConfigured = true
	m.InheritedFields = app.FulfillmentDefaultFields()
	rev := m.ShippingPolicy.Revision
	m.ShippingPolicy.ExpectedRevision = &rev
	saved, err := r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "PR718", ExpectedRevision: unchanged.Revision, Settings: app.FulfillmentSettings{CustomerID: 501, DefaultsRevision: d.Revision, Modes: []app.FulfillmentModeSettings{m}}})
	if err != nil {
		t.Fatal(err)
	}
	quote := func() app.FulfillmentFeeQuote {
		t.Helper()
		q, e := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: "stock", Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 911, Qty: 1}}})
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	freightRepo := freightpg.NewRepository(pool, schema)
	policyBefore, err := freightRepo.LoadPolicy(ctx, pool, 501, "stock", false)
	if err != nil {
		t.Fatal(err)
	}
	conflicting := policyBefore
	conflicting.SurchargeCents = 999
	if _, err = freightRepo.SavePolicy(ctx, conflicting, policyBefore.Revision, "PR718-legacy"); err == nil || !strings.Contains(err.Error(), "履约默认配置") {
		t.Fatalf("legacy editor silently overwrote inherited policy: %v", err)
	}
	policyAfter, err := freightRepo.LoadPolicy(ctx, pool, 501, "stock", false)
	if err != nil || policyAfter.Revision != policyBefore.Revision || policyAfter.SurchargeCents != policyBefore.SurchargeCents {
		t.Fatalf("rejected legacy edit persisted: %+v %v", policyAfter, err)
	}
	q1 := quote()
	if q1.ServiceFeeCents != 320 || !q1.Freight.Policy.FreeShipping {
		t.Fatalf("quote=%+v", q1)
	}
	d.Modes[0].FixedFeeCents = 600
	d, err = r.SaveFulfillmentDefaults(ctx, app.SaveFulfillmentDefaultsCommand{Actor: "PR718", ExpectedRevision: d.Revision, Defaults: d})
	if err != nil {
		t.Fatal(err)
	}
	policies, err := freightRepo.Policies(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range policies {
		if policy.Scene == "stock" && policy.FulfillmentDefaultsRevision != d.Revision {
			t.Fatalf("policy reader returned stale raw values: %+v", policy)
		}
	}
	q2 := quote()
	if q2.ServiceFeeCents != 620 || q1.FeeQuoteToken == q2.FeeQuoteToken {
		t.Fatalf("stale quote: %+v", q2)
	}
	if _, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "PR718", ExpectedRevision: saved.Revision, Settings: saved}); err == nil {
		t.Fatal("stale defaults editor accepted")
	}
	if _, err = r.SaveFulfillmentDefaults(ctx, app.SaveFulfillmentDefaultsCommand{Actor: "PR718", ExpectedRevision: d.Revision - 1, Defaults: d}); err == nil {
		t.Fatal("stale defaults accepted")
	}
	live, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	custom := live.Modes[0]
	custom.InheritedFields = []string{"per_package_fee_cents"}
	custom.FixedFeeCents = 0
	custom.ShippingFree = true
	rev = custom.ShippingPolicy.Revision
	custom.ShippingPolicy.ExpectedRevision = &rev
	saved, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "PR718", ExpectedRevision: live.Revision, Settings: app.FulfillmentSettings{CustomerID: 501, DefaultsRevision: d.Revision, Modes: []app.FulfillmentModeSettings{custom}}})
	if err != nil {
		t.Fatal(err)
	}
	if quote().ServiceFeeCents != 20 {
		t.Fatal("explicit zero override lost")
	}
	p, err := freightpg.NewRepository(pool, schema).ResolvePolicy(ctx, pool, 501, "stock", false)
	if err != nil || !p.FreeShipping {
		t.Fatalf("shared freight resolution=%+v %v", p, err)
	}
	var audits int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='fulfillment_defaults'`, schema)).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audits=%d %v", audits, err)
	}
}

func TestFulfillmentDefaultsAPIConcurrencyAndFrozenHistory(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := NewRepository(pool, schema)
	customerID, orderID, original := seedConcurrentFulfillmentFeeOrder(t, pool, schema, r, "PR718-FROZEN")
	e := echo.New()
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: app.NewService(r)})
	request := map[string]any{"expected_revision": 0, "modes": []map[string]any{{"mode": "stock", "fee_configured": true, "fixed_fee_cents": 0, "per_package_fee_cents": 0, "shipping_free": false}, {"mode": "on_demand", "fee_configured": true, "fixed_fee_cents": 0, "per_package_fee_cents": 0, "shipping_free": false}}}
	raw, _ := json.Marshal(request)
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPut, "/api/settings/logistics/fulfillment-defaults", bytes.NewReader(raw)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec.Code
		}()
	}
	accepted, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		switch <-results {
		case 200:
			accepted++
		case 400:
			conflicts++
		default:
			t.Fatal("unexpected API response")
		}
	}
	if accepted != 1 || conflicts != 1 {
		t.Fatalf("accepted=%d conflicts=%d", accepted, conflicts)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settings/logistics/fulfillment-defaults", nil))
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var defaults app.FulfillmentDefaults
	if err := json.Unmarshal(rec.Body.Bytes(), &defaults); err != nil || defaults.Revision != 1 {
		t.Fatalf("defaults=%+v err=%v", defaults, err)
	}
	frozen, err := r.GetFulfillmentOrderFees(ctx, customerID, orderID)
	if err != nil || frozen.FeeQuoteToken != original.FeeQuoteToken || frozen.ServiceFeeCents != original.ServiceFeeCents {
		t.Fatalf("history changed: %+v %v", frozen, err)
	}
}
