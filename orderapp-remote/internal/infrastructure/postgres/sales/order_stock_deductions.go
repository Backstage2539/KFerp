package sales

import (
	"context"
	"fmt"
	"strings"

	inventorydomain "orderapp/internal/domain/inventory"

	"github.com/jackc/pgx/v5"
)

const salesOrderShipmentStockSource = "sales_order_shipment"

type orderStockDeductionAllocation struct {
	ProductID   int64
	ProductName string
	SpecG       int64
	BatchID     int64
	BatchCode   string
	AllocatedG  int64
}

func (r Repository) deductOrderAllocatedStockTx(ctx context.Context, tx pgx.Tx, orderID int64, actor string) error {
	if orderID <= 0 {
		return nil
	}
	var alreadyDeducted bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.order_stock_deductions WHERE order_id=$1)`, r.schema), orderID).Scan(&alreadyDeducted); err != nil {
		return err
	}
	if alreadyDeducted {
		return nil
	}
	warehouse, err := orderSourceWarehouseTx(ctx, tx, r.schema, orderID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, fmt.Sprintf(`
		WITH locked AS (
			SELECT *
			FROM %s.order_stock_batch_allocations
			WHERE order_id=$1
			FOR UPDATE
		)
		SELECT a.product_id,
		       COALESCE(NULLIF(p.name,''),'') AS product_name,
		       a.spec_g,
		       a.batch_id,
		       a.batch_code,
		       SUM(a.allocated_g)::bigint AS allocated_g
		FROM locked a
		LEFT JOIN %s.products p ON p.id=a.product_id
		GROUP BY a.product_id, p.name, a.spec_g, a.batch_id, a.batch_code
		ORDER BY MIN(a.id)
	`, r.schema, r.schema), orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	allocations := make([]orderStockDeductionAllocation, 0)
	for rows.Next() {
		var alloc orderStockDeductionAllocation
		if err := rows.Scan(&alloc.ProductID, &alloc.ProductName, &alloc.SpecG, &alloc.BatchID, &alloc.BatchCode, &alloc.AllocatedG); err != nil {
			return err
		}
		if alloc.ProductID > 0 && alloc.SpecG > 0 && alloc.AllocatedG > 0 && strings.TrimSpace(alloc.BatchCode) != "" {
			allocations = append(allocations, alloc)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, alloc := range allocations {
		if alloc.BatchID > 0 {
			if err := r.deductFinishedBatchAllocationTx(ctx, tx, orderID, alloc, warehouse, actor); err != nil {
				return err
			}
			continue
		}
		if err := r.deductLegacyFinishedInventoryAllocationTx(ctx, tx, orderID, alloc, warehouse, actor); err != nil {
			return err
		}
	}
	if len(allocations) == 0 && warehouse != "finished_goods" {
		return r.deductOrderSourceWarehouseItemsTx(ctx, tx, orderID, warehouse, actor)
	}
	return nil
}

func orderSourceWarehouseTx(ctx context.Context, tx pgx.Tx, schema string, orderID int64) (string, error) {
	var warehouse string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(source_warehouse,''),'finished_goods')
		FROM %s.orders
		WHERE id=$1
	`, schema), orderID).Scan(&warehouse)
	if err != nil {
		return "", err
	}
	warehouse = strings.TrimSpace(warehouse)
	if warehouse == "" {
		warehouse = "finished_goods"
	}
	return warehouse, nil
}

func (r Repository) deductFinishedBatchAllocationTx(ctx context.Context, tx pgx.Tx, orderID int64, alloc orderStockDeductionAllocation, sourceWarehouse string, actor string) error {
	sourceWarehouse = strings.TrimSpace(sourceWarehouse)
	if sourceWarehouse == "" {
		sourceWarehouse = "finished_goods"
	}
	var beforeG, beforeUnits int64
	var itemName, qualityStatus, warehouse string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.remaining_g,
		       b.remaining_units,
		       COALESCE(NULLIF(b.item_name,''), NULLIF(p.name,''), '') AS item_name,
		       COALESCE(b.quality_status,'unchecked') AS quality_status,
		       COALESCE(last_ledger.warehouse,'finished_goods') AS warehouse
		FROM %s.stock_batches b
		LEFT JOIN %s.products p ON p.id=b.item_id
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
		WHERE b.id=$1
		  AND b.batch_code=$2
		  AND b.item_type='finished_product'
		  AND b.item_id=$3
		  AND b.spec_g=$4
		FOR UPDATE OF b
	`, r.schema, r.schema, r.schema), alloc.BatchID, alloc.BatchCode, alloc.ProductID, alloc.SpecG).Scan(&beforeG, &beforeUnits, &itemName, &qualityStatus, &warehouse)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("finished stock batch not found: %s", alloc.BatchCode)
		}
		return err
	}
	if qualityStatus == "hold" || qualityStatus == "reject" {
		return fmt.Errorf("finished stock batch is not releasable: %s", alloc.BatchCode)
	}
	if warehouse != sourceWarehouse {
		return fmt.Errorf("finished stock batch is not in %s: %s", sourceWarehouse, alloc.BatchCode)
	}
	if beforeG < alloc.AllocatedG {
		return fmt.Errorf("finished stock batch insufficient: %s", alloc.BatchCode)
	}
	afterG := beforeG - alloc.AllocatedG
	afterUnits := afterG / alloc.SpecG
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches
		SET remaining_g=$2,
		    remaining_units=$3
		WHERE id=$1
	`, r.schema), alloc.BatchID, afterG, afterUnits); err != nil {
		return err
	}
	return r.recordOrderStockDeductionTx(ctx, tx, orderID, alloc, itemName, warehouse, beforeG, -alloc.AllocatedG, afterG, beforeUnits, afterUnits-beforeUnits, afterUnits, actor)
}

