package customerportal

import (
	"context"
	"fmt"

	app "orderapp/internal/application/customerportal"
)

// loadCustomerOrderLogisticsEvents only receives IDs already scoped to the
// current customer by listCustomerOrders. It returns persisted events only.
func (r Repository) loadCustomerOrderLogisticsEvents(ctx context.Context, orderIDs []int64) (map[int64]*app.CustomerOrderLogisticsEvent, error) {
	result := make(map[int64]*app.CustomerOrderLogisticsEvent)
	if len(orderIDs) == 0 || !portalRelationExists(ctx, r.pool, r.schema+".order_shipping_tracking_events") {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT DISTINCT ON (order_id) order_id, COALESCE(tracking_no,''), to_char(event_time AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI:SS'), COALESCE(status,''), COALESCE(description,''), COALESCE(location,'') FROM %s.order_shipping_tracking_events WHERE order_id=ANY($1::bigint[]) ORDER BY order_id,event_time DESC,id DESC`, r.schema), orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var event app.CustomerOrderLogisticsEvent
		if err = rows.Scan(&id, &event.TrackingNo, &event.EventTime, &event.Status, &event.Description, &event.Location); err != nil {
			return nil, err
		}
		result[id] = &event
	}
	return result, rows.Err()
}
