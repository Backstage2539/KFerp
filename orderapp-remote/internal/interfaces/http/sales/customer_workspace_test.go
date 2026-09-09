package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"image/png"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"os"
	"path/filepath"
	"strconv"
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
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
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
	var ids []int64
	for i := 1; i <= 8; i++ {
		result := request(http.MethodPost, path, payload(i), 200)
		if i == 1 {
			first = int64(result["order_id"].(float64))
		}
		ids = append(ids, int64(result["order_id"].(float64)))
		again := request(http.MethodPost, path, payload(i), 200)
		if again["order_id"] != result["order_id"] {
			t.Fatal("duplicate order on retry")
		}
	}
	list, err := svc.ListOrders(ctx, salesapp.OrderListQuery{CustomerID: 3, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Rows) != 8 {
		t.Fatal("expected eight fulfillment orders")
	}
	for _, row := range list.Rows {
		if row.ReceiverName != "" || row.ReceiverPhone != "" || row.ReceiverAddress != "" {
			t.Fatal("missing fulfillment recipient replaced by customer contact")
		}
	}
	edit, err := svc.OrderForm(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if edit.EditData.ReceiverName != "" || edit.EditData.ReceiverPhone != "" || edit.EditData.ReceiverAddress != "" {
		t.Fatal("edit form replaced missing recipient by customer contact")
	}
	for _, id := range ids {
		if _, err := svc.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: id, Revision: 1, Decision: "accepted", Admin: true, Actor: "管理员"}); err != nil {
			t.Fatal(err)
		}
	}
	verifyCustomerEightCombined(t, pool, schema, ids)
	var count int
	var amount float64
	var orderType int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*),sum(grand_total)::float8,min(order_type_id) FROM %s.orders", schema)).Scan(&count, &amount, &orderType); err != nil || count != 8 || amount != 480 || orderType != 1 {
		t.Fatalf("count=%d amount=%v type=%d err=%v", count, amount, orderType, err)
	}
	// The customer edits the same order through the full entry form. Accepted
	// accounting survives a pending edit and a rejected modification.
	currentForm := request(http.MethodGet, fmt.Sprintf("/api/customer-processing/portal/order/form?service=direct_ship&edit_id=%d", first), "", 200)
	currentEdit := currentForm["edit_data"].(map[string]any)
	var modified map[string]any
	json.Unmarshal([]byte(payload(1)), &modified)
	modified["request_id"] = "efs-existing-edit-0001"
	modified["edit_id"] = first
	modified["edit_revision"] = currentEdit["edit_revision"]
	modified["qty"] = []string{"3"}
	modified["order_date"] = "2026-07-20"
	modifiedBytes, _ := json.Marshal(modified)
	result := request(http.MethodPost, path, string(modifiedBytes), 200)
	if int64(result["order_id"].(float64)) != first || result["confirmation_status"] != "pending" {
		t.Fatalf("customer edit=%v", result)
	}
	var acceptedAmount float64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT grand_total FROM %s.orders WHERE id=$1", schema), first).Scan(&acceptedAmount); err != nil || acceptedAmount != 60 {
		t.Fatalf("pending accounting=%v err=%v", acceptedAmount, err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.bean_list_publications SET status='archived' WHERE id=88", schema))
	if _, err := svc.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: first, Revision: 2, Decision: "accepted", Admin: true, Actor: "管理员"}); err == nil {
		t.Fatal("archived price was accepted")
	}
	state, err := svc.OrderConfirmation(ctx, first)
	if err != nil || state.Status != "pending" {
		t.Fatalf("failed approval did not roll back: %+v %v", state, err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.bean_list_publications SET status='published' WHERE id=88", schema))
	if _, err := svc.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: first, Revision: 2, Decision: "rejected", Reason: "保留原订单", Admin: true, Actor: "管理员"}); err != nil {
		t.Fatal(err)
	}
	currentForm = request(http.MethodGet, fmt.Sprintf("/api/customer-processing/portal/order/form?service=direct_ship&edit_id=%d", first), "", 200)
	if currentForm["edit_data"].(map[string]any)["order_date"] != "2026-07-19" {
		t.Fatal("rejection did not restore order date")
	}
	request(http.MethodPost, fmt.Sprintf("/api/orders/%d/confirmation", first), `{"revision":2,"decision":"accepted"}`, 403)
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
	if _, err := svc.ReviewOrder(ctx, salesapp.ReviewOrderCommand{OrderID: first, Revision: 3, Decision: "accepted", Admin: true, Actor: "管理员"}); err != nil {
		t.Fatal(err)
	}
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

func verifyCustomerEightCombined(t *testing.T, pool *pgxpool.Pool, schema string, ids []int64) {
	t.Helper()
	if err := postgressales.EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatal(err)
	}
	e := newCombinedDocumentAPITestEcho(pool, schema, t.TempDir())
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		return rec
	}
	var values []string
	for _, id := range ids {
		values = append(values, strconv.FormatInt(id, 10))
	}
	query := strings.Join(values, ",")
	preview := request(http.MethodGet, "/api/orders/combined/sales-order-preview?order_ids="+query, "")
	var data salesapp.CombinedSalesOrderPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Snapshot.Groups) != 8 || data.Snapshot.GrandTotal != "480.00" {
		t.Fatal("combined order group/amount mismatch", data.Snapshot.GrandTotal, len(data.Snapshot.Groups))
	}
	for _, group := range data.Snapshot.Groups {
		if group.OrderDate != "2026-07-19" || group.DocumentDate != "2026-09-09" || len(group.Items) != 1 {
			t.Fatal("combined lost original order fields", group)
		}
	}
	pdf := request(http.MethodGet, "/api/orders/combined/sales-order-preview.pdf?order_ids="+query, "")
	if !bytes.HasPrefix(pdf.Body.Bytes(), []byte("%PDF-")) {
		t.Fatal("invalid PDF")
	}
	payload, _ := json.Marshal(map[string]any{"order_ids": ids})
	created := request(http.MethodPost, "/api/orders/combined/sales-order-images", string(payload))
	var imageDoc salesapp.CombinedSalesOrderImageDocument
	json.Unmarshal(created.Body.Bytes(), &imageDoc)
	image := request(http.MethodGet, imageDoc.DownloadURL, "")
	decoded, err := png.Decode(bytes.NewReader(image.Body.Bytes()))
	if err != nil || decoded.Bounds().Dy() < 1700 {
		t.Fatal("invalid PNG", err)
	}
	if dir := os.Getenv("KFERP_TEST_ARTIFACT_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "efs-eight-combined.pdf"), pdf.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "efs-eight-combined.png"), image.Body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
