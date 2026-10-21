package customerportal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	fulfillmentapp "orderapp/internal/application/customerfulfillment"
	app "orderapp/internal/application/customerportal"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

type requiredProductOrderAPI struct {
	portalhttp.MiniCustomerFulfillment
	portalhttp.MiniProductOrderFulfillment
	calls int
}

func (f *requiredProductOrderAPI) MiniProductOrderCatalog(_ context.Context, q fulfillmentapp.MiniDirectShipCatalogQuery) (fulfillmentapp.MiniDirectShipCatalog, error) {
	f.calls++
	return fulfillmentapp.MiniDirectShipCatalog{CurrentCustomerID: q.CustomerID, ProductFamilies: []map[string]any{}}, nil
}

func TestFulfillmentModesRequireProductOrderAcrossLegacyTemplateAdminAndMiniAPI(t *testing.T) {
	pool, schema := newCustomerPortalTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresauthz.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.customer_fulfillment_settings(customer_id BIGINT PRIMARY KEY,revision BIGINT NOT NULL DEFAULT 1,modes_json JSONB NOT NULL DEFAULT '[]');CREATE TABLE %[1]s.customer_erp_user_bindings(id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL,employee_id BIGINT NOT NULL,role TEXT NOT NULL DEFAULT 'customer',status TEXT NOT NULL DEFAULT 'active',updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT '');INSERT INTO %[1]s.customers(id,name) VALUES(501,'PR711旧客户'),(502,'PR711模板客户');INSERT INTO %[1]s.customer_portal_profiles(customer_id,enabled,capability_template_key) VALUES(501,true,''),(502,true,'processing_fulfillment');INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled,config_json) VALUES(501,'product_order',false,'{"keep":"legacy","required_by_fulfillment":true}'),(502,'product_order',false,'{}')`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{501, 502} {
		var userID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.mini_users(openid) VALUES($1) RETURNING id`, schema), fmt.Sprintf("PR711-%d", id)).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_portal_user_bindings(mini_user_id,customer_id,role,status) VALUES($1,$2,'owner','approved')`, schema), userID, id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.mini_sessions(token,mini_user_id,current_customer_id,expire_at) VALUES($1,$2,$3,now()+INTERVAL '1 day')`, schema), fmt.Sprintf("PR711-token-%d", id), userID, id); err != nil {
			t.Fatal(err)
		}
	}
	fulfillment := &requiredProductOrderAPI{}
	e := echo.New()
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: app.NewService(repo, nil), CustomerFulfillment: fulfillment})
	for _, id := range []int64{501, 502} {
		t.Run(fmt.Sprintf("customer-%d", id), func(t *testing.T) {
			check := func(want, required bool) {
				t.Helper()
				caps, err := repo.CapabilitiesForCustomer(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				current, err := repo.CurrentContextByToken(ctx, fmt.Sprintf("PR711-token-%d", id))
				if err != nil {
					t.Fatal(err)
				}
				if current.HasCapability(app.CapabilityProductOrder) != want {
					t.Fatalf("mini context product ordering=%v want=%v", current.HasCapability(app.CapabilityProductOrder), want)
				}
				for _, c := range caps {
					if c.Code == app.CapabilityProductOrder {
						if c.Enabled != want || (c.Config["required_by_fulfillment"] == true) != required {
							t.Fatalf("effective capability=%+v want enabled=%v required=%v", c, want, required)
						}
					}
				}
				detail, err := repo.PortalAdminDetail(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, c := range detail.Capabilities {
					if c.Code == app.CapabilityProductOrder {
						found = true
						if c.Enabled != want || (c.Config["required_by_fulfillment"] == true) != required {
							t.Fatalf("admin capability=%+v want enabled=%v required=%v", c, want, required)
						}
					}
				}
				if !found {
					t.Fatal("admin omitted product order capability")
				}
				// Exercise the application and HTTP layers too: the live template
				// projection must not replace this customer's derived capability.
				adminReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/customer-portal/admin/customers/%d", id), nil)
				adminRec := httptest.NewRecorder()
				e.ServeHTTP(adminRec, adminReq)
				if adminRec.Code != http.StatusOK {
					t.Fatalf("admin detail API status=%d body=%s", adminRec.Code, adminRec.Body.String())
				}
				var adminDetail app.PortalAdminDetail
				if err := json.Unmarshal(adminRec.Body.Bytes(), &adminDetail); err != nil {
					t.Fatal(err)
				}
				meReq := httptest.NewRequest(http.MethodGet, "/api/mini/me", nil)
				meReq.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer PR711-token-%d", id))
				meRec := httptest.NewRecorder()
				e.ServeHTTP(meRec, meReq)
				if meRec.Code != http.StatusOK {
					t.Fatalf("mini me API status=%d body=%s", meRec.Code, meRec.Body.String())
				}
				var me app.CurrentContext
				if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
					t.Fatal(err)
				}
				for _, c := range adminDetail.Capabilities {
					if c.Code == app.CapabilityProductOrder && (c.Enabled != want || (c.Config["required_by_fulfillment"] == true) != required || c.Enabled != me.HasCapability(c.Code)) {
						t.Fatalf("admin HTTP capability contradicts Mini Me: admin=%+v mini=%+v want=%v required=%v", c, me.Capabilities, want, required)
					}
				}
				req := httptest.NewRequest(http.MethodGet, "/api/mini/product-orders/catalog?customer_id=999", nil)
				req.Header.Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer PR711-token-%d", id))
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				wantStatus := 403
				if want {
					wantStatus = 200
				}
				if rec.Code != wantStatus {
					t.Fatalf("product-order API status=%d want=%d body=%s", rec.Code, wantStatus, rec.Body.String())
				}
				if want {
					var catalog fulfillmentapp.MiniDirectShipCatalog
					if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
						t.Fatal(err)
					}
					if catalog.CurrentCustomerID != id {
						t.Fatal("product ordering escaped login customer scope")
					}
				}
			}
			check(false, false)
			for _, modes := range []string{`[{"mode":"stock","enabled":true}]`, `[{"mode":"on_demand","enabled":true}]`, `[{"mode":"stock","enabled":true},{"mode":"on_demand","enabled":true}]`} {
				if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fulfillment_settings(customer_id,modes_json) VALUES($1,$2) ON CONFLICT(customer_id) DO UPDATE SET modes_json=excluded.modes_json,revision=customer_fulfillment_settings.revision+1`, schema), id, modes); err != nil {
					t.Fatal(err)
				}
				check(true, true)
			}
			template := ""
			if id == 502 {
				template = "processing_fulfillment"
			}
			detail, err := repo.UpdatePortalVisibility(ctx, app.UpdatePortalVisibilityCommand{CustomerID: id, Enabled: true, CapabilityTemplateKey: template, Capabilities: []app.CapabilityOption{{Code: app.CapabilityProductOrder, Enabled: false}}, UpdatedBy: "PR711"})
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range detail.Capabilities {
				if c.Code == app.CapabilityProductOrder && (!c.Enabled || c.Config["required_by_fulfillment"] != true) {
					t.Fatalf("close request bypassed dependency: %+v", c)
				}
			}
			visibilityReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/customer-portal/admin/customers/%d/visibility", id), strings.NewReader(fmt.Sprintf(`{"enabled":true,"capability_template_key":%q,"capabilities":[{"code":"product_order","enabled":false}]}`, template)))
			visibilityReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			visibilityRec := httptest.NewRecorder()
			e.ServeHTTP(visibilityRec, visibilityReq)
			if visibilityRec.Code != http.StatusOK {
				t.Fatalf("visibility API status=%d body=%s", visibilityRec.Code, visibilityRec.Body.String())
			}
			var savedDetail app.PortalAdminDetail
			if err := json.Unmarshal(visibilityRec.Body.Bytes(), &savedDetail); err != nil {
				t.Fatal(err)
			}
			for _, c := range savedDetail.Capabilities {
				if c.Code == app.CapabilityProductOrder && (!c.Enabled || c.Config["required_by_fulfillment"] != true) {
					t.Fatalf("visibility HTTP save lost effective dependency: %+v", c)
				}
			}
			check(true, true)
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json='[{"mode":"stock","enabled":false},{"mode":"on_demand","enabled":false}]' WHERE customer_id=$1`, schema), id); err != nil {
				t.Fatal(err)
			}
			check(false, false)
		})
	}
	// Enabling a mode on one customer cannot change the shared template for others.
	template, ok, err := repo.capabilityTemplateForKeyTx(ctx, pool, "processing_fulfillment")
	if err != nil || !ok || template.HasCapability(app.CapabilityProductOrder) {
		t.Fatalf("dependency rewrote shared template: %+v %v", template, err)
	}
}
