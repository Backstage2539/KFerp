package production

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"

	app "orderapp/internal/application/production"
	"orderapp/internal/infrastructure/postgres/customerstock"
)

// The completion event already paid for its inputs. Receipt only moves its
// output and carried cost; it must never run the legacy whole-order recipe.
func (r Repository) receiveOperationOutputTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, receipt app.OperationCompletionReceipt, cost float64, actor string) (int64, error) {
	if wo.OutputType == "material" {
		return r.receiveOperationMaterialTx(ctx, tx, wo, receipt, cost, actor)
	}
	productID, bomSpecID, bomVariantID, err := resolveOperationOutputProductIdentityTx(ctx, tx, r.schema, wo)
	if err != nil {
		return 0, err
	}
	run := ProduceRunRow{ID: wo.RunningItemID, BatchID: wo.BatchID, ProductID: productID, Product: wo.ProductName, BomSpecID: bomSpecID, BomVariantID: bomVariantID, SpecG: wo.SpecG, OrderNos: wo.OrderNos}
	var jobID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT job_card_id FROM %s.production_operation_events WHERE id=$1`, r.schema), receipt.EventID).Scan(&jobID); err != nil {
		return 0, err
	}
	run.OwnerCustomerID, err = warehouseCustomerID(ctx, tx, r.schema, receipt.Warehouse)
	if err != nil {
		return 0, err
	}
	add := InvQty{}
	if factor := app.OperationUnitFactor(receipt.OutputUnit); factor > 0 {
		add.LooseG = int64(math.Round(receipt.OutputQty * factor))
	} else {
		add.Units = int64(receipt.OutputQty)
	}
	grams := finishedTotalG(run.SpecG, add.Units, add.LooseG)
	before := InvQty{}
	before.Units, before.LooseG, err = finishedInventoryQtyIdentityOwnedTx(ctx, tx, r.schema, run.ProductID, run.BomSpecID, run.SpecG, receipt.Warehouse, run.OwnerCustomerID)
	if err != nil {
		return 0, err
	}
	after := InvQty{Units: before.Units + add.Units, LooseG: before.LooseG + add.LooseG}
	if err = r.upsertOperationInventoryTx(ctx, tx, run.ProductID, run.BomSpecID, run.BomVariantID, run.SpecG, receipt.Warehouse, after.Units, after.LooseG, run.OwnerCustomerID); err != nil {
		return 0, err
	}
	batch := fmt.Sprintf("FP-OP-%010d", receipt.EventID)
	if err = recordFinishedProductOperationMovementTx(ctx, tx, r.schema, receipt.EventID, batch, run, before, add, after, grams, receipt.Warehouse, actor); err != nil {
		return 0, err
	}
	unitCost := cost / receipt.OutputQty
	if grams > 0 {
		unitCost = cost * 1000 / float64(grams)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET unit_cost=$2 WHERE batch_code=$1 AND item_type='finished_product' AND item_id=$3`, r.schema), batch, unitCost, run.ProductID); err != nil {
		return 0, err
	}
	entry, err := createStockEntryRecordTx(ctx, tx, r.schema, app.StockEntryCommand{EntryType: "finished_receipt", Purpose: "manufacture", WorkOrderID: wo.ID, JobCardID: jobID, RunningItemID: run.ID, SourceType: "operation_completion", SourceID: receipt.EventID, Operator: actor, Note: "工序完成直接入库；来源工单 " + wo.WorkOrderNo, Items: []app.StockEntryItemCommand{{ProductID: run.ProductID, BomSpecID: run.BomSpecID, BomVariantID: run.BomVariantID, ItemType: stockItemTypeFinishedProduct, ItemName: run.Product, SpecG: run.SpecG, OwnerCustomerID: run.OwnerCustomerID, ToWarehouse: receipt.Warehouse, QtyG: grams, QtyUnits: add.Units, BatchCode: batch, UnitCost: unitCost}}}, false)
	if err != nil {
		return 0, err
	}
	if err = customerstock.ReceiveProductionTx(ctx, tx, r.schema, run.ID, run.ProductID, run.BomSpecID, run.BomVariantID, run.SpecG, add.Units, fmt.Sprintf("operation:%d", receipt.EventID), batch, receipt.Warehouse, run.OrderNos, actor); err != nil {
		return 0, err
	}
	if err = allocateCustomerProcessingOutputReservationsTx(ctx, tx, r.schema, run.ID, run.ProductID, run.BomSpecID, run.BomVariantID, run.SpecG, add.Units, batch, receipt.Warehouse, actor); err != nil {
		return 0, err
	}
	if err = allocateFinishedProductOutputToDownstreamReservationsTx(ctx, tx, r.schema, run.ID, run.ProductID, run.BomSpecID, run.BomVariantID, run.SpecG, grams, add.Units); err != nil {
		return 0, err
	}
	no, err := nextCompletionNoForRunningItemTx(ctx, tx, r.schema, run.ID)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_logs(running_item_id,completion_no,batch_id,product_id,bom_spec_id,bom_variant_id,product_name,spec_g,order_nos,planned_need_g,input_g,bom_yield_rate,finished_units,finished_loose_g,finished_total_g,actual_yield_rate,started_by,started_at,finished_by,finished_at,inventory_units_before,inventory_loose_g_before,inventory_units_after,inventory_loose_g_after,material_summary,created_at)
 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,CASE WHEN e.input_unit='kg' THEN ROUND(e.input_qty*1000) WHEN e.input_unit='g' THEN ROUND(e.input_qty) ELSE 0 END,1,$11,$12,$13,1,run.started_by,run.started_at,$14,now(),$15,$16,$17,$18,'[]'::jsonb,now() FROM %[1]s.produce_running_items run JOIN %[1]s.production_operation_events e ON e.id=$19 WHERE run.id=$1`, r.schema), run.ID, no, run.BatchID, run.ProductID, run.BomSpecID, run.BomVariantID, run.Product, run.SpecG, run.OrderNos, wo.PlannedOutputG, add.Units, add.LooseG, grams, actor, before.Units, before.LooseG, after.Units, after.LooseG, receipt.EventID)
	return entry.ID, err
}

// Legacy work orders can retain an output_product_id that pointed to a retired
// child SKU. Use that ID only while it is still a valid active identity. When
// it is gone, fall back to the frozen parent product only if the exact frozen
// spec and variant are still present in that product's published BOM.
func resolveOperationOutputProductIdentityTx(ctx context.Context, tx pgx.Tx, schema string, wo app.WorkOrderRow) (int64, int64, int64, error) {
	candidateID := wo.OutputProductID
	if candidateID <= 0 {
		candidateID = wo.ProductID
	}
	if candidateID <= 0 {
		return 0, 0, 0, fmt.Errorf("工单没有有效的成品商品身份")
	}
	var authorityViewExists bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, schema+`.product_bom_spec_authorities`).Scan(&authorityViewExists); err != nil {
		return 0, 0, 0, err
	}
	if !authorityViewExists {
		// Older isolated fixtures predate the spec-authority projection. Preserve
		// their direct-product path only when the frozen order carries no spec.
		var active bool
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.products WHERE id=$1`, schema), candidateID).Scan(&active)
		if err == nil && active && wo.BomSpecID == 0 && wo.BomVariantID == 0 {
			return candidateID, 0, 0, nil
		}
		if err != nil && err != pgx.ErrNoRows {
			return 0, 0, 0, err
		}
		return 0, 0, 0, fmt.Errorf("工单缺少可校验的商品规格身份资料")
	}

	var candidateExists, candidateActive, candidateConfigured bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT true,product.active,COALESCE(authority.configured,false)
		FROM %s.products product
		LEFT JOIN %s.product_bom_spec_authorities authority ON authority.product_id=product.id
		WHERE product.id=$1
	`, schema, schema), candidateID).Scan(&candidateExists, &candidateActive, &candidateConfigured)
	if err != nil && err != pgx.ErrNoRows {
		return 0, 0, 0, err
	}
	if candidateExists && candidateActive {
		if candidateConfigured {
			valid, validateErr := operationOutputSpecIdentityValidTx(ctx, tx, schema, candidateID, wo.BomSpecID, wo.BomVariantID)
			if validateErr != nil {
				return 0, 0, 0, validateErr
			}
			if valid {
				return candidateID, wo.BomSpecID, wo.BomVariantID, nil
			}
		} else if wo.BomSpecID == 0 && wo.BomVariantID == 0 {
			return candidateID, 0, 0, nil
		}
	}

	// A different output ID can be a retired legacy child. The work order's
	// frozen parent identity is authoritative only when its full variant still
	// resolves against the current published BOM, so never infer or drop a spec.
	if wo.ProductID > 0 && wo.ProductID != candidateID {
		valid, validateErr := operationOutputSpecIdentityValidTx(ctx, tx, schema, wo.ProductID, wo.BomSpecID, wo.BomVariantID)
		if validateErr != nil {
			return 0, 0, 0, validateErr
		}
		if valid {
			return wo.ProductID, wo.BomSpecID, wo.BomVariantID, nil
		}
	}
	if wo.ProductID == candidateID && candidateExists && candidateActive && !candidateConfigured && wo.BomSpecID != 0 {
		return 0, 0, 0, fmt.Errorf("工单冻结了规格，但目标商品未配置规格身份；请核对商品与工单冻结资料")
	}
	if !candidateExists {
		return 0, 0, 0, fmt.Errorf("工单目标商品 %d 已不存在，且冻结商品规格无法匹配当前已发布配方", candidateID)
	}
	return 0, 0, 0, fmt.Errorf("工单目标商品与冻结规格不匹配，请核对工单资料")
}

func operationOutputSpecIdentityValidTx(ctx context.Context, tx pgx.Tx, schema string, productID, bomSpecID, bomVariantID int64) (bool, error) {
	if productID <= 0 || bomSpecID <= 0 || bomVariantID <= 0 {
		return false, nil
	}
	var valid bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS (
			SELECT 1
			FROM %s.product_bom_spec_authorities authority
			JOIN %s.products product ON product.id=authority.product_id AND product.active=true
			JOIN %s.production_bom_output_bindings binding
			  ON binding.output_type='product' AND binding.output_id=product.id AND binding.is_default=true
			JOIN %s.production_bom_versions version
			  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
			JOIN %s.production_bom_specs spec ON spec.id=$2 AND spec.bom_id=binding.bom_id
			JOIN %s.production_bom_version_variants variant
			  ON variant.id=$3 AND variant.version_id=version.id AND variant.bom_spec_id=spec.id
			WHERE authority.product_id=$1 AND authority.configured=true
		)
	`, schema, schema, schema, schema, schema, schema), productID, bomSpecID, bomVariantID).Scan(&valid)
	return valid, err
}

