package production

import (
	"context"
	"strconv"
	"strings"

	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func (r Repository) ListProductionLogs(ctx context.Context, query productionapp.ProductionLogsQuery) (productionapp.ProductionLogsResult, error) {
	products, err := postgresinfra.FetchProducts(ctx, r.pool, r.schema)
	if err != nil {
		return productionapp.ProductionLogsResult{}, err
	}
	productOptions := make([]productionapp.ProductionLogProductOption, 0, len(products))
	for _, product := range products {
		productOptions = append(productOptions, productionapp.ProductionLogProductOption{ID: product.ID, Name: product.Name})
	}
	rows, err := r.listProductionLogs(ctx, query)
	if err != nil {
		return productionapp.ProductionLogsResult{}, err
	}
	return productionapp.ProductionLogsResult{Products: productOptions, Rows: rows}, nil
}

func (r Repository) listProductionLogs(ctx context.Context, query productionapp.ProductionLogsQuery) ([]productionapp.ProductionLogRow, error) {
	limit := query.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args := []any{}
	where := "WHERE 1=1"
	if query.ProductID > 0 {
		args = append(args, query.ProductID)
		where += " AND pl.product_id=$" + strconv.Itoa(len(args))
	}
	if query.RunningItemID > 0 {
		args = append(args, query.RunningItemID)
		where += " AND pl.running_item_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.BatchID) != "" {
		args = append(args, strings.TrimSpace(query.BatchID))
		where += " AND pl.batch_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.Operator) != "" {
		args = append(args, strings.TrimSpace(query.Operator))
		where += " AND pl.finished_by=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.From) != "" {
		args = append(args, strings.TrimSpace(query.From))
		where += " AND pl.finished_at >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(query.To) != "" {
		args = append(args, strings.TrimSpace(query.To))
		where += " AND pl.finished_at < ($" + strconv.Itoa(len(args)) + "::date + INTERVAL '1 day')"
	}
	args = append(args, limit)

	q := `
		SELECT pl.id,pl.batch_id,pl.product_id,pl.product_name,pl.spec_g,pl.order_nos,
		       pl.planned_need_g,pl.input_g,pl.bom_yield_rate,
		       pl.finished_units,pl.finished_loose_g,pl.finished_total_g,pl.actual_yield_rate,
		       pl.started_by,COALESCE(to_char(pl.started_at,'YYYY-MM-DD HH24:MI'),''),
		       pl.finished_by,COALESCE(to_char(pl.finished_at,'YYYY-MM-DD HH24:MI'),''),
		       pl.inventory_units_before,pl.inventory_loose_g_before,
		       pl.inventory_units_after,pl.inventory_loose_g_after,
		       COALESCE(pl.material_summary::text,'[]'),
		       COALESCE(finished_batch.batch_code,'')
		FROM ` + r.schema + `.production_logs pl
		LEFT JOIN LATERAL (
			SELECT b.batch_code
			FROM ` + r.schema + `.stock_batches b
			WHERE b.source_doc_type='production_run'
			  AND b.source_doc_id=pl.running_item_id
			  AND b.item_type='finished_product'
			  AND b.item_id=pl.product_id
			  AND b.spec_g=pl.spec_g
			ORDER BY b.created_at DESC, b.id DESC
			LIMIT 1
		) finished_batch ON true
		` + where + `
		ORDER BY pl.finished_at DESC NULLS LAST, pl.id DESC
		LIMIT $` + strconv.Itoa(len(args))

	dbRows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	out := make([]productionapp.ProductionLogRow, 0)
	for dbRows.Next() {
		var row productionapp.ProductionLogRow
		if err := dbRows.Scan(
			&row.ID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.OrderNos,
			&row.PlannedNeedG, &row.InputG, &row.BomYieldRate,
			&row.FinishedUnits, &row.FinishedLooseG, &row.FinishedTotalG, &row.ActualYieldRate,
			&row.StartedBy, &row.StartedAt,
			&row.FinishedBy, &row.FinishedAt,
			&row.InventoryUnitsBefore, &row.InventoryLooseGBefore,
			&row.InventoryUnitsAfter, &row.InventoryLooseGAfter,
			&row.MaterialSummary,
			&row.FinishedBatchCode,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, dbRows.Err()
}
