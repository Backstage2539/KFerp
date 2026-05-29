package production

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UnprodNeedRow struct {
	ProductID                int64  `json:"product_id"`
	Product                  string `json:"product"`
	OrderNos                 string `json:"order_nos"`
	SpecG                    int64  `json:"spec_g"`
	NeedUnits                int64  `json:"need_units"`
	NeedG                    int64  `json:"need_g"`
	InvUnits                 int64  `json:"inv_units"`
	InvLooseG                int64  `json:"inv_loose_g"`
	InvG                     int64  `json:"inv_g"`
	GapG                     int64  `json:"gap_g"`
	ProductionKind           string `json:"production_kind,omitempty"`
	ProductTypeCategoryID    int64  `json:"product_type_category_id,omitempty"`
	ProductSubtypeCategoryID int64  `json:"product_subtype_category_id,omitempty"`
	ProductTypeName          string `json:"product_type_name,omitempty"`
	ProductSubtypeName       string `json:"product_subtype_name,omitempty"`
	OperationTemplateID      int64  `json:"operation_template_id,omitempty"`
}

func fetchUnproducedNeeds(ctx context.Context, pool *pgxpool.Pool, schema, from, to string, customerID int64) ([]UnprodNeedRow, error) {
	where := fmt.Sprintf(`WHERE o.is_void=false AND (
		COALESCE(o.process_status_id,0) = 0
		OR EXISTS (
			SELECT 1 FROM %s.order_process_statuses ops
			WHERE ops.id=o.process_status_id
			  AND ops.name IN ('待处理','待生产')
		)
	)
	AND COALESCE(oi.product_id,0) > 0
	AND NOT EXISTS (
		SELECT 1 FROM %s.ship_statuses ss
		WHERE ss.id=o.ship_status_id
		  AND ss.name='已发货'
	)`, schema, schema)
	demandWhere := []string{"d.status='planned'", "COALESCE(d.product_id,0) > 0"}
	args := []any{}
	argn := 1
	if customerID > 0 {
		where += fmt.Sprintf(" AND o.customer_id = $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.customer_id = $%d", argn))
		args = append(args, customerID)
		argn++
	}
	if s := strings.TrimSpace(from); s != "" {
		where += fmt.Sprintf(" AND o.order_date >= $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.created_at::date >= $%d::date", argn))
		args = append(args, s)
		argn++
	}
	if s := strings.TrimSpace(to); s != "" {
		where += fmt.Sprintf(" AND o.order_date <= $%d", argn)
		demandWhere = append(demandWhere, fmt.Sprintf("d.created_at::date <= $%d::date", argn))
		args = append(args, s)
		argn++
	}

	// spec is stored like "454g"; extract digits safely.
	q := fmt.Sprintf(`
		WITH need AS (
			SELECT
				oi.product_id,
				COALESCE(p.name,'') AS product,
				COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted_bean') AS production_kind,
				COALESCE(type_pc.id,0) AS product_type_category_id,
				COALESCE(subtype_pc.id,0) AS product_subtype_category_id,
				COALESCE(type_pc.name,'') AS product_type_name,
				COALESCE(subtype_pc.name,'') AS product_subtype_name,
				COALESCE(
					NULLIF(cpro.operation_template_id,0),
					NULLIF(cpti.operation_template_id,0),
					NULLIF(p.operation_template_id_override,0),
					NULLIF(subtype_pc.operation_template_id,0),
					type_pc.operation_template_id,
					0
				) AS effective_operation_template_id,
				STRING_AGG(DISTINCT COALESCE(o.order_no,''), ',' ORDER BY COALESCE(o.order_no,'')) AS order_nos,
				COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g,
				SUM(COALESCE(oi.qty,0))::bigint AS need_units,
				SUM(CASE WHEN COALESCE(osd.decision,'') = 'produce' THEN COALESCE(oi.qty,0) ELSE 0 END)::bigint AS force_produce_units
			FROM %s.order_items oi
			JOIN %s.orders o ON o.id = oi.order_id
			LEFT JOIN %s.products p ON p.id = oi.product_id
			LEFT JOIN %s.product_categories subtype_pc ON subtype_pc.id=COALESCE(p.product_category_id,0)
			LEFT JOIN %s.product_categories type_pc ON type_pc.id=COALESCE(subtype_pc.parent_id,0)
			LEFT JOIN %s.customers rule_customer ON rule_customer.id=o.customer_id AND rule_customer.active=true
			LEFT JOIN %s.customer_product_rule_template_items cpti
			  ON cpti.active=true
			 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
			 AND cpti.product_subtype_category_id=COALESCE(subtype_pc.id,0)
			LEFT JOIN %s.customer_product_rule_overrides cpro
			  ON cpro.active=true
			 AND cpro.customer_id=o.customer_id
			 AND cpro.product_subtype_category_id=COALESCE(subtype_pc.id,0)
			LEFT JOIN %s.order_stock_decisions osd ON osd.order_id = o.id
			%s
			GROUP BY oi.product_id, p.name, COALESCE(NULLIF(oi.product_kind,''), NULLIF(p.product_kind,''), 'roasted_bean'), type_pc.id, subtype_pc.id, type_pc.name, subtype_pc.name, COALESCE(NULLIF(cpro.operation_template_id,0), NULLIF(cpti.operation_template_id,0), NULLIF(p.operation_template_id_override,0), NULLIF(subtype_pc.operation_template_id,0), type_pc.operation_template_id,0), COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')
			UNION ALL
			SELECT
				d.product_id,
				COALESCE(NULLIF(d.product_name,''), p.name, '') AS product,
				COALESCE(NULLIF(p.product_kind,''), 'roasted_bean') AS production_kind,
				COALESCE(type_pc.id,0) AS product_type_category_id,
				COALESCE(subtype_pc.id,0) AS product_subtype_category_id,
				COALESCE(type_pc.name,'') AS product_type_name,
				COALESCE(subtype_pc.name,'') AS product_subtype_name,
				COALESCE(
					NULLIF(cpro.operation_template_id,0),
					NULLIF(cpti.operation_template_id,0),
					NULLIF(p.operation_template_id_override,0),
					NULLIF(subtype_pc.operation_template_id,0),
					type_pc.operation_template_id,
					0
				) AS effective_operation_template_id,
				STRING_AGG(DISTINCT COALESCE(d.request_no,''), ',' ORDER BY COALESCE(d.request_no,'')) AS order_nos,
				d.spec_g,
				SUM(COALESCE(d.target_qty,0))::bigint AS need_units,
				SUM(COALESCE(d.target_qty,0))::bigint AS force_produce_units
			FROM %s.customer_processing_production_demands d
			LEFT JOIN %s.products p ON p.id=d.product_id
			LEFT JOIN %s.product_categories subtype_pc ON subtype_pc.id=COALESCE(p.product_category_id,0)
			LEFT JOIN %s.product_categories type_pc ON type_pc.id=COALESCE(subtype_pc.parent_id,0)
			LEFT JOIN %s.customers rule_customer ON rule_customer.id=d.customer_id AND rule_customer.active=true
			LEFT JOIN %s.customer_product_rule_template_items cpti
			  ON cpti.active=true
			 AND cpti.template_id=COALESCE(rule_customer.customer_product_rule_template_id,0)
			 AND cpti.product_subtype_category_id=COALESCE(subtype_pc.id,0)
			LEFT JOIN %s.customer_product_rule_overrides cpro
			  ON cpro.active=true
			 AND cpro.customer_id=d.customer_id
			 AND cpro.product_subtype_category_id=COALESCE(subtype_pc.id,0)
			WHERE %s
			GROUP BY d.product_id, d.product_name, p.name, COALESCE(NULLIF(p.product_kind,''), 'roasted_bean'), type_pc.id, subtype_pc.id, type_pc.name, subtype_pc.name, COALESCE(NULLIF(cpro.operation_template_id,0), NULLIF(cpti.operation_template_id,0), NULLIF(p.operation_template_id_override,0), NULLIF(subtype_pc.operation_template_id,0), type_pc.operation_template_id,0), d.spec_g
		)
		, reserved AS (
			SELECT
				a.product_id,
				a.spec_g,
				SUM(a.allocated_g)::bigint AS reserved_g
			FROM %s.order_stock_batch_allocations a
			WHERE NOT EXISTS (
				SELECT 1
				FROM %s.order_stock_deductions d
				WHERE d.order_id=a.order_id
				  AND d.product_id=a.product_id
				  AND d.spec_g=a.spec_g
				  AND d.batch_code=a.batch_code
			)
			GROUP BY a.product_id, a.spec_g
		)
		SELECT
			n.product_id,
			n.product,
			n.production_kind,
			n.product_type_category_id,
			n.product_subtype_category_id,
			n.product_type_name,
			n.product_subtype_name,
			n.effective_operation_template_id,
			COALESCE(n.order_nos,'') AS order_nos,
			n.spec_g,
			n.need_units,
			(n.need_units * n.spec_g) AS need_g,
			COALESCE(fi.onhand_units,0) AS inv_units,
			COALESCE(fi.onhand_loose_g,0) AS inv_loose_g,
			(COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0)) AS inv_g,
			(
				(n.force_produce_units * n.spec_g)
				+ GREATEST(
					0,
					((n.need_units - n.force_produce_units) * n.spec_g)
					- GREATEST(
						0,
						(COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0))
						- COALESCE(reserved.reserved_g,0)
					)
				)
			)::bigint AS gap_g
		FROM need n
		LEFT JOIN %s.finished_inventory fi
			ON fi.product_id = n.product_id AND fi.spec_g = n.spec_g AND fi.warehouse = 'finished_goods'
		LEFT JOIN reserved
			ON reserved.product_id = n.product_id AND reserved.spec_g = n.spec_g
		WHERE n.spec_g > 0
		ORDER BY gap_g DESC, n.product, n.spec_g
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema, where, schema, schema, schema, schema, schema, schema, schema, strings.Join(demandWhere, " AND "), schema, schema, schema)

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UnprodNeedRow, 0)
	for rows.Next() {
		var r UnprodNeedRow
		if err := rows.Scan(&r.ProductID, &r.Product, &r.ProductionKind, &r.ProductTypeCategoryID, &r.ProductSubtypeCategoryID, &r.ProductTypeName, &r.ProductSubtypeName, &r.OperationTemplateID, &r.OrderNos, &r.SpecG, &r.NeedUnits, &r.NeedG, &r.InvUnits, &r.InvLooseG, &r.InvG, &r.GapG); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
