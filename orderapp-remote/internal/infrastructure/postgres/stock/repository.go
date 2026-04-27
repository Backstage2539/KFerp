package stock

import (
	"context"
	"fmt"
	"strings"

	stockapp "orderapp/internal/application/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	itemTypeMaterial        = "material"
	itemTypeFinishedProduct = "finished_product"
	sourceMaterialReceipt   = "material_receipt"
	sourceStockAdjustment   = "stock_adjustment"
)

type Repository struct {
	pool   *pgxpool.Pool
	schema string
}

func NewRepository(pool *pgxpool.Pool, schema string) Repository {
	return Repository{pool: pool, schema: schema}
}

func (r Repository) ListLedger(ctx context.Context, query stockapp.LedgerQuery) (stockapp.LedgerResult, error) {
	where, args := []string{"1=1"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if query.Q != "" {
		args = append(args, "%"+query.Q+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(item_name ILIKE $%d OR source_batch_code ILIKE $%d OR source_batch_id ILIKE $%d)", n, n, n))
	}
	if query.ItemType != "" {
		add("item_type=$%d", query.ItemType)
	}
	if query.Warehouse != "" {
		add("warehouse=$%d", query.Warehouse)
	}
	if query.SourceDocType != "" {
		add("source_doc_type=$%d", query.SourceDocType)
	}
	if query.SourceBatch != "" {
		args = append(args, query.SourceBatch)
		n := len(args)
		where = append(where, fmt.Sprintf("(source_batch_code=$%d OR source_batch_id=$%d)", n, n))
	}
	if query.From != "" {
		add("created_at >= $%d::date", query.From)
	}
	if query.To != "" {
		add("created_at < ($%d::date + INTERVAL '1 day')", query.To)
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT id,item_type,item_id,item_name,spec_g,warehouse,
		       source_doc_type,source_doc_id,source_batch_code,source_batch_id,
		       qty_before_g,qty_change_g,qty_after_g,
		       qty_before_units,qty_change_units,qty_after_units,
		       operator,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_ledger_entries
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, r.schema, strings.Join(where, " AND "), limitArg, offsetArg)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return stockapp.LedgerResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.LedgerRow, 0)
	for rows.Next() {
		var row stockapp.LedgerRow
		if err := rows.Scan(&row.ID, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.Warehouse, &row.SourceDocType, &row.SourceDocID, &row.SourceBatchCode, &row.SourceBatchID, &row.QtyBeforeG, &row.QtyChangeG, &row.QtyAfterG, &row.QtyBeforeUnits, &row.QtyChangeUnits, &row.QtyAfterUnits, &row.Operator, &row.CreatedAt); err != nil {
			return stockapp.LedgerResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.LedgerResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.LedgerResult{Rows: out, HasNext: hasNext}, nil
}

func (r Repository) ListBatches(ctx context.Context, query stockapp.BatchQuery) (stockapp.BatchResult, error) {
	where, args := []string{"1=1"}, []any{}
	if query.Q != "" {
		args = append(args, "%"+query.Q+"%")
		where = append(where, fmt.Sprintf("(batch_code ILIKE $%d OR item_name ILIKE $%d OR source_batch_id ILIKE $%d)", len(args), len(args), len(args)))
	}
	if query.ItemType != "" {
		args = append(args, query.ItemType)
		where = append(where, fmt.Sprintf("item_type=$%d", len(args)))
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT id,batch_code,item_type,item_id,item_name,spec_g,
		       source_doc_type,source_doc_id,source_batch_id,
		       qty_g,qty_units,remaining_g,remaining_units,COALESCE(unit_cost,0),
		       operator,to_char(created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_batches
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, r.schema, strings.Join(where, " AND "), limitArg, offsetArg)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return stockapp.BatchResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.BatchRow, 0)
	for rows.Next() {
		var row stockapp.BatchRow
		if err := rows.Scan(&row.ID, &row.BatchCode, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.SourceDocType, &row.SourceDocID, &row.SourceBatchID, &row.QtyG, &row.QtyUnits, &row.RemainingG, &row.RemainingUnits, &row.UnitCost, &row.Operator, &row.CreatedAt); err != nil {
			return stockapp.BatchResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.BatchResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.BatchResult{Rows: out, HasNext: hasNext}, nil
}

func (r Repository) ListMaterialBatches(ctx context.Context, query stockapp.MaterialBatchQuery) (stockapp.MaterialBatchResult, error) {
	where, args := []string{"1=1"}, []any{}
	if query.Q != "" {
		args = append(args, "%"+query.Q+"%")
		where = append(where, fmt.Sprintf("(b.batch_code ILIKE $%d OR m.name ILIKE $%d OR b.supplier ILIKE $%d)", len(args), len(args), len(args)))
	}
	if query.MaterialID > 0 {
		args = append(args, query.MaterialID)
		where = append(where, fmt.Sprintf("b.material_id=$%d", len(args)))
	}
	if query.ActiveOnly {
		where = append(where, "b.remaining_g > 0 AND b.status='active'")
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT b.id,b.batch_code,b.material_id,COALESCE(m.name,''),b.supplier,b.receipt_id,
		       b.qty_g,b.remaining_g,COALESCE(b.unit_cost,0),
		       to_char(b.received_at,'YYYY-MM-DD HH24:MI'),b.status,b.note
		FROM %s.material_batches b
		LEFT JOIN %s.materials m ON m.id=b.material_id
		WHERE %s
		ORDER BY b.received_at DESC, b.id DESC
		LIMIT $%d OFFSET $%d
	`, r.schema, r.schema, strings.Join(where, " AND "), limitArg, offsetArg)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return stockapp.MaterialBatchResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.MaterialBatchRow, 0)
	for rows.Next() {
		var row stockapp.MaterialBatchRow
		if err := rows.Scan(&row.ID, &row.BatchCode, &row.MaterialID, &row.MaterialName, &row.Supplier, &row.ReceiptID, &row.QtyG, &row.RemainingG, &row.UnitCost, &row.ReceivedAt, &row.Status, &row.Note); err != nil {
			return stockapp.MaterialBatchResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.MaterialBatchResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.MaterialBatchResult{Rows: out, HasNext: hasNext}, nil
}

func (r Repository) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var materialName string
	var beforeG, beforeUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.MaterialID).Scan(&materialName, &beforeG, &beforeUnits); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.MaterialReceiptResult{}, fmt.Errorf("material not found")
		}
		return stockapp.MaterialReceiptResult{}, err
	}
	afterG := beforeG + cmd.QtyG
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,updated_at=now() WHERE id=$1`, r.schema), cmd.MaterialID, afterG); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}

	var receiptID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_receipts(material_id,supplier,qty_g,unit_cost,note,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
		RETURNING id
	`, r.schema), cmd.MaterialID, cmd.Supplier, cmd.QtyG, cmd.UnitCost, cmd.Note, cmd.Operator).Scan(&receiptID); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	batchCode := fmt.Sprintf("MB-%010d", receiptID)
	var batchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(batch_code,material_id,supplier,receipt_id,qty_g,remaining_g,unit_cost,note,received_at,created_at)
		VALUES($1,$2,$3,$4,$5,$5,$6,$7,now(),now())
		RETURNING id
	`, r.schema), batchCode, cmd.MaterialID, cmd.Supplier, receiptID, cmd.QtyG, cmd.UnitCost, cmd.Note).Scan(&batchID); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,unit_cost,operator,created_at
		) VALUES($1,$2,$3,$4,0,$5,$6,$1,$7,0,$7,0,$8,$9,now())
	`, r.schema), batchCode, itemTypeMaterial, cmd.MaterialID, materialName, sourceMaterialReceipt, receiptID, cmd.QtyG, cmd.UnitCost, cmd.Operator); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeMaterial, ItemID: cmd.MaterialID, ItemName: materialName, Warehouse: "materials",
		SourceDocType: sourceMaterialReceipt, SourceDocID: receiptID, SourceBatchCode: batchCode, SourceBatchID: batchCode,
		BeforeG: beforeG, ChangeG: cmd.QtyG, AfterG: afterG, BeforeUnits: beforeUnits, ChangeUnits: 0, AfterUnits: beforeUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "material_receipt", &receiptID, "submit", postgresinfra.StrPtr("qty_g"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.QtyG)), postgresinfra.AuditMeta{"material_id": cmd.MaterialID, "batch_code": batchCode}); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	return stockapp.MaterialReceiptResult{ReceiptID: receiptID, BatchID: batchID, BatchCode: batchCode}, nil
}

func (r Repository) CreateAdjustment(ctx context.Context, cmd stockapp.StockAdjustmentCommand) (stockapp.StockAdjustmentResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	itemName, beforeG, beforeUnits, afterG, afterUnits, err := r.applyAdjustmentTx(ctx, tx, cmd)
	if err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	changeG := afterG - beforeG
	changeUnits := afterUnits - beforeUnits
	var adjustmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustments(item_type,item_id,item_name,spec_g,warehouse,reason,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		RETURNING id
	`, r.schema), cmd.ItemType, cmd.ItemID, itemName, cmd.SpecG, cmd.Warehouse, cmd.Reason, cmd.Operator).Scan(&adjustmentID); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustment_items(adjustment_id,item_type,item_id,spec_g,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, r.schema), adjustmentID, cmd.ItemType, cmd.ItemID, cmd.SpecG, beforeG, changeG, afterG, beforeUnits, changeUnits, afterUnits); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	batchCode := fmt.Sprintf("ADJ-%010d", adjustmentID)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,'',$8,$9,0,0,$10,now())
	`, r.schema), batchCode, cmd.ItemType, cmd.ItemID, itemName, cmd.SpecG, sourceStockAdjustment, adjustmentID, changeG, changeUnits, cmd.Operator); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: cmd.ItemType, ItemID: cmd.ItemID, ItemName: itemName, SpecG: cmd.SpecG, Warehouse: cmd.Warehouse,
		SourceDocType: sourceStockAdjustment, SourceDocID: adjustmentID, SourceBatchCode: batchCode,
		BeforeG: beforeG, ChangeG: changeG, AfterG: afterG, BeforeUnits: beforeUnits, ChangeUnits: changeUnits, AfterUnits: afterUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	return stockapp.StockAdjustmentResult{AdjustmentID: adjustmentID}, nil
}

func (r Repository) applyAdjustmentTx(ctx context.Context, tx pgx.Tx, cmd stockapp.StockAdjustmentCommand) (string, int64, int64, int64, int64, error) {
	if cmd.ItemType == itemTypeMaterial {
		var name string
		var beforeG, beforeUnits int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.ItemID).Scan(&name, &beforeG, &beforeUnits); err != nil {
			return "", 0, 0, 0, 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, r.schema), cmd.ItemID, cmd.TargetG, cmd.TargetUnits); err != nil {
			return "", 0, 0, 0, 0, err
		}
		return name, beforeG, beforeUnits, cmd.TargetG, cmd.TargetUnits, nil
	}

	var name string
	var beforeUnits, beforeLoose int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), cmd.ItemID).Scan(&name); err != nil {
		return "", 0, 0, 0, 0, err
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 FOR UPDATE`, r.schema), cmd.ItemID, cmd.SpecG).Scan(&beforeUnits, &beforeLoose)
	if err != nil && err != pgx.ErrNoRows {
		return "", 0, 0, 0, 0, err
	}
	beforeG := beforeUnits*cmd.SpecG + beforeLoose
	afterG := cmd.TargetUnits*cmd.SpecG + cmd.TargetG
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT (product_id,spec_g) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, r.schema), cmd.ItemID, cmd.SpecG, cmd.TargetUnits, cmd.TargetG); err != nil {
		return "", 0, 0, 0, 0, err
	}
	return name, beforeG, beforeUnits, afterG, cmd.TargetUnits, nil
}

type ledgerEntry struct {
	ItemType        string
	ItemID          int64
	ItemName        string
	SpecG           int64
	Warehouse       string
	SourceDocType   string
	SourceDocID     int64
	SourceBatchCode string
	SourceBatchID   string
	BeforeG         int64
	ChangeG         int64
	AfterG          int64
	BeforeUnits     int64
	ChangeUnits     int64
	AfterUnits      int64
	Operator        string
}

func insertLedgerTx(ctx context.Context, tx pgx.Tx, schema string, e ledgerEntry) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,now())
	`, schema),
		e.ItemType, e.ItemID, e.ItemName, e.SpecG, e.Warehouse,
		e.SourceDocType, e.SourceDocID, e.SourceBatchCode, e.SourceBatchID,
		e.BeforeG, e.ChangeG, e.AfterG, e.BeforeUnits, e.ChangeUnits, e.AfterUnits, e.Operator,
	)
	return err
}
