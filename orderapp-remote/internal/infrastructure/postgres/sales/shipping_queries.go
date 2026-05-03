package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
)

func (r Repository) ListSFSmallShippingRows(ctx context.Context, query salesapp.ShippingExportQuery) ([]salesapp.ShippingExportRow, error) {
	where := make([]string, 0)
	args := make([]any, 0)
	argn := 1

	if query.Q != "" {
		where = append(where, fmt.Sprintf("(o.order_no ILIKE $%d OR c.name ILIKE $%d)", argn, argn))
		args = append(args, "%"+query.Q+"%")
		argn++
	}
	if query.CustomerID > 0 {
		where = append(where, fmt.Sprintf("o.customer_id = $%d", argn))
		args = append(args, query.CustomerID)
		argn++
	}
	if query.From != "" {
		where = append(where, fmt.Sprintf("o.order_date >= $%d", argn))
		args = append(args, query.From)
		argn++
	}
	if query.To != "" {
		where = append(where, fmt.Sprintf("o.order_date <= $%d", argn))
		args = append(args, query.To)
		argn++
	}

	switch strings.TrimSpace(query.Void) {
	case "void":
		where = append(where, "o.is_void = true")
	case "all":
	default:
		where = append(where, "o.is_void = false")
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
	if query.CompletedOnly {
		where = append(where, "COALESCE(o.pay_status_id,0)=2 AND COALESCE(o.ship_status_id,0) IN (3,4)")
	}
	if query.OneClick {
		where = append(where, "EXISTS (SELECT 1 FROM "+r.schema+".order_process_statuses ops WHERE ops.id=o.process_status_id AND ops.name IN ('生产完成','已生产完成','无需生产','库存待发货'))")
	} else {
		where = append(where, "COALESCE(o.ship_method,'') = 'sf_small'")
	}

	wsql := ""
	if len(where) > 0 {
		wsql = "WHERE " + strings.Join(where, " AND ")
	}

	qsql := fmt.Sprintf(`
		SELECT
			o.id,
			COALESCE(o.order_no,'') AS order_no,
			COALESCE(o.customer_id,0) AS customer_id,
			COALESCE(NULLIF(c.contact,''), c.name, '') AS recv_name,
			COALESCE(c.phone,'') AS recv_phone,
			COALESCE(c.address,'') AS recv_addr,
			'' AS recv_company,
			COALESCE(o.ship_tracking_no,'') AS tracking_no,
			COALESCE(SUM(
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.qty::text,''), '[^0-9.\-]', '', 'g'), ''), '0')::numeric
				*
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec::text,''), '[^0-9.\-]', '', 'g'), ''), '0')::numeric
			),0) AS total_g
		FROM %s.orders o
		LEFT JOIN %s.customers c ON c.id=o.customer_id
		LEFT JOIN %s.order_items oi ON oi.order_id=o.id
		%s
		GROUP BY o.id, o.order_no, o.customer_id, recv_name, recv_phone, recv_addr, tracking_no
		ORDER BY o.id DESC
	`, r.schema, r.schema, r.schema, wsql)

	rows, err := r.pool.Query(ctx, qsql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]salesapp.ShippingExportRow, 0)
	for rows.Next() {
		var row salesapp.ShippingExportRow
		var totalG float64
		if err := rows.Scan(&row.OrderID, &row.OrderNo, &row.CustomerID, &row.RecvName, &row.RecvPhone, &row.RecvAddr, &row.RecvCompany, &row.TrackingNo, &totalG); err != nil {
			return nil, err
		}
		row.WeightKg = totalG / 1000.0
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
