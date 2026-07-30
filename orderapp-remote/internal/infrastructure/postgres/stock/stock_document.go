package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	stockapp "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r Repository) CreateStockDocumentDraft(ctx context.Context, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := r.createStockDocumentDraftTx(ctx, tx, cmd)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return detail, nil
}

func (r Repository) UpdateStockDocumentDraft(ctx context.Context, id int64, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.StockDocumentDetail{}, fmt.Errorf("stock document not found")
		}
		return stockapp.StockDocumentDetail{}, err
	}
	if status != "draft" {
		return stockapp.StockDocumentDetail{}, fmt.Errorf("only draft stock document can be edited")
	}
	if err := r.resolveStockDocumentWorkOrderContextTx(ctx, tx, &cmd); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entries
		SET entry_type=$2,purpose=$3,is_return=$4,work_order_id=$5,job_card_id=$6,running_item_id=$7,
		    source_type=$8,source_id=$9,return_source=$10,operator=$11,note=$12,idempotency_key=$13,updated_at=now()
		WHERE id=$1
	`, r.schema), id, cmd.EntryType, cmd.Purpose, cmd.IsReturn, cmd.WorkOrderID, cmd.JobCardID, cmd.RunningItemID,
		cmd.SourceType, cmd.SourceID, cmd.ReturnSource, cmd.Operator, cmd.Note, cmd.IdempotencyKey); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.stock_entry_batch_allocations
		WHERE stock_entry_item_id IN (SELECT id FROM %s.stock_entry_items WHERE stock_entry_id=$1)
	`, r.schema, r.schema), id); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.stock_entry_items WHERE stock_entry_id=$1`, r.schema), id); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := r.insertStockDocumentItemsTx(ctx, tx, id, cmd.Items); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "stock_entry", &id, "update_draft", postgresinfra.StrPtr("purpose"), nil, postgresinfra.StrPtr(cmd.Purpose), postgresinfra.AuditMeta{"work_order_id": cmd.WorkOrderID, "is_return": cmd.IsReturn}); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	detail, err := r.loadStockDocumentDetailTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return detail, nil
}

func (r Repository) SubmitStockDocument(ctx context.Context, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := r.submitStockDocumentTx(ctx, tx, id, actor)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return detail, nil
}

func (r Repository) CreateAndSubmitStockDocument(ctx context.Context, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	draft, err := r.createStockDocumentDraftTx(ctx, tx, cmd)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if draft.Status == "submitted" {
		if err := tx.Commit(ctx); err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
		return draft, nil
	}
	detail, err := r.submitStockDocumentTx(ctx, tx, draft.ID, cmd.Operator)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return detail, nil
}

func (r Repository) CancelStockDocument(ctx context.Context, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	status, err := r.lockStockDocumentStatusTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if status == "cancelled" {
		detail, err := r.loadStockDocumentDetailTx(ctx, tx, id)
		if err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
		return detail, nil
	}
	if status != "submitted" {
		return stockapp.StockDocumentDetail{}, fmt.Errorf("only submitted stock document can be cancelled")
	}
	detail, err := r.loadStockDocumentDetailTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	for i := len(detail.Items) - 1; i >= 0; i-- {
		if err := r.reverseStockDocumentItemTx(ctx, tx, detail, detail.Items[i], actor); err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
	}
	if err := r.updateWorkOrderStockStatsTx(ctx, tx, detail, -1); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entries SET status='cancelled',cancelled_at=now(),updated_at=now(),operator=$2 WHERE id=$1
	`, r.schema), id, actor); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "stock_entry", &id, "cancel", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("submitted"), postgresinfra.StrPtr("cancelled"), postgresinfra.AuditMeta{"entry_no": detail.EntryNo, "purpose": detail.Purpose, "work_order_id": detail.WorkOrderID}); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	result, err := r.loadStockDocumentDetailTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return result, nil
}

func (r Repository) ListStockDocuments(ctx context.Context, query stockapp.StockDocumentQuery) (stockapp.StockDocumentResult, error) {
	where := []string{"1=1"}
	args := []any{}
	add := func(expr string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(expr, len(args)))
	}
	if query.Q != "" {
		add("(d.entry_no ILIKE $%d OR d.note ILIKE $%d)", "%"+query.Q+"%")
	}
	if query.Purpose != "" {
		add("d.purpose=$%d", query.Purpose)
	}
	if query.Status != "" {
		add("d.status=$%d", query.Status)
	}
	if query.WorkOrderID > 0 {
		add("d.work_order_id=$%d", query.WorkOrderID)
	}
	if query.JobCardID > 0 {
		add("d.job_card_id=$%d", query.JobCardID)
	}
	documentsCTE := fmt.Sprintf(`
		WITH documents AS (
			SELECT se.id,se.entry_no,se.entry_type,se.purpose,se.is_return,se.status,
			       se.work_order_id,COALESCE(wo.work_order_no,'') AS work_order_no,se.job_card_id,se.running_item_id,se.source_type,se.source_id,se.return_source,
			       COUNT(si.id)::bigint AS item_count,COALESCE(SUM(si.qty_g),0)::bigint AS total_qty_g,
			       COALESCE(SUM(si.qty_units),0)::bigint AS total_qty_units,
			       COALESCE(SUM(si.total_cost),0)::float8 AS total_cost,se.operator,se.note,se.legacy,
			       se.created_at,se.updated_at
			FROM %[1]s.stock_entries se
			LEFT JOIN %[1]s.stock_entry_items si ON si.stock_entry_id=se.id
			LEFT JOIN %[1]s.work_orders wo ON wo.id=se.work_order_id
			GROUP BY se.id,wo.work_order_no

			UNION ALL

			SELECT -(1000000000000::bigint+mr.id),
			       concat('MR-HIST-',lpad(mr.id::text,10,'0')),
			       'material_receipt','material_receipt',false,mr.status,
			       0,'',0,0,'material_receipt',mr.id,'',
			       1,mr.qty_g,mr.qty_units,
			       (CASE WHEN mr.qty_g>0 THEN mr.qty_g::numeric/1000 ELSE mr.qty_units::numeric END*mr.unit_cost)::float8,
			       mr.operator,mr.note,true,mr.created_at,mr.created_at
			FROM %[1]s.material_receipts mr

			UNION ALL

			SELECT -(2000000000000::bigint+mt.id),mt.transfer_no,
			       'material_transfer','material_transfer',false,mt.status,
			       0,'',0,0,'material_transfer',mt.id,'',
			       1,mt.qty_g,0::bigint,0::float8,mt.operator,mt.note,true,mt.created_at,mt.created_at
			FROM %[1]s.material_transfers mt

			UNION ALL

			SELECT -(3000000000000::bigint+ft.id),ft.transfer_no,
			       'finished_transfer','material_transfer',false,ft.status,
			       0,'',0,0,'finished_product_transfer',ft.id,'',
			       1,ft.qty_g,ft.qty_units,0::float8,ft.operator,ft.note,true,ft.created_at,ft.created_at
			FROM %[1]s.finished_product_transfers ft
		)
	`, r.schema)
	var total int
	if err := r.pool.QueryRow(ctx, documentsCTE+fmt.Sprintf(`SELECT count(*)::int FROM documents d WHERE %s`, strings.Join(where, " AND ")), args...).Scan(&total); err != nil {
		return stockapp.StockDocumentResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	rows, err := r.pool.Query(ctx, documentsCTE+fmt.Sprintf(`
		SELECT d.id,d.entry_no,d.entry_type,d.purpose,d.is_return,d.status,d.work_order_id,d.work_order_no,d.job_card_id,d.running_item_id,
		       d.source_type,d.source_id,d.return_source,d.item_count,d.total_qty_g,d.total_qty_units,d.total_cost,d.operator,d.note,d.legacy,
		       to_char(d.created_at,'YYYY-MM-DD HH24:MI'),to_char(d.updated_at,'YYYY-MM-DD HH24:MI')
		FROM documents d
		WHERE %s
		ORDER BY d.created_at DESC,d.id DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), len(args)-1, len(args)), args...)
	if err != nil {
		return stockapp.StockDocumentResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.StockDocumentRow, 0)
	for rows.Next() {
		var row stockapp.StockDocumentRow
		if err := rows.Scan(&row.ID, &row.EntryNo, &row.EntryType, &row.Purpose, &row.IsReturn, &row.Status, &row.WorkOrderID, &row.WorkOrderNo, &row.JobCardID, &row.RunningItemID,
			&row.SourceType, &row.SourceID, &row.ReturnSource, &row.ItemCount, &row.TotalQtyG, &row.TotalQtyUnits, &row.TotalCost, &row.Operator, &row.Note, &row.Legacy, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return stockapp.StockDocumentResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.StockDocumentResult{}, err
	}
	hasNext := len(out) > query.Limit
	if hasNext {
		out = out[:query.Limit]
	}
	return stockapp.StockDocumentResult{Rows: out, Total: total, HasNext: hasNext, Limit: query.Limit, Offset: query.Offset}, nil
}

func (r Repository) GetStockDocument(ctx context.Context, id int64) (stockapp.StockDocumentDetail, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	detail, err := r.loadStockDocumentDetailTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return detail, nil
}

func (r Repository) createStockDocumentDraftTx(ctx context.Context, tx pgx.Tx, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	if cmd.IdempotencyKey != "" {
		var id int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.stock_entries WHERE idempotency_key=$1 FOR UPDATE`, r.schema), cmd.IdempotencyKey).Scan(&id)
		if err == nil {
			return r.loadStockDocumentDetailTx(ctx, tx, id)
		}
		if err != pgx.ErrNoRows {
			return stockapp.StockDocumentDetail{}, err
		}
	}
	if err := r.resolveStockDocumentWorkOrderContextTx(ctx, tx, &cmd); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entries(
			entry_no,entry_type,purpose,is_return,status,work_order_id,job_card_id,running_item_id,
			source_type,source_id,return_source,operator,note,idempotency_key,legacy,created_at,updated_at
		) VALUES(concat('SE-TMP-',txid_current(),'-',floor(extract(epoch from clock_timestamp())*1000000)::bigint),$1,$2,$3,'draft',$4,$5,$6,$7,$8,$9,$10,$11,$12,false,now(),now())
		RETURNING id
	`, r.schema), cmd.EntryType, cmd.Purpose, cmd.IsReturn, cmd.WorkOrderID, cmd.JobCardID, cmd.RunningItemID,
		cmd.SourceType, cmd.SourceID, cmd.ReturnSource, cmd.Operator, cmd.Note, cmd.IdempotencyKey).Scan(&id); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	entryNo := fmt.Sprintf("SE-%010d", id)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entries SET entry_no=$2 WHERE id=$1`, r.schema), id, entryNo); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := r.insertStockDocumentItemsTx(ctx, tx, id, cmd.Items); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "stock_entry", &id, "create_draft", postgresinfra.StrPtr("purpose"), nil, postgresinfra.StrPtr(cmd.Purpose), postgresinfra.AuditMeta{"entry_no": entryNo, "work_order_id": cmd.WorkOrderID, "is_return": cmd.IsReturn}); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return r.loadStockDocumentDetailTx(ctx, tx, id)
}

