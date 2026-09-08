package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"
)

func TestOrderExperienceAPIDeliveryRoundTripAndCustomerTypeSync(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.customer_order_production_demands(order_id bigint);
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_reference_id bigint NOT NULL DEFAULT 0;
ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS material_source_mode text DEFAULT 'factory';`, schema))
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.ship_statuses(id,name) VALUES(2,'已发货') ON CONFLICT(id) DO UPDATE SET name=EXCLUDED.name`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sales_order_documents", "sales_order_images", "delivery_note_documents"} {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s(order_id bigint,is_latest boolean)`, schema, table))
	}
	for _, table := range []string{"combined_sales_order_documents", "combined_sales_order_images", "combined_delivery_note_documents"} {
		mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s(order_ids jsonb,is_latest boolean)`, schema, table))
	}
	e := newOrderAPITestEcho(pool, schema)
	for _, method := range []string{"pickup", "local_delivery", "sf_small"} {
		payload := map[string]any{"order_date": "2026-09-08", "customer_id": 3, "pay_status_id": 1, "ship_status_id": 1, "ship_method": method,
			"product_id": []string{"7"}, "parent_product_id": []string{"7"}, "bom_spec_id": []string{"9001"}, "bom_variant_id": []string{"9101"}, "item_name": []string{"测试商品"}, "tier_id": []string{"manual"}, "unit_price": []string{"88"}, "qty": []string{"5"}, "unit": []string{"件"}, "spec": []string{"454"}}
		send := func() *httptest.ResponseRecorder {
			b, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(string(b)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			return rec
		}
		rec := send()
		if rec.Code != 200 {
			t.Fatalf("create %s: %d %s", method, rec.Code, rec.Body.String())
		}
		var result struct {
			OrderID int64 `json:"order_id"`
		}
		json.Unmarshal(rec.Body.Bytes(), &result)
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customers SET default_order_type_id=1 WHERE id=3`, schema)); err != nil {
			t.Fatal(err)
		}
		var before int64
		pool.QueryRow(ctx, fmt.Sprintf(`SELECT order_type_id FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&before)
		if before != 2 {
			t.Fatalf("customer change modified historical order: %d", before)
		}
		payload["edit_id"] = result.OrderID
		payload["order_type_id"] = 2 // stale browser values cannot override customer defaults.
		if method != "sf_small" {
			payload["ship_status_id"] = 2
		}
		rec = send()
		if rec.Code != 200 {
			t.Fatalf("edit %s: %d %s", method, rec.Code, rec.Body.String())
		}
		var savedMethod string
		var savedType int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT ship_method,order_type_id FROM %s.orders WHERE id=$1`, schema), result.OrderID).Scan(&savedMethod, &savedType); err != nil {
			t.Fatal(err)
		}
		if savedMethod != method || savedType != 1 {
			t.Fatalf("roundtrip got %s/%d", savedMethod, savedType)
		}
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/orders/%d/detail", result.OrderID), nil)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ship_method":"`+method+`"`) || !strings.Contains(rec.Body.String(), `"order_type_id":"1"`) {
			t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
		}
		var audit string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT new_value FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 ORDER BY id DESC LIMIT 1`, schema), result.OrderID).Scan(&audit); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(audit, `"ship_method": "`+method+`"`) || !strings.Contains(audit, `"order_type_id": 1`) {
			t.Fatalf("delivery/type missing from audit: %s", audit)
		}
		if method != "sf_small" {
			req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/orders/%d/shipping-tracking", result.OrderID), strings.NewReader(`{"tracking_no":"SF123456789"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec = httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != 400 {
				t.Fatalf("non-courier tracking accepted: %d %s", rec.Code, rec.Body.String())
			}
		}
		pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customers SET default_order_type_id=2 WHERE id=3`, schema))
	}
}

func TestOrderExperienceCourierExportRejectsNonCourier(t *testing.T) {
	for _, method := range []string{"pickup", "local_delivery"} {
		if orderShippingReady(salesapp.OrderShippingExportData{ShipMethod: method, ProcessStatus: "无需生产"}) {
			t.Fatal("non courier export allowed")
		}
	}
}

func TestOrderExperienceEditAPIRetainsThreeProductSpecIdentities(t *testing.T) {
	data := editDataForAPI(&OrderEditData{ShipMethod: "pickup", Items: []OrderEditItem{
		{ProductID: 58, BomSpecID: 69, BomVariantID: 230, Product: "墨照啡石", Qty: "5", UnitPrice: "56", CustomerProductReferenceID: 101},
		{ProductID: 57, BomSpecID: 48, BomVariantID: 48, Product: "菠浪清甜", Qty: "5", UnitPrice: "92", CustomerProductReferenceID: 102},
		{ProductID: 48, BomSpecID: 242, BomVariantID: 407, Product: "酒心可可", Qty: "5", UnitPrice: "59", CustomerProductReferenceID: 103},
	}})
	b, _ := json.Marshal(data["items"])
	var items []struct {
		ProductID   int64  `json:"product_id"`
		BomSpecID   int64  `json:"bom_spec_id"`
		ReferenceID int64  `json:"customer_product_reference_id"`
		ProductName string `json:"product_name"`
	}
	json.Unmarshal(b, &items)
	if len(items) != 3 || items[2].ProductID != 48 || items[2].BomSpecID != 242 || items[2].ReferenceID != 103 || items[2].ProductName != "酒心可可" {
		t.Fatalf("identity lost: %s", b)
	}
}
