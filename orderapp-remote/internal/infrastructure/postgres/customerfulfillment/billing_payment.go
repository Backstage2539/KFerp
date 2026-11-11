package customerfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerfulfillment"
	salesapp "orderapp/internal/application/sales"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	messagepg "orderapp/internal/infrastructure/postgres/messagecenter"
	"orderapp/internal/infrastructure/postgres/receipts"
	salesinfra "orderapp/internal/infrastructure/postgres/sales"
)

type billingOrderSnapshot struct {
	LegacyCash   int64  `json:"legacy_cash"`
	Offset       int64  `json:"offset"`
	SettledOut   int64  `json:"settled_out"`
	Status       int64  `json:"status"`
	Method       string `json:"method"`
	Prepayment   int64  `json:"prepayment"`
	Goods        int64  `json:"goods"`
	ShippingPaid int64  `json:"shipping_paid"`
	Total        int64  `json:"total"`
	Shipping     int64  `json:"shipping"`
}
type billingOrderAllocation struct {
	OrderID        int64                `json:"order_id"`
	Amount         int64                `json:"amount"`
	Before         billingOrderSnapshot `json:"before"`
	After          billingOrderSnapshot `json:"after"`
	AuditWatermark int64                `json:"audit_watermark"`
}

// ChangeBillingPayment serializes the customer, bill, and source orders. The
// allocation snapshots make reversal conservative in the presence of other
// writers; no unrelated payment is silently removed.
func (r *Repository) ChangeBillingPayment(ctx context.Context, cmd app.BillingPaymentCommand) (app.BillingBill, error) {
	if cmd.ReceiptDate != "" && (cmd.Action != "pay" || salesapp.ValidateReceiptDate(cmd.ReceiptDate) != nil) {
		return app.BillingBill{}, fmt.Errorf("实际到账日期无效或晚于今天")
	}
	if cmd.Action != "pay" && (cmd.ActualCents != nil || len(cmd.Allocations) > 0) {
		return app.BillingBill{}, fmt.Errorf("撤销收款不能指定金额或分配")
	}
	if cmd.ActualCents != nil && (*cmd.ActualCents <= 0 || cmd.ExpectedDueAmountCents == nil || *cmd.ActualCents > *cmd.ExpectedDueAmountCents) {
		return app.BillingBill{}, fmt.Errorf("实际收款金额无效")
	}
	allocationHash := ""
	if len(cmd.Allocations) > 0 {
		encoded, e := json.Marshal(cmd.Allocations)
		if e != nil {
			return app.BillingBill{}, e
		}
		digest := sha256.Sum256(encoded)
		allocationHash = hex.EncodeToString(digest[:])
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.BillingBill{}, err
	}
	defer tx.Rollback(ctx)
	for _, key := range []string{fmt.Sprintf("mini-direct-ship:%d", cmd.CustomerID), r.schema + ":finance-period"} {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
			return app.BillingBill{}, err
		}
	}
	var id int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.customers WHERE id=$1 FOR UPDATE`, r.schema), cmd.CustomerID).Scan(&id); err != nil {
		return app.BillingBill{}, fmt.Errorf("客户不存在")
	}
	var status string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_settlement_batches WHERE id=$1 AND customer_id=$2 AND status IN ('confirmed','paid','settled') AND COALESCE(processing_billing_run_id,0)=0 FOR UPDATE`, r.schema), cmd.BatchID, cmd.CustomerID).Scan(&status); err != nil {
		return app.BillingBill{}, fmt.Errorf("账单不存在或不可操作")
	}
	var previousAction string
	var previousExpected int64
	var previousRevision int64
	var resultRevision int64
	var previousMethod, previousDate, previousAllocationHash string
	var previousActual *int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT action,expected_due_amount_cents,expected_payment_revision,payment_method,result_payment_revision,receipt_date,actual_cents,allocation_hash FROM %s.customer_settlement_payment_keys WHERE customer_id=$1 AND settlement_batch_id=$2 AND idempotency_key=$3`, r.schema), cmd.CustomerID, cmd.BatchID, cmd.IdempotencyKey).Scan(&previousAction, &previousExpected, &previousRevision, &previousMethod, &resultRevision, &previousDate, &previousActual, &previousAllocationHash)
	if err == nil {
		if previousAction != cmd.Action || cmd.ExpectedDueAmountCents == nil || previousExpected != *cmd.ExpectedDueAmountCents || cmd.ExpectedPaymentRevision == nil || previousRevision != *cmd.ExpectedPaymentRevision || previousMethod != strings.TrimSpace(cmd.PaymentMethod) || previousDate != cmd.ReceiptDate || previousAllocationHash != allocationHash || !sameOptionalCents(previousActual, cmd.ActualCents) {
			return app.BillingBill{}, fmt.Errorf("收款操作标识已用于其他操作")
		}
		bill, err := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
		if err != nil {
			return app.BillingBill{}, err
		}
		if bill.PaymentRevision != resultRevision {
			return app.BillingBill{}, fmt.Errorf("该操作已处理，账单状态已变化，请刷新核对")
		}
		return bill, nil
	}
	if err != pgx.ErrNoRows {
		return app.BillingBill{}, err
	}
	if err = receipts.EnsureDateOpenTx(ctx, tx, r.schema, cmd.ReceiptDate); err != nil {
		return app.BillingBill{}, err
	}
	finish := func() (app.BillingBill, error) {
		result, err := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
		if err != nil {
			return app.BillingBill{}, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_payment_keys(customer_id,settlement_batch_id,idempotency_key,action,expected_due_amount_cents,expected_payment_revision,payment_method,result_payment_revision,receipt_date,actual_cents,allocation_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, r.schema), cmd.CustomerID, cmd.BatchID, cmd.IdempotencyKey, cmd.Action, *cmd.ExpectedDueAmountCents, *cmd.ExpectedPaymentRevision, strings.TrimSpace(cmd.PaymentMethod), result.PaymentRevision, cmd.ReceiptDate, cmd.ActualCents, allocationHash); err != nil {
			return app.BillingBill{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return app.BillingBill{}, err
		}
		return result, nil
	}
	var paymentID int64
	var raw []byte
	var activeActual *int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,allocations,actual_cents FROM %s.customer_settlement_payments WHERE customer_id=$1 AND settlement_batch_id=$2 AND reversed_at IS NULL ORDER BY id DESC LIMIT 1 FOR UPDATE`, r.schema), cmd.CustomerID, cmd.BatchID).Scan(&paymentID, &raw, &activeActual)
	if err != nil && err != pgx.ErrNoRows {
		return app.BillingBill{}, err
	}
	active := err == nil
	if active && status != "paid" && !(status == "confirmed" && activeActual != nil) {
		return app.BillingBill{}, fmt.Errorf("账单状态与收款记录不一致，请核对")
	}
	if cmd.Action == "reverse" && active {
		var receiptDate string
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(receipt_date::text,'') FROM %s.cash_receipts WHERE source_type='billing_payment' AND source_id=$1`, r.schema), paymentID).Scan(&receiptDate)
		if err != nil && err != pgx.ErrNoRows {
			return app.BillingBill{}, err
		}
		if err = receipts.EnsureDateOpenTx(ctx, tx, r.schema, receiptDate); err != nil {
			return app.BillingBill{}, err
		}
	}
	if cmd.Action == "pay" && active {
		bill, e := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
		if e != nil {
			return app.BillingBill{}, e
		}
		if bill.DueAmountCents > 0 && !bill.PaymentAdditionalAllowed {
			return app.BillingBill{}, fmt.Errorf("账单收款后金额发生变化，请核对账单和订单")
		}
		if cmd.ExpectedPaymentRevision == nil || bill.PaymentRevision != *cmd.ExpectedPaymentRevision {
			return app.BillingBill{}, fmt.Errorf("账单付款状态已变化，请刷新核对")
		}
		if bill.DueAmountCents == 0 {
			return finish()
		}
	}
	if cmd.Action == "reverse" && !active {
		if status == "paid" || status == "settled" {
			return app.BillingBill{}, fmt.Errorf("历史账单没有可核对的收款记录，不能自动撤销")
		}
		var reversed bool
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customer_settlement_payments WHERE customer_id=$1 AND settlement_batch_id=$2 AND reversed_at IS NOT NULL)`, r.schema), cmd.CustomerID, cmd.BatchID).Scan(&reversed); err != nil {
			return app.BillingBill{}, err
		}
		if !reversed {
			return app.BillingBill{}, fmt.Errorf("账单没有本功能登记的收款记录")
		}
		bill, e := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
		if e != nil {
			return app.BillingBill{}, e
		}
		if cmd.ExpectedPaymentRevision == nil || bill.PaymentRevision != *cmd.ExpectedPaymentRevision {
			return app.BillingBill{}, fmt.Errorf("账单付款状态已变化，请刷新核对")
		}
		return finish()
	}
	if cmd.Action == "pay" && status != "confirmed" && !(status == "paid" && active) {
		return app.BillingBill{}, fmt.Errorf("历史账单状态不能自动登记收款，请先核对")
	}
	var allocs []billingOrderAllocation
	if cmd.Action == "reverse" {
		if err = json.Unmarshal(raw, &allocs); err != nil {
			return app.BillingBill{}, fmt.Errorf("收款记录无效，请核对")
		}
	} else {
		lines, e := listSettlementLines(ctx, tx, r.schema, cmd.CustomerID, cmd.BatchID)
		if e != nil {
			return app.BillingBill{}, e
		}
		ids := make(map[int64]bool)
		for _, line := range lines {
			if line.ChargeType == "order_goods" && line.SourceType == "order" {
				ids[line.SourceID] = true
			}
		}
		for orderID := range ids {
			allocs = append(allocs, billingOrderAllocation{OrderID: orderID})
		}
	}
	sort.Slice(allocs, func(i, j int) bool { return allocs[i].OrderID < allocs[j].OrderID })
	snapshots := make(map[int64]billingOrderSnapshot, len(allocs))
	for _, a := range allocs {
		snap, e := r.lockBillingOrder(ctx, tx, cmd.CustomerID, a.OrderID)
		if e != nil {
			return app.BillingBill{}, e
		}
		snapshots[a.OrderID] = snap
	}
	currentBill, err := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
	if err != nil {
		return app.BillingBill{}, err
	}
	if cmd.ExpectedDueAmountCents == nil || currentBill.DueAmountCents != *cmd.ExpectedDueAmountCents || cmd.ExpectedPaymentRevision == nil || currentBill.PaymentRevision != *cmd.ExpectedPaymentRevision {
		return app.BillingBill{}, fmt.Errorf("账单未收金额已变化，请刷新并重新核对")
	}
	expectedAfter := int64(-1)
	auditIDs := map[int64][]int64{}
	if cmd.Action == "pay" {
		actual := currentBill.DueAmountCents
		if cmd.ActualCents != nil {
			actual = *cmd.ActualCents
		}
		if actual <= 0 || actual > currentBill.DueAmountCents {
			return app.BillingBill{}, fmt.Errorf("实际收款金额无效")
		}
		if actual < currentBill.DueAmountCents && currentBill.AdjustmentCents > 0 {
			return app.BillingBill{}, fmt.Errorf("账单含调整，部分收款请先核对")
		}
		selected := map[int64]int64{}
		if len(cmd.Allocations) > 0 {
			var sum int64
			for _, a := range cmd.Allocations {
				if a.LineID <= 0 || a.AmountCents <= 0 || selected[a.LineID] != 0 || a.AmountCents > actual-sum {
					return app.BillingBill{}, fmt.Errorf("账单行分配无效")
				}
				selected[a.LineID] = a.AmountCents
				sum += a.AmountCents
			}
			if sum != actual {
				return app.BillingBill{}, fmt.Errorf("账单行分配合计须等于本次收款")
			}
		} else if actual != currentBill.DueAmountCents {
			return app.BillingBill{}, fmt.Errorf("部分收款须明确分配到账单明细")
		}
		lineAllocations := make([]app.BillingPaymentLineAllocation, 0, len(currentBill.Lines))
		due := make(map[int64]int64)
		for _, line := range currentBill.Lines {
			amount := line.DueCents
			if len(cmd.Allocations) > 0 {
				amount = selected[line.ID]
				delete(selected, line.ID)
			}
			if amount < 0 || amount > line.DueCents {
				return app.BillingBill{}, fmt.Errorf("账单明细待收已变化，请刷新核对")
			}
			if amount > 0 {
				lineAllocations = append(lineAllocations, app.BillingPaymentLineAllocation{LineID: line.ID, AmountCents: amount})
			}
			if line.ChargeType == "order_goods" && line.SourceType == "order" {
				due[line.SourceID] += amount
			}
		}
		if len(selected) > 0 {
			return app.BillingBill{}, fmt.Errorf("分配包含不属于账单的明细")
		}
		expectedAfter = currentBill.DueAmountCents - actual
		for i := range allocs {
			a := &allocs[i]
			a.Before = snapshots[a.OrderID]
			a.Amount = due[a.OrderID]
			a.After = a.Before
			if a.Amount == 0 {
				continue
			}
			goodsDue := max(int64(0), a.Before.Total-a.Before.Shipping)
			goods, shipping := a.Before.Goods, a.Before.ShippingPaid
			if goods == 0 && shipping == 0 && a.Before.Prepayment > 0 {
				goods = min(goodsDue, a.Before.Prepayment)
				shipping = min(a.Before.Shipping, max(int64(0), a.Before.Prepayment-goods))
			}
			if goods+shipping < a.Before.Prepayment {
				return app.BillingBill{}, fmt.Errorf("订单预付款与实收明细不一致，请先核对")
			}
			effective := max(int64(0), goods+shipping+a.Before.LegacyCash+a.Before.Offset-a.Before.SettledOut)
			if goods < 0 || shipping < 0 || effective+a.Amount > a.Before.Total || a.Before.Total < 0 {
				return app.BillingBill{}, fmt.Errorf("订单收款或应收金额已变化，请核对")
			}
			a.After.Goods = goods + a.Amount
			a.After.ShippingPaid = shipping
			if strings.TrimSpace(a.After.Method) == "" {
				a.After.Method = strings.TrimSpace(cmd.PaymentMethod)
			}
			if a.After.Method == "" {
				return app.BillingBill{}, fmt.Errorf("请填写实际付款方式")
			}
			totalPaid := a.After.Goods + a.After.ShippingPaid
			if totalPaid+a.Before.LegacyCash+a.Before.Offset-a.Before.SettledOut > a.Before.Total {
				return app.BillingBill{}, fmt.Errorf("订单收款超过应收，请核对")
			}
			a.After.Prepayment = totalPaid
			name := "预付款（付款未完成）"
			if totalPaid+a.Before.LegacyCash+a.Before.Offset-a.Before.SettledOut >= a.Before.Total {
				name = "已付款"
			}
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.pay_statuses WHERE name=$1 ORDER BY id LIMIT 1`, r.schema), name).Scan(&a.After.Status); err != nil {
				return app.BillingBill{}, fmt.Errorf("缺少付款状态 %s", name)
			}
			ids := auditIDs[a.OrderID]
			if err = r.writeBillingOrder(ctx, tx, cmd, a.OrderID, a.Before, a.After, &ids); err != nil {
				return app.BillingBill{}, err
			}
			auditIDs[a.OrderID] = ids
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(id),0) FROM %s.order_audit_logs WHERE order_id=$1`, r.schema), a.OrderID).Scan(&a.AuditWatermark); err != nil {
				return app.BillingBill{}, err
			}
		}
		data, e := json.Marshal(allocs)
		if e != nil {
			return app.BillingBill{}, e
		}
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_payments(customer_id,settlement_batch_id,allocations,actor,actual_cents,fully_settled) VALUES($1,$2,$3::jsonb,$4,$5,$6) RETURNING id`, r.schema), cmd.CustomerID, cmd.BatchID, data, cmd.Actor, actual, expectedAfter == 0).Scan(&paymentID); err != nil {
			return app.BillingBill{}, err
		}
		if err = r.saveBillingAuditRefs(ctx, tx, paymentID, auditIDs); err != nil {
			return app.BillingBill{}, err
		}
		for _, a := range lineAllocations {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_payment_line_allocations(payment_id,line_id,amount_cents) VALUES($1,$2,$3)`, r.schema), paymentID, a.LineID, a.AmountCents); err != nil {
				return app.BillingBill{}, err
			}
		}
		if currentBill.DueAmountCents > 0 {
			receiptAllocations, e := json.Marshal(lineAllocations)
			if e != nil {
				return app.BillingBill{}, e
			}
			receiptID, receiptNo, receiptDate, e := receipts.RecordTx(ctx, tx, r.schema, receipts.Receipt{CustomerID: cmd.CustomerID, SourceType: "billing_payment", SourceID: paymentID, ActualCents: actual, ReceivableCents: currentBill.TotalAmountCents, ReceiptDate: cmd.ReceiptDate, Method: cmd.PaymentMethod, Actor: cmd.Actor, Allocations: receiptAllocations})
			if e != nil {
				return app.BillingBill{}, e
			}
			if e = messagepg.RecordBusinessEvent(ctx, tx, r.schema, "receipt_received", fmt.Sprintf("receipt_received:%d", receiptID), 0, cmd.CustomerID, map[string]any{"receipt_id": receiptID, "receipt_no": receiptNo, "receipt_date": receiptDate, "amount_cents": actual, "receivable_cents": currentBill.TotalAmountCents, "bill_id": cmd.BatchID}); e != nil {
				return app.BillingBill{}, e
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status=CASE WHEN $2 THEN 'paid' ELSE 'confirmed' END,paid_at=CASE WHEN $2 THEN now() ELSE NULL END WHERE id=$1`, r.schema), cmd.BatchID, expectedAfter == 0); err != nil {
			return app.BillingBill{}, err
		}
	} else {
		for _, a := range allocs {
			if a.Amount == 0 {
				continue
			}
			if snapshots[a.OrderID] != a.After {
				return app.BillingBill{}, fmt.Errorf("关联订单收款或金额在登记后发生变化，不能自动撤销，请核对")
			}
			var laterChange bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.order_audit_logs l WHERE l.order_id=$1 AND l.id>$2 AND l.field IN ('pay_status_id','payment_method','prepayment_amount','payment_goods_amount','payment_shipping_amount','payment_voucher_asset_id','order','header') AND NOT EXISTS(SELECT 1 FROM %[1]s.customer_settlement_payment_order_audit_refs ref JOIN %[1]s.customer_settlement_payments p ON p.id=ref.payment_id WHERE ref.order_audit_id=l.id AND ref.order_id=l.order_id AND p.settlement_batch_id=$3 AND p.customer_id=$4 AND p.reversed_at IS NOT NULL))`, r.schema), a.OrderID, a.AuditWatermark, cmd.BatchID, cmd.CustomerID).Scan(&laterChange); err != nil {
				return app.BillingBill{}, err
			}
			if laterChange {
				return app.BillingBill{}, fmt.Errorf("关联订单收款在登记后发生变化，不能自动撤销，请核对")
			}
			ids := auditIDs[a.OrderID]
			if err = r.writeBillingOrder(ctx, tx, cmd, a.OrderID, a.After, a.Before, &ids); err != nil {
				return app.BillingBill{}, err
			}
			auditIDs[a.OrderID] = ids
		}
		if err = r.saveBillingAuditRefs(ctx, tx, paymentID, auditIDs); err != nil {
			return app.BillingBill{}, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_payments SET reversed_at=now(),reversed_by=$2 WHERE id=$1`, r.schema), paymentID, cmd.Actor); err != nil {
			return app.BillingBill{}, err
		}
		var receiptID int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.cash_receipts SET reversed_at=now(),reversed_by=$2 WHERE source_type='billing_payment' AND source_id=$1 AND reversed_at IS NULL RETURNING id`, r.schema), paymentID, cmd.Actor).Scan(&receiptID)
		if err != nil && err != pgx.ErrNoRows {
			return app.BillingBill{}, err
		}
		if err == nil {
			if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "cash_receipt", &receiptID, "reverse", nil, nil, nil, postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "settlement_id": cmd.BatchID, "source_payment_id": paymentID}); err != nil {
				return app.BillingBill{}, err
			}
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.customer_settlement_batches SET status=CASE WHEN EXISTS(SELECT 1 FROM %[1]s.customer_settlement_payments WHERE settlement_batch_id=$1 AND reversed_at IS NULL AND fully_settled)THEN 'paid' ELSE 'confirmed' END,paid_at=CASE WHEN EXISTS(SELECT 1 FROM %[1]s.customer_settlement_payments WHERE settlement_batch_id=$1 AND reversed_at IS NULL AND fully_settled)THEN paid_at ELSE NULL END WHERE id=$1`, r.schema), cmd.BatchID); err != nil {
			return app.BillingBill{}, err
		}
	}
	if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "customer_fulfillment_settlement", &cmd.BatchID, cmd.Action, nil, nil, nil, postgresinfra.AuditMeta{"customer_id": cmd.CustomerID, "settlement_id": cmd.BatchID}); err != nil {
		return app.BillingBill{}, err
	}
	result, err := r.getIssuedBill(ctx, tx, cmd.CustomerID, cmd.BatchID, nil)
	if err != nil {
		return app.BillingBill{}, err
	}
	if cmd.Action == "pay" && result.DueAmountCents != expectedAfter {
		return app.BillingBill{}, fmt.Errorf("账单收款后待收金额不一致，请核对")
	}
	return finish()
}

func sameOptionalCents(a, b *int64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func (r *Repository) lockBillingOrder(ctx context.Context, tx pgx.Tx, customerID, orderID int64) (billingOrderSnapshot, error) {
	var s billingOrderSnapshot
	var void, unaccepted bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(pay_status_id,0),COALESCE(payment_method,''),round(COALESCE(prepayment_amount,0)*100)::bigint,round(COALESCE(payment_goods_amount,0)*100)::bigint,round(COALESCE(payment_shipping_amount,0)*100)::bigint,round((CASE WHEN COALESCE(grand_total,0)<>0 OR COALESCE(NULLIF(to_jsonb(o)->>'discount_amount','')::numeric,0)<>0 OR COALESCE(shipping_amount,0)<>0 OR COALESCE(NULLIF(to_jsonb(o)->>'rounding_amount','')::numeric,0)<>0 THEN COALESCE(grand_total,0) ELSE COALESCE(total_amount,0) END)*100)::bigint-COALESCE((to_jsonb(o)->>'resolution_credit_cents')::bigint,0),GREATEST(0,round(COALESCE(shipping_amount,0)*100)::bigint-COALESCE((to_jsonb(o)->>'resolution_shipping_credit_cents')::bigint,0)),COALESCE((to_jsonb(o)->>'resolution_legacy_cash_cents')::bigint,0),COALESCE((to_jsonb(o)->>'resolution_offset_cents')::bigint,0),COALESCE((to_jsonb(o)->>'resolution_settled_out_cents')::bigint,0),COALESCE(is_void,false),COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false) AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')<>'accepted' FROM %s.orders o WHERE id=$1 AND customer_id=$2 FOR UPDATE`, r.schema), orderID, customerID).Scan(&s.Status, &s.Method, &s.Prepayment, &s.Goods, &s.ShippingPaid, &s.Total, &s.Shipping, &s.LegacyCash, &s.Offset, &s.SettledOut, &void, &unaccepted)
	if err != nil || void || unaccepted {
		return s, fmt.Errorf("关联订单不存在、已作废或尚未确认，请核对")
	}
	return s, nil
}

