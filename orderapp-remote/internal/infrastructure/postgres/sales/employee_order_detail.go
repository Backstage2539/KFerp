package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
)

func (r Repository) EmployeeOrderRow(ctx context.Context, query salesapp.OrderListQuery) (*salesapp.OrderRow, error) {
	rows, _, err := fetchOrders(ctx, r.pool, r.schema, query)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

// OrderDetailForm supplies only the dictionaries used by a single order.
func (r Repository) OrderDetailForm(ctx context.Context, id int64) (salesapp.OrderFormData, error) {
	var data salesapp.OrderFormData
	ed, err := r.OrderStatusEditData(ctx, id)
	if err != nil || ed == nil {
		return data, err
	}
	data.EditData = ed
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT 'source', id, name FROM %[1]s.sources WHERE id=$1
        UNION ALL SELECT 'type', id, name FROM %[1]s.order_types WHERE id=$2
        UNION ALL SELECT 'pay', id, name FROM %[1]s.pay_statuses
        UNION ALL SELECT 'ship', id, name FROM %[1]s.ship_statuses ORDER BY 1,2`, r.schema), ed.SourceID, ed.OrderTypeID)
	if err != nil {
		return data, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var option salesapp.Option
		if err := rows.Scan(&kind, &option.ID, &option.Name); err != nil {
			return data, err
		}
		switch kind {
		case "source":
			data.Sources = append(data.Sources, option)
		case "type":
			data.OrderTypes = append(data.OrderTypes, option)
		case "pay":
			data.PayStatuses = append(data.PayStatuses, option)
		case "ship":
			data.ShipStatuses = append(data.ShipStatuses, option)
		}
	}
	if err := rows.Err(); err != nil {
		return data, err
	}
	if ed.LogisticsCompanyID > 0 {
		data.LogisticsCompanies, err = r.ListLogisticsCompanies(ctx, false)
	}
	return data, err
}
