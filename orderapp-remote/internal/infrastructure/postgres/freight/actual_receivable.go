package freight

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/freight"
	salesdomain "orderapp/internal/domain/sales"
)

// ReceivableState counts each existing shipping charge once, irrespective of order type.
type ReceivableState struct {
	Current, Paid, Surcharge int64
	Free, Unknown            bool
}

func (r *Repository) ReceivableState(ctx context.Context, tx interface {
	Queryer
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, oid int64) (ReceivableState, error) {
	var s ReceivableState
	var raw []byte
	var base, prepay, goods int64
	var status string
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT round(coalesce(o.shipping_amount,0)*100)::bigint,round(coalesce(o.payment_shipping_amount,0)*100)::bigint,round(coalesce((to_jsonb(o)->>'prepayment_amount')::numeric,0)*100)::bigint,round(coalesce(o.payment_goods_amount,0)*100)::bigint,coalesce(p.name,''),coalesce(f.snapshot_json,'{}') FROM %[1]s.orders o LEFT JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id LEFT JOIN %[1]s.order_freight_snapshots f ON f.order_id=o.id WHERE o.id=$1`, r.schema), oid).Scan(&base, &s.Paid, &prepay, &goods, &status, &raw)
	if e != nil {
		return s, e
	}
	var q domain.Snapshot
	if e = json.Unmarshal(raw, &q); e != nil {
		return s, e
	}
	s.Current = base
	s.Surcharge = q.SurchargeCents
	s.Free = q.Policy.FreeShipping || q.ChargeMode == "free" || (q.ChargeMode == "manual_current" && q.ManualAmountCents != nil && *q.ManualAmountCents == 0)
	if q.Protocol == "" {
		s.Free = base == 0
		e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT coalesce(p.surcharge_cents,sys.surcharge_cents,0) FROM %[1]s.orders o LEFT JOIN %[1]s.customer_shipping_policies p ON p.customer_id=o.customer_id AND p.scene=CASE WHEN coalesce(to_jsonb(o)->>'fulfillment_mode','') IN ('stock','on_demand') THEN to_jsonb(o)->>'fulfillment_mode' ELSE 'sale' END LEFT JOIN %[1]s.system_shipping_policy sys ON sys.singleton=true WHERE o.id=$1`, r.schema), oid).Scan(&s.Surcharge)
		if e != nil {
			return s, e
		}
	}
	// A legacy full payment covers the freight already included in that order, never deferred charges.
	if salesdomain.IsFullyPaid(status) && base > 0 && s.Paid == 0 {
		s.Paid = base
	}
	s.Unknown = prepay > 0 && goods == 0 && s.Paid == 0
	rows, e := tx.Query(ctx, fmt.Sprintf(`SELECT f.source_type,f.source_id,round(f.amount*100)::bigint,coalesce(b.status,''),b.paid_at IS NOT NULL,coalesce(f.status,'unsettled') FROM %[1]s.customer_fee_items f LEFT JOIN %[1]s.freight_actual_entries e ON f.source_type='freight_actual' AND e.id=f.source_id LEFT JOIN %[1]s.freight_actual_shipments a ON a.id=e.shipment_id LEFT JOIN %[1]s.customer_settlement_batches b ON b.id=f.settlement_batch_id AND b.customer_id=f.customer_id WHERE f.customer_id=(SELECT customer_id FROM %[1]s.orders WHERE id=$1) AND f.fee_type='shipping' AND ((f.source_type='order' AND f.source_id=$1) OR a.order_id=$1) ORDER BY f.id`, r.schema), oid)
	if e != nil {
		return s, e
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var source, status, feeStatus string
		var id, amount int64
		var paid bool
		if e = rows.Scan(&source, &id, &amount, &status, &paid, &feeStatus); e != nil {
			return s, e
		}
		key := fmt.Sprintf("%s:%d", source, id)
		if seen[key] {
			continue
		}
		seen[key] = true
		duplicatePrincipal := source == "order" && base != 0
		if !duplicatePrincipal {
			s.Current += amount
		}
		if status == "paid" || (paid && status != "reversed") {
			if duplicatePrincipal {
				s.Paid = max(s.Paid, amount)
			} else {
				s.Paid += amount
			}
		}
		if (status == "settled" || (status == "" && feeStatus != "unsettled")) && !paid {
			s.Unknown = true
		}
	}
	if q.Protocol == "" && base == 0 && s.Current > 0 {
		s.Free = false
	}
	return s, rows.Err()
}
func (r *Repository) ConfirmActualCostTx(ctx context.Context, tx pgx.Tx, oid, carrier int64, tracking string, actual int64, key, actor string) (string, error) {
	var oldKey string
	e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_key FROM %s.freight_actual_entries WHERE request_key=$1`, r.schema), key).Scan(&oldKey)
	if e == nil {
		return "已处理", nil
	}
	if e != pgx.ErrNoRows {
		return "", e
	}
	if e = r.lockOrderReceivable(ctx, tx, oid); e != nil {
		return "", e
	}
	s, e := r.ReceivableState(ctx, tx, oid)
	if e != nil {
		return "", e
	}
	d, e := domain.ConfirmActualReceivable(domain.ActualReceivableInput{Actual: actual, Surcharge: s.Surcharge, Current: s.Current, Paid: s.Paid, Free: s.Free, UnknownPayment: s.Unknown})
	if e != nil {
		return "", e
	}
	var cid int64
	if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT customer_id FROM %s.orders WHERE id=$1`, r.schema), oid).Scan(&cid); e != nil {
		return "", e
	}
	var sid, owner, prevActual, prevBilled int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_shipments(carrier_id,tracking_no,order_id,customer_id)VALUES($1,$2,$3,$4) ON CONFLICT(carrier_id,tracking_no) DO UPDATE SET tracking_no=excluded.tracking_no RETURNING id,order_id,actual_cents,billed_cents`, r.schema), carrier, tracking, oid, cid).Scan(&sid, &owner, &prevActual, &prevBilled)
	if e != nil {
		return "", e
	}
	if owner != oid {
		return "", fmt.Errorf("运单已经关联其他订单")
	}
	if e = r.lockShipmentFees(ctx, tx, sid); e != nil {
		return "", e
	}
	delta := d.TargetCents - s.Current
	var eid int64
	e = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.freight_actual_entries(shipment_id,request_key,actual_cents,target_cents,delta_cents,previous_actual_cents,previous_billed_cents,reason,created_by)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)RETURNING id`, r.schema), sid, key, actual, prevBilled+delta, delta, prevActual, prevBilled, d.Reason, actor).Scan(&eid)
	if e != nil {
		return "", e
	}
	if delta != 0 {
		_, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_fee_items(customer_id,source_type,source_id,fee_type,amount,note)VALUES($1,'freight_actual',$2,'shipping',$3,$4)`, r.schema), cid, eid, float64(delta)/100, "寄件实际运费调整 · "+tracking)
		if e != nil {
			return "", e
		}
	}
	_, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.freight_actual_shipments SET actual_cents=$2,billed_cents=$3,revision=revision+1 WHERE id=$1`, r.schema), sid, actual, prevBilled+delta)
	if e != nil {
		return "", e
	}
	e = r.Audit(ctx, tx, actor, "freight_actual", eid, "confirm", s, map[string]any{"actual_cents": actual, "target_cents": d.TargetCents, "delta_cents": delta, "reason": d.Reason, "request_key": key})
	return d.Reason, e
}

