package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	portalhttp "orderapp/internal/interfaces/http/customerportal"
)

func TestMiniRetainedPurchaseAPIAuthorizesWarehouseWithoutShippingRecipient(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	content := `{"price_rows":[{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"source_price_record_id":704454,"min_qty":1,"final_unit_price":30,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","effective_sales_spec":{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"net_content_qty":454,"net_content_unit":"g","sales_unit":"袋","inventory_unit":"袋"}}]}`
	var publication int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,content_json)VALUES('commercial','RETAINED-API','published','official','','factory_supply',$1::jsonb)RETURNING id`, schema), content).Scan(&publication); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES(501,'product_order','commercial',$1)`, schema), publication); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'product_order',false)`, schema)); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(pool, schema)
	svc := app.NewService(repository)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	e := echo.New()
	portalhttp.RegisterRoutes(e, portalhttp.Dependencies{CustomerPortal: &priceCatalogPrincipal{customerID: 501, productOrder: true}, CustomerFulfillment: svc})
	post := func(t *testing.T, path string, body map[string]any, want int) []byte {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set(echo.HeaderAuthorization, "Bearer retained-api")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("POST %s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	command := func() map[string]any {
		return map[string]any{
			"idempotency_key": "RETAINED-API-NO-ADDRESS", "order_date": "2026-10-22",
			"delivery_mode": "customer_stock", "delivery_warehouse_code": "DEV-E2E-A1",
			"recipient_name": "", "recipient_phone": "", "detail_address": "",
			"customer_id": 502, "usage_code": "direct_ship",
			"items": []map[string]any{{"product_id": 910, "bom_spec_id": 7031, "bom_variant_id": 7032, "spec_g": 0, "qty": 1, "sales_unit": "袋"}},
		}
	}
	var preview app.MiniDirectShipPreview
	if err := json.Unmarshal(post(t, "/api/mini/product-orders/preview", command(), http.StatusOK), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.CanSubmit || preview.PriceQuoteToken == "" || preview.Fees != nil || preview.TotalAmount != 30 {
		t.Fatalf("retained purchase preview=%+v", preview)
	}
	for _, code := range []string{"", "DEV-E2E-A2", "DEV-E2E-B1", "finished_goods"} {
		t.Run("unauthorized-"+code, func(t *testing.T) {
			body := command()
			body["delivery_warehouse_code"] = code
			body["price_quote_token"] = preview.PriceQuoteToken
			for _, path := range []string{"/api/mini/product-orders/preview", "/api/mini/product-orders"} {
				post(t, path, body, http.StatusBadRequest)
			}
		})
	}
	t.Run("disabled-stock-mode", func(t *testing.T) {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json='[{"mode":"stock","enabled":false,"warehouse_code":"DEV-E2E-A1"}]' WHERE customer_id=501`, schema)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json='[{"mode":"stock","enabled":true,"warehouse_code":"DEV-E2E-A1"}]' WHERE customer_id=501`, schema)); err != nil {
				t.Fatal(err)
			}
		})
		body := command()
		body["price_quote_token"] = preview.PriceQuoteToken
		if err := repository.requireCustomerCapability(ctx, 501, "product_order"); err == nil {
			t.Fatal("all modes closed must restore the disabled ordinary product-order capability")
		}
		for _, path := range []string{"/api/mini/product-orders/preview", "/api/mini/product-orders"} {
			post(t, path, body, http.StatusBadRequest)
		}
	})
	for _, invalid := range []struct{ name, change string }{
		{"inactive", "active=false"}, {"unbound", "customer_id=0"}, {"other-customer", "customer_id=502"}, {"nonvirtual", "is_virtual=false"},
	} {
		t.Run("revoked-warehouse-"+invalid.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET %s WHERE code='DEV-E2E-A1'`, schema, invalid.change)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET active=true,customer_id=501,is_virtual=true WHERE code='DEV-E2E-A1'`, schema)); err != nil {
					t.Fatal(err)
				}
			})
			body := command()
			body["price_quote_token"] = preview.PriceQuoteToken
			for _, path := range []string{"/api/mini/product-orders/preview", "/api/mini/product-orders"} {
				post(t, path, body, http.StatusBadRequest)
			}
		})
	}
	for _, mode := range []string{"", "ship"} {
		t.Run("ordinary-shipment-"+mode, func(t *testing.T) {
			body := command()
			body["delivery_mode"], body["delivery_warehouse_code"] = mode, ""
			body["stock_purchase"] = true // UI hints cannot bypass shipment validation.
			for _, path := range []string{"/api/mini/product-orders/preview", "/api/mini/product-orders", "/api/mini/direct-ship/preview", "/api/mini/direct-ship/requests"} {
				post(t, path, body, http.StatusBadRequest)
			}
		})
	}
	for _, path := range []string{"/api/mini/direct-ship/preview", "/api/mini/direct-ship/requests"} {
		post(t, path, command(), http.StatusBadRequest)
	}
	assertMiniDirectShipCount(t, pool, schema, "orders", "customer_id=501", 0)
	body := command()
	body["price_quote_token"] = preview.PriceQuoteToken
	var created app.MiniDirectShipRequest
	if err := json.Unmarshal(post(t, "/api/mini/product-orders", body, http.StatusCreated), &created); err != nil {
		t.Fatal(err)
	}
	var customerID, promised, physical int64
	var mode, warehouse, receiverName, receiverPhone, receiverAddress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id,delivery_mode,delivery_warehouse_code,receiver_name,receiver_phone,receiver_address FROM %s.orders WHERE id=$1`, schema), created.OrderID).Scan(&customerID, &mode, &warehouse, &receiverName, &receiverPhone, &receiverAddress); err != nil {
		t.Fatal(err)
	}
	if customerID != 501 || mode != "customer_stock" || warehouse != "DEV-E2E-A1" || receiverName != "" || receiverPhone != "" || receiverAddress != "" {
		t.Fatalf("retained purchase saved incorrect customer/delivery/recipient: %d %q %q %q %q %q", customerID, mode, warehouse, receiverName, receiverPhone, receiverAddress)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(promised_qty),0) FROM %s.customer_stock_entitlements WHERE source_order_id=$1 AND customer_id=501 AND warehouse_code='DEV-E2E-A1'`, schema), created.OrderID).Scan(&promised); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(onhand_units),0) FROM %s.finished_inventory WHERE product_id=910 AND owner_customer_id=501`, schema)).Scan(&physical); err != nil {
		t.Fatal(err)
	}
	if promised != 1 || physical != 0 {
		t.Fatalf("retained purchase must create pending rights only: promised=%d physical=%d", promised, physical)
	}
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d AND action='submit'", created.ID), 1)
	var retry app.MiniDirectShipRequest
	if err := json.Unmarshal(post(t, "/api/mini/product-orders", body, http.StatusCreated), &retry); err != nil || retry.OrderID != created.OrderID {
		t.Fatalf("purchase retry=%+v err=%v", retry, err)
	}
	// Retain the ordinary service capability: HTTP authorization must still be
	// satisfied when retrying a saved purchase after stock configuration changes.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.customer_service_capabilities SET enabled=true WHERE customer_id=501 AND capability_code='product_order'; UPDATE %[1]s.warehouses SET active=false WHERE code='DEV-E2E-A1'; UPDATE %[1]s.customer_fulfillment_settings SET modes_json='[]' WHERE customer_id=501`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(post(t, "/api/mini/product-orders", body, http.StatusCreated), &retry); err != nil || retry.OrderID != created.OrderID || retry.TotalAmount != created.TotalAmount {
		t.Fatalf("saved purchase retry must retain saved result after warehouse/mode changes: %+v err=%v", retry, err)
	}
	newBody := command()
	newBody["idempotency_key"], newBody["price_quote_token"] = "RETAINED-API-NEW-AFTER-REVOKED", preview.PriceQuoteToken
	post(t, "/api/mini/product-orders", newBody, http.StatusBadRequest)
	assertMiniDirectShipCount(t, pool, schema, "orders", "customer_id=501", 1)
}

