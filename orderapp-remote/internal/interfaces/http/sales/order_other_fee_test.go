package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrderOtherFeeAPI(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedEditableOrderAPIFixture(t, ctx, pool, schema)
	e := newOrderAPITestEcho(pool, schema)
	payload := map[string]any{"order_date": "2026-05-22", "customer_id": 3, "pay_status_id": 1, "ship_status_id": 1, "product_id": []string{"7"}, "tier_id": []string{"manual"}, "unit_price": []string{"88"}, "qty": []string{"1"}, "bom_spec_id": []string{"9001"}, "parent_product_id": []string{"7"}, "bom_variant_id": []string{"9101"}, "item_name": []string{"橘皮乌龙"}, "discount_amount": "3.00", "other_fee": 12.34, "other_fee_note": "  加急  "}
	post := func() *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/api/order", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		e.ServeHTTP(res, req)
		return res
	}
	res := post()
	if res.Code != 200 {
		t.Fatalf("create: %d %s", res.Code, res.Body.String())
	}
	var id struct {
		ID int64 `json:"order_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &id); err != nil {
		t.Fatal(err)
	}
	check := func(fee, note, grand string) {
		t.Helper()
		var gotFee, gotNote, gotGrand string
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT to_jsonb(o)->>'other_fee',to_jsonb(o)->>'other_fee_note',grand_total::numeric(12,2)::text FROM %s.orders o WHERE id=$1", schema), id.ID).Scan(&gotFee, &gotNote, &gotGrand); err != nil {
			t.Fatal(err)
		}
		if gotFee != fee || gotNote != note || gotGrand != grand {
			t.Fatalf("fee/note/total = %s/%s/%s, want %s/%s/%s", gotFee, gotNote, gotGrand, fee, note, grand)
		}
	}
	check("12.34", "加急", "97.34")
	payload["edit_id"] = id.ID
	delete(payload, "other_fee")
	delete(payload, "other_fee_note")
	if res = post(); res.Code != 200 {
		t.Fatalf("legacy edit: %d %s", res.Code, res.Body.String())
	}
	check("12.34", "加急", "97.34")
	for _, bad := range []string{"-1", "1.001", "NaN", "10000000000"} {
		payload["other_fee"] = bad
		if res = post(); res.Code != 400 {
			t.Fatalf("invalid fee %q: %d %s", bad, res.Code, res.Body.String())
		}
		check("12.34", "加急", "97.34")
	}
	// An issued bill freezes the charge. Both amount and public explanation belong to the snapshot.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.customer_settlement_lines(source_id bigint,source_type text,charge_type text,settlement_batch_id bigint,reversed_at timestamptz);
 CREATE TABLE %[1]s.customer_settlement_batches(id bigint,status text,processing_billing_run_id bigint);
 INSERT INTO %[1]s.customer_settlement_batches VALUES(9,'confirmed',0);
 INSERT INTO %[1]s.customer_settlement_lines VALUES(%[2]d,'order','order_goods',9,NULL);`, schema, id.ID))
	payload["other_fee"] = "10"
	if res = post(); res.Code != 409 || !strings.Contains(res.Body.String(), "出账") {
		t.Fatalf("issued fee: %d %s", res.Code, res.Body.String())
	}
	check("12.34", "加急", "97.34")
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.customer_settlement_lines SET reversed_at=now()", schema))
	payload["other_fee"] = "0.00"
	if res = post(); res.Code != 200 {
		t.Fatalf("clear: %d %s", res.Code, res.Body.String())
	}
	check("0.00", "", "85.00")
	var feeAudit bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 AND action='update'
 AND old_value::jsonb->>'other_fee_note'='加急' AND (new_value::jsonb->>'other_fee')::numeric=0)`, schema), id.ID).Scan(&feeAudit); err != nil || !feeAudit {
		t.Fatalf("fee change missing in audit: %v", err)
	}
	// Adding a fee to a previously paid order must carry forward exactly the money collected.
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS prepayment_amount NUMERIC DEFAULT 0;
 INSERT INTO %[1]s.pay_statuses(id,name) VALUES(3,'预付款（付款未完成）');
 INSERT INTO %[1]s.sales_order_assets(id,kind,created_by) VALUES(9,'payment_voucher','测试员');
 UPDATE %[1]s.orders SET pay_status_id=2,payment_method='微信支付',payment_goods_amount=85,payment_voucher_asset_id=9 WHERE id=%[2]d`, schema, id.ID))
	payload["payment_method"] = "微信支付"
	payload["payment_goods_amount"] = "85"
	payload["payment_voucher_asset_id"] = 9
	payload["pay_status_id"] = 2
	payload["other_fee"] = "2.00"
	if res = post(); res.Code != 200 {
		t.Fatalf("paid order fee increase: %d %s", res.Code, res.Body.String())
	}
	var status int64
	var paid float64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT pay_status_id,prepayment_amount FROM %s.orders WHERE id=$1", schema), id.ID).Scan(&status, &paid); err != nil || status != 3 || paid != 85 {
		t.Fatalf("paid order now %d/%.2f: %v", status, paid, err)
	}
	check("2.00", "", "87.00")
}
