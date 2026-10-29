package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	authzapp "orderapp/internal/application/authz"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
)

func TestBatchOrderStatusAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
        ALTER TABLE %[1]s.orders ADD COLUMN confirmation_required BOOLEAN NOT NULL DEFAULT false, ADD COLUMN confirmation_status TEXT NOT NULL DEFAULT 'accepted', ADD COLUMN delivery_mode TEXT NOT NULL DEFAULT 'ship';
        INSERT INTO %[1]s.pay_statuses(id,name) VALUES (3,'已收款');
        INSERT INTO %[1]s.ship_statuses(id,name) VALUES (2,'已发货');
        INSERT INTO %[1]s.sales_order_assets(id,kind,filename,created_by) VALUES (71,'payment_voucher','one.png','tester'),(72,'payment_voucher','other.png','other');
        ALTER TABLE %[1]s.orders ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
        ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS material_source_mode TEXT NOT NULL DEFAULT 'factory';
        ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS customer_product_reference_id BIGINT NOT NULL DEFAULT 0;
        CREATE TABLE %[1]s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL DEFAULT 'legacy',configured BOOLEAN NOT NULL DEFAULT false);
        INSERT INTO %[1]s.orders(id,order_no,customer_id,order_type_id,order_date,document_date,pay_status_id,ship_status_id,payment_method,ship_method,total_amount,grand_total,is_void)
        VALUES(901,'BATCH-1',3,1,'2026-10-18','2026-10-18',1,1,'','pickup',100,100,false),
              (902,'BATCH-2',3,1,'2026-10-18','2026-10-18',1,1,'','pickup',100,100,false),
              (903,'BATCH-VOID',3,1,'2026-10-18','2026-10-18',1,1,'','pickup',100,100,true);
    `, schema))
	if _, err := salesapp.NewService(postgressales.NewRepository(pool, schema)).OrderStatusEditData(ctx, 901); err != nil {
		t.Fatalf("form fixture: %v", err)
	}
	actor := &orderAPIAuthzService{actor: authzapp.Actor{Permissions: []string{"orders.write"}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(1)); c.Set("actor", "tester"); return next(c) }
	})
	registerOrderAPI(e, salesapp.NewService(postgressales.NewRepository(pool, schema)), nil, actor)
	request := func(path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	preview := request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901, 902, 903, 999}})
	if preview.Code != 200 {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var p struct {
		Orders []batchStatusPreviewRow `json:"orders"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Orders) != 4 || p.Orders[0].EditRevision == "" || p.Orders[2].Error == "" || p.Orders[3].Error == "" {
		t.Fatalf("preview: %s", preview.Body.String())
	}
	body := map[string]any{"orders": []any{
		map[string]any{"order_id": 901, "edit_revision": p.Orders[0].EditRevision, "pay_status_id": 2, "payment_method": "现金"},
		map[string]any{"order_id": 902, "edit_revision": "stale", "pay_status_id": 2, "payment_method": "现金"},
		map[string]any{"order_id": 903, "edit_revision": p.Orders[2].EditRevision, "pay_status_id": 2, "payment_method": "现金"},
	}}
	result := request("/api/orders/status-batch", body)
	if result.Code != 200 {
		t.Fatalf("save %d %s", result.Code, result.Body.String())
	}
	var saved struct {
		Results []batchStatusResult `json:"results"`
	}
	json.Unmarshal(result.Body.Bytes(), &saved)

	if len(saved.Results) != 3 || saved.Results[0].Status != "updated" || saved.Results[1].Status != "failed" || saved.Results[2].Status != "failed" {
		t.Fatalf("partial results: %s", result.Body.String())
	}
	retry := request("/api/orders/status-batch", body)
	json.Unmarshal(retry.Body.Bytes(), &saved)
	if saved.Results[0].Status != "failed" {
		t.Fatalf("stale retry must fail: %s", retry.Body.String())
	}
	preview = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901}})
	json.Unmarshal(preview.Body.Bytes(), &p)
	unchanged := request("/api/orders/status-batch", map[string]any{"orders": []any{map[string]any{"order_id": 901, "edit_revision": p.Orders[0].EditRevision, "pay_status_id": 2, "payment_method": "现金"}}})
	json.Unmarshal(unchanged.Body.Bytes(), &saved)
	if saved.Results[0].Status != "unchanged" {
		t.Fatalf("no-op: %s", unchanged.Body.String())
	}
	// A combined payment/shipping change must roll back as a single order when
	// the selected courier shipment lacks logistics information.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET ship_method='快递' WHERE id=902`, schema))
	preview = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{902}})
	json.Unmarshal(preview.Body.Bytes(), &p)
	combined := request("/api/orders/status-batch", map[string]any{"orders": []any{map[string]any{"order_id": 902, "edit_revision": p.Orders[0].EditRevision, "pay_status_id": 2, "ship_status_id": 2, "payment_method": "现金"}}})
	json.Unmarshal(combined.Body.Bytes(), &saved)
	if saved.Results[0].Status != "failed" {
		t.Fatalf("missing logistics: %s", combined.Body.String())
	}
	var pay int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id FROM %s.orders WHERE id=902`, schema)).Scan(&pay); err != nil || pay != 1 {
		t.Fatalf("combined update partly committed %d %v", pay, err)
	}

	// Each order gets its own receipt and uploader ownership is enforced.
	preview = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901, 902}})
	json.Unmarshal(preview.Body.Bytes(), &p)
	receipts := request("/api/orders/status-batch", map[string]any{"orders": []any{
		map[string]any{"order_id": 901, "edit_revision": p.Orders[0].EditRevision, "pay_status_id": 3, "payment_method": "现金", "payment_goods_amount": 100, "payment_shipping_amount": 0, "payment_voucher_asset_id": 71},
		map[string]any{"order_id": 902, "edit_revision": p.Orders[1].EditRevision, "pay_status_id": 3, "payment_method": "现金", "payment_goods_amount": 50, "payment_shipping_amount": 0, "payment_voucher_asset_id": 72},
	}})
	json.Unmarshal(receipts.Body.Bytes(), &saved)
	if saved.Results[0].Status != "updated" || saved.Results[1].Status != "failed" {
		t.Fatalf("receipt ownership: %s", receipts.Body.String())
	}
	var goods float64
	var voucher int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_goods_amount::float8,payment_voucher_asset_id FROM %s.orders WHERE id=901`, schema)).Scan(&goods, &voucher); err != nil || goods != 100 || voucher != 71 {
		t.Fatalf("receipt %v/%v: %v", goods, voucher, err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id FROM %s.orders WHERE id=902`, schema)).Scan(&pay); err != nil || pay != 1 {
		t.Fatalf("failed receipt altered order %d: %v", pay, err)
	}

	// Pending fulfillment and customer-stock delivery remain protected.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending' WHERE id=902`, schema))
	preview = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{902}})
	json.Unmarshal(preview.Body.Bytes(), &p)
	if p.Orders[0].Error == "" {
		t.Fatalf("pending unprotected: %s", preview.Body.String())
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=false,confirmation_status='accepted',delivery_mode='customer_stock' WHERE id=902`, schema))
	preview = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{902}})
	json.Unmarshal(preview.Body.Bytes(), &p)
	blocked := request("/api/orders/status-batch", map[string]any{"orders": []any{map[string]any{"order_id": 902, "edit_revision": p.Orders[0].EditRevision, "ship_status_id": 2}}})
	json.Unmarshal(blocked.Body.Bytes(), &saved)
	if saved.Results[0].Status != "failed" {
		t.Fatalf("stock shipping unprotected: %s", blocked.Body.String())
	}

	actor.actor.Permissions = nil
	denied := request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901}})
	if denied.Code != 403 {
		t.Fatalf("permission %d", denied.Code)
	}
	actor.actor = authzapp.Actor{AccountType: "channel_customer", Permissions: []string{"orders.write"}}
	denied = request("/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901}})
	if denied.Code != 403 {
		t.Fatalf("channel permission %d", denied.Code)
	}
}
