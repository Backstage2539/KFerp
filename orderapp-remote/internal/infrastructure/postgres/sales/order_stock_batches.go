package sales

import (
	"context"
	"fmt"
	"strings"

	salesapp "orderapp/internal/application/sales"
	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

type orderStockItem struct {
	ProductID    int64
	BomSpecID    int64
	BomVariantID int64
	ProductName  string
	SpecG        int64
	Units        int64
	NeedG        int64
}

type orderStockBatchRow struct {
	BatchID        int64
	BatchCode      string
	BomVariantID   int64
	ProductName    string
	AvailableG     int64
	AvailableUnits int64
	CreatedAt      string
}

const legacyFinishedInventoryCreatedAt = "库存余额"

type stockBatchQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r Repository) PreviewOrderStockBatches(ctx context.Context, cmd salesapp.OrderStockBatchPreviewCommand) (salesapp.OrderStockBatchPreview, error) {
	items := orderStockItemsFromCommands(cmd.Items)
	return r.previewOrderStockBatches(ctx, r.pool, items, cmd.EditID, false)
}

func orderStockItemsFromCommands(commands []salesapp.OrderItemCommand) []orderStockItem {
	byKey := map[string]int{}
	out := make([]orderStockItem, 0)
	for _, cmd := range commands {
		if cmd.ProductID == nil || *cmd.ProductID <= 0 || cmd.Units <= 0 || (cmd.BomSpecID <= 0 && cmd.SpecG <= 0) {
			continue
		}
		key := orderStockItemKey(*cmd.ProductID, cmd.BomSpecID, cmd.SpecG)
		if idx, ok := byKey[key]; ok {
			out[idx].Units += cmd.Units
			if cmd.BomSpecID <= 0 {
				out[idx].NeedG += cmd.Units * cmd.SpecG
			}
			if out[idx].BomVariantID <= 0 {
				out[idx].BomVariantID = cmd.BomVariantID
			}
			if out[idx].ProductName == "" {
				out[idx].ProductName = strings.TrimSpace(cmd.Name)
			}
			continue
		}
		byKey[key] = len(out)
		out = append(out, orderStockItem{
			ProductID:    *cmd.ProductID,
			BomSpecID:    cmd.BomSpecID,
			BomVariantID: cmd.BomVariantID,
			ProductName:  strings.TrimSpace(cmd.Name),
			SpecG:        cmd.SpecG,
			Units:        cmd.Units,
			NeedG:        legacyOrderStockNeedG(cmd.BomSpecID, cmd.SpecG, cmd.Units),
		})
	}
	return out
}

func legacyOrderStockNeedG(bomSpecID, specG, units int64) int64 {
	if bomSpecID > 0 {
		return 0
	}
	return specG * units
}

func orderStockItemKey(productID, bomSpecID, specG int64) string {
	if bomSpecID > 0 {
		return fmt.Sprintf("%d-bom-%d", productID, bomSpecID)
	}
	return fmt.Sprintf("%d-legacy-%d", productID, specG)
}

func (r Repository) previewOrderStockBatches(ctx context.Context, q stockBatchQueryer, items []orderStockItem, excludeOrderID int64, lock bool) (salesapp.OrderStockBatchPreview, error) {
	items = aggregateOrderStockItems(items)
	preview := salesapp.OrderStockBatchPreview{Sufficient: len(items) > 0}
	for _, item := range items {
		batches, err := r.loadOrderStockBatchAvailability(ctx, q, item, excludeOrderID, lock)
		if err != nil {
			return salesapp.OrderStockBatchPreview{}, err
		}
		aggregateBatches, err := r.loadFinishedInventoryAggregateAvailability(ctx, q, item, excludeOrderID, batches, lock)
		if err != nil {
			return salesapp.OrderStockBatchPreview{}, err
		}
		batches = append(batches, aggregateBatches...)
		availableG := int64(0)
		availableUnits := int64(0)
		productName := strings.TrimSpace(item.ProductName)
		for _, batch := range batches {
			if productName == "" {
				productName = strings.TrimSpace(batch.ProductName)
			}
			availableG += batch.AvailableG
			availableUnits += batch.AvailableUnits
		}
		allocations, allocErr := allocateOrderStockFIFO(item, batches)
		line := salesapp.OrderStockBatchPreviewLine{
			ProductID:      item.ProductID,
			BomSpecID:      item.BomSpecID,
			BomVariantID:   item.BomVariantID,
			ProductName:    productName,
			SpecG:          item.SpecG,
			NeedUnits:      item.Units,
			NeedG:          item.NeedG,
			AvailableUnits: availableUnits,
			AvailableG:     availableG,
			Sufficient:     allocErr == nil,
		}
		if allocErr == nil {
			line.Allocations = allocations
		}
		if !line.Sufficient {
			preview.Sufficient = false
		}
		if len(line.Allocations) > 0 {
			preview.HasBatchChoices = true
		}
		preview.TotalNeedUnits += item.Units
		preview.TotalNeedG += item.NeedG
		preview.TotalAvailableUnits += availableUnits
		preview.TotalAvailableG += availableG
		preview.Lines = append(preview.Lines, line)
	}
	return preview, nil
}

