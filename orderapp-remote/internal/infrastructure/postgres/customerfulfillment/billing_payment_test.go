package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	fulfillmenthttp "orderapp/internal/interfaces/http/customerfulfillment"
	"strings"
	"sync"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestBillingPaymentMixedOrdersFeeAndSafeReverse(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS prepayment_amount NUMERIC NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS payment_goods_amount NUMERIC NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS payment_shipping_amount NUMERIC NOT NULL DEFAULT 0; CREATE TABLE IF NOT EXISTS %[1]s.order_audit_logs(id BIGSERIAL PRIMARY KEY,order_id BIGINT,actor TEXT,field TEXT,old_value TEXT,new_value TEXT)`, schema)); err != nil {
		t.Fatal(err)
	}
	var customer, partial, unpaid, o1, o2, fee int64
	for _, item := range []struct {
		sql  string
		dest *int64
	}{{fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('吴客户') RETURNING id`, schema), &customer}, {fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='预付款（付款未完成）' LIMIT 1`, schema), &partial}, {fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='未付款' LIMIT 1`, schema), &unpaid}} {
		if err := pool.QueryRow(ctx, item.sql).Scan(item.dest); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,pay_status_id,grand_total,shipping_amount,prepayment_amount,payment_goods_amount,is_void) VALUES('A',$1,'2026-10-01',$2,100,0,40,40,false) RETURNING id`, schema), customer, partial).Scan(&o1); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,order_date,pay_status_id,grand_total,shipping_amount,is_void) VALUES('B',$1,'2026-10-01',$2,120,20,false) RETURNING id`, schema), customer, unpaid).Scan(&o2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'order',$2,'direct_ship_service',3,'2026-10-01','unsettled','代发费') RETURNING id`, schema), customer, o2).Scan(&fee); err != nil {
		t.Fatal(err)
	}
	previewCmd := app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "order_goods", SourceType: "order", SourceID: o1}, {ChargeType: "order_goods", SourceType: "order", SourceID: o2}, {ChargeType: "direct_ship_service", SourceType: "customer_fee_item", SourceID: fee}}}
	preview, err := repo.PreviewBilling(ctx, previewCmd)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: previewCmd, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "mixed"})
	if err != nil {
		t.Fatal(err)
	}
	if bill.DueAmountCents != 16300 {
		t.Fatalf("bill=%+v", bill)
	}
	expected := int64(16299)
	revision := int64(0)
	cmd := app.BillingPaymentCommand{CustomerID: customer, BatchID: bill.BatchID, Action: "pay", Actor: "test", IdempotencyKey: "pay-1", ExpectedDueAmountCents: &expected, ExpectedPaymentRevision: &revision, PaymentMethod: "微信"}
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err == nil {
		t.Fatal("stale amount accepted")
	}
	expected = 16300
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=50 WHERE id=$1`, schema), o1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ChangeBillingPayment(ctx, cmd); err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("inconsistent prepayment: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=40 WHERE id=$1`, schema), o1); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan app.BillingBill, 2)
	errors := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, e := repo.ChangeBillingPayment(ctx, cmd)
			results <- result
			errors <- e
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for e := range errors {
		if e != nil {
			t.Fatalf("concurrent pay: %v", e)
		}
	}
	var paid app.BillingBill
	for result := range results {
		paid = result
		if result.PaymentRevision != 1 {
			t.Fatalf("duplicate payment: %+v", result)
		}
	}

	if paid.DueAmountCents != 0 || !paid.PaymentReversible {
		t.Fatalf("paid=%+v", paid)
	}
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err != nil {
		t.Fatalf("repeat pay: %v", err)
	}
	var name string
	var pre, goods int64
	read := func(id int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.name,round(o.prepayment_amount*100)::bigint,round(o.payment_goods_amount*100)::bigint FROM %s.orders o JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, schema, schema), id).Scan(&name, &pre, &goods); err != nil {
			t.Fatal(err)
		}
	}
	read(o1)
	if name != "已付款" || goods != 10000 {
		t.Fatalf("order 1: %s %d %d", name, pre, goods)
	}
	read(o2)
	if name != "预付款（付款未完成）" || pre != 10000 || goods != 10000 {
		t.Fatalf("order 2: %s %d %d", name, pre, goods)
	}
	expected = 0
	cmd.Action = "reverse"
	cmd.IdempotencyKey = "reverse-1"
	cmd.PaymentMethod = ""
	revision = 1
	reversed, err := repo.ChangeBillingPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if reversed.DueAmountCents != 16300 || reversed.PaymentReversible {
		t.Fatalf("reverse=%+v", reversed)
	}
	read(o1)
	if name != "预付款（付款未完成）" || pre != 4000 || goods != 4000 {
		t.Fatalf("preserve prepayment: %s %d %d", name, pre, goods)
	}
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err != nil {
		t.Fatalf("repeat reverse: %v", err)
	}
	expected = 16300
	cmd.Action = "pay"
	cmd.IdempotencyKey = "pay-2"
	cmd.PaymentMethod = "微信"
	revision = 2
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	// A delayed repeat of the previous reversal must not undo the second payment.
	expected = 0
	cmd.Action = "reverse"
	cmd.IdempotencyKey = "reverse-1"
	cmd.PaymentMethod = ""
	revision = 1
	if _, err := repo.ChangeBillingPayment(ctx, cmd); err == nil {
		t.Fatal("old reverse replay must report changed state")
	}
	cmd.IdempotencyKey = "stale-reverse"
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err == nil {
		t.Fatal("old payment revision accepted")
	}
	// Even if amounts return to the same values, later audited payment edits block reversal.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,'other','order','before','after')`, schema), o2); err != nil {
		t.Fatal(err)
	}
	cmd.IdempotencyKey = "audit-reverse"
	revision = 3
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err == nil || !strings.Contains(err.Error(), "发生变化") {
		t.Fatalf("later audit reversal: %v", err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_audit_logs WHERE actor='other'`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET payment_shipping_amount=10,prepayment_amount=110 WHERE id=$1`, schema), o2); err != nil {
		t.Fatal(err)
	}
	expected = 0
	cmd.Action = "reverse"
	cmd.IdempotencyKey = "reverse-2"
	revision = 3
	if _, err = repo.ChangeBillingPayment(ctx, cmd); err == nil || !strings.Contains(err.Error(), "发生变化") {
		t.Fatalf("unsafe reversal: %v", err)
	}
	read(o2)
	if pre != 11000 {
		t.Fatalf("later payment changed %d", pre)
	}
}

