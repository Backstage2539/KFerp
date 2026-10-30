package sales

import (
	"context"
	"fmt"
	"sync"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func combinedPaymentFixture(t *testing.T) (*Repository, func(int64) string) {
	t.Helper()
	ctx := context.Background()
	pool, schema := newSalesPostgresTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.orders(id BIGINT PRIMARY KEY,customer_id BIGINT,order_no TEXT,pay_status_id BIGINT,ship_status_id BIGINT,payment_method TEXT DEFAULT '',prepayment_amount NUMERIC DEFAULT 0,grand_total NUMERIC,total_amount NUMERIC,shipping_amount NUMERIC DEFAULT 0,notes TEXT DEFAULT '',is_void BOOLEAN DEFAULT false,confirmation_required BOOLEAN DEFAULT false,confirmation_status TEXT DEFAULT 'accepted',settlement_reason TEXT DEFAULT '',delivery_mode TEXT DEFAULT 'shipment',is_sample_order BOOLEAN DEFAULT false,payment_goods_amount NUMERIC DEFAULT 0,payment_shipping_amount NUMERIC DEFAULT 0,payment_voucher_asset_id BIGINT DEFAULT 0,logistics_company_id BIGINT DEFAULT 0,logistics_product_id BIGINT DEFAULT 0,ship_method TEXT DEFAULT 'pickup');
INSERT INTO %[1]s.orders(id,customer_id,order_no,pay_status_id,ship_status_id,grand_total,total_amount,shipping_amount,prepayment_amount,payment_goods_amount,payment_method,notes) VALUES
(11,5,'A',3,1,100,80,20,30,30,'现金','已有备注'),(12,5,'B',1,1,50,50,0,0,0,'','备注二'),(13,5,'C',2,1,80,80,0,0,0,'支付宝','已付备注');
CREATE TABLE %[1]s.order_items(id BIGINT,order_id BIGINT,price_source_json JSONB,qty NUMERIC,unit_price NUMERIC);
INSERT INTO %[1]s.order_items VALUES(1,11,'{}',1,80),(2,12,'{}',1,50),(3,13,'{}',1,80);
CREATE TABLE %[1]s.pay_statuses(id BIGINT,name TEXT);INSERT INTO %[1]s.pay_statuses VALUES(1,'未付款'),(2,'已付款'),(3,'预付款（付款未完成）'),(4,'已收款');
CREATE TABLE %[1]s.ship_statuses(id BIGINT,name TEXT);INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'已发货');
CREATE TABLE %[1]s.order_audit_logs(order_id BIGINT,actor TEXT,field TEXT,old_value TEXT,new_value TEXT);
CREATE TABLE %[1]s.audit_logs(actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
CREATE TABLE %[1]s.sales_order_assets(id BIGINT,kind TEXT,created_by TEXT);
CREATE TABLE %[1]s.sales_order_documents(order_id BIGINT,is_latest BOOLEAN);INSERT INTO %[1]s.sales_order_documents VALUES(11,true),(12,true),(13,true);
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureCombinedPaymentSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	revision := func(id int64) string {
		t.Helper()
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		v, e := currentOrderEditRevisionTx(ctx, tx, schema, id)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	return &repo, revision
}

func combinedFixtureCommand(revision func(int64) string) salesapp.CombinedOrderPaymentCommand {
	return salesapp.CombinedOrderPaymentCommand{Orders: []salesapp.CombinedPaymentOrder{{OrderID: 11, EditRevision: revision(11)}, {OrderID: 12, EditRevision: revision(12)}, {OrderID: 13, EditRevision: revision(13)}}, ActualCents: 12500, ExpectedDueCents: 12000, PaymentMethod: "微信支付", Actor: "tester", IdempotencyKey: "key-1"}
}

func TestCombinedOrderPaymentAllocationAndIdempotency(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	cmd := combinedFixtureCommand(rev)
	result, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.CustomerID != 5 || result.DueCents != 12000 || result.ActualCents != 12500 || result.UnallocatedCents != 500 || len(result.Allocations) != 2 || result.Allocations[0].GoodsCents != 5000 || result.Allocations[0].ShippingCents != 2000 || result.Allocations[1].AmountCents != 5000 {
		t.Fatalf("bad allocation: %+v", result)
	}
	for _, tc := range []struct {
		id                   int64
		pay                  string
		pre, goods, shipping int64
		method, note         string
	}{{11, "已付款", 10000, 8000, 2000, "微信支付", "已有备注\n合并付款"}, {12, "已付款", 5000, 5000, 0, "微信支付", "备注二\n合并付款"}, {13, "已付款", 0, 0, 0, "支付宝", "已付备注"}} {
		var name, method, note string
		var pre, goods, shipping int64
		err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT p.name,round(o.prepayment_amount*100)::bigint,round(o.payment_goods_amount*100)::bigint,round(o.payment_shipping_amount*100)::bigint,o.payment_method,o.notes FROM %s.orders o JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1`, repo.schema, repo.schema), tc.id).Scan(&name, &pre, &goods, &shipping, &method, &note)
		if err != nil || name != tc.pay || pre != tc.pre || goods != tc.goods || shipping != tc.shipping || method != tc.method || note != tc.note {
			t.Fatalf("order %d: %s %d %d %d %s %s err=%v", tc.id, name, pre, goods, shipping, method, note, err)
		}
	}
	retry, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil || !retry.Reused || retry.BatchID != result.BatchID {
		t.Fatalf("retry=%+v %v", retry, err)
	}
	cmd.ActualCents++
	if _, err = repo.CombinedOrderPayment(ctx, cmd); err == nil {
		t.Fatal("same key with different receipt accepted")
	}
	entries, err := repo.OrderCombinedPayments(ctx, 11)
	if err != nil || len(entries) != 1 || entries[0].UnallocatedCents != 500 {
		t.Fatalf("history=%+v err=%v", entries, err)
	}
	var audit, records int
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='order_combined_payment' AND action='collect'`, repo.schema)).Scan(&audit); err != nil || audit != 1 {
		t.Fatalf("audit=%d err=%v", audit, err)
	}
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_combined_payments`, repo.schema)).Scan(&records); err != nil || records != 1 {
		t.Fatalf("records=%d err=%v", records, err)
	}
}

func TestCombinedOrderPaymentRollsBackAnyInvalidOrder(t *testing.T) {
	for _, mode := range []string{"other_customer", "void", "pending", "stale", "insufficient", "changed_due", "inconsistent_received", "shipping_for_retained", "free_sample", "wrong_voucher"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			repo, rev := combinedPaymentFixture(t)
			stmt := ""
			switch mode {
			case "other_customer":
				stmt = "UPDATE %s.orders SET customer_id=6 WHERE id=12"
			case "void":
				stmt = "UPDATE %s.orders SET is_void=true WHERE id=12"
			case "pending":
				stmt = "UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending' WHERE id=12"
			case "inconsistent_received":
				stmt = "UPDATE %s.orders SET payment_goods_amount=10 WHERE id=12"
			case "shipping_for_retained":
				stmt = "UPDATE %s.orders SET delivery_mode='customer_stock' WHERE id=12"
			case "free_sample":
				stmt = "UPDATE %s.orders SET settlement_reason='free_sample' WHERE id=12"
			}
			if stmt != "" {
				if _, err := repo.pool.Exec(ctx, fmt.Sprintf(stmt, repo.schema)); err != nil {
					t.Fatal(err)
				}
			}
			cmd := combinedFixtureCommand(rev)
			switch mode {
			case "stale":
				cmd.Orders[1].EditRevision = "stale"
			case "insufficient":
				cmd.ActualCents = 11999
			case "changed_due":
				cmd.ExpectedDueCents = 11999
			case "shipping_for_retained":
				v := int64(2)
				cmd.ShipStatusID = &v
			case "wrong_voucher":
				cmd.PaymentVoucherAssetID = 999
			}
			_, err := repo.CombinedOrderPayment(ctx, cmd)
			if err == nil {
				t.Fatalf("%s accepted", mode)
			}
			var records, order11, changedNotes int
			if e := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_combined_payments),(SELECT pay_status_id FROM %[1]s.orders WHERE id=11),(SELECT count(*) FROM %[1]s.orders WHERE notes LIKE '%%合并付款%%')`, repo.schema)).Scan(&records, &order11, &changedNotes); e != nil {
				t.Fatal(e)
			}
			if records != 0 || order11 != 3 || changedNotes != 0 {
				t.Fatalf("partial write: records=%d pay=%d notes=%d", records, order11, changedNotes)
			}
		})
	}
}

