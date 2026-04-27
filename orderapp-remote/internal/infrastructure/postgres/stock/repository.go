package stock

import (
	"context"
	"fmt"
	"strings"
	"time"

	stockapp "orderapp/internal/application/stock"
	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	itemTypeMaterial        = "material"
	itemTypeFinishedProduct = "finished_product"
	sourceMaterialReceipt   = "material_receipt"
	sourceStockAdjustment   = "stock_adjustment"
	sourceMaterialTransfer  = "material_transfer"
	sourceFinishedTransfer  = "finished_product_transfer"
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

func (r Repository) ListWarehouses(ctx context.Context) ([]stockapp.WarehouseRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT code,name,kind,parent_code,sort_order,is_default,active,description
		FROM %s.warehouses
		WHERE active=true
		ORDER BY sort_order, code
	`, r.schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]stockapp.WarehouseRow, 0)
	for rows.Next() {
		var row stockapp.WarehouseRow
		if err := rows.Scan(&row.Code, &row.Name, &row.Kind, &row.ParentCode, &row.SortOrder, &row.IsDefault, &row.Active, &row.Description); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) ListMaterialBatchLocations(ctx context.Context, query stockapp.MaterialBatchLocationQuery) (stockapp.MaterialBatchLocationResult, error) {
	where, args := []string{"1=1"}, []any{}
	if query.Q != "" {
		args = append(args, "%"+query.Q+"%")
		where = append(where, fmt.Sprintf("(l.batch_code ILIKE $%d OR m.name ILIKE $%d)", len(args), len(args)))
	}
	if query.MaterialID > 0 {
		args = append(args, query.MaterialID)
		where = append(where, fmt.Sprintf("l.material_id=$%d", len(args)))
	}
	if query.Warehouse != "" {
		args = append(args, query.Warehouse)
		where = append(where, fmt.Sprintf("l.warehouse=$%d", len(args)))
	}
	if query.ActiveOnly {
		where = append(where, "l.qty_g > 0")
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.material_batch_id,l.batch_code,l.material_id,COALESCE(m.name,''),l.warehouse,
		       COALESCE(w.name,l.warehouse),l.qty_g,
		       COALESCE(to_char(b.received_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(l.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.material_batch_locations l
		LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		LEFT JOIN %s.warehouses w ON w.code=l.warehouse
		WHERE %s
		ORDER BY l.warehouse, b.received_at, l.material_batch_id
		LIMIT $%d OFFSET $%d
	`, r.schema, r.schema, r.schema, r.schema, strings.Join(where, " AND "), limitArg, offsetArg), args...)
	if err != nil {
		return stockapp.MaterialBatchLocationResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.MaterialBatchLocationRow, 0)
	for rows.Next() {
		var row stockapp.MaterialBatchLocationRow
		if err := rows.Scan(&row.MaterialBatchID, &row.BatchCode, &row.MaterialID, &row.MaterialName, &row.Warehouse, &row.WarehouseName, &row.QtyG, &row.ReceivedAt, &row.UpdatedAt); err != nil {
			return stockapp.MaterialBatchLocationResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.MaterialBatchLocationResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.MaterialBatchLocationResult{Rows: out, HasNext: hasNext}, nil
}

func (r Repository) ListWarehouseInventory(ctx context.Context, query stockapp.WarehouseInventoryQuery) (stockapp.WarehouseInventoryResult, error) {
	q := strings.TrimSpace(query.Q)
	qLike := "%" + q + "%"
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH warehouse_inventory AS (
			SELECT l.warehouse,
			       COALESCE(w.name,l.warehouse) AS warehouse_name,
			       COALESCE(w.kind,'') AS warehouse_kind,
			       'material' AS item_type,
			       l.material_id AS item_id,
			       COALESCE(m.name,'') AS item_name,
			       0::bigint AS spec_g,
			       l.material_batch_id AS batch_id,
			       l.batch_code AS batch_code,
			       l.qty_g AS qty_g,
			       0::bigint AS qty_units,
			       COALESCE(b.unit_cost,0) AS unit_cost,
			       l.updated_at AS updated_at
			FROM %s.material_batch_locations l
			LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
			LEFT JOIN %s.materials m ON m.id=l.material_id
			LEFT JOIN %s.warehouses w ON w.code=l.warehouse
			WHERE l.qty_g <> 0
			  AND ($1 = '' OR l.batch_code ILIKE $2 OR m.name ILIKE $2)
			  AND ($3 = '' OR l.warehouse = $3)
			  AND ($4 = '' OR $4 = 'material')
			UNION ALL
			SELECT fi.warehouse,
			       COALESCE(w.name,fi.warehouse) AS warehouse_name,
			       COALESCE(w.kind,'finished') AS warehouse_kind,
			       'finished_product' AS item_type,
			       fi.product_id AS item_id,
			       COALESCE(p.name,'') AS item_name,
			       fi.spec_g,
			       0::bigint AS batch_id,
			       '' AS batch_code,
			       (fi.onhand_units * fi.spec_g + fi.onhand_loose_g) AS qty_g,
			       fi.onhand_units AS qty_units,
			       0::numeric AS unit_cost,
			       fi.updated_at AS updated_at
			FROM %s.finished_inventory fi
			LEFT JOIN %s.products p ON p.id=fi.product_id
			LEFT JOIN %s.warehouses w ON w.code=fi.warehouse
			WHERE (fi.onhand_units <> 0 OR fi.onhand_loose_g <> 0)
			  AND ($1 = '' OR p.name ILIKE $2)
			  AND ($3 = '' OR fi.warehouse = $3)
			  AND ($4 = '' OR $4 = 'finished_product')
		)
		SELECT warehouse,warehouse_name,warehouse_kind,item_type,item_id,item_name,spec_g,batch_id,batch_code,
		       qty_g,qty_units,COALESCE(unit_cost,0),to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM warehouse_inventory
		ORDER BY warehouse_name,item_type,item_name,spec_g,batch_code
		LIMIT $5 OFFSET $6
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), q, qLike, query.Warehouse, query.ItemType, query.Limit+1, query.Offset)
	if err != nil {
		return stockapp.WarehouseInventoryResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.WarehouseInventoryRow, 0)
	for rows.Next() {
		var row stockapp.WarehouseInventoryRow
		if err := rows.Scan(&row.Warehouse, &row.WarehouseName, &row.WarehouseKind, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.BatchID, &row.BatchCode, &row.QtyG, &row.QtyUnits, &row.UnitCost, &row.UpdatedAt); err != nil {
			return stockapp.WarehouseInventoryResult{}, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.WarehouseInventoryResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.WarehouseInventoryResult{Rows: out, HasNext: hasNext}, nil
}

func (r Repository) GetStockTrace(ctx context.Context, query stockapp.StockTraceQuery) (stockapp.StockTraceResult, error) {
	var result stockapp.StockTraceResult
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.batch_code,b.item_id,b.item_name,b.spec_g,COALESCE(l.warehouse,'finished_goods'),
		       b.qty_g,b.qty_units,b.remaining_g,b.remaining_units,to_char(b.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_batches b
		LEFT JOIN %s.stock_ledger_entries l
		  ON l.source_doc_type=b.source_doc_type
		 AND l.source_doc_id=b.source_doc_id
		 AND l.item_type=b.item_type
		 AND l.item_id=b.item_id
		 AND l.spec_g=b.spec_g
		 AND l.source_batch_code=b.batch_code
		WHERE b.batch_code=$1
		  AND b.item_type=$2
		ORDER BY l.id NULLS LAST
		LIMIT 1
	`, r.schema, r.schema), query.BatchCode, itemTypeFinishedProduct).Scan(
		&result.FinishedBatch.BatchCode,
		&result.FinishedBatch.ProductID,
		&result.FinishedBatch.ProductName,
		&result.FinishedBatch.SpecG,
		&result.FinishedBatch.Warehouse,
		&result.FinishedBatch.QtyG,
		&result.FinishedBatch.QtyUnits,
		&result.FinishedBatch.RemainingG,
		&result.FinishedBatch.RemainingUnits,
		&result.FinishedBatch.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.StockTraceResult{}, fmt.Errorf("batch not found")
		}
		return stockapp.StockTraceResult{}, err
	}

	var runningItemID int64
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(p.running_item_id,0),
		       COALESCE(wo.work_order_no,''),
		       COALESCE(p.batch_id,''),
		       COALESCE(p.order_nos,''),
		       COALESCE(p.input_g,0),
		       COALESCE(p.finished_total_g,0),
		       COALESCE(p.actual_yield_rate,0),
		       COALESCE(p.started_by,''),
		       COALESCE(p.finished_by,''),
		       COALESCE(to_char(p.finished_at,'YYYY-MM-DD HH24:MI'),'')
		FROM %s.stock_batches b
		LEFT JOIN %s.production_logs p ON p.running_item_id=b.source_doc_id
		LEFT JOIN %s.work_orders wo ON wo.running_item_id=p.running_item_id
		WHERE b.batch_code=$1
		  AND b.source_doc_type=$2
		LIMIT 1
	`, r.schema, r.schema, r.schema), query.BatchCode, "production_run").Scan(
		&result.Production.RunningItemID,
		&result.Production.WorkOrderNo,
		&result.Production.BatchID,
		&result.Production.OrderNos,
		&result.Production.InputG,
		&result.Production.FinishedTotalG,
		&result.Production.ActualYieldRate,
		&result.Production.StartedBy,
		&result.Production.FinishedBy,
		&result.Production.FinishedAt,
	)
	if err != nil && err != pgx.ErrNoRows {
		return stockapp.StockTraceResult{}, err
	}
	runningItemID = result.Production.RunningItemID
	if runningItemID <= 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT material_id,COALESCE(material_name,''),COALESCE(unit,''),deduct_g,deduct_units,
		       COALESCE(material_batch_id,0),COALESCE(material_batch_code,'')
		FROM %s.material_consumption_logs
		WHERE running_item_id=$1
		ORDER BY id
	`, r.schema), runningItemID)
	if err != nil {
		return stockapp.StockTraceResult{}, err
	}
	defer rows.Close()
	result.Materials = make([]stockapp.TraceMaterial, 0)
	for rows.Next() {
		var row stockapp.TraceMaterial
		if err := rows.Scan(&row.MaterialID, &row.MaterialName, &row.Unit, &row.DeductG, &row.DeductUnits, &row.MaterialBatchID, &row.MaterialBatchCode); err != nil {
			return stockapp.StockTraceResult{}, err
		}
		result.Materials = append(result.Materials, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.StockTraceResult{}, err
	}
	return result, nil
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
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
			batch_code=excluded.batch_code,
			material_id=excluded.material_id,
			qty_g=material_batch_locations.qty_g+excluded.qty_g,
			updated_at=now()
	`, r.schema), batchID, batchCode, cmd.MaterialID, stockdomain.WarehouseRawMaterials, cmd.QtyG); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeMaterial, ItemID: cmd.MaterialID, ItemName: materialName, Warehouse: stockdomain.WarehouseRawMaterials,
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

