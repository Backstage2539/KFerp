package stock

import (
	"context"
	"fmt"
	"math"
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

func stockSchemaColumnExists(ctx context.Context, pool *pgxpool.Pool, schema, table, column string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
		)
	`, schema, table, column).Scan(&exists)
	return exists, err
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
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.stock_ledger_entries
		WHERE %s
	`, r.schema, strings.Join(where, " AND ")), args...).Scan(&total); err != nil {
		return stockapp.LedgerResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT id,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,
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
		if err := rows.Scan(&row.ID, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.BomSpecID, &row.BomVariantID, &row.Warehouse, &row.SourceDocType, &row.SourceDocID, &row.SourceBatchCode, &row.SourceBatchID, &row.QtyBeforeG, &row.QtyChangeG, &row.QtyAfterG, &row.QtyBeforeUnits, &row.QtyChangeUnits, &row.QtyAfterUnits, &row.Operator, &row.CreatedAt); err != nil {
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
	return stockapp.LedgerResult{Rows: out, Total: total, HasNext: hasNext}, nil
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
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.stock_batches
		WHERE %s
	`, r.schema, strings.Join(where, " AND ")), args...).Scan(&total); err != nil {
		return stockapp.BatchResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT id,batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,
		       source_doc_type,source_doc_id,source_batch_id,
		       qty_g,qty_units,remaining_g,remaining_units,COALESCE(unit_cost,0),
		       COALESCE(quality_status,'unchecked'),
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
		if err := rows.Scan(&row.ID, &row.BatchCode, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.BomSpecID, &row.BomVariantID, &row.SourceDocType, &row.SourceDocID, &row.SourceBatchID, &row.QtyG, &row.QtyUnits, &row.RemainingG, &row.RemainingUnits, &row.UnitCost, &row.QualityStatus, &row.Operator, &row.CreatedAt); err != nil {
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
	return stockapp.BatchResult{Rows: out, Total: total, HasNext: hasNext}, nil
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
		where = append(where, "(b.remaining_g > 0 OR b.remaining_units > 0) AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')")
	}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.material_batches b
		LEFT JOIN %s.materials m ON m.id=b.material_id
		WHERE %s
	`, r.schema, r.schema, strings.Join(where, " AND ")), args...).Scan(&total); err != nil {
		return stockapp.MaterialBatchResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	sql := fmt.Sprintf(`
		SELECT b.id,b.batch_code,b.material_id,COALESCE(m.name,''),b.supplier,b.receipt_id,
		       b.qty_g,b.qty_units,b.remaining_g,b.remaining_units,COALESCE(b.unit_cost,0),
		       COALESCE(b.crop_season,''),COALESCE(b.origin,''),COALESCE(b.producer_flavor_description,''),
		       to_char(b.received_at,'YYYY-MM-DD HH24:MI'),b.status,COALESCE(b.quality_status,'unchecked'),b.note
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
		if err := rows.Scan(&row.ID, &row.BatchCode, &row.MaterialID, &row.MaterialName, &row.Supplier, &row.ReceiptID, &row.QtyG, &row.QtyUnits, &row.RemainingG, &row.RemainingUnits, &row.UnitCost, &row.CropSeason, &row.Origin, &row.ProducerFlavorDescription, &row.ReceivedAt, &row.Status, &row.QualityStatus, &row.Note); err != nil {
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
	return stockapp.MaterialBatchResult{Rows: out, Total: total, HasNext: hasNext}, nil
}

func (r Repository) ListWarehouses(ctx context.Context, query stockapp.WarehouseListQuery) ([]stockapp.WarehouseRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT w.code,w.name,w.kind,w.parent_code,w.sort_order,w.is_default,w.active,w.description,
		       COALESCE(w.customer_id,0), COALESCE(c.name,''),
		       COALESCE(bga.group_id,0), COALESCE(bg.name,''),
		       COALESCE(bga.group_item_id,0), COALESCE(item.name,''),
		       CASE WHEN bga.id IS NULL THEN '' ELSE 'warehouse_inventory' END
		FROM %s.warehouses w
		LEFT JOIN %s.customers c ON c.id=w.customer_id
		LEFT JOIN %s.business_group_assignments bga ON bga.object_ref=w.code AND bga.object_id=0 AND lower(bga.usage_key)='warehouse_inventory' AND lower(bga.object_key)='warehouse'
		LEFT JOIN %s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %s.business_group_items item ON item.id=bga.group_item_id
		WHERE w.active=true
		  AND ($1::bigint=0 OR COALESCE(w.customer_id,0) IN (0, $1::bigint))
		ORDER BY w.sort_order, w.code
	`, r.schema, r.schema, r.schema, r.schema, r.schema), query.CustomerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]stockapp.WarehouseRow, 0)
	for rows.Next() {
		var row stockapp.WarehouseRow
		if err := rows.Scan(&row.Code, &row.Name, &row.Kind, &row.ParentCode, &row.SortOrder, &row.IsDefault, &row.Active, &row.Description, &row.CustomerID, &row.CustomerName, &row.GroupID, &row.GroupName, &row.GroupItemID, &row.GroupItemName, &row.GroupSource); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) BindWarehouseCustomer(ctx context.Context, cmd stockapp.BindWarehouseCustomerCommand) (stockapp.WarehouseRow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var oldCustomerID int64
	var warehouseName string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(customer_id,0), COALESCE(name,'')
		FROM %s.warehouses
		WHERE code=$1 AND active=true
	`, r.schema), cmd.WarehouseCode).Scan(&oldCustomerID, &warehouseName); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.WarehouseRow{}, fmt.Errorf("warehouse not found")
		}
		return stockapp.WarehouseRow{}, err
	}
	if cmd.CustomerID > 0 {
		var exists bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.customers WHERE id=$1 AND active=true)`, r.schema), cmd.CustomerID).Scan(&exists); err != nil {
			return stockapp.WarehouseRow{}, err
		}
		if !exists {
			return stockapp.WarehouseRow{}, fmt.Errorf("customer not found")
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.warehouses
		SET customer_id=$2, updated_at=now()
		WHERE code=$1
	`, r.schema), cmd.WarehouseCode, cmd.CustomerID); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	if oldCustomerID != cmd.CustomerID {
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "warehouse", nil, "update", postgresinfra.StrPtr("customer_id"), postgresinfra.StrPtr(fmt.Sprintf("%d", oldCustomerID)), postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.CustomerID)), postgresinfra.AuditMeta{"warehouse": cmd.WarehouseCode, "warehouse_name": warehouseName}); err != nil {
			return stockapp.WarehouseRow{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.WarehouseRow{}, err
	}
	rows, err := r.ListWarehouses(ctx, stockapp.WarehouseListQuery{})
	if err != nil {
		return stockapp.WarehouseRow{}, err
	}
	for _, row := range rows {
		if row.Code == cmd.WarehouseCode {
			return row, nil
		}
	}
	return stockapp.WarehouseRow{}, fmt.Errorf("warehouse not found")
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
		where = append(where, "(l.qty_g > 0 OR l.qty_units > 0) AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')")
	}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*)::int
		FROM %s.material_batch_locations l
		LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		WHERE %s
	`, r.schema, r.schema, r.schema, strings.Join(where, " AND ")), args...).Scan(&total); err != nil {
		return stockapp.MaterialBatchLocationResult{}, err
	}
	args = append(args, query.Limit+1, query.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.material_batch_id,l.batch_code,l.material_id,COALESCE(m.name,''),l.warehouse,
		       COALESCE(w.name,l.warehouse),l.qty_g,l.qty_units,
		       COALESCE(b.quality_status,'unchecked'),
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
		if err := rows.Scan(&row.MaterialBatchID, &row.BatchCode, &row.MaterialID, &row.MaterialName, &row.Warehouse, &row.WarehouseName, &row.QtyG, &row.QtyUnits, &row.QualityStatus, &row.ReceivedAt, &row.UpdatedAt); err != nil {
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
	return stockapp.MaterialBatchLocationResult{Rows: out, Total: total, HasNext: hasNext}, nil
}

func (r Repository) ListMaterialBalances(ctx context.Context, query stockapp.MaterialBalanceQuery) ([]stockapp.MaterialBalanceRow, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT m.id,COALESCE(m.name,''),$1::text,COALESCE(w.name,$1::text),COALESCE(m.unit,''),
		       COALESCE(SUM(l.qty_g),0)::bigint,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') IN ('hold','reject') THEN 0 ELSE l.qty_g END),0)::bigint,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') IN ('hold','reject') THEN l.qty_g ELSE 0 END),0)::bigint,
		       COALESCE(SUM(l.qty_units),0)::bigint,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') IN ('hold','reject') THEN 0 ELSE l.qty_units END),0)::bigint,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') IN ('hold','reject') THEN l.qty_units ELSE 0 END),0)::bigint
		FROM %s.materials m
		LEFT JOIN %s.material_batch_locations l ON l.material_id=m.id AND l.warehouse=$1
		LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN %s.warehouses w ON w.code=$1
		WHERE m.id = ANY($2::bigint[])
		GROUP BY m.id,m.name,m.unit,w.name
		ORDER BY m.id
	`, r.schema, r.schema, r.schema, r.schema), query.Warehouse, query.MaterialIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]stockapp.MaterialBalanceRow, 0, len(query.MaterialIDs))
	for rows.Next() {
		var row stockapp.MaterialBalanceRow
		if err := rows.Scan(&row.MaterialID, &row.MaterialName, &row.Warehouse, &row.WarehouseName, &row.UnitCode,
			&row.BookG, &row.AvailableG, &row.FrozenG, &row.BookUnits, &row.AvailableUnits, &row.FrozenUnits); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
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
			       0::bigint AS bom_spec_id,
			       0::bigint AS bom_variant_id,
			       ''::text AS bom_spec_name,
			       ''::text AS inventory_unit,
			       l.material_batch_id AS batch_id,
			       l.batch_code AS batch_code,
			       l.qty_g AS qty_g,
			       l.qty_units AS qty_units,
			       COALESCE(b.unit_cost,0) AS unit_cost,
			       COALESCE(b.quality_status,'unchecked') AS quality_status,
			       l.updated_at AS updated_at
			FROM %s.material_batch_locations l
			LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
			LEFT JOIN %s.materials m ON m.id=l.material_id
			LEFT JOIN %s.warehouses w ON w.code=l.warehouse
			WHERE (l.qty_g <> 0 OR l.qty_units <> 0)
			  AND ($1 = '' OR l.batch_code ILIKE $2 OR m.name ILIKE $2)
			  AND ($3 = '' OR l.warehouse = $3)
			  AND ($4 = '' OR $4 = 'material')
			  AND ($7::bigint = 0 OR COALESCE(w.customer_id,0) IN (0, $7::bigint))
			UNION ALL
			SELECT COALESCE(last_ledger.warehouse,'finished_goods') AS warehouse,
			       COALESCE(w.name,COALESCE(last_ledger.warehouse,'finished_goods')) AS warehouse_name,
			       COALESCE(w.kind,'finished') AS warehouse_kind,
			       'finished_product' AS item_type,
			       b.item_id AS item_id,
			       COALESCE(p.name,'') AS item_name,
			       b.spec_g,
			       b.bom_spec_id,
			       b.bom_variant_id,
			       COALESCE(NULLIF(variant.spec_name_snapshot,''),NULLIF(spec.name,''),'') AS bom_spec_name,
			       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),'') AS inventory_unit,
			       b.id AS batch_id,
			       b.batch_code AS batch_code,
			       b.remaining_g AS qty_g,
			       b.remaining_units AS qty_units,
			       COALESCE(b.unit_cost,0) AS unit_cost,
			       COALESCE(b.quality_status,'unchecked') AS quality_status,
			       COALESCE(last_ledger.created_at,b.created_at) AS updated_at
			FROM %s.stock_batches b
			LEFT JOIN %s.products p ON p.id=b.item_id
			LEFT JOIN %s.production_bom_specs spec ON spec.id=b.bom_spec_id
			LEFT JOIN %s.production_bom_version_variants variant
			  ON variant.id=b.bom_variant_id AND variant.bom_spec_id=b.bom_spec_id
			LEFT JOIN LATERAL (
				SELECT l.warehouse,l.created_at
				FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code
				  AND l.item_type=b.item_type
				  AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id
				  AND l.spec_g=b.spec_g
				ORDER BY l.id DESC
				LIMIT 1
			) last_ledger ON true
			LEFT JOIN %s.warehouses w ON w.code=COALESCE(last_ledger.warehouse,'finished_goods')
			WHERE b.item_type='finished_product'
			  AND (b.remaining_g <> 0 OR b.remaining_units <> 0)
			  AND ($1 = '' OR b.batch_code ILIKE $2 OR p.name ILIKE $2 OR COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,'') ILIKE $2)
			  AND ($3 = '' OR COALESCE(last_ledger.warehouse,'finished_goods') = $3)
			  AND ($4 = '' OR $4 = 'finished_product')
			  AND ($7::bigint = 0 OR COALESCE(w.customer_id,0) IN (0, $7::bigint) OR COALESCE(p.customer_id,0) IN (0, $7::bigint))
			UNION ALL
			SELECT fi.warehouse,
			       COALESCE(w.name,fi.warehouse) AS warehouse_name,
			       COALESCE(w.kind,'finished') AS warehouse_kind,
			       'finished_product' AS item_type,
			       fi.product_id AS item_id,
			       COALESCE(p.name,'') AS item_name,
			       fi.spec_g,
			       fi.bom_spec_id,
			       fi.bom_variant_id,
			       COALESCE(NULLIF(variant.spec_name_snapshot,''),NULLIF(spec.name,''),'') AS bom_spec_name,
			       COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),'') AS inventory_unit,
			       0::bigint AS batch_id,
			       '' AS batch_code,
			       (fi.onhand_units * fi.spec_g + fi.onhand_loose_g) AS qty_g,
			       fi.onhand_units AS qty_units,
			       0::numeric AS unit_cost,
			       'unchecked' AS quality_status,
			       fi.updated_at AS updated_at
			FROM %s.finished_inventory fi
			LEFT JOIN %s.products p ON p.id=fi.product_id
			LEFT JOIN %s.production_bom_specs spec ON spec.id=fi.bom_spec_id
			LEFT JOIN %s.production_bom_version_variants variant
			  ON variant.id=fi.bom_variant_id AND variant.bom_spec_id=fi.bom_spec_id
			LEFT JOIN %s.warehouses w ON w.code=fi.warehouse
			WHERE (fi.onhand_units <> 0 OR fi.onhand_loose_g <> 0)
			  AND ($1 = '' OR p.name ILIKE $2 OR COALESCE(NULLIF(variant.spec_name_snapshot,''),spec.name,'') ILIKE $2)
			  AND ($3 = '' OR fi.warehouse = $3)
			  AND ($4 = '' OR $4 = 'finished_product')
			  AND ($7::bigint = 0 OR COALESCE(w.customer_id,0) IN (0, $7::bigint) OR COALESCE(p.customer_id,0) IN (0, $7::bigint))
			  AND NOT EXISTS (
			    SELECT 1
			    FROM %s.stock_batches b
			    WHERE b.item_type='finished_product'
			      AND b.item_id=fi.product_id
			      AND b.bom_spec_id=fi.bom_spec_id
			      AND b.spec_g=fi.spec_g
			      AND (b.remaining_g <> 0 OR b.remaining_units <> 0)
			  )
		)
		SELECT wi.warehouse,wi.warehouse_name,wi.warehouse_kind,
		       COALESCE(bga.group_id,0), COALESCE(bg.name,''),
		       COALESCE(bga.group_item_id,0), COALESCE(item.name,''),
		       CASE WHEN bga.id IS NULL THEN '' ELSE 'warehouse_inventory' END,
		       wi.item_type,wi.item_id,wi.item_name,wi.spec_g,wi.bom_spec_id,wi.bom_variant_id,wi.bom_spec_name,wi.inventory_unit,wi.batch_id,wi.batch_code,
		       wi.qty_g,wi.qty_units,COALESCE(wi.unit_cost,0),wi.quality_status,to_char(wi.updated_at,'YYYY-MM-DD HH24:MI'),
		       count(*) OVER()::int AS total_count
		FROM warehouse_inventory wi
		LEFT JOIN %[1]s.business_group_assignments bga ON bga.object_ref=wi.warehouse AND bga.object_id=0 AND lower(bga.usage_key)='warehouse_inventory' AND lower(bga.object_key)='warehouse'
		LEFT JOIN %[1]s.business_groups bg ON bg.id=bga.group_id
		LEFT JOIN %[1]s.business_group_items item ON item.id=bga.group_item_id
		WHERE ($8::bigint=0 OR bga.group_id=$8::bigint)
		  AND ($9::bigint=0 OR bga.group_item_id=$9::bigint)
		ORDER BY wi.warehouse_name,wi.item_type,wi.item_name,wi.bom_spec_id,wi.spec_g,wi.batch_code
		LIMIT $5 OFFSET $6
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), q, qLike, query.Warehouse, query.ItemType, query.Limit+1, query.Offset, query.CustomerID, query.GroupID, query.GroupItemID)
	if err != nil {
		return stockapp.WarehouseInventoryResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.WarehouseInventoryRow, 0)
	total := 0
	for rows.Next() {
		var row stockapp.WarehouseInventoryRow
		if err := rows.Scan(&row.Warehouse, &row.WarehouseName, &row.WarehouseKind, &row.GroupID, &row.GroupName, &row.GroupItemID, &row.GroupItemName, &row.GroupSource, &row.ItemType, &row.ItemID, &row.ItemName, &row.SpecG, &row.BomSpecID, &row.BomVariantID, &row.BomSpecName, &row.InventoryUnit, &row.BatchID, &row.BatchCode, &row.QtyG, &row.QtyUnits, &row.UnitCost, &row.QualityStatus, &row.UpdatedAt, &total); err != nil {
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
	return stockapp.WarehouseInventoryResult{Rows: out, Total: total, HasNext: hasNext}, nil
}

func (r Repository) ListOutboundLogs(ctx context.Context, query stockapp.OutboundLogQuery) (stockapp.OutboundLogResult, error) {
	q := strings.TrimSpace(query.Q)
	qLike := "%" + q + "%"
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		WITH outbound_logs AS (
			SELECT d.id AS document_id,
			       d.order_id,
			       COALESCE(d.order_no, '') AS order_no,
			       COALESCE(NULLIF(d.snapshot_json->>'customer_company_name', ''), NULLIF(d.snapshot_json->>'customer_name', ''), NULLIF(c.company_name, ''), c.name, '') AS customer_name,
			       COALESCE(NULLIF(d.snapshot_json->>'posting_date', ''), COALESCE(to_char(f.posting_date, 'YYYY-MM-DD'), '')) AS posting_date,
			       COALESCE(NULLIF(d.snapshot_json->>'source_warehouse', ''), NULLIF(f.source_warehouse, ''), 'finished_goods') AS source_warehouse,
			       COALESCE(
			          NULLIF(d.snapshot_json->>'source_warehouse_name', ''),
			          NULLIF(w.name, ''),
			          CASE COALESCE(NULLIF(d.snapshot_json->>'source_warehouse', ''), NULLIF(f.source_warehouse, ''), 'finished_goods')
			             WHEN 'finished_goods' THEN '成品仓'
			             WHEN 'finished_shop' THEN '门店成品仓'
			             ELSE COALESCE(NULLIF(d.snapshot_json->>'source_warehouse', ''), NULLIF(f.source_warehouse, ''), 'finished_goods')
			          END
			       ) AS warehouse_name,
			       CASE COALESCE(NULLIF(d.snapshot_json->>'delivery_method', ''), NULLIF(f.delivery_method, ''), NULLIF(o.ship_method, ''), '')
			          WHEN 'sf_small' THEN '顺丰发货'
			          WHEN 'sf_large' THEN '顺丰大件'
			          WHEN 'sf_express' THEN '顺丰标快'
			          WHEN 'sf_fast' THEN '顺丰特快'
			          WHEN 'sf_cold' THEN '顺丰冷运'
			          ELSE COALESCE(NULLIF(d.snapshot_json->>'delivery_method', ''), NULLIF(f.delivery_method, ''), NULLIF(o.ship_method, ''), '')
			       END AS delivery_method,
			       COALESCE(NULLIF(d.snapshot_json->>'tracking_no', ''), NULLIF(f.tracking_no, ''), o.ship_tracking_no, '') AS tracking_no,
			       d.version_no,
			       d.is_latest,
			       d.created_at AS created_sort,
			       to_char(d.created_at, 'YYYY-MM-DD HH24:MI') AS created_at,
			       COALESCE(d.created_by, '') AS created_by,
			       COALESCE(ps.name, '') AS pay_status,
			       COALESCE(ss.name, '') AS ship_status,
			       COALESCE(ops.name, '') AS process_status,
			       COALESCE(oi.status, '') AS invoice_status
			FROM %s.delivery_note_documents d
			LEFT JOIN %s.orders o ON o.id=d.order_id
			LEFT JOIN %s.customers c ON c.id=o.customer_id
			LEFT JOIN %s.delivery_note_forms f ON f.order_id=d.order_id
			LEFT JOIN %s.warehouses w ON w.code=COALESCE(NULLIF(d.snapshot_json->>'source_warehouse', ''), NULLIF(f.source_warehouse, ''), 'finished_goods')
			LEFT JOIN %s.pay_statuses ps ON ps.id=o.pay_status_id
			LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
			LEFT JOIN %s.order_process_statuses ops ON ops.id=o.process_status_id
			LEFT JOIN %s.order_invoices oi ON oi.order_id=o.id
		)
		SELECT document_id, order_id, order_no, customer_name, posting_date, source_warehouse, warehouse_name,
		       delivery_method, tracking_no, version_no, is_latest, created_at, created_by,
		       pay_status, ship_status, process_status, invoice_status,
		       count(*) OVER()::int AS total_count
		FROM outbound_logs
		WHERE ($1 = '' OR order_no ILIKE $2 OR customer_name ILIKE $2 OR tracking_no ILIKE $2 OR delivery_method ILIKE $2)
		  AND ($3 = '' OR created_sort >= $3::date)
		  AND ($4 = '' OR created_sort < ($4::date + INTERVAL '1 day'))
		ORDER BY created_sort DESC, document_id DESC
		LIMIT $5 OFFSET $6
	`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema), q, qLike, query.From, query.To, query.Limit+1, query.Offset)
	if err != nil {
		return stockapp.OutboundLogResult{}, err
	}
	defer rows.Close()
	out := make([]stockapp.OutboundLogRow, 0)
	total := 0
	for rows.Next() {
		var row stockapp.OutboundLogRow
		if err := rows.Scan(&row.DocumentID, &row.OrderID, &row.OrderNo, &row.CustomerName, &row.PostingDate, &row.SourceWarehouse, &row.WarehouseName, &row.DeliveryMethod, &row.TrackingNo, &row.VersionNo, &row.IsLatest, &row.CreatedAt, &row.CreatedBy, &row.PayStatus, &row.ShipStatus, &row.ProcessStatus, &row.InvoiceStatus, &total); err != nil {
			return stockapp.OutboundLogResult{}, err
		}
		row.DownloadURL = fmt.Sprintf("/orders/%d/delivery-notes/%d.pdf", row.OrderID, row.DocumentID)
		row.LatestURL = fmt.Sprintf("/orders/%d/delivery-note-latest.pdf", row.OrderID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return stockapp.OutboundLogResult{}, err
	}
	hasNext := false
	if len(out) > query.Limit {
		out = out[:query.Limit]
		hasNext = true
	}
	return stockapp.OutboundLogResult{Rows: out, Total: total, HasNext: hasNext}, nil
}

