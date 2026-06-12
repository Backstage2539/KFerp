package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	stockdomain "orderapp/internal/domain/stock"
	"strings"
)

func (r Repository) MRPSuggestions(ctx context.Context, query productionapp.MRPSuggestionQuery) (productionapp.MRPSuggestionResult, error) {
	args := []any{query.From, query.To}
	where := []string{"COALESCE(wo.planned_start_at, wo.created_at)::date >= $1::date", "COALESCE(wo.planned_start_at, wo.created_at)::date <= $2::date"}
	if query.Status != "" {
		args = append(args, query.Status)
		where = append(where, fmt.Sprintf("wo.status=$%d", len(args)))
	} else {
		where = append(where, "wo.status IN ('released','running','partially_completed')")
	}
	if query.WorkCenter != "" {
		args = append(args, query.WorkCenter)
		where = append(where, fmt.Sprintf("COALESCE(wo.work_center,'')=$%d", len(args)))
	}
	if query.MaterialID > 0 {
		args = append(args, query.MaterialID)
		where = append(where, fmt.Sprintf("res.material_id=$%d", len(args)))
	}
	args = append(args, query.Limit)
	limitArg := len(args)

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH demand AS (
			SELECT res.material_id,
			       MAX(res.material_name) AS material_name,
			       MAX(res.unit) AS unit,
			       COALESCE(SUM(res.required_g),0)::bigint AS required_g,
			       COALESCE(SUM(res.required_units),0)::bigint AS required_units,
			       COALESCE(SUM(res.reserved_g),0)::bigint AS reserved_g,
			       COALESCE(SUM(res.consumed_g),0)::bigint AS consumed_g,
			       COALESCE(SUM(res.returned_g),0)::bigint AS returned_g,
			       COUNT(DISTINCT wo.id)::bigint AS work_order_count,
			       COALESCE(string_agg(DISTINCT COALESCE(wo.work_order_no,''), ',' ORDER BY COALESCE(wo.work_order_no,'')), '') AS source_work_orders,
			       COALESCE(to_char(MIN(COALESCE(wo.planned_start_at, wo.created_at)), 'YYYY-MM-DD HH24:MI'), '') AS earliest_planned_at
			FROM %s.work_order_material_reservations res
			JOIN %s.work_orders wo ON wo.id=res.work_order_id
			WHERE %s
			  AND res.material_id > 0
			  AND res.status IN ('reserved','consumed')
			GROUP BY res.material_id
		)
		SELECT demand.material_id,demand.material_name,demand.unit,demand.required_g,demand.required_units,demand.reserved_g,demand.consumed_g,demand.returned_g,
		       COALESCE(wip.wip_g,0)::bigint AS wip_g,
		       COALESCE(raw.raw_g,0)::bigint AS raw_g,
		       COALESCE(open_res.open_reserved_g,0)::bigint AS open_reserved_g,
		       demand.work_order_count,demand.source_work_orders,demand.earliest_planned_at
		FROM demand
		LEFT JOIN LATERAL (
			SELECT SUM(l.qty_g)::bigint AS wip_g
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_id=demand.material_id
			  AND l.warehouse=$%d
			  AND l.qty_g > 0
			  AND b.status='active'
			  AND b.remaining_g > 0
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		) wip ON true
		LEFT JOIN LATERAL (
			SELECT SUM(l.qty_g)::bigint AS raw_g
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_id=demand.material_id
			  AND l.warehouse=$%d
			  AND l.qty_g > 0
			  AND b.status='active'
			  AND b.remaining_g > 0
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		) raw ON true
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,r2.reserved_g-r2.consumed_g-r2.returned_g))::bigint AS open_reserved_g
			FROM %s.work_order_material_reservations r2
			WHERE r2.material_id=demand.material_id AND r2.status='reserved'
		) open_res ON true
		ORDER BY demand.earliest_planned_at, demand.material_id
		LIMIT $%d
	`, r.schema, r.schema, strings.Join(where, " AND "), r.schema, r.schema, len(args)+1, r.schema, r.schema, len(args)+2, r.schema, limitArg), append(args, string(stockdomain.WarehouseWIP), string(stockdomain.WarehouseRawMaterials))...)
	if err != nil {
		return productionapp.MRPSuggestionResult{}, err
	}
	defer rows.Close()

	out := make([]productionapp.MRPSuggestionRow, 0)
	var totalPurchaseG, totalTransferG int64
	for rows.Next() {
		var row productionapp.MRPSuggestionRow
		var openReservedG int64
		if err := rows.Scan(
			&row.MaterialID, &row.MaterialName, &row.Unit, &row.RequiredG, &row.RequiredUnits, &row.ReservedG, &row.ConsumedG, &row.ReturnedG,
			&row.WIPG, &row.RawG, &openReservedG, &row.WorkOrderCount, &row.SourceWorkOrders, &row.EarliestPlannedAt,
		); err != nil {
			return productionapp.MRPSuggestionResult{}, err
		}
		remainingRequiredG := row.RequiredG - row.ConsumedG
		if remainingRequiredG < 0 {
			remainingRequiredG = 0
		}
		row.AvailableG = row.WIPG - openReservedG
		if row.AvailableG < 0 {
			row.AvailableG = 0
		}
		row.WIPTransferSuggestionG = remainingRequiredG - row.AvailableG
		if row.WIPTransferSuggestionG < 0 {
			row.WIPTransferSuggestionG = 0
		}
		if row.WIPTransferSuggestionG > row.RawG {
			row.WIPTransferSuggestionG = row.RawG
		}
		row.ShortageG = remainingRequiredG - row.AvailableG - row.RawG
		if row.ShortageG < 0 {
			row.ShortageG = 0
		}
		row.PurchaseSuggestionG = row.ShortageG
		row.SuggestionType = "covered"
		if row.WIPTransferSuggestionG > 0 {
			row.SuggestionType = "transfer_suggestion"
		}
		if row.PurchaseSuggestionG > 0 {
			row.SuggestionType = "purchase_suggestion"
		}
		totalPurchaseG += row.PurchaseSuggestionG
		totalTransferG += row.WIPTransferSuggestionG
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return productionapp.MRPSuggestionResult{}, err
	}
	return productionapp.MRPSuggestionResult{Rows: out, PurchaseSuggestionG: totalPurchaseG, TransferSuggestionG: totalTransferG}, nil
}