func (r Repository) TransferMaterial(ctx context.Context, cmd stockapp.MaterialTransferCommand) (stockapp.MaterialTransferResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if cmd.IdempotencyKey != "" {
		result, found, err := r.loadTransferByIdempotencyTx(ctx, tx, cmd.IdempotencyKey)
		if err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		if found {
			if err := tx.Commit(ctx); err != nil {
				return stockapp.MaterialTransferResult{}, err
			}
			return result, nil
		}
	}

	var materialName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.MaterialID).Scan(&materialName); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.MaterialTransferResult{}, fmt.Errorf("material not found")
		}
		return stockapp.MaterialTransferResult{}, err
	}
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.FromWarehouse); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.ToWarehouse); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,l.qty_g
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1
		  AND l.warehouse=$2
		  AND l.qty_g > 0
		  AND b.status='active'
		  AND b.remaining_g > 0
		ORDER BY b.received_at, b.id
		FOR UPDATE OF l,b
	`, r.schema, r.schema), cmd.MaterialID, cmd.FromWarehouse)
	if err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	defer rows.Close()
	available := make([]stockdomain.BatchAvailability, 0)
	beforeFromByBatch := map[int64]int64{}
	for rows.Next() {
		var batch stockdomain.BatchAvailability
		if err := rows.Scan(&batch.BatchID, &batch.BatchCode, &batch.AvailableG); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		available = append(available, batch)
		beforeFromByBatch[batch.BatchID] = batch.AvailableG
	}
	if err := rows.Err(); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	allocations, err := stockdomain.AllocateFIFO(available, cmd.QtyG)
	if err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	if len(allocations) == 0 {
		return stockapp.MaterialTransferResult{}, fmt.Errorf("material stock insufficient in %s", cmd.FromWarehouse)
	}

	var transferID int64
	transferNo := ""
	tempTransferNo := fmt.Sprintf("MT-TMP-%d", time.Now().UnixNano())
	if cmd.IdempotencyKey != "" {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_transfers(transfer_no,material_id,material_name,from_warehouse,to_warehouse,qty_g,note,operator,idempotency_key,created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.MaterialID, materialName, cmd.FromWarehouse, cmd.ToWarehouse, cmd.QtyG, cmd.Note, cmd.Operator, cmd.IdempotencyKey).Scan(&transferID, &transferNo)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_transfers(transfer_no,material_id,material_name,from_warehouse,to_warehouse,qty_g,note,operator,created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.MaterialID, materialName, cmd.FromWarehouse, cmd.ToWarehouse, cmd.QtyG, cmd.Note, cmd.Operator).Scan(&transferID, &transferNo)
	}
	if err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	if strings.HasPrefix(transferNo, "MT-TMP-") {
		transferNo = fmt.Sprintf("MT-%010d", transferID)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_transfers SET transfer_no=$2 WHERE id=$1`, r.schema), transferID, transferNo); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
	}

	outAllocations := make([]stockapp.MaterialTransferAllocation, 0, len(allocations))
	for _, alloc := range allocations {
		beforeFrom := beforeFromByBatch[alloc.BatchID]
		afterFrom := beforeFrom - alloc.QtyG
		if afterFrom < 0 {
			return stockapp.MaterialTransferResult{}, fmt.Errorf("material stock insufficient in %s", cmd.FromWarehouse)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batch_locations
			SET qty_g=$3, updated_at=now()
			WHERE material_batch_id=$1 AND warehouse=$2
		`, r.schema), alloc.BatchID, cmd.FromWarehouse, afterFrom); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}

		beforeTo, err := materialBatchLocationQtyTx(ctx, tx, r.schema, alloc.BatchID, cmd.ToWarehouse)
		if err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		afterTo := beforeTo + alloc.QtyG
		if beforeTo == 0 {
			_, err = tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,updated_at)
				VALUES($1,$2,$3,$4,$5,now())
				ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
					batch_code=excluded.batch_code,
					material_id=excluded.material_id,
					qty_g=material_batch_locations.qty_g+excluded.qty_g,
					updated_at=now()
			`, r.schema), alloc.BatchID, alloc.BatchCode, cmd.MaterialID, cmd.ToWarehouse, alloc.QtyG)
		} else {
			_, err = tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.material_batch_locations
				SET qty_g=$3, updated_at=now()
				WHERE material_batch_id=$1 AND warehouse=$2
			`, r.schema), alloc.BatchID, cmd.ToWarehouse, afterTo)
		}
		if err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_transfer_items(transfer_id,material_batch_id,material_batch_code,qty_g)
			VALUES($1,$2,$3,$4)
		`, r.schema), transferID, alloc.BatchID, alloc.BatchCode, alloc.QtyG); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
			ItemType: itemTypeMaterial, ItemID: cmd.MaterialID, ItemName: materialName, Warehouse: cmd.FromWarehouse,
			SourceDocType: sourceMaterialTransfer, SourceDocID: transferID, SourceBatchCode: alloc.BatchCode, SourceBatchID: transferNo,
			BeforeG: beforeFrom, ChangeG: -alloc.QtyG, AfterG: afterFrom, Operator: cmd.Operator,
		}); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
			ItemType: itemTypeMaterial, ItemID: cmd.MaterialID, ItemName: materialName, Warehouse: cmd.ToWarehouse,
			SourceDocType: sourceMaterialTransfer, SourceDocID: transferID, SourceBatchCode: alloc.BatchCode, SourceBatchID: transferNo,
			BeforeG: beforeTo, ChangeG: alloc.QtyG, AfterG: afterTo, Operator: cmd.Operator,
		}); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		outAllocations = append(outAllocations, stockapp.MaterialTransferAllocation{MaterialBatchID: alloc.BatchID, BatchCode: alloc.BatchCode, QtyG: alloc.QtyG})
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "material_transfer", &transferID, "submit", postgresinfra.StrPtr("qty_g"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.QtyG)), postgresinfra.AuditMeta{"material_id": cmd.MaterialID, "transfer_no": transferNo, "from": cmd.FromWarehouse, "to": cmd.ToWarehouse}); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	return stockapp.MaterialTransferResult{TransferID: transferID, TransferNo: transferNo, Allocations: outAllocations}, nil
}

