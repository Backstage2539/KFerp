package main

import (
	"context"
	"fmt"
	"strings"

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
		// 未生产：未设置(0) / 已接单(1) / 已排产(2)
		where = append(where, "COALESCE(o.process_status_id,0) IN (0,1,2)")
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
			COALESCE(ot.name, '') AS order_type,
			COALESCE(ps.name, '') AS pay_status,
			COALESCE(ss.name, '') AS ship_status,
			COALESCE(ops.name, '') AS process_status,
			COALESCE((SELECT al.actor FROM %s.order_audit_logs al WHERE al.order_id=o.id ORDER BY al.id ASC LIMIT 1), '未知') AS created_by_employee,
			COALESCE(o.order_type_id,0) AS order_type_id,
			COALESCE(o.pay_status_id,0) AS pay_status_id,
			COALESCE(o.ship_status_id,0) AS ship_status_id,
			COALESCE(o.process_status_id,0) AS process_status_id,
			COALESCE(o.notes,'') AS notes,
			o.is_void
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		LEFT JOIN %s.order_types ot ON ot.id = o.order_type_id
		LEFT JOIN %s.pay_statuses ps ON ps.id = o.pay_status_id
		LEFT JOIN %s.ship_statuses ss ON ss.id = o.ship_status_id
		LEFT JOIN %s.order_process_statuses ops ON ops.id = o.process_status_id
		%s
		ORDER BY o.order_date DESC, o.id DESC
		LIMIT $%d OFFSET $%d
	`, schema, schema, schema, schema, schema, schema, schema, wsql, limitArg, offsetArg)

	dbRows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, err
	}
	defer dbRows.Close()

	out := make([]OrderRow, 0)
	for dbRows.Next() {
		var r OrderRow
		if err := dbRows.Scan(&r.ID, &r.OrderNo, &r.OrderDate, &r.CustomerID, &r.Customer, &r.GrandTotal, &r.OrderType, &r.PayStatus, &r.ShipStatus, &r.ProcessStatus, &r.CreatedByEmployee, &r.OrderTypeID, &r.PayStatusID, &r.ShipStatusID, &r.ProcessStatusID, &r.Notes, &r.IsVoid); err != nil {
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
