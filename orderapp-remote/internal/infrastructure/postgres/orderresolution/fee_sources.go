package orderresolution

import (
	"context"
	"fmt"
)

// The aliases f and o always refer to the fee and its exact source order.
func (r Repository) feeOrderPredicate(ctx context.Context, q querier) (string, error) {
	direct := `(f.source_type IN('order','direct_ship_order') AND f.source_id=o.id AND f.fee_type NOT IN('shipping','product','order_goods'))`
	var actual bool
	if err := q.QueryRow(ctx, `SELECT to_regclass($1)IS NOT NULL AND to_regclass($2)IS NOT NULL`, r.schema+".freight_actual_shipments", r.schema+".freight_actual_entries").Scan(&actual); err != nil {
		return "", err
	}
	if actual {
		direct += fmt.Sprintf(` OR (f.source_type='freight_actual' AND EXISTS(SELECT 1 FROM %[1]s.freight_actual_entries fe JOIN %[1]s.freight_actual_shipments fs ON fs.id=fe.shipment_id WHERE fe.id=f.source_id AND fs.order_id=o.id AND fs.customer_id=f.customer_id))`, r.schema)
	}
	return `f.customer_id=o.customer_id AND COALESCE(f.status,'') NOT IN('reversed','void','cancelled') AND (` + direct + `)`, nil
}
func feeLabel(kind string) string {
	if label := map[string]string{"shipping": "实际运费", "direct_ship_service": "代发服务费", "processing": "加工费", "roasting": "烘焙费", "packaging": "包装费", "storage": "仓储费", "labor": "人工费", "material": "材料费", "adjustment": "其他费用调整"}[kind]; label != "" {
		return label
	}
	return kind
}
