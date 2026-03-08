package main

import (
	"context"
	"fmt"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func fetchOrders(ctx context.Context, pool *pgxpool.Pool, schema, q, from, to, voidFilter string, customerID int64, payStatusID, shipStatusID, procStatusID int64, unproducedOnly, completedOnly bool, limit, offset int) (rows []OrderRow, hasNext bool, err error) {
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if q = strings.TrimSpace(q); q != "" {
		// match order_no or customer name
		where = append(where, fmt.Sprintf("(o.order_no ILIKE $%d OR c.name ILIKE $%d)", argn, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	if customerID > 0 {
		where = append(where, fmt.Sprintf("o.customer_id = $%d", argn))
		args = append(args, customerID)
		argn++
	}
	if payStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.pay_status_id,0) = $%d", argn))
		args = append(args, payStatusID)
		argn++
	}
	if shipStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.ship_status_id,0) = $%d", argn))
		args = append(args, shipStatusID)
		argn++
	}
	if procStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.process_status_id,0) = $%d", argn))
		args = append(args, procStatusID)
		argn++
	}
	if unproducedOnly {
		// 未生产：已接单(1) / 已排产(2)
		where = append(where, "COALESCE(o.process_status_id,0) IN (1,2)")
	}
	if completedOnly {
		where = append(where, "COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4)")
	}
	if from = strings.TrimSpace(from); from != "" {
		where = append(where, fmt.Sprintf("o.order_date >= $%d", argn))
		args = append(args, from)
		argn++
	}
	if to = strings.TrimSpace(to); to != "" {
		where = append(where, fmt.Sprintf("o.order_date <= $%d", argn))
		args = append(args, to)
		argn++
	}

	voidFilter = strings.TrimSpace(voidFilter)
	switch voidFilter {
	case "void":
		where = append(where, "o.is_void = true")
	case "all":
		// no filter
	default:
		// normal
		where = append(where, "o.is_void = false")
	}

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	// fetch one more row to determine hasNext
	args = append(args, limit+1, offset)
	limitArg := argn
	offsetArg := argn + 1

	sql := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), '') AS order_date,
			COALESCE(o.customer_id,0) AS customer_id,
			COALESCE(c.name, '') AS customer,
			COALESCE(to_char(o.grand_total, 'FM999999999.00'), '') AS grand_total,
			COALESCE(ps.name, '') AS pay_status,
			COALESCE(ss.name, '') AS ship_status,
			COALESCE(ops.name, '') AS process_status,
			COALESCE((SELECT al.actor FROM %s.order_audit_logs al WHERE al.order_id=o.id ORDER BY al.id ASC LIMIT 1), '未知') AS created_by_employee,
			COALESCE(o.pay_status_id,0) AS pay_status_id,
			COALESCE(o.ship_status_id,0) AS ship_status_id,
			COALESCE(o.process_status_id,0) AS process_status_id,
			COALESCE(o.notes,'') AS notes,
			o.is_void
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		LEFT JOIN %s.pay_statuses ps ON ps.id = o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id = o.ship_status_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id = o.process_status_id
		%s
		ORDER BY o.order_date DESC, o.id DESC
		LIMIT $%d OFFSET $%d
	`, schema, schema, schema, schema, schema, schema, wsql, limitArg, offsetArg)

	dbRows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer dbRows.Close()

	out := make([]OrderRow, 0)
	for dbRows.Next() {
		var r OrderRow
		if err := dbRows.Scan(&r.ID, &r.OrderNo, &r.OrderDate, &r.CustomerID, &r.Customer, &r.GrandTotal, &r.PayStatus, &r.ShipStatus, &r.ProcessStatus, &r.CreatedByEmployee, &r.PayStatusID, &r.ShipStatusID, &r.ProcessStatusID, &r.Notes, &r.IsVoid); err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, false, err
	}

	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func fetchOrderDetail(ctx context.Context, pool *pgxpool.Pool, schema string, id int64) (*OrderDetailData, error) {
	q := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(to_char(o.order_date, 'YYYY-MM-DD'), '') AS order_date,
			COALESCE(c.name, '') AS customer,
			COALESCE(src.name, '') AS source,
			COALESCE(ot.name, '') AS order_type,
			COALESCE(ps.name, '') AS pay_status,
			COALESCE(ss.name, '') AS ship_status,
			COALESCE(ops.name, '') AS process_status,
			COALESCE((SELECT al.actor FROM %s.order_audit_logs al WHERE al.order_id=o.id ORDER BY al.id ASC LIMIT 1), '未知') AS created_by_employee,
			o.is_void,
			CASE WHEN o.voided_at IS NULL THEN NULL ELSE to_char(o.voided_at, 'YYYY-MM-DD HH24:MI:SS') END AS voided_at,
			COALESCE(o.void_reason,'') AS void_reason,
			COALESCE(o.notes,'') AS notes,
			COALESCE(o.total_amount,0) AS total_amount,
			COALESCE(o.shipping_amount,0) AS shipping_amount,
			COALESCE(o.discount_amount,0) AS discount_amount,
			COALESCE(o.round_to_int,false) AS round_to_int,
			COALESCE(o.rounding_amount,0) AS rounding_amount,
			COALESCE(o.grand_total,0) AS grand_total,
			COALESCE(NULLIF(o.express_fee,''),'0')::numeric AS express_fee
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		LEFT JOIN %s.sources src ON src.id = o.source_id
		LEFT JOIN %s.order_types ot ON ot.id = o.order_type_id
		LEFT JOIN %s.pay_statuses ps ON ps.id = o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id = o.ship_status_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id = o.process_status_id
		WHERE o.id = $1
	`, schema, schema, schema, schema, schema, schema, schema, schema)

	var d OrderDetailData
	row := pool.QueryRow(ctx, q, id)
	var processStatus string
	if err := row.Scan(
		&d.ID,
		&d.OrderNo,
		&d.OrderDate,
		&d.Customer,
		&d.Source,
		&d.OrderType,
		&d.PayStatus,
		&d.ShipStatus,
		&processStatus,
		&d.CreatedByEmployee,
		&d.IsVoid,
		&d.VoidedAt,
		&d.VoidReason,
		&d.Notes,
		&d.TotalAmount,
		&d.ShippingAmt,
		&d.DiscountAmt,
		&d.RoundToInt,
		&d.RoundingAmt,
		&d.GrandTotal,
		&d.ExpressFee,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.ProcessStatus = processStatus

	itemsQ := fmt.Sprintf(`
		SELECT oi.line_no,
			COALESCE(p.name,'') AS product,
			COALESCE(oi.item_name,'') AS item_name,
			oi.qty, oi.unit, oi.spec,
			oi.unit_price, oi.line_total
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id = oi.product_id
		WHERE oi.order_id=$1
		ORDER BY oi.line_no
	`, schema, schema)
	rows, err := pool.Query(ctx, itemsQ, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	d.Items = make([]OrderItemRow, 0)
	for rows.Next() {
		var it OrderItemRow
		if err := rows.Scan(&it.LineNo, &it.Product, &it.ItemName, &it.Qty, &it.Unit, &it.Spec, &it.UnitPrice, &it.LineTotal); err != nil {
			return nil, err
		}
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}
