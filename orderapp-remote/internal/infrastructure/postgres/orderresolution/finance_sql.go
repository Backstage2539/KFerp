package orderresolution

import "fmt"

// These expressions preserve cash receipts and separately apply credit,
// outgoing settlement and incoming allocation to the current order balance.
func OriginalAmountSQL(alias string) string {
	return fmt.Sprintf(`(CASE WHEN COALESCE(%[1]s.grand_total,0)<>0 OR COALESCE((to_jsonb(%[1]s)->>'discount_amount')::numeric,0)<>0 OR COALESCE((to_jsonb(%[1]s)->>'shipping_amount')::numeric,0)<>0 OR COALESCE((to_jsonb(%[1]s)->>'rounding_amount')::numeric,0)<>0 THEN COALESCE(%[1]s.grand_total,0) ELSE COALESCE((to_jsonb(%[1]s)->>'total_amount')::numeric,0) END)`, alias)
}
func ReceivableAmountSQL(alias string) string {
	return fmt.Sprintf(`GREATEST(0,%s-COALESCE((to_jsonb(%s)->>'resolution_credit_cents')::numeric,0)/100)`, OriginalAmountSQL(alias), alias)
}
func CashAmountSQL(alias, statusExpression string) string {
	return fmt.Sprintf(`(GREATEST(COALESCE((to_jsonb(%[1]s)->>'prepayment_amount')::numeric,0),COALESCE((to_jsonb(%[1]s)->>'payment_goods_amount')::numeric,0)+COALESCE((to_jsonb(%[1]s)->>'payment_shipping_amount')::numeric,0),CASE WHEN NOT COALESCE((to_jsonb(%[1]s)->>'resolution_payment_tracked')::boolean,false) AND COALESCE(%[2]s,'')~'(已付款|已收款|已支付)' THEN %[3]s ELSE 0 END)+COALESCE((to_jsonb(%[1]s)->>'resolution_legacy_cash_cents')::numeric,0)/100)`, alias, statusExpression, OriginalAmountSQL(alias))
}
func SettledAmountSQL(alias, statusExpression string) string {
	return fmt.Sprintf(`GREATEST(0,%s+COALESCE((to_jsonb(%s)->>'resolution_offset_cents')::numeric,0)/100-COALESCE((to_jsonb(%s)->>'resolution_settled_out_cents')::numeric,0)/100)`, CashAmountSQL(alias, statusExpression), alias, alias)
}
func DueAmountSQL(alias, statusExpression string) string {
	return fmt.Sprintf(`GREATEST(0,%s-%s)`, ReceivableAmountSQL(alias), SettledAmountSQL(alias, statusExpression))
}

// FeeCashCentsSQL preserves confirmed fee cash independently of merchandise.
// Legacy settled-only batches are not evidence of an actual receipt.
func FeeCashCentsSQL(alias, status string) string {
	return fmt.Sprintf(`(CASE WHEN COALESCE((to_jsonb(%[1]s)->>'resolution_cash_tracked')::boolean,false) THEN COALESCE((to_jsonb(%[1]s)->>'resolution_cash_received_cents')::bigint,0) WHEN COALESCE(%[2]s,'')='paid' THEN round(%[1]s.amount*100)::bigint-COALESCE((to_jsonb(%[1]s)->>'resolution_credit_cents')::bigint,0) ELSE 0 END)`, alias, status)
}
