package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"sync"
	"testing"
)

type workspaceTestScope struct{}

func (workspaceTestScope) BoundCustomerID(context.Context, int64) (int64, error) { return 3, nil }
func (workspaceTestScope) CustomerWorkspace(_ context.Context, employee, id int64, page string, _ int64) (map[string]any, error) {
	if employee != 1 || id != 0 && id != 3 {
		return nil, fmt.Errorf("scope mismatch")
	}
	if page != "context" && page != "direct_ship" {
		return nil, fmt.Errorf("capability unavailable")
	}
	return map[string]any{"customer_id": int64(3), "customer_name": "测试客户", "capabilities": []string{"direct_ship"}}, nil
}
func TestCustomerWorkspaceContinuousOrdersAndRecipientIsolation(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
 ALTER TABLE %[1]s.order_items ADD COLUMN customer_product_reference_id BIGINT,ADD COLUMN material_source_mode TEXT;
 CREATE TABLE %[1]s.customer_order_production_demands(order_id BIGINT);
 UPDATE %[1]s.customers SET default_order_type_id=1 WHERE id=3;
 UPDATE %[1]s.products SET customer_id=3,visibility='customer_only' WHERE id=7;
 INSERT INTO %[1]s.ship_statuses(id,name)VALUES(9,'已发货');
 INSERT INTO %[1]s.bean_list_publications(id,list_type,version_no,status,owner_type,owner_key,config_json,content_json) VALUES(88,'commercial','V3.0.6','published','customer','3','{"publication_batch":{"release_id":"efs-test","table_key":"normal","table_name":"测试价格表","is_default_table":true}}','{"price_rows":[{"product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"min_qty":1,"final_unit_price":30,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","parent_product_id":7,"sales_unit":"袋","effective_sales_spec":{"product_id":7,"bom_spec_id":9001,"bom_variant_id":9101,"spec_name":"227g袋","sales_unit":"袋"}}]}');`, schema))
	if err := postgressales.EnsureCustomerOrderSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	e := newOrderAPITestEcho(pool, schema)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("customer_fulfillment_order_scope_limited", true); return next(c) }
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc, nil, "", workspaceTestScope{})
	request := func(method, path, body string, code int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != code {
			t.Fatalf("%s %s: %d want %d: %s", method, path, rec.Code, code, rec.Body.String())
		}
		var result map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &result)
		return result
	}
	payload := func(i int) string {
		return fmt.Sprintf(`{"request_id":"efs-order-request-%02d","backfill_mode":true,"portal_service_code":"direct_ship","document_date":"2026-09-09","order_date":"2026-07-19","customer_id":3,"selected_price_table_ids":[88],"bean_list_publication_id":88,"item_bean_list_publication_id":["88"],"product_id":["7"],"bom_spec_id":["9001"],"bom_variant_id":["9101"],"item_name":["测试豆"],"tier_id":["auto"],"unit_price":[""],"qty":["2"],"unit":["袋"],"spec":["227"]}`, i)
	}
	path := "/api/customer-processing/portal/order"
	request(http.MethodPost, path, strings.Replace(payload(1), `"backfill_mode":true`, `"backfill_mode":false`, 1), 400)
	var first int64
	for i := 1; i <= 8; i++ {
		result := request(http.MethodPost, path, payload(i), 200)
		if i == 1 {
			first = int64(result["order_id"].(float64))
		}
		again := request(http.MethodPost, path, payload(i), 200)
		if again["order_id"] != result["order_id"] {
			t.Fatal("duplicate order on retry")
		}
	}
	var count int
	var amount float64
	var orderType int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*),sum(grand_total)::float8,min(order_type_id) FROM %s.orders", schema)).Scan(&count, &amount, &orderType); err != nil || count != 8 || amount != 480 || orderType != 1 {
		t.Fatalf("count=%d amount=%v type=%d err=%v", count, amount, orderType, err)
	}
	request(http.MethodPost, path, strings.Replace(payload(1), `"qty":["2"]`, `"qty":["3"]`, 1), 400)
	request(http.MethodPost, path, strings.Replace(payload(9), `"unit_price":[""]`, `"unit_price":["1"]`, 1), 400)
	request(http.MethodPost, path, strings.Replace(payload(9), `"customer_id":3`, `"customer_id":4`, 1), 403)
	request(http.MethodGet, "/api/customer-processing/portal/order/form?service=processing", "", 400)
	form := request(http.MethodGet, "/api/customer-processing/portal/order/form?service=direct_ship", "", 200)
	if len(form["customers"].([]any)) != 1 || len(form["employees"].([]any)) != 0 {
		t.Fatal("form leaked unscoped identities")
	}
	request(http.MethodGet, "/api/customer-processing/portal/order/form?service=direct_ship&customer_id=4", "", 403)
	if _, err := pool.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET ship_status_id=9 WHERE id=$1", schema), first); err == nil {
		t.Fatal("shipped without recipient")
	}
	recipient := `{"receiver_name":"测试收件人","receiver_phone":"13800000000","receiver_address":"测试地址"}`
	request(http.MethodPatch, fmt.Sprintf("/api/customer-processing/portal/orders/%d/recipient", first), recipient, 200)
	var after float64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total::float8 FROM %s.orders WHERE id=$1", schema), first).Scan(&after); err != nil || after != 60 {
		t.Fatal("recipient changed historical amount", after, err)
	}
	request(http.MethodPatch, "/api/customer-processing/portal/orders/99999/recipient", recipient, 400)
	if _, err := pool.Exec(ctx, fmt.Sprintf("UPDATE %s.orders SET ship_status_id=9 WHERE id=$1", schema), first); err != nil {
		t.Fatal("complete recipient should allow shipping", err)
	}
	request(http.MethodPatch, fmt.Sprintf("/api/customer-processing/portal/orders/%d/recipient", first), recipient, 400)
	// Concurrent delivery of the same request must serialize to one order.
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(payload(9)))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec
		}()
	}
	wg.Wait()
	close(results)
	var ninth any
	for rec := range results {
		if rec.Code != 200 {
			t.Fatal(rec.Code, rec.Body.String())
		}
		var row map[string]any
		json.Unmarshal(rec.Body.Bytes(), &row)
		if ninth == nil {
			ninth = row["order_id"]
		}
		if ninth != row["order_id"] {
			t.Fatal("concurrent retry created duplicates")
		}
	}
	// The original response remains available after the price table is withdrawn.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.bean_list_publications SET status='archived' WHERE id=88", schema))
	request(http.MethodPost, path, payload(1), 200)
	request(http.MethodPost, path, payload(10), 400)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.bean_list_publications SET status='published' WHERE id=88", schema))
	request(http.MethodPost, path, payload(10), 200) // a rejected request can be corrected and retried.

}