func (r *Repository) writeBillingOrder(ctx context.Context, tx pgx.Tx, cmd app.BillingPaymentCommand, orderID int64, before, after billingOrderSnapshot, auditIDs *[]int64) error {
	if before == after {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET pay_status_id=$2,prepayment_amount=$3::numeric/100,payment_goods_amount=$4::numeric/100,payment_shipping_amount=$5::numeric/100,payment_method=$6 WHERE id=$1`, r.schema), orderID, after.Status, after.Prepayment, after.Goods, after.ShippingPaid, after.Method)
	if err != nil {
		return err
	}
	if before.Method != after.Method {
		var auditID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,$2,'payment_method',$3,$4) RETURNING id`, r.schema), orderID, cmd.Actor, before.Method, after.Method).Scan(&auditID); err != nil {
			return err
		}
		*auditIDs = append(*auditIDs, auditID)
		field := "payment_method"
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order", &orderID, "update", &field, &before.Method, &after.Method, postgresinfra.AuditMeta{"order_id": orderID, "source": "fulfillment-bill-payment", "settlement_id": cmd.BatchID}); err != nil {
			return err
		}
	}
	for _, d := range []struct {
		field    string
		old, new int64
	}{{"pay_status_id", before.Status, after.Status}, {"prepayment_amount", before.Prepayment, after.Prepayment}, {"payment_goods_amount", before.Goods, after.Goods}, {"payment_shipping_amount", before.ShippingPaid, after.ShippingPaid}} {
		if d.old == d.new {
			continue
		}
		old, new := strconv.FormatInt(d.old, 10), strconv.FormatInt(d.new, 10)
		if d.field != "pay_status_id" {
			old = fmt.Sprintf("%.2f", float64(d.old)/100)
			new = fmt.Sprintf("%.2f", float64(d.new)/100)
		}
		var auditID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id,actor,field,old_value,new_value) VALUES($1,$2,$3,$4,$5) RETURNING id`, r.schema), orderID, cmd.Actor, d.field, old, new).Scan(&auditID); err != nil {
			return err
		}
		*auditIDs = append(*auditIDs, auditID)
		if err = postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "order", &orderID, "update", postgresinfra.StrPtr(d.field), postgresinfra.StrPtr(old), postgresinfra.StrPtr(new), postgresinfra.AuditMeta{"order_id": orderID, "source": "fulfillment-bill-payment", "settlement_id": cmd.BatchID}); err != nil {
			return err
		}
	}
	return salesinfra.InvalidateConfirmedOrderDocumentsTx(ctx, tx, r.schema, orderID)
}

func (r *Repository) saveBillingAuditRefs(ctx context.Context, tx pgx.Tx, paymentID int64, refs map[int64][]int64) error {
	for orderID, ids := range refs {
		for _, id := range ids {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_settlement_payment_order_audit_refs(payment_id,order_id,order_audit_id) VALUES($1,$2,$3)`, r.schema), paymentID, orderID, id); err != nil {
				return err
			}
		}
	}
	return nil
}
