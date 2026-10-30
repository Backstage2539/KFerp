package sales

import "fmt"

// orderReceivableSQL uses the same legacy fallback as freight and billing:
// a calculated zero, with a discount, shipping or rounding, remains zero.
func orderReceivableSQL(alias string) string {
	return fmt.Sprintf(`(CASE WHEN COALESCE(%[1]s.grand_total,0)<>0 OR COALESCE(NULLIF(to_jsonb(%[1]s)->>'discount_amount','')::numeric,0)<>0 OR COALESCE(NULLIF(to_jsonb(%[1]s)->>'shipping_amount','')::numeric,0)<>0 OR COALESCE(NULLIF(to_jsonb(%[1]s)->>'rounding_amount','')::numeric,0)<>0 THEN COALESCE(%[1]s.grand_total,0) ELSE COALESCE(%[1]s.total_amount,0) END)`, alias)
}
