package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r Repository) CreateStockEntry(ctx context.Context, cmd productionapp.StockEntryCommand) (productionapp.StockEntryDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := createStockEntryRecordTx(ctx, tx, r.schema, cmd, true)
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	return detail, nil
}

func (r Repository) ListStockEntries(ctx context.Context, query productionapp.StockEntryQuery) ([]productionapp.StockEntryRow, error) {
	where := []string{"1=1"}
	args := []any{}
	if query.EntryType != "" {
		args = append(args, query.EntryType)
		where = append(where, fmt.Sprintf("se.entry_type=$%d", len(args)))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		where = append(where, fmt.Sprintf("se.status=$%d", len(args)))
	}
	if query.WorkOrderID > 0 {
		args = append(args, query.WorkOrderID)
		where = append(where, fmt.Sprintf("se.work_order_id=$%d", len(args)))
	}
	if query.JobCardID > 0 {
		args = append(args, query.JobCardID)
		where = append(where, fmt.Sprintf("se.job_card_id=$%d", len(args)))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT se.id,se.entry_no,se.entry_type,se.status,se.work_order_id,se.job_card_id,se.running_item_id,
		       se.source_type,se.source_id,COUNT(si.id)::bigint,COALESCE(SUM(si.qty_g),0)::bigint,
		       COALESCE(SUM(si.total_cost),0)::float8,se.operator,se.note,to_char(se.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_entries se
		LEFT JOIN %s.stock_entry_items si ON si.stock_entry_id=se.id
		WHERE %s
		GROUP BY se.id
		ORDER BY se.created_at DESC,se.id DESC
		LIMIT $%d
	`, r.schema, r.schema, strings.Join(where, " AND "), limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.StockEntryRow, 0)
	for rows.Next() {
		var row productionapp.StockEntryRow
		if err := rows.Scan(&row.ID, &row.EntryNo, &row.EntryType, &row.Status, &row.WorkOrderID, &row.JobCardID, &row.RunningItemID, &row.SourceType, &row.SourceID, &row.ItemCount, &row.TotalQtyG, &row.TotalCost, &row.Operator, &row.Note, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) GetStockEntry(ctx context.Context, id int64) (productionapp.StockEntryDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := loadStockEntryDetailTx(ctx, tx, r.schema, id)
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	return detail, nil
}

func (r Repository) ListWorkOrderLedgerEntries(ctx context.Context, query productionapp.WorkOrderLedgerQuery) ([]productionapp.WorkOrderLedgerEntryRow, error) {
	where, args := workOrderLedgerWhere(query)
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.id,
		       COALESCE(se.id,0),
		       COALESCE(se.entry_no,''),
		       COALESCE(se.entry_type, CASE WHEN l.source_doc_type='production_run' THEN 'finished_receipt' ELSE '' END),
		       l.item_type,l.item_id,l.item_name,l.spec_g,l.warehouse,
		       l.qty_change_g,l.qty_after_g,l.qty_change_units,l.qty_after_units,
		       l.source_doc_type,l.source_doc_id,l.source_batch_code,l.operator,
		       to_char(l.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_ledger_entries l
		LEFT JOIN %s.stock_entries se ON se.id=l.source_doc_id AND l.source_doc_type='stock_entry'
		LEFT JOIN %s.work_orders wo ON wo.running_item_id=l.source_doc_id AND l.source_doc_type='production_run'
		WHERE %s
		ORDER BY l.created_at DESC,l.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, strings.Join(where, " AND "), limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderLedgerEntryRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderLedgerEntryRow
		if err := rows.Scan(
			&row.ID, &row.StockEntryID, &row.EntryNo, &row.EntryType,
			&row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.Warehouse,
			&row.QtyChangeG, &row.QtyAfterG, &row.QtyChangeUnits, &row.QtyAfterUnits,
			&row.SourceDocType, &row.SourceDocID, &row.SourceBatchCode, &row.Operator, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		row.Purpose = productionapp.StockEntryPurposeForType(row.EntryType)
		out = append(out, row)
	}
	return out, rows.Err()
}

func workOrderLedgerWhere(query productionapp.WorkOrderLedgerQuery) ([]string, []any) {
	args := []any{}
	where := []string{"1=1"}
	evidencePredicates := []string{}
	if query.WorkOrderID > 0 {
		args = append(args, query.WorkOrderID)
		evidencePredicates = append(evidencePredicates, fmt.Sprintf("(se.work_order_id=$%d OR wo.id=$%d)", len(args), len(args)))
	}
	if query.RunningItemID > 0 {
		args = append(args, query.RunningItemID)
		evidencePredicates = append(evidencePredicates, fmt.Sprintf("(se.running_item_id=$%d OR (l.source_doc_type='production_run' AND l.source_doc_id=$%d) OR wo.running_item_id=$%d)", len(args), len(args), len(args)))
	}
	if len(evidencePredicates) > 0 {
		where = append(where, "("+strings.Join(evidencePredicates, " OR ")+")")
	}
	return where, args
}

func createStockEntryRecordTx(ctx context.Context, tx pgx.Tx, schema string, cmd productionapp.StockEntryCommand, writeLedger bool) (productionapp.StockEntryDetail, error) {
	tempEntryNo := fmt.Sprintf("SE-TMP-%d", time.Now().UnixNano())
	var entryID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entries(entry_no,entry_type,status,work_order_id,job_card_id,running_item_id,source_type,source_id,operator,note,created_at)
		VALUES($1,$2,'submitted',$3,$4,$5,$6,$7,$8,$9,now())
		RETURNING id
	`, schema), tempEntryNo, cmd.EntryType, cmd.WorkOrderID, cmd.JobCardID, cmd.RunningItemID, cmd.SourceType, cmd.SourceID, cmd.Operator, cmd.Note).Scan(&entryID); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	entryNo := fmt.Sprintf("SE-%010d", entryID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entries SET entry_no=$2 WHERE id=$1`, schema), entryID, entryNo); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	for _, item := range cmd.Items {
		totalCost := item.UnitCost
		if item.QtyG > 0 {
			totalCost = (float64(item.QtyG) / 1000.0) * item.UnitCost
		} else if item.QtyUnits > 0 {
			totalCost = float64(item.QtyUnits) * item.UnitCost
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_entry_items(
				stock_entry_id,material_id,product_id,item_type,item_name,spec_g,
				from_warehouse,to_warehouse,qty_g,qty_units,batch_code,unit_cost,total_cost
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		`, schema), entryID, item.MaterialID, item.ProductID, item.ItemType, item.ItemName, item.SpecG, item.FromWarehouse, item.ToWarehouse, item.QtyG, item.QtyUnits, item.BatchCode, item.UnitCost, totalCost); err != nil {
			return productionapp.StockEntryDetail{}, err
		}
		if writeLedger {
			if err := insertStockEntryLedgerTx(ctx, tx, schema, entryID, entryNo, cmd, item); err != nil {
				return productionapp.StockEntryDetail{}, err
			}
		}
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, schema, cmd.Operator, "stock_entry", &entryID, "submit", postgresinfra.StrPtr("entry_type"), nil, postgresinfra.StrPtr(cmd.EntryType), postgresinfra.AuditMeta{"entry_no": entryNo, "work_order_id": cmd.WorkOrderID, "job_card_id": cmd.JobCardID, "running_item_id": cmd.RunningItemID, "source_type": cmd.SourceType, "source_id": cmd.SourceID}); err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	return loadStockEntryDetailTx(ctx, tx, schema, entryID)
}