func TestFulfillmentSubmitCapabilityUsesCustomerSpecificModeDependency(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customer_portal_profiles(customer_id,enabled)VALUES(501,true); INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled,config_json)VALUES(501,'product_order',false,'{"required_by_fulfillment":true}')`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, template := range []string{"", "processing_fulfillment"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET capability_template_key=$1 WHERE customer_id=501`, schema), template); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			modes   string
			allowed bool
		}{
			{`[{"mode":"stock","enabled":true}]`, true},
			{`[{"mode":"on_demand","enabled":true}]`, true},
			{`[{"mode":"stock","enabled":true},{"mode":"on_demand","enabled":true}]`, true},
			{`[{"mode":"stock","enabled":false},{"mode":"on_demand","enabled":false}]`, false},
			{`[]`, false},
		} {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_fulfillment_settings SET modes_json=$1 WHERE customer_id=501`, schema), tc.modes); err != nil {
				t.Fatal(err)
			}
			if err := r.requireCustomerCapability(ctx, 501, "product_order"); (err == nil) != tc.allowed {
				t.Fatalf("template=%q modes=%s allowed=%v err=%v", template, tc.modes, tc.allowed, err)
			}
			if err := r.requireCustomerCapability(ctx, 502, "product_order"); err == nil {
				t.Fatal("another customer inherited the mode dependency")
			}
		}
	}
	var rawEnabled bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT enabled FROM %s.customer_service_capabilities WHERE customer_id=501 AND capability_code='product_order'`, schema)).Scan(&rawEnabled); err != nil || rawEnabled {
		t.Fatalf("derived capability changed customer configuration: enabled=%v err=%v", rawEnabled, err)
	}
}
