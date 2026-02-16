package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrdersSummary struct {
	Orders    int
	Customers int
}

func fetchOrdersSummary(ctx context.Context, pool *pgxpool.Pool, schema, q, from, to, voidFilter string, customerID int64, payStatusID, shipStatusID, procStatusID int64, unproducedOnly, completedOnly bool) (OrdersSummary, error) {
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if q = strings.TrimSpace(q); q != "" {
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
		where = append(where, "o.is_void = false")
	}

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

	var s OrdersSummary
	if err := pool.QueryRow(ctx, sql, args...).Scan(&s.Orders, &s.Customers); err != nil {
		return OrdersSummary{}, err
	}
	return s, nil
}