func insertStockEntryLedgerTx(ctx context.Context, tx pgx.Tx, schema string, entryID int64, entryNo string, cmd productionapp.StockEntryCommand, item productionapp.StockEntryItemCommand) error {
	itemID := item.MaterialID
	if item.ProductID > 0 {
		itemID = item.ProductID
	}
	itemName := item.ItemName
	qty := stockLedgerQty{ChangeG: item.QtyG, AfterG: item.QtyG, ChangeUnits: item.QtyUnits, AfterUnits: item.QtyUnits}
	if item.FromWarehouse != "" {
		fromQty := qty
		fromQty.ChangeG = -item.QtyG
		fromQty.AfterG = -item.QtyG
		fromQty.ChangeUnits = -item.QtyUnits
		fromQty.AfterUnits = -item.QtyUnits
		if err := insertStockLedgerEntryTx(ctx, tx, schema, item.ItemType, itemID, itemName, item.SpecG, item.FromWarehouse, "stock_entry", entryID, item.BatchCode, entryNo, fromQty, cmd.Operator); err != nil {
			return err
		}
	}
	if item.ToWarehouse != "" {
		return insertStockLedgerEntryTx(ctx, tx, schema, item.ItemType, itemID, itemName, item.SpecG, item.ToWarehouse, "stock_entry", entryID, item.BatchCode, entryNo, qty, cmd.Operator)
	}
	return nil
}

func loadStockEntryDetailTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.StockEntryDetail, error) {
	var detail productionapp.StockEntryDetail
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,entry_no,entry_type,status,work_order_id,job_card_id,running_item_id,source_type,source_id,operator,note,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_entries
		WHERE id=$1
	`, schema), id).Scan(&detail.ID, &detail.EntryNo, &detail.EntryType, &detail.Status, &detail.WorkOrderID, &detail.JobCardID, &detail.RunningItemID, &detail.SourceType, &detail.SourceID, &detail.Operator, &detail.Note, &detail.CreatedAt)
	if err == pgx.ErrNoRows {
		return productionapp.StockEntryDetail{}, fmt.Errorf("stock entry not found")
	}
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,stock_entry_id,material_id,product_id,item_type,item_name,spec_g,from_warehouse,to_warehouse,qty_g,qty_units,batch_code,COALESCE(unit_cost,0)::float8,COALESCE(total_cost,0)::float8
		FROM %s.stock_entry_items
		WHERE stock_entry_id=$1
		ORDER BY id
	`, schema), id)
	if err != nil {
		return productionapp.StockEntryDetail{}, err
	}
	defer rows.Close()
	detail.Items = make([]productionapp.StockEntryItemRow, 0)
	for rows.Next() {
		var item productionapp.StockEntryItemRow
		if err := rows.Scan(&item.ID, &item.StockEntryID, &item.MaterialID, &item.ProductID, &item.ItemType, &item.ItemName, &item.SpecG, &item.FromWarehouse, &item.ToWarehouse, &item.QtyG, &item.QtyUnits, &item.BatchCode, &item.UnitCost, &item.TotalCost); err != nil {
			return productionapp.StockEntryDetail{}, err
		}
		detail.Items = append(detail.Items, item)
	}
	return detail, rows.Err()
}