func (r Repository) GetStockTrace(ctx context.Context, query stockapp.StockTraceQuery) (stockapp.StockTraceResult, error) {
	var result stockapp.StockTraceResult
	bomSelect := "0::bigint,0::bigint"
	bomJoin := ""
	hasBatchBomSpec, err := stockSchemaColumnExists(ctx, r.pool, r.schema, "stock_batches", "bom_spec_id")
	if err != nil {
		return stockapp.StockTraceResult{}, err
	}
	hasBatchBomVariant, err := stockSchemaColumnExists(ctx, r.pool, r.schema, "stock_batches", "bom_variant_id")
	if err != nil {
		return stockapp.StockTraceResult{}, err
	}
	hasLedgerBomSpec, err := stockSchemaColumnExists(ctx, r.pool, r.schema, "stock_ledger_entries", "bom_spec_id")
	if err != nil {
		return stockapp.StockTraceResult{}, err
	}
	if hasBatchBomSpec && hasBatchBomVariant {
		bomSelect = "b.bom_spec_id,b.bom_variant_id"
	}
	if hasBatchBomSpec && hasLedgerBomSpec {
		bomJoin = "AND l.bom_spec_id=b.bom_spec_id"
	}
	err = r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.batch_code,b.item_id,b.item_name,b.spec_g,%s,COALESCE(l.warehouse,'finished_goods'),
		       b.qty_g,b.qty_units,b.remaining_g,b.remaining_units,COALESCE(b.quality_status,'unchecked'),to_char(b.created_at,'YYYY-MM-DD HH24:MI')
		FROM %s.stock_batches b
		LEFT JOIN %s.stock_ledger_entries l
		  ON l.source_doc_type=b.source_doc_type
		 AND l.source_doc_id=b.source_doc_id
		 AND l.item_type=b.item_type
		 AND l.item_id=b.item_id
		 %s
		 AND l.spec_g=b.spec_g
		 AND l.source_batch_code=b.batch_code
		WHERE b.batch_code=$1
		  AND b.item_type=$2
		ORDER BY l.id NULLS LAST
		LIMIT 1
	`, bomSelect, r.schema, r.schema, bomJoin), query.BatchCode, itemTypeFinishedProduct).Scan(
		&result.FinishedBatch.BatchCode,
		&result.FinishedBatch.ProductID,
		&result.FinishedBatch.ProductName,
		&result.FinishedBatch.SpecG,
		&result.FinishedBatch.BomSpecID,
		&result.FinishedBatch.BomVariantID,
		&result.FinishedBatch.Warehouse,
		&result.FinishedBatch.QtyG,
		&result.FinishedBatch.QtyUnits,
		&result.FinishedBatch.RemainingG,
		&result.FinishedBatch.RemainingUnits,
		&result.FinishedBatch.QualityStatus,
		&result.FinishedBatch.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return r.getMaterialStockTrace(ctx, query.BatchCode)
		}
		return stockapp.StockTraceResult{}, err
	}
	result.TraceType = "finished_batch"

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
		SELECT l.material_id,COALESCE(l.material_name,''),COALESCE(l.unit,''),l.deduct_g,l.deduct_units,
		       COALESCE(NULLIF(l.material_batch_id,0), mb.id, 0),
		       COALESCE(NULLIF(l.material_batch_code,''), NULLIF(ledger_material.source_batch_code,''), '')
		FROM %s.material_consumption_logs l
		LEFT JOIN LATERAL (
			SELECT sle.source_batch_code
			FROM %s.stock_ledger_entries sle
			WHERE sle.source_doc_type='production_run'
			  AND sle.source_doc_id=$1
			  AND sle.item_type='material'
			  AND sle.item_id=l.material_id
			  AND sle.qty_change_g < 0
			  AND COALESCE(sle.source_batch_code,'') <> ''
			ORDER BY ABS(sle.qty_change_g) DESC, sle.id
			LIMIT 1
		) ledger_material ON true
		LEFT JOIN %s.material_batches mb ON mb.batch_code=ledger_material.source_batch_code
		WHERE l.running_item_id=$1
		ORDER BY l.id
	`, r.schema, r.schema, r.schema), runningItemID)
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

