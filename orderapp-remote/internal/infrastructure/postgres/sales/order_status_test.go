package sales

import (
	"context"
	"fmt"
	"strings"
	"testing"

	salesapp "orderapp/internal/application/sales"
)

func TestOrderStatusUpdatePreservesContentAndRecordsAudit(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.orders(id BIGINT PRIMARY KEY,order_no TEXT,pay_status_id BIGINT,ship_status_id BIGINT,payment_method TEXT,prepayment_amount NUMERIC,grand_total NUMERIC,total_amount NUMERIC,notes TEXT,is_void BOOLEAN,confirmation_required BOOLEAN,confirmation_status TEXT,settlement_reason TEXT,is_sample_order BOOLEAN,payment_goods_amount NUMERIC,payment_shipping_amount NUMERIC,payment_voucher_asset_id BIGINT,logistics_company_id BIGINT,logistics_product_id BIGINT,ship_method TEXT);
INSERT INTO %[1]s.orders VALUES(42,'SO-42',1,2,'',0,100,100,'keep notes',false,false,'accepted','',false,0,0,0,0,0,'pickup');
CREATE TABLE %[1]s.order_items(id BIGINT,order_id BIGINT,price_source_json JSONB,qty NUMERIC,unit_price NUMERIC);
INSERT INTO %[1]s.order_items VALUES(51,42,'{"frozen":true}',2,50);
CREATE TABLE %[1]s.pay_statuses(id BIGINT,name TEXT); INSERT INTO %[1]s.pay_statuses VALUES(1,'未付款'),(2,'已付款'),(3,'预付款（付款未完成）'),(4,'已收款');
CREATE TABLE %[1]s.ship_statuses(id BIGINT,name TEXT); INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'已发货');
CREATE TABLE %[1]s.order_audit_logs(order_id BIGINT,actor TEXT,field TEXT,old_value TEXT,new_value TEXT);
CREATE TABLE %[1]s.audit_logs(actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
CREATE TABLE %[1]s.sales_order_assets(id BIGINT,kind TEXT,created_by TEXT); INSERT INTO %[1]s.sales_order_assets VALUES(12,'payment_voucher','mini-employee:7:test'),(13,'payment_voucher','another-employee');
CREATE TABLE %[1]s.sales_order_documents(order_id BIGINT,is_latest BOOLEAN); INSERT INTO %[1]s.sales_order_documents VALUES(42,true),(99,true),(42,false);
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	revision := func() string {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		v, e := currentOrderEditRevisionTx(ctx, tx, schema, 42)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	var beforeItems string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(i))::text FROM %s.order_items i`, schema)).Scan(&beforeItems); err != nil {
		t.Fatal(err)
	}
	cmd := salesapp.UpdateOrderStatusCommand{OrderID: 42, PayStatusID: 2, ShipStatusID: 2, PaymentMethod: "现金", ExpectedEditRevision: revision(), Actor: "mini-employee:7:test"}
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var afterItems, notes, payment string
	var total float64
	var audits, latest int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT notes,total_amount::float8,payment_method FROM %s.orders WHERE id=42`, schema)).Scan(&notes, &total, &payment); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(i))::text FROM %s.order_items i`, schema)).Scan(&afterItems); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_id=42 AND actor='mini-employee:7:test'`, schema)).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.sales_order_documents WHERE is_latest`, schema)).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if beforeItems != afterItems || notes != "keep notes" || total != 100 || payment != "现金" || audits != 2 || latest != 1 {
		t.Fatalf("content/audit mismatch notes=%s amount=%v payment=%s audits=%d latest=%d", notes, total, payment, audits, latest)
	}
	if err = repo.UpdateOrderStatus(ctx, cmd); err == nil {
		t.Fatal("stale revision must conflict")
	}
	// Ordinary orders can explicitly change shipping status using the saved
	// non-courier delivery method without rebuilding order content.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=1 WHERE id=42`, schema)); err != nil {
		t.Fatal(err)
	}
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var updatedShip int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT ship_status_id FROM %s.orders WHERE id=42`, schema)).Scan(&updatedShip); err != nil || updatedShip != 2 {
		t.Fatalf("ship=%v err=%v", updatedShip, err)
	}
	deposit := 20.0
	cmd.PayStatusID = 3
	cmd.PrepaymentAmount = &deposit
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	cmd.PayStatusID = 2
	cmd.PrepaymentAmount = nil
	for _, mode := range []string{"void", "managed_shipping", "pending", "receipt", "method", "prepayment", "logistics", "free_sample"} {
		t.Run(mode, func(t *testing.T) {
			_, e := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=false,confirmation_required=false,confirmation_status='accepted',settlement_reason='',ship_method='pickup',pay_status_id=2,ship_status_id=2,payment_method='现金',prepayment_amount=0 WHERE id=42`, schema))
			if e != nil {
				t.Fatal(e)
			}
			c := salesapp.UpdateOrderStatusCommand{OrderID: 42, PayStatusID: 2, ShipStatusID: 2, PaymentMethod: "现金", Actor: "test"}
			switch mode {
			case "void":
				_, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=true WHERE id=42`, schema))
			case "managed_shipping":
				_, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true WHERE id=42`, schema))
				c.ShipStatusID = 1
			case "pending":
				_, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending' WHERE id=42`, schema))
			case "receipt":
				c.PayStatusID = 4
			case "method":
				c.PaymentMethod = ""
			case "prepayment":
				c.PayStatusID = 3
				amount := 120.0
				c.PrepaymentAmount = &amount
			case "logistics":
				_, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_method='sf_small',ship_status_id=1 WHERE id=42`, schema))
			case "free_sample":
				_, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET settlement_reason='free_sample',grand_total=0,total_amount=0 WHERE id=42`, schema))
				c.PayStatusID = 1
			}
			if e != nil {
				t.Fatal(e)
			}
			c.ExpectedEditRevision = revision()
			if e = repo.UpdateOrderStatus(ctx, c); e == nil {
				t.Fatal("business guard did not reject")
			}
		})
	}
	// Settled fulfillment may receive payment, while stock/shipment progress remains authoritative.
	_, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=false,confirmation_required=true,confirmation_status='accepted',settlement_reason='',grand_total=100,total_amount=100,pay_status_id=1,ship_method='sf_small' WHERE id=42`, schema))
	if err != nil {
		t.Fatal(err)
	}
	cmd.ExpectedEditRevision = revision()
	// Stand in for PostgreSQL's existing UPDATE OF ship_status_id recipient
	// and shipment progress triggers: payment-only SQL must not name the column.
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_shipping_touch() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'payment update touched shipping'; END$$;CREATE TRIGGER reject_shipping_touch BEFORE UPDATE OF ship_status_id ON %[1]s.orders FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_shipping_touch()`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	// A new receipt must use the same employee's uploaded voucher; omitted
	// receipt fields on a later update preserve the persisted reconciliation.
	goodsAmount, shippingAmount, voucher := 100.0, 0.0, int64(12)
	cmd.PayStatusID = 4
	cmd.PaymentGoodsAmount = &goodsAmount
	cmd.PaymentShippingAmount = &shippingAmount
	cmd.PaymentVoucherAssetID = &voucher
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	cmd.PaymentGoodsAmount = nil
	cmd.PaymentShippingAmount = nil
	cmd.PaymentVoucherAssetID = nil
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var savedGoods float64
	var savedVoucher int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_goods_amount::float8,payment_voucher_asset_id FROM %s.orders WHERE id=42`, schema)).Scan(&savedGoods, &savedVoucher); err != nil {
		t.Fatal(err)
	}
	if savedGoods != 100 || savedVoucher != 12 {
		t.Fatalf("receipt lost: goods=%v voucher=%v", savedGoods, savedVoucher)
	}
	wrongVoucher := int64(13)
	cmd.PaymentVoucherAssetID = &wrongVoucher
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err == nil {
		t.Fatal("another employee's voucher must be rejected")
	}
	// A sample with zero goods and billable freight uses the existing sample
	// workflow link rather than a client flag to allow a freight-only receipt.
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s.sample_work_orders(order_id BIGINT);INSERT INTO %s.sample_work_orders VALUES(42);UPDATE %s.orders SET total_amount=0,grand_total=20 WHERE id=42`, schema, schema, schema))
	if err != nil {
		t.Fatal(err)
	}
	goodsAmount, shippingAmount = 0, 20
	cmd.PaymentGoodsAmount = &goodsAmount
	cmd.PaymentShippingAmount = &shippingAmount
	cmd.PaymentVoucherAssetID = nil
	cmd.ExpectedEditRevision = revision()
	if err = repo.UpdateOrderStatus(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(beforeItems, "frozen") {
		t.Fatal("fixture lost frozen content")
	}
}
