package customerfulfillment

import (
	"context"
	"fmt"

	app "orderapp/internal/application/customerfulfillment"
)

// customerInventoryOrderSources returns only explicit, active retained-order
// entitlements in authorized warehouses. Received quantities are cumulative;
// they must not be presented as a current stock balance.
func (r *Repository) customerInventoryOrderSources(ctx context.Context, customerID int64, warehouses []string) (map[string][]app.CustomerAssetOrderSource, error) {
	result := map[string][]app.CustomerAssetOrderSource{}
	if !relationExists(ctx, r.pool, r.schema+".customer_stock_entitlements") {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
        SELECT e.product_id,e.bom_spec_id,e.spec_g,o.id,o.order_no,
               COALESCE(NULLIF(ps.name,''),'未设置'),
               SUM(GREATEST(e.promised_qty-e.received_qty,0))::bigint,
               SUM(e.received_qty)::bigint
        FROM %[1]s.customer_stock_entitlements e
        JOIN %[1]s.orders o ON o.id=e.source_order_id AND o.customer_id=e.customer_id
        LEFT JOIN %[1]s.pay_statuses ps ON ps.id=o.pay_status_id
        WHERE e.customer_id=$1 AND e.warehouse_code=ANY($2::text[])
          AND e.status='active' AND NOT COALESCE(o.is_void,false)
          AND COALESCE(to_jsonb(o)->>'confirmation_status','accepted')='accepted'
        GROUP BY e.product_id,e.bom_spec_id,e.spec_g,o.id,o.order_no,ps.name
        ORDER BY o.order_no,o.id`, r.schema), customerID, warehouses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// Specs with a BOM id use the same identity as the stock rows, so combine
	// quantities for repeated order/spec entries before rendering them.
	positions := map[string]map[int64]int{}
	for rows.Next() {
		var productID, specID, specG int64
		var source app.CustomerAssetOrderSource
		if err := rows.Scan(&productID, &specID, &specG, &source.OrderID, &source.OrderNo, &source.PayStatus, &source.PendingInboundQty, &source.ReceivedQty); err != nil {
			return nil, err
		}
		key := miniStockKey(productID, specID, specG)
		if positions[key] == nil {
			positions[key] = map[int64]int{}
		}
		if index, ok := positions[key][source.OrderID]; ok {
			result[key][index].PendingInboundQty += source.PendingInboundQty
			result[key][index].ReceivedQty += source.ReceivedQty
		} else {
			positions[key][source.OrderID] = len(result[key])
			result[key] = append(result[key], source)
		}
	}
	return result, rows.Err()
}
