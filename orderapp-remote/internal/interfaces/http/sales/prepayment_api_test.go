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
	"testing"
)

func TestPrepaymentOrderAPIStoresPartialPaymentAndBalances(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedOrderAPIBOMSpecIdentity(t, ctx, pool, schema)
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN prepayment_amount NUMERIC(12,2) NOT NULL DEFAULT 0; ALTER TABLE %[1]s.order_items ADD COLUMN customer_product_reference_id BIGINT, ADD COLUMN material_source_mode TEXT; CREATE TABLE %[1]s.customer_order_production_demands(order_id BIGINT); INSERT INTO %s.pay_statuses(id,name) VALUES (9,'预付款（付款未完成）')`, schema, schema))
	e := newOrderAPITestEcho(pool, schema)
	for _, tc := range []struct {
		amount       string
		status, want int
	}{{"30", 9, 200}, {"0", 9, 400}, {"100", 9, 400}, {"101", 9, 400}, {"30", 1, 400}, {"30.123", 9, 400}, {"30", 2, 200}} {
		payload := fmt.Sprintf(`{"order_date":"2026-09-07","customer_id":3,"pay_status_id":%d,"payment_method":"微信","prepayment_amount":"%s","ship_status_id":1,"product_id":["7"],"bom_spec_id":["9002"],"bom_variant_id":["9102"],"item_name":["橘皮乌龙"],"tier_id":["manual"],"unit_price":["88"],"qty":["1"],"unit":["袋"],"spec":["454"],"shipping_amount":"12"}`, tc.status, tc.amount)
		req := httptest.NewRequest(http.MethodPost, "/api/order", strings.NewReader(payload))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("amount=%s status=%d: %d %s", tc.amount, tc.status, rec.Code, rec.Body.String())
		}
		if tc.want != 200 {
			continue
		}
		var response struct {
			OrderID int64 `json:"order_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		var deposit, total string
		var status int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT prepayment_amount::text,to_char(grand_total,'FM9999990.00'),pay_status_id FROM %s.orders WHERE id=$1`, schema), response.OrderID).Scan(&deposit, &total, &status); err != nil {
			t.Fatal(err)
		}
		if deposit != "30.00" || total != "100.00" || status != tc.status {
			t.Fatalf("saved %s %s %d", deposit, total, status)
		}
		var audit string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT new_value FROM %s.audit_logs WHERE entity_type='order' AND entity_id=$1 ORDER BY id DESC LIMIT 1`, schema), response.OrderID).Scan(&audit); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(audit, "prepayment_amount") || !strings.Contains(audit, "30") {
			t.Fatalf("deposit not logged: %s", audit)
		}
		repo := postgressales.NewRepository(pool, schema)
		page, err := repo.ListOrders(ctx, salesapp.OrderListQuery{OrderID: response.OrderID, Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		paid, unpaid := "30.00", "70.00"
		if tc.status == 2 {
			paid, unpaid = "100.00", "0.00"
		}
		if page.Summary.PaidAmount != paid || page.Summary.PendingSettlementAmount != unpaid {
			t.Fatalf("summary=%+v", page.Summary)
		}
		if len(page.Rows) != 1 || page.Rows[0].PaidAmount != paid || page.Rows[0].UnpaidAmount != unpaid {
			t.Fatalf("rows=%+v", page.Rows)
		}
	}
}