func (r Repository) resolveStockDocumentWorkOrderContextTx(ctx context.Context, tx pgx.Tx, cmd *stockapp.StockDocumentCommand) error {
	if cmd == nil || cmd.WorkOrderID <= 0 {
		return nil
	}
	switch cmd.Purpose {
	case stockapp.PurposeMaterialTransferForManufacture, stockapp.PurposeMaterialConsumption, stockapp.PurposeManufacture, stockapp.PurposeMaterialIssue:
	default:
		return fmt.Errorf("stock document purpose cannot be linked to work order")
	}
	var runningItemID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(running_item_id,0) FROM %s.work_orders WHERE id=$1`, r.schema), cmd.WorkOrderID).Scan(&runningItemID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("work order not found")
		}
		return err
	}
	cmd.RunningItemID = runningItemID
	if cmd.JobCardID > 0 {
		var belongs bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.job_cards WHERE id=$1 AND work_order_id=$2)`, r.schema), cmd.JobCardID, cmd.WorkOrderID).Scan(&belongs); err != nil {
			return err
		}
		if !belongs {
			return fmt.Errorf("job card does not belong to work order")
		}
	}
	if cmd.SourceType == "" {
		cmd.SourceType = "work_order"
	}
	if cmd.SourceID <= 0 {
		cmd.SourceID = cmd.WorkOrderID
	}
	return nil
}

func (r Repository) insertStockDocumentItemsTx(ctx context.Context, tx pgx.Tx, stockEntryID int64, items []stockapp.StockDocumentItemCommand) error {
	for _, item := range items {
		totalCost := float64(item.QtyUnits) * item.UnitCost
		if item.QtyG > 0 {
			totalCost = float64(item.QtyG) / 1000 * item.UnitCost
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_entry_items(
				stock_entry_id,material_id,product_id,item_type,item_name,spec_g,inventory_unit,
				from_warehouse,to_warehouse,qty_g,qty_units,batch_code,unit_cost,total_cost,
				supplier,crop_season,origin,producer_flavor_description
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		`, r.schema), stockEntryID, item.MaterialID, item.ProductID, item.ItemType, item.ItemName, item.SpecG, item.InventoryUnit,
			item.FromWarehouse, item.ToWarehouse, item.QtyG, item.QtyUnits, item.BatchCode, item.UnitCost, totalCost,
			item.Supplier, item.CropSeason, item.Origin, item.ProducerFlavorDescription); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) submitStockDocumentTx(ctx context.Context, tx pgx.Tx, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	status, err := r.lockStockDocumentStatusTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if status == "submitted" {
		return r.loadStockDocumentDetailTx(ctx, tx, id)
	}
	if status != "draft" {
		return stockapp.StockDocumentDetail{}, fmt.Errorf("only draft stock document can be submitted")
	}
	detail, err := r.loadStockDocumentDetailTx(ctx, tx, id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := r.validateStockDocumentWorkOrderTx(ctx, tx, detail); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	for i := range detail.Items {
		if err := r.postStockDocumentItemTx(ctx, tx, detail, &detail.Items[i], actor); err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
	}
	if err := r.updateWorkOrderStockStatsTx(ctx, tx, detail, 1); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entries SET status='submitted',submitted_at=now(),updated_at=now(),operator=$2 WHERE id=$1
	`, r.schema), id, actor); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "stock_entry", &id, "submit", postgresinfra.StrPtr("status"), postgresinfra.StrPtr("draft"), postgresinfra.StrPtr("submitted"), postgresinfra.AuditMeta{"entry_no": detail.EntryNo, "purpose": detail.Purpose, "is_return": detail.IsReturn, "work_order_id": detail.WorkOrderID}); err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return r.loadStockDocumentDetailTx(ctx, tx, id)
}

func (r Repository) lockStockDocumentStatusTx(ctx context.Context, tx pgx.Tx, id int64) (string, error) {
	var status string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.stock_entries WHERE id=$1 FOR UPDATE`, r.schema), id).Scan(&status); err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("stock document not found")
		}
		return "", err
	}
	return status, nil
}

func (r Repository) loadStockDocumentDetailTx(ctx context.Context, tx pgx.Tx, id int64) (stockapp.StockDocumentDetail, error) {
	var out stockapp.StockDocumentDetail
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT se.id,se.entry_no,se.entry_type,se.purpose,se.is_return,se.status,se.work_order_id,COALESCE(wo.work_order_no,''),se.job_card_id,se.running_item_id,
		       se.source_type,se.source_id,se.return_source,se.operator,se.note,se.legacy,
		       to_char(se.created_at,'YYYY-MM-DD HH24:MI'),to_char(se.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_entries se
		LEFT JOIN %s.work_orders wo ON wo.id=se.work_order_id
		WHERE se.id=$1
	`, r.schema, r.schema), id).Scan(&out.ID, &out.EntryNo, &out.EntryType, &out.Purpose, &out.IsReturn, &out.Status, &out.WorkOrderID, &out.WorkOrderNo, &out.JobCardID, &out.RunningItemID,
		&out.SourceType, &out.SourceID, &out.ReturnSource, &out.Operator, &out.Note, &out.Legacy, &out.CreatedAt, &out.UpdatedAt)
	if err == pgx.ErrNoRows {
		return stockapp.StockDocumentDetail{}, fmt.Errorf("stock document not found")
	}
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,stock_entry_id,material_id,product_id,item_type,item_name,spec_g,inventory_unit,
		       from_warehouse,to_warehouse,qty_g,qty_units,batch_code,COALESCE(unit_cost,0)::float8,
		       COALESCE(total_cost,0)::float8,supplier,crop_season,origin,producer_flavor_description
		FROM %s.stock_entry_items WHERE stock_entry_id=$1 ORDER BY id
	`, r.schema), id)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	out.Items = make([]stockapp.StockDocumentItemRow, 0)
	for rows.Next() {
		var item stockapp.StockDocumentItemRow
		if err := rows.Scan(&item.ID, &item.StockEntryID, &item.MaterialID, &item.ProductID, &item.ItemType, &item.ItemName, &item.SpecG, &item.InventoryUnit,
			&item.FromWarehouse, &item.ToWarehouse, &item.QtyG, &item.QtyUnits, &item.BatchCode, &item.UnitCost, &item.TotalCost,
			&item.Supplier, &item.CropSeason, &item.Origin, &item.ProducerFlavorDescription); err != nil {
			rows.Close()
			return stockapp.StockDocumentDetail{}, err
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return stockapp.StockDocumentDetail{}, err
	}
	rows.Close()
	for i := range out.Items {
		allocRows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT material_batch_id,batch_code,qty_g,qty_units,COALESCE(unit_cost,0)::float8
			FROM %s.stock_entry_batch_allocations WHERE stock_entry_item_id=$1 ORDER BY id
		`, r.schema), out.Items[i].ID)
		if err != nil {
			return stockapp.StockDocumentDetail{}, err
		}
		out.Items[i].Allocations = make([]stockapp.StockDocumentBatchAllocation, 0)
		for allocRows.Next() {
			var alloc stockapp.StockDocumentBatchAllocation
			if err := allocRows.Scan(&alloc.MaterialBatchID, &alloc.BatchCode, &alloc.QtyG, &alloc.QtyUnits, &alloc.UnitCost); err != nil {
				allocRows.Close()
				return stockapp.StockDocumentDetail{}, err
			}
			out.Items[i].Allocations = append(out.Items[i].Allocations, alloc)
		}
		if err := allocRows.Err(); err != nil {
			allocRows.Close()
			return stockapp.StockDocumentDetail{}, err
		}
		allocRows.Close()
	}
	out.ItemCount = int64(len(out.Items))
	for _, item := range out.Items {
		out.TotalQtyG += item.QtyG
		out.TotalQtyUnits += item.QtyUnits
		out.TotalCost += item.TotalCost
	}
	return out, nil
}

