package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	"strings"
	"testing"
)

func TestFreightEditPreservesRealReceipts(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, e := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.pay_statuses(id bigint,name text);INSERT INTO %[1]s.pay_statuses VALUES(1,'已收款'),(2,'预付款（付款未完成）');CREATE TABLE %[1]s.orders(id bigint,shipping_amount numeric,payment_goods_amount numeric,payment_shipping_amount numeric,prepayment_amount numeric,payment_voucher_asset_id bigint,pay_status_id bigint,grand_total numeric,total_amount numeric DEFAULT 0,discount_amount numeric DEFAULT 0,rounding_amount numeric DEFAULT 0);INSERT INTO %[1]s.orders VALUES(1,20,60,20,0,9,1,80,60,0,0);`, schema))
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

func TestFreightEditProtectsLegacyFullyPaidStatus(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, e := pool.Exec(ctx, fmt.Sprintf(`CREATE TABLE %[1]s.pay_statuses(id bigint,name text);INSERT INTO %[1]s.pay_statuses VALUES(1,'已付款'),(2,'预付款（付款未完成）');CREATE TABLE %[1]s.orders(id bigint,shipping_amount numeric,payment_goods_amount numeric,payment_shipping_amount numeric,prepayment_amount numeric,payment_voucher_asset_id bigint,pay_status_id bigint,grand_total numeric,total_amount numeric DEFAULT 0,discount_amount numeric DEFAULT 0,rounding_amount numeric DEFAULT 0);INSERT INTO %[1]s.orders VALUES(1,20,0,0,0,0,1,80,60,0,0);`, schema))
	if e != nil {
		t.Fatal(e)
	}
	cmd := salesapp.SaveOrderCommand{PayStatusID: 1}
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = protectFreightReceiptsTx(ctx, tx, schema, 1, &cmd, 15, 75); e == nil {
		t.Fatal("legacy paid freight reduction silently changed settled payment")
	}
	sameFee := salesapp.SaveOrderCommand{PayStatusID: 1}
	if e = protectFreightReceiptsTx(ctx, tx, schema, 1, &sameFee, 20, 90); e != nil || sameFee.PayStatusID != 2 || sameFee.PrepaymentAmount == nil || *sameFee.PrepaymentAmount != 80 {
		t.Fatalf("basis change invented a payment: %+v %v", sameFee, e)
	}
	cleared := salesapp.SaveOrderCommand{PayStatusID: 1}
	if e = protectFreightReceiptsTx(ctx, tx, schema, 1, &cleared, 15, 80); e != nil || cleared.PayStatusID != 1 || cleared.PrepaymentAmount != nil {
		t.Fatalf("paid state cleared: %+v %v", cleared, e)
	}
	if e = protectFreightReceiptsTx(ctx, tx, schema, 1, &cmd, 30, 90); e != nil || cmd.PayStatusID != 2 || cmd.PrepaymentAmount == nil || *cmd.PrepaymentAmount != 80 {
		t.Fatal(cmd, e)
	}
}

func TestFreightPaymentRegression(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.pay_statuses(id bigint,name text);
INSERT INTO %[1]s.pay_statuses VALUES(1,'已付款'),(2,'预付款（付款未完成）'),(3,'未付款'),(4,'已收款');
CREATE TABLE %[1]s.sales_order_assets(id bigint,kind text);
INSERT INTO %[1]s.sales_order_assets VALUES(9,'payment_voucher'),(10,'seal');
CREATE TABLE %[1]s.orders(id bigint,shipping_amount numeric,payment_goods_amount numeric,payment_shipping_amount numeric,prepayment_amount numeric,payment_voucher_asset_id bigint,pay_status_id bigint,grand_total numeric,total_amount numeric,discount_amount numeric,rounding_amount numeric DEFAULT 0);
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	ptr := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name                                                    string
		oldTotal, oldGoods, oldDiscount, oldShipping, oldPrepay float64
		oldStatus                                               int64
		oldReceipt, oldFreightReceipt                           float64
		oldVoucher                                              int64
		shipping, total, oldRounding                            float64
		cmd                                                     salesapp.SaveOrderCommand
		wantStatus                                              int64
		wantPrepay                                              *float64
		wantError                                               string
	}{
		{name: "legacy_zero_grand_total_increase", oldGoods: 120, oldStatus: 1, shipping: 20, total: 140, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 2, wantPrepay: ptr(120)},
		{name: "legacy_zero_grand_total_unchanged", oldGoods: 120, oldStatus: 1, total: 120, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 1},
		{name: "genuine_zero_increase_is_unpaid", oldStatus: 1, shipping: 20, total: 20, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 3},
		{name: "full_discount_increase_is_unpaid", oldGoods: 120, oldDiscount: 120, oldStatus: 1, shipping: 20, total: 20, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 3},
		{name: "full_discount_unchanged", oldGoods: 120, oldDiscount: 120, oldStatus: 1, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 1},
		{name: "rounded_zero_increase_is_unpaid", oldGoods: 0.4, oldRounding: -0.4, oldStatus: 1, shipping: 20, total: 20, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 3},
		{name: "zero_with_shipping_is_not_legacy", oldGoods: 120, oldShipping: 20, oldStatus: 1, shipping: 30, total: 10, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 3},
		{name: "fully_paid_same_total_preserves_no_deposit", oldTotal: 100, oldShipping: 20, oldStatus: 1, shipping: 10, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 1},
		{name: "fully_paid_same_total_preserves_original_deposit", oldTotal: 100, oldShipping: 20, oldPrepay: 30, oldStatus: 1, shipping: 10, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1, PrepaymentAmount: ptr(30)}, wantStatus: 1, wantPrepay: ptr(30)},
		{name: "fully_paid_increase_preserves_full_collection", oldTotal: 100, oldPrepay: 30, oldStatus: 1, shipping: 20, total: 120, cmd: salesapp.SaveOrderCommand{PayStatusID: 1, PrepaymentAmount: ptr(30)}, wantStatus: 2, wantPrepay: ptr(100)},
		{name: "paid_reduction_rejected", oldTotal: 100, oldShipping: 20, oldStatus: 1, shipping: 10, total: 90, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantError: "超出应收"},
		{name: "partial_reduction_stays_partial", oldTotal: 100, oldShipping: 20, oldPrepay: 30, oldStatus: 2, shipping: 10, total: 90, cmd: salesapp.SaveOrderCommand{PayStatusID: 2}, wantStatus: 2, wantPrepay: ptr(30)},
		{name: "partial_reduction_settles", oldTotal: 100, oldShipping: 70, oldPrepay: 30, oldStatus: 2, total: 30, cmd: salesapp.SaveOrderCommand{PayStatusID: 2}, wantStatus: 1},
		{name: "partial_reduction_below_received_rejected", oldTotal: 100, oldShipping: 80, oldPrepay: 30, oldStatus: 2, total: 20, cmd: salesapp.SaveOrderCommand{PayStatusID: 2}, wantError: "超出应收"},
		{name: "explicit_settlement_keeps_deposit", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1, PrepaymentAmount: ptr(30)}, wantStatus: 1, wantPrepay: ptr(30)},
		{name: "explicit_settlement_omits_deposit", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantStatus: 1},
		{name: "explicit_additional_prepayment", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PrepaymentAmount: ptr(50)}, wantStatus: 2, wantPrepay: ptr(50)},
		{name: "explicit_receipt_settlement", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 4, PaymentGoodsAmount: 100, PaymentVoucherAssetID: 9}, wantStatus: 4},
		{name: "explicit_receipt_requires_real_voucher", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1, PaymentGoodsAmount: 100, PaymentVoucherAssetID: 999}, wantError: "invalid payment_voucher_asset_id"},
		{name: "explicit_receipt_rejects_other_asset_kind", oldTotal: 100, oldPrepay: 30, oldStatus: 2, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PrepaymentAmount: ptr(50), PaymentGoodsAmount: 50, PaymentVoucherAssetID: 10}, wantError: "invalid payment_voucher_asset_id"},
		{name: "receipt_must_match_settlement", oldTotal: 100, oldPrepay: 30, oldStatus: 2, oldReceipt: 30, oldVoucher: 9, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1, PaymentGoodsAmount: 30, PaymentVoucherAssetID: 9}, wantError: "实收金额"},
		{name: "receipt_must_match_prepayment", oldTotal: 100, oldPrepay: 30, oldStatus: 2, oldReceipt: 30, oldVoucher: 9, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PrepaymentAmount: ptr(50), PaymentGoodsAmount: 30, PaymentVoucherAssetID: 9}, wantError: "实收金额"},
		{name: "cannot_clear_existing_receipt_to_settle", oldTotal: 100, oldPrepay: 30, oldStatus: 2, oldReceipt: 30, oldVoucher: 9, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantError: "凭证"},
		{name: "explicit_matching_partial_receipt", oldTotal: 100, oldPrepay: 30, oldStatus: 2, oldReceipt: 30, oldVoucher: 9, total: 100, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PrepaymentAmount: ptr(50), PaymentGoodsAmount: 50, PaymentVoucherAssetID: 9}, wantStatus: 2, wantPrepay: ptr(50)},
		{name: "mixed_freight_and_deposit_rejected", oldTotal: 100, oldPrepay: 30, oldStatus: 2, shipping: 20, total: 120, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PrepaymentAmount: ptr(50)}, wantError: "单独"},
		{name: "mixed_freight_and_settlement_rejected", oldTotal: 100, oldPrepay: 30, oldStatus: 2, shipping: 20, total: 120, cmd: salesapp.SaveOrderCommand{PayStatusID: 1}, wantError: "单独"},
		{name: "mixed_freight_and_receipt_rejected", oldTotal: 100, oldPrepay: 30, oldStatus: 2, shipping: 20, total: 120, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PaymentGoodsAmount: 30, PaymentVoucherAssetID: 9}, wantError: "单独"},
		{name: "receipt_shipping_floor", oldTotal: 100, oldShipping: 20, oldPrepay: 30, oldStatus: 2, oldReceipt: 10, oldFreightReceipt: 20, oldVoucher: 9, shipping: 10, total: 90, cmd: salesapp.SaveOrderCommand{PayStatusID: 2, PaymentGoodsAmount: 10, PaymentShippingAmount: 20, PaymentVoucherAssetID: 9}, wantError: "超出应收"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders VALUES(1,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, schema), tc.oldShipping, tc.oldReceipt, tc.oldFreightReceipt, tc.oldPrepay, tc.oldVoucher, tc.oldStatus, tc.oldTotal, tc.oldGoods, tc.oldDiscount, tc.oldRounding)
			if err != nil {
				t.Fatal(err)
			}
			before := tc.cmd
			err = protectFreightReceiptsTx(ctx, tx, schema, 1, &tc.cmd, tc.shipping, tc.total)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error=%v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.cmd.PayStatusID != tc.wantStatus || (tc.cmd.PrepaymentAmount == nil) != (tc.wantPrepay == nil) || tc.wantPrepay != nil && *tc.cmd.PrepaymentAmount != *tc.wantPrepay {
				t.Fatalf("status=%d prepayment=%v, want status=%d prepayment=%v", tc.cmd.PayStatusID, tc.cmd.PrepaymentAmount, tc.wantStatus, tc.wantPrepay)
			}
			if tc.cmd.PaymentGoodsAmount != before.PaymentGoodsAmount || tc.cmd.PaymentShippingAmount != before.PaymentShippingAmount || tc.cmd.PaymentVoucherAssetID != before.PaymentVoucherAssetID {
				t.Fatal("receipt fields were fabricated or overwritten")
			}
		})
	}
}
