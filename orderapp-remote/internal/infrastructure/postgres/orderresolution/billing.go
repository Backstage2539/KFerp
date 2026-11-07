package orderresolution

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	domain "orderapp/internal/domain/orderresolution"
)

func (r Repository) adjustIssuedBilling(ctx context.Context, tx pgx.Tx, orderID, requestID int64, cmd domain.Command, p domain.Plan, actor string) error {
	var goods int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT round((%s-COALESCE(shipping_amount,0))*100)::bigint-resolution_credit_cents+resolution_shipping_credit_cents FROM %s.orders o WHERE id=$1`, OriginalAmountSQL("o"), r.schema), orderID).Scan(&goods); err != nil {
		return err
	}
	goods -= p.CreditCents
	for _, f := range p.Fees {
		if f.SourceID == -1 {
			goods += f.CreditCents
		}
	}
	goods = max(int64(0), goods)
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT l.id,l.receivable_cents-COALESCE((SELECT sum(a.credit_cents)FROM %[1]s.order_resolution_billing_adjustments a WHERE a.line_id=l.id),0) FROM %[1]s.customer_settlement_lines l JOIN %[1]s.customer_settlement_batches b ON b.id=l.settlement_batch_id AND b.customer_id=l.customer_id WHERE l.source_type='order' AND l.source_id=$1 AND l.charge_type='order_goods' AND l.reversed_at IS NULL AND b.status IN('confirmed','paid','settled') AND COALESCE(b.processing_billing_run_id,0)=0 ORDER BY l.id FOR UPDATE OF l,b`, r.schema), orderID)
	if err != nil {
		return err
	}
	type line struct{ id, amount int64 }
	var lines []line
	for rows.Next() {
		var l line
		if err = rows.Scan(&l.id, &l.amount); err != nil {
			rows.Close()
			return err
		}
		lines = append(lines, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(lines) > 1 {
		return fmt.Errorf("订单存在多笔有效商品账单，请先由财务核对归属")
	}
	for _, l := range lines {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_billing_adjustments(request_id,line_id,credit_cents,posting_date,actor)VALUES($1,$2,$3,$4::date,$5)`, r.schema), requestID, l.id, l.amount-goods, cmd.PostingDate, actor); err != nil {
			return err
		}
	}
	for _, fee := range p.Fees {
		if fee.SourceID <= 0 || fee.CreditCents == 0 {
			continue
		}
		var amount int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT round(amount*100)::bigint-resolution_credit_cents-$2 FROM %s.customer_fee_items WHERE id=$1 FOR UPDATE`, r.schema), fee.SourceID, fee.CreditCents).Scan(&amount); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT l.id,l.receivable_cents-COALESCE((SELECT sum(a.credit_cents)FROM %[1]s.order_resolution_billing_adjustments a WHERE a.line_id=l.id),0)FROM %[1]s.customer_settlement_lines l JOIN %[1]s.customer_settlement_batches b ON b.id=l.settlement_batch_id WHERE l.source_type='customer_fee_item'AND l.source_id=$1 AND l.reversed_at IS NULL AND b.status IN('confirmed','paid','settled')AND COALESCE(b.processing_billing_run_id,0)=0 FOR UPDATE OF l,b`, r.schema), fee.SourceID)
		if err != nil {
			return err
		}
		lines = nil
		for rows.Next() {
			var l line
			if err = rows.Scan(&l.id, &l.amount); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(lines) > 1 {
			return fmt.Errorf("费用存在多笔有效账单，请先由财务核对归属")
		}
		for _, l := range lines {
			if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_resolution_billing_adjustments(request_id,line_id,credit_cents,posting_date,actor)VALUES($1,$2,$3,$4::date,$5)`, r.schema), requestID, l.id, l.amount-amount, cmd.PostingDate, actor); err != nil {
				return err
			}
		}
	}
	return nil
}
