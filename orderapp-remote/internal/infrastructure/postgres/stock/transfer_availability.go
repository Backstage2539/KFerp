package stock

import (
	"context"
	"fmt"
	"strings"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5"
)

func resolveTransferBatchIdentityTx(ctx context.Context, tx pgx.Tx, schema string, productID, specID, variantID, owner int64, batch, unit string) (finishedProductBomSpecIdentity, error) {
	var out finishedProductBomSpecIdentity
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.bom_variant_id,COALESCE(NULLIF(v.inventory_unit,''),NULLIF(s.inventory_unit,''),'')
FROM %[1]s.stock_batches b JOIN %[1]s.production_bom_specs s ON s.id=b.bom_spec_id
JOIN %[1]s.production_bom_version_variants v ON v.id=b.bom_variant_id AND v.bom_spec_id=b.bom_spec_id
WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.batch_code=$3 AND b.owner_customer_id=$4`, schema), productID, specID, batch, owner).Scan(&out.BomVariantID, &out.InventoryUnit)
	if err == pgx.ErrNoRows {
		return out, fmt.Errorf("来源批次与商品、规格或货主不匹配")
	}
	if err != nil {
		return out, err
	}
	if variantID > 0 && variantID != out.BomVariantID || strings.TrimSpace(unit) != "" && !strings.EqualFold(strings.TrimSpace(unit), out.InventoryUnit) {
		return out, fmt.Errorf("来源批次规格或库存单位不匹配")
	}
	if out.InventoryUnit == "" {
		return out, fmt.Errorf("来源批次缺少库存单位")
	}
	return out, nil
}

// A legacy reservation without a batch binding cannot safely be attributed to
// any particular batch. Block ordinary transfers until it is reconciled.
func hasUnboundTransferReservationTx(ctx context.Context, tx pgx.Tx, schema string, itemID, owner int64, warehouse string, product bool) (bool, error) {
	has, err := stockSchemaRelationExistsTx(ctx, tx, schema, "work_order_material_reservations")
	if err != nil || !has {
		return false, err
	}
	hasComponent, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "component_type")
	if err != nil {
		return false, err
	}
	itemCondition := "r.material_id=$1"
	if product {
		if !hasComponent {
			return false, nil
		}
		itemCondition = "r.component_type='product' AND r.component_id=$1"
	} else if hasComponent {
		itemCondition = "r.component_type='material' AND r.material_id=$1"
	}
	warehouseCondition, ownerCondition := "true", "$3::bigint=0"
	hasWarehouse, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "source_warehouse")
	if err != nil {
		return false, err
	}
	if hasWarehouse {
		warehouseCondition = "(COALESCE(r.source_warehouse,'')='' OR r.source_warehouse=$2)"
	} else {
		warehouseCondition = "$2::text<>''"
	}
	hasOwner, err := stockSchemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "source_owner_customer_id")
	if err != nil {
		return false, err
	}
	if hasOwner {
		ownerCondition = "COALESCE(r.source_owner_customer_id,0)=$3"
	}
	boundG, boundN := "0::bigint", "0::bigint"
	hasBindings, err := stockSchemaRelationExistsTx(ctx, tx, schema, "work_order_material_reservation_batches")
	if err != nil {
		return false, err
	}
	if hasBindings {
		boundG = fmt.Sprintf("(SELECT COALESCE(SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)),0) FROM %s.work_order_material_reservation_batches b WHERE b.reservation_id=r.id AND b.status='reserved')", schema)
		boundN = fmt.Sprintf("(SELECT COALESCE(SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)),0) FROM %s.work_order_material_reservation_batches b WHERE b.reservation_id=r.id AND b.status='reserved')", schema)
	}
	var unbound bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.work_order_material_reservations r WHERE r.status='reserved' AND %s AND %s AND %s AND (GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)>%s OR GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)>%s))`, schema, itemCondition, warehouseCondition, ownerCondition, boundG, boundN), itemID, warehouse, owner).Scan(&unbound)
	return unbound, err
}

// transferReservedTx returns all active batch reservations, including customer
// processing reservations that are not already represented in the generic table.
func transferReservedTx(ctx context.Context, tx pgx.Tx, schema string, batchID, itemID, owner int64, warehouse string, product bool) (int64, int64, error) {
	var g, n int64
	has, err := stockSchemaRelationExistsTx(ctx, tx, schema, "work_order_material_reservation_batches")
	if err != nil {
		return 0, 0, err
	}
	if has {
		column := "material_batch_id"
		if product {
			column = "stock_batch_id"
		}
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)),0)::bigint,
COALESCE(SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)),0)::bigint
FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id
WHERE b.%[2]s=$1 AND b.warehouse=$2 AND b.owner_customer_id=$3 AND b.status='reserved' AND r.status='reserved'`, schema, column), batchID, warehouse, owner).Scan(&g, &n)
		if err != nil {
			return 0, 0, err
		}
	}
	pg, pn, err := operationOtherProcessingReservedTx(ctx, tx, schema, -1, batchID, itemID, owner, warehouse, product)
	if err != nil {
		return 0, 0, err
	}
	return g + pg, n + pn, nil
}

func transferBatchRetainedTx(ctx context.Context, tx pgx.Tx, schema string, batchID int64) (bool, error) {
	has, err := stockSchemaRelationExistsTx(ctx, tx, schema, "customer_stock_entitlement_receipts")
	if err != nil || !has {
		return false, err
	}
	var retained bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.customer_stock_entitlement_receipts r JOIN %[1]s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE r.stock_batch_id=$1 AND e.status='active')`, schema), batchID).Scan(&retained)
	return retained, err
}