func (r Repository) validateStockDocumentWorkOrderTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail) error {
	requiresWorkOrder := detail.Purpose == stockapp.PurposeMaterialTransferForManufacture ||
		detail.Purpose == stockapp.PurposeMaterialConsumption ||
		detail.Purpose == stockapp.PurposeManufacture
	if detail.WorkOrderID <= 0 {
		if requiresWorkOrder {
			return fmt.Errorf("work_order_id required")
		}
		return nil
	}
	if !requiresWorkOrder && detail.Purpose != stockapp.PurposeMaterialIssue {
		return fmt.Errorf("stock document purpose cannot be linked to work order")
	}
	expectedItemType := itemTypeMaterial
	if detail.Purpose == stockapp.PurposeManufacture {
		expectedItemType = itemTypeFinishedProduct
	}
	for index, item := range detail.Items {
		if item.ItemType != expectedItemType {
			return fmt.Errorf("item %d item type must be %s for work order purpose %s", index+1, expectedItemType, detail.Purpose)
		}
	}
	var status string
	var plannedG, plannedOutputG, plannedUnits int64
	var materialSnapshot string
	hasSnapshot, err := stockSchemaColumnExistsTx(ctx, tx, r.schema, "work_orders", "material_snapshot")
	if err != nil {
		return err
	}
	if hasSnapshot {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT status,COALESCE(planned_g,0),COALESCE(planned_output_g,0),CEIL(COALESCE(sales_spec_count,0))::bigint,
			       COALESCE(material_snapshot,'[]'::jsonb)::text
			FROM %s.work_orders WHERE id=$1 FOR UPDATE
		`, r.schema), detail.WorkOrderID).Scan(&status, &plannedG, &plannedOutputG, &plannedUnits, &materialSnapshot)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), detail.WorkOrderID).Scan(&status)
	}
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("work order not found")
		}
		return err
	}
	if status == "cancelled" || status == "completed" {
		return fmt.Errorf("work order is not open")
	}
	if detail.Purpose == stockapp.PurposeManufacture {
		var incompleteJobCards int
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT count(*)::int
			FROM %s.job_cards
			WHERE work_order_id=$1 AND status NOT IN ('completed','cancelled')
		`, r.schema), detail.WorkOrderID).Scan(&incompleteJobCards); err != nil {
			return err
		}
		if incompleteJobCards > 0 {
			return fmt.Errorf("work order has incomplete job cards")
		}
		var remainingG, remainingUnits int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,
			       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint
			FROM %s.work_order_material_reservations
			WHERE work_order_id=$1
		`, r.schema), detail.WorkOrderID).Scan(&remainingG, &remainingUnits); err != nil {
			return err
		}
		if remainingG > 0 || remainingUnits > 0 {
			return fmt.Errorf("work order still has unconsumed or unreturned material")
		}
	}
	frozenRequirements, err := stockFrozenMaterialRequirements(materialSnapshot, plannedG, plannedOutputG, plannedUnits)
	if err != nil {
		return err
	}
	type validatedMaterialRequirement struct {
		name        string
		requirement stockFrozenMaterialRequirement
		currentG    int64
		currentUnit int64
	}
	validatedRequirements := make(map[int64]validatedMaterialRequirement)
	validatedMaterialIDs := make([]int64, 0, len(detail.Items))
	for _, item := range detail.Items {
		if item.MaterialID <= 0 {
			continue
		}
		validated, alreadyValidated := validatedRequirements[item.MaterialID]
		if !alreadyValidated {
			validatedMaterialIDs = append(validatedMaterialIDs, item.MaterialID)
			reservationRequirement, reservationMember, err := stockReservationMaterialRequirementTx(
				ctx, tx, r.schema, detail.WorkOrderID, item.MaterialID,
			)
			if err != nil {
				return err
			}
			requirement, frozenMember := frozenRequirements[item.MaterialID]
			if !frozenMember && reservationMember {
				requirement = reservationRequirement
			}
			requirementMember := frozenMember || reservationMember
			if !requirementMember {
				return fmt.Errorf("material does not belong to work order")
			}
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(name,'') FROM %s.materials WHERE id=$1
			`, r.schema), item.MaterialID).Scan(&validated.name); err != nil {
				if err == pgx.ErrNoRows {
					return fmt.Errorf("material not found")
				}
				return err
			}
			validated.requirement = requirement
		}
		requirement := validated.requirement
		if item.InventoryUnit != "" && requirement.InventoryUnit != "" && !sameInventoryUnit(item.InventoryUnit, requirement.InventoryUnit) {
			return fmt.Errorf("material inventory unit does not match frozen work order requirement")
		}
		if requirement.RequiredG > 0 && requirement.RequiredUnits <= 0 && item.QtyUnits > 0 {
			return fmt.Errorf("物料“%s”的工单需求按重量计量，请填写重量数量", validated.name)
		}
		if requirement.RequiredUnits > 0 && requirement.RequiredG <= 0 && item.QtyG > 0 {
			return fmt.Errorf("物料“%s”的工单需求按计数计量，请填写计数数量", validated.name)
		}
		validated.currentG += item.QtyG
		validated.currentUnit += item.QtyUnits
		validatedRequirements[item.MaterialID] = validated
	}
	if detail.Purpose == stockapp.PurposeMaterialConsumption {
		consumptionErrors := make([]string, 0)
		for _, materialID := range validatedMaterialIDs {
			validated := validatedRequirements[materialID]
			var submittedConsumedG, submittedConsumedUnits int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(i.qty_g),0)::bigint,COALESCE(SUM(i.qty_units),0)::bigint
				FROM %s.stock_entry_items i
				JOIN %s.stock_entries e ON e.id=i.stock_entry_id
				WHERE e.work_order_id=$1
				  AND e.purpose=$2
				  AND e.status='submitted'
				  AND i.material_id=$3
			`, r.schema, r.schema), detail.WorkOrderID, stockapp.PurposeMaterialConsumption, materialID).Scan(&submittedConsumedG, &submittedConsumedUnits); err != nil {
				return err
			}
			var reservationConsumedG, reservationConsumedUnits int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(SUM(consumed_g),0)::bigint,COALESCE(SUM(consumed_units),0)::bigint
				FROM %s.work_order_material_reservations
				WHERE work_order_id=$1 AND material_id=$2
			`, r.schema), detail.WorkOrderID, materialID).Scan(&reservationConsumedG, &reservationConsumedUnits); err != nil {
				return err
			}
			// Unified Stock Entry posting also updates reservation consumption.
			// Take the larger projection so historical reservation-only usage is
			// honored without counting newly unified consumption twice.
			consumedG := maxInt64(submittedConsumedG, reservationConsumedG)
			consumedUnits := maxInt64(submittedConsumedUnits, reservationConsumedUnits)
			remainingG := maxInt64(0, validated.requirement.RequiredG-consumedG)
			remainingUnits := maxInt64(0, validated.requirement.RequiredUnits-consumedUnits)
			if validated.currentG > remainingG || validated.currentUnit > remainingUnits {
				currentLabel := stockQuantityLabel(validated.currentG, validated.currentUnit, validated.requirement.InventoryUnit)
				remainingLabel := stockQuantityLabel(remainingG, remainingUnits, validated.requirement.InventoryUnit)
				consumptionErrors = append(consumptionErrors, fmt.Sprintf(
					"%s，本次消耗%s，剩余可消耗%s",
					validated.name, currentLabel, remainingLabel,
				))
			}
		}
		if len(consumptionErrors) > 0 {
			return fmt.Errorf("生产消耗数量超过工单剩余需求：%s", strings.Join(consumptionErrors, "；"))
		}
	}
	return nil
}

func stockReservationMaterialRequirementTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID, materialID int64) (stockFrozenMaterialRequirement, bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s.work_order_material_reservations
			WHERE work_order_id=$1 AND material_id=$2
		)
	`, schema), workOrderID, materialID).Scan(&exists); err != nil || !exists {
		return stockFrozenMaterialRequirement{}, exists, err
	}
	hasRequiredG, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "required_g")
	if err != nil {
		return stockFrozenMaterialRequirement{}, false, err
	}
	hasRequiredUnits, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "required_units")
	if err != nil {
		return stockFrozenMaterialRequirement{}, false, err
	}
	hasUnit, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "unit")
	if err != nil {
		return stockFrozenMaterialRequirement{}, false, err
	}
	requiredGExpr := "COALESCE(SUM(reserved_g),0)::bigint"
	if hasRequiredG {
		requiredGExpr = "COALESCE(SUM(CASE WHEN required_g>0 THEN required_g ELSE reserved_g END),0)::bigint"
	}
	requiredUnitsExpr := "COALESCE(SUM(reserved_units),0)::bigint"
	if hasRequiredUnits {
		requiredUnitsExpr = "COALESCE(SUM(CASE WHEN required_units>0 THEN required_units ELSE reserved_units END),0)::bigint"
	}
	var requirement stockFrozenMaterialRequirement
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s,%s
		FROM %s.work_order_material_reservations
		WHERE work_order_id=$1 AND material_id=$2
	`, requiredGExpr, requiredUnitsExpr, schema), workOrderID, materialID).Scan(&requirement.RequiredG, &requirement.RequiredUnits); err != nil {
		return stockFrozenMaterialRequirement{}, false, err
	}
	if hasUnit {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(NULLIF(MAX(unit),''),'')
			FROM %s.work_order_material_reservations
			WHERE work_order_id=$1 AND material_id=$2
		`, schema), workOrderID, materialID).Scan(&requirement.InventoryUnit); err != nil {
			return stockFrozenMaterialRequirement{}, false, err
		}
	}
	if strings.TrimSpace(requirement.InventoryUnit) == "" {
		if requirement.RequiredUnits > 0 && requirement.RequiredG <= 0 {
			requirement.InventoryUnit = "个"
		} else {
			requirement.InventoryUnit = "g"
		}
	}
	return requirement, true, nil
}

type stockFrozenMaterialRequirement struct {
	InventoryUnit string
	RequiredG     int64
	RequiredUnits int64
}

type stockFrozenMaterialSnapshotRow struct {
	MaterialID                int64   `json:"material_id"`
	Unit                      string  `json:"unit"`
	RatioPct                  float64 `json:"ratio_pct"`
	MaterialLossRate          float64 `json:"material_loss_rate"`
	InputIncludesMaterialLoss bool    `json:"input_includes_material_loss"`
	Source                    string  `json:"source"`
	ConsumeUnit               string  `json:"consume_unit"`
	QtyPerUnit                float64 `json:"qty_per_unit"`
	OutputQty                 float64 `json:"output_qty"`
	OutputUnit                string  `json:"output_unit"`
}

func stockFrozenMaterialRequirements(raw string, plannedG, plannedOutputG, plannedUnits int64) (map[int64]stockFrozenMaterialRequirement, error) {
	out := map[int64]stockFrozenMaterialRequirement{}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return out, nil
	}
	var rows []stockFrozenMaterialSnapshotRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, fmt.Errorf("invalid frozen work order material snapshot: %w", err)
	}
	if plannedOutputG <= 0 {
		plannedOutputG = plannedG
	}
	for _, row := range rows {
		if row.MaterialID <= 0 {
			continue
		}
		unit := strings.TrimSpace(row.Unit)
		if unit == "" {
			unit = "g"
		}
		weightFactor := stockWeightUnitGrams(unit)
		requiredG, requiredUnits := int64(0), int64(0)
		source := strings.ToLower(strings.TrimSpace(row.Source))
		consumeUnit := strings.ToLower(strings.TrimSpace(row.ConsumeUnit))
		if source == "packaging" {
			requiredUnits = plannedUnits
		} else if consumeUnit == "" || consumeUnit == "ratio_pct" {
			ratio := row.RatioPct
			if ratio <= 0 && weightFactor == 0 {
				ratio = 100
			}
			base := float64(plannedG) * ratio / 100
			loss := row.MaterialLossRate
			if loss > 1 {
				loss /= 100
			}
			if row.InputIncludesMaterialLoss {
				loss = 0
			}
			if loss > 0 && loss < 1 {
				base /= 1 - loss
			}
			if weightFactor > 0 {
				requiredG = int64(math.Ceil(base))
			} else {
				requiredUnits = int64(math.Ceil(base))
			}
		} else {
			outputQty := row.OutputQty
			if outputQty <= 0 {
				outputQty = 1
			}
			outputFactor := float64(plannedOutputG)
			switch strings.ToLower(strings.TrimSpace(row.OutputUnit)) {
			case "kg", "千克", "公斤":
				outputFactor /= 1000
			case "lb", "磅":
				outputFactor /= 453.59237
			}
			outputFactor /= outputQty
			if weightFactor > 0 {
				grams := row.QtyPerUnit * outputFactor * weightFactor
				switch consumeUnit {
				case "g":
					grams = row.QtyPerUnit * outputFactor
				case "kg":
					grams = row.QtyPerUnit * outputFactor * 1000
				case "g_per_bag":
					grams = row.QtyPerUnit * float64(plannedUnits)
				case "unit_per_bag":
					grams = row.QtyPerUnit * float64(plannedUnits) * weightFactor
				case "unit_per_box":
					grams = 0
				}
				requiredG = int64(math.Ceil(grams))
			} else {
				qty := row.QtyPerUnit * outputFactor
				switch consumeUnit {
				case "unit_per_bag", "g_per_bag":
					qty = row.QtyPerUnit * float64(plannedUnits)
				case "unit_per_box":
					qty = 0
				}
				requiredUnits = int64(math.Ceil(qty))
			}
		}
		current := out[row.MaterialID]
		current.InventoryUnit = unit
		current.RequiredG += requiredG
		current.RequiredUnits += requiredUnits
		out[row.MaterialID] = current
	}
	return out, nil
}

func stockWeightUnitGrams(unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克":
		return 1
	case "kg", "千克", "公斤":
		return 1000
	case "lb", "磅":
		return 453.59237
	default:
		return 0
	}
}

func stockQuantityLabel(qtyG, qtyUnits int64, unit string) string {
	unit = strings.TrimSpace(unit)
	if qtyUnits > 0 && qtyG <= 0 {
		if unit == "" {
			unit = "件"
		}
		return strconv.FormatInt(qtyUnits, 10) + unit
	}
	if factor := stockWeightUnitGrams(unit); factor > 0 {
		return strconv.FormatFloat(float64(qtyG)/factor, 'f', -1, 64) + unit
	}
	if unit == "" {
		unit = "g"
	}
	if qtyG > 0 || qtyUnits <= 0 {
		return strconv.FormatInt(qtyG, 10) + unit
	}
	return strconv.FormatInt(qtyUnits, 10) + unit
}

func stockWarehouseLabel(code string) string {
	switch strings.TrimSpace(code) {
	case "raw_materials":
		return "原料仓"
	case "wip":
		return "WIP在制仓"
	case "finished_goods":
		return "成品仓"
	default:
		if strings.TrimSpace(code) == "" {
			return "来源仓"
		}
		return code
	}
}

func stockMaterialDisplayNameTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, fallback string) (string, error) {
	name := strings.TrimSpace(fallback)
	var storedName string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.materials WHERE id=$1`, schema), materialID).Scan(&storedName)
	if err != nil && err != pgx.ErrNoRows {
		return "", err
	}
	if strings.TrimSpace(storedName) != "" {
		name = strings.TrimSpace(storedName)
	}
	if name == "" {
		name = fmt.Sprintf("物料#%d", materialID)
	}
	return name, nil
}

func sameInventoryUnit(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return true
	}
	aliases := map[string]string{"克": "g", "千克": "kg", "公斤": "kg", "磅": "lb"}
	if alias := aliases[a]; alias != "" {
		a = alias
	}
	if alias := aliases[b]; alias != "" {
		b = alias
	}
	return a == b
}

func eligibleWIPBalanceForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID, materialID int64) (int64, int64, int64, int64, error) {
	var wipG, wipUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(l.qty_g),0)::bigint,COALESCE(SUM(l.qty_units),0)::bigint
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1 AND l.warehouse='wip'
		  AND (l.qty_g>0 OR l.qty_units>0)
		  AND b.status='active'
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
	`, schema, schema), materialID).Scan(&wipG, &wipUnits); err != nil {
		return 0, 0, 0, 0, err
	}
	var reservedG, reservedUnits int64
	hasWorkOrderStatus, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_orders", "status")
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if hasWorkOrderStatus {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint
			FROM %s.work_order_material_reservations r
			JOIN %s.work_orders wo ON wo.id=r.work_order_id
			WHERE r.material_id=$1 AND r.status='reserved' AND r.work_order_id<>$2
			  AND wo.status IN ('released','running','partially_completed','paused')
		`, schema, schema), materialID, workOrderID).Scan(&reservedG, &reservedUnits)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint,
			       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units)),0)::bigint
			FROM %s.work_order_material_reservations
			WHERE material_id=$1 AND status='reserved' AND work_order_id<>$2
		`, schema), materialID, workOrderID).Scan(&reservedG, &reservedUnits)
	}
	if err != nil {
		return 0, 0, 0, 0, err
	}
	var consumedG, consumedUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(consumed_g),0)::bigint,COALESCE(SUM(consumed_units),0)::bigint
		FROM %s.work_order_material_reservations
		WHERE work_order_id=$1 AND material_id=$2
	`, schema), workOrderID, materialID).Scan(&consumedG, &consumedUnits); err != nil {
		return 0, 0, 0, 0, err
	}
	return nonnegativeStockQty(wipG - reservedG), nonnegativeStockQty(wipUnits - reservedUnits), consumedG, consumedUnits, nil
}

func nonnegativeStockQty(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func stockSchemaColumnExistsTx(ctx context.Context, tx pgx.Tx, schema, table, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
}

func (r Repository) postStockDocumentItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item *stockapp.StockDocumentItemRow, actor string) error {
	for _, warehouse := range []string{item.FromWarehouse, item.ToWarehouse} {
		if warehouse == "" {
			continue
		}
		if err := r.ensureWarehouseExistsTx(ctx, tx, warehouse); err != nil {
			return err
		}
	}
	if item.ItemType == itemTypeMaterial {
		if detail.Purpose == stockapp.PurposeMaterialReceipt {
			return r.postMaterialReceiptItemTx(ctx, tx, detail, item, actor)
		}
		return r.postMaterialMovementItemTx(ctx, tx, detail, item, actor)
	}
	return r.postFinishedItemTx(ctx, tx, detail, item, actor)
}

func (r Repository) postMaterialReceiptItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item *stockapp.StockDocumentItemRow, actor string) error {
	var materialName, unit string
	var onhandG, onhandUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),COALESCE(unit,''),onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), item.MaterialID).
		Scan(&materialName, &unit, &onhandG, &onhandUnits); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("material not found")
		}
		return err
	}
	beforeG, beforeUnits, err := materialWarehouseBalanceTx(ctx, tx, r.schema, item.MaterialID, item.ToWarehouse)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, r.schema),
		item.MaterialID, onhandG+item.QtyG, onhandUnits+item.QtyUnits); err != nil {
		return err
	}
	batchCode := fmt.Sprintf("MB-SE-%010d-%03d", detail.ID, item.ID%1000)
	var batchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(
			batch_code,material_id,material_name,supplier,receipt_id,qty_g,qty_units,received_g,
			remaining_g,remaining_units,unit_cost,crop_season,origin,producer_flavor_description,
			status,quality_status,note,received_at,created_at
		) VALUES($1,$2,$3,$4,0,$5,$6,$5,$5,$6,$7,$8,$9,$10,'active','unchecked',$11,now(),now())
		RETURNING id
	`, r.schema), batchCode, item.MaterialID, materialName, item.Supplier, item.QtyG, item.QtyUnits, item.UnitCost,
		item.CropSeason, item.Origin, item.ProducerFlavorDescription, detail.Note).Scan(&batchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
	`, r.schema), batchID, batchCode, item.MaterialID, item.ToWarehouse, item.QtyG, item.QtyUnits); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,unit_cost,operator,created_at
		) VALUES($1,$2,$3,$4,0,'stock_entry_item',$5,$6,$7,$8,$7,$8,$9,$10,now())
	`, r.schema), batchCode, itemTypeMaterial, item.MaterialID, materialName, item.ID, detail.EntryNo, item.QtyG, item.QtyUnits, item.UnitCost, actor); err != nil {
		return err
	}
	if err := r.insertStockDocumentAllocationTx(ctx, tx, item.ID, stockapp.StockDocumentBatchAllocation{MaterialBatchID: batchID, BatchCode: batchCode, QtyG: item.QtyG, QtyUnits: item.QtyUnits, UnitCost: item.UnitCost}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entry_items SET item_name=$2,inventory_unit=$3,batch_code=$4,total_cost=$5 WHERE id=$1
	`, r.schema), item.ID, materialName, unit, batchCode, stockItemTotalCost(item.QtyG, item.QtyUnits, item.UnitCost)); err != nil {
		return err
	}
	return insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: materialName, Warehouse: item.ToWarehouse,
		SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchCode: batchCode, SourceBatchID: detail.EntryNo,
		BeforeG: beforeG, ChangeG: item.QtyG, AfterG: beforeG + item.QtyG,
		BeforeUnits: beforeUnits, ChangeUnits: item.QtyUnits, AfterUnits: beforeUnits + item.QtyUnits, Operator: actor,
	})
}

type materialMoveAvailability struct {
	BatchID      int64
	BatchCode    string
	AvailableG   int64
	AvailableQty int64
	UnitCost     float64
	Quality      string
}