// Lock fee membership before its payment batches. NOWAIT prevents a deadlock
// with settlement creation (batch -> fees); retrying observes the committed payment.
func (r *Repository) lockOrderReceivable(ctx context.Context, tx pgx.Tx, oid int64) error {
	clause := fmt.Sprintf(`f.customer_id=(SELECT customer_id FROM %[1]s.orders WHERE id=$1) AND f.fee_type='shipping' AND ((f.source_type='order' AND f.source_id=$1) OR (f.source_type='freight_actual' AND f.source_id IN(SELECT e.id FROM %[1]s.freight_actual_entries e JOIN %[1]s.freight_actual_shipments a ON a.id=e.shipment_id WHERE a.order_id=$1)))`, r.schema)
	queries := []string{
		fmt.Sprintf(`SELECT f.id FROM %s.customer_fee_items f WHERE %s ORDER BY f.id FOR UPDATE OF f`, r.schema, clause),
		fmt.Sprintf(`SELECT b.id FROM %[1]s.customer_settlement_batches b WHERE b.id IN(SELECT f.settlement_batch_id FROM %[1]s.customer_fee_items f WHERE %[2]s) ORDER BY b.id FOR UPDATE OF b NOWAIT`, r.schema, clause),
	}
	for _, query := range queries {
		rows, err := tx.Query(ctx, query, oid)
		if err != nil {
			return fmt.Errorf("收款或结算记录正在更新，请刷新重试: %w", err)
		}
		for rows.Next() {
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
