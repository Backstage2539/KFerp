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

func TestCombinedPaymentHTTP(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
      ALTER TABLE %[1]s.orders ADD COLUMN confirmation_required BOOLEAN NOT NULL DEFAULT false, ADD COLUMN confirmation_status TEXT NOT NULL DEFAULT 'accepted', ADD COLUMN delivery_mode TEXT NOT NULL DEFAULT 'ship', ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0;
      ALTER TABLE %[1]s.order_items ADD COLUMN IF NOT EXISTS material_source_mode TEXT NOT NULL DEFAULT 'factory', ADD COLUMN IF NOT EXISTS customer_product_reference_id BIGINT NOT NULL DEFAULT 0;
      CREATE TABLE %[1]s.product_bom_spec_authorities(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL DEFAULT 'legacy',configured BOOLEAN NOT NULL DEFAULT false);
	      INSERT INTO %[1]s.orders(id,order_no,customer_id,order_type_id,order_date,document_date,pay_status_id,ship_status_id,payment_method,ship_method,total_amount,grand_total,is_void,notes)
      VALUES(901,'COMBINED-1',3,1,'2026-10-18','2026-10-18',1,1,'','pickup',100,100,false,'原备注'),
	            (902,'COMBINED-2',3,1,'2026-10-18','2026-10-18',1,1,'','pickup',50,50,false,''),
	            (903,'COMBINED-PAID',3,1,'2026-10-18','2026-10-18',2,1,'现金','pickup',75,75,false,'');
      CREATE TABLE %[1]s.order_combined_payments(id BIGSERIAL PRIMARY KEY,idempotency_key TEXT NOT NULL UNIQUE,request_hash TEXT NOT NULL,customer_id BIGINT NOT NULL,actual_cents BIGINT NOT NULL,due_cents BIGINT NOT NULL,unallocated_cents BIGINT NOT NULL,payment_method TEXT NOT NULL,payment_voucher_asset_id BIGINT NOT NULL DEFAULT 0,actor TEXT NOT NULL,result_json JSONB NOT NULL DEFAULT '{}'::jsonb,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
      CREATE TABLE %[1]s.order_combined_payment_allocations(batch_id BIGINT NOT NULL REFERENCES %[1]s.order_combined_payments(id),order_id BIGINT NOT NULL REFERENCES %[1]s.orders(id),order_no TEXT NOT NULL,goods_cents BIGINT NOT NULL,shipping_cents BIGINT NOT NULL,amount_cents BIGINT NOT NULL,PRIMARY KEY(batch_id,order_id));
    `, schema))
	actor := &orderAPIAuthzService{actor: authzapp.Actor{Permissions: []string{"orders.read", "orders.write"}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("employee_id", int64(1)); c.Set("actor", "tester"); return next(c) }
	})
	registerOrderAPI(e, salesapp.NewService(postgressales.NewRepository(pool, schema)), nil, actor)
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	preview := request(http.MethodPost, "/api/orders/status-batch/preview", map[string]any{"order_ids": []int{901, 902}})
	var p struct {
		Orders []batchStatusPreviewRow `json:"orders"`
	}
	if preview.Code != 200 || json.Unmarshal(preview.Body.Bytes(), &p) != nil || len(p.Orders) != 2 || p.Orders[0].Error != "" || p.Orders[1].Error != "" || p.Orders[0].DueCents != 10000 || p.Orders[1].DueCents != 5000 {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	paidPreview := request(http.MethodPost, "/api/orders/status-batch/preview", map[string]any{"order_ids": []int{903}})
	var paidRows struct {
		Orders []batchStatusPreviewRow `json:"orders"`
	}
	if paidPreview.Code != 200 || json.Unmarshal(paidPreview.Body.Bytes(), &paidRows) != nil || len(paidRows.Orders) != 1 || !paidRows.Orders[0].FullyPaid || paidRows.Orders[0].DueCents != 0 || paidRows.Orders[0].TotalCents != 7500 {
		t.Fatalf("paid preview %d %s", paidPreview.Code, paidPreview.Body.String())
	}
	path := "/api/orders/status-batch/combined-payment"
	orders := []any{map[string]any{"order_id": 901, "edit_revision": p.Orders[0].EditRevision}, map[string]any{"order_id": 902, "edit_revision": p.Orders[1].EditRevision}}
	body := map[string]any{"orders": orders, "actual_cents": 15500, "expected_due_cents": 15000, "payment_method": "微信", "idempotency_key": "http-test-1"}
	under := map[string]any{"orders": orders, "actual_cents": 14999, "expected_due_cents": 15000, "payment_method": "微信", "idempotency_key": "http-under"}
	if rec := request(http.MethodPost, path, under); rec.Code != http.StatusConflict {
		t.Fatalf("underpaid %d %s", rec.Code, rec.Body.String())
	}
	fractional := map[string]any{"orders": orders, "actual_cents": 15000.5, "expected_due_cents": 15000, "payment_method": "微信", "idempotency_key": "http-fraction"}
	if rec := request(http.MethodPost, path, fractional); rec.Code != http.StatusBadRequest {
		t.Fatalf("fractional cents %d %s", rec.Code, rec.Body.String())
	}
	stale := map[string]any{"orders": []any{orders[0], map[string]any{"order_id": 902, "edit_revision": "stale"}}, "actual_cents": 15000, "expected_due_cents": 15000, "payment_method": "微信", "idempotency_key": "http-stale"}
	if rec := request(http.MethodPost, path, stale); rec.Code != http.StatusConflict {
		t.Fatalf("stale %d %s", rec.Code, rec.Body.String())
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_combined_payments`, schema)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid request wrote record: %d %v", count, err)
	}
	paid := request(http.MethodPost, path, body)
	var result salesapp.CombinedOrderPaymentResult
	if paid.Code != 200 || json.Unmarshal(paid.Body.Bytes(), &result) != nil || result.UnallocatedCents != 500 || len(result.Allocations) != 2 {
		t.Fatalf("payment %d %s", paid.Code, paid.Body.String())
	}
	replay := request(http.MethodPost, path, body)
	var replayResult salesapp.CombinedOrderPaymentResult
	if replay.Code != 200 || json.Unmarshal(replay.Body.Bytes(), &replayResult) != nil || !replayResult.Reused || replayResult.BatchID != result.BatchID {
		t.Fatalf("replay %d %s", replay.Code, replay.Body.String())
	}
	changedRetry := map[string]any{"orders": orders, "actual_cents": 15600, "expected_due_cents": 15000, "payment_method": "微信", "idempotency_key": "http-test-1"}
	if rec := request(http.MethodPost, path, changedRetry); rec.Code != http.StatusConflict {
		t.Fatalf("changed retry %d %s", rec.Code, rec.Body.String())
	}
	history := request(http.MethodGet, "/api/orders/901/combined-payments", nil)
	var h struct {
		Payments []salesapp.CombinedOrderPaymentResult `json:"payments"`
	}
	if history.Code != 200 || json.Unmarshal(history.Body.Bytes(), &h) != nil || len(h.Payments) != 1 || h.Payments[0].UnallocatedCents != 500 || len(h.Payments[0].Allocations) != 1 || h.Payments[0].Allocations[0].OrderID != 901 {
		t.Fatalf("history %d %s", history.Code, history.Body.String())
	}
	actor.actor.Permissions = nil
	if rec := request(http.MethodGet, "/api/orders/901/combined-payments", nil); rec.Code != 403 {
		t.Fatalf("history permission %d", rec.Code)
	}
	if rec := request(http.MethodPost, path, body); rec.Code != 403 {
		t.Fatalf("payment permission %d", rec.Code)
	}
	actor.actor = authzapp.Actor{AccountType: "channel_customer", Permissions: []string{"orders.read", "orders.write"}}
	if rec := request(http.MethodGet, "/api/orders/901/combined-payments", nil); rec.Code != 403 {
		t.Fatalf("channel history permission %d", rec.Code)
	}
	if rec := request(http.MethodPost, path, body); rec.Code != 403 {
		t.Fatalf("channel payment permission %d", rec.Code)
	}
}

func TestOrderMoneyCents(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		want  int64
		valid bool
	}{{".00", 0, true}, {"12.34", 1234, true}, {"1.2", 120, true}, {"0", 0, true}, {"1.234", 0, false}, {"-1", 0, false}, {"1e2", 0, false}, {"", 0, false}} {
		got, err := orderMoneyCents(tc.raw)
		if (err == nil) != tc.valid || (err == nil && got != tc.want) {
			t.Errorf("%q: %d %v", tc.raw, got, err)
		}
	}
}