func (r Repository) postMaterialMovementItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item *stockapp.StockDocumentItemRow, actor string) error {
	var materialName, unit string
	var onhandG, onhandUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),COALESCE(unit,''),onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), item.MaterialID).
		Scan(&materialName, &unit, &onhandG, &onhandUnits); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("material not found")
		}
		return err
	}
	available, blockedG, blockedUnits, err := r.materialMoveAvailabilityTx(ctx, tx, detail, *item)
	if err != nil {
		return err
	}
	requiredG, requiredUnits := item.QtyG, item.QtyUnits
	availableG, availableUnits := int64(0), int64(0)
	for _, row := range available {
		availableG += row.AvailableG
		availableUnits += row.AvailableQty
	}
	allocations := make([]stockapp.StockDocumentBatchAllocation, 0)
	for _, row := range available {
		if requiredG <= 0 && requiredUnits <= 0 {
			break
		}
		takeG := minInt64(requiredG, row.AvailableG)
		takeUnits := minInt64(requiredUnits, row.AvailableQty)
		if takeG <= 0 && takeUnits <= 0 {
			continue
		}
		allocations = append(allocations, stockapp.StockDocumentBatchAllocation{
			MaterialBatchID: row.BatchID, BatchCode: row.BatchCode, QtyG: takeG, QtyUnits: takeUnits, UnitCost: row.UnitCost,
		})
		requiredG -= takeG
		requiredUnits -= takeUnits
	}
	if requiredG > 0 || requiredUnits > 0 {
		warehouseLabel := stockWarehouseLabel(item.FromWarehouse)
		requiredLabel := stockQuantityLabel(item.QtyG, item.QtyUnits, unit)
		availableLabel := stockQuantityLabel(availableG, availableUnits, unit)
		shortageLabel := stockQuantityLabel(
			nonnegativeStockQty(item.QtyG-availableG),
			nonnegativeStockQty(item.QtyUnits-availableUnits),
			unit,
		)
		blockedCanCover := item.QtyG <= availableG+blockedG && item.QtyUnits <= availableUnits+blockedUnits
		if blockedCanCover && (blockedG > 0 || blockedUnits > 0) {
			return fmt.Errorf(
				"%s存在质检冻结且合格库存不足：%s，需领用%s，可用合格库存%s，缺口%s；请先处理质检或更换合格批次",
				warehouseLabel, materialName, requiredLabel, availableLabel, shortageLabel,
			)
		}
		blockedNote := ""
		if blockedG > 0 || blockedUnits > 0 {
			blockedNote = fmt.Sprintf(
				"；另有质检冻结库存%s，解除后仍不足",
				stockQuantityLabel(blockedG, blockedUnits, unit),
			)
		}
		return fmt.Errorf(
			"%s库存不足：%s，需领用%s，可用%s，缺口%s%s",
			warehouseLabel, materialName, requiredLabel, availableLabel, shortageLabel, blockedNote,
		)
	}
	isConsumption := detail.Purpose == stockapp.PurposeMaterialIssue || detail.Purpose == stockapp.PurposeMaterialConsumption
	var weightedCost float64
	for _, alloc := range allocations {
		beforeFromG, beforeFromUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.FromWarehouse)
		if err != nil {
			return err
		}
		if beforeFromG < alloc.QtyG || beforeFromUnits < alloc.QtyUnits {
			return fmt.Errorf(
				"%s库存不足：%s，批次%s需领用%s，当前可用%s",
				stockWarehouseLabel(item.FromWarehouse),
				materialName,
				alloc.BatchCode,
				stockQuantityLabel(alloc.QtyG, alloc.QtyUnits, unit),
				stockQuantityLabel(beforeFromG, beforeFromUnits, unit),
			)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batch_locations SET qty_g=$3,qty_units=$4,updated_at=now()
			WHERE material_batch_id=$1 AND warehouse=$2
		`, r.schema), alloc.MaterialBatchID, item.FromWarehouse, beforeFromG-alloc.QtyG, beforeFromUnits-alloc.QtyUnits); err != nil {
			return err
		}
		if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
			ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: materialName, Warehouse: item.FromWarehouse,
			SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo,
			BeforeG: beforeFromG, ChangeG: -alloc.QtyG, AfterG: beforeFromG - alloc.QtyG,
			BeforeUnits: beforeFromUnits, ChangeUnits: -alloc.QtyUnits, AfterUnits: beforeFromUnits - alloc.QtyUnits, Operator: actor,
		}); err != nil {
			return err
		}
		if item.ToWarehouse != "" {
			beforeToG, beforeToUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.ToWarehouse)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
				VALUES($1,$2,$3,$4,$5,$6,now())
				ON CONFLICT (material_batch_id,warehouse) DO UPDATE SET
					qty_g=material_batch_locations.qty_g+excluded.qty_g,
					qty_units=material_batch_locations.qty_units+excluded.qty_units,
					updated_at=now()
			`, r.schema), alloc.MaterialBatchID, alloc.BatchCode, item.MaterialID, item.ToWarehouse, alloc.QtyG, alloc.QtyUnits); err != nil {
				return err
			}
			if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
				ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: materialName, Warehouse: item.ToWarehouse,
				SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo,
				BeforeG: beforeToG, ChangeG: alloc.QtyG, AfterG: beforeToG + alloc.QtyG,
				BeforeUnits: beforeToUnits, ChangeUnits: alloc.QtyUnits, AfterUnits: beforeToUnits + alloc.QtyUnits, Operator: actor,
			}); err != nil {
				return err
			}
		}
		if isConsumption {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.material_batches
				SET remaining_g=remaining_g-$2,remaining_units=remaining_units-$3,
				    status=CASE WHEN remaining_g-$2<=0 AND remaining_units-$3<=0 THEN 'consumed' ELSE status END
				WHERE id=$1 AND remaining_g >= $2 AND remaining_units >= $3
			`, r.schema), alloc.MaterialBatchID, alloc.QtyG, alloc.QtyUnits); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.stock_batches SET remaining_g=GREATEST(0,remaining_g-$2),remaining_units=GREATEST(0,remaining_units-$3)
				WHERE item_type=$4 AND batch_code=$1
			`, r.schema), alloc.BatchCode, alloc.QtyG, alloc.QtyUnits, itemTypeMaterial); err != nil {
				return err
			}
		}
		if err := r.insertStockDocumentAllocationTx(ctx, tx, item.ID, alloc); err != nil {
			return err
		}
		weightedCost += stockItemTotalCost(alloc.QtyG, alloc.QtyUnits, alloc.UnitCost)
	}
	if isConsumption {
		if onhandG < item.QtyG || onhandUnits < item.QtyUnits {
			return fmt.Errorf("material onhand insufficient")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, r.schema),
			item.MaterialID, onhandG-item.QtyG, onhandUnits-item.QtyUnits); err != nil {
			return err
		}
	}
	batchCode := ""
	if len(allocations) == 1 {
		batchCode = allocations[0].BatchCode
	}
	unitCost := item.UnitCost
	if item.QtyG > 0 {
		unitCost = weightedCost / (float64(item.QtyG) / 1000)
	} else if item.QtyUnits > 0 {
		unitCost = weightedCost / float64(item.QtyUnits)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entry_items SET item_name=$2,inventory_unit=$3,batch_code=$4,unit_cost=$5,total_cost=$6 WHERE id=$1
	`, r.schema), item.ID, materialName, unit, batchCode, unitCost, weightedCost)
	return err
}

func (r Repository) materialMoveAvailabilityTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item stockapp.StockDocumentItemRow) ([]materialMoveAvailability, int64, int64, error) {
	args := []any{item.MaterialID, item.FromWarehouse}
	batchFilter := ""
	if item.BatchCode != "" {
		args = append(args, item.BatchCode)
		batchFilter = fmt.Sprintf(" AND b.batch_code=$%d", len(args))
	}
	workFilter := ""
	if detail.WorkOrderID > 0 && (detail.IsReturn || detail.Purpose == stockapp.PurposeMaterialConsumption) {
		args = append(args, detail.WorkOrderID)
		workFilter = fmt.Sprintf(`
			AND l.material_batch_id IN (
				SELECT a.material_batch_id
				FROM %s.stock_entry_batch_allocations a
				JOIN %s.stock_entry_items si ON si.id=a.stock_entry_item_id
				JOIN %s.stock_entries se ON se.id=si.stock_entry_id
				WHERE se.work_order_id=$%d AND se.status='submitted' AND si.material_id=$1
				GROUP BY a.material_batch_id
				HAVING SUM(CASE WHEN se.purpose='material_transfer_for_manufacture' AND se.is_return=false THEN a.qty_g ELSE 0 END)
				     - SUM(CASE WHEN (se.purpose='material_transfer_for_manufacture' AND se.is_return=true)
				                    OR se.purpose='material_consumption_for_manufacture' THEN a.qty_g ELSE 0 END) > 0
				    OR SUM(CASE WHEN se.purpose='material_transfer_for_manufacture' AND se.is_return=false THEN a.qty_units ELSE 0 END)
				     - SUM(CASE WHEN (se.purpose='material_transfer_for_manufacture' AND se.is_return=true)
				                    OR se.purpose='material_consumption_for_manufacture' THEN a.qty_units ELSE 0 END) > 0
			)
		`, r.schema, r.schema, r.schema, len(args))
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,l.qty_g,l.qty_units,COALESCE(b.unit_cost,0)::float8,COALESCE(b.quality_status,'unchecked')
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1 AND l.warehouse=$2
		  AND (l.qty_g>0 OR l.qty_units>0) AND b.status='active'
		  %s %s
		ORDER BY b.received_at,b.id
		FOR UPDATE OF l,b
	`, r.schema, r.schema, batchFilter, workFilter), args...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	out := make([]materialMoveAvailability, 0)
	blockedRows := make([]materialMoveAvailability, 0)
	for rows.Next() {
		var row materialMoveAvailability
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.AvailableG, &row.AvailableQty, &row.UnitCost, &row.Quality); err != nil {
			return nil, 0, 0, err
		}
		if row.Quality == "hold" || row.Quality == "reject" {
			blockedRows = append(blockedRows, row)
			continue
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}
	// pgx cannot execute another statement on the same transaction connection
	// while this result set is still open. Material return/consumption needs a
	// second query to cap each batch by what the same work order actually
	// received, so close the FIFO cursor before calculating those caps.
	rows.Close()
	if detail.WorkOrderID > 0 && (detail.IsReturn || detail.Purpose == stockapp.PurposeMaterialConsumption) {
		capToWorkOrderBalance := func(row *materialMoveAvailability) error {
			var allowedG, allowedUnits int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT
					COALESCE(SUM(CASE WHEN se.purpose='material_transfer_for_manufacture' AND se.is_return=false THEN a.qty_g ELSE 0 END),0)
					- COALESCE(SUM(CASE WHEN (se.purpose='material_transfer_for_manufacture' AND se.is_return=true)
					                        OR se.purpose='material_consumption_for_manufacture' THEN a.qty_g ELSE 0 END),0),
					COALESCE(SUM(CASE WHEN se.purpose='material_transfer_for_manufacture' AND se.is_return=false THEN a.qty_units ELSE 0 END),0)
					- COALESCE(SUM(CASE WHEN (se.purpose='material_transfer_for_manufacture' AND se.is_return=true)
					                        OR se.purpose='material_consumption_for_manufacture' THEN a.qty_units ELSE 0 END),0)
				FROM %s.stock_entry_batch_allocations a
				JOIN %s.stock_entry_items si ON si.id=a.stock_entry_item_id
				JOIN %s.stock_entries se ON se.id=si.stock_entry_id
				WHERE se.work_order_id=$1 AND se.status='submitted' AND si.material_id=$2 AND a.material_batch_id=$3
			`, r.schema, r.schema, r.schema), detail.WorkOrderID, item.MaterialID, row.BatchID).Scan(&allowedG, &allowedUnits); err != nil {
				return err
			}
			row.AvailableG = minInt64(row.AvailableG, allowedG)
			row.AvailableQty = minInt64(row.AvailableQty, allowedUnits)
			return nil
		}
		for index := range out {
			if err := capToWorkOrderBalance(&out[index]); err != nil {
				return nil, 0, 0, err
			}
		}
		for index := range blockedRows {
			if err := capToWorkOrderBalance(&blockedRows[index]); err != nil {
				return nil, 0, 0, err
			}
		}
	}
	var blockedG, blockedUnits int64
	for _, row := range blockedRows {
		blockedG += row.AvailableG
		blockedUnits += row.AvailableQty
	}
	return out, blockedG, blockedUnits, nil
}

func (r Repository) postFinishedItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item *stockapp.StockDocumentItemRow, actor string) error {
	var productName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), item.ProductID).Scan(&productName); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("product not found")
		}
		return err
	}
	specG := item.SpecG
	if specG <= 0 {
		specG = 1
	}
	totalG := item.QtyUnits*specG + item.QtyG
	units, looseG := totalG/specG, totalG%specG
	if detail.Purpose == stockapp.PurposeManufacture {
		var woProductID, woSpecG int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,spec_g FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), detail.WorkOrderID).Scan(&woProductID, &woSpecG); err != nil {
			return err
		}
		if woProductID != item.ProductID || (woSpecG > 0 && item.SpecG > 0 && woSpecG != item.SpecG) {
			return fmt.Errorf("finished product does not match work order")
		}
		beforeUnits, beforeLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse)
		if err != nil {
			return err
		}
		afterUnits, afterLoose, _, err := normalizeFinishedQty(item.SpecG, beforeUnits+units, beforeLoose+looseG)
		if err != nil {
			return err
		}
		if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse, afterUnits, afterLoose); err != nil {
			return err
		}
		batchCode := fmt.Sprintf("FP-SE-%010d-%03d", detail.ID, item.ID%1000)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_batches(
				batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
				qty_g,qty_units,remaining_g,remaining_units,unit_cost,operator,created_at
			) VALUES($1,$2,$3,$4,$5,'stock_entry_item',$6,$7,$8,$9,$8,$9,$10,$11,now())
		`, r.schema), batchCode, itemTypeFinishedProduct, item.ProductID, productName, item.SpecG, item.ID, detail.EntryNo, totalG, units, item.UnitCost, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET item_name=$2,batch_code=$3,total_cost=$4 WHERE id=$1`, r.schema),
			item.ID, productName, batchCode, stockItemTotalCost(totalG, units, item.UnitCost)); err != nil {
			return err
		}
		if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
			ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: productName, SpecG: item.SpecG, Warehouse: item.ToWarehouse,
			SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchCode: batchCode, SourceBatchID: detail.EntryNo,
			BeforeG: beforeUnits*item.SpecG + beforeLoose, ChangeG: totalG, AfterG: afterUnits*item.SpecG + afterLoose,
			BeforeUnits: beforeUnits, ChangeUnits: afterUnits - beforeUnits, AfterUnits: afterUnits, Operator: actor,
		}); err != nil {
			return err
		}
		return nil
	}
	if item.FromWarehouse == "" || item.ToWarehouse == "" {
		return fmt.Errorf("finished product transfer requires from/to warehouse")
	}
	beforeFromUnits, beforeFromLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.FromWarehouse)
	if err != nil {
		return err
	}
	beforeFromG := beforeFromUnits*item.SpecG + beforeFromLoose
	if beforeFromG < totalG {
		return fmt.Errorf("finished product stock insufficient in %s", item.FromWarehouse)
	}
	afterFromUnits, afterFromLoose, _, err := normalizeFinishedQty(item.SpecG, 0, beforeFromG-totalG)
	if err != nil {
		return err
	}
	beforeToUnits, beforeToLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse)
	if err != nil {
		return err
	}
	afterToUnits, afterToLoose, _, err := normalizeFinishedQty(item.SpecG, 0, beforeToUnits*item.SpecG+beforeToLoose+totalG)
	if err != nil {
		return err
	}
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.FromWarehouse, afterFromUnits, afterFromLoose); err != nil {
		return err
	}
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse, afterToUnits, afterToLoose); err != nil {
		return err
	}
	for _, ledger := range []ledgerEntry{
		{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: productName, SpecG: item.SpecG, Warehouse: item.FromWarehouse, SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchID: detail.EntryNo, BeforeG: beforeFromG, ChangeG: -totalG, AfterG: beforeFromG - totalG, BeforeUnits: beforeFromUnits, ChangeUnits: afterFromUnits - beforeFromUnits, AfterUnits: afterFromUnits, Operator: actor},
		{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: productName, SpecG: item.SpecG, Warehouse: item.ToWarehouse, SourceDocType: "stock_entry", SourceDocID: detail.ID, SourceBatchID: detail.EntryNo, BeforeG: beforeToUnits*item.SpecG + beforeToLoose, ChangeG: totalG, AfterG: afterToUnits*item.SpecG + afterToLoose, BeforeUnits: beforeToUnits, ChangeUnits: afterToUnits - beforeToUnits, AfterUnits: afterToUnits, Operator: actor},
	} {
		if err := insertLedgerTx(ctx, tx, r.schema, ledger); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET item_name=$2,total_cost=$3 WHERE id=$1`, r.schema),
		item.ID, productName, stockItemTotalCost(totalG, units, item.UnitCost))
	return err
}

