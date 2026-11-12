package finance

import (
	"context"
	"fmt"
	app "orderapp/internal/application/finance"
)

// Merchandise and order-priced freight already contribute through the order.
// Independent service charges contribute once, on their original occurrence date.
func (r Repository) independentFeePredicate() string {
	return fmt.Sprintf(`COALESCE(f.status,'') NOT IN('reversed','void','cancelled') AND NOT EXISTS(SELECT 1 FROM %s.orders o WHERE o.id=f.source_id AND o.customer_id=f.customer_id AND f.source_type IN('order','direct_ship_order') AND f.fee_type IN('shipping','product','order_goods'))`, r.schema)
}
func (r Repository) feeRevenueDetails(ctx context.Context, filter app.ReportFilter) ([]app.SourceDetail, error) {
	out := []app.SourceDetail{}
	var fees, events bool
	if err := r.query().QueryRow(ctx, `SELECT to_regclass($1)IS NOT NULL,to_regclass($2)IS NOT NULL`, r.schema+".customer_fee_items", r.schema+".order_resolution_fee_events").Scan(&fees, &events); err != nil || !fees {
		return out, err
	}
	rows, err := r.query().Query(ctx, fmt.Sprintf(`SELECT 'revenue','customer_fee_item',f.id,to_char(f.occurred_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD'),'独立费用 #'||f.id::text,CASE f.fee_type WHEN 'direct_ship_service' THEN '代发服务费' WHEN 'shipping' THEN '实际运费' WHEN 'processing' THEN '加工费' WHEN 'storage' THEN '仓储费' ELSE f.fee_type END,COALESCE(c.name,''),'','','',f.amount::float8,'/app/vue-shell?view=customerAccount&customer_id='||f.customer_id::text FROM %[1]s.customer_fee_items f LEFT JOIN %[1]s.customers c ON c.id=f.customer_id WHERE %[2]s AND (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date >= $1::date AND (f.occurred_at AT TIME ZONE 'Asia/Shanghai')::date < $1::date+interval '1 month' AND ($2::bigint=0 OR f.customer_id=$2) ORDER BY f.occurred_at,f.id`, r.schema, r.independentFeePredicate()), filter.Month+"-01", filter.CustomerID)
	if err != nil {
		return out, err
	}
	if err = scanSourceDetails(rows, &out); err != nil {
		return out, err
	}
	if !events {
		return out, nil
	}
	rows, err = r.query().Query(ctx, fmt.Sprintf(`SELECT 'revenue','order_resolution_fee',r.id,r.command_json->>'posting_date','费用冲减 #'||f.id::text,'独立费用冲减',COALESCE(c.name,''),'','','',-e.credit_cents::float8/100,'/app/vue-shell?view=customerAccount&customer_id='||f.customer_id::text FROM %[1]s.order_resolution_fee_events e JOIN %[1]s.order_resolution_requests r ON r.id=e.request_id AND r.applied JOIN %[1]s.customer_fee_items f ON f.id=e.source_id LEFT JOIN %[1]s.customers c ON c.id=f.customer_id WHERE %[2]s AND NULLIF(BTRIM(r.command_json->>'posting_date'),'')::date >= $1::date AND NULLIF(BTRIM(r.command_json->>'posting_date'),'')::date < $1::date+interval '1 month' AND ($2::bigint=0 OR f.customer_id=$2) AND e.credit_cents<>0 ORDER BY r.id,f.id`, r.schema, r.independentFeePredicate()), filter.Month+"-01", filter.CustomerID)
	if err != nil {
		return out, err
	}
	err = scanSourceDetails(rows, &out)
	return out, err
}
