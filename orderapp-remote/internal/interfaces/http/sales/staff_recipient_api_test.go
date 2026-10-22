package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authzapp "orderapp/internal/application/authz"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"

	"github.com/labstack/echo/v4"
)

func TestStaffFulfillmentRecipientPatchPreservesOrderContent(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	if err := postgressales.EnsureCustomerOrderSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
ALTER TABLE %[1]s.orders ADD COLUMN fulfillment_mode TEXT NOT NULL DEFAULT '';
INSERT INTO %[1]s.ship_statuses(id,name) VALUES(9,'已发货');
INSERT INTO %[1]s.orders(id,order_no,customer_id,ship_status_id,process_status_id,pay_status_id,
 portal_service_code,fulfillment_mode,settlement_reason,receiver_name,receiver_phone,receiver_address,receiver_company,
 grand_total,payment_goods_amount,payment_shipping_amount,payment_voucher_asset_id,notes)
 VALUES(42,'SO-RECIPIENT-STOCK',3,1,1,2,'direct_ship','stock','prepaid_customer_stock','原收件人','13800000001','原地址','原公司',9,0,9,18,'保留备注');
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,
 unit_price,bean_list_publication_id,price_source_json,logistics_unit_weight_g,logistics_item_weight_g,weight_snapshot_json)
 VALUES(43,42,1,7,9001,9101,'客户库存商品','1KG袋装',2,'袋',0,0,'{"source":"prepaid_customer_stock","frozen":true}',1000,2000,'{"frozen":true}');