func (r Repository) getMaterialStockTrace(ctx context.Context, batchCode string) (stockapp.StockTraceResult, error) {
	var result stockapp.StockTraceResult
	result.TraceType = "material_batch"
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,b.material_id,COALESCE(m.name,''),b.supplier,b.receipt_id,
		       b.qty_g,b.qty_units,b.remaining_g,b.remaining_units,COALESCE(b.unit_cost,0),
		       COALESCE(b.crop_season,''),COALESCE(b.origin,''),COALESCE(b.producer_flavor_description,''),
		       to_char(b.received_at,'YYYY-MM-DD HH24:MI'),b.status,COALESCE(b.quality_status,'unchecked'),b.note
		FROM %s.material_batches b
		LEFT JOIN %s.materials m ON m.id=b.material_id
		WHERE b.batch_code=$1
		LIMIT 1
	`, r.schema, r.schema), batchCode).Scan(
		&result.MaterialBatch.ID,
		&result.MaterialBatch.BatchCode,
		&result.MaterialBatch.MaterialID,
		&result.MaterialBatch.MaterialName,
		&result.MaterialBatch.Supplier,
		&result.MaterialBatch.ReceiptID,
		&result.MaterialBatch.QtyG,
		&result.MaterialBatch.QtyUnits,
		&result.MaterialBatch.RemainingG,
		&result.MaterialBatch.RemainingUnits,
		&result.MaterialBatch.UnitCost,
		&result.MaterialBatch.CropSeason,
		&result.MaterialBatch.Origin,
		&result.MaterialBatch.ProducerFlavorDescription,
		&result.MaterialBatch.ReceivedAt,
		&result.MaterialBatch.Status,
		&result.MaterialBatch.QualityStatus,
		&result.MaterialBatch.Note,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.StockTraceResult{}, fmt.Errorf("batch not found")
		}
		return stockapp.StockTraceResult{}, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.material_batch_id,l.batch_code,l.material_id,COALESCE(m.name,''),l.warehouse,
		       COALESCE(w.name,l.warehouse),l.qty_g,l.qty_units,
		       COALESCE(b.quality_status,'unchecked'),
		       COALESCE(to_char(b.received_at,'YYYY-MM-DD HH24:MI'),''),
		       to_char(l.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.material_batch_locations l
		LEFT JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN %s.materials m ON m.id=l.material_id
		LEFT JOIN %s.warehouses w ON w.code=l.warehouse
		WHERE l.batch_code=$1
		  AND (l.qty_g <> 0 OR l.qty_units <> 0)
		ORDER BY l.warehouse, l.material_batch_id
	`, r.schema, r.schema, r.schema, r.schema), batchCode)
	if err != nil {
		return stockapp.StockTraceResult{}, err
	}
	defer rows.Close()
	result.MaterialLocations = make([]stockapp.MaterialBatchLocationRow, 0)
	for rows.Next() {
		var row stockapp.MaterialBatchLocationRow
		if err := rows.Scan(&row.MaterialBatchID, &row.BatchCode, &row.MaterialID, &row.MaterialName, &row.Warehouse, &row.WarehouseName, &row.QtyG, &row.QtyUnits, &row.QualityStatus, &row.ReceivedAt, &row.UpdatedAt); err != nil {
			return stockapp.StockTraceResult{}, err
		}
		result.MaterialLocations = append(result.MaterialLocations, row)
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
	if err := r.assertMaterialCanUseOrdinaryReceiptTx(ctx, tx, cmd.MaterialID); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}

	var materialName, materialUnit string
	var beforeG, beforeUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),COALESCE(unit,''),onhand_g,onhand_units FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.MaterialID).Scan(&materialName, &materialUnit, &beforeG, &beforeUnits); err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.MaterialReceiptResult{}, fmt.Errorf("material not found")
		}
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := validateMaterialReceiptUnitAndQuantity(materialName, materialUnit, cmd.UnitCode, cmd.QtyG, cmd.QtyUnits); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	cmd.UnitCode = materialUnit
	afterG := beforeG + cmd.QtyG
	afterUnits := beforeUnits + cmd.QtyUnits
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1`, r.schema), cmd.MaterialID, afterG, afterUnits); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}

	var receiptID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_receipts(material_id,supplier,qty_g,qty_units,unit_cost,crop_season,origin,producer_flavor_description,note,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())
		RETURNING id
	`, r.schema), cmd.MaterialID, cmd.Supplier, cmd.QtyG, cmd.QtyUnits, cmd.UnitCost, cmd.CropSeason, cmd.Origin, cmd.ProducerFlavorDescription, cmd.Note, cmd.Operator).Scan(&receiptID); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	batchCode := fmt.Sprintf("MB-%010d", receiptID)
	var batchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(batch_code,material_id,supplier,receipt_id,qty_g,qty_units,remaining_g,remaining_units,unit_cost,crop_season,origin,producer_flavor_description,note,received_at,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$5,$6,$7,$8,$9,$10,$11,now(),now())
		RETURNING id
	`, r.schema), batchCode, cmd.MaterialID, cmd.Supplier, receiptID, cmd.QtyG, cmd.QtyUnits, cmd.UnitCost, cmd.CropSeason, cmd.Origin, cmd.ProducerFlavorDescription, cmd.Note).Scan(&batchID); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,unit_cost,operator,created_at
		) VALUES($1,$2,$3,$4,0,$5,$6,$1,$7,$8,$7,$8,$9,$10,now())
	`, r.schema), batchCode, itemTypeMaterial, cmd.MaterialID, materialName, sourceMaterialReceipt, receiptID, cmd.QtyG, cmd.QtyUnits, cmd.UnitCost, cmd.Operator); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
			batch_code=excluded.batch_code,
			material_id=excluded.material_id,
			qty_g=material_batch_locations.qty_g+excluded.qty_g,
			qty_units=material_batch_locations.qty_units+excluded.qty_units,
			updated_at=now()
	`, r.schema), batchID, batchCode, cmd.MaterialID, stockdomain.WarehouseRawMaterials, cmd.QtyG, cmd.QtyUnits); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeMaterial, ItemID: cmd.MaterialID, ItemName: materialName, Warehouse: stockdomain.WarehouseRawMaterials,
		SourceDocType: sourceMaterialReceipt, SourceDocID: receiptID, SourceBatchCode: batchCode, SourceBatchID: batchCode,
		BeforeG: beforeG, ChangeG: cmd.QtyG, AfterG: afterG, BeforeUnits: beforeUnits, ChangeUnits: cmd.QtyUnits, AfterUnits: afterUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.MaterialReceiptResult{}, err
	}
	auditField := "qty_g"
	auditNewValue := fmt.Sprintf("%d", cmd.QtyG)
	if cmd.QtyG == 0 && cmd.QtyUnits > 0 {
		auditField = "qty_units"
		auditNewValue = fmt.Sprintf("%d", cmd.QtyUnits)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "material_receipt", &receiptID, "submit", postgresinfra.StrPtr(auditField), nil, postgresinfra.StrPtr(auditNewValue), postgresinfra.AuditMeta{"material_id": cmd.MaterialID, "batch_code": batchCode, "qty_g": cmd.QtyG, "qty_units": cmd.QtyUnits, "unit_code": cmd.UnitCode}); err != nil {
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
		SELECT b.id,b.batch_code,l.qty_g,COALESCE(b.quality_status,'unchecked')
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
	var frozenG int64
	for rows.Next() {
		var batch stockdomain.BatchAvailability
		var qualityStatus string
		if err := rows.Scan(&batch.BatchID, &batch.BatchCode, &batch.AvailableG, &qualityStatus); err != nil {
			return stockapp.MaterialTransferResult{}, err
		}
		if qualityStatus == "hold" || qualityStatus == "reject" {
			frozenG += batch.AvailableG
			continue
		}
		available = append(available, batch)
		beforeFromByBatch[batch.BatchID] = batch.AvailableG
	}
	if err := rows.Err(); err != nil {
		return stockapp.MaterialTransferResult{}, err
	}
	allocations, err := stockdomain.AllocateFIFO(available, cmd.QtyG)
	if err != nil {
		if frozenG > 0 {
			return stockapp.MaterialTransferResult{}, fmt.Errorf("material stock blocked by quality status in %s", cmd.FromWarehouse)
		}
		return stockapp.MaterialTransferResult{}, err
	}
	if len(allocations) == 0 {
		if frozenG > 0 {
			return stockapp.MaterialTransferResult{}, fmt.Errorf("material stock blocked by quality status in %s", cmd.FromWarehouse)
		}
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

	identity, err := resolveFinishedProductBomSpecIdentityTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, cmd.UnitCode)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if cmd.BomSpecID > 0 {
		cmd.BomVariantID = identity.BomVariantID
		cmd.UnitCode = identity.InventoryUnit
	}
	transferUnits, transferLoose, transferG := cmd.QtyUnits, cmd.QtyLooseG, int64(0)
	if cmd.BomSpecID == 0 {
		transferUnits, transferLoose, transferG, err = normalizeFinishedQty(cmd.SpecG, cmd.QtyUnits, cmd.QtyLooseG)
		if err != nil {
			return stockapp.FinishedProductTransferResult{}, err
		}
	}
	beforeFromUnits, beforeFromLoose, err := finishedInventoryIdentityQtyTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.SpecG, cmd.FromWarehouse)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeFromG := beforeFromUnits*cmd.SpecG + beforeFromLoose
	if (cmd.BomSpecID > 0 && beforeFromUnits < transferUnits) || (cmd.BomSpecID == 0 && beforeFromG < transferG) {
		return stockapp.FinishedProductTransferResult{}, fmt.Errorf("finished stock insufficient in %s", cmd.FromWarehouse)
	}
	qualityAvailableG, qualityAvailableUnits, hasQualityBatches, err := availableFinishedQualityIdentityTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.SpecG, cmd.FromWarehouse)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if hasQualityBatches && ((cmd.BomSpecID > 0 && qualityAvailableUnits < transferUnits) || (cmd.BomSpecID == 0 && qualityAvailableG < transferG)) {
		return stockapp.FinishedProductTransferResult{}, fmt.Errorf("finished stock blocked by quality status in %s", cmd.FromWarehouse)
	}
	afterFromUnits, afterFromLoose := beforeFromUnits-transferUnits, int64(0)
	if cmd.BomSpecID == 0 {
		afterFromUnits, afterFromLoose, _, err = normalizeFinishedQty(cmd.SpecG, 0, beforeFromG-transferG)
		if err != nil {
			return stockapp.FinishedProductTransferResult{}, err
		}
	}
	beforeToUnits, beforeToLoose, err := finishedInventoryIdentityQtyTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.SpecG, cmd.ToWarehouse)
	if err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	beforeToG := beforeToUnits*cmd.SpecG + beforeToLoose
	afterToUnits, afterToLoose := beforeToUnits+transferUnits, int64(0)
	if cmd.BomSpecID == 0 {
		afterToUnits, afterToLoose, _, err = normalizeFinishedQty(cmd.SpecG, 0, beforeToG+transferG)
		if err != nil {
			return stockapp.FinishedProductTransferResult{}, err
		}
	}

	var transferID int64
	transferNo := ""
	tempTransferNo := fmt.Sprintf("FT-TMP-%d", time.Now().UnixNano())
	if cmd.IdempotencyKey != "" {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_product_transfers(
				transfer_no,product_id,product_name,spec_g,bom_spec_id,bom_variant_id,from_warehouse,to_warehouse,
				qty_g,qty_units,qty_loose_g,note,operator,idempotency_key,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.ProductID, productName, cmd.SpecG, cmd.BomSpecID, cmd.BomVariantID, cmd.FromWarehouse, cmd.ToWarehouse, transferG, transferUnits, transferLoose, cmd.Note, cmd.Operator, cmd.IdempotencyKey).Scan(&transferID, &transferNo)
	} else {
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_product_transfers(
				transfer_no,product_id,product_name,spec_g,bom_spec_id,bom_variant_id,from_warehouse,to_warehouse,
				qty_g,qty_units,qty_loose_g,note,operator,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now())
			RETURNING id, transfer_no
		`, r.schema), tempTransferNo, cmd.ProductID, productName, cmd.SpecG, cmd.BomSpecID, cmd.BomVariantID, cmd.FromWarehouse, cmd.ToWarehouse, transferG, transferUnits, transferLoose, cmd.Note, cmd.Operator).Scan(&transferID, &transferNo)
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

	if err := upsertFinishedInventoryIdentityTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, cmd.SpecG, cmd.FromWarehouse, afterFromUnits, afterFromLoose); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := upsertFinishedInventoryIdentityTx(ctx, tx, r.schema, cmd.ProductID, cmd.BomSpecID, cmd.BomVariantID, cmd.SpecG, cmd.ToWarehouse, afterToUnits, afterToLoose); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeFinishedProduct, ItemID: cmd.ProductID, ItemName: productName, SpecG: cmd.SpecG, BomSpecID: cmd.BomSpecID, BomVariantID: cmd.BomVariantID, Warehouse: cmd.FromWarehouse,
		SourceDocType: sourceFinishedTransfer, SourceDocID: transferID, SourceBatchCode: transferNo, SourceBatchID: transferNo,
		BeforeG: beforeFromG, ChangeG: -transferG, AfterG: beforeFromG - transferG, BeforeUnits: beforeFromUnits, ChangeUnits: afterFromUnits - beforeFromUnits, AfterUnits: afterFromUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: itemTypeFinishedProduct, ItemID: cmd.ProductID, ItemName: productName, SpecG: cmd.SpecG, BomSpecID: cmd.BomSpecID, BomVariantID: cmd.BomVariantID, Warehouse: cmd.ToWarehouse,
		SourceDocType: sourceFinishedTransfer, SourceDocID: transferID, SourceBatchCode: transferNo, SourceBatchID: transferNo,
		BeforeG: beforeToG, ChangeG: transferG, AfterG: beforeToG + transferG, BeforeUnits: beforeToUnits, ChangeUnits: afterToUnits - beforeToUnits, AfterUnits: afterToUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	auditField, auditValue := "qty_g", fmt.Sprintf("%d", transferG)
	if cmd.BomSpecID > 0 {
		auditField, auditValue = "qty_units", fmt.Sprintf("%d", transferUnits)
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "finished_product_transfer", &transferID, "submit", postgresinfra.StrPtr(auditField), nil, postgresinfra.StrPtr(auditValue), postgresinfra.AuditMeta{"product_id": cmd.ProductID, "bom_spec_id": cmd.BomSpecID, "bom_variant_id": cmd.BomVariantID, "inventory_unit": cmd.UnitCode, "spec_g": cmd.SpecG, "transfer_no": transferNo, "from": cmd.FromWarehouse, "to": cmd.ToWarehouse}); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.FinishedProductTransferResult{}, err
	}
	return stockapp.FinishedProductTransferResult{TransferID: transferID, TransferNo: transferNo, ProductID: cmd.ProductID, SpecG: cmd.SpecG, BomSpecID: cmd.BomSpecID, BomVariantID: cmd.BomVariantID}, nil
}