func aggregateOrderStockItems(items []orderStockItem) []orderStockItem {
	byKey := map[string]int{}
	out := make([]orderStockItem, 0, len(items))
	for _, item := range items {
		if item.ProductID <= 0 || item.Units <= 0 || (item.BomSpecID <= 0 && item.SpecG <= 0) {
			continue
		}
		if item.BomSpecID <= 0 && item.NeedG <= 0 {
			item.NeedG = item.Units * item.SpecG
		}
		if item.BomSpecID > 0 {
			item.SpecG = 0
			item.NeedG = 0
		}
		key := orderStockItemKey(item.ProductID, item.BomSpecID, item.SpecG)
		if idx, ok := byKey[key]; ok {
			out[idx].Units += item.Units
			out[idx].NeedG += item.NeedG
			if out[idx].BomVariantID <= 0 {
				out[idx].BomVariantID = item.BomVariantID
			}
			if out[idx].ProductName == "" {
				out[idx].ProductName = item.ProductName
			}
			continue
		}
		byKey[key] = len(out)
		out = append(out, item)
	}
	return out
}

func allocateOrderStockFIFO(item orderStockItem, batches []orderStockBatchRow) ([]salesapp.OrderStockBatchAllocation, error) {
	if item.BomSpecID > 0 {
		remaining := item.Units
		out := make([]salesapp.OrderStockBatchAllocation, 0, len(batches))
		for _, batch := range batches {
			if remaining <= 0 {
				break
			}
			allocated := batch.AvailableUnits
			if allocated > remaining {
				allocated = remaining
			}
			if allocated <= 0 {
				continue
			}
			out = append(out, salesapp.OrderStockBatchAllocation{
				BatchID:        batch.BatchID,
				BatchCode:      batch.BatchCode,
				BomVariantID:   batch.BomVariantID,
				AvailableUnits: batch.AvailableUnits,
				AllocatedUnits: allocated,
				CreatedAt:      batch.CreatedAt,
			})
			remaining -= allocated
		}
		if remaining > 0 {
			return nil, fmt.Errorf("finished stock batch insufficient")
		}
		return out, nil
	}
	availability := make([]stockdomain.BatchAvailability, 0, len(batches))
	for _, batch := range batches {
		availability = append(availability, stockdomain.BatchAvailability{
			BatchID:    batch.BatchID,
			BatchCode:  batch.BatchCode,
			AvailableG: batch.AvailableG,
		})
	}
	allocations, err := stockdomain.AllocateFIFO(availability, item.NeedG)
	if err != nil {
		return nil, err
	}
	return orderStockAllocationsForPreview(allocations, batches), nil
}

func orderStockAllocationsForPreview(allocations []stockdomain.BatchAllocation, batches []orderStockBatchRow) []salesapp.OrderStockBatchAllocation {
	batchByID := map[int64]orderStockBatchRow{}
	for _, batch := range batches {
		batchByID[batch.BatchID] = batch
	}
	out := make([]salesapp.OrderStockBatchAllocation, 0, len(allocations))
	for _, alloc := range allocations {
		batch := batchByID[alloc.BatchID]
		out = append(out, salesapp.OrderStockBatchAllocation{
			BatchID:        alloc.BatchID,
			BatchCode:      alloc.BatchCode,
			BomVariantID:   batch.BomVariantID,
			AvailableG:     batch.AvailableG,
			AvailableUnits: batch.AvailableUnits,
			AllocatedG:     alloc.QtyG,
			CreatedAt:      batch.CreatedAt,
		})
	}
	return out
}

