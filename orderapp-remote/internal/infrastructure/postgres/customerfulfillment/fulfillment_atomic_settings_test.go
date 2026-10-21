package customerfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	domain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
)

func TestAtomicFulfillmentSettingsAPIWithoutSaleAndRollback(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	seedManualFreightWeight(t, pool, schema, 911, 1000)
	r := NewRepository(pool, schema)
	tm, err := r.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Actor: "PR710", Template: app.ShippingFeeTemplate{Name: "原子配置测试", Active: true, Rules: []app.ShippingFeeRegionRule{{Province: "*", FirstWeightG: 1000, AdditionalWeightG: 1000, FirstFeeCents: 800}}}})
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: app.NewService(r)})
	call := func(method string, body any, want int) app.FulfillmentSettings {
		t.Helper()
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(method, "/api/customer-fulfillment/501/settings", bytes.NewReader(b))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status=%d want=%d: %s", method, rec.Code, want, rec.Body.String())
		}
		var out app.FulfillmentSettings
		if want == 200 {
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	request := func(revision int64) map[string]any {
		return map[string]any{"customer_id": 502, "expected_revision": revision, "modes": []map[string]any{
			{"mode": "stock", "enabled": true, "warehouse_code": "DEV-E2E-A1", "fee_configured": true, "fixed_fee_cents": 300, "shipping_policy": map[string]any{"template_id": tm.ID, "surcharge_cents": 125, "free_shipping": false, "expected_revision": 0}},
			{"mode": "on_demand", "enabled": true, "fee_configured": true, "fixed_fee_cents": 400, "shipping_policy": map[string]any{"template_id": 0, "surcharge_cents": 0, "free_shipping": true, "expected_revision": 0}},
		}}
	}
	snapshot := func() string {
		t.Helper()
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('settings',(SELECT to_jsonb(s) FROM %[1]s.customer_fulfillment_settings s WHERE customer_id=501),'policies',(SELECT jsonb_agg(to_jsonb(p) ORDER BY scene) FROM %[1]s.customer_shipping_policies p WHERE customer_id=501),'audit_count',(SELECT count(*) FROM %[1]s.audit_logs))::text`, schema)).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	initial := call(http.MethodPut, request(0), 200)
	if initial.CustomerID != 501 || initial.Revision != 1 || len(initial.Modes) != 2 {
		t.Fatalf("saved=%+v", initial)
	}
	loaded := call(http.MethodGet, nil, 200)
	encoded, _ := json.Marshal(loaded)
	var nested struct {
		Modes []struct {
			Policy struct {
				TemplateID int64 `json:"template_id"`
				Surcharge  int64 `json:"surcharge_cents"`
				Revision   int64 `json:"revision"`
				Free       bool  `json:"free_shipping"`
			} `json:"shipping_policy"`
		} `json:"modes"`
	}
	if err := json.Unmarshal(encoded, &nested); err != nil {
		t.Fatal(err)
	}
	if len(nested.Modes) != 2 || nested.Modes[0].Policy.TemplateID != tm.ID || nested.Modes[0].Policy.Surcharge != 125 || nested.Modes[0].Policy.Revision != 1 || !nested.Modes[1].Policy.Free {
		t.Fatalf("nested policies=%s", encoded)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_shipping_policies", "customer_id=501 AND scene='sale'", 0)
	assertMiniDirectShipCount(t, pool, schema, "customer_shipping_policies", "customer_id=502", 0)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "entity_type='shipping_policy'", 2)
	for _, mode := range []string{"stock", "on_demand"} {
		quote, err := r.QuoteFulfillmentFees(ctx, app.FulfillmentFeeQuoteCommand{CustomerID: 501, FulfillmentMode: mode, Province: "云南省", Items: []app.FulfillmentFeeItem{{ProductID: 911, Qty: 1}}})
		if err != nil || quote.Freight == nil {
			t.Fatalf("mode=%s quote=%+v err=%v", mode, quote, err)
		}
	}

	update := func() map[string]any {
		req := request(1)
		for _, mode := range req["modes"].([]map[string]any) {
			mode["shipping_policy"].(map[string]any)["expected_revision"] = int64(1)
		}
		return req
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"second policy conflict", func(req map[string]any) {
			m := req["modes"].([]map[string]any)
			m[0]["fixed_fee_cents"] = 900
			m[0]["shipping_policy"].(map[string]any)["surcharge_cents"] = 999
			m[1]["shipping_policy"].(map[string]any)["expected_revision"] = int64(0)
		}},
		{"foreign warehouse", func(req map[string]any) { req["modes"].([]map[string]any)[0]["warehouse_code"] = "DEV-E2E-B1" }},
		{"missing active template", func(req map[string]any) {
			req["modes"].([]map[string]any)[1]["shipping_policy"] = map[string]any{"template_id": 999999, "surcharge_cents": 0, "free_shipping": false, "expected_revision": 1}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := snapshot()
			req := update()
			tc.change(req)
			call(http.MethodPut, req, 400)
			if after := snapshot(); after != before {
				t.Fatalf("failed atomic save changed data\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_fee_templates SET active=false WHERE id=$1`, schema), tm.ID); err != nil {
		t.Fatal(err)
	}
	beforeStopped := snapshot()
	call(http.MethodPut, update(), 400)
	if snapshot() != beforeStopped {
		t.Fatal("inactive template left partial changes")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.shipping_fee_templates SET active=true WHERE id=$1`, schema), tm.ID); err != nil {
		t.Fatal(err)
	}

	// Fail the final settings audit after policy writes and audits have run.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_settings_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.entity_type='customer_fulfillment_settings' THEN RAISE EXCEPTION 'PR710 forced audit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_settings_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_settings_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	before := snapshot()
	call(http.MethodPut, update(), 400)
	if after := snapshot(); after != before {
		t.Fatal("audit failure left partial configuration or audit rows")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_settings_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}

	// Closing a mode only changes its switch, retaining settings and policy versions.
	disabled := call(http.MethodPut, map[string]any{"expected_revision": 1, "modes": []map[string]any{{"mode": "stock", "enabled": false, "warehouse_code": "invalid", "fixed_fee_cents": -1, "shipping_policy": map[string]any{"surcharge_cents": -1}}, {"mode": "on_demand", "enabled": false}}}, 200)
	if disabled.Revision != 2 || disabled.Modes[0].Enabled || disabled.Modes[0].FixedFeeCents != 300 || disabled.Modes[0].WarehouseCode != "DEV-E2E-A1" || disabled.Modes[1].FixedFeeCents != 400 {
		t.Fatalf("disabled lost saved values: %+v", disabled)
	}
	fr := freightpg.NewRepository(pool, schema)
	for _, mode := range []string{"stock", "on_demand"} {
		policy, err := fr.LoadPolicy(ctx, pool, 501, mode, false)
		if err != nil || policy.Revision != 1 {
			t.Fatalf("disabled policy written: %+v err=%v", policy, err)
		}
	}
	// Legacy clients omit the nested policy: surcharge and free shipping survive.
	disabled.Modes[0].Enabled = true
	disabled.Modes[1].Enabled = true
	legacyRaw, _ := json.Marshal(disabled)
	var legacy map[string]any
	json.Unmarshal(legacyRaw, &legacy)
	legacy["expected_revision"] = int64(2)
	for _, mode := range legacy["modes"].([]any) {
		delete(mode.(map[string]any), "shipping_policy")
	}
	call(http.MethodPut, legacy, 200)
	policy, err := fr.LoadPolicy(ctx, pool, 501, "stock", false)
	if err != nil || policy.SurchargeCents != 125 {
		t.Fatalf("legacy cleared surcharge: %+v %v", policy, err)
	}
	policy, err = fr.LoadPolicy(ctx, pool, 501, "on_demand", false)
	if err != nil || !policy.FreeShipping {
		t.Fatalf("legacy cleared free shipping: %+v %v", policy, err)
	}
	// Ordinary sale policy is maintained separately and cannot stale an open
	// dropshipping editor or change either dropshipping scenario's policy.
	beforeSale, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	beforeSaleJSON, _ := json.Marshal(beforeSale)
	if _, err = fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "sale", TemplateID: tm.ID, SurchargeCents: 500}, 0, "PR710 ordinary sale editor"); err != nil {
		t.Fatal(err)
	}
	afterSale, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	afterSaleJSON, _ := json.Marshal(afterSale)
	if !bytes.Equal(beforeSaleJSON, afterSaleJSON) {
		t.Fatalf("ordinary sale policy changed dropshipping settings\nbefore=%s\nafter=%s", beforeSaleJSON, afterSaleJSON)
	}
	// A separate policy editor invalidates the old full settings revision.
	oldSettings, err := r.LoadFulfillmentSettings(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	for i := range oldSettings.Modes {
		if oldSettings.Modes[i].ShippingPolicy != nil {
			revision := oldSettings.Modes[i].ShippingPolicy.Revision
			oldSettings.Modes[i].ShippingPolicy.ExpectedRevision = &revision
		}
	}
	_, err = fr.SavePolicy(ctx, domain.Policy{CustomerID: 501, Scene: "on_demand", FreeShipping: true}, policy.Revision, "PR710 external editor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Settings: oldSettings, ExpectedRevision: oldSettings.Revision, Actor: "PR710 stale editor"}); err == nil {
		t.Fatal("stale full editor overwrote separate policy edit")
	}
}

func TestAtomicFulfillmentSettingsBothDisabledNeedNoConfiguration(t *testing.T) {
	pool, schema := newCustomerFulfillmentTestDB(t)
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('PR710未开通代发') RETURNING id`, schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(pool, schema)
	saved, err := r.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "PR710", Settings: app.FulfillmentSettings{CustomerID: id, Modes: []app.FulfillmentModeSettings{{Mode: "stock"}, {Mode: "on_demand"}}}})
	if err != nil || saved.Revision != 1 {
		t.Fatalf("unconfigured disabled settings=%+v err=%v", saved, err)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_shipping_policies", fmt.Sprintf("customer_id=%d", id), 0)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "entity_type='customer_fulfillment_settings'", 1)
}