func (r Repository) TransferFinishedProduct(ctx context.Context, cmd stockapp.FinishedProductTransferCommand) (stockapp.FinishedProductTransferResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if cmd.IdempotencyKey != "" {
		result, found, err := r.loadFinishedTransferByIdempotencyTx(ctx, tx, cmd.IdempotencyKey)
		if err != nil {
			return stockapp.FinishedProductTransferResult{}, err
		}
		if found {
			if err := tx.Commit(ctx); err != nil {
				return stockapp.FinishedProductTransferResult{}, err
			}
			return result, nil
		}
	}

	var productName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), cmd.ProductID).Scan(&productName); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.FinishedProductTransferResult{}, fmt.Errorf("product not found")
		}
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.FromWarehouse); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.ToWarehouse); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}

	transferUnits, transferLoose, transferG, err := normalizeFinishedQty(cmd.SpecG, cmd.QtyUnits, cmd.QtyLooseG)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeFromUnits, beforeFromLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, cmd.ProductID, cmd.SpecG, cmd.FromWarehouse)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeFromG := beforeFromUnits*cmd.SpecG + beforeFromLoose
	if beforeFromG < transferG {
		return stockapp.FinishedProductTransferResult{}, fmt.Errorf("finished stock insufficient in %s", cmd.FromWarehouse)
	}
	afterFromUnits, afterFromLoose, _, err := normalizeFinishedQty(cmd.SpecG, 0, beforeFromG-transferG)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeToUnits, beforeToLoose, err := finishedInventoryQtyTx(ctx, tx, r.schema, cmd.ProductID, cmd.SpecG, cmd.ToWarehouse)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeToG := beforeToUnits*cmd.SpecG + beforeToLoose
	afterToUnits, afterToLoose, _, err := normalizeFinishedQty(cmd.SpecG, 0, beforeToG+transferG)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}

	var transferID int64
	transferNo := ""
	tempTransferNo := fmt.Sprintf("FT-TMP-%d", time.Now().UnixNano())
	if cmd.IdempotencyKey != "" {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_product_transfers(
				transfer_no,product_id,product_name,spec_g,from_warehouse,to_warehouse,
				qty_g,qty_units,qty_loose_g,note,operator,idempotency_key,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.ProductID, productName, cmd.SpecG, cmd.FromWarehouse, cmd.ToWarehouse, transferG, transferUnits, transferLoose, cmd.Note, cmd.Operator, cmd.IdempotencyKey).Scan(&transferID, &transferNo)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_product_transfers(
				transfer_no,product_id,product_name,spec_g,from_warehouse,to_warehouse,
				qty_g,qty_units,qty_loose_g,note,operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.ProductID, productName, cmd.SpecG, cmd.FromWarehouse, cmd.ToWarehouse, transferG, transferUnits, transferLoose, cmd.Note, cmd.Operator).Scan(&transferID, &transferNo)
	}
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if strings.HasPrefix(transferNo, "FT-TMP-") {
		transferNo = fmt.Sprintf("FT-%010d", transferID)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_product_transfers SET transfer_no=$2 WHERE id=$1`, r.schema), transferID, transferNo); err != nil {
			return stockapp.FinishedProductTransferResult{}, err
		}
	}

	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, cmd.ProductID, cmd.SpecG, cmd.FromWarehouse, afterFromUnits, afterFromLoose); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, cmd.ProductID, cmd.SpecG, cmd.ToWarehouse, afterToUnits, afterToLoose); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeFinishedProduct, ItemID: cmd.ProductID, ItemName: productName, SpecG: cmd.SpecG, Warehouse: cmd.FromWarehouse,
		SourceDocType: sourceFinishedTransfer, SourceDocID: transferID, SourceBatchCode: transferNo, SourceBatchID: transferNo,
		BeforeG: beforeFromG, ChangeG: -transferG, AfterG: beforeFromG - transferG, BeforeUnits: beforeFromUnits, ChangeUnits: afterFromUnits - beforeFromUnits, AfterUnits: afterFromUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeFinishedProduct, ItemID: cmd.ProductID, ItemName: productName, SpecG: cmd.SpecG, Warehouse: cmd.ToWarehouse,
		SourceDocType: sourceFinishedTransfer, SourceDocID: transferID, SourceBatchCode: transferNo, SourceBatchID: transferNo,
		BeforeG: beforeToG, ChangeG: transferG, AfterG: beforeToG + transferG, BeforeUnits: beforeToUnits, ChangeUnits: afterToUnits - beforeToUnits, AfterUnits: afterToUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "finished_product_transfer", &transferID, "submit", postgresinfra.StrPtr("qty_g"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", transferG)), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "spec_g": cmd.SpecG, "transfer_no": transferNo, "from": cmd.FromWarehouse, "to": cmd.ToWarehouse}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	return stockapp.FinishedProductTransferResult{TransferID: transferID, TransferNo: transferNo}, nil
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
	stockRemainingG := int64(0)
	stockRemainingUnits := int64(0)
	if cmd.ItemType == itemTypeMaterial && changeG > 0 {
		stockRemainingG = changeG
	}
	if cmd.ItemType == itemTypeFinishedProduct && changeG > 0 {
		stockRemainingG = changeG
		stockRemainingUnits = changeUnits
		if stockRemainingUnits < 0 {
			stockRemainingUnits = 0
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$1,$8,$9,$10,$11,$12,now())
	`, r.schema), batchCode, cmd.ItemType, cmd.ItemID, itemName, cmd.SpecG, sourceStockAdjustment, adjustmentID, changeG, changeUnits, stockRemainingG, stockRemainingUnits, cmd.Operator); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if cmd.ItemType == itemTypeMaterial && changeG > 0 {
		var materialBatchID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_batches(batch_code,material_id,supplier,receipt_id,qty_g,remaining_g,unit_cost,note,received_at,created_at)
			VALUES($1,$2,'stock_adjustment',$3,$4,$4,0,$5,now(),now())
			ON CONFLICT (batch_code) DO UPDATE SET
				remaining_g=excluded.remaining_g,
				status='active',
				note=excluded.note
			RETURNING id
		`, r.schema), batchCode, cmd.ItemID, adjustmentID, changeG, cmd.Reason).Scan(&materialBatchID); err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,updated_at)
			VALUES($1,$2,$3,$4,$5,now())
			ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
				batch_code=excluded.batch_code,
				material_id=excluded.material_id,
				qty_g=excluded.qty_g,
				updated_at=now()
		`, r.schema), materialBatchID, batchCode, cmd.ItemID, cmd.Warehouse, changeG); err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
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
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.Warehouse); err != nil {
		return "", 0, 0, 0, 0, err
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 FOR UPDATE`, r.schema), cmd.ItemID, cmd.SpecG, cmd.Warehouse).Scan(&beforeUnits, &beforeLoose)
	if err != nil && err != pgx.ErrNoRows {
		return "", 0, 0, 0, 0, err
	}
	beforeG := beforeUnits*cmd.SpecG + beforeLoose
	afterG := cmd.TargetUnits*cmd.SpecG + cmd.TargetG
	if err := upsertFinishedInventoryTx(ctx, tx, r.schema, cmd.ItemID, cmd.SpecG, cmd.Warehouse, cmd.TargetUnits, cmd.TargetG); err != nil {
		return "", 0, 0, 0, 0, err
	}
	return name, beforeG, beforeUnits, afterG, cmd.TargetUnits, nil
}

func (r Repository) ensureWarehouseExistsTx(ctx context.Context, tx pgx.Tx, warehouse string) error {
	var exists bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.warehouses WHERE code=$1 AND active=true)`, r.schema), warehouse).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("warehouse not found: %s", warehouse)
	}
	return nil
}