func (r Repository) insertStockDocumentAllocationTx(ctx context.Context, tx pgx.Tx, itemID int64, alloc stockapp.StockDocumentBatchAllocation) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entry_batch_allocations(stock_entry_item_id,material_batch_id,batch_code,qty_g,qty_units,unit_cost)
		VALUES($1,$2,$3,$4,$5,$6)
	`, r.schema), itemID, alloc.MaterialBatchID, alloc.BatchCode, alloc.QtyG, alloc.QtyUnits, alloc.UnitCost)
	return err
}

func (r Repository) updateWorkOrderStockStatsTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, direction int64) error {
	if detail.WorkOrderID <= 0 {
		return nil
	}
	if detail.Purpose == stockapp.PurposeManufacture {
		if direction > 0 {
			_, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.work_orders SET status='completed',completed_at=now() WHERE id=$1
			`, r.schema), detail.WorkOrderID)
			return err
		}
		_, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_orders SET status='running',completed_at=NULL WHERE id=$1 AND status='completed'
		`, r.schema), detail.WorkOrderID)
		return err
	}
	column := ""
	if detail.Purpose == stockapp.PurposeMaterialConsumption {
		column = "consumed"
	} else if detail.Purpose == stockapp.PurposeMaterialTransferForManufacture && detail.IsReturn {
		column = "returned"
	}
	if column == "" {
		return nil
	}
	for _, item := range detail.Items {
		if item.MaterialID <= 0 {
			continue
		}
		query := fmt.Sprintf(`
			UPDATE %s.work_order_material_reservations
			SET %s_g=GREATEST(0,%s_g+$3),%s_units=GREATEST(0,%s_units+$4),updated_at=now()
			WHERE work_order_id=$1 AND material_id=$2
		`, r.schema, column, column, column, column)
		if _, err := tx.Exec(ctx, query, detail.WorkOrderID, item.MaterialID, direction*item.QtyG, direction*item.QtyUnits); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) reverseStockDocumentItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item stockapp.StockDocumentItemRow, actor string) error {
	if item.ItemType == itemTypeFinishedProduct {
		return r.reverseFinishedStockDocumentItemTx(ctx, tx, detail, item, actor)
	}
	isReceipt := detail.Purpose == stockapp.PurposeMaterialReceipt
	isConsumption := detail.Purpose == stockapp.PurposeMaterialIssue || detail.Purpose == stockapp.PurposeMaterialConsumption
	for _, alloc := range item.Allocations {
		if isReceipt {
			beforeG, beforeUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.ToWarehouse)
			if err != nil {
				return err
			}
			if beforeG != alloc.QtyG || beforeUnits != alloc.QtyUnits {
				return fmt.Errorf("received batch has been consumed; create a correction document instead")
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=0,qty_units=0,updated_at=now() WHERE material_batch_id=$1 AND warehouse=$2`, r.schema), alloc.MaterialBatchID, item.ToWarehouse); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET remaining_g=0,remaining_units=0,status='cancelled' WHERE id=$1`, r.schema), alloc.MaterialBatchID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.stock_batches SET remaining_g=0,remaining_units=0
				WHERE source_doc_type='stock_entry_item' AND source_doc_id=$1
			`, r.schema), item.ID); err != nil {
				return err
			}
			if err := r.adjustMaterialOnhandTx(ctx, tx, item.MaterialID, -alloc.QtyG, -alloc.QtyUnits); err != nil {
				return err
			}
			if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: item.ItemName, Warehouse: item.ToWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo, BeforeG: beforeG, ChangeG: -alloc.QtyG, AfterG: 0, BeforeUnits: beforeUnits, ChangeUnits: -alloc.QtyUnits, AfterUnits: 0, Operator: actor}); err != nil {
				return err
			}
			continue
		}
		if isConsumption {
			beforeFromG, beforeFromUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.FromWarehouse)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
				VALUES($1,$2,$3,$4,$5,$6,now())
				ON CONFLICT (material_batch_id,warehouse) DO UPDATE SET qty_g=material_batch_locations.qty_g+excluded.qty_g,qty_units=material_batch_locations.qty_units+excluded.qty_units,updated_at=now()
			`, r.schema), alloc.MaterialBatchID, alloc.BatchCode, item.MaterialID, item.FromWarehouse, alloc.QtyG, alloc.QtyUnits); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET remaining_g=remaining_g+$2,remaining_units=remaining_units+$3,status='active' WHERE id=$1`, r.schema), alloc.MaterialBatchID, alloc.QtyG, alloc.QtyUnits); err != nil {
				return err
			}
			if err := r.adjustMaterialOnhandTx(ctx, tx, item.MaterialID, alloc.QtyG, alloc.QtyUnits); err != nil {
				return err
			}
			if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: item.ItemName, Warehouse: item.FromWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo, BeforeG: beforeFromG, ChangeG: alloc.QtyG, AfterG: beforeFromG + alloc.QtyG, BeforeUnits: beforeFromUnits, ChangeUnits: alloc.QtyUnits, AfterUnits: beforeFromUnits + alloc.QtyUnits, Operator: actor}); err != nil {
				return err
			}
			continue
		}
		beforeToG, beforeToUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.ToWarehouse)
		if err != nil {
			return err
		}
		if beforeToG < alloc.QtyG || beforeToUnits < alloc.QtyUnits {
			return fmt.Errorf("transferred stock has been consumed; create a correction document instead")
		}
		beforeFromG, beforeFromUnits, err := materialBatchLocationBalanceTx(ctx, tx, r.schema, alloc.MaterialBatchID, item.FromWarehouse)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=qty_g-$3,qty_units=qty_units-$4,updated_at=now() WHERE material_batch_id=$1 AND warehouse=$2`, r.schema), alloc.MaterialBatchID, item.ToWarehouse, alloc.QtyG, alloc.QtyUnits); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,now())
			ON CONFLICT (material_batch_id,warehouse) DO UPDATE SET qty_g=material_batch_locations.qty_g+excluded.qty_g,qty_units=material_batch_locations.qty_units+excluded.qty_units,updated_at=now()
		`, r.schema), alloc.MaterialBatchID, alloc.BatchCode, item.MaterialID, item.FromWarehouse, alloc.QtyG, alloc.QtyUnits); err != nil {
			return err
		}
		for _, ledger := range []ledgerEntry{
			{ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: item.ItemName, Warehouse: item.ToWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo, BeforeG: beforeToG, ChangeG: -alloc.QtyG, AfterG: beforeToG - alloc.QtyG, BeforeUnits: beforeToUnits, ChangeUnits: -alloc.QtyUnits, AfterUnits: beforeToUnits - alloc.QtyUnits, Operator: actor},
			{ItemType: itemTypeMaterial, ItemID: item.MaterialID, ItemName: item.ItemName, Warehouse: item.FromWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchCode: alloc.BatchCode, SourceBatchID: detail.EntryNo, BeforeG: beforeFromG, ChangeG: alloc.QtyG, AfterG: beforeFromG + alloc.QtyG, BeforeUnits: beforeFromUnits, ChangeUnits: alloc.QtyUnits, AfterUnits: beforeFromUnits + alloc.QtyUnits, Operator: actor},
		} {
			if err := insertLedgerTx(ctx, tx, r.schema, ledger); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r Repository) reverseFinishedStockDocumentItemTx(ctx context.Context, tx pgx.Tx, detail stockapp.StockDocumentDetail, item stockapp.StockDocumentItemRow, actor string) error {
	totalG := item.QtyUnits*item.SpecG + item.QtyG
	if detail.Purpose == stockapp.PurposeManufacture {
		beforeUnits, beforeLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse)
		if err != nil {
			return err
		}
		beforeG := beforeUnits*item.SpecG + beforeLoose
		if beforeG < totalG {
			return fmt.Errorf("manufactured stock has been consumed; create a correction document instead")
		}
		afterUnits, afterLoose, _, err := normalizeFinishedQty(item.SpecG, 0, beforeG-totalG)
		if err != nil {
			return err
		}
		if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse, afterUnits, afterLoose); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches SET remaining_g=0,remaining_units=0
			WHERE source_doc_type='stock_entry_item' AND source_doc_id=$1
		`, r.schema), item.ID); err != nil {
			return err
		}
		return insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: item.ItemName, SpecG: item.SpecG, Warehouse: item.ToWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchCode: item.BatchCode, SourceBatchID: detail.EntryNo, BeforeG: beforeG, ChangeG: -totalG, AfterG: beforeG - totalG, BeforeUnits: beforeUnits, ChangeUnits: afterUnits - beforeUnits, AfterUnits: afterUnits, Operator: actor})
	}
	beforeToUnits, beforeToLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse)
	if err != nil {
		return err
	}
	beforeToG := beforeToUnits*item.SpecG + beforeToLoose
	if beforeToG < totalG {
		return fmt.Errorf("transferred finished stock has been consumed; create a correction document instead")
	}
	beforeFromUnits, beforeFromLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.FromWarehouse)
	if err != nil {
		return err
	}
	afterToUnits, afterToLoose, _, _ := normalizeFinishedQty(item.SpecG, 0, beforeToG-totalG)
	afterFromUnits, afterFromLoose, _, _ := normalizeFinishedQty(item.SpecG, 0, beforeFromUnits*item.SpecG+beforeFromLoose+totalG)
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.ToWarehouse, afterToUnits, afterToLoose); err != nil {
		return err
	}
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, item.ProductID, item.SpecG, item.FromWarehouse, afterFromUnits, afterFromLoose); err != nil {
		return err
	}
	for _, ledger := range []ledgerEntry{
		{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: item.ItemName, SpecG: item.SpecG, Warehouse: item.ToWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchID: detail.EntryNo, BeforeG: beforeToG, ChangeG: -totalG, AfterG: beforeToG - totalG, BeforeUnits: beforeToUnits, ChangeUnits: afterToUnits - beforeToUnits, AfterUnits: afterToUnits, Operator: actor},
		{ItemType: itemTypeFinishedProduct, ItemID: item.ProductID, ItemName: item.ItemName, SpecG: item.SpecG, Warehouse: item.FromWarehouse, SourceDocType: "stock_entry_cancel", SourceDocID: detail.ID, SourceBatchID: detail.EntryNo, BeforeG: beforeFromUnits*item.SpecG + beforeFromLoose, ChangeG: totalG, AfterG: afterFromUnits*item.SpecG + afterFromLoose, BeforeUnits: beforeFromUnits, ChangeUnits: afterFromUnits - beforeFromUnits, AfterUnits: afterFromUnits, Operator: actor},
	} {
		if err := insertLedgerTx(ctx, tx, r.schema, ledger); err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) adjustMaterialOnhandTx(ctx context.Context, tx pgx.Tx, materialID, changeG, changeUnits int64) error {
	var beforeG, beforeUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), materialID).Scan(&beforeG, &beforeUnits); err != nil {
		return err
	}
	if beforeG+changeG < 0 || beforeUnits+changeUnits < 0 {
		return fmt.Errorf("material onhand insufficient")
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, r.schema), materialID, beforeG+changeG, beforeUnits+changeUnits)
	return err
}

func materialWarehouseBalanceTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, warehouse string) (int64, int64, error) {
	var qtyG, qtyUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(qty_g),0)::bigint,COALESCE(SUM(qty_units),0)::bigint
		FROM %s.material_batch_locations WHERE material_id=$1 AND warehouse=$2
	`, schema), materialID, warehouse).Scan(&qtyG, &qtyUnits)
	return qtyG, qtyUnits, err
}

func materialBatchLocationBalanceTx(ctx context.Context, tx pgx.Tx, schema string, batchID int64, warehouse string) (int64, int64, error) {
	var qtyG, qtyUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g,qty_units FROM %s.material_batch_locations
		WHERE material_batch_id=$1 AND warehouse=$2 FOR UPDATE
	`, schema), batchID, warehouse).Scan(&qtyG, &qtyUnits)
	if err == pgx.ErrNoRows {
		return 0, 0, nil
	}
	return qtyG, qtyUnits, err
}

func stockItemTotalCost(qtyG, qtyUnits int64, unitCost float64) float64 {
	if qtyG > 0 {
		return math.Round((float64(qtyG)/1000*unitCost)*10000) / 10000
	}
	return math.Round((float64(qtyUnits)*unitCost)*10000) / 10000
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
