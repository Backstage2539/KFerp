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
	ProductID   int64
	ProductName string
	SpecG       int64
	Units       int64
	NeedG       int64
}

type orderStockBatchRow struct {
	BatchID     int64
	BatchCode   string
	ProductName string
	AvailableG  int64
	CreatedAt   string
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
		if cmd.ProductID == nil || *cmd.ProductID <= 0 || cmd.SpecG <= 0 || cmd.Units <= 0 {
			continue
		}
		key := fmt.Sprintf("%d-%d", *cmd.ProductID, cmd.SpecG)
		if idx, ok := byKey[key]; ok {
			out[idx].Units += cmd.Units
			out[idx].NeedG += cmd.Units * cmd.SpecG
			if out[idx].ProductName == "" {
				out[idx].ProductName = strings.TrimSpace(cmd.Name)
			}
			continue
		}
		byKey[key] = len(out)
		out = append(out, orderStockItem{
			ProductID:   *cmd.ProductID,
			ProductName: strings.TrimSpace(cmd.Name),
			SpecG:       cmd.SpecG,
			Units:       cmd.Units,
			NeedG:       cmd.Units * cmd.SpecG,
		})
	}
	return out
}

func (r Repository) previewOrderStockBatches(ctx context.Context, q stockBatchQueryer, items []orderStockItem, excludeOrderID int64, lock bool) (salesapp.OrderStockBatchPreview, error) {
	items = aggregateOrderStockItems(items)
	preview := salesapp.OrderStockBatchPreview{Sufficient: len(items) > 0}
	for _, item := range items {
		batches, err := r.loadOrderStockBatchAvailability(ctx, q, item.ProductID, item.SpecG, excludeOrderID, lock)
		if err != nil {
			return salesapp.OrderStockBatchPreview{}, err
		}
		legacyBatches, err := r.loadLegacyFinishedInventoryAvailability(ctx, q, item.ProductID, item.SpecG, excludeOrderID, sumOrderStockBatchAvailableG(batches), lock)
		if err != nil {
			return salesapp.OrderStockBatchPreview{}, err
		}
		batches = append(batches, legacyBatches...)
		available := int64(0)
		availability := make([]stockdomain.BatchAvailability, 0, len(batches))
		productName := strings.TrimSpace(item.ProductName)
		for _, batch := range batches {
			if productName == "" {
				productName = strings.TrimSpace(batch.ProductName)
			}
			available += batch.AvailableG
			availability = append(availability, stockdomain.BatchAvailability{
				BatchID:    batch.BatchID,
				BatchCode:  batch.BatchCode,
				AvailableG: batch.AvailableG,
			})
		}
		allocations, allocErr := stockdomain.AllocateFIFO(availability, item.NeedG)
		line := salesapp.OrderStockBatchPreviewLine{
			ProductID:   item.ProductID,
			ProductName: productName,
			SpecG:       item.SpecG,
			NeedUnits:   item.Units,
			NeedG:       item.NeedG,
			AvailableG:  available,
			Sufficient:  allocErr == nil,
		}
		if allocErr == nil {
			line.Allocations = orderStockAllocationsForPreview(allocations, batches)
		}
		if !line.Sufficient {
			preview.Sufficient = false
		}
		if len(line.Allocations) > 0 {
			preview.HasBatchChoices = true
		}
		preview.TotalNeedG += item.NeedG
		preview.TotalAvailableG += available
		preview.Lines = append(preview.Lines, line)
	}
	return preview, nil
}