type stockAdjustmentBatchAllocation struct {
	MaterialBatchID int64
	BatchCode       string
	Warehouse       string
	QtyChangeG      int64
	QtyChangeUnits  int64
	UnitCost        float64
}

func (r Repository) CreateAdjustment(ctx context.Context, cmd stockapp.StockAdjustmentCommand) (stockapp.StockAdjustmentResult, error) {
	if cmd.AdjustmentType == "material_cost" {
		return r.createMaterialCostAdjustment(ctx, cmd)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if cmd.ItemType == itemTypeFinishedProduct {
		identity, err := resolveFinishedProductBomSpecIdentityTx(ctx, tx, r.schema, cmd.ItemID, cmd.BomSpecID, cmd.BomVariantID, cmd.UnitCode)
		if err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
		if cmd.BomSpecID > 0 {
			cmd.BomVariantID = identity.BomVariantID
			cmd.UnitCode = identity.InventoryUnit
		}
	}

	itemName, beforeG, beforeUnits, afterG, afterUnits, allocations, err := r.applyAdjustmentTx(ctx, tx, cmd)
	if err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	changeG := afterG - beforeG
	changeUnits := afterUnits - beforeUnits
	var adjustmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustments(adjustment_type,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,reason,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())
		RETURNING id
	`, r.schema), "quantity", cmd.ItemType, cmd.ItemID, itemName, cmd.SpecG, cmd.BomSpecID, cmd.BomVariantID, cmd.Warehouse, cmd.Reason, cmd.Operator).Scan(&adjustmentID); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustment_items(adjustment_id,item_type,item_id,spec_g,bom_spec_id,bom_variant_id,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, r.schema), adjustmentID, cmd.ItemType, cmd.ItemID, cmd.SpecG, cmd.BomSpecID, cmd.BomVariantID, beforeG, changeG, afterG, beforeUnits, changeUnits, afterUnits); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	batchCode := fmt.Sprintf("ADJ-%010d", adjustmentID)
	stockRemainingG := int64(0)
	stockRemainingUnits := int64(0)
	if cmd.ItemType == itemTypeMaterial && changeG > 0 {
		stockRemainingG = changeG
	}
	if cmd.ItemType == itemTypeMaterial && changeUnits > 0 {
		stockRemainingUnits = changeUnits
	}
	if cmd.ItemType == itemTypeFinishedProduct && (changeG > 0 || changeUnits > 0) {
		stockRemainingG = changeG
		stockRemainingUnits = changeUnits
		if stockRemainingUnits < 0 {
			stockRemainingUnits = 0
		}
	}
	unitCost := 0.0
	if cmd.ItemType == itemTypeMaterial && (changeG > 0 || changeUnits > 0) {
		unitCost, err = r.materialAdjustmentUnitCostTx(ctx, tx, cmd)
		if err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,unit_cost,operator,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$1,$10,$11,$12,$13,$14,$15,now())
	`, r.schema), batchCode, cmd.ItemType, cmd.ItemID, itemName, cmd.SpecG, cmd.BomSpecID, cmd.BomVariantID, sourceStockAdjustment, adjustmentID, changeG, changeUnits, stockRemainingG, stockRemainingUnits, unitCost, cmd.Operator); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if cmd.ItemType == itemTypeMaterial && (changeG > 0 || changeUnits > 0) {
		var materialBatchID int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_batches(batch_code,material_id,supplier,receipt_id,qty_g,qty_units,remaining_g,remaining_units,unit_cost,note,received_at,created_at)
			VALUES($1,$2,'stock_adjustment',$3,$4,$5,$4,$5,$6,$7,now(),now())
			ON CONFLICT (batch_code) DO UPDATE SET
				qty_g=excluded.qty_g,
				qty_units=excluded.qty_units,
				remaining_g=excluded.remaining_g,
				remaining_units=excluded.remaining_units,
				unit_cost=excluded.unit_cost,
				status='active',
				note=excluded.note
			RETURNING id
		`, r.schema), batchCode, cmd.ItemID, adjustmentID, changeG, changeUnits, unitCost, cmd.Reason).Scan(&materialBatchID); err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,now())
			ON CONFLICT (material_batch_id, warehouse) DO UPDATE SET
				batch_code=excluded.batch_code,
				material_id=excluded.material_id,
				qty_g=excluded.qty_g,
				qty_units=excluded.qty_units,
				updated_at=now()
		`, r.schema), materialBatchID, batchCode, cmd.ItemID, cmd.Warehouse, changeG, changeUnits); err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
		allocations = append(allocations, stockAdjustmentBatchAllocation{
			MaterialBatchID: materialBatchID, BatchCode: batchCode, Warehouse: cmd.Warehouse,
			QtyChangeG: changeG, QtyChangeUnits: changeUnits, UnitCost: unitCost,
		})
	}
	if cmd.ItemType == itemTypeMaterial {
		if err := r.recomputeMaterialOnhandTx(ctx, tx, cmd.ItemID); err != nil {
			return stockapp.StockAdjustmentResult{}, err
		}
		for _, allocation := range allocations {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.stock_adjustment_batch_allocations(
					adjustment_id,material_batch_id,batch_code,warehouse,qty_change_g,qty_change_units,unit_cost,created_at
				) VALUES($1,$2,$3,$4,$5,$6,$7,now())
			`, r.schema), adjustmentID, allocation.MaterialBatchID, allocation.BatchCode, allocation.Warehouse,
				allocation.QtyChangeG, allocation.QtyChangeUnits, allocation.UnitCost); err != nil {
				return stockapp.StockAdjustmentResult{}, err
			}
		}
	}
	if err := insertLedgerTx(ctx, tx, r.schema, ledgerEntry{
		ItemType: cmd.ItemType, ItemID: cmd.ItemID, ItemName: itemName, SpecG: cmd.SpecG, BomSpecID: cmd.BomSpecID, BomVariantID: cmd.BomVariantID, Warehouse: cmd.Warehouse,
		SourceDocType: sourceStockAdjustment, SourceDocID: adjustmentID, SourceBatchCode: batchCode,
		BeforeG: beforeG, ChangeG: changeG, AfterG: afterG, BeforeUnits: beforeUnits, ChangeUnits: changeUnits, AfterUnits: afterUnits,
		Operator: cmd.Operator,
	}); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	auditField := postgresinfra.StrPtr("qty_g")
	auditBefore, auditAfter := beforeG, afterG
	if (cmd.ItemType == itemTypeMaterial && changeG == 0) || (cmd.ItemType == itemTypeFinishedProduct && cmd.BomSpecID > 0) {
		auditField, auditBefore, auditAfter = postgresinfra.StrPtr("qty_units"), beforeUnits, afterUnits
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "stock_adjustment", &adjustmentID, "submit", auditField, postgresinfra.StrPtr(fmt.Sprintf("%d", auditBefore)), postgresinfra.StrPtr(fmt.Sprintf("%d", auditAfter)), postgresinfra.AuditMeta{
		"adjustment_type": "quantity",
		"item_type":       cmd.ItemType,
		"item_id":         cmd.ItemID,
		"item_name":       itemName,
		"spec_g":          cmd.SpecG,
		"bom_spec_id":     cmd.BomSpecID,
		"bom_variant_id":  cmd.BomVariantID,
		"warehouse":       cmd.Warehouse,
		"reason":          cmd.Reason,
		"batch_code":      batchCode,
		"before_g":        beforeG,
		"change_g":        changeG,
		"after_g":         afterG,
		"before_units":    beforeUnits,
		"change_units":    changeUnits,
		"after_units":     afterUnits,
		"target_qty":      cmd.TargetQty,
		"unit_code":       cmd.UnitCode,
	}); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	result := stockapp.StockAdjustmentResult{AdjustmentID: adjustmentID}
	if cmd.ItemType == itemTypeFinishedProduct {
		result.ProductID = cmd.ItemID
		result.SpecG = cmd.SpecG
		result.BomSpecID = cmd.BomSpecID
		result.BomVariantID = cmd.BomVariantID
	}
	return result, nil
}

func (r Repository) createMaterialCostAdjustment(ctx context.Context, cmd stockapp.StockAdjustmentCommand) (stockapp.StockAdjustmentResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var itemName, batchCode, materialUnit, batchWarehouses string
	var remainingG, remainingUnits int64
	var beforeCost float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(m.name,''), b.batch_code, b.remaining_g, b.remaining_units,COALESCE(m.unit,''),
		       COALESCE((
		           SELECT string_agg(l.warehouse, ',' ORDER BY l.warehouse)
		           FROM %s.material_batch_locations l
		           WHERE l.material_batch_id=b.id AND (l.qty_g>0 OR l.qty_units>0)
		       ),''),
		       COALESCE(b.unit_cost,0)::float8
		FROM %s.material_batches b
		JOIN %s.materials m ON m.id=b.material_id
		WHERE b.id=$1 AND b.material_id=$2
		  AND (b.remaining_g > 0 OR b.remaining_units > 0)
		  AND b.status='active'
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		FOR UPDATE OF b
	`, r.schema, r.schema, r.schema), cmd.MaterialBatchID, cmd.ItemID).Scan(&itemName, &batchCode, &remainingG, &remainingUnits, &materialUnit, &batchWarehouses, &beforeCost); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if remainingG <= 0 && remainingUnits <= 0 {
		return stockapp.StockAdjustmentResult{}, fmt.Errorf("material batch has no remaining stock")
	}

	remainingQty := float64(remainingUnits)
	if remainingG > 0 {
		factor := stockWeightUnitGrams(materialUnit)
		if factor <= 0 {
			factor = 1000
		}
		remainingQty = float64(remainingG) / factor
	}
	valueChange := (cmd.TargetUnitCost - beforeCost) * remainingQty
	if strings.TrimSpace(cmd.Warehouse) == "" {
		cmd.Warehouse = batchWarehouses
	}
	var adjustmentID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustments(adjustment_type,item_type,item_id,item_name,spec_g,warehouse,reason,operator,material_batch_id,unit_cost_before,unit_cost_after,value_change,created_at)
		VALUES($1,$2,$3,$4,0,$5,$6,$7,$8,$9,$10,$11,now())
		RETURNING id
	`, r.schema), "material_cost", itemTypeMaterial, cmd.ItemID, itemName, cmd.Warehouse, cmd.Reason, cmd.Operator, cmd.MaterialBatchID, beforeCost, cmd.TargetUnitCost, valueChange).Scan(&adjustmentID); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.material_batches
		SET unit_cost=$2
		WHERE id=$1
	`, r.schema), cmd.MaterialBatchID, cmd.TargetUnitCost); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches
		SET unit_cost=$2
		WHERE item_type=$3 AND batch_code=$1
	`, r.schema), batchCode, cmd.TargetUnitCost, itemTypeMaterial); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_adjustment_items(adjustment_id,item_type,item_id,spec_g,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units)
		VALUES($1,$2,$3,0,$4,0,$4,$5,0,$5)
	`, r.schema), adjustmentID, itemTypeMaterial, cmd.ItemID, remainingG, remainingUnits); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "stock_adjustment", &adjustmentID, "submit", postgresinfra.StrPtr("unit_cost"), postgresinfra.StrPtr(fmt.Sprintf("%.4f", beforeCost)), postgresinfra.StrPtr(fmt.Sprintf("%.4f", cmd.TargetUnitCost)), postgresinfra.AuditMeta{
		"adjustment_type":   "material_cost",
		"item_type":         itemTypeMaterial,
		"item_id":           cmd.ItemID,
		"item_name":         itemName,
		"warehouse":         cmd.Warehouse,
		"reason":            cmd.Reason,
		"material_batch_id": cmd.MaterialBatchID,
		"batch_code":        batchCode,
		"remaining_g":       remainingG,
		"remaining_units":   remainingUnits,
		"unit_code":         materialUnit,
		"value_change":      valueChange,
	}); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return stockapp.StockAdjustmentResult{}, err
	}
	return stockapp.StockAdjustmentResult{AdjustmentID: adjustmentID}, nil
}

