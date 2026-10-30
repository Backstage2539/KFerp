package sales

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

type combinedOrderLocked struct {
	ID, CustomerID, PayID, ShipID, VoucherID             int64
	OrderNo, PayStatus, Notes, Reason                    string
	Grand, Shipping, Prepayment, GoodsPaid, ShippingPaid int64
	Allocation                                           salesapp.CombinedPaymentAllocation
	AlreadyPaid                                          bool
}

func combinedValidation(message string) error {
	return salesapp.NewCombinedPaymentValidationError(message)
}

// CombinedOrderPayment validates and records the receipt and every requested
// shipping change in one transaction. Row locks are always acquired by id.
func (r Repository) CombinedOrderPayment(ctx context.Context, cmd salesapp.CombinedOrderPaymentCommand) (salesapp.CombinedOrderPaymentResult, error) {
	var empty salesapp.CombinedOrderPaymentResult
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	orders := append([]salesapp.CombinedPaymentOrder(nil), cmd.Orders...)
	sort.Slice(orders, func(i, j int) bool { return orders[i].OrderID < orders[j].OrderID })
	// Serialize retries even before an idempotency row exists.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "order-combined-payment:"+cmd.IdempotencyKey); err != nil {
		return empty, err
	}
	fingerprint := struct {
		Orders                                               []salesapp.CombinedPaymentOrder `json:"orders"`
		ActualCents, ExpectedDueCents, PaymentVoucherAssetID int64
		ShipStatusID                                         *int64
		PaymentMethod, Actor                                 string
	}{orders, cmd.ActualCents, cmd.ExpectedDueCents, cmd.PaymentVoucherAssetID, cmd.ShipStatusID, strings.TrimSpace(cmd.PaymentMethod), strings.TrimSpace(cmd.Actor)}
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return empty, err
	}
	digest := sha256.Sum256(encoded)
	requestHash := hex.EncodeToString(digest[:])
	var existingHash string
	var existingRaw []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash,result_json FROM %s.order_combined_payments WHERE idempotency_key=$1`, r.schema), cmd.IdempotencyKey).Scan(&existingHash, &existingRaw)
	if err == nil {
		if existingHash != requestHash {
			return empty, combinedValidation("操作标识已用于不同的收款，请重新核对")
		}
		var result salesapp.CombinedOrderPaymentResult
		if err = json.Unmarshal(existingRaw, &result); err != nil {
			return empty, err
		}
		result.Reused = true
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	// The batch owns its voucher independently of any voucher already linked
	// to an order. Validate every supplied voucher even if it happens to match
	// an existing order voucher, rather than relying on per-order change checks.
	if cmd.PaymentVoucherAssetID > 0 {
		var owned bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.sales_order_assets WHERE id=$1 AND kind='payment_voucher' AND created_by=$2)`, r.schema), cmd.PaymentVoucherAssetID, strings.TrimSpace(cmd.Actor)).Scan(&owned); err != nil {
			return empty, err
		}
		if !owned {
			return empty, combinedValidation("invalid payment_voucher_asset_id")
		}
	}
	var paidStatusID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name='已付款' ORDER BY id LIMIT 1`, r.schema)).Scan(&paidStatusID); err != nil {
		return empty, fmt.Errorf("查找已付款状态失败: %w", err)
	}
	locked := make([]combinedOrderLocked, 0, len(orders))
	var customerID, totalDue int64
	for _, item := range orders {
		var row combinedOrderLocked
		var isVoid, managed bool
		var confirm string
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT o.id,COALESCE(o.customer_id,0),COALESCE(o.order_no,''),COALESCE(o.pay_status_id,0),COALESCE(p.name,''),COALESCE(o.ship_status_id,0),COALESCE(o.payment_voucher_asset_id,0),COALESCE(o.notes,''),COALESCE(to_jsonb(o)->>'settlement_reason',''),
round(%s*100)::bigint,round(COALESCE(o.shipping_amount,0)*100)::bigint,round(COALESCE(o.prepayment_amount,0)*100)::bigint,round(COALESCE(o.payment_goods_amount,0)*100)::bigint,round(COALESCE(o.payment_shipping_amount,0)*100)::bigint,
COALESCE(o.is_void,false),COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),COALESCE(to_jsonb(o)->>'confirmation_status','accepted')
FROM %s.orders o LEFT JOIN %s.pay_statuses p ON p.id=o.pay_status_id WHERE o.id=$1 FOR UPDATE OF o`, orderReceivableSQL("o"), r.schema, r.schema), item.OrderID).Scan(&row.ID, &row.CustomerID, &row.OrderNo, &row.PayID, &row.PayStatus, &row.ShipID, &row.VoucherID, &row.Notes, &row.Reason, &row.Grand, &row.Shipping, &row.Prepayment, &row.GoodsPaid, &row.ShippingPaid, &isVoid, &managed, &confirm)
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, combinedValidation("订单不存在或无权访问")
		}
		if err != nil {
			return empty, err
		}
		if row.CustomerID <= 0 {
			return empty, combinedValidation("订单缺少客户，不能合并收款")
		}
		if customerID == 0 {
			customerID = row.CustomerID
		} else if customerID != row.CustomerID {
			return empty, combinedValidation("合并收款只能选择同一客户的订单")
		}
		if isVoid {
			return empty, combinedValidation("订单已作废，不能修改状态")
		}
		if managed && confirm != "accepted" {
			return empty, combinedValidation("订单尚未确认，请先确认订单")
		}
		revision, e := currentOrderEditRevisionTx(ctx, tx, r.schema, row.ID)
		if e != nil {
			return empty, e
		}
		if revision != strings.TrimSpace(item.EditRevision) {
			return empty, combinedValidation("订单已发生变化，请刷新后重新核对")
		}
		row.AlreadyPaid = salesdomain.IsFullyPaid(row.PayStatus)
		if !row.AlreadyPaid {
			if row.Reason == "free_sample" || row.Grand <= 0 {
				return empty, combinedValidation("免费样品或零金额订单不能参与合并收款，请单独处理")
			}
			if row.Shipping < 0 || row.Shipping > row.Grand || row.Prepayment < 0 || row.Prepayment >= row.Grand || row.GoodsPaid < 0 || row.ShippingPaid < 0 {
				return empty, combinedValidation("订单应收或已收金额异常，请先核对")
			}
			goodsDue := row.Grand - row.Shipping
			goodsPaid, shipPaid := row.GoodsPaid, row.ShippingPaid
			if goodsPaid == 0 && shipPaid == 0 && row.Prepayment > 0 {
				goodsPaid = min(goodsDue, row.Prepayment)
				shipPaid = row.Prepayment - goodsPaid
			}
			if goodsPaid > goodsDue || shipPaid > row.Shipping || goodsPaid+shipPaid != row.Prepayment {
				return empty, combinedValidation("订单预付款与实收明细不一致，请先核对")
			}
			goodsDelta, shipDelta := goodsDue-goodsPaid, row.Shipping-shipPaid
			amount := goodsDelta + shipDelta
			if amount <= 0 || totalDue > math.MaxInt64-amount {
				return empty, combinedValidation("订单待收金额异常，请先核对")
			}
			row.Allocation = salesapp.CombinedPaymentAllocation{OrderID: row.ID, OrderNo: row.OrderNo, GoodsCents: goodsDelta, ShippingCents: shipDelta, AmountCents: amount}
			totalDue += amount
		}
		locked = append(locked, row)
	}
	if totalDue <= 0 {
		return empty, combinedValidation("所选订单均已付款，无需重复收款")
	}
	if totalDue != cmd.ExpectedDueCents {
		return empty, combinedValidation("订单待收金额已变化，请刷新并重新核对")
	}
	if cmd.ActualCents < totalDue {
		return empty, combinedValidation("实际收款不得少于待收合计")
	}
	result := salesapp.CombinedOrderPaymentResult{CustomerID: customerID, ActualCents: cmd.ActualCents, DueCents: totalDue, UnallocatedCents: cmd.ActualCents - totalDue, PaymentMethod: strings.TrimSpace(cmd.PaymentMethod), PaymentVoucherAssetID: cmd.PaymentVoucherAssetID, Actor: strings.TrimSpace(cmd.Actor), Allocations: []salesapp.CombinedPaymentAllocation{}}
	for i, row := range locked {
		item := orders[i]
		shipID := row.ShipID
		if cmd.ShipStatusID != nil {
			shipID = *cmd.ShipStatusID
		}
		statusCmd := salesapp.UpdateOrderStatusCommand{OrderID: row.ID, ExpectedEditRevision: item.EditRevision, PayStatusID: row.PayID, ShipStatusID: shipID, Actor: cmd.Actor, Source: "erp-combined-order-payment", PaymentUnchanged: row.AlreadyPaid}
		if !row.AlreadyPaid {
			statusCmd.PayStatusID = paidStatusID
			statusCmd.PaymentMethod = strings.TrimSpace(cmd.PaymentMethod)
			total := float64(row.Grand) / 100
			goods := float64(row.Grand-row.Shipping) / 100
			shipping := float64(row.Shipping) / 100
			statusCmd.PrepaymentAmount, statusCmd.PaymentGoodsAmount, statusCmd.PaymentShippingAmount = &total, &goods, &shipping
			// Preserve an earlier receipt's voucher; the new voucher remains
			// attached to this batch and is visible in its history.
			if cmd.PaymentVoucherAssetID > 0 && row.VoucherID == 0 {
				voucher := cmd.PaymentVoucherAssetID
				statusCmd.PaymentVoucherAssetID = &voucher
			}
		}
		if err = r.updateOrderStatusTx(ctx, tx, statusCmd); err != nil {
			return empty, combinedStatusError(err)
		}
		if row.AlreadyPaid {
			continue
		}
		newNotes := appendCombinedPaymentNote(row.Notes)
		if newNotes != row.Notes {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes=$2 WHERE id=$1`, r.schema), row.ID, newNotes); err != nil {
				return empty, err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,$2,'notes',$3,$4)`, r.schema), row.ID, cmd.Actor, row.Notes, newNotes); err != nil {
				return empty, err
			}
			if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order", &row.ID, "update", postgresinfra.StrPtr("notes"), postgresinfra.StrPtr(row.Notes), postgresinfra.StrPtr(newNotes), postgresinfra.AuditMeta{"order_id": row.ID, "source": "erp-combined-order-payment"}); err != nil {
				return empty, err
			}
			if err = invalidateConfirmedOrderDocumentsTx(ctx, tx, r.schema, row.ID); err != nil {
				return empty, err
			}
		}
		result.Allocations = append(result.Allocations, row.Allocation)
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_combined_payments(idempotency_key,request_hash,customer_id,actual_cents,due_cents,unallocated_cents,payment_method,payment_voucher_asset_id,actor) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS')`, r.schema), cmd.IdempotencyKey, requestHash, customerID, result.ActualCents, result.DueCents, result.UnallocatedCents, result.PaymentMethod, result.PaymentVoucherAssetID, result.Actor).Scan(&result.BatchID, &result.CreatedAt)
	if err != nil {
		return empty, err
	}
	for _, a := range result.Allocations {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_combined_payment_allocations(batch_id,order_id,order_no,goods_cents,shipping_cents,amount_cents) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), result.BatchID, a.OrderID, a.OrderNo, a.GoodsCents, a.ShippingCents, a.AmountCents); err != nil {
			return empty, err
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return empty, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_combined_payments SET result_json=$2::jsonb WHERE id=$1`, r.schema), result.BatchID, raw); err != nil {
		return empty, err
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order_combined_payment", &result.BatchID, "collect", nil, nil, nil, postgresinfra.AuditMeta{"customer_id": customerID, "actual_cents": result.ActualCents, "due_cents": result.DueCents, "unallocated_cents": result.UnallocatedCents}); err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return result, nil
}

func appendCombinedPaymentNote(old string) string {
	for _, line := range strings.Split(old, "\n") {
		if strings.TrimSpace(line) == "合并付款" {
			return old
		}
	}
	if strings.TrimSpace(old) == "" {
		return "合并付款"
	}
	return old + "\n合并付款"
}

func combinedStatusError(err error) error {
	if _, ok := salesapp.OrderEditConflictMessage(err); ok {
		return err
	}
	switch err.Error() {
	case "payment_goods_amount required", "payment_voucher_asset_id required", "invalid payment_voucher_asset_id", "logistics_company_id required", "logistics_product_id required", "invalid logistics product", "invalid pay_status_id", "invalid ship_status_id", "payment_method required":
		return combinedValidation(err.Error())
	}
	if strings.Contains(err.Error(), "预付款") || strings.HasPrefix(err.Error(), "实收金额") {
		return combinedValidation(err.Error())
	}
	return err
}

// OrderCombinedPayments returns historical receipt records linked to the given
// order. HTTP callers must first verify the actor can view that order.
func (r Repository) OrderCombinedPayments(ctx context.Context, orderID int64) ([]salesapp.CombinedOrderPaymentResult, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT p.result_json FROM %s.order_combined_payments p JOIN %s.order_combined_payment_allocations a ON a.batch_id=p.id WHERE a.order_id=$1 ORDER BY p.id DESC`, r.schema, r.schema), orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []salesapp.CombinedOrderPaymentResult{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var result salesapp.CombinedOrderPaymentResult
		if err = json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, rows.Err()
}
