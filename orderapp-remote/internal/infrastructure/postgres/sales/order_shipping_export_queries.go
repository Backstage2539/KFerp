package sales

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	salesapp "orderapp/internal/application/sales"
)

func (r Repository) LoadOrderShippingExportData(ctx context.Context, orderID int64) (salesapp.OrderShippingExportData, error) {
	var out salesapp.OrderShippingExportData
	q := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,''),
			COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), ''),
			COALESCE(c.name,''),
			COALESCE(NULLIF(c.contact,''), c.name, ''),
			COALESCE(c.phone,''),
			COALESCE(c.address,''),
			''
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		WHERE o.id=$1
	`, r.schema, r.schema)
	if err := r.pool.QueryRow(ctx, q, orderID).Scan(
		&out.OrderID,
		&out.OrderNo,
		&out.OrderDate,
		&out.CustomerName,
		&out.RecvName,
		&out.RecvPhone,
		&out.RecvAddr,
		&out.RecvCompany,
	); err != nil {
		return salesapp.OrderShippingExportData{}, err
	}

	itemsQ := fmt.Sprintf(`
		SELECT
			COALESCE(oi.line_no, 0),
			COALESCE(NULLIF(oi.item_name,''), p.name, ''),
			COALESCE(oi.spec,''),
			COALESCE(oi.qty::text,''),
			COALESCE(oi.unit,'件'),
			COALESCE(oi.unit_price::text,'0'),
			COALESCE(oi.line_total::text,'0')
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no, oi.id
	`, r.schema, r.schema)
	rows, err := r.pool.Query(ctx, itemsQ, orderID)
	if err != nil {
		return salesapp.OrderShippingExportData{}, err
	}
	defer rows.Close()

	out.Items = make([]salesapp.OrderShippingExportItem, 0)
	for rows.Next() {
		var item salesapp.OrderShippingExportItem
		if err := rows.Scan(&item.LineNo, &item.Name, &item.Spec, &item.Qty, &item.Unit, &item.UnitPrice, &item.LineTotal); err != nil {
			return salesapp.OrderShippingExportData{}, err
		}
		item.Qty = trimDecimalText(item.Qty)
		item.UnitPrice = trimMoneyText(item.UnitPrice)
		item.LineTotal = trimMoneyText(item.LineTotal)
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return salesapp.OrderShippingExportData{}, err
	}
	return out, nil
}

func trimDecimalText(raw string) string {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return trimFloatZero(v)
}

func trimMoneyText(raw string) string {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return fmt.Sprintf("%.2f", v)
}