func (r Repository) materialAdjustmentUnitCostTx(ctx context.Context, tx pgx.Tx, cmd stockapp.StockAdjustmentCommand) (float64, error) {
	if cmd.TargetUnitCost > 0 {
		return cmd.TargetUnitCost, nil
	}
	var unitCost float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(purchase_price,0)::float8
		FROM %s.materials
		WHERE id=$1
	`, r.schema), cmd.ItemID).Scan(&unitCost); err != nil {
		return 0, err
	}
	return unitCost, nil
}

func (r Repository) applyAdjustmentTx(ctx context.Context, tx pgx.Tx, cmd stockapp.StockAdjustmentCommand) (string, int64, int64, int64, int64, []stockAdjustmentBatchAllocation, error) {
	if cmd.ItemType == itemTypeMaterial {
		var name string
		var materialUnit string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,''),COALESCE(unit,'') FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), cmd.ItemID).Scan(&name, &materialUnit); err != nil {
			return "", 0, 0, 0, 0, nil, err
		}
		if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.Warehouse); err != nil {
			return "", 0, 0, 0, 0, nil, err
		}
		var beforeG, beforeUnits int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(qty_g),0)::bigint,COALESCE(SUM(qty_units),0)::bigint
			FROM %s.material_batch_locations WHERE material_id=$1 AND warehouse=$2
		`, r.schema), cmd.ItemID, cmd.Warehouse).Scan(&beforeG, &beforeUnits); err != nil {
			return "", 0, 0, 0, 0, nil, err
		}
		unitCode := strings.TrimSpace(cmd.UnitCode)
		if strings.TrimSpace(materialUnit) == "" {
			return "", 0, 0, 0, 0, nil, fmt.Errorf("物料“%s”的库存单位为空，请先完善物料档案", name)
		}
		if unitCode != "" && !sameInventoryUnit(unitCode, materialUnit) {
			return "", 0, 0, 0, 0, nil, fmt.Errorf("物料“%s”的盘点库存单位必须与物料档案一致：%s", name, materialUnit)
		}
		if unitCode == "" {
			unitCode = materialUnit
		}
		targetG, targetUnits := cmd.TargetG, cmd.TargetUnits
		if cmd.HasTargetQty {
			if !isWeightUnit(unitCode, "") && math.Abs(cmd.TargetQty-math.Round(cmd.TargetQty)) > 0.000001 {
				return "", 0, 0, 0, 0, nil, fmt.Errorf("物料“%s”按离散单位计量，盘点数量必须为整数", name)
			}
			targetG, targetUnits = r.materialTargetQtyToLegacyTx(ctx, tx, unitCode, cmd.TargetQty)
		} else if stockWeightUnitGrams(materialUnit) > 0 {
			if targetUnits != 0 {
				return "", 0, 0, 0, 0, nil, fmt.Errorf("物料“%s”按重量计量，请填写重量数量", name)
			}
		} else if targetG != 0 {
			return "", 0, 0, 0, 0, nil, fmt.Errorf("物料“%s”按计数计量，请填写计数数量", name)
		}
		if targetG < 0 || targetUnits < 0 {
			return "", 0, 0, 0, 0, nil, fmt.Errorf("target quantity must be >= 0")
		}
		allocations := make([]stockAdjustmentBatchAllocation, 0)
		if targetG < beforeG || targetUnits < beforeUnits {
			var err error
			allocations, err = r.reduceMaterialWarehouseFIFOForAdjustmentTx(ctx, tx, cmd.ItemID, cmd.Warehouse, beforeG-targetG, beforeUnits-targetUnits)
			if err != nil {
				return "", 0, 0, 0, 0, nil, err
			}
		}
		return name, beforeG, beforeUnits, targetG, targetUnits, allocations, nil
	}

	var name string
	var beforeUnits, beforeLoose int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), cmd.ItemID).Scan(&name); err != nil {
		return "", 0, 0, 0, 0, nil, err
	}
	if err := r.ensureWarehouseExistsTx(ctx, tx, cmd.Warehouse); err != nil {
		return "", 0, 0, 0, 0, nil, err
	}
	beforeUnits, beforeLoose, err := finishedInventoryIdentityQtyTx(ctx, tx, r.schema, cmd.ItemID, cmd.BomSpecID, cmd.SpecG, cmd.Warehouse)
	if err != nil {
		return "", 0, 0, 0, 0, nil, err
	}
	beforeG := beforeUnits*cmd.SpecG + beforeLoose
	afterG := cmd.TargetUnits*cmd.SpecG + cmd.TargetG
	if cmd.BomSpecID > 0 {
		beforeG, afterG = 0, 0
	}
	if err := upsertFinishedInventoryIdentityTx(ctx, tx, r.schema, cmd.ItemID, cmd.BomSpecID, cmd.BomVariantID, cmd.SpecG, cmd.Warehouse, cmd.TargetUnits, cmd.TargetG); err != nil {
		return "", 0, 0, 0, 0, nil, err
	}
	return name, beforeG, beforeUnits, afterG, cmd.TargetUnits, nil, nil
}