func (r Repository) receiveOperationMaterialTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, receipt app.OperationCompletionReceipt, cost float64, actor string) (int64, error) {
	var owner, beforeG, beforeUnits int64
	var jobID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT job_card_id FROM %s.production_operation_events WHERE id=$1`, r.schema), receipt.EventID).Scan(&jobID); err != nil {
		return 0, err
	}
	var name string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT owner_customer_id,name FROM %s.materials WHERE id=$1 FOR UPDATE`, r.schema), wo.OutputMaterialID).Scan(&owner, &name)
	if err != nil {
		return 0, err
	}
	warehouseOwner, err := warehouseCustomerID(ctx, tx, r.schema, receipt.Warehouse)
	if err != nil {
		return 0, err
	}
	if warehouseOwner != owner && receipt.Warehouse != "wip" {
		return 0, fmt.Errorf("入库仓与产出物料货主不匹配")
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(l.qty_g),0)::bigint,COALESCE(SUM(l.qty_units),0)::bigint FROM %[1]s.material_batch_locations l JOIN %[1]s.material_batches b ON b.id=l.material_batch_id WHERE l.material_id=$1 AND l.warehouse=$2 AND b.owner_customer_id=$3`, r.schema), wo.OutputMaterialID, receipt.Warehouse, owner).Scan(&beforeG, &beforeUnits)
	if err != nil {
		return 0, err
	}
	var grams, units int64
	if factor := app.OperationUnitFactor(receipt.OutputUnit); factor > 0 {
		grams = int64(math.Round(receipt.OutputQty * factor))
	} else {
		units = int64(receipt.OutputQty)
	}
	unitCost := cost / receipt.OutputQty
	if grams > 0 {
		unitCost = cost * 1000 / float64(grams)
	}
	batch := fmt.Sprintf("MP-OP-%010d", receipt.EventID)
	var materialBatchID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.material_batches(batch_code,material_id,material_name,owner_customer_id,supplier,receipt_id,received_g,qty_g,qty_units,remaining_g,remaining_units,unit_cost,status,quality_status,note,received_at,created_at) VALUES($1,$2,$3,$4,'production',$5,$6,$6,$7,$6,$7,$8,'active','pass',$9,now(),now()) RETURNING id`, r.schema), batch, wo.OutputMaterialID, name, owner, wo.ID, grams, units, unitCost, "来源工单 "+wo.WorkOrderNo).Scan(&materialBatchID)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at) VALUES($1,$2,$3,$4,$5,$6,now())`, r.schema), materialBatchID, batch, wo.OutputMaterialID, receipt.Warehouse, grams, units); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,owner_customer_id,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,unit_cost,operator,created_at) VALUES($1,'material',$2,$3,$4,0,'operation_completion',$5,$6,$7,$8,$7,$8,'pass',$9,$10,now())`, r.schema), batch, wo.OutputMaterialID, name, owner, receipt.EventID, wo.BatchID, grams, units, unitCost, actor); err != nil {
		return 0, err
	}
	if owner == 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=onhand_g+$2,onhand_units=onhand_units+$3,updated_at=now() WHERE id=$1`, r.schema), wo.OutputMaterialID, grams, units); err != nil {
			return 0, err
		}
	}
	if err = insertStockLedgerEntryOwnedTx(ctx, tx, r.schema, stockItemTypeMaterial, wo.OutputMaterialID, name, owner, 0, receipt.Warehouse, "operation_completion", receipt.EventID, batch, wo.BatchID, stockLedgerQty{BeforeG: beforeG, ChangeG: grams, AfterG: beforeG + grams, BeforeUnits: beforeUnits, ChangeUnits: units, AfterUnits: beforeUnits + units}, actor); err != nil {
		return 0, err
	}
	entry, err := createStockEntryRecordTx(ctx, tx, r.schema, app.StockEntryCommand{EntryType: "finished_receipt", Purpose: "manufacture", WorkOrderID: wo.ID, JobCardID: jobID, RunningItemID: wo.RunningItemID, SourceType: "operation_completion", SourceID: receipt.EventID, Operator: actor, Note: "工序完成直接入库；来源工单 " + wo.WorkOrderNo, Items: []app.StockEntryItemCommand{{MaterialID: wo.OutputMaterialID, ItemType: stockItemTypeMaterial, ItemName: name, OwnerCustomerID: owner, ToWarehouse: receipt.Warehouse, QtyG: grams, QtyUnits: units, BatchCode: batch, UnitCost: unitCost}}}, false)
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_material_receipts(work_order_id,running_item_id,material_batch_id,stock_entry_id,finished_g,finished_units,input_g,material_cost,operation_cost,completion_mode) SELECT $1,$2,$3,$4,$5,$6,CASE WHEN input_unit='kg' THEN ROUND(input_qty*1000) WHEN input_unit='g' THEN ROUND(input_qty) ELSE 0 END,GREATEST(0,$7-operation_cost),operation_cost,'operation' FROM %s.production_operation_events WHERE id=$8`, r.schema, r.schema), wo.ID, wo.RunningItemID, materialBatchID, entry.ID, grams, units, cost, receipt.EventID); err != nil {
		return 0, err
	}
	err = allocateMaterialOutputToDownstreamReservationsTx(ctx, tx, r.schema, wo.ID, wo.OutputMaterialID, materialBatchID, batch, grams, units)
	return entry.ID, err
}
