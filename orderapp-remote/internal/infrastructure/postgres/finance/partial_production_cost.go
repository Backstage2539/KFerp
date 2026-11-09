package finance

import (
	"context"
	"fmt"
)

// partialUnknownProductionCosts counts affected work orders whose posted
// completion events occurred in this month but have no already-counted unknown
// batch cost for the same month. It never adds event amounts to the batch sum.
func (r Repository) partialUnknownProductionCosts(ctx context.Context, monthStart string) (int, error) {
	var hasEvents, hasOrders bool
	if err := r.query().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL`, r.schema+".production_operation_events", r.schema+".work_orders").Scan(&hasEvents, &hasOrders); err != nil {
		return 0, err
	}
	if !hasEvents {
		return 0, nil
	}
	exclude := ""
	if hasOrders {
		exclude = fmt.Sprintf(`AND NOT EXISTS (SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_batch_costs b ON NULLIF(NULLIF(to_jsonb(b)->>'running_item_id',''),'0')::bigint=NULLIF(NULLIF(to_jsonb(w)->>'running_item_id',''),'0')::bigint WHERE w.id=e.work_order_id AND b.created_at >= $1::date AND b.created_at < $1::date+interval '1 month' AND COALESCE((to_jsonb(b)->>'cost_unavailable')::boolean,false))`, r.schema)
	}
	var n int
	err := r.query().QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT e.work_order_id)::int FROM %s.production_operation_events e WHERE e.kind='complete' AND e.status='posted' AND e.created_at >= $1::date AND e.created_at < $1::date+interval '1 month' AND COALESCE((e.payload->>'cost_unavailable')::boolean,false) %s`, r.schema, exclude), monthStart).Scan(&n)
	return n, err
}

// Closed snapshots remain immutable. Completion events posted after the close
// are surfaced separately even if no batch-cost row exists yet.
func (r Repository) latePartialProductionCosts(ctx context.Context, monthStart string) (int, error) {
	var hasEvents, hasOrders bool
	if err := r.query().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL,to_regclass($2) IS NOT NULL`, r.schema+".production_operation_events", r.schema+".work_orders").Scan(&hasEvents, &hasOrders); err != nil {
		return 0, err
	}
	if !hasEvents {
		return 0, nil
	}
	exclude := ""
	if hasOrders {
		exclude = fmt.Sprintf(`AND NOT EXISTS (SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_batch_costs b ON NULLIF(NULLIF(to_jsonb(b)->>'running_item_id',''),'0')::bigint=NULLIF(NULLIF(to_jsonb(w)->>'running_item_id',''),'0')::bigint WHERE w.id=e.work_order_id AND b.created_at >= $1::date AND b.created_at < $1::date+interval '1 month' AND NULLIF(to_jsonb(b)->>'cost_updated_at','')::timestamptz>f.closed_at)`, r.schema)
	}
	var n int
	err := r.query().QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT e.work_order_id)::int FROM %[1]s.production_operation_events e JOIN %[1]s.finance_monthly_reports f ON f.month=to_char(e.created_at,'YYYY-MM') WHERE e.kind='complete' AND e.status='posted' AND e.created_at >= $1::date AND e.created_at < $1::date+interval '1 month' AND f.status IN ('closed','adjusted') AND f.closed_at IS NOT NULL AND e.created_at>f.closed_at %s`, r.schema, exclude), monthStart).Scan(&n)
	return n, err
}