CREATE TABLE %[1]s.customer_direct_ship_request_orders(order_id BIGINT,status TEXT);
INSERT INTO %[1]s.customer_direct_ship_request_orders VALUES(42,'reserved');
CREATE TABLE %[1]s.customer_processing_output_reservations(order_id BIGINT,order_item_id BIGINT,reserved_qty NUMERIC,status TEXT);
INSERT INTO %[1]s.customer_processing_output_reservations VALUES(42,43,2,'reserved');
CREATE TABLE %[1]s.customer_fee_items(source_id BIGINT,fee_type TEXT,amount NUMERIC);
INSERT INTO %[1]s.customer_fee_items VALUES(42,'direct_ship_service',9);
CREATE TABLE %[1]s.order_freight_snapshots(order_id BIGINT,snapshot_json JSONB);
INSERT INTO %[1]s.order_freight_snapshots VALUES(42,'{"template_id":1,"template_revision":2,"estimated_total_cents":810,"frozen":true}');
CREATE TABLE %[1]s.sales_order_documents(order_id BIGINT,is_latest BOOLEAN);
INSERT INTO %[1]s.sales_order_documents VALUES(42,true),(99,true);
`, schema))
	authz := &orderAPIAuthzService{actor: authzapp.Actor{AccountType: "internal_employee", Permissions: []string{"orders.write"}}}
	scoped, authenticated := false, true
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if authenticated {
				c.Set("employee_id", int64(5))
				c.Set("actor", "销售小王")
			}
			c.Set("customer_fulfillment_order_scope_limited", scoped)
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc, nil, authz)

	revision := func() string {
		t.Helper()
		var value string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT md5(to_jsonb(o)::text || '|' || COALESCE((SELECT jsonb_agg(to_jsonb(i) ORDER BY i.id)::text FROM %[1]s.order_items i WHERE i.order_id=o.id),'[]')) FROM %[1]s.orders o WHERE id=42`, schema)).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	protected := func() string {
		t.Helper()
		var value string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object(
 'order',(SELECT to_jsonb(o)-ARRAY['receiver_name','receiver_phone','receiver_address','receiver_company'] FROM %[1]s.orders o WHERE id=42),
 'items',(SELECT jsonb_agg(to_jsonb(i) ORDER BY id) FROM %[1]s.order_items i WHERE order_id=42),
 'request_orders',(SELECT jsonb_agg(to_jsonb(r)) FROM %[1]s.customer_direct_ship_request_orders r),
 'reservations',(SELECT jsonb_agg(to_jsonb(r)) FROM %[1]s.customer_processing_output_reservations r),
 'fees',(SELECT jsonb_agg(to_jsonb(f)) FROM %[1]s.customer_fee_items f),
 'freight',(SELECT jsonb_agg(to_jsonb(f)) FROM %[1]s.order_freight_snapshots f)
)::text`, schema)).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	payload := func(rev string) map[string]any {
		return map[string]any{"edit_revision": rev, "receiver_name": "新收件人", "receiver_phone": "13800000002", "receiver_address": "新地址", "receiver_company": "新公司"}
	}
	request := func(t *testing.T, body map[string]any, expected int) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPatch, "/api/orders/42/recipient", strings.NewReader(string(raw)))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != expected {
			t.Fatalf("recipient PATCH=%d want %d: %s", w.Code, expected, w.Body.String())
		}
		return w
	}

	for _, mode := range []string{"stock", "on_demand"} {
		t.Run(mode, func(t *testing.T) {
			mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.orders SET fulfillment_mode='%[2]s',receiver_address='原地址' WHERE id=42; UPDATE %[1]s.sales_order_documents SET is_latest=true`, schema, mode))
			if mode == "on_demand" {
				mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.orders SET total_amount=66,grand_total=75,settlement_reason='' WHERE id=42; UPDATE %[1]s.order_items SET unit_price=33,line_total=66,bean_list_publication_id=88,price_source_json='{"source":"customer_price_table","frozen":true}' WHERE id=43`, schema))
			}
			before, oldRevision := protected(), revision()
			request(t, payload(oldRevision), http.StatusOK)
			if protected() != before {
				t.Fatal("recipient edit changed merchandise, fulfillment, finance, reservation or freight snapshot")
			}
			var name, phone, address, company string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT receiver_name,receiver_phone,receiver_address,receiver_company FROM %s.orders WHERE id=42`, schema)).Scan(&name, &phone, &address, &company); err != nil {
				t.Fatal(err)
			}
			if name != "新收件人" || phone != "13800000002" || address != "新地址" || company != "新公司" || revision() == oldRevision {
				t.Fatalf("recipient/revision not updated: %q/%q/%q/%q", name, phone, address, company)
			}
			var staleDocs, otherDocs int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER(WHERE order_id=42 AND is_latest),count(*) FILTER(WHERE order_id=99 AND is_latest) FROM %s.sales_order_documents`, schema)).Scan(&staleDocs, &otherDocs); err != nil || staleDocs != 0 || otherDocs != 1 {
				t.Fatalf("document invalidation=%d/%d err=%v", staleDocs, otherDocs, err)
			}
			request(t, payload(oldRevision), http.StatusConflict)
		})
	}
	var audits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=42 AND field='recipient' AND actor='销售小王' AND new_value LIKE '%%新公司%%'`, schema)).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("recipient audit count=%d err=%v", audits, err)
	}

	t.Run("omitted company preserves original", func(t *testing.T) {
		body := payload(revision())
		delete(body, "receiver_company")
		body["receiver_address"] = "再次修改地址"
		request(t, body, http.StatusOK)
		var company string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT receiver_company FROM %s.orders WHERE id=42`, schema)).Scan(&company); err != nil || company != "新公司" {
			t.Fatalf("omitted company lost: %q err=%v", company, err)
		}
	})
	for _, tc := range []struct {
		name, mutation string
		bodyKey        string
		bodyValue      any
		code           int
	}{
		{name: "missing revision", bodyKey: "edit_revision", bodyValue: "", code: 409},
		{name: "wrong customer", bodyKey: "customer_id", bodyValue: 999, code: 403},
		{name: "commercial field", bodyKey: "unit_price", bodyValue: []string{"0"}, code: 400},
		{name: "missing recipient", bodyKey: "receiver_address", bodyValue: "", code: 400},
		{name: "shipped", mutation: "ship_status_id=9", code: 409},
		{name: "void", mutation: "is_void=true", code: 409},
		{name: "ordinary order", mutation: "fulfillment_mode=''", code: 400},
		{name: "wrong service", mutation: "portal_service_code='product_order'", code: 400},
		{name: "pending confirmation", mutation: "confirmation_required=true,confirmation_status='pending'", code: 409},
		{name: "accepted managed confirmation", mutation: "confirmation_required=true,confirmation_status='accepted'", code: 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=1,is_void=false,fulfillment_mode='stock',portal_service_code='direct_ship',confirmation_required=false,confirmation_status='accepted' WHERE id=42`, schema))
			if tc.mutation != "" {
				mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET %s WHERE id=42`, schema, tc.mutation))
			}
			before, rev := protected(), revision()
			body := payload(rev)
			if tc.bodyKey != "" {
				body[tc.bodyKey] = tc.bodyValue
			}
			request(t, body, tc.code)
			if protected() != before || revision() != rev {
				t.Fatal("rejected request changed saved order")
			}
		})
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=false,confirmation_status='accepted' WHERE id=42`, schema))
	for _, mode := range []string{"customer scope", "channel customer", "no write permission", "unauthenticated"} {
		t.Run(mode, func(t *testing.T) {
			authz.actor = authzapp.Actor{AccountType: "internal_employee", Permissions: []string{"orders.write"}}
			scoped, authenticated = false, true
			code := 403
			switch mode {
			case "customer scope":
				scoped = true
			case "channel customer":
				authz.actor.AccountType = "channel_customer"
			case "no write permission":
				authz.actor.Permissions = []string{"orders.read"}
			case "unauthenticated":
				authenticated, code = false, 401
			}
			before := revision()
			request(t, payload(before), code)
			if revision() != before {
				t.Fatal("unauthorized request changed order")
			}
		})
	}
	scoped, authenticated = false, true
	authz.actor = authzapp.Actor{AccountType: "internal_employee", Permissions: []string{"orders.write"}}
	t.Run("shipment commits while recipient waits on order lock", func(t *testing.T) {
		body, err := json.Marshal(payload(revision()))
		if err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=9 WHERE id=42`, schema)); err != nil {
			t.Fatal(err)
		}
		done := make(chan int, 1)
		go func() {
			r := httptest.NewRequest(http.MethodPatch, "/api/orders/42/recipient", strings.NewReader(string(body)))
			r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			done <- w.Code
		}()
		select {
		case code := <-done:
			t.Fatalf("recipient ignored shipment row lock: %d", code)
		case <-time.After(100 * time.Millisecond):
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case code := <-done:
			if code != 409 {
				t.Fatalf("recipient accepted concurrent shipment: %d", code)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("recipient request did not finish")
		}
	})
}

func TestPortalRecipientPatchPreservesPendingCompanyWhenOmitted(t *testing.T) {
	pool, schema := newOrderAPITestDB(t)
	ctx := context.Background()
	seedOrderAPITestData(t, ctx, pool, schema)
	seedConfirmationExecutionFixtures(t, ctx, pool, schema)
	if err := postgressales.EnsureCustomerOrderSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecOrderAPITestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,ship_status_id,process_status_id,portal_service_code,
 receiver_name,receiver_phone,receiver_address,receiver_company)
 VALUES(42,'SO-PENDING-COMPANY',3,1,1,'product_order','原收件人','13800000001','原地址','已确认公司');
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,qty,unit,unit_price,line_total)
 VALUES(43,42,1,7,'原商品',2,'袋',33,66);
`, schema))
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("actor", "客户一")
			return next(c)
		}
	})
	svc := salesapp.NewService(postgressales.NewRepository(pool, schema))
	registerOrderAPI(e, svc, nil, workspaceTestScope{})
	for _, body := range []string{
		`{"receiver_name":"收件人","receiver_phone":"13800000002","receiver_address":"待确认地址","receiver_company":"待确认公司"}`,
		`{"receiver_name":"收件人","receiver_phone":"13800000002","receiver_address":"再次修改地址"}`,
	} {
		r := httptest.NewRequest(http.MethodPatch, "/api/customer-processing/portal/orders/42/recipient", strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("portal recipient PATCH=%d: %s", w.Code, w.Body.String())
		}
	}
	var accepted, pending, pendingAddress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT receiver_company,confirmation_pending->'order'->>'receiver_company',confirmation_pending->'order'->>'receiver_address' FROM %s.orders WHERE id=42`, schema)).Scan(&accepted, &pending, &pendingAddress); err != nil {
		t.Fatal(err)
	}
	if accepted != "已确认公司" || pending != "待确认公司" || pendingAddress != "再次修改地址" {
		t.Fatalf("portal company snapshots changed: accepted=%q pending=%q address=%q", accepted, pending, pendingAddress)
	}
}
