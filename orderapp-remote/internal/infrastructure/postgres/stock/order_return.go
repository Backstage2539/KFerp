package stock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	infra "orderapp/internal/infrastructure/postgres"
)

// PostOrderReturnTx receives a validated processing-record id, not arbitrary
// inbound quantities. The pending warehouse task and its exact command must
// authorize this record; ordinary stock endpoints cannot supply such a record.
func (r Repository) PostOrderReturnTx(ctx context.Context, tx pgx.Tx, id int64, actor string) (int64, error) {
	var kind string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_kind FROM %s.order_resolution_stock_returns WHERE id=$1`, r.schema), id).Scan(&kind); err != nil {
		return 0, err
	}
	if kind == "ownership" {
		return r.postOrderOwnershipReturnTx(ctx, tx, id, actor)
	}
	var product, spec, variant, owner, quantity, sourceBatch int64
	var warehouse, name, code, quality string
	var cost float64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,bom_spec_id FROM %s.order_resolution_stock_returns WHERE id=$1`, r.schema), id).Scan(&product, &spec); err != nil {
		return 0, err
	}
	if err := infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, product, spec, 0); err != nil {
		return 0, err
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT x.product_id,x.bom_spec_id,x.bom_variant_id,x.target_owner_customer_id,x.quantity,x.batch_id,x.target_warehouse,b.item_name,b.batch_code,x.quality,x.unit_cost
 FROM %[1]s.order_resolution_stock_returns x
 JOIN %[1]s.order_resolution_requests request ON request.id=x.request_id AND request.order_id=x.order_id AND request.status='pending' AND request.command_json->>'action'='return'
 JOIN %[1]s.order_resolution_steps step ON step.id=x.step_id AND step.request_id=request.id AND step.kind='return_stock' AND step.choice='receive' AND step.permission='stock.resolve' AND step.status IN('pending','failed')
 JOIN %[1]s.stock_batches b ON b.id=x.batch_id AND b.item_type='finished_product' AND b.item_id=x.product_id AND b.bom_spec_id=x.bom_spec_id AND b.bom_variant_id=x.bom_variant_id AND b.spec_g=0 AND b.owner_customer_id=x.source_owner_customer_id AND b.unit_cost=x.unit_cost
 JOIN %[1]s.order_stock_deductions d ON d.id=x.source_id AND d.order_id=x.order_id AND d.batch_id=b.id AND d.batch_code=b.batch_code AND d.product_id=b.item_id AND d.bom_spec_id=b.bom_spec_id AND d.bom_variant_id=b.bom_variant_id AND d.deducted_g=0 AND d.spec_g=0
 JOIN %[1]s.order_items item ON item.id=x.order_item_id AND item.order_id=x.order_id AND item.product_id=b.item_id AND item.bom_spec_id=b.bom_spec_id
 JOIN %[1]s.warehouses destination ON destination.code=x.target_warehouse AND destination.active AND COALESCE(destination.customer_id,0)=x.target_owner_customer_id
 WHERE x.id=$1 AND x.status='posting' AND x.source_kind='shipment' AND x.source_owner_customer_id=x.target_owner_customer_id
 AND EXISTS(SELECT 1 FROM %[1]s.order_stock_batch_allocations a WHERE a.order_id=x.order_id AND a.order_item_id=x.order_item_id AND a.product_id=x.product_id AND a.bom_spec_id=x.bom_spec_id AND a.bom_variant_id=x.bom_variant_id AND a.batch_id=x.batch_id AND a.batch_code=b.batch_code AND a.warehouse=x.source_warehouse AND a.allocated_units>=d.deducted_units AND a.allocated_g=0 AND a.spec_g=0)
 AND x.quantity<=d.deducted_units-COALESCE((SELECT sum(previous.quantity) FROM %[1]s.order_resolution_stock_returns previous WHERE previous.source_kind=x.source_kind AND previous.source_id=x.source_id AND previous.status='posted'),0)
 AND EXISTS(SELECT 1 FROM jsonb_array_elements(request.command_json->'return_selections') selected WHERE selected->>'source_kind'=x.source_kind AND (selected->>'source_id')::bigint=x.source_id AND (selected->>'batch_id')::bigint=x.batch_id AND (selected->>'quantity')::bigint=x.quantity AND selected->>'warehouse'=x.target_warehouse AND selected->>'quality'=x.quality AND selected->>'ownership'=CASE WHEN x.target_owner_customer_id=0 THEN 'company' ELSE 'customer' END)
 FOR UPDATE OF x,b`, r.schema), id).Scan(&product, &spec, &variant, &owner, &quantity, &sourceBatch, &warehouse, &name, &code, &quality, &cost)
	if err != nil {
		return 0, fmt.Errorf("退货记录、原出库或仓库确认不匹配：%w", err)
	}
	if err = infra.LockFinishedInventoryIdentityTx(ctx, tx, r.schema, product, spec, 0); err != nil {
		return 0, err
	}
	var otherOwner bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.finished_inventory WHERE product_id=$1 AND bom_spec_id=$2 AND spec_g=0 AND warehouse=$3 AND owner_customer_id<>$4 AND (onhand_units<>0 OR onhand_loose_g<>0))`, r.schema), product, spec, warehouse, owner).Scan(&otherOwner); err != nil {
		return 0, err
	}
	if otherOwner {
		return 0, fmt.Errorf("退回仓库现有实物货权不一致，请先核对库存")
	}
	beforeUnits, beforeG, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, 0, warehouse, owner)
	if err != nil {
		return 0, err
	}
	targetCode := fmt.Sprintf("ORRET-%d", id)
	qualityStatus := "hold"
	if quality == "restock" {
		qualityStatus = "passed"
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('kferp.order_return_id',$1,true)", fmt.Sprint(id)); err != nil {
		return 0, err
	}
	defer func() { _, _ = tx.Exec(ctx, "SELECT set_config('kferp.order_return_id','',true)") }()
	var targetID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,source_doc_type,source_doc_id,source_batch_id,qty_units,remaining_units,quality_status,unit_cost,operator)VALUES($1,'finished_product',$2,$3,$4,$5,0,$6,'order_resolution_return',$7,$8,$9,$9,$10,$11,$12)RETURNING id`, r.schema), targetCode, product, name, spec, variant, owner, id, code, quantity, qualityStatus, cost, actor).Scan(&targetID)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_stock_returns SET target_batch_id=$2,target_batch_code=$3 WHERE id=$1 AND status='posting'`, r.schema), id, targetID, targetCode); err != nil {
		return 0, err
	}
	if err = upsertFinishedInventoryIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, variant, 0, warehouse, beforeUnits+quantity, beforeG, owner); err != nil {
		return 0, err
	}
	if err = insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: "finished_product", ItemID: product, ItemName: name, BomSpecID: spec, BomVariantID: variant, OwnerCustomerID: owner, Warehouse: warehouse, SourceDocType: "order_resolution_return", SourceDocID: id, SourceBatchCode: targetCode, SourceBatchID: code, BeforeUnits: beforeUnits, ChangeUnits: quantity, AfterUnits: beforeUnits + quantity, BeforeG: beforeG, AfterG: beforeG, Operator: actor}); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_stock_returns SET status='posted' WHERE id=$1`, r.schema), id); err != nil {
		return 0, err
	}
	return targetID, nil
}
