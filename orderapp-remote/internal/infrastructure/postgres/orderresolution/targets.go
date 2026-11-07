package orderresolution

import (
	"context"
	"fmt"
)

type Target struct {
	ID             int64  `json:"id"`
	Label          string `json:"label"`
	AvailableCents int64  `json:"available_cents"`
}

func (r Repository) Targets(ctx context.Context, orderID int64) ([]Target, error) {
	due := DueAmountSQL("o", "p.name")
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT o.id,o.order_no,round((%[2]s)*100)::bigint FROM %[1]s.orders o JOIN %[1]s.orders src ON src.customer_id=o.customer_id LEFT JOIN %[1]s.pay_statuses p ON p.id=o.pay_status_id WHERE src.id=$1 AND o.id<>src.id AND NOT o.is_void AND o.resolution_status<>'cancelled' AND (%[2]s)>0 AND NOT EXISTS(SELECT 1 FROM %[1]s.order_resolution_requests r WHERE r.order_id=o.id AND r.status='pending') ORDER BY o.order_date,o.id LIMIT 200`, r.schema, due), orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		var target Target
		if err = rows.Scan(&target.ID, &target.Label, &target.AvailableCents); err != nil {
			return nil, err
		}
		out = append(out, target)
	}
	return out, rows.Err()
}
