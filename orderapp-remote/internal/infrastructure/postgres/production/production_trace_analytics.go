package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"strings"
)

func (r Repository) ProductionTraceAnalytics(ctx context.Context, query productionapp.ProductionTraceAnalyticsQuery) (productionapp.ProductionTraceAnalyticsResult, error) {
	traceLinks, err := r.productionTraceLinks(ctx, query)
	if err != nil {
		return productionapp.ProductionTraceAnalyticsResult{}, err
	}
	costVariance, err := r.productionCostVariance(ctx, query)
	if err != nil {
		return productionapp.ProductionTraceAnalyticsResult{}, err
	}
	abnormalLosses, err := r.productionAbnormalLosses(ctx, query)
	if err != nil {
		return productionapp.ProductionTraceAnalyticsResult{}, err
	}
	var totalVariance float64
	for _, row := range costVariance {
		totalVariance += row.Variance
	}
	return productionapp.ProductionTraceAnalyticsResult{
		TraceLinks:        traceLinks,
		CostVariance:      costVariance,
		AbnormalLosses:    abnormalLosses,
		TotalVariance:     totalVariance,
		AbnormalLossCount: len(abnormalLosses),
	}, nil
}

func (r Repository) productionTraceLinks(ctx context.Context, query productionapp.ProductionTraceAnalyticsQuery) ([]productionapp.ProductionTraceLinkRow, error) {
	where, args := productionTraceWhere(query)
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.running_item_id,COALESCE(wo.batch_id,''),
		       COALESCE(jc.id,0),COALESCE(jc.operation,''),COALESCE(jc.status,''),
		       COALESCE(se.id,0),COALESCE(se.entry_no,''),COALESCE(se.entry_type,''),
		       COALESCE(si.material_id,0),COALESCE(NULLIF(si.item_name,''), ''),
		       COALESCE(si.batch_code,''),COALESCE(si.qty_g,0)::bigint,
		       COALESCE(to_char(se.created_at,'YYYY-MM-DD HH24:MI'), to_char(wo.created_at,'YYYY-MM-DD HH24:MI'))
			FROM %s.work_orders wo
			JOIN %s.stock_entries se ON se.work_order_id=wo.id OR (wo.running_item_id > 0 AND se.running_item_id=wo.running_item_id)
			JOIN %s.stock_entry_items si ON si.stock_entry_id=se.id
			LEFT JOIN %s.job_cards jc ON jc.id=se.job_card_id
			WHERE %s
			ORDER BY se.created_at DESC, wo.id DESC, jc.sequence_no, si.id
			LIMIT $%d
		`, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionTraceLinkRow, 0)
	for rows.Next() {
		var row productionapp.ProductionTraceLinkRow
		if err := rows.Scan(
			&row.WorkOrderID, &row.WorkOrderNo, &row.RunningItemID, &row.BatchID,
			&row.JobCardID, &row.Operation, &row.JobCardStatus,
			&row.StockEntryID, &row.EntryNo, &row.EntryType,
			&row.MaterialID, &row.MaterialName, &row.BatchCode, &row.QtyG, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) productionCostVariance(ctx context.Context, query productionapp.ProductionTraceAnalyticsQuery) ([]productionapp.ProductionCostVarianceRow, error) {
	where, args := productionTraceWhere(query)
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,COALESCE(NULLIF(pbc.batch_id,''), wo.batch_id, ''),wo.product_name,
		       COALESCE(pbc.unit_cost_per_kg * (NULLIF(wo.planned_g,0)::float8 / 1000.0), 0)::float8 AS planned_cost,
		       COALESCE(NULLIF(wo.actual_cost,0), pbc.total_cost, 0)::float8 AS actual_cost,
		       COALESCE((
		           SELECT SUM(COALESCE(jc.planned_operation_cost,0))::float8
		           FROM %s.job_cards jc
		           WHERE jc.work_order_id=wo.id
		       ),0)::float8 AS planned_operation_cost,
		       COALESCE((
		           SELECT SUM(CASE
		               WHEN COALESCE(jc.actual_operation_cost,0) > 0 THEN COALESCE(jc.actual_operation_cost,0)
		               ELSE COALESCE(jc.planned_operation_cost,0)
		           END)::float8
		           FROM %s.job_cards jc
		           WHERE jc.work_order_id=wo.id
		       ),0)::float8 AS actual_operation_cost
		FROM %s.work_orders wo
		LEFT JOIN %s.production_batch_costs pbc ON pbc.running_item_id=wo.running_item_id OR (pbc.batch_id <> '' AND pbc.batch_id=wo.batch_id)
		WHERE %s
		ORDER BY wo.completed_at DESC NULLS LAST, wo.created_at DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionCostVarianceRow, 0)
	for rows.Next() {
		var row productionapp.ProductionCostVarianceRow
		if err := rows.Scan(&row.WorkOrderID, &row.WorkOrderNo, &row.BatchID, &row.ProductName, &row.PlannedCost, &row.ActualCost, &row.PlannedOperationCost, &row.ActualOperationCost); err != nil {
			return nil, err
		}
		row.Variance = row.ActualCost - row.PlannedCost
		if row.PlannedCost != 0 {
			row.VarianceRate = row.Variance / row.PlannedCost
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) productionAbnormalLosses(ctx context.Context, query productionapp.ProductionTraceAnalyticsQuery) ([]productionapp.ProductionAbnormalLossRow, error) {
	where, args := productionTraceWhere(query)
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT jc.id,wo.id,wo.work_order_no,jc.operation,
		       COALESCE(jc.actual_input_qty,0)::float8,COALESCE(jc.actual_output_qty,0)::float8,COALESCE(jc.actual_loss_qty,0)::float8,COALESCE(jc.actual_loss_rate,0)::float8,
		       COALESCE(jc.loss_reason,''),COALESCE(jc.exception_reason,'')
		FROM %s.job_cards jc
		JOIN %s.work_orders wo ON wo.id=jc.work_order_id
		WHERE %s
		  AND (COALESCE(jc.actual_loss_rate,0) >= 0.05 OR COALESCE(jc.loss_reason,'') <> '' OR COALESCE(jc.exception_reason,'') <> '')
		ORDER BY jc.actual_loss_rate DESC, jc.completed_at DESC NULLS LAST, jc.id DESC
		LIMIT $%d
	`, r.schema, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.ProductionAbnormalLossRow, 0)
	for rows.Next() {
		var row productionapp.ProductionAbnormalLossRow
		if err := rows.Scan(&row.JobCardID, &row.WorkOrderID, &row.WorkOrderNo, &row.Operation, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate, &row.LossReason, &row.ExceptionReason); err != nil {
			return nil, err
		}
		row.Severity = abnormalLossSeverity(row.ActualLossRate)
		out = append(out, row)
	}
	return out, rows.Err()
}

func productionTraceWhere(query productionapp.ProductionTraceAnalyticsQuery) (string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if query.WorkOrderID > 0 {
		args = append(args, query.WorkOrderID)
		where = append(where, fmt.Sprintf("wo.id=$%d", len(args)))
	}
	if query.BatchID != "" {
		args = append(args, query.BatchID)
		where = append(where, fmt.Sprintf("wo.batch_id=$%d", len(args)))
	}
	return strings.Join(where, " AND "), args
}

func abnormalLossSeverity(rate float64) string {
	if rate >= 0.15 {
		return "error"
	}
	if rate >= 0.05 {
		return "warning"
	}
	return "info"
}
