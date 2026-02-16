package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UnprodNeedRow struct {
	ProductID int64
	Product   string
	SpecG     int64
	NeedUnits int64
	NeedG     int64
	InvUnits  int64
	InvLooseG int64
	InvG      int64
	GapG      int64
}

func fetchUnproducedNeeds(ctx context.Context, pool *pgxpool.Pool, schema, from, to string, customerID int64) ([]UnprodNeedRow, error) {
	where := "WHERE o.is_void=false AND COALESCE(o.process_status_id,0) IN (1,2)"
	args := []any{}
	argn := 1
	if customerID > 0 {
		where += fmt.Sprintf(" AND o.customer_id = $%d", argn)
		args = append(args, customerID)
		argn++
	}
	if s := strings.TrimSpace(from); s != "" {
		where += fmt.Sprintf(" AND o.order_date >= $%d", argn)
		args = append(args, s)
		argn++
	}
	if s := strings.TrimSpace(to); s != "" {
		where += fmt.Sprintf(" AND o.order_date <= $%d", argn)
		args = append(args, s)
		argn++
	}

	// spec is stored like "454g"; extract digits safely.
	q := fmt.Sprintf(`
		WITH need AS (
			SELECT
				oi.product_id,
				COALESCE(p.name,'') AS product,
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g,
				SUM(COALESCE(oi.qty,0))::bigint AS need_units
			FROM %s.order_items oi
			JOIN %s.orders o ON o.id = oi.order_id
			LEFT JOIN %s.products p ON p.id = oi.product_id
			%s
			GROUP BY oi.product_id, p.name, COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')
		)
		SELECT
			n.product_id,
			n.product,
			n.spec_g,
			n.need_units,
			(n.need_units * n.spec_g) AS need_g,
			COALESCE(fi.onhand_units,0) AS inv_units,
			COALESCE(fi.onhand_loose_g,0) AS inv_loose_g,
			(COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0)) AS inv_g,
			GREATEST(0, (n.need_units * n.spec_g) - (COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0))) AS gap_g
		FROM need n
		LEFT JOIN %s.finished_inventory fi
			ON fi.product_id = n.product_id AND fi.spec_g = n.spec_g
		WHERE n.spec_g > 0
		ORDER BY gap_g DESC, n.product, n.spec_g
	`, schema, schema, schema, where, schema)

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UnprodNeedRow, 0)
	for rows.Next() {
		var r UnprodNeedRow
		if err := rows.Scan(&r.ProductID, &r.Product, &r.SpecG, &r.NeedUnits, &r.NeedG, &r.InvUnits, &r.InvLooseG, &r.InvG, &r.GapG); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
