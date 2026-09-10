package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/customerfulfillment"
	"orderapp/internal/infrastructure/postgres/orderconfirmation"
	"strings"
	"time"
)

// CustomerAccount reads a consistent view of existing sales and fee documents.
// Settlement batches are references to fees, never additional order receivables.
func (r *Repository) CustomerAccount(ctx context.Context, q app.AccountQuery) (app.AccountData, error) {
	d := app.AccountData{Rows: []app.AccountOrder{}, Fees: []app.AccountFee{}, Settlements: []app.AccountSettlement{}, DateFrom: q.DateFrom, DateTo: q.DateTo, AsOf: time.Now().In(time.FixedZone("Asia/Shanghai", 28800)).Format("2006-01-02 15:04:05")}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return d, err
	}
	defer tx.Rollback(ctx)
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, r.schema), q.CustomerID).Scan(&d.CustomerName); err != nil {
		return d, err
	}
	where := []string{"o.customer_id=$1"}
	if !q.CurrentVersion {
		where = append(where, "(NOT COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false) OR COALESCE((to_jsonb(o)->>'confirmation_accepted_revision')::bigint,0)>0)")
	}
	currentQuery := func(query string) string {
		if q.CurrentVersion {
			return orderconfirmation.CurrentRead(query, r.schema)
		}
		return query
	}
	args := []any{q.CustomerID}
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if !q.IncludeVoid {
		where = append(where, "coalesce(o.is_void,false)=false")
	}
	if q.OrderID > 0 {
		add("o.id=$%d", q.OrderID)
	}
	if q.DateFrom != "" {
		add("o.order_date >= $%d::date", q.DateFrom)
	}
	if q.DateTo != "" {
		add("o.order_date <= $%d::date", q.DateTo)
	}
	if q.ShipStatus != "" {
		add("coalesce(ss.name,'')=$%d", q.ShipStatus)
	}
	if q.Query != "" {
		args = append(args, "%"+q.Query+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(`(o.order_no ILIKE $%[1]d OR o.receiver_name ILIKE $%[1]d OR o.receiver_phone ILIKE $%[1]d OR EXISTS(SELECT 1 FROM %[2]s.order_items i WHERE i.order_id=o.id AND i.item_name ILIKE $%[1]d))`, n, r.schema))
	}
	rows, err := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT o.id,coalesce(o.order_no,''),coalesce(to_char(o.order_date,'YYYY-MM-DD'),''),coalesce(o.receiver_name,''),coalesce(o.receiver_phone,''),coalesce(o.receiver_address,''),coalesce(ss.name,''),coalesce(o.ship_tracking_no,''),coalesce(ps.name,''),coalesce(o.portal_service_code,''),coalesce(o.is_void,false),round(coalesce(o.grand_total,0)*100)::bigint,round(coalesce(o.shipping_amount,0)*100)::bigint,round(coalesce(o.discount_amount,0)*100)::bigint,round(coalesce((to_jsonb(o)->>'prepayment_amount')::numeric,0)*100)::bigint,COALESCE(to_jsonb(o)->>'confirmation_status','accepted'),COALESCE((to_jsonb(o)->>'confirmation_required')::boolean,false),COALESCE((to_jsonb(o)->>'confirmation_accepted_revision')::bigint,0),COALESCE(ops.name,'') FROM %[1]s.orders o LEFT JOIN %[1]s.ship_statuses ss ON ss.id=o.ship_status_id LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id LEFT JOIN %[1]s.order_process_statuses ops ON ops.id=o.process_status_id WHERE %[2]s ORDER BY o.order_date DESC NULLS LAST,o.id DESC`, r.schema, strings.Join(where, " AND "))), args...)
	if err != nil {
		return d, err
	}
	for rows.Next() {
		var o app.AccountOrder
		if err = rows.Scan(&o.ID, &o.OrderNo, &o.OrderDate, &o.ReceiverName, &o.ReceiverPhone, &o.ReceiverAddress, &o.ShipStatus, &o.TrackingNo, &o.PayStatus, &o.Service, &o.IsVoid, &o.TotalCents, &o.ShippingCents, &o.DiscountCents, &o.PrepaymentCents, &o.ConfirmationStatus, &o.ConfirmationRequired, &o.AcceptedRevision, &o.ProcessStatus); err != nil {
			rows.Close()
			return d, err
		}
		o.GoodsCents = o.TotalCents - o.ShippingCents + o.DiscountCents
		o.SetPayment()
		if q.PayStatus != "" && o.PaymentStatus != q.PayStatus {
			continue
		}
		d.Rows = append(d.Rows, o)
		if !o.IsVoid {
			d.Summary.Count++
			d.Summary.GoodsCents += o.GoodsCents
			d.Summary.ShippingCents += o.ShippingCents
			d.Summary.DiscountCents += o.DiscountCents
			d.Summary.TotalCents += o.TotalCents
			d.Summary.PaidCents += o.PaidCents
			d.Summary.DueCents += o.DueCents
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return d, err
	}
	if q.OrderID > 0 && len(d.Rows) > 0 {
		items, err := tx.Query(ctx, currentQuery(fmt.Sprintf(`SELECT coalesce(item_name,''),coalesce(spec,''),coalesce(qty,0)::text,coalesce(unit,''),coalesce(unit_price,0)::text,coalesce(line_total,0)::text FROM %s.order_items current_item WHERE order_id=$1 ORDER BY id`, r.schema)), q.OrderID)
		if err != nil {
			return d, err
		}
		d.Rows[0].Items = []app.AccountOrderItem{}
		for items.Next() {
			var i app.AccountOrderItem
			if err = items.Scan(&i.Name, &i.Spec, &i.Quantity, &i.Unit, &i.UnitPrice, &i.LineTotal); err != nil {
				items.Close()
				return d, err
			}
			d.Rows[0].Items = append(d.Rows[0].Items, i)
		}
		err = items.Err()
		items.Close()
		if err != nil {
			return d, err
		}
	}
	if q.OrderID > 0 {
		return d, tx.Commit(ctx)
	}
	// An order link is only valid when its source explicitly identifies an order of this customer.
	fees, err := tx.Query(ctx, fmt.Sprintf(`SELECT f.id,coalesce(o.id,0),coalesce(o.order_no,''),f.fee_type,round(f.amount*100)::bigint,coalesce(nullif(f.currency,''),'CNY'),to_char(f.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),coalesce(s.id,0),coalesce(s.settlement_no,''),coalesce(s.status,''),coalesce(to_char(s.paid_at,'YYYY-MM-DD HH24:MI'),'') FROM %[1]s.customer_fee_items f LEFT JOIN %[1]s.orders o ON f.source_type='order' AND o.id=f.source_id AND o.customer_id=f.customer_id LEFT JOIN %[1]s.customer_settlement_batches s ON s.id=f.settlement_batch_id AND s.customer_id=f.customer_id WHERE f.customer_id=$1 ORDER BY f.occurred_at DESC,f.id DESC`, r.schema), q.CustomerID)
	if err != nil {
		return d, err
	}
	allFees := []app.AccountFee{}
	for fees.Next() {
		var f app.AccountFee
		var paid string
		if err = fees.Scan(&f.ID, &f.OrderID, &f.OrderNo, &f.FeeType, &f.AmountCents, &f.Currency, &f.OccurredAt, &f.SettlementID, &f.SettlementNo, &f.SettlementStatus, &paid); err != nil {
			fees.Close()
			return d, err
		}
		f.PaymentStatus = "unpaid"
		if f.SettlementStatus == "paid" || (paid != "" && f.SettlementStatus != "reversed") {
			f.PaymentStatus = "paid"
		} else if f.SettlementStatus == "reversed" {
			f.PaymentStatus = "reversed"
		} else if f.SettlementStatus == "settled" {
			f.PaymentStatus = "unknown"
		}
		allFees = append(allFees, f)
		date := f.OccurredAt
		if len(date) > 10 {
			date = date[:10]
		}
		if (q.DateFrom == "" || date >= q.DateFrom) && (q.DateTo == "" || date <= q.DateTo) && (q.OrderID == 0 || f.OrderID == q.OrderID) {
			d.Fees = append(d.Fees, f)
		}
	}
	err = fees.Err()
	fees.Close()
	if err != nil {
		return d, err
	}
	bills, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,settlement_no,coalesce(to_char(period_from,'YYYY-MM-DD'),''),coalesce(to_char(period_to,'YYYY-MM-DD'),''),status,round(total_amount*100)::bigint,coalesce(to_char(confirmed_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),''),coalesce(to_char(paid_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),'') FROM %s.customer_settlement_batches WHERE customer_id=$1 AND status IN ('confirmed','paid','reversed','settled') ORDER BY created_at DESC,id DESC`, r.schema), q.CustomerID)
	if err != nil {
		return d, err
	}
	for bills.Next() {
		var b app.AccountSettlement
		if err = bills.Scan(&b.ID, &b.SettlementNo, &b.PeriodFrom, &b.PeriodTo, &b.Status, &b.TotalCents, &b.ConfirmedAt, &b.PaidAt); err != nil {
			bills.Close()
			return d, err
		}
		b.Fees = []app.AccountFee{}
		for _, f := range allFees {
			if f.SettlementID == b.ID {
				b.Fees = append(b.Fees, f)
			}
		}
		d.Settlements = append(d.Settlements, b)
	}
	err = bills.Err()
	bills.Close()
	if err != nil {
		return d, err
	}
	return d, tx.Commit(ctx)
}