func (r Repository) loadOrderStockBatchAvailability(ctx context.Context, q stockBatchQueryer, item orderStockItem, excludeOrderID int64, lock bool) ([]orderStockBatchRow, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF b"
	}
	if item.BomSpecID > 0 {
		sql := fmt.Sprintf(`
			SELECT b.id,
			       b.batch_code,
			       COALESCE(b.bom_variant_id,0),
			       COALESCE(NULLIF(p.name,''), NULLIF(b.item_name,''), '') AS product_name,
			       GREATEST(0, COALESCE(b.remaining_units,0) - COALESCE(reserved.reserved_units,0))::bigint AS available_units,
			       to_char(b.created_at,'YYYY-MM-DD HH24:MI') AS created_at
			FROM %s.stock_batches b
			LEFT JOIN %s.products p ON p.id=b.item_id
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(a.allocated_units),0)::bigint AS reserved_units
				FROM %s.order_stock_batch_allocations a
				WHERE a.batch_code=b.batch_code
				  AND a.product_id=b.item_id
				  AND a.bom_spec_id=b.bom_spec_id
				  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
				  AND NOT EXISTS (
					SELECT 1
					FROM %s.order_stock_deductions d
					WHERE d.order_id=a.order_id
					  AND d.product_id=a.product_id
					  AND d.bom_spec_id=a.bom_spec_id
					  AND d.batch_code=a.batch_code
				  )
			) reserved ON true
			LEFT JOIN LATERAL (
				SELECT l.warehouse
				FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code
				  AND l.item_type=b.item_type
				  AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id
				ORDER BY l.id DESC
				LIMIT 1
			) last_ledger ON true
			WHERE b.item_type='finished_product'
			  AND b.item_id=$1
			  AND b.bom_spec_id=$2
			  AND COALESCE(b.remaining_units,0) > 0
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			  AND COALESCE(last_ledger.warehouse,'finished_goods') = 'finished_goods'
			ORDER BY b.created_at, b.id%s
		`, r.schema, r.schema, r.schema, r.schema, r.schema, lockClause)
		rows, err := q.Query(ctx, sql, item.ProductID, item.BomSpecID, excludeOrderID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := make([]orderStockBatchRow, 0)
		for rows.Next() {
			var row orderStockBatchRow
			if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.BomVariantID, &row.ProductName, &row.AvailableUnits, &row.CreatedAt); err != nil {
				return nil, err
			}
			if row.AvailableUnits > 0 {
				out = append(out, row)
			}
		}
		return out, rows.Err()
	}
	sql := fmt.Sprintf(`
		SELECT b.id,
		       b.batch_code,
		       COALESCE(b.bom_variant_id,0),
		       COALESCE(NULLIF(p.name,''), NULLIF(b.item_name,''), '') AS product_name,
		       GREATEST(0, COALESCE(b.remaining_g,0) - COALESCE(reserved.reserved_g,0))::bigint AS available_g,
		       to_char(b.created_at,'YYYY-MM-DD HH24:MI') AS created_at
		FROM %s.stock_batches b
		LEFT JOIN %s.products p ON p.id=b.item_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.allocated_g),0)::bigint AS reserved_g
			FROM %s.order_stock_batch_allocations a
			WHERE a.batch_code=b.batch_code
			  AND a.product_id=b.item_id
			  AND a.bom_spec_id=0
			  AND a.spec_g=b.spec_g
			  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
			  AND NOT EXISTS (
				SELECT 1
				FROM %s.order_stock_deductions d
				WHERE d.order_id=a.order_id
				  AND d.product_id=a.product_id
				  AND d.bom_spec_id=0
				  AND d.spec_g=a.spec_g
				  AND d.batch_code=a.batch_code
			  )
		) reserved ON true
		LEFT JOIN LATERAL (
			SELECT l.warehouse
			FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code
			  AND l.item_type=b.item_type
			  AND l.item_id=b.item_id
			  AND l.bom_spec_id=0
			  AND l.spec_g=b.spec_g
			ORDER BY l.id DESC
			LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product'
		  AND b.item_id=$1
		  AND b.bom_spec_id=0
		  AND b.spec_g=$2
		  AND COALESCE(b.remaining_g,0) > 0
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		  AND COALESCE(last_ledger.warehouse,'finished_goods') = 'finished_goods'
		ORDER BY b.created_at, b.id%s
	`, r.schema, r.schema, r.schema, r.schema, r.schema, lockClause)
	rows, err := q.Query(ctx, sql, item.ProductID, item.SpecG, excludeOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]orderStockBatchRow, 0)
	for rows.Next() {
		var row orderStockBatchRow
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.BomVariantID, &row.ProductName, &row.AvailableG, &row.CreatedAt); err != nil {
			return nil, err
		}
		if row.AvailableG > 0 {
			out = append(out, row)
		}
	}
	return out, rows.Err()
}

func legacyFinishedInventoryBatchCode(productID, specG int64) string {
	return fmt.Sprintf("LEGACY-FP-%d-%d", productID, specG)
}