func aggregateOrderStockItems(items []orderStockItem) []orderStockItem {
	byKey := map[string]int{}
	out := make([]orderStockItem, 0, len(items))
	for _, item := range items {
		if item.ProductID <= 0 || item.SpecG <= 0 || item.Units <= 0 {
			continue
		}
		if item.NeedG <= 0 {
			item.NeedG = item.Units * item.SpecG
		}
		key := fmt.Sprintf("%d-%d", item.ProductID, item.SpecG)
		if idx, ok := byKey[key]; ok {
			out[idx].Units += item.Units
			out[idx].NeedG += item.NeedG
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

func orderStockAllocationsForPreview(allocations []stockdomain.BatchAllocation, batches []orderStockBatchRow) []salesapp.OrderStockBatchAllocation {
	batchByID := map[int64]orderStockBatchRow{}
	for _, batch := range batches {
		batchByID[batch.BatchID] = batch
	}
	out := make([]salesapp.OrderStockBatchAllocation, 0, len(allocations))
	for _, alloc := range allocations {
		batch := batchByID[alloc.BatchID]
		out = append(out, salesapp.OrderStockBatchAllocation{
			BatchID:    alloc.BatchID,
			BatchCode:  alloc.BatchCode,
			AvailableG: batch.AvailableG,
			AllocatedG: alloc.QtyG,
			CreatedAt:  batch.CreatedAt,
		})
	}
	return out
}

func (r Repository) loadOrderStockBatchAvailability(ctx context.Context, q stockBatchQueryer, productID, specG, excludeOrderID int64, lock bool) ([]orderStockBatchRow, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF b"
	}
	sql := fmt.Sprintf(`
		SELECT b.id,
		       b.batch_code,
		       COALESCE(NULLIF(p.name,''), NULLIF(b.item_name,''), '') AS product_name,
		       GREATEST(0, COALESCE(b.remaining_g,0) - COALESCE(reserved.reserved_g,0))::bigint AS available_g,
		       to_char(b.created_at,'YYYY-MM-DD HH24:MI') AS created_at
		FROM %s.stock_batches b
		LEFT JOIN %s.products p ON p.id=b.item_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(a.allocated_g),0)::bigint AS reserved_g
			FROM %s.order_stock_batch_allocations a
			WHERE a.batch_code=b.batch_code
			  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
		) reserved ON true
		LEFT JOIN LATERAL (
			SELECT l.warehouse
			FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code
			  AND l.item_type=b.item_type
			  AND l.item_id=b.item_id
			  AND l.spec_g=b.spec_g
			ORDER BY l.id DESC
			LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product'
		  AND b.item_id=$1
		  AND b.spec_g=$2
		  AND COALESCE(b.remaining_g,0) > 0
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		  AND COALESCE(last_ledger.warehouse,'finished_goods') = 'finished_goods'
		ORDER BY b.created_at, b.id%s
	`, r.schema, r.schema, r.schema, r.schema, lockClause)
	rows, err := q.Query(ctx, sql, productID, specG, excludeOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]orderStockBatchRow, 0)
	for rows.Next() {
		var row orderStockBatchRow
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.ProductName, &row.AvailableG, &row.CreatedAt); err != nil {
			return nil, err
		}
		if row.AvailableG > 0 {
			out = append(out, row)
		}
	}
	return out, rows.Err()
}

func sumOrderStockBatchAvailableG(batches []orderStockBatchRow) int64 {
	total := int64(0)
	for _, batch := range batches {
		total += batch.AvailableG
	}
	return total
}

func legacyFinishedInventoryBatchCode(productID, specG int64) string {
	return fmt.Sprintf("LEGACY-FP-%d-%d", productID, specG)
}

func (r Repository) loadLegacyFinishedInventoryAvailability(ctx context.Context, q stockBatchQueryer, productID, specG, excludeOrderID, realBatchAvailableG int64, lock bool) ([]orderStockBatchRow, error) {
	if productID <= 0 || specG <= 0 {
		return nil, nil
	}
	hasBatchRows, err := r.hasAnyFinishedStockBatchRows(ctx, q, productID, specG)
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
			  AND a.spec_g=$2
			  AND ($3::bigint <= 0 OR a.order_id <> $3::bigint)
		) reserved ON true
		WHERE fi.product_id=$1
		  AND fi.spec_g=$2
		  AND fi.warehouse='finished_goods'%s
	`, r.schema, r.schema, r.schema, lockClause)
	rows, err := q.Query(ctx, sql, productID, specG, excludeOrderID)
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
		inventoryG += units*specG + looseG
		reservedG = rowReservedG
		if productName == "" {
			productName = rowProductName
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	availableG := inventoryG - reservedG - realBatchAvailableG
	if availableG <= 0 {
		return nil, nil
	}
	return []orderStockBatchRow{{
		BatchID:     0,
		BatchCode:   legacyFinishedInventoryBatchCode(productID, specG),
		ProductName: productName,
		AvailableG:  availableG,
		CreatedAt:   legacyFinishedInventoryCreatedAt,
	}}, nil
}

func (r Repository) hasAnyFinishedStockBatchRows(ctx context.Context, q stockBatchQueryer, productID, specG int64) (bool, error) {
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT 1
		FROM %s.stock_batches
		WHERE item_type='finished_product'
		  AND item_id=$1
		  AND spec_g=$2
		  AND COALESCE(remaining_g,0) > 0
		LIMIT 1
	`, r.schema), productID, specG)
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
					INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,spec_g,need_g,batch_id,batch_code,allocated_g,operator)
					VALUES($1,$2,$3,$4,$5,$6,$7,$8)
				`, r.schema), orderID, line.ProductID, line.SpecG, line.NeedG, alloc.BatchID, alloc.BatchCode, alloc.AllocatedG, actor); err != nil {
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