func TestCombinedOrderPaymentConcurrentRetry(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	cmd := combinedFixtureCommand(rev)
	var wg sync.WaitGroup
	results := make(chan salesapp.CombinedOrderPaymentResult, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := repo.CombinedOrderPayment(ctx, cmd); results <- r; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var first salesapp.CombinedOrderPaymentResult
	count := 0
	for r := range results {
		if count == 0 {
			first = r
		} else if r.BatchID != first.BatchID || r.Reused == first.Reused {
			t.Fatalf("results: %+v %+v", first, r)
		}
		count++
	}
	var n int
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_combined_payments`, repo.schema)).Scan(&n); err != nil || n != 1 {
		t.Fatalf("records=%d err=%v", n, err)
	}
}

func TestCombinedPaymentNoteKeepsExisting(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"", "合并付款"}, {"旧备注", "旧备注\n合并付款"}, {"旧备注\n合并付款", "旧备注\n合并付款"}, {"非合并付款", "非合并付款\n合并付款"}} {
		if got := appendCombinedPaymentNote(tc.in); got != tc.want {
			t.Errorf("%q => %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestCombinedOrderPaymentWithShippingAndLegacyPrepayment(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET payment_goods_amount=0 WHERE id=11`, repo.schema)); err != nil {
		t.Fatal(err)
	}
	cmd := combinedFixtureCommand(rev)
	cmd.ActualCents = cmd.ExpectedDueCents
	ship := int64(2)
	cmd.ShipStatusID = &ship
	result, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.UnallocatedCents != 0 || result.Allocations[0].GoodsCents != 5000 || result.Allocations[0].ShippingCents != 2000 {
		t.Fatalf("legacy prepayment allocation: %+v", result)
	}
	var count int
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.orders WHERE ship_status_id=2`, repo.schema)).Scan(&count); err != nil || count != 3 {
		t.Fatalf("shipping updates count=%d err=%v", count, err)
	}
	var method, note string
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_method,notes FROM %s.orders WHERE id=13`, repo.schema)).Scan(&method, &note); err != nil || method != "支付宝" || note != "已付备注" {
		t.Fatalf("paid order changed method=%q note=%q err=%v", method, note, err)
	}
}