func finishedTransferBalancesMatchTx(ctx context.Context, tx pgx.Tx, schema string, productID, specID, specG, owner int64, warehouse string) (bool, error) {
	var units, loose int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=$3 AND owner_customer_id=$4 AND warehouse=$5`, schema), productID, specID, specG, owner, warehouse).Scan(&units, &loose)
	if err != nil && err != pgx.ErrNoRows {
		return false, err
	}
	var batchG, batchN int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(b.remaining_g),0)::bigint,COALESCE(SUM(b.remaining_units),0)::bigint
FROM %[1]s.stock_batches b LEFT JOIN LATERAL (SELECT l.warehouse FROM %[1]s.stock_ledger_entries l
WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g AND l.owner_customer_id=b.owner_customer_id ORDER BY l.id DESC LIMIT 1) l ON true
WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3 AND b.owner_customer_id=$4 AND COALESCE(l.warehouse,'finished_goods')=$5`, schema), productID, specID, specG, owner, warehouse).Scan(&batchG, &batchN)
	if err != nil {
		return false, err
	}
	if specID > 0 {
		return units == batchN, nil
	}
	return units == batchN && units*specG+loose == batchG, nil
}

func validateFinishedTransferBatchTx(ctx context.Context, tx pgx.Tx, schema string, item stockapp.StockDocumentItemRow) error {
	unbound, err := hasUnboundTransferReservationTx(ctx, tx, schema, item.ProductID, item.OwnerCustomerID, item.FromWarehouse, true)
	if err != nil {
		return err
	}
	if unbound {
		return fmt.Errorf("存在未绑定批次的预留，请先核对")
	}
	for _, warehouse := range []string{item.FromWarehouse, item.ToWarehouse} {
		var customerID int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0) FROM %s.warehouses WHERE code=$1 AND active=true`, schema), warehouse).Scan(&customerID)
		if err == pgx.ErrNoRows {
			return fmt.Errorf("转仓仓库不存在或已停用")
		}
		if err != nil {
			return err
		}
		if customerID > 0 && customerID != item.OwnerCustomerID {
			return fmt.Errorf("仓库货主与来源批次不一致")
		}
	}
	matched, err := finishedTransferBalancesMatchTx(ctx, tx, schema, item.ProductID, item.BomSpecID, item.SpecG, item.OwnerCustomerID, item.FromWarehouse)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("来源仓成品库存与批次汇总不一致，请先核对")
	}
	matched, err = finishedTransferBalancesMatchTx(ctx, tx, schema, item.ProductID, item.BomSpecID, item.SpecG, item.OwnerCustomerID, item.ToWarehouse)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("目标仓成品库存与批次汇总不一致，请先核对")
	}
	var id, g, n int64
	var quality string
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.id,b.remaining_g,b.remaining_units,COALESCE(b.quality_status,'unchecked')
FROM %[1]s.stock_batches b
LEFT JOIN LATERAL (SELECT warehouse FROM %[1]s.stock_ledger_entries l
 WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
 AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g AND l.owner_customer_id=b.owner_customer_id
 ORDER BY l.id DESC LIMIT 1) l ON true
WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3
 AND b.batch_code=$4 AND b.owner_customer_id=$5 AND COALESCE(l.warehouse,'finished_goods')=$6 FOR UPDATE OF b`, schema),
		item.ProductID, item.BomSpecID, item.SpecG, item.BatchCode, item.OwnerCustomerID, item.FromWarehouse).Scan(&id, &g, &n, &quality)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("来源批次与仓库、商品、规格或货主不匹配，请刷新后重新选择")
	}
	if err != nil {
		return err
	}
	if quality == "hold" || quality == "reject" {
		return fmt.Errorf("来源批次处于质检冻结状态")
	}
	retained, err := transferBatchRetainedTx(ctx, tx, schema, id)
	if err != nil {
		return err
	}
	if retained {
		return fmt.Errorf("来源批次包含客户寄存库存，不能普通转仓")
	}
	rg, rn, err := transferReservedTx(ctx, tx, schema, id, item.ProductID, item.OwnerCustomerID, item.FromWarehouse, true)
	if err != nil {
		return err
	}
	if item.BomSpecID > 0 {
		if item.QtyG != 0 || item.QtyUnits > nonnegativeStockQty(n-rn) {
			return fmt.Errorf("来源批次可转库存不足，请刷新")
		}
		return nil
	}
	if item.QtyUnits > 0 && item.QtyG > 0 {
		return fmt.Errorf("每行只能转整件或散装克数")
	}
	loose := g - n*item.SpecG
	reservedLoose := nonnegativeStockQty(rg - rn*item.SpecG)
	if loose < 0 || item.QtyUnits > nonnegativeStockQty(n-rn) || item.QtyG > nonnegativeStockQty(loose-reservedLoose) {
		return fmt.Errorf("来源批次可转库存不足或库存需核对，请刷新")
	}
	return nil
}