func (r Repository) loadTransferByIdempotencyTx(ctx context.Context, tx pgx.Tx, key string) (stockapp.MaterialTransferResult, bool, error) {
	var result stockapp.MaterialTransferResult
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, transfer_no
		FROM %s.material_transfers
		WHERE idempotency_key=$1
		FOR UPDATE
	`, r.schema), key).Scan(&result.TransferID, &result.TransferNo)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.MaterialTransferResult{}, false, nil
		}
		return stockapp.MaterialTransferResult{}, false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT material_batch_id, material_batch_code, qty_g
		FROM %s.material_transfer_items
		WHERE transfer_id=$1
		ORDER BY id
	`, r.schema), result.TransferID)
	if err != nil {
		return stockapp.MaterialTransferResult{}, false, err
	}
	defer rows.Close()
	result.Allocations = make([]stockapp.MaterialTransferAllocation, 0)
	for rows.Next() {
		var alloc stockapp.MaterialTransferAllocation
		if err := rows.Scan(&alloc.MaterialBatchID, &alloc.BatchCode, &alloc.QtyG); err != nil {
			return stockapp.MaterialTransferResult{}, false, err
		}
		result.Allocations = append(result.Allocations, alloc)
	}
	if err := rows.Err(); err != nil {
		return stockapp.MaterialTransferResult{}, false, err
	}
	return result, true, nil
}

