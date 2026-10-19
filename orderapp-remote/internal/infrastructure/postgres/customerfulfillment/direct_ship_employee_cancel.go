package customerfulfillment

import (
	"context"
	"fmt"
)

func (r *Repository) FindDirectShipRequestForOrder(ctx context.Context, customerID, orderID int64) (int64, error) {
	if customerID <= 0 || orderID <= 0 {
		return 0, fmt.Errorf("请选择有效客户和订单")
	}
	var requestID, count int64
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(min(request.id),0),count(DISTINCT request.id)
 FROM %[1]s.customer_direct_ship_request_orders link
 JOIN %[1]s.customer_direct_ship_requests request ON request.id=link.request_id AND request.customer_id=$1
 JOIN %[1]s.orders sale ON sale.id=link.order_id AND sale.customer_id=$1
 WHERE link.order_id=$2`, r.schema), customerID, orderID).Scan(&requestID, &count)
	if err != nil {
		return 0, err
	}
	if requestID <= 0 || count != 1 {
		return 0, fmt.Errorf("代发订单不存在、归属不匹配或没有唯一发货申请")
	}
	return requestID, nil
}
