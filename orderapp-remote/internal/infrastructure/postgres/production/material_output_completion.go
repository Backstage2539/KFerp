package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r Repository) completeMaterialOutputWorkOrder(ctx context.Context, known productionapp.WorkOrderRow, cmd productionapp.WorkOrderCompleteCommand) (productionapp.WorkOrderCompleteResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wo productionapp.WorkOrderRow
	var materialSnapshot string
	var run ProduceRunRow
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.id,wo.work_order_no,wo.running_item_id,wo.production_plan_id,wo.production_plan_item_id,wo.batch_id,
		       COALESCE(NULLIF(wo.output_type,''),'product'),wo.output_product_id,wo.output_material_id,wo.output_name,wo.output_qty::float8,wo.output_unit,
		       wo.product_id,wo.product_name,wo.spec_g,wo.planned_g,wo.planned_output_g,wo.status,
		       wo.bom_version_id,wo.operation_template_id,wo.target_warehouse,
		       COALESCE(wo.material_snapshot,'[]'::jsonb)::text,
		       run.id,run.batch_id,run.need_g,run.input_g,run.started_by,run.started_at
		FROM %s.work_orders wo
		JOIN %s.produce_running_items run ON run.id=wo.running_item_id
		WHERE wo.id=$1
		FOR UPDATE OF wo,run
	`, r.schema, r.schema), known.ID).Scan(
		&wo.ID, &wo.WorkOrderNo, &wo.RunningItemID, &wo.ProductionPlanID, &wo.ProductionPlanItemID, &wo.BatchID,
		&wo.OutputType, &wo.OutputProductID, &wo.OutputMaterialID, &wo.OutputName, &wo.OutputQty, &wo.OutputUnit,
		&wo.ProductID, &wo.ProductName, &wo.SpecG, &wo.PlannedG, &wo.PlannedOutputG, &wo.Status,
		&wo.BomVersionID, &wo.OperationTemplateID, &wo.TargetWarehouse,
		&materialSnapshot,
		&run.ID, &run.BatchID, &run.NeedG, &run.InputG, &run.StartedBy, &run.StartedAtTime,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("material output work order is not running")
		}
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if wo.OutputType != "material" || wo.OutputMaterialID <= 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("material output work order required")
	}
	if wo.Status == "completed" {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("work order already completed")
	}
	if wo.Status != "running" && wo.Status != "partially_completed" {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("material output work order must be running")
	}
	var incomplete int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(1)::bigint FROM %s.job_cards
		WHERE work_order_id=$1 AND status NOT IN ('completed','cancelled')
	`, r.schema), wo.ID).Scan(&incomplete); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if incomplete > 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("work order has unfinished job cards")
	}

	finishedG := cmd.FinishedQtyG
	finishedUnits := cmd.FinishedQtyUnits
	if finishedG <= 0 && finishedUnits <= 0 {
		finishedG = cmd.FinishedLooseG
		finishedUnits = cmd.FinishedUnits
	}
	if finishedG <= 0 && finishedUnits <= 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("finished material quantity required")
	}
	frozenWarehouse := strings.TrimSpace(wo.TargetWarehouse)
	if frozenWarehouse == "" {
		frozenWarehouse = stockdomain.WarehouseWIP
	}
	warehouse := strings.TrimSpace(cmd.Warehouse)
	if warehouse == "" {
		warehouse = frozenWarehouse
	}
	if warehouse != frozenWarehouse {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("completion warehouse must match frozen work order target warehouse: %s", frozenWarehouse)
	}
	var warehouseActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.warehouses WHERE code=$1`, r.schema), warehouse).Scan(&warehouseActive); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("target warehouse not found: %s", warehouse)
		}
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if !warehouseActive {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("target warehouse is inactive: %s", warehouse)
	}
	ownerCustomerID, err := warehouseCustomerID(ctx, tx, r.schema, warehouse)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	var ownerBeforeG, ownerBeforeUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(location.qty_g),0)::bigint,
		       COALESCE(SUM(location.qty_units),0)::bigint
		FROM %s.material_batch_locations location
		JOIN %s.material_batches batch ON batch.id=location.material_batch_id
		WHERE location.material_id=$1
		  AND location.warehouse=$2
		  AND COALESCE(batch.owner_customer_id,0)=$3
	`, r.schema, r.schema), wo.OutputMaterialID, warehouse, ownerCustomerID).Scan(&ownerBeforeG, &ownerBeforeUnits); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}

	run.OutputType = "material"
	run.OutputMaterialID = wo.OutputMaterialID
	run.OutputName = firstNonEmpty(wo.OutputName, wo.ProductName)
	run.OutputQty = wo.OutputQty
	run.OutputUnit = wo.OutputUnit
	run.TargetWarehouse = warehouse
	run.Product = run.OutputName
	run.ProductID = 0
	run.SpecG = 0
	run.MaterialSnapshot = defaultJSONArray(materialSnapshot)
	needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: finishedUnits, LooseG: finishedG})
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if !ok || len(needs) == 0 {
		return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("material output work order has no frozen component snapshot")
	}
	needs = aggregateMaterialConsumptionNeeds(needs)
	if err := ensureWIPStockForWorkOrderNeedsTx(ctx, tx, r.schema, wo.ID, needs); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := deductMaterialNeedsForRunningItemTx(ctx, tx, r.schema, run, needs, cmd.Operator); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}

	var materialName string
	var beforeG, beforeUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(name,''),onhand_g,onhand_units
		FROM %s.materials WHERE id=$1 FOR UPDATE
	`, r.schema), wo.OutputMaterialID).Scan(&materialName, &beforeG, &beforeUnits); err != nil {
		if err == pgx.ErrNoRows {
			return productionapp.WorkOrderCompleteResult{}, fmt.Errorf("output material not found")
		}
		return productionapp.WorkOrderCompleteResult{}, err
	}
	materialName = firstNonEmpty(strings.TrimSpace(wo.OutputName), materialName)
	factoryFinishedG, factoryFinishedUnits := finishedG, finishedUnits
	if ownerCustomerID > 0 {
		factoryFinishedG, factoryFinishedUnits = 0, 0
	}
	afterG := beforeG + factoryFinishedG
	afterUnits := beforeUnits + factoryFinishedUnits
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.materials SET onhand_g=$2,onhand_units=$3,updated_at=now() WHERE id=$1
	`, r.schema), wo.OutputMaterialID, afterG, afterUnits); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}

	batchCode := fmt.Sprintf("MP-%010d", run.ID)
	var materialBatchID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batches(
			batch_code,material_id,material_name,owner_customer_id,supplier,receipt_id,received_g,
			qty_g,qty_units,remaining_g,remaining_units,unit_cost,status,quality_status,note,received_at,created_at
		) VALUES($1,$2,$3,$4,'production',$5,$6,$6,$7,$6,$7,0,'active','pass',$8,now(),now())
		RETURNING id
	`, r.schema), batchCode, wo.OutputMaterialID, materialName, ownerCustomerID, wo.ID, finishedG, finishedUnits, cmd.Note).Scan(&materialBatchID); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(
			material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at
		) VALUES($1,$2,$3,$4,$5,$6,now())
	`, r.schema), materialBatchID, batchCode, wo.OutputMaterialID, warehouse, finishedG, finishedUnits); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,owner_customer_id,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,remaining_g,remaining_units,quality_status,operator,created_at
		) VALUES($1,'material',$2,$3,$4,0,'production_run',$5,$6,$7,$8,$7,$8,'pass',$9,now())
	`, r.schema), batchCode, wo.OutputMaterialID, materialName, ownerCustomerID, run.ID, run.BatchID, finishedG, finishedUnits, cmd.Operator); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := insertStockLedgerEntryOwnedTx(ctx, tx, r.schema,
		stockItemTypeMaterial, wo.OutputMaterialID, materialName, ownerCustomerID, 0, warehouse,
		stockSourceProductionRun, run.ID, batchCode, run.BatchID,
		stockLedgerQty{
			BeforeG: ownerBeforeG, ChangeG: finishedG, AfterG: ownerBeforeG + finishedG,
			BeforeUnits: ownerBeforeUnits, ChangeUnits: finishedUnits, AfterUnits: ownerBeforeUnits + finishedUnits,
		}, cmd.Operator,
	); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}

	var entry productionapp.StockEntryDetail
	if cmd.StockDocumentID > 0 {
		entry, err = finalizeMaterialOutputStockDocumentTx(
			ctx, tx, r.schema,
			cmd.StockDocumentID, wo.ID, run.ID, wo.OutputMaterialID,
			materialName, wo.OutputUnit, warehouse,
			ownerCustomerID, finishedG, finishedUnits, batchCode, cmd.Operator, cmd.Note,
		)
	} else {
		entry, err = createStockEntryRecordTx(ctx, tx, r.schema, productionapp.StockEntryCommand{
			EntryType: "finished_receipt", WorkOrderID: wo.ID, RunningItemID: run.ID,
			SourceType: "work_order_complete", SourceID: wo.ID, Operator: cmd.Operator, Note: cmd.Note,
			Items: []productionapp.StockEntryItemCommand{{
				MaterialID: wo.OutputMaterialID, ItemType: stockItemTypeMaterial, ItemName: materialName,
				OwnerCustomerID: ownerCustomerID,
				ToWarehouse:     warehouse, QtyG: finishedG, QtyUnits: finishedUnits, BatchCode: batchCode,
			}},
		}, false)
	}
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	actualCost, err := recordBatchCostForRunningItemTx(ctx, tx, r.schema, run, finishedG)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	unitCost := materialOutputUnitCost(actualCost, wo.OutputUnit, finishedG, finishedUnits)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=$2 WHERE id=$1`, r.schema), materialBatchID, unitCost); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches SET unit_cost=$2
		WHERE batch_code=$1 AND item_type='material' AND item_id=$3
	`, r.schema), batchCode, unitCost, wo.OutputMaterialID); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_entry_items SET unit_cost=$2,total_cost=$3
		WHERE stock_entry_id=$1 AND item_type='material' AND material_id=$4
	`, r.schema), entry.ID, unitCost, actualCost, wo.OutputMaterialID); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := allocateMaterialOutputToDownstreamReservationsTx(ctx, tx, r.schema, wo.ID, wo.OutputMaterialID, materialBatchID, batchCode, finishedG, finishedUnits); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := completeMaterialReservationsForRunningItemTx(ctx, tx, r.schema, run.ID); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	actualInputG := cmd.ConsumedInputG
	if actualInputG <= 0 {
		for _, need := range needs {
			actualInputG += need.DeductG
		}
	}
	if err := completeWorkOrderForRunningItemTx(ctx, tx, r.schema, run.ID, actualCost, actualInputG, finishedG, cmd.Operator); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1
	`, r.schema), run.ID, cmd.Operator, time.Now()); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &wo.ID, "complete", postgresinfra.StrPtr("status"), postgresinfra.StrPtr(wo.Status), postgresinfra.StrPtr("completed"), postgresinfra.AuditMeta{
		"output_type": "material", "output_material_id": wo.OutputMaterialID,
		"finished_qty_g": finishedG, "finished_qty_units": finishedUnits,
		"warehouse": warehouse, "batch_code": batchCode,
	}); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	updated, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, wo.ID)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	cost, err := loadBatchCostForRunningItemTx(ctx, tx, r.schema, run.ID)
	if err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WorkOrderCompleteResult{}, err
	}
	return productionapp.WorkOrderCompleteResult{
		WorkOrder: updated, StockEntries: []productionapp.StockEntryRow{stockEntryRowFromDetail(entry)}, Cost: cost,
	}, nil
}

func materialOutputUnitCost(actualCost float64, outputUnit string, finishedG, finishedUnits int64) float64 {
	if isWeightMaterialUnit(outputUnit) && finishedG > 0 {
		return actualCost / (float64(finishedG) / 1000)
	}
	if finishedUnits > 0 {
		return actualCost / float64(finishedUnits)
	}
	if finishedG > 0 {
		return actualCost / (float64(finishedG) / 1000)
	}
	return 0
}

func allocateMaterialOutputToDownstreamReservationsTx(ctx context.Context, tx pgx.Tx, schema string, upstreamWorkOrderID, materialID, materialBatchID int64, batchCode string, producedG, producedUnits int64) error {
	var warehouse string
	var ownerCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT location.warehouse,COALESCE(batch.owner_customer_id,0)
		FROM %s.material_batches batch
		JOIN %s.material_batch_locations location ON location.material_batch_id=batch.id
		WHERE batch.id=$1
		ORDER BY location.updated_at DESC LIMIT 1
	`, schema, schema), materialBatchID).Scan(&warehouse, &ownerCustomerID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT reservation.id,reservation.required_g,reservation.required_units,reservation.reserved_g,reservation.reserved_units
		FROM %s.work_order_dependencies dependency
		JOIN %s.work_order_material_reservations reservation
		  ON reservation.work_order_id=dependency.work_order_id
		 AND reservation.material_id=dependency.material_id
		 AND reservation.status='reserved'
		WHERE dependency.depends_on_work_order_id=$1 AND dependency.material_id=$2
		ORDER BY dependency.work_order_id,reservation.id
		FOR UPDATE OF reservation
	`, schema, schema), upstreamWorkOrderID, materialID)
	if err != nil {
		return err
	}
	type reservationGap struct {
		id, requiredG, requiredUnits, reservedG, reservedUnits int64
	}
	reservations := make([]reservationGap, 0)
	for rows.Next() {
		var row reservationGap
		if err := rows.Scan(&row.id, &row.requiredG, &row.requiredUnits, &row.reservedG, &row.reservedUnits); err != nil {
			rows.Close()
			return err
		}
		reservations = append(reservations, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	remainingG, remainingUnits := producedG, producedUnits
	for _, row := range reservations {
		addG := minInt64(nonnegativeQuantity(row.requiredG-row.reservedG), remainingG)
		addUnits := minInt64(nonnegativeQuantity(row.requiredUnits-row.reservedUnits), remainingUnits)
		if addG <= 0 && addUnits <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_order_material_reservations
			SET reserved_g=reserved_g+$2,reserved_units=reserved_units+$3,updated_at=now()
			WHERE id=$1
		`, schema), row.id, addG, addUnits); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservation_batches(
				reservation_id,work_order_id,material_id,component_type,component_id,
				component_bom_spec_id,component_bom_variant_id,component_spec_g,
				material_batch_id,stock_batch_id,batch_code,warehouse,owner_customer_id,
				reserved_g,reserved_units,status,created_at,updated_at
			)
			SELECT id,work_order_id,material_id,'material',material_id,0,0,0,$2,0,$3,$4,$5,$6,$7,'reserved',now(),now()
			FROM %s.work_order_material_reservations WHERE id=$1
			ON CONFLICT(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id) DO UPDATE SET
				reserved_g=work_order_material_reservation_batches.reserved_g+excluded.reserved_g,
				reserved_units=work_order_material_reservation_batches.reserved_units+excluded.reserved_units,
				warehouse=excluded.warehouse,owner_customer_id=excluded.owner_customer_id,
				status='reserved',updated_at=now()
		`, schema, schema), row.id, materialBatchID, batchCode, warehouse, ownerCustomerID, addG, addUnits); err != nil {
			return err
		}
		remainingG -= addG
		remainingUnits -= addUnits
	}
	return nil
}
