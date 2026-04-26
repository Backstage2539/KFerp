package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"

	"github.com/jackc/pgx/v5/pgxpool"
)

func (r Repository) ListOrders(ctx context.Context, query salesapp.OrderListQuery) (salesapp.OrderListResult, error) {
	rows, hasNext, err := fetchOrders(ctx, r.pool, r.schema, query)
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	summary, err := fetchOrdersSummary(ctx, r.pool, r.schema, query)
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	orderTypes, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".order_types ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	payStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".pay_statuses ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	shipStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".ship_statuses ORDER BY id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	processStatuses, err := fetchOrderOptions(ctx, r.pool, "SELECT id, name FROM "+r.schema+".order_process_statuses WHERE active=true ORDER BY sort,id")
	if err != nil {
		return salesapp.OrderListResult{}, err
	}
	return salesapp.OrderListResult{
		Rows:            rows,
		Summary:         summary,
		OrderTypes:      orderTypes,
		PayStatuses:     payStatuses,
		ShipStatuses:    shipStatuses,
		ProcessStatuses: processStatuses,
		HasNext:         hasNext,
	}, nil
}

func fetchOrders(ctx context.Context, pool *pgxpool.Pool, schema string, query salesapp.OrderListQuery) ([]salesapp.OrderRow, bool, error) {
	where, args, nextArg := orderListWhere(query)

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	args = append(args, query.Limit+1, query.Offset)
	limitArg := nextArg
	offsetArg := nextArg + 1

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

	out := make([]salesapp.OrderRow, 0)
	for dbRows.Next() {
		var r salesapp.OrderRow
		if err := dbRows.Scan(&r.ID, &r.OrderNo, &r.OrderDate, &r.CustomerID, &r.Customer, &r.GrandTotal, &r.OrderType, &r.PayStatus, &r.ShipStatus, &r.ProcessStatus, &r.CreatedByEmployee, &r.OrderTypeID, &r.PayStatusID, &r.ShipStatusID, &r.ProcessStatusID, &r.Notes, &r.IsVoid); err != nil {
			return nil, false, err
		}
		out = append(out, r)
	}
	if err := dbRows.Err(); err != nil {
		return nil, false, err
	}

	hasNext := false
	if len(out) > query.Limit {
		hasNext = true
		out = out[:query.Limit]
	}
	return out, hasNext, nil
}

func fetchOrdersSummary(ctx context.Context, pool *pgxpool.Pool, schema string, query salesapp.OrderListQuery) (salesapp.OrdersSummary, error) {
	where, args, _ := orderListWhere(query)
	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	sql := fmt.Sprintf(`
		SELECT count(*)::int AS orders,
		       count(distinct o.customer_id)::int AS customers
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id = o.customer_id
		%s
	`, schema, schema, wsql)

	var s salesapp.OrdersSummary
	if err := pool.QueryRow(ctx, sql, args...).Scan(&s.Orders, &s.Customers); err != nil {
		return salesapp.OrdersSummary{}, err
	}
	return s, nil
}

func orderListWhere(query salesapp.OrderListQuery) ([]string, []any, int) {
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if q := strings.TrimSpace(query.Q); q != "" {
		where = append(where, fmt.Sprintf("(o.order_no ILIKE $%d OR c.name ILIKE $%d)", argn, argn))
		args = append(args, "%"+q+"%")
		argn++
	}
	if query.CustomerID > 0 {
		where = append(where, fmt.Sprintf("o.customer_id = $%d", argn))
		args = append(args, query.CustomerID)
		argn++
	}
	if query.PayStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.pay_status_id,0) = $%d", argn))
		args = append(args, query.PayStatusID)
		argn++
	}
	if query.ShipStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.ship_status_id,0) = $%d", argn))
		args = append(args, query.ShipStatusID)
		argn++
	}
	if query.ProcessStatusID > 0 {
		where = append(where, fmt.Sprintf("COALESCE(o.process_status_id,0) = $%d", argn))
		args = append(args, query.ProcessStatusID)
		argn++
	}
	if query.UnproducedOnly {
		where = append(where, "COALESCE(o.process_status_id,0) IN (0,1,2)")
	}
	if query.CompletedOnly {
		where = append(where, "COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4)")
	}
	if from := strings.TrimSpace(query.From); from != "" {
		where = append(where, fmt.Sprintf("o.order_date >= $%d", argn))
		args = append(args, from)
		argn++
	}
	if to := strings.TrimSpace(query.To); to != "" {
		where = append(where, fmt.Sprintf("o.order_date <= $%d", argn))
		args = append(args, to)
		argn++
	}

	switch strings.TrimSpace(query.Void) {
	case "void":
		where = append(where, "o.is_void = true")
	case "all":
	default:
		where = append(where, "o.is_void = false")
	}
	return where, args, argn
}

func fetchOrderOptions(ctx context.Context, pool *pgxpool.Pool, sqlstr string) ([]salesapp.Option, error) {
	rows, err := pool.Query(ctx, sqlstr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.Option, 0)
	for rows.Next() {
		var o salesapp.Option
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
