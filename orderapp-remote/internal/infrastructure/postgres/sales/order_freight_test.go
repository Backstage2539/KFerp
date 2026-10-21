package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestFreightEditPreservesRealReceipts(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, e := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.pay_statuses(id bigint,name text);INSERT INTO %[1]s.pay_statuses VALUES(1,'已收款'),(2,'预付款（付款未完成）');CREATE TABLE %[1]s.orders(id bigint,shipping_amount numeric,payment_goods_amount numeric,payment_shipping_amount numeric,prepayment_amount numeric,payment_voucher_asset_id bigint,pay_status_id bigint);INSERT INTO %[1]s.orders VALUES(1,20,60,20,0,9,1);`, schema))
	if e != nil {
		t.Fatal(e)
	}
	run := func(shipping, total float64, c salesapp.SaveOrderCommand) (salesapp.SaveOrderCommand, error) {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		e = protectFreightReceiptsTx(ctx, tx, schema, 1, &c, shipping, total)
		return c, e
	}
	cmd := salesapp.SaveOrderCommand{PayStatusID: 1, PaymentGoodsAmount: 60, PaymentShippingAmount: 20, PaymentVoucherAssetID: 9}
	if _, e = run(15, 75, cmd); e == nil {
		t.Fatal("reduction below real collection accepted")
	}
	forged := cmd
	forged.PaymentShippingAmount = 0
	forged.PaymentVoucherAssetID = 0
	if _, e = run(30, 90, forged); e == nil {
		t.Fatal("cleared receipt accepted")
	}
	revised, e := run(30, 90, cmd)
	if e != nil || revised.PayStatusID != 2 || revised.PrepaymentAmount == nil || *revised.PrepaymentAmount != 80 || revised.PaymentShippingAmount != 20 {
		t.Fatal(revised, e)
	}
}