func (r Repository) TransitionJobCard(ctx context.Context, cmd productionapp.JobCardActionCommand) (productionapp.JobCardActionResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT work_order_id FROM %s.job_cards WHERE id=$1`, r.schema), cmd.ID).Scan(&workOrderID); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.JobCardActionResult{}, fmt.Errorf("job card not found")
		}
		return productionapp.JobCardActionResult{}, err
	}
	var workOrderStatus string
	var runningItemID int64
	var inventoryQtyPerSalesUnit float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,COALESCE(running_item_id,0),COALESCE(inventory_qty_per_sales_unit,0)::float8
		FROM %s.work_orders
		WHERE id=$1
		FOR UPDATE
	`, r.schema), workOrderID).Scan(&workOrderStatus, &runningItemID, &inventoryQtyPerSalesUnit); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.JobCardActionResult{}, fmt.Errorf("work order not found for job card")
		}
		return productionapp.JobCardActionResult{}, err
	}
	var currentStatus string
	var currentCostMethod string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT status,COALESCE(NULLIF(cost_method,''),'time')
		FROM %s.job_cards
		WHERE id=$1 AND work_order_id=$2
		FOR UPDATE
	`, r.schema), cmd.ID, workOrderID).Scan(&currentStatus, &currentCostMethod); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.JobCardActionResult{}, fmt.Errorf("job card not found")
		}
		return productionapp.JobCardActionResult{}, err
	}
	if cmd.Action == "start" && !jobCardStartAllowedForWorkOrder(workOrderStatus, runningItemID) {
		return productionapp.JobCardActionResult{}, fmt.Errorf("work order must be running before job card start")
	}
	if !validJobCardTransition(currentStatus, cmd.Action) {
		return productionapp.JobCardActionResult{}, fmt.Errorf("invalid job card action %s from %s", cmd.Action, currentStatus)
	}
	actualPieceQty := actualPieceQuantity(cmd.MetricsJSON, cmd.ActualOutputQty, inventoryQtyPerSalesUnit)
	if cmd.Action == "complete" && normalizeProductionCostMethod(currentCostMethod) == "piece" && actualPieceQty <= 0 {
		return productionapp.JobCardActionResult{}, fmt.Errorf("计件工序完成时必须填写成品件数，或提供可按冻结规格换算的实际产出数量")
	}
	nextStatus := nextJobCardStatus(cmd.Action)
	switch cmd.Action {
	case "start":
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status=$2,started_at=now(),operator=$3 WHERE id=$1`, r.schema), cmd.ID, nextStatus, cmd.Operator)
	case "pause":
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status=$2,paused_at=now(),operator=$3 WHERE id=$1`, r.schema), cmd.ID, nextStatus, cmd.Operator)
	case "resume":
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET status=$2,resumed_at=now(),operator=$3 WHERE id=$1`, r.schema), cmd.ID, nextStatus, cmd.Operator)
	case "complete":
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.job_cards
			SET status=$2,
			    completed_at=now(),
			    operator=$3,
			    actual_input_qty=$4,
			    actual_output_qty=$5,
			    actual_loss_qty=$6,
			    actual_loss_rate=$7,
			    loss_reason=$8,
			    exception_reason=$9,
			    metrics_json=$10::jsonb,
			    actual_minutes=$11,
			    actual_operation_cost=CASE
			        WHEN COALESCE(NULLIF(cost_method,''),'time')='piece'
			            THEN CASE WHEN $12 > 0
			                THEN ROUND($12::numeric * COALESCE(piece_rate,0), 4)
			                ELSE actual_operation_cost
			            END
			        WHEN $11 > 0 THEN ROUND(($11::numeric / 60.0) * COALESCE(hourly_rate,0), 4)
			        ELSE actual_operation_cost
			    END
			WHERE id=$1
		`, r.schema), cmd.ID, nextStatus, cmd.Operator, cmd.ActualInputQty, cmd.ActualOutputQty, cmd.ActualLossQty, cmd.ActualLossRate, cmd.LossReason, cmd.ExceptionReason, cmd.MetricsJSON, cmd.ActualMinutes, actualPieceQty)
	}
	if err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, r.schema), workOrderID, summary); err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	if err := updateWorkOrderStatusFromJobCardsTx(ctx, tx, r.schema, workOrderID); err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.ID, cmd.Action, postgresinfra.StrPtr("status"), postgresinfra.StrPtr(currentStatus), postgresinfra.StrPtr(nextStatus), postgresinfra.AuditMeta{"work_order_id": workOrderID, "actual_input_qty": cmd.ActualInputQty, "actual_output_qty": cmd.ActualOutputQty, "actual_piece_qty": actualPieceQty, "actual_loss_qty": cmd.ActualLossQty, "actual_loss_rate": cmd.ActualLossRate, "actual_minutes": cmd.ActualMinutes, "loss_reason": cmd.LossReason, "exception_reason": cmd.ExceptionReason}); err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	card, err := loadJobCardRowTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	workOrder, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.JobCardActionResult{}, err
	}
	return productionapp.JobCardActionResult{JobCard: card, WorkOrder: workOrder}, nil
}

func (r Repository) CompleteWorkOrder(ctx context.Context, cmd productionapp.WorkOrderCompleteCommand) (productionapp.WorkOrderCompleteResult, error) {
	wo, incomplete, err := r.workOrderCompletionPrecheck(ctx, cmd.ID)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if wo.RunningItemID <= 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("work order has not started")
	}
	if incomplete > 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("work order has unfinished job cards")
	}
	if wo.Status == "completed" {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("work order already completed")
	}
	completionWarehouse, err := completionWarehouseForWorkOrder(wo, cmd.Warehouse)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := r.Finish(ctx, productionapp.FinishCommand{ID: wo.RunningItemID, FinishedUnits: cmd.FinishedUnits, FinishedLooseG: cmd.FinishedLooseG, HasFinishedInput: true, Warehouse: completionWarehouse, ConsumedInputG: cmd.ConsumedInputG, Operator: cmd.Operator}); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finishedG := cmd.FinishedLooseG
	if wo.SpecG > 0 {
		finishedG += cmd.FinishedUnits * wo.SpecG
	}
	entry, err := createStockEntryRecordTx(ctx, tx, r.schema, productionapp.StockEntryCommand{
		EntryType:     "finished_receipt",
		WorkOrderID:   wo.ID,
		RunningItemID: wo.RunningItemID,
		SourceType:    "work_order_complete",
		SourceID:      wo.ID,
		Operator:      cmd.Operator,
		Note:          cmd.Note,
		Items: []productionapp.StockEntryItemCommand{{
			ProductID:   wo.ProductID,
			ItemType:    stockItemTypeFinishedProduct,
			ItemName:    wo.ProductName,
			SpecG:       wo.SpecG,
			ToWarehouse: completionWarehouse,
			QtyG:        finishedG,
			QtyUnits:    cmd.FinishedUnits,
		}},
	}, false)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	cost, err := loadBatchCostForRunningItemTx(ctx, tx, r.schema, wo.RunningItemID)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	updated, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, wo.ID)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	return productionapp.WorkOrderCompleteResult{WorkOrder: updated, StockEntries: []productionapp.StockEntryRow{stockEntryRowFromDetail(entry)}, Cost: cost}, nil
}

func completionWarehouseForWorkOrder(wo productionapp.WorkOrderRow, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if wo.ProcessingRequestItemID <= 0 {
		if requested == "" {
			return stockdomain.WarehouseFinishedGoods, nil
		}
		return requested, nil
	}
	target := strings.TrimSpace(wo.TargetWarehouse)
	if target == "" {
		return "", fmt.Errorf("customer processing target warehouse missing")
	}
	if requested != "" && requested != target {
		return "", fmt.Errorf("customer processing completion warehouse must be %s", target)
	}
	return target, nil
}

func validJobCardTransition(current, action string) bool {
	switch action {
	case "start":
		return current == "pending" || current == "ready"
	case "pause":
		return current == "running"
	case "resume":
		return current == "paused"
	case "complete":
		return current == "running" || current == "paused"
	default:
		return false
	}
}

func jobCardStartAllowedForWorkOrder(status string, runningItemID int64) bool {
	if runningItemID <= 0 {
		return false
	}
	switch strings.TrimSpace(status) {
	case "running", "partially_completed":
		return true
	default:
		return false
	}
}

func nextJobCardStatus(action string) string {
	switch action {
	case "pause":
		return "paused"
	case "complete":
		return "completed"
	default:
		return "running"
	}
}

func updateWorkOrderStatusFromJobCardsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) error {
	var current string
	var total, completed, running, paused int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.status,
		       COUNT(jc.id)::bigint,
		       COUNT(jc.id) FILTER (WHERE jc.status='completed')::bigint,
		       COUNT(jc.id) FILTER (WHERE jc.status='running')::bigint,
		       COUNT(jc.id) FILTER (WHERE jc.status='paused')::bigint
		FROM %s.work_orders wo
		LEFT JOIN %s.job_cards jc ON jc.work_order_id=wo.id
		WHERE wo.id=$1
		GROUP BY wo.id
	`, schema, schema), workOrderID).Scan(&current, &total, &completed, &running, &paused); err != nil {
		return err
	}
	if current == "completed" || current == "cancelled" {
		return nil
	}
	next := deriveWorkOrderStatusFromJobCardCounts(current, total, completed, running, paused)
	if next == current {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status=$2 WHERE id=$1`, schema), workOrderID, next)
	return err
}

func deriveWorkOrderStatusFromJobCardCounts(current string, total, completed, running, paused int64) string {
	if completed > 0 {
		return "partially_completed"
	}
	if running > 0 {
		return "running"
	}
	if paused > 0 {
		return "paused"
	}
	if total > 0 && current == "released" {
		return "released"
	}
	return current
}

func loadJobCardRowTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.JobCardRow, error) {
	var row productionapp.JobCardRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,sequence_no,
		       COALESCE(operation_id,0),COALESCE(workstation_id,0),
		       operation,workstation,
		       COALESCE(workstation_capacity_id,0),COALESCE(workstation_capacity_name,''),
		       COALESCE(batch_size_qty,0)::float8,COALESCE(batch_size_unit,''),
		       COALESCE(planned_batch_count,0),COALESCE(planned_minutes,0),
		       COALESCE(hourly_rate,0)::float8,COALESCE(NULLIF(cost_method,''),'time'),
		       COALESCE(piece_rate,0)::float8,COALESCE(planned_operation_cost,0)::float8,
		       COALESCE(actual_minutes,0),COALESCE(actual_operation_cost,0)::float8,
		       status,
		       COALESCE(to_char(started_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(paused_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(resumed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator,
		       COALESCE(planned_input_qty,0)::float8,
		       COALESCE(actual_input_qty,0)::float8,
		       COALESCE(actual_output_qty,0)::float8,
		       COALESCE(actual_loss_qty,0)::float8,
		       COALESCE(actual_loss_rate,0)::float8,
		       COALESCE(records_loss,false),
		       COALESCE(loss_reason,''),
		       COALESCE(exception_reason,''),
		       COALESCE(metrics_json,'{}'::jsonb)::text,
		       COALESCE(parameter_schema_json,'{}'::jsonb)::text
		FROM %s.job_cards
		WHERE id=$1
	`, schema), id).Scan(
		&row.ID, &row.WorkOrderID, &row.SequenceNo, &row.OperationID, &row.WorkstationID,
		&row.Operation, &row.Workstation, &row.WorkstationCapacityID, &row.WorkstationCapacityName,
		&row.BatchSizeQty, &row.BatchSizeUnit, &row.PlannedBatchCount, &row.PlannedMinutes,
		&row.HourlyRate, &row.CostMethod, &row.PieceRate, &row.PlannedOperationCost, &row.ActualMinutes, &row.ActualOperationCost,
		&row.Status, &row.StartedAt, &row.PausedAt, &row.ResumedAt, &row.CompletedAt, &row.Operator,
		&row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate,
		&row.RecordsLoss, &row.LossReason, &row.ExceptionReason, &row.MetricsJSON, &row.ParameterSchemaJSON,
	)
	if err == pgx.ErrNoRows {
		return productionapp.JobCardRow{}, fmt.Errorf("job card not found")
	}
	return row, err
}