func (r Repository) reduceMaterialWarehouseFIFOForAdjustmentTx(ctx context.Context, tx pgx.Tx, materialID int64, warehouse string, deductG, deductUnits int64) ([]stockAdjustmentBatchAllocation, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT l.material_batch_id,l.batch_code,l.qty_g,l.qty_units,COALESCE(b.unit_cost,0)::float8
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1 AND l.warehouse=$2 AND (l.qty_g>0 OR l.qty_units>0)
		ORDER BY b.received_at,b.id
		FOR UPDATE OF l,b
	`, r.schema, r.schema), materialID, warehouse)
	if err != nil {
		return nil, err
	}
	type location struct {
		batchID int64
		code    string
		qtyG    int64
		units   int64
		cost    float64
	}
	locations := make([]location, 0)
	for rows.Next() {
		var row location
		if err := rows.Scan(&row.batchID, &row.code, &row.qtyG, &row.units, &row.cost); err != nil {
			rows.Close()
			return nil, err
		}
		locations = append(locations, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	allocations := make([]stockAdjustmentBatchAllocation, 0)
	for _, row := range locations {
		if deductG <= 0 && deductUnits <= 0 {
			break
		}
		takeG, takeUnits := int64(0), int64(0)
		if deductG > 0 {
			takeG = minInt64(deductG, row.qtyG)
		}
		if deductUnits > 0 {
			takeUnits = minInt64(deductUnits, row.units)
		}
		if takeG == 0 && takeUnits == 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batch_locations SET qty_g=qty_g-$3,qty_units=qty_units-$4,updated_at=now()
			WHERE material_batch_id=$1 AND warehouse=$2
		`, r.schema), row.batchID, warehouse, takeG, takeUnits); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.material_batches
			SET remaining_g=GREATEST(0,remaining_g-$2),remaining_units=GREATEST(0,remaining_units-$3),
			    status=CASE WHEN GREATEST(0,remaining_g-$2)=0 AND GREATEST(0,remaining_units-$3)=0 THEN 'depleted' ELSE status END
			WHERE id=$1
		`, r.schema), row.batchID, takeG, takeUnits); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.stock_batches
			SET remaining_g=GREATEST(0,remaining_g-$2),remaining_units=GREATEST(0,remaining_units-$3)
			WHERE item_type=$4 AND item_id=$5 AND batch_code=$1
		`, r.schema), row.code, takeG, takeUnits, itemTypeMaterial, materialID); err != nil {
			return nil, err
		}
		deductG -= takeG
		deductUnits -= takeUnits
		allocations = append(allocations, stockAdjustmentBatchAllocation{
			MaterialBatchID: row.batchID, BatchCode: row.code, Warehouse: warehouse,
			QtyChangeG: -takeG, QtyChangeUnits: -takeUnits, UnitCost: row.cost,
		})
	}
	if deductG > 0 || deductUnits > 0 {
		return nil, fmt.Errorf("selected warehouse stock is insufficient for adjustment")
	}
	return allocations, nil
}