func bomSpecFinishedInventoryBatchCode(productID, bomSpecID int64) string {
	return fmt.Sprintf("BOM-SPEC-FP-%d-%d", productID, bomSpecID)
}

func (r Repository) loadFinishedInventoryAggregateAvailability(ctx context.Context, q stockBatchQueryer, item orderStockItem, excludeOrderID int64, batches []orderStockBatchRow, lock bool) ([]orderStockBatchRow, error) {
	if item.ProductID <= 0 || (item.BomSpecID <= 0 && item.SpecG <= 0) {
		return nil, nil
	}
	hasBatchRows, err := r.hasAnyFinishedStockBatchRows(ctx, q, item)
	if err != nil {
		return nil, err
	}
	if hasBatchRows {
		return nil, nil
	}

	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF fi"
	}
	if item.BomSpecID > 0 {
		sql := fmt.Sprintf(`
			SELECT COALESCE(fi.onhand_units,0)::bigint,
			       COALESCE(fi.bom_variant_id,0)::bigint,
			       COALESCE(NULLIF(p.name,''), '') AS product_name,
			       COALESCE(reserved.reserved_units,0)::bigint AS reserved_units
			FROM %s.finished_inventory fi
			LEFT JOIN %s.products p ON p.id=fi.product_id
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(a.allocated_units),0)::bigint AS reserved_units
				FROM %s.order_stock_batch_allocations a
				WHERE a.product_id=$1
				  AND a.bom_spec_id=$2
				  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
				  AND NOT EXISTS (
					SELECT 1
					FROM %s.order_stock_deductions d
					WHERE d.order_id=a.order_id
					  AND d.product_id=a.product_id
					  AND d.bom_spec_id=a.bom_spec_id
					  AND d.batch_code=a.batch_code
				  )
			) reserved ON true
			WHERE fi.product_id=$1
			  AND fi.bom_spec_id=$2
			  AND fi.spec_g=0
			  AND fi.warehouse='finished_goods'%s
		`, r.schema, r.schema, r.schema, r.schema, lockClause)
		rows, err := q.Query(ctx, sql, item.ProductID, item.BomSpecID, excludeOrderID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var inventoryUnits, reservedUnits, bomVariantID int64
		productName := ""
		for rows.Next() {
			var rowInventoryUnits, rowReservedUnits, rowVariantID int64
			var rowProductName string
			if err := rows.Scan(&rowInventoryUnits, &rowVariantID, &rowProductName, &rowReservedUnits); err != nil {
				return nil, err
			}
			inventoryUnits += rowInventoryUnits
			reservedUnits = rowReservedUnits
			if bomVariantID <= 0 {
				bomVariantID = rowVariantID
			}
			if productName == "" {
				productName = rowProductName
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		availableUnits := inventoryUnits - reservedUnits
		if availableUnits <= 0 {
			return nil, nil
		}
		return []orderStockBatchRow{{
			BatchID:        0,
			BatchCode:      bomSpecFinishedInventoryBatchCode(item.ProductID, item.BomSpecID),
			BomVariantID:   bomVariantID,
			ProductName:    productName,
			AvailableUnits: availableUnits,
			CreatedAt:      legacyFinishedInventoryCreatedAt,
		}}, nil
	}
	sql := fmt.Sprintf(`
		SELECT COALESCE(fi.onhand_units,0)::bigint,
		       COALESCE(fi.onhand_loose_g,0)::bigint,
		       COALESCE(NULLIF(p.name,''), '') AS product_name,
		       COALESCE(reserved.reserved_g,0)::bigint AS reserved_g
		FROM %s.finished_inventory fi
		LEFT JOIN %s.products p ON p.id=fi.product_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.allocated_g),0)::bigint AS reserved_g
			FROM %s.order_stock_batch_allocations a
			WHERE a.product_id=$1
			  AND a.bom_spec_id=0
			  AND a.spec_g=$2
			  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
			  AND NOT EXISTS (
				SELECT 1
				FROM %s.order_stock_deductions d
				WHERE d.order_id=a.order_id
				  AND d.product_id=a.product_id
				  AND d.bom_spec_id=0
				  AND d.spec_g=a.spec_g
				  AND d.batch_code=a.batch_code
			  )
		) reserved ON true
		WHERE fi.product_id=$1
		  AND fi.bom_spec_id=0
		  AND fi.spec_g=$2
		  AND fi.warehouse='finished_goods'%s
	`, r.schema, r.schema, r.schema, r.schema, lockClause)
	rows, err := q.Query(ctx, sql, item.ProductID, item.SpecG, excludeOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	inventoryG := int64(0)
	reservedG := int64(0)
	productName := ""
	for rows.Next() {
		var units, looseG, rowReservedG int64
		var rowProductName string
		if err := rows.Scan(&units, &looseG, &rowProductName, &rowReservedG); err != nil {
			return nil, err
		}
		inventoryG += units*item.SpecG + looseG
		reservedG = rowReservedG
		if productName == "" {
			productName = rowProductName
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	availableG := inventoryG - reservedG
	if availableG <= 0 {
		return nil, nil
	}
	return []orderStockBatchRow{{
		BatchID:     0,
		BatchCode:   legacyFinishedInventoryBatchCode(item.ProductID, item.SpecG),
		ProductName: productName,
		AvailableG:  availableG,
		CreatedAt:   legacyFinishedInventoryCreatedAt,
	}}, nil
}

func (r Repository) hasAnyFinishedStockBatchRows(ctx context.Context, q stockBatchQueryer, item orderStockItem) (bool, error) {
	identityClause := "bom_spec_id=$2 AND COALESCE(remaining_units,0)>0"
	identityValue := item.BomSpecID
	if item.BomSpecID <= 0 {
		identityClause = "bom_spec_id=0 AND spec_g=$2 AND COALESCE(remaining_g,0)>0"
		identityValue = item.SpecG
	}
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT 1
		FROM %s.stock_batches
		WHERE item_type='finished_product'
		  AND item_id=$1
		  AND %s
		LIMIT 1
	`, r.schema, identityClause), item.ProductID, identityValue)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (r Repository) applyOrderStockDecisionTx(ctx context.Context, tx pgx.Tx, orderID int64, items []orderStockItem, decision, actor string) error {
	decision = strings.TrimSpace(decision)
	if decision == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_stock_decisions(order_id, decision, operator, updated_at)
		VALUES($1,$2,$3,now())
		ON CONFLICT(order_id) DO UPDATE SET
			decision=excluded.decision,
			operator=excluded.operator,
			updated_at=now()
	`, r.schema), orderID, decision, actor); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_stock_batch_allocations WHERE order_id=$1`, r.schema), orderID); err != nil {
		return err
	}
	switch decision {
	case "use_batch":
		preview, err := r.previewOrderStockBatches(ctx, tx, items, orderID, true)
		if err != nil {
			return err
		}
		if !preview.Sufficient || !preview.HasBatchChoices {
			return fmt.Errorf("finished stock batch insufficient")
		}
		for _, line := range preview.Lines {
			for _, alloc := range line.Allocations {
				if _, err := tx.Exec(ctx, fmt.Sprintf(`
					INSERT INTO %s.order_stock_batch_allocations(
						order_id,product_id,bom_spec_id,bom_variant_id,spec_g,
						need_g,need_units,batch_id,batch_code,allocated_g,allocated_units,operator
					) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
				`, r.schema),
					orderID, line.ProductID, line.BomSpecID, line.BomVariantID, line.SpecG,
					line.NeedG, line.NeedUnits, alloc.BatchID, alloc.BatchCode, alloc.AllocatedG, alloc.AllocatedUnits, actor,
				); err != nil {
					return err
				}
			}
		}
		statusID := lookupDefaultStatusID(ctx, tx, r.schema, "order_process_statuses", "库存待发货")
		if statusID <= 0 {
			return fmt.Errorf("stock ready status missing")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE id=$1`, r.schema), orderID, statusID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,'stock_batch_decision',NULL,'use_batch')`, r.schema), orderID, actor); err != nil {
			return err
		}
		return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "update", postgresinfra.StrPtr("stock_batch_decision"), nil, postgresinfra.StrPtr("use_batch"), postgresinfra.AuditMeta{"stock_status": "库存待发货"})
	case "produce":
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.orders o
			SET process_status_id=NULL
			WHERE o.id=$1
			  AND EXISTS (
				SELECT 1 FROM %s.order_process_statuses ops
				WHERE ops.id=o.process_status_id
				  AND ops.name IN ('库存待发货','无需生产')
			  )
		`, r.schema, r.schema), orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_audit_logs(order_id, actor, field, old_value, new_value) VALUES ($1,$2,'stock_batch_decision',NULL,'produce')`, r.schema), orderID, actor); err != nil {
			return err
		}
		return postgresinfra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &orderID, "update", postgresinfra.StrPtr("stock_batch_decision"), nil, postgresinfra.StrPtr("produce"), postgresinfra.AuditMeta{"production_gap": true})
	default:
		return fmt.Errorf("invalid stock_batch_decision")
	}
}