func TestBillingPaymentHTTPValidationAndCustomerIsolation(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)
	var customer, other, fee int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('客户甲') RETURNING id`, schema)).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('客户乙') RETURNING id`, schema)).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_service_capabilities(customer_id,capability_code,enabled) VALUES($1,'settlement',true)`, schema), customer); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,occurred_at,status,note) VALUES($1,'freight_actual',1,'shipping',10,'2026-10-01','unsettled','运费') RETURNING id`, schema), customer).Scan(&fee); err != nil {
		t.Fatal(err)
	}
	pc := app.BillingPreviewCommand{CustomerID: customer, Sources: []app.BillingSource{{ChargeType: "shipping", SourceType: "customer_fee_item", SourceID: fee}}}
	preview, err := repo.PreviewBilling(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	bill, err := repo.IssueBilling(ctx, app.IssueBillingCommand{BillingPreviewCommand: pc, PreviewVersion: preview.PreviewVersion, IdempotencyKey: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("operator_employee", "测试员工"); return next(c) }
	})
	fulfillmenthttp.RegisterRoutes(e, fulfillmenthttp.Dependencies{CustomerFulfillment: app.NewService(repo)})
	call := func(cid int64, body string, want int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/customer-fulfillment/%d/billing/bills/%d/payment", cid, bill.BatchID), strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s status=%d body=%s", body, rec.Code, rec.Body.String())
		}
	}
	call(customer, `{"action":"pay","idempotency_key":"p","expected_due_amount_cents":1000,"expected_payment_revision":0}`, 409)
	call(customer, `{"action":"pay","idempotency_key":"p","payment_method":"微信","expected_due_amount_cents":999,"expected_payment_revision":0}`, 409)
	call(other, `{"action":"pay","idempotency_key":"p","payment_method":"微信","expected_due_amount_cents":1000,"expected_payment_revision":0}`, 409)
	valid := `{"action":"pay","idempotency_key":"p","payment_method":"微信","expected_due_amount_cents":1000,"expected_payment_revision":0}`
	call(customer, valid, 200)
	call(customer, valid, 200)
	call(customer, `{"action":"reverse","idempotency_key":"r","expected_due_amount_cents":0,"expected_payment_revision":0}`, 409)
	call(customer, `{"action":"reverse","idempotency_key":"r","expected_due_amount_cents":0,"expected_payment_revision":1}`, 200)
	// Old paid records without this feature's allocation journal cannot be undone.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status='paid',paid_at=now() WHERE id=$1`, schema), bill.BatchID); err != nil {
		t.Fatal(err)
	}
	call(customer, `{"action":"reverse","idempotency_key":"legacy","expected_due_amount_cents":0,"expected_payment_revision":2}`, 409)
}
