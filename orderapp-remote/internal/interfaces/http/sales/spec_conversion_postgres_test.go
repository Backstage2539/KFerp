package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	conversion "orderapp/internal/domain/specconversion"
	"orderapp/internal/infrastructure/postgres/orderbeans"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strconv"
	"strings"
	"testing"
)

func TestConversionPostgresHTTPAndFrozenOrder(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	if err := postgressales.EnsureSpecConversionSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_boms(id BIGINT PRIMARY KEY,status TEXT);
 INSERT INTO %[1]s.production_boms VALUES(8001,'active');
 ALTER TABLE %[1]s.production_bom_specs ADD COLUMN source_spec_template_id BIGINT DEFAULT 0,ADD COLUMN source_spec_template_key TEXT DEFAULT '';
 ALTER TABLE %[1]s.stock_batches ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.finished_inventory ADD COLUMN IF NOT EXISTS bom_spec_id BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS bom_variant_id BIGINT NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.orders ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.order_items ADD COLUMN customer_product_reference_id BIGINT, ADD COLUMN material_source_mode TEXT;
 CREATE TABLE %[1]s.customer_order_production_demands(order_id BIGINT);
 UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
 CREATE TABLE %[1]s.product_unit_definitions(code TEXT PRIMARY KEY,name TEXT,active BOOLEAN);
 INSERT INTO %[1]s.product_unit_definitions VALUES('kg','千克',true);
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(9003,8001,'250g','250g','250g 袋装','袋'),(9004,8001,'unrelated','unrelated','500g 袋装','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit) VALUES(9103,8101,9003,'250g 袋装','袋'),(9104,8101,9004,'500g 袋装','袋');
 UPDATE %[1]s.production_bom_specs SET name='500g 袋装' WHERE id=9001;
 UPDATE %[1]s.production_bom_version_variants SET spec_name_snapshot='500g 袋装' WHERE id=9101;
 INSERT INTO %[1]s.bean_list_publications(id,owner_type,owner_key,list_type,status,publication_purpose,version_no,content_json,config_json) VALUES(88,'official','','commercial','published','factory_supply','V-692',
 '{"price_rows":[{"product_id":7,"bom_spec_id":9002,"bom_variant_id":9102,"quantity_basis":"sales_spec_count","min_qty":48,"final_unit_price":48,"price_unit":"袋","inventory_unit":"袋","tier_label":"48袋以上"}]}',
 '{"publication_batch":{"release_id":"692","table_key":"lb","table_name":"磅装表","is_default":true}}');`, schema))
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	e := newOrderAPITestEcho(pool, schema)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("basic_auth_admin", true); return next(c) }
	})
	call := func(method, path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec
	}
	group := conversion.Group{Name: "包装转KG", BaseUnit: "kg", Active: true, Members: []conversion.Member{{SpecID: 9002, InventoryUnit: "袋", BaseQuantity: "0.454"}, {SpecID: 9001, InventoryUnit: "袋", BaseQuantity: "0.5"}, {SpecID: 9003, InventoryUnit: "袋", BaseQuantity: "0.25"}}}
	rec := call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": group}, 200)
	if err := json.Unmarshal(rec.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	for _, factor := range []string{"0", "-1", ""} {
		invalid := group
		invalid.ID = 0
		invalid.Name = "invalid"
		invalid.Members = append([]conversion.Member{}, group.Members...)
		invalid.Members[0].BaseQuantity = factor
		call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": invalid}, 400)
	}
	duplicate := group
	duplicate.ID = 0
	duplicate.Name = "duplicate"
	duplicate.Members = append(append([]conversion.Member{}, group.Members...), group.Members[0])
	call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": duplicate}, 400)
	key := orderbeans.PublishedPriceRowKey(88, "commercial", 7, 9002, 9102, "flat", 0)
	selection := salesapp.SpecConversionSelection{GroupID: group.ID, Revision: group.Revision, SourceSpecID: 9002, SourceVariantID: 9102}
	quoteReq := salesapp.SpecConversionQuoteRequest{SpecConversionSelection: selection, CustomerID: 3, ProductID: 7, PublicationID: 88, PriceRowKey: key, TargetSpecID: 9001, TargetVariantID: 9101, Quantity: 2}

	call("POST", "/api/order/spec-conversions/quote", quoteReq, 410)
	// Seed a pre-rollback converted line with a server-generated legacy snapshot.
	quote, err := postgressales.NewRepository(pool, schema).QuoteSpecConversion(ctx, quoteReq)
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := json.Marshal(selection)
	payload := map[string]any{"order_date": "2026-10-18", "customer_id": 3, "pay_status_id": 1, "ship_status_id": 1, "selected_price_table_ids": []int64{88}, "product_id": []string{"7"}, "bom_spec_id": []string{"9002"}, "bom_variant_id": []string{"9102"}, "item_name": []string{"橘皮乌龙"}, "qty": []string{"48"}, "price_selection_mode": []string{"auto"}, "item_bean_list_publication_id": []string{"88"}}
	rec = call("POST", "/api/order", payload, 200)
	var saved struct {
		ID int64 `json:"order_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &saved)
	if _, err := pool.Exec(ctx, fmt.Sprintf("UPDATE %s.order_items SET bom_spec_id=9001,bom_variant_id=9101,qty=2,unit_price=52.86,line_total=105.72,price_source_json=$1 WHERE order_id=$2", schema), quote.PriceSourceJSON, saved.ID); err != nil {
		t.Fatal(err)
	}
	var itemID int64
	var price, total, snapshot string
	readItem := func() {
		t.Helper()
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT id,unit_price::text,line_total::text,price_source_json::text FROM %s.order_items WHERE order_id=$1", schema), saved.ID).Scan(&itemID, &price, &total, &snapshot); err != nil {
			t.Fatal(err)
		}
	}
	readItem()
	payload["bom_spec_id"] = []string{"9001"}
	payload["bom_variant_id"] = []string{"9101"}
	payload["qty"] = []string{"3"}
	payload["price_selection_mode"] = []string{"converted_tier"}
	payload["selected_price_row_key"] = []string{key}
	payload["spec_conversion"] = []string{string(selected)}
	// A new line, even with a forged price snapshot or another order item ID, is rejected.
	payload["item_id"] = []string{strconv.FormatInt(itemID, 10)}
	payload["price_source_json"] = []string{`{"spec_conversion":{"unit_price":"0.01"}}`}
	call("POST", "/api/order", payload, 400)
	payload["edit_id"] = saved.ID
	call("POST", "/api/order", payload, 200)
	readItem()
	if price != "52.86" || total != "158.58" {
		t.Fatal("frozen price changed", price, total)
	}
	selection.Reprice = true
	selected, _ = json.Marshal(selection)
	payload["spec_conversion"] = []string{string(selected)}
	call("POST", "/api/order", payload, 400)
	// Group maintenance remains auditable; it cannot reopen retired order pricing.
	group.Members[1].BaseQuantity = "0.6"
	rec = call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": group}, 200)
	_ = json.Unmarshal(rec.Body.Bytes(), &group)
	stale := group
	stale.Revision--
	call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": stale}, 409)
	group.Active = false
	call("POST", "/api/product-settings/spec-conversion-groups", map[string]any{"group": group}, 200)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.bean_list_publications SET status='withdrawn' WHERE id=88", schema))
	selection.Reprice = false
	selected, _ = json.Marshal(selection)
	payload["spec_conversion"] = []string{string(selected)}
	payload["qty"] = []string{"4"}
	payload["item_id"] = []string{strconv.FormatInt(itemID, 10)}
	call("POST", "/api/order", payload, 200)
	readItem()
	if price != "52.86" || total != "211.44" {
		t.Fatal("withdrawn history changed", price, total)
	}
	var count, audits int
	var sourcePrice string
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*),max(content_json->'price_rows'->0->>'final_unit_price') FROM %s.bean_list_publications", schema)).Scan(&count, &sourcePrice); err != nil {
		t.Fatal(err)
	}
	if count != 1 || sourcePrice != "48" {
		t.Fatal("price table changed")
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s.audit_logs WHERE entity_type='spec_conversion_group'", schema)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits < 3 {
		t.Fatal("group audit missing")
	}
	if !strings.Contains(snapshot, "spec_conversion") {
		t.Fatal("lost historical price basis")
	}
}
