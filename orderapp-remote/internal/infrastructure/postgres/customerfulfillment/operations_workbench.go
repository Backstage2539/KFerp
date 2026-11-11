package customerfulfillment

import (
	"context"
	"fmt"
	app "orderapp/internal/application/customerfulfillment"
	"orderapp/internal/infrastructure/postgres/orderconfirmation"
	salespg "orderapp/internal/infrastructure/postgres/sales"
)

func (r *Repository) fulfillmentOperations(ctx context.Context, id int64) (app.FulfillmentOperations, error) {
	var out app.FulfillmentOperations
	query := fmt.Sprintf(`SELECT count(*) FILTER(WHERE %s),count(*) FILTER(WHERE %s),count(*) FILTER(WHERE %s),count(*) FILTER(WHERE %s),count(*) FILTER(WHERE %s) FROM %s.orders o WHERE o.customer_id=$1 AND NOT COALESCE(o.is_void,false) AND %s`, salespg.FulfillmentQueuePredicate(r.schema, "pending_shipment"), salespg.FulfillmentQueuePredicate(r.schema, "stock_waiting"), salespg.FulfillmentQueuePredicate(r.schema, "missing_recipient"), salespg.FulfillmentQueuePredicate(r.schema, "pending_payment"), salespg.FulfillmentQueuePredicate(r.schema, "pending_confirmation"), r.schema, salespg.FulfillmentOrderPredicate())
	err := r.pool.QueryRow(ctx, orderconfirmation.CurrentRead(query, r.schema), id).Scan(&out.PendingShipment, &out.StockWaiting, &out.MissingRecipient, &out.PendingPayment, &out.PendingConfirmation)
	if err != nil {
		return out, err
	}
	// Amounts use accepted ledger rows, matching the orders list. Proposed edits
	// are only used for operational counts until the customer accepts them.
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT round(coalesce(sum(%s),0)*100)::bigint FROM %s.orders o WHERE customer_id=$1 AND NOT COALESCE(o.is_void,false) AND %s`, salespg.FulfillmentPendingAmount(r.schema), r.schema, salespg.FulfillmentOrderPredicate()), id).Scan(&out.PendingOrderCents)
	if err != nil {
		return out, err
	}
	// One statement gives all fee stages the same snapshot. A fee contributes
	// either to the unassigned balance or its settlement, never both.
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`WITH billable_fees AS (
  SELECT round(f.amount*100)::bigint-COALESCE((to_jsonb(f)->>'resolution_credit_cents')::bigint,0) AS due_cents
  FROM %[1]s.customer_fee_items f
  WHERE f.customer_id=$1 AND f.settlement_batch_id=0 AND f.status='unsettled'
    AND f.fee_type<>'order_goods'
    AND NOT (f.fee_type='shipping' AND f.source_type<>'freight_actual')
)
SELECT
  (SELECT count(*) FROM billable_fees WHERE due_cents>0),
  (SELECT coalesce(sum(due_cents),0)::bigint FROM billable_fees WHERE due_cents>0),
  round(coalesce(sum(total_amount) FILTER(WHERE status='draft'),0)*100)::bigint,
  round(coalesce(sum(total_amount) FILTER(WHERE status='confirmed'),0)*100)::bigint,
  round(coalesce(sum(total_amount) FILTER(WHERE status='paid'),0)*100)::bigint
  FROM %[1]s.customer_settlement_batches WHERE customer_id=$1`, r.schema), id).Scan(&out.UnsettledFeeCount, &out.UnsettledFeeCents, &out.DraftSettlementCents, &out.ConfirmedSettlementCents, &out.PaidSettlementCents)
	return out, err
}