func (r Repository) workOrderCompletionPrecheck(ctx context.Context, workOrderID int64) (productionapp.WorkOrderRow, int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderRow{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	wo, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return productionapp.WorkOrderRow{}, 0, err
	}
	var incomplete int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(1)::bigint FROM %s.job_cards WHERE work_order_id=$1 AND status NOT IN ('completed','cancelled')`, r.schema), workOrderID).Scan(&incomplete); err != nil {
		return productionapp.WorkOrderRow{}, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderRow{}, 0, err
	}
	return wo, incomplete, nil
}

func loadWorkOrderExecutionRowTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.WorkOrderRow, error) {
	var row productionapp.WorkOrderRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,planned_g,COALESCE(NULLIF(planned_output_g,0),planned_g),status,
		       COALESCE(actual_cost,0)::float8,to_char(created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(order_nos,''),COALESCE(bom_version_id,0),COALESCE(operation_template_id,0),COALESCE(process_template_id,0),COALESCE(process_template_name,''),
		       COALESCE(process_snapshot_json,'{}'::jsonb)::text,COALESCE(operation_summary_json,'[]'::jsonb)::text,
		       customer_id,target_warehouse,processing_request_item_id,
		       COALESCE((SELECT SUM(reserved_g)::bigint FROM %s.work_order_material_reservations WHERE work_order_id=work_orders.id),0),
		       COALESCE((SELECT SUM(consumed_g)::bigint FROM %s.work_order_material_reservations WHERE work_order_id=work_orders.id),0),
		       COALESCE((SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g))::bigint FROM %s.work_order_material_reservations WHERE work_order_id=work_orders.id AND status='reserved'),0)
		FROM %s.work_orders
		WHERE id=$1
	`, schema, schema, schema, schema), id).Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt, &row.OrderNos, &row.BomVersionID, &row.OperationTemplateID, &row.ProcessTemplateID, &row.ProcessTemplateName, &row.ProcessSnapshotJSON, &row.OperationSummaryJSON, &row.CustomerID, &row.TargetWarehouse, &row.ProcessingRequestItemID, &row.WIPReservedG, &row.WIPConsumedG, &row.WIPRemainingReservedG)
	if err == pgx.ErrNoRows {
		return productionapp.WorkOrderRow{}, fmt.Errorf("work order not found")
	}
	return row, err
}

func loadBatchCostForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) (productionapp.BatchCostRow, error) {
	var row productionapp.BatchCostRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,running_item_id,batch_id,product_name,COALESCE(material_cost,0)::float8,COALESCE(operation_cost,0)::float8,COALESCE(total_cost,0)::float8,finished_g,COALESCE(unit_cost_per_kg,0)::float8,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.production_batch_costs
		WHERE running_item_id=$1
	`, schema), runningItemID).Scan(&row.ID, &row.RunningItemID, &row.BatchID, &row.ProductName, &row.MaterialCost, &row.OperationCost, &row.TotalCost, &row.FinishedG, &row.UnitCostPerKG, &row.CreatedAt)
	if err == pgx.ErrNoRows {
		return productionapp.BatchCostRow{RunningItemID: runningItemID}, nil
	}
	return row, err
}

func stockEntryRowFromDetail(detail productionapp.StockEntryDetail) productionapp.StockEntryRow {
	var qtyG int64
	var totalCost float64
	for _, item := range detail.Items {
		qtyG += item.QtyG
		totalCost += item.TotalCost
	}
	return productionapp.StockEntryRow{
		ID:            detail.ID,
		EntryNo:       detail.EntryNo,
		EntryType:     detail.EntryType,
		Purpose:       productionapp.StockEntryPurposeForType(detail.EntryType),
		Status:        detail.Status,
		WorkOrderID:   detail.WorkOrderID,
		JobCardID:     detail.JobCardID,
		RunningItemID: detail.RunningItemID,
		SourceType:    detail.SourceType,
		SourceID:      detail.SourceID,
		ItemCount:     int64(len(detail.Items)),
		TotalQtyG:     qtyG,
		TotalCost:     totalCost,
		Operator:      detail.Operator,
		Note:          detail.Note,
		CreatedAt:     detail.CreatedAt,
	}
}
