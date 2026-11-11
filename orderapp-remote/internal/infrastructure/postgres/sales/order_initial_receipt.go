package sales

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	pg "orderapp/internal/infrastructure/postgres"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"orderapp/internal/infrastructure/postgres/receipts"
)

// initialOrderReceiptAmount never infers cash from a payment-status label.
// An old client supplying inconsistent cumulative values cannot establish a
// unique cash fact; its order remains compatible but no receipt is invented.
func initialOrderReceiptAmount(prepayment, goods, shipping float64) (int64, bool) {
	cents := func(v float64) (int64, bool) {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > float64(math.MaxInt64)/100 || math.Abs(v*100-math.Round(v*100)) > 0.00001 {
			return 0, false
		}
		return int64(math.Round(v * 100)), true
	}
	p, pok := cents(prepayment)
	g, gok := cents(goods)
	s, sok := cents(shipping)
	if !pok || !gok || !sok || g > math.MaxInt64-s {
		return 0, false
	}
	parts := g + s
	if p > 0 && parts > 0 && p != parts {
		return 0, false
	}
	if p > 0 {
		return p, true
	}
	if parts > 0 {
		return parts, true
	}
	return 0, false
}

// The order's cumulative amount and explicit goods/freight amounts must agree
// before a new cash fact can be emitted. Never infer an allocation.
func initialOrderReceiptConsistent(status string, total, freight, prepayment, goods, shipping float64) (int64, bool) {
	actual, ok := initialOrderReceiptAmount(prepayment, goods, shipping)
	if !ok || math.IsNaN(total) || math.IsInf(total, 0) || math.IsNaN(freight) || math.IsInf(freight, 0) || total <= 0 || freight < 0 || freight > total || math.Abs(total*100-math.Round(total*100)) > 0.00001 || math.Abs(freight*100-math.Round(freight*100)) > 0.00001 {
		return 0, false
	}
	if int64(math.Round(prepayment*100)) != actual || int64(math.Round((goods+shipping)*100)) != actual {
		return 0, false
	}
	totalCents, freightCents := int64(math.Round(total*100)), int64(math.Round(freight*100))
	if actual > totalCents || int64(math.Round(shipping*100)) > freightCents || int64(math.Round(goods*100)) > totalCents-freightCents {
		return 0, false
	}
	if salesdomain.IsFullyPaid(status) {
		return actual, actual == totalCents
	}
	return actual, strings.Contains(status, "预付款") && actual < totalCents
}

func recordInitialOrderReceipt(ctx context.Context, tx pgx.Tx, schema string, orderID, customerID int64, orderNo, method, actor, date string, voucherID int64, actual, receivable int64) error {
	if voucherID > 0 {
		var owned bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher' AND created_by=$2)`, schema), voucherID, actor).Scan(&owned); err != nil {
			return err
		}
		if !owned {
			return fmt.Errorf("收款凭证不属于当前操作人，请重新上传")
		}
	}
	allocations, err := json.Marshal([]map[string]any{{"order_id": orderID, "order_no": orderNo, "amount_cents": actual}})
	if err != nil {
		return err
	}
	id, no, received, err := receipts.RecordTx(ctx, tx, schema, receipts.Receipt{CustomerID: customerID, SourceType: "initial_order", SourceID: orderID, ActualCents: actual, ReceivableCents: receivable, ReceiptDate: date, Method: method, VoucherAssetID: voucherID, Actor: actor, Allocations: allocations})
	if err != nil {
		return err
	}
	if err = pg.AuditInsertTx(ctx, tx, schema, actor, "cash_receipt", &id, "collect", nil, nil, nil, pg.AuditMeta{"customer_id": customerID, "order_id": orderID, "actual_cents": actual, "source": "initial_order"}); err != nil {
		return err
	}
	return messagepg.RecordBusinessEvent(ctx, tx, schema, "receipt_received", fmt.Sprintf("receipt_received:%d", id), orderID, customerID, map[string]any{"receipt_id": id, "receipt_no": no, "receipt_date": received, "amount_cents": actual, "receivable_cents": receivable})
}

func validateInitialOrderReceiptDate(date string) error {
	if date == "" {
		return nil
	}
	return app.ValidateReceiptDate(date)
}