func (r Repository) recomputeMaterialOnhandTx(ctx context.Context, tx pgx.Tx, materialID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.materials m SET
			onhand_g=COALESCE((SELECT SUM(qty_g) FROM %s.material_batch_locations WHERE material_id=m.id),0),
			onhand_units=COALESCE((SELECT SUM(qty_units) FROM %s.material_batch_locations WHERE material_id=m.id),0),
			updated_at=now()
		WHERE m.id=$1
	`, r.schema, r.schema, r.schema), materialID)
	return err
}

func (r Repository) materialTargetQtyToLegacyTx(ctx context.Context, tx pgx.Tx, unitCode string, qty float64) (int64, int64) {
	unitCode = strings.TrimSpace(unitCode)
	unitType := ""
	if unitCode != "" {
		var tableName string
		_ = tx.QueryRow(ctx, `SELECT COALESCE(to_regclass($1)::text,'')`, fmt.Sprintf("%s.product_unit_definitions", r.schema)).Scan(&tableName)
		if tableName != "" {
			_ = tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT COALESCE(unit_type,'')
				FROM %s.product_unit_definitions
				WHERE code=$1 AND active=true
				LIMIT 1`, r.schema), unitCode).Scan(&unitType)
		}
	}
	if isWeightUnit(unitCode, unitType) {
		return int64(math.Round(weightQtyToGrams(unitCode, qty))), 0
	}
	return 0, int64(math.Round(qty))
}