func (r Repository) loadFinishedTransferByIdempotencyTx(ctx context.Context, tx pgx.Tx, key string) (stockapp.FinishedProductTransferResult, bool, error) {
	var result stockapp.FinishedProductTransferResult
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, transfer_no
		FROM %s.finished_product_transfers
		WHERE idempotency_key=$1
		FOR UPDATE
	`, r.schema), key).Scan(&result.TransferID, &result.TransferNo)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.FinishedProductTransferResult{}, false, nil
		}
		return stockapp.FinishedProductTransferResult{}, false, err
	}
	return result, true, nil
}

func materialBatchLocationQtyTx(ctx context.Context, tx pgx.Tx, schema string, batchID int64, warehouse string) (int64, error) {
	var qty int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g
		FROM %s.material_batch_locations
		WHERE material_batch_id=$1 AND warehouse=$2
		FOR UPDATE
	`, schema), batchID, warehouse).Scan(&qty)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return qty, nil
}

func normalizeFinishedQty(specG, units, looseG int64) (int64, int64, int64, error) {
	if specG <= 0 {
		return 0, 0, 0, fmt.Errorf("spec_g required")
	}
	if units < 0 || looseG < 0 {
		return 0, 0, 0, fmt.Errorf("negative qty")
	}
	totalG := units*specG + looseG
	return totalG / specG, totalG % specG, totalG, nil
}

func finishedInventoryQtyTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG int64, warehouse string) (int64, int64, error) {
	var units, looseG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3
		FOR UPDATE
	`, schema), productID, specG, warehouse).Scan(&units, &looseG)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return units, looseG, nil
}

func upsertFinishedInventoryTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG int64, warehouse string, units, looseG int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, schema), productID, specG, warehouse, units, looseG)
	return err
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
