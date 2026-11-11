package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/sales"
	pg "orderapp/internal/infrastructure/postgres"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"orderapp/internal/infrastructure/postgres/receipts"
)

// RegisterOrderReceipt commits the payment, cash fact, audit, and marker together.
func (r Repository) RegisterOrderReceipt(ctx context.Context, cmd app.OrderReceiptCommand) (app.OrderReceiptResult, error) {
	var result app.OrderReceiptResult
	if err := app.ValidateOrderReceipt(cmd); err != nil {
		return result, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "single-receipt:"+cmd.IdempotencyKey); err != nil {
		return result, err
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256(append(raw, []byte("\x00"+cmd.Actor)...))
	hash := hex.EncodeToString(digest[:])
	var old string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash,receipt_id,receipt_no FROM %s.order_single_receipt_requests WHERE idempotency_key=$1`, r.schema), cmd.IdempotencyKey).Scan(&old, &result.ReceiptID, &result.ReceiptNo)
	if err == nil {
		if old != hash {
			return result, combinedValidation("收款操作标识已用于其他操作")
		}
		result.Reused = true
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	// Finance close and bill payments take this lock before locking orders.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":finance-period"); err != nil {
		return result, err
	}
	if err = receipts.EnsureDateOpenTx(ctx, tx, r.schema, cmd.ReceiptDate); err != nil {
		return result, combinedValidation(err.Error())
	}
	var customer, ship, total, shipping, pre, goods, shipPaid, legacy, offset, settled, oldVoucher int64
	var orderNo, reason string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0),COALESCE(order_no,''),COALESCE(ship_status_id,0),round(%s*100)::bigint,round(COALESCE(shipping_amount,0)*100)::bigint,round(COALESCE(prepayment_amount,0)*100)::bigint,round(COALESCE(payment_goods_amount,0)*100)::bigint,round(COALESCE(payment_shipping_amount,0)*100)::bigint,COALESCE((to_jsonb(o)->>'resolution_legacy_cash_cents')::bigint,0),COALESCE((to_jsonb(o)->>'resolution_offset_cents')::bigint,0),COALESCE((to_jsonb(o)->>'resolution_settled_out_cents')::bigint,0),COALESCE(to_jsonb(o)->>'settlement_reason',''),COALESCE(payment_voucher_asset_id,0) FROM %s.orders o WHERE id=$1 FOR UPDATE`, orderReceivableSQL("o"), r.schema), cmd.OrderID).Scan(&customer, &orderNo, &ship, &total, &shipping, &pre, &goods, &shipPaid, &legacy, &offset, &settled, &reason, &oldVoucher)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, combinedValidation("订单不存在")
	}
	if err != nil {
		return result, err
	}
	if customer <= 0 || total <= 0 || shipping < 0 || shipping > total || pre < 0 || goods < 0 || shipPaid < 0 || legacy != 0 || offset != 0 || settled != 0 || reason == "free_sample" {
		return result, combinedValidation("订单收款或应收状态需人工核对")
	}
	if cmd.PaymentVoucherAssetID > 0 {
		var owned bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher' AND created_by=$2)`, r.schema), cmd.PaymentVoucherAssetID, cmd.Actor).Scan(&owned); err != nil {
			return result, err
		}
		if !owned {
			return result, combinedValidation("invalid payment_voucher_asset_id")
		}
	}
	goodsDue := total - shipping
	if goods == 0 && shipPaid == 0 && pre > 0 {
		goods = min(goodsDue, pre)
		shipPaid = pre - goods
	}
	if goods+shipPaid != pre || goods > goodsDue || shipPaid > shipping || total-pre != cmd.ExpectedDueCents || cmd.ActualCents > total-pre {
		return result, combinedValidation("订单待收金额已变化，请刷新核对")
	}
	goodsAdd := min(goodsDue-goods, cmd.ActualCents)
	shippingAdd := cmd.ActualCents - goodsAdd
	goods += goodsAdd
	shipPaid += shippingAdd
	pre += cmd.ActualCents
	status := "预付款（付款未完成）"
	if pre == total {
		status = "已付款"
	}
	var payID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name=$1 ORDER BY id LIMIT 1`, r.schema), status).Scan(&payID); err != nil {
		return result, err
	}
	toYuan := func(x int64) *float64 { v := float64(x) / 100; return &v }
	statusCmd := app.UpdateOrderStatusCommand{OrderID: cmd.OrderID, ExpectedEditRevision: cmd.EditRevision, PayStatusID: payID, ShipStatusID: ship, PaymentMethod: cmd.PaymentMethod, PrepaymentAmount: toYuan(pre), PaymentGoodsAmount: toYuan(goods), PaymentShippingAmount: toYuan(shipPaid), Actor: cmd.Actor, Source: "erp-order-receipt"}
	if cmd.PaymentVoucherAssetID > 0 && oldVoucher == 0 {
		v := cmd.PaymentVoucherAssetID
		statusCmd.PaymentVoucherAssetID = &v
	}
	if err = r.updateOrderStatusTx(ctx, tx, statusCmd); err != nil {
		return result, combinedStatusError(err)
	}
	var requestID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_single_receipt_requests(idempotency_key,request_hash) VALUES($1,$2) RETURNING id`, r.schema), cmd.IdempotencyKey, hash).Scan(&requestID)
	if err != nil {
		return result, err
	}
	alloc, _ := json.Marshal([]app.CombinedPaymentAllocation{{OrderID: cmd.OrderID, OrderNo: orderNo, GoodsCents: goodsAdd, ShippingCents: shippingAdd, AmountCents: cmd.ActualCents}})
	id, no, date, err := receipts.RecordTx(ctx, tx, r.schema, receipts.Receipt{CustomerID: customer, SourceType: "single_payment", SourceID: requestID, ActualCents: cmd.ActualCents, ReceivableCents: total, ReceiptDate: cmd.ReceiptDate, Method: cmd.PaymentMethod, VoucherAssetID: cmd.PaymentVoucherAssetID, Actor: cmd.Actor, Allocations: alloc})
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_single_receipt_requests SET receipt_id=$2,receipt_no=$3 WHERE id=$1`, r.schema), requestID, id, no); err != nil {
		return result, err
	}
	if err = pg.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "cash_receipt", &id, "collect", nil, nil, nil, pg.AuditMeta{"customer_id": customer, "order_id": cmd.OrderID, "actual_cents": cmd.ActualCents}); err != nil {
		return result, err
	}
	if err = messagepg.RecordBusinessEvent(ctx, tx, r.schema, "receipt_received", fmt.Sprintf("receipt_received:%d", id), cmd.OrderID, customer, map[string]any{"receipt_id": id, "receipt_no": no, "receipt_date": date, "amount_cents": cmd.ActualCents, "receivable_cents": total}); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return app.OrderReceiptResult{ReceiptID: id, ReceiptNo: no}, nil
}

// OrderReceipts exposes only receipt facts allocated solely to this order.
func (r Repository) OrderReceipts(ctx context.Context, orderID int64) ([]app.OrderReceiptHistory, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,receipt_no,COALESCE(receipt_date::text,''),actual_cents,receivable_cents,payment_method,voucher_asset_id,actor FROM %s.cash_receipts WHERE (source_type='single_payment' AND allocations @> jsonb_build_array(jsonb_build_object('order_id',$1::bigint))) OR (source_type='initial_order' AND source_id=$1) ORDER BY id DESC`, r.schema), orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []app.OrderReceiptHistory{}
	for rows.Next() {
		var v app.OrderReceiptHistory
		if err = rows.Scan(&v.ReceiptID, &v.ReceiptNo, &v.ReceiptDate, &v.ActualCents, &v.ReceivableCents, &v.PaymentMethod, &v.VoucherAssetID, &v.Actor); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