func TestCombinedOrderPaymentDiscountAndFreightAllocation(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	// A merchandise discount and fees have already been reflected in grand_total.
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET total_amount=100,grand_total=95,shipping_amount=20,prepayment_amount=25,payment_goods_amount=20,payment_shipping_amount=5 WHERE id=11`, repo.schema)); err != nil {
		t.Fatal(err)
	}
	cmd := combinedFixtureCommand(rev)
	cmd.ActualCents, cmd.ExpectedDueCents = 12000, 12000
	result, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Allocations[0]; got.GoodsCents != 5500 || got.ShippingCents != 1500 || got.AmountCents != 7000 {
		t.Fatalf("discount allocation: %+v", got)
	}
	var goods, shipping int64
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(payment_goods_amount*100)::bigint,round(payment_shipping_amount*100)::bigint FROM %s.orders WHERE id=11`, repo.schema)).Scan(&goods, &shipping); err != nil || goods != 7500 || shipping != 2000 {
		t.Fatalf("discount receipt: goods=%d shipping=%d err=%v", goods, shipping, err)
	}
}

func TestCombinedOrderPaymentConcurrentDifferentKeys(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	first := combinedFixtureCommand(rev)
	second := first
	second.Orders = append([]salesapp.CombinedPaymentOrder(nil), first.Orders...)
	// Reverse selection to exercise the consistent lock order.
	second.Orders[0], second.Orders[2] = second.Orders[2], second.Orders[0]
	second.IdempotencyKey = "another-key"
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, cmd := range []salesapp.CombinedOrderPaymentCommand{first, second} {
		wg.Add(1)
		go func(c salesapp.CombinedOrderPaymentCommand) {
			defer wg.Done()
			_, err := repo.CombinedOrderPayment(ctx, c)
			errs <- err
		}(cmd)
	}
	wg.Wait()
	close(errs)
	ok, conflict := 0, 0
	for err := range errs {
		if err == nil {
			ok++
		} else if _, yes := salesapp.CombinedPaymentValidationMessage(err); yes {
			conflict++
		} else {
			t.Fatalf("unexpected concurrency error: %v", err)
		}
	}
	var records int
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_combined_payments`, repo.schema)).Scan(&records); err != nil || ok != 1 || conflict != 1 || records != 1 {
		t.Fatalf("ok=%d conflict=%d records=%d err=%v", ok, conflict, records, err)
	}
}

func TestCombinedOrderPaymentPreservesEarlierVoucher(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.sales_order_assets VALUES(1,'payment_voucher','other'),(2,'payment_voucher','tester'),(3,'payment_voucher','other'); UPDATE %s.orders SET payment_voucher_asset_id=1 WHERE id=11`, repo.schema, repo.schema)); err != nil {
		t.Fatal(err)
	}
	cmd := combinedFixtureCommand(rev)
	cmd.PaymentVoucherAssetID = 3
	if _, err := repo.CombinedOrderPayment(ctx, cmd); err == nil {
		t.Fatal("another operator's voucher was accepted")
	}
	cmd.PaymentVoucherAssetID = 2
	result, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.PaymentVoucherAssetID != 2 {
		t.Fatalf("batch voucher: %+v", result)
	}
	var oldVoucher, newVoucher, alreadyPaidVoucher int64
	if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT payment_voucher_asset_id FROM %[1]s.orders WHERE id=11),(SELECT payment_voucher_asset_id FROM %[1]s.orders WHERE id=12),(SELECT payment_voucher_asset_id FROM %[1]s.orders WHERE id=13)`, repo.schema)).Scan(&oldVoucher, &newVoucher, &alreadyPaidVoucher); err != nil || oldVoucher != 1 || newVoucher != 2 || alreadyPaidVoucher != 0 {
		t.Fatalf("vouchers old=%d new=%d paid=%d err=%v", oldVoucher, newVoucher, alreadyPaidVoucher, err)
	}
	entries, err := repo.OrderCombinedPayments(ctx, 11)
	if err != nil || len(entries) != 1 || entries[0].PaymentVoucherAssetID != 2 {
		t.Fatalf("history=%+v err=%v", entries, err)
	}
}

func TestCombinedOrderPaymentPaidOrderKeepsOthersVoucherWhenShipping(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.sales_order_assets VALUES(1,'payment_voucher','other'); UPDATE %s.orders SET payment_voucher_asset_id=1 WHERE id=13`, repo.schema, repo.schema)); err != nil {
		t.Fatal(err)
	}
	cmd := combinedFixtureCommand(rev)
	ship := int64(2)
	cmd.ShipStatusID = &ship
	cmd.PaymentVoucherAssetID = 1
	if _, err := repo.CombinedOrderPayment(ctx, cmd); err == nil {
		t.Fatal("other operator's voucher accepted for this receipt")
	}
	cmd.PaymentVoucherAssetID = 0
	if _, err := repo.CombinedOrderPayment(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var voucher, status int64
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_voucher_asset_id,ship_status_id FROM %s.orders WHERE id=13`, repo.schema)).Scan(&voucher, &status); err != nil || voucher != 1 || status != 2 {
		t.Fatalf("voucher=%d shipping=%d err=%v", voucher, status, err)
	}
}

func TestCombinedOrderPaymentRejectsLaterReceiptWithoutChangingIt(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	cmd := combinedFixtureCommand(rev)
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=40,payment_goods_amount=40 WHERE id=11`, repo.schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CombinedOrderPayment(ctx, cmd); err == nil {
		t.Fatal("stale balance was accepted")
	}
	var paid, goods int64
	var records int
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint,round(payment_goods_amount*100)::bigint,(SELECT count(*) FROM %[1]s.order_combined_payments) FROM %[1]s.orders WHERE id=11`, repo.schema)).Scan(&paid, &goods, &records); err != nil || paid != 4000 || goods != 4000 || records != 0 {
		t.Fatalf("receipt paid=%d goods=%d records=%d err=%v", paid, goods, records, err)
	}
}

func TestCombinedOrderPaymentHistoricalReceivable(t *testing.T) {
	for _, tc := range []struct {
		name, grand, discount, shipping, rounding string
		valid                                     bool
	}{
		{"legacy_zero", "0", "0", "0", "0", true},
		{"legacy_null", "NULL", "0", "0", "0", true},
		{"discounted_zero", "0", "50", "0", "0", false},
		{"rounded_zero", "0", "0", "0", "-50", false},
		{"shipping_zero", "0", "0", "10", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo, rev := combinedPaymentFixture(t)
			_, err := repo.pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN discount_amount NUMERIC DEFAULT 0, ADD COLUMN rounding_amount NUMERIC DEFAULT 0; UPDATE %[1]s.orders SET grand_total=%[2]s,total_amount=50,discount_amount=%[3]s,shipping_amount=%[4]s,rounding_amount=%[5]s WHERE id=12`, repo.schema, tc.grand, tc.discount, tc.shipping, tc.rounding))
			if err != nil {
				t.Fatal(err)
			}
			cmd := combinedFixtureCommand(rev)
			result, err := repo.CombinedOrderPayment(ctx, cmd)
			if !tc.valid {
				if err == nil {
					t.Fatalf("calculated zero was treated as legacy: %+v", result)
				}
				var n int
				if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.order_combined_payments`, repo.schema)).Scan(&n); err != nil || n != 0 {
					t.Fatalf("writes=%d err=%v", n, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.DueCents != 12000 || result.Allocations[1].GoodsCents != 5000 {
				t.Fatalf("history allocation: %+v", result)
			}
			var total, paid, status string
			if err = repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(grand_total::text,'NULL'),prepayment_amount::text,(SELECT name FROM %[1]s.pay_statuses WHERE id=o.pay_status_id) FROM %[1]s.orders o WHERE id=12`, repo.schema)).Scan(&total, &paid, &status); err != nil || (total != tc.grand && !(tc.grand == "NULL" && total == "NULL")) || paid != "50" || status != "已付款" {
				t.Fatalf("stored total=%s paid=%s status=%s err=%v", total, paid, status, err)
			}
		})
	}
}

func TestCombinedOrderPaymentHistoricalPartialPrepayment(t *testing.T) {
	ctx := context.Background()
	repo, rev := combinedPaymentFixture(t)
	if _, err := repo.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET grand_total=0,total_amount=50,prepayment_amount=20,payment_goods_amount=20,pay_status_id=3 WHERE id=12`, repo.schema)); err != nil {
		t.Fatal(err)
	}
	cmd := combinedFixtureCommand(rev)
	cmd.ExpectedDueCents, cmd.ActualCents = 10000, 10000
	result, err := repo.CombinedOrderPayment(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if result.Allocations[1].AmountCents != 3000 {
		t.Fatalf("partial history allocation=%+v", result.Allocations[1])
	}
	var prepay, goods int64
	if err := repo.pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(prepayment_amount*100)::bigint,round(payment_goods_amount*100)::bigint FROM %s.orders WHERE id=12`, repo.schema)).Scan(&prepay, &goods); err != nil || prepay != 5000 || goods != 5000 {
		t.Fatalf("prepayment=%d goods=%d err=%v", prepay, goods, err)
	}
}