func isWeightUnit(unitCode, unitType string) bool {
	switch strings.ToLower(strings.TrimSpace(unitType)) {
	case "weight", "重量":
		return true
	}
	return stockWeightUnitGrams(unitCode) > 0
}

func weightQtyToGrams(unitCode string, qty float64) float64 {
	if factor := stockWeightUnitGrams(unitCode); factor > 0 {
		return qty * factor
	}
	return qty
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
		SELECT id,transfer_no,product_id,spec_g,bom_spec_id,bom_variant_id
		FROM %s.finished_product_transfers
		WHERE idempotency_key=$1
		FOR UPDATE
	`, r.schema), key).Scan(&result.TransferID, &result.TransferNo, &result.ProductID, &result.SpecG, &result.BomSpecID, &result.BomVariantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return stockapp.FinishedProductTransferResult{}, false, nil
		}
		return stockapp.FinishedProductTransferResult{}, false, err
	}
	return result, true, nil
}

type finishedProductBomSpecIdentity struct {
	BomVariantID  int64
	InventoryUnit string
}

func resolveFinishedProductBomSpecIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, explicitVariantID int64, explicitUnit string) (finishedProductBomSpecIdentity, error) {
	if bomSpecID <= 0 {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("product_bom_spec_not_configured")
	}
	for _, table := range []string{"production_bom_specs", "production_bom_version_variants", "production_bom_versions", "production_bom_output_bindings"} {
		exists, existsErr := stockSchemaRelationExistsTx(ctx, tx, schema, table)
		if existsErr != nil {
			return finishedProductBomSpecIdentity{}, existsErr
		}
		if !exists {
			return finishedProductBomSpecIdentity{}, fmt.Errorf("BOM specification catalog is not available")
		}
	}
	var bomID, versionID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT binding.bom_id,binding.bom_version_id
		FROM %s.production_bom_output_bindings binding
		WHERE binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
		FOR SHARE OF binding
	`, schema), productID).Scan(&bomID, &versionID)
	if err == pgx.ErrNoRows {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("BOM specification does not belong to the current default published BOM")
	}
	if err != nil {
		return finishedProductBomSpecIdentity{}, err
	}
	var identity finishedProductBomSpecIdentity
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT variant.id,COALESCE(NULLIF(variant.inventory_unit,''),NULLIF(spec.inventory_unit,''),'')
		FROM %s.production_bom_versions version
		JOIN %s.production_bom_specs spec ON spec.id=$3 AND spec.bom_id=version.bom_id
		JOIN %s.production_bom_version_variants variant
		  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
		WHERE version.id=$2 AND version.bom_id=$1 AND version.status='published'
		ORDER BY variant.id
		LIMIT 1
	`, schema, schema, schema), bomID, versionID, bomSpecID).Scan(&identity.BomVariantID, &identity.InventoryUnit)
	if err == pgx.ErrNoRows {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("BOM specification does not belong to the current default published BOM")
	}
	if err != nil {
		return finishedProductBomSpecIdentity{}, err
	}
	identity.InventoryUnit = strings.TrimSpace(identity.InventoryUnit)
	if explicitVariantID > 0 && explicitVariantID != identity.BomVariantID {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("BOM variant is stale; use current default published BOM specification")
	}
	if unit := strings.TrimSpace(explicitUnit); unit != "" && !strings.EqualFold(unit, identity.InventoryUnit) {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("inventory unit does not match current BOM specification: %s", identity.InventoryUnit)
	}
	if identity.InventoryUnit == "" {
		return finishedProductBomSpecIdentity{}, fmt.Errorf("current default published BOM specification inventory unit is empty")
	}
	return identity, nil
}

func stockSchemaRelationExistsTx(ctx context.Context, tx pgx.Tx, schema, table string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, fmt.Sprintf("%s.%s", schema, table)).Scan(&exists)
	return exists, err
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
	return finishedInventoryIdentityQtyTx(ctx, tx, schema, productID, 0, specG, warehouse)
}

func finishedInventoryIdentityQtyTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, specG int64, warehouse string) (int64, int64, error) {
	var units, looseG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND warehouse=$4
		FOR UPDATE
	`, schema), productID, bomSpecID, specG, warehouse).Scan(&units, &looseG)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return units, looseG, nil
}

func availableFinishedQualityGTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG int64, warehouse string) (int64, bool, error) {
	availableG, _, hasBatches, err := availableFinishedQualityIdentityTx(ctx, tx, schema, productID, 0, specG, warehouse)
	return availableG, hasBatches, err
}

func availableFinishedQualityIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, specG int64, warehouse string) (int64, int64, bool, error) {
	var availableG, availableUnits, batchCount int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		WITH finished_batches AS (
			SELECT b.id,
			       b.remaining_g,
			       b.remaining_units,
			       COALESCE(b.quality_status,'unchecked') AS quality_status,
			       COALESCE(last_ledger.warehouse,'finished_goods') AS warehouse
			FROM %s.stock_batches b
			LEFT JOIN LATERAL (
				SELECT l.warehouse
				FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code
				  AND l.item_type=b.item_type
				  AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id
				  AND l.spec_g=b.spec_g
				ORDER BY l.id DESC
				LIMIT 1
			) last_ledger ON true
			WHERE b.item_type=$1
			  AND b.item_id=$2
			  AND b.bom_spec_id=$3
			  AND b.spec_g=$4
			  AND (b.remaining_g > 0 OR b.remaining_units > 0)
		)
		SELECT COALESCE(SUM(CASE WHEN quality_status NOT IN ('hold','reject') THEN remaining_g ELSE 0 END),0)::bigint,
		       COALESCE(SUM(CASE WHEN quality_status NOT IN ('hold','reject') THEN remaining_units ELSE 0 END),0)::bigint,
		       COUNT(*)::bigint
		FROM finished_batches
		WHERE warehouse=$5
	`, schema, schema), itemTypeFinishedProduct, productID, bomSpecID, specG, warehouse).Scan(&availableG, &availableUnits, &batchCount)
	if err != nil {
		if strings.Contains(err.Error(), "stock_batches") || strings.Contains(err.Error(), "quality_status") {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	return availableG, availableUnits, batchCount > 0, nil
}

func upsertFinishedInventoryTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG int64, warehouse string, units, looseG int64) error {
	return upsertFinishedInventoryIdentityTx(ctx, tx, schema, productID, 0, 0, specG, warehouse, units, looseG)
}

func upsertFinishedInventoryIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, bomVariantID, specG int64, warehouse string, units, looseG int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
		SET bom_variant_id=excluded.bom_variant_id,onhand_units=excluded.onhand_units,
		    onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, schema), productID, bomSpecID, bomVariantID, specG, warehouse, units, looseG)
	return err
}

type ledgerEntry struct {
	ItemType        string
	ItemID          int64
	ItemName        string
	SpecG           int64
	BomSpecID       int64
	BomVariantID    int64
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
			item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,now())
	`, schema),
		e.ItemType, e.ItemID, e.ItemName, e.SpecG, e.BomSpecID, e.BomVariantID, e.Warehouse,
		e.SourceDocType, e.SourceDocID, e.SourceBatchCode, e.SourceBatchID,
		e.BeforeG, e.ChangeG, e.AfterG, e.BeforeUnits, e.ChangeUnits, e.AfterUnits, e.Operator,
	)
	return err
}
