package sales

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	freightdomain "orderapp/internal/domain/freight"
	freightpg "orderapp/internal/infrastructure/postgres/freight"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
)

// Exercise the full HTTP -> SaveOrder -> PostgreSQL path with freight-v2
// snapshots. Every order and receipt here belongs to the isolated test schema.
func TestOrderFreightPaymentSaveAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.customer_order_production_demands(id bigserial PRIMARY KEY,order_id bigint,line_no int,customer_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,target_qty numeric,material_source_mode text,status text,migrated_at timestamptz,migration_note text DEFAULT '',created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now(),UNIQUE(order_id,line_no))`, schema))
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.shipping_fee_templates(id bigint,name text,active boolean,revision bigint,packaging_weight_g bigint,rules_json jsonb);
CREATE TABLE %[1]s.customer_fulfillment_settings(customer_id bigint,modes_json jsonb);
CREATE TABLE %[1]s.customer_fee_items(source_type text,source_id bigint);
INSERT INTO %[1]s.pay_statuses(id,name) VALUES(71001,'预付款（付款未完成）'),(71002,'已收款');
`, schema))
	fr := freightpg.NewRepository(pool, schema)
	if err := fr.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	e := newOrderAPITestEcho(pool, schema)
	request := func(t *testing.T, payload map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/order", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	quote := func(t *testing.T, id int64, previous string, amount int64) freightdomain.Snapshot {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		q, err := fr.QuoteOrderTx(ctx, tx, 3, id, previous, freightdomain.Destination{}, []freightpg.WeightItem{{ProductID: 7, BomSpecID: 9001, BomVariantID: 9101, Qty: 2}}, &freightdomain.Override{ManualAmountCents: &amount})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	for _, tc := range []struct {
		name                     string
		price, deposit, discount float64
		status, nextStatus       int64
		legacy                   bool
		shipping                 int64
		wantStatus               int64
		wantPrepay, wantTotal    float64
	}{
		{"legacy_paid_add_freight", 60, 0, 0, 2, 2, true, 2000, 71001, 120, 140},
		{"partial_to_paid", 50, 30, 0, 71001, 2, false, 0, 2, 30, 100},
		{"explicit_receipt_settlement", 50, 30, 0, 71001, 71001, false, 0, 71001, 30, 100},
		{"fully_discounted_add_freight", 60, 0, 120, 2, 2, false, 2000, 1, 0, 20},
		{"paid_same_total_preserves_no_deposit", 60, 0, 20, 2, 2, false, 2000, 2, 0, 100},
		{"split_freight_then_settlement", 50, 30, 0, 71001, 71001, false, 0, 71001, 30, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := quote(t, 0, "", 0)
			payload := map[string]any{"order_date": "2026-10-22", "customer_id": 3, "pay_status_id": tc.status, "payment_method": "现金", "prepayment_amount": fmt.Sprint(tc.deposit), "discount_amount": fmt.Sprint(tc.discount), "ship_status_id": 1,
				"product_id": []string{"7"}, "parent_product_id": []string{"7"}, "bom_spec_id": []string{"9001"}, "bom_variant_id": []string{"9101"}, "item_name": []string{"付款回归测试"}, "tier_id": []string{"manual"}, "unit_price": []string{fmt.Sprint(tc.price)}, "qty": []string{"2"},
				"freight_protocol": freightdomain.OrderProtocol, "freight_quote_token": q.Token, "freight_override": map[string]any{"manual_amount_cents": 0}, "shipping_amount": "0"}
			rec := request(t, payload)
			if rec.Code != http.StatusOK {
				t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
			}
			var result struct {
				ID int64 `json:"order_id"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.ID <= 0 {
				t.Fatalf("create response: %s err=%v", rec.Body.String(), err)
			}
			previous := q.Token
			if tc.legacy {
				// Recreate the supported historical storage shape, without a freight snapshot.
				mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %s.order_freight_snapshots WHERE order_id=%d; UPDATE %s.orders SET grand_total=0 WHERE id=%d`, schema, result.ID, schema, result.ID))
				previous = ""
			}
			q = quote(t, result.ID, previous, tc.shipping)
			payload["edit_id"] = result.ID
			payload["pay_status_id"] = tc.nextStatus
			payload["expected_freight_token"] = previous
			payload["freight_quote_token"] = q.Token
			payload["freight_override"] = map[string]any{"manual_amount_cents": tc.shipping}
			payload["shipping_amount"] = fmt.Sprint(float64(tc.shipping) / 100)
			if tc.name == "paid_same_total_preserves_no_deposit" {
				payload["discount_amount"] = "40"
			}
			rec = request(t, payload)
			if rec.Code != http.StatusOK {
				t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
			}
			var status, voucher int64
			var prepay, total, goods, receivedShipping float64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id,prepayment_amount::float8,grand_total::float8,COALESCE(payment_goods_amount,0)::float8,COALESCE(payment_shipping_amount,0)::float8,COALESCE(payment_voucher_asset_id,0) FROM %s.orders WHERE id=$1`, schema), result.ID).Scan(&status, &prepay, &total, &goods, &receivedShipping, &voucher); err != nil {
				t.Fatal(err)
			}
			if status != tc.wantStatus || prepay != tc.wantPrepay || total != tc.wantTotal || goods != 0 || receivedShipping != 0 || voucher != 0 {
				t.Fatalf("saved status=%d prepay=%v total=%v receipt=%v/%v voucher=%d, want %d/%v/%v and no invented receipt", status, prepay, total, goods, receivedShipping, voucher, tc.wantStatus, tc.wantPrepay, tc.wantTotal)
			}
			var audits int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='update'`, schema), result.ID).Scan(&audits); err != nil || audits == 0 {
				t.Fatalf("missing operation log count=%d err=%v", audits, err)
			}
			if tc.name == "split_freight_then_settlement" {
				prepareFreight := func(amount int64) {
					t.Helper()
					var token string
					if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json->>'token' FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.ID).Scan(&token); err != nil {
						t.Fatal(err)
					}
					q = quote(t, result.ID, token, amount)
					payload["expected_freight_token"] = token
					payload["freight_quote_token"] = q.Token
					payload["freight_override"] = map[string]any{"manual_amount_cents": amount}
					payload["shipping_amount"] = fmt.Sprint(float64(amount) / 100)
				}
				storedState := func() string {
					t.Helper()
					var state string
					if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('order',to_jsonb(o),'freight',f.snapshot_json,'audits',(SELECT count(*) FROM %[1]s.audit_logs WHERE entity_type='order' AND entity_id=o.id))::text FROM %[1]s.orders o JOIN %[1]s.order_freight_snapshots f ON f.order_id=o.id WHERE o.id=$1`, schema), result.ID).Scan(&state); err != nil {
						t.Fatal(err)
					}
					return state
				}
				before := storedState()
				prepareFreight(2000)
				payload["pay_status_id"] = int64(2)
				rec = request(t, payload)
				if rec.Code != http.StatusConflict {
					t.Fatalf("mixed collection/freight: %d %s", rec.Code, rec.Body.String())
				}
				for _, guidance := range []string{"分两次保存", "保留原付款信息", "重新打开订单", "实际累计收款"} {
					if !strings.Contains(rec.Body.String(), guidance) {
						t.Fatalf("missing actionable guidance %q: %s", guidance, rec.Body.String())
					}
				}
				if before != storedState() {
					t.Fatal("rejected mixed save changed order, freight snapshot or audit")
				}
				// Follow the guidance: preserve the original payment while saving
				// the new fee, then reopen and explicitly acknowledge collection.
				payload["pay_status_id"] = int64(71001)
				rec = request(t, payload)
				if rec.Code != http.StatusOK {
					t.Fatalf("fee-only save: %d %s", rec.Code, rec.Body.String())
				}
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id,prepayment_amount::float8,grand_total::float8 FROM %s.orders WHERE id=$1`, schema), result.ID).Scan(&status, &prepay, &total); err != nil || status != 71001 || prepay != 30 || total != 120 {
					t.Fatalf("fee save must preserve 30/120 partial payment: %d/%v/%v err=%v", status, prepay, total, err)
				}
				prepareFreight(2000)
				payload["pay_status_id"] = int64(2)
				rec = request(t, payload)
				if rec.Code != http.StatusOK {
					t.Fatalf("payment-only save: %d %s", rec.Code, rec.Body.String())
				}
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id,prepayment_amount::float8,grand_total::float8,COALESCE(payment_goods_amount,0)::float8,COALESCE(payment_shipping_amount,0)::float8,COALESCE(payment_voucher_asset_id,0) FROM %s.orders WHERE id=$1`, schema), result.ID).Scan(&status, &prepay, &total, &goods, &receivedShipping, &voucher); err != nil || status != 2 || prepay != 30 || total != 120 || goods != 0 || receivedShipping != 0 || voucher != 0 {
					t.Fatalf("split settlement: %d/%v/%v receipts=%v/%v/%d err=%v", status, prepay, total, goods, receivedShipping, voucher, err)
				}
			}
			if tc.name == "explicit_receipt_settlement" {
				// Explicit receipt edits must persist the operator's amounts, and a
				// settlement inconsistent with those amounts must roll back.
				mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.sales_order_assets(id,kind,created_by) VALUES(71001,'payment_voucher','测试员')`, schema))
				var savedToken string
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json->>'token' FROM %s.order_freight_snapshots WHERE order_id=$1`, schema), result.ID).Scan(&savedToken); err != nil {
					t.Fatal(err)
				}
				payload["expected_freight_token"] = savedToken
				q = quote(t, result.ID, savedToken, 0)
				payload["freight_quote_token"] = q.Token
				payload["pay_status_id"] = int64(71001)
				payload["prepayment_amount"] = "50"
				payload["payment_goods_amount"] = "50"
				payload["payment_voucher_asset_id"] = int64(71001)
				rec = request(t, payload)
				if rec.Code != http.StatusOK {
					t.Fatalf("explicit receipt: %d %s", rec.Code, rec.Body.String())
				}
				var before, after string
				readOrder := func() string {
					t.Helper()
					var raw string
					if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT to_jsonb(o)::text FROM %s.orders o WHERE id=$1`, schema), result.ID).Scan(&raw); err != nil {
						t.Fatal(err)
					}
					return raw
				}
				before = readOrder()
				payload["pay_status_id"] = int64(71002)
				rec = request(t, payload)
				if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "累计实收金额") {
					t.Fatalf("inconsistent receipt: %d %s", rec.Code, rec.Body.String())
				}
				after = readOrder()
				if before != after {
					t.Fatal("rejected settlement changed the order")
				}
				payload["payment_goods_amount"] = "100"
				rec = request(t, payload)
				if rec.Code != http.StatusOK {
					t.Fatalf("settle receipt: %d %s", rec.Code, rec.Body.String())
				}
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT pay_status_id,prepayment_amount::float8,payment_goods_amount::float8,payment_voucher_asset_id FROM %s.orders WHERE id=$1`, schema), result.ID).Scan(&status, &prepay, &goods, &voucher); err != nil || status != 71002 || prepay != 50 || goods != 100 || voucher != 71001 {
					t.Fatalf("explicit settlement %d/%v/%v/%d err=%v", status, prepay, goods, voucher, err)
				}
				// Once actual freight is confirmed, full-form editing is still frozen.
				mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(carrier_id,tracking_no,order_id,customer_id,revision) VALUES(1,'PAYMENT-TEST',%d,3,1)`, schema, result.ID))
				payload["expected_freight_token"] = q.Token
				rec = request(t, payload)
				if rec.Code == http.StatusOK || !strings.Contains(rec.Body.String(), "已确认实际费用") {
					t.Fatalf("frozen order edit: %d %s", rec.Code, rec.Body.String())
				}
			}
		})
	}
}
