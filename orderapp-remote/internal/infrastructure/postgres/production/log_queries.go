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
		where += " AND product_id=$" + strconv.Itoa(len(args))
	}
	if query.RunningItemID > 0 {
		args = append(args, query.RunningItemID)
		where += " AND running_item_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.BatchID) != "" {
		args = append(args, strings.TrimSpace(query.BatchID))
		where += " AND batch_id=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.Operator) != "" {
		args = append(args, strings.TrimSpace(query.Operator))
		where += " AND finished_by=$" + strconv.Itoa(len(args))
	}
	if strings.TrimSpace(query.From) != "" {
		args = append(args, strings.TrimSpace(query.From))
		where += " AND finished_at >= $" + strconv.Itoa(len(args)) + "::date"
	}
	if strings.TrimSpace(query.To) != "" {
		args = append(args, strings.TrimSpace(query.To))
		where += " AND finished_at < ($" + strconv.Itoa(len(args)) + "::date + INTERVAL '1 day')"
	}
	args = append(args, limit)

	q := `
		SELECT id,batch_id,product_id,product_name,spec_g,order_nos,
		       planned_need_g,input_g,bom_yield_rate,
		       finished_units,finished_loose_g,finished_total_g,actual_yield_rate,
		       started_by,COALESCE(to_char(started_at,'YYYY-MM-DD HH24:MI'),''),
		       finished_by,COALESCE(to_char(finished_at,'YYYY-MM-DD HH24:MI'),''),
		       inventory_units_before,inventory_loose_g_before,
		       inventory_units_after,inventory_loose_g_after,
		       COALESCE(material_summary::text,'[]')
		FROM ` + r.schema + `.production_logs
		` + where + `
		ORDER BY finished_at DESC NULLS LAST, id DESC
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
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, dbRows.Err()
}
