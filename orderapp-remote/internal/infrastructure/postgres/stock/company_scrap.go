package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/stock"
)

// This is called inside the order processing transaction after both approvals.
// It deducts only the selected original batch and never chooses a substitute.
func (r Repository) PostResolutionCompanyScrapTx(ctx context.Context, tx pgx.Tx, id int64, actor string) error {
	var product, spec, variant, batch, quantity int64
	var warehouse, code, name string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT x.product_id,x.bom_spec_id,x.bom_variant_id,x.batch_id,x.quantity,x.warehouse,x.batch_code,b.item_name
 FROM %[1]s.order_resolution_company_scraps x JOIN %[1]s.order_resolution_requests request ON request.id=x.request_id AND request.status='pending' AND NOT request.applied
 JOIN %[1]s.stock_batches b ON b.id=x.batch_id AND b.batch_code=x.batch_code AND b.item_type='finished_product' AND b.item_id=x.product_id AND b.bom_spec_id=x.bom_spec_id AND b.bom_variant_id=x.bom_variant_id AND b.spec_g=0 AND b.owner_customer_id=0 AND b.unit_cost=x.unit_cost AND b.remaining_g=0
 JOIN %[1]s.production_batch_costs c ON c.id=x.cost_record_id AND c.total_cost*100>=x.cost_cents
 WHERE x.id=$1 AND x.status='posting' AND x.cost_cents=round(x.quantity*x.unit_cost*100) AND b.remaining_units=(x.snapshot_json->'batch'->>'remaining_units')::bigint
 AND EXISTS(SELECT 1 FROM %[1]s.work_orders w WHERE w.production_plan_id=x.plan_id AND w.running_item_id=c.running_item_id AND ((b.source_doc_type='production_run' AND b.source_doc_id=w.running_item_id) OR (b.source_doc_type IN('production_operation_event','operation_completion') AND b.source_doc_id IN(SELECT id FROM %[1]s.production_operation_events WHERE work_order_id=w.id)))) FOR UPDATE OF x,b`, r.schema), id).Scan(&product, &spec, &variant, &batch, &quantity, &warehouse, &code, &name)
	if err != nil {
		return fmt.Errorf("报损原批次、原生产和成本不一致：%w", err)
	}
	options, err := r.ListCompanyScrapOptionsTx(ctx, tx, app.TransferOptionsQuery{Warehouse: warehouse, ItemType: "finished_product", ItemID: product, BomSpecID: spec, Limit: 1000000})
	if err != nil {
		return err
	}
	found := false
	for _, row := range options.Rows {
		if row.BatchID == batch && row.BomVariantID == variant && row.OwnerCustomerID == 0 && row.UnavailableReason == "" && row.AvailableUnits >= quantity {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("原批次实物或可用量不足，不能占用其他需求或预留")
	}
	before, grams, err := finishedInventoryQtyIdentityOwnedStockTx(ctx, tx, r.schema, product, spec, 0, warehouse, 0)
	if err != nil {
		return err
	}
	if before < quantity || grams != 0 {
		return fmt.Errorf("原仓库规格库存不足或计量不一致")
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=remaining_units-$2 WHERE id=$1 AND remaining_units>=$2`, r.schema), batch, quantity)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("原批次不足")
	}
	tag, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=onhand_units-$5 WHERE product_id=$1 AND bom_spec_id=$2 AND bom_variant_id=$3 AND spec_g=0 AND owner_customer_id=0 AND warehouse=$4 AND onhand_units>=$5`, r.schema), product, spec, variant, warehouse, quantity)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("原仓库存身份或数量已变化")
	}
	return insertLedgerTx(ctx, tx, r.schema, ledgerEntry{ItemType: "finished_product", ItemID: product, ItemName: name, BomSpecID: spec, BomVariantID: variant, Warehouse: warehouse, SourceDocType: "order_resolution_company_scrap", SourceDocID: id, SourceBatchCode: code, SourceBatchID: code, BeforeUnits: before, ChangeUnits: -quantity, AfterUnits: before - quantity, Operator: actor})
}