func (r Repository) deductLegacyFinishedInventoryAllocationTx(ctx context.Context, tx pgx.Tx, orderID int64, alloc orderStockDeductionAllocation, warehouse string, actor string) error {
	warehouse = strings.TrimSpace(warehouse)
	if warehouse == "" {
		warehouse = "finished_goods"
	}
	productName := strings.TrimSpace(alloc.ProductName)
	if productName == "" {
		_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, r.schema), alloc.ProductID).Scan(&productName)
	}

	var beforeUnits, beforeLooseG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g
		FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3
		FOR UPDATE
	`, r.schema), alloc.ProductID, alloc.SpecG, warehouse).Scan(&beforeUnits, &beforeLooseG)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	beforeQty := inventorydomain.Quantity{Units: beforeUnits, LooseG: beforeLooseG}
	beforeG, err := inventorydomain.TotalGrams(alloc.SpecG, beforeQty)
	if err != nil {
		return err
	}
	afterQty, deductedG, gapG, err := inventorydomain.Deduct(alloc.SpecG, beforeQty, alloc.AllocatedG)
	if err != nil {
		return err
	}
	if gapG > 0 || deductedG != alloc.AllocatedG {
		return fmt.Errorf("finished inventory insufficient: %s", alloc.BatchCode)
	}
	afterG, err := inventorydomain.TotalGrams(alloc.SpecG, afterQty)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,
		    onhand_loose_g=excluded.onhand_loose_g,
		    updated_at=now()
	`, r.schema), alloc.ProductID, alloc.SpecG, warehouse, afterQty.Units, afterQty.LooseG); err != nil {
		return err
	}
	return r.recordOrderStockDeductionTx(ctx, tx, orderID, alloc, productName, warehouse, beforeG, -alloc.AllocatedG, afterG, beforeUnits, afterQty.Units-beforeUnits, afterQty.Units, actor)
}

func (r Repository) deductOrderSourceWarehouseItemsTx(ctx context.Context, tx pgx.Tx, orderID int64, warehouse string, actor string) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT oi.product_id,
		       COALESCE(NULLIF(oi.item_name,''), NULLIF(p.name,''), '') AS product_name,
		       COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g,
		       SUM(COALESCE(oi.qty,0))::bigint AS units
		FROM %s.order_items oi
		LEFT JOIN %s.products p ON p.id=oi.product_id
		WHERE oi.order_id=$1
		GROUP BY oi.product_id, oi.item_name, p.name, COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')
	`, r.schema, r.schema), orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var alloc orderStockDeductionAllocation
		var units int64
		if err := rows.Scan(&alloc.ProductID, &alloc.ProductName, &alloc.SpecG, &units); err != nil {
			return err
		}
		if alloc.ProductID <= 0 || alloc.SpecG <= 0 || units <= 0 {
			continue
		}
		alloc.AllocatedG = alloc.SpecG * units
		alloc.BatchCode = "SOURCE-WH:" + warehouse
		if err := r.deductLegacyFinishedInventoryAllocationTx(ctx, tx, orderID, alloc, warehouse, actor); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (r Repository) recordOrderStockDeductionTx(ctx context.Context, tx pgx.Tx, orderID int64, alloc orderStockDeductionAllocation, itemName, warehouse string, beforeG, changeG, afterG, beforeUnits, changeUnits, afterUnits int64, actor string) error {
	warehouse = strings.TrimSpace(warehouse)
	if warehouse == "" {
		warehouse = "finished_goods"
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,
			qty_before_units,qty_change_units,qty_after_units,
			operator,created_at
		) VALUES('finished_product',$1,$2,$3,$4,$5,$6,$7,'',$8,$9,$10,$11,$12,$13,$14,now())
	`, r.schema),
		alloc.ProductID, itemName, alloc.SpecG, warehouse,
		salesOrderShipmentStockSource, orderID, alloc.BatchCode,
		beforeG, changeG, afterG,
		beforeUnits, changeUnits, afterUnits,
		actor,
	); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_stock_deductions(
			order_id,product_id,spec_g,batch_id,batch_code,deducted_g,
			source_doc_type,source_doc_id,operator,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
		ON CONFLICT(order_id,product_id,spec_g,batch_code) DO NOTHING
	`, r.schema),
		orderID, alloc.ProductID, alloc.SpecG, alloc.BatchID, alloc.BatchCode, alloc.AllocatedG,
		salesOrderShipmentStockSource, orderID, actor,
	)
	return err
}
