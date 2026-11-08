package stock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
)

// OwnershipReturnStockTx exposes only a real receipt of the selected retained
// sale. It keeps the normal quality, ledger, summary and reservation checks.
func (r Repository) OwnershipReturnStockTx(ctx context.Context, tx pgx.Tx, receiptID, orderID int64) (stockapp.TransferOptionRow, error) {
	var product, spec, variant, batch, owner int64
	var warehouse, unit, code string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.item_id,b.bom_spec_id,b.bom_variant_id,b.id,b.owner_customer_id,receipt.warehouse_code,item.unit,b.batch_code
 FROM %[1]s.customer_stock_entitlement_receipts receipt JOIN %[1]s.customer_stock_entitlements e ON e.id=receipt.entitlement_id
 JOIN %[1]s.orders o ON o.id=e.source_order_id AND o.delivery_mode='customer_stock' AND NOT o.is_void AND o.customer_id=e.customer_id
 JOIN %[1]s.order_items item ON item.id=e.source_order_item_id AND item.order_id=o.id AND item.product_id=e.product_id AND item.bom_spec_id=e.bom_spec_id AND item.bom_variant_id=e.bom_variant_id
 JOIN %[1]s.stock_batches b ON b.id=receipt.stock_batch_id AND b.batch_code=receipt.batch_code AND b.item_type='finished_product' AND b.item_id=e.product_id AND b.bom_spec_id=e.bom_spec_id AND b.bom_variant_id=e.bom_variant_id AND b.spec_g=0 AND b.owner_customer_id=e.customer_id
 JOIN %[1]s.production_bom_version_variants v ON v.id=b.bom_variant_id AND v.bom_spec_id=b.bom_spec_id AND v.inventory_unit=item.unit
 WHERE receipt.id=$1 AND o.id=$2 AND e.received_qty>=receipt.quantity AND receipt.quantity>0 AND e.customer_id>0 AND e.spec_g=0 AND e.bom_spec_id>0 AND receipt.warehouse_code=e.warehouse_code
 AND (SELECT sum(other.quantity) FROM %[1]s.customer_stock_entitlement_receipts other WHERE other.stock_batch_id=b.id)<=b.qty_units
 AND EXISTS(SELECT 1 FROM %[1]s.stock_ledger_entries l WHERE l.source_batch_code=b.batch_code AND l.item_type='finished_product' AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.bom_variant_id=b.bom_variant_id AND l.spec_g=0 AND l.owner_customer_id=e.customer_id AND l.warehouse=receipt.warehouse_code AND l.qty_change_units>=receipt.quantity AND l.qty_change_g=0 AND l.source_doc_type=b.source_doc_type AND l.source_doc_id=b.source_doc_id)`, r.schema), receiptID, orderID).Scan(&product, &spec, &variant, &batch, &owner, &warehouse, &unit, &code)
	if err != nil {
		return stockapp.TransferOptionRow{}, err
	}
	options, err := r.listTransferOptionsTx(ctx, tx, stockapp.TransferOptionsQuery{Warehouse: warehouse, ItemType: "finished_product", ItemID: product, BomSpecID: spec, Limit: 1000000}, true)
	if err != nil {
		return stockapp.TransferOptionRow{}, err
	}
	for _, row := range options.Rows {
		if row.BatchID == batch && row.BatchCode == code && row.BomVariantID == variant && row.OwnerCustomerID == owner && row.InventoryUnit == unit {
			return row, nil
		}
	}
	return stockapp.TransferOptionRow{UnavailableReason: "原客户入库批次已不在原仓库，请核对实物与批次来源"}, nil
}

func (r Repository) postOrderOwnershipReturnTx(ctx context.Context, tx pgx.Tx, id int64, actor string) (int64, error) {
	var product, spec, variant, owner, quantity, sourceBatch, receiptID, orderID, entitlementID int64
	var warehouse, sourceWarehouse, name, code, quality string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id FROM %s.order_resolution_stock_returns WHERE id=$1`, r.schema), id).Scan(&product, &spec); err != nil {
		return 0, err
	}
	if err := infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, product, spec, 0); err != nil {
		return 0, err
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT x.product_id,x.bom_spec_id,x.bom_variant_id,x.source_owner_customer_id,x.quantity,x.batch_id,x.source_id,x.order_id,e.id,x.target_warehouse,x.source_warehouse,b.item_name,b.batch_code,x.quality
 FROM %[1]s.order_resolution_stock_returns x
 JOIN %[1]s.order_resolution_requests request ON request.id=x.request_id AND request.order_id=x.order_id AND request.status='pending' AND request.command_json->>'action'='return'
 JOIN %[1]s.order_resolution_steps step ON step.id=x.step_id AND step.request_id=request.id AND step.kind='return_stock' AND step.choice='receive' AND step.permission='stock.resolve' AND step.status IN('pending','failed')
 JOIN %[1]s.customer_stock_entitlement_receipts receipt ON receipt.id=x.source_id AND receipt.stock_batch_id=x.batch_id AND receipt.warehouse_code=x.source_warehouse
 JOIN %[1]s.customer_stock_entitlements e ON e.id=receipt.entitlement_id AND e.source_order_id=x.order_id AND e.source_order_item_id=x.order_item_id AND e.customer_id=x.source_owner_customer_id
 JOIN %[1]s.stock_batches b ON b.id=x.batch_id AND b.batch_code=receipt.batch_code AND b.item_type='finished_product' AND b.item_id=x.product_id AND b.bom_spec_id=x.bom_spec_id AND b.bom_variant_id=x.bom_variant_id AND b.spec_g=0 AND b.owner_customer_id=x.source_owner_customer_id AND b.unit_cost=x.unit_cost AND b.remaining_g=0 AND b.qty_g=0
 JOIN %[1]s.warehouses destination ON destination.code=x.target_warehouse AND destination.active AND COALESCE(destination.customer_id,0)=0 AND destination.kind IN('finished','customer_finished','customer_processing','customer')
 WHERE x.id=$1 AND x.status='posting' AND x.source_kind='ownership' AND x.source_owner_customer_id>0 AND x.target_owner_customer_id=0 AND x.target_warehouse<>x.source_warehouse
 AND x.quantity<=receipt.quantity-COALESCE((SELECT sum(previous.quantity) FROM %[1]s.order_resolution_stock_returns previous WHERE previous.source_kind='ownership' AND previous.source_id=x.source_id AND previous.status='posted'),0)
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(request.command_json->'return_selections') selected WHERE selected->>'source_kind'='ownership' AND (selected->>'source_id')::bigint=x.source_id AND (selected->>'batch_id')::bigint=x.batch_id AND (selected->>'quantity')::bigint=x.quantity AND selected->>'warehouse'=x.target_warehouse AND selected->>'quality'=x.quality AND selected->>'ownership'='company')
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(request.command_json->'dispositions') selected WHERE selected->>'kind'='customer_stock' AND (selected->>'id')::bigint=e.id AND selected->>'choice'='ownership_return')
 FOR UPDATE OF x,b,e`, r.schema), id).Scan(&product, &spec, &variant, &owner, &quantity, &sourceBatch, &receiptID, &orderID, &entitlementID, &warehouse, &sourceWarehouse, &name, &code, &quality)
	if err != nil {
		return 0, fmt.Errorf("客户货权退回记录与原收货权益不一致：%w", err)
	}
	physical, err := r.OwnershipReturnStockTx(ctx, tx, receiptID, orderID)
	if err != nil {
		return 0, err
	}
	if physical.BatchID != sourceBatch || physical.UnavailableReason != "" || quantity > physical.AvailableUnits {
		return 0, fmt.Errorf("原客户实物、预留或可退数量已变化，请重新核对")
	}
	var otherOwner bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.finished_inventory WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=0 AND warehouse=$3 AND owner_customer_id<>0 AND (onhand_units<>0 OR onhand_loose_g<>0))`, r.schema), product, spec, warehouse).Scan(&otherOwner); err != nil {
		return 0, err
	}
	if otherOwner {
		return 0, fmt.Errorf("公司接收仓存在其他货权的库存，请核对")
	}
	sourceUnits, sourceG, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, 0, sourceWarehouse, owner)
	if err != nil {
		return 0, err
	}
	targetUnits, targetG, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, 0, warehouse, 0)
	if err != nil {
		return 0, err
	}
	if sourceUnits < quantity {
		return 0, fmt.Errorf("原客户仓实物不足")
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=remaining_units-$2 WHERE id=$1 AND remaining_units>=$2`, r.schema), sourceBatch, quantity)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() != 1 {
		return 0, fmt.Errorf("原客户批次不足")
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('kferp.order_return_id',$1,true)", fmt.Sprint(id)); err != nil {
		return 0, err
	}
	defer func() { _, _ = tx.Exec(ctx, "SELECT set_config('kferp.order_return_id','',true)") }()
	if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, variant, 0, sourceWarehouse, sourceUnits-quantity, sourceG, owner); err != nil {
		return 0, err
	}
	if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: "finished_product", ItemID: product, ItemName: name, BomSpecID: spec, BomVariantID: variant, OwnerCustomerID: owner, Warehouse: sourceWarehouse, SourceDocType: "order_resolution_ownership_return_out", SourceDocID: id, SourceBatchCode: code, SourceBatchID: code, BeforeUnits: sourceUnits, ChangeUnits: -quantity, AfterUnits: sourceUnits - quantity, BeforeG: sourceG, AfterG: sourceG, Operator: actor}); err != nil {
		return 0, err
	}
	targetCode := fmt.Sprintf("ORRET-%d", id)
	qualityStatus := "hold"
	if quality == "restock" {
		qualityStatus = "passed"
	}
	var targetID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,source_doc_type,source_doc_id,source_batch_id,qty_units,remaining_units,quality_status,unit_cost,operator)VALUES($1,'finished_product',$2,$3,$4,$5,0,0,'order_resolution_return',$6,$7,$8,$8,$9,(SELECT unit_cost FROM %[1]s.order_resolution_stock_returns WHERE id=$6),$10)RETURNING id`, r.schema), targetCode, product, name, spec, variant, id, code, quantity, qualityStatus, actor).Scan(&targetID)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_stock_returns SET target_batch_id=$2,target_batch_code=$3 WHERE id=$1 AND status='posting'`, r.schema), id, targetID, targetCode); err != nil {
		return 0, err
	}
	if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, variant, 0, warehouse, targetUnits+quantity, targetG, 0); err != nil {
		return 0, err
	}
	if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: "finished_product", ItemID: product, ItemName: name, BomSpecID: spec, BomVariantID: variant, OwnerCustomerID: 0, Warehouse: warehouse, SourceDocType: "order_resolution_return", SourceDocID: id, SourceBatchCode: targetCode, SourceBatchID: code, BeforeUnits: targetUnits, ChangeUnits: quantity, AfterUnits: targetUnits + quantity, BeforeG: targetG, AfterG: targetG, Operator: actor}); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_stock_returns SET status='posted' WHERE id=$1`, r.schema), id); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_stock_entitlements SET returned_qty=returned_qty+$2,updated_at=now() WHERE id=$1`, r.schema), entitlementID, quantity); err != nil {
		return 0, err
	}
	return targetID, nil
}
