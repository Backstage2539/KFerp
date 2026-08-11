package production

import (
	"context"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func validateProductionPlanAvailabilityAtSubmitTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []productionapp.ProductionPlanItem) error {
	type itemNeed struct {
		itemID int64
		need   materialConsumptionNeed
	}
	itemNeeds := make([]itemNeed, 0)
	materialSet := map[int64]bool{}
	productSet := map[string]materialConsumptionNeed{}
	for _, item := range items {
		plan := plannedFinishedInventoryAddition(item.SpecG, item.PlannedOutputG)
		if item.OutputType == "material" {
			outputG, outputUnits := canonicalFromManufacturingQty(item.OutputQty, item.OutputUnit)
			plan = InvQty{Units: outputUnits, LooseG: outputG}
		} else if item.SalesSpecCount > 0 {
			plan.Units = int64(math.Ceil(item.SalesSpecCount))
			if item.SpecG > 0 {
				plan.LooseG = nonnegativeQuantity(item.PlannedOutputG - plan.Units*item.SpecG)
			}
		}
		run := ProduceRunRow{
			Product: item.ProductName, ProductID: item.ProductID, SpecG: item.SpecG,
			NeedG: item.PlannedOutputG, InputG: item.PlannedG,
			PlanUnits: plan.Units, PlanLooseG: plan.LooseG, MaterialSnapshot: item.MaterialSnapshot,
		}
		needs, ok, err := materialSnapshotNeedsTx(run, plan)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		for _, need := range aggregateManufacturingConsumptionNeeds(needs) {
			itemNeeds = append(itemNeeds, itemNeed{itemID: item.ID, need: need})
			componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
			if componentType == "product" {
				productSet[manufacturingReservationKey(componentType, componentID, componentSpecG)] = need
			} else {
				materialSet[componentID] = true
			}
		}
	}
	materialIDs := make([]int64, 0, len(materialSet))
	for materialID := range materialSet {
		materialIDs = append(materialIDs, materialID)
	}
	sort.Slice(materialIDs, func(i, j int) bool { return materialIDs[i] < materialIDs[j] })
	if len(materialIDs) > 0 {
		rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE id=ANY($1::bigint[]) ORDER BY id FOR UPDATE`, schema), materialIDs)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	productKeys := make([]string, 0, len(productSet))
	for key := range productSet {
		productKeys = append(productKeys, key)
	}
	sort.Strings(productKeys)
	for _, key := range productKeys {
		_, productID, specG := manufacturingNeedIdentity(productSet[key])
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT id FROM %s.stock_batches
			WHERE item_type='finished_product' AND item_id=$1 AND spec_g=$2
			ORDER BY id FOR UPDATE
		`, schema), productID, specG)
		if err != nil {
			return err
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
	}
	availableG := map[string]int64{}
	availableUnits := map[string]int64{}
	var availabilityErr error
	for _, row := range itemNeeds {
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(row.need)
		key := manufacturingReservationKey(componentType, componentID, componentSpecG)
		if _, exists := availableG[key]; exists {
			continue
		}
		if componentType == "product" {
			availableG[key], availableUnits[key], availabilityErr = finishedProductAvailableForPlanningTx(ctx, tx, schema, componentID, componentSpecG, 0)
		} else {
			coverage, coverageErr := workOrderWIPCoverageForNeedsTx(ctx, tx, schema, 0, []materialConsumptionNeed{row.need})
			availabilityErr = coverageErr
			if len(coverage) > 0 {
				availableG[key], availableUnits[key] = coverage[0].AvailableG, coverage[0].AvailableUnits
			}
		}
		if availabilityErr != nil {
			return availabilityErr
		}
	}
	type dependencyQty struct{ g, units int64 }
	dependencyByItemComponent := map[string]dependencyQty{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT production_plan_item_id,component_type,component_id,component_spec_g,
		       SUM(required_g)::bigint,SUM(required_units)::bigint
		FROM %s.production_plan_item_dependencies
		WHERE production_plan_id=$1
		GROUP BY production_plan_item_id,component_type,component_id,component_spec_g
	`, schema), planID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var itemID, componentID, componentSpecG int64
		var componentType string
		var qty dependencyQty
		if err := rows.Scan(&itemID, &componentType, &componentID, &componentSpecG, &qty.g, &qty.units); err != nil {
			rows.Close()
			return err
		}
		dependencyByItemComponent[fmt.Sprintf("%d:%s", itemID, manufacturingReservationKey(componentType, componentID, componentSpecG))] = qty
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, row := range itemNeeds {
		need := row.need
		componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
		key := manufacturingReservationKey(componentType, componentID, componentSpecG)
		requiredG, requiredUnits := manufacturingNeedCanonicalQuantities(need)
		shortageG := nonnegativeQuantity(requiredG - availableG[key])
		availableG[key] = nonnegativeQuantity(availableG[key] - (requiredG - shortageG))
		shortageUnits := nonnegativeQuantity(requiredUnits - availableUnits[key])
		availableUnits[key] = nonnegativeQuantity(availableUnits[key] - (requiredUnits - shortageUnits))
		frozen := dependencyByItemComponent[fmt.Sprintf("%d:%s", row.itemID, key)]
		if shortageG != frozen.g || shortageUnits != frozen.units {
			return fmt.Errorf("生产计划库存可用量已变化，请重新生成生产计划")
		}
	}
	return nil
}

func createWorkOrderDependenciesForProductionPlanTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, workOrderByPlanItem map[int64]int64) (int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT production_plan_item_id,depends_on_plan_item_id,material_id,
		       component_type,component_id,component_spec_g,required_g,required_units
		FROM %s.production_plan_item_dependencies
		WHERE production_plan_id=$1
		ORDER BY id
	`, schema), planID)
	if err != nil {
		return 0, err
	}
	type dependency struct {
		itemID, upstreamItemID, materialID, componentID, componentSpecG, requiredG, requiredUnits int64
		componentType                                                                             string
	}
	dependencies := make([]dependency, 0)
	for rows.Next() {
		var row dependency
		if err := rows.Scan(&row.itemID, &row.upstreamItemID, &row.materialID, &row.componentType, &row.componentID, &row.componentSpecG, &row.requiredG, &row.requiredUnits); err != nil {
			rows.Close()
			return 0, err
		}
		dependencies = append(dependencies, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	var count int64
	for _, row := range dependencies {
		workOrderID := workOrderByPlanItem[row.itemID]
		upstreamWorkOrderID := workOrderByPlanItem[row.upstreamItemID]
		if workOrderID <= 0 || upstreamWorkOrderID <= 0 {
			return 0, fmt.Errorf("production plan dependency work order mapping missing")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_dependencies(
				work_order_id,depends_on_work_order_id,material_id,component_type,component_id,component_spec_g,
				required_g,required_units,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now())
			ON CONFLICT(work_order_id,depends_on_work_order_id,component_type,component_id,component_spec_g) DO UPDATE SET
				required_g=excluded.required_g,required_units=excluded.required_units
		`, schema), workOrderID, upstreamWorkOrderID, row.materialID, row.componentType, row.componentID, row.componentSpecG, row.requiredG, row.requiredUnits); err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

func manufacturingReservationKey(componentType string, componentID, componentSpecG int64) string {
	return fmt.Sprintf("%s:%d:%d", strings.ToLower(strings.TrimSpace(componentType)), componentID, componentSpecG)
}

func attachTypedProductReservationsToRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID, runningItemID int64) error {
	if workOrderID <= 0 || runningItemID <= 0 {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET running_item_id=$2,updated_at=now()
		WHERE work_order_id=$1 AND component_type='product' AND status='reserved'
		  AND running_item_id IN (0,$2)
	`, schema), workOrderID, runningItemID)
	return err
}

func bindFinishedProductReservationBatchesTx(ctx context.Context, tx pgx.Tx, schema string, reservationID, workOrderID, productID, specG, reserveG, reserveUnits int64) error {
	reserveG, reserveUnits = normalizeCustomerProcessingFinishedQuantity(specG, reserveG, reserveUnits)
	if reservationID <= 0 || workOrderID <= 0 || productID <= 0 || (reserveG <= 0 && reserveUnits <= 0) {
		return nil
	}
	hasCustomerReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "finished_stock_batch_id")
	if err != nil {
		return err
	}
	customerJoin := `LEFT JOIN LATERAL (SELECT 0::bigint AS reserved_g,0::bigint AS reserved_units) customer_reserved ON true`
	if hasCustomerReservations {
		customerJoin = fmt.Sprintf(`
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
				FROM %s.customer_processing_material_reservations r
				WHERE r.finished_stock_batch_id=b.id AND r.status='reserved'
			) customer_reserved ON true`, schema)
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,COALESCE(NULLIF(location.warehouse,''),'finished_goods'),
		       GREATEST(0,b.remaining_g-COALESCE(customer_reserved.reserved_g,0)-COALESCE(work_reserved.reserved_g,0))::bigint,
		       GREATEST(0,b.remaining_units-COALESCE(customer_reserved.reserved_units,0)-COALESCE(work_reserved.reserved_units,0))::bigint
		FROM %s.stock_batches b
		LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.spec_g=b.spec_g
			  AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code)
			ORDER BY l.id DESC LIMIT 1
		) location ON true
		%s
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
			FROM %s.work_order_material_reservation_batches rb
			JOIN %s.work_order_material_reservations r ON r.id=rb.reservation_id AND r.status='reserved'
			WHERE rb.stock_batch_id=b.id AND rb.component_type='product' AND rb.status='reserved'
			  AND rb.reservation_id<>$3
		) work_reserved ON true
		WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.spec_g=$2
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		ORDER BY b.created_at,b.id
		FOR UPDATE OF b
	`, schema, schema, customerJoin, schema, schema), productID, specG, reservationID)
	if err != nil {
		return err
	}
	type batchAvailability struct {
		id, availableG, availableUnits int64
		code, warehouse                string
	}
	batches := make([]batchAvailability, 0)
	for rows.Next() {
		var row batchAvailability
		if err := rows.Scan(&row.id, &row.code, &row.warehouse, &row.availableG, &row.availableUnits); err != nil {
			rows.Close()
			return err
		}
		row.availableG, row.availableUnits = normalizeCustomerProcessingFinishedQuantity(specG, row.availableG, row.availableUnits)
		batches = append(batches, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	remainingG, remainingUnits := reserveG, reserveUnits
	for _, batch := range batches {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		addG := minInt64(remainingG, batch.availableG)
		addUnits := minInt64(remainingUnits, batch.availableUnits)
		if specG > 0 && addG > 0 {
			addUnits = addG / specG
		}
		if addG <= 0 && addUnits <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservation_batches(
				reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,
				material_batch_id,stock_batch_id,batch_code,warehouse,reserved_g,reserved_units,status,created_at,updated_at
			) VALUES($1,$2,0,'product',$3,$4,0,$5,$6,$7,$8,$9,'reserved',now(),now())
			ON CONFLICT(reservation_id,component_type,component_id,component_spec_g,material_batch_id,stock_batch_id) DO UPDATE SET
				reserved_g=work_order_material_reservation_batches.reserved_g+excluded.reserved_g,
				reserved_units=work_order_material_reservation_batches.reserved_units+excluded.reserved_units,
				warehouse=excluded.warehouse,
				status='reserved',updated_at=now()
		`, schema), reservationID, workOrderID, productID, specG, batch.id, batch.code, batch.warehouse, addG, addUnits); err != nil {
			return err
		}
		remainingG -= addG
		remainingUnits -= addUnits
		if specG > 0 {
			remainingUnits = nonnegativeQuantity(remainingG / specG)
		}
	}
	if remainingG > 0 || remainingUnits > 0 {
		return fmt.Errorf("finished product reservation batch unavailable: product %d missing %dg/%d units", productID, remainingG, remainingUnits)
	}
	return nil
}

func allocateFinishedProductOutputToDownstreamReservationsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, productID, specG, producedG, producedUnits int64) error {
	producedG, producedUnits = normalizeCustomerProcessingFinishedQuantity(specG, producedG, producedUnits)
	if runningItemID <= 0 || productID <= 0 || (producedG <= 0 && producedUnits <= 0) {
		return nil
	}
	var upstreamWorkOrderID, stockBatchID int64
	var batchCode, warehouse string
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT wo.id,b.id,b.batch_code,
		       COALESCE(NULLIF(location.warehouse,''),NULLIF(wo.target_warehouse,''),'finished_goods')
		FROM %s.work_orders wo
		JOIN %s.stock_batches b
		  ON b.source_doc_type='production_run' AND b.source_doc_id=wo.running_item_id
		 AND b.item_type='finished_product' AND b.item_id=$2 AND b.spec_g=$3
		LEFT JOIN LATERAL (
			SELECT ledger.warehouse
			FROM %s.stock_ledger_entries ledger
			WHERE ledger.item_type='finished_product' AND ledger.item_id=b.item_id AND ledger.spec_g=b.spec_g
			  AND (ledger.source_batch_code=b.batch_code OR ledger.source_batch_id=b.batch_code)
			ORDER BY ledger.id DESC LIMIT 1
		) location ON true
		WHERE wo.running_item_id=$1
		ORDER BY b.id DESC LIMIT 1
		FOR UPDATE OF b
	`, schema, schema, schema), runningItemID, productID, specG).Scan(&upstreamWorkOrderID, &stockBatchID, &batchCode, &warehouse)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT reservation.id,reservation.work_order_id,
		       reservation.required_g,reservation.required_units,reservation.reserved_g,reservation.reserved_units
		FROM %s.work_order_dependencies dependency
		JOIN %s.work_order_material_reservations reservation
		  ON reservation.work_order_id=dependency.work_order_id
		 AND reservation.component_type=dependency.component_type
		 AND reservation.component_id=dependency.component_id
		 AND reservation.component_spec_g=dependency.component_spec_g
		 AND reservation.status='reserved'
		WHERE dependency.depends_on_work_order_id=$1
		  AND dependency.component_type='product' AND dependency.component_id=$2 AND dependency.component_spec_g=$3
		ORDER BY dependency.work_order_id,reservation.id
		FOR UPDATE OF reservation
	`, schema, schema), upstreamWorkOrderID, productID, specG)
	if err != nil {
		return err
	}
	type reservationGap struct {
		id, workOrderID, requiredG, requiredUnits, reservedG, reservedUnits int64
	}
	reservations := make([]reservationGap, 0)
	for rows.Next() {
		var row reservationGap
		if err := rows.Scan(&row.id, &row.workOrderID, &row.requiredG, &row.requiredUnits, &row.reservedG, &row.reservedUnits); err != nil {
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
		batchUnits := addUnits
		if specG > 0 && addG > 0 {
			batchUnits = addG / specG
		}
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
				reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,
				material_batch_id,stock_batch_id,batch_code,warehouse,reserved_g,reserved_units,status,created_at,updated_at
			) VALUES($1,$2,0,'product',$3,$4,0,$5,$6,$7,$8,$9,'reserved',now(),now())
			ON CONFLICT(reservation_id,component_type,component_id,component_spec_g,material_batch_id,stock_batch_id) DO UPDATE SET
				reserved_g=work_order_material_reservation_batches.reserved_g+excluded.reserved_g,
				reserved_units=work_order_material_reservation_batches.reserved_units+excluded.reserved_units,
				warehouse=excluded.warehouse,
				status='reserved',updated_at=now()
		`, schema), row.id, row.workOrderID, productID, specG, stockBatchID, batchCode, warehouse, addG, batchUnits); err != nil {
			return err
		}
		remainingG -= addG
		remainingUnits -= addUnits
		if specG > 0 {
			remainingUnits = nonnegativeQuantity(remainingG / specG)
		}
	}
	return nil
}

func consumeTypedFinishedProductReservationBatchesTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, productID, specG, deductG, deductUnits int64) ([]customerProcessingFinishedBatchAllocation, bool, error) {
	deductG, deductUnits = normalizeCustomerProcessingFinishedQuantity(specG, deductG, deductUnits)
	if runningItemID <= 0 || productID <= 0 || (deductG <= 0 && deductUnits <= 0) {
		return nil, false, nil
	}
	var reservationID, workOrderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT r.id,r.work_order_id
		FROM %s.work_order_material_reservations r
		JOIN %s.work_orders wo ON wo.id=r.work_order_id AND wo.running_item_id=$1
		WHERE r.component_type='product' AND r.component_id=$2 AND r.component_spec_g=$3
		  AND r.status='reserved'
		ORDER BY r.id LIMIT 1
		FOR UPDATE OF r
	`, schema, schema), runningItemID, productID, specG).Scan(&reservationID, &workOrderID)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT rb.id,rb.stock_batch_id,rb.batch_code,
		       COALESCE(NULLIF(rb.warehouse,''),NULLIF(location.warehouse,''),'finished_goods'),
		       GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)::bigint,
		       GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)::bigint,
		       b.remaining_g,b.remaining_units
		FROM %s.work_order_material_reservation_batches rb
		JOIN %s.stock_batches b ON b.id=rb.stock_batch_id
		LEFT JOIN LATERAL (
			SELECT ledger.warehouse
			FROM %s.stock_ledger_entries ledger
			WHERE ledger.item_type='finished_product' AND ledger.item_id=b.item_id AND ledger.spec_g=b.spec_g
			  AND (ledger.source_batch_code=b.batch_code OR ledger.source_batch_id=b.batch_code)
			ORDER BY ledger.id DESC LIMIT 1
		) location ON true
		WHERE rb.reservation_id=$1 AND rb.component_type='product' AND rb.component_id=$2 AND rb.component_spec_g=$3
		  AND rb.status='reserved' AND b.item_type='finished_product'
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.created_at,b.id
		FOR UPDATE OF rb,b
	`, schema, schema, schema), reservationID, productID, specG)
	if err != nil {
		return nil, true, err
	}
	type reservedBatch struct {
		bindingID, stockBatchID, reservedG, reservedUnits, stockG, stockUnits int64
		batchCode, warehouse                                                  string
	}
	batches := make([]reservedBatch, 0)
	for rows.Next() {
		var row reservedBatch
		if err := rows.Scan(&row.bindingID, &row.stockBatchID, &row.batchCode, &row.warehouse, &row.reservedG, &row.reservedUnits, &row.stockG, &row.stockUnits); err != nil {
			rows.Close()
			return nil, true, err
		}
		batches = append(batches, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, true, err
	}
	rows.Close()
	remainingG, remainingUnits := deductG, deductUnits
	allocations := make([]customerProcessingFinishedBatchAllocation, 0)
	for _, batch := range batches {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		availableG := minInt64(batch.reservedG, batch.stockG)
		availableUnits := minInt64(batch.reservedUnits, batch.stockUnits)
		availableG, availableUnits = normalizeCustomerProcessingFinishedQuantity(specG, availableG, availableUnits)
		addG := minInt64(remainingG, availableG)
		addUnits := minInt64(remainingUnits, availableUnits)
		if specG > 0 && addG > 0 {
			addUnits = addG / specG
		}
		if addG <= 0 && addUnits <= 0 {
			continue
		}
		allocation := customerProcessingFinishedBatchAllocation{
			StockBatchID: batch.stockBatchID, BatchCode: batch.batchCode, Warehouse: batch.warehouse,
			QtyG: addG, QtyUnits: addUnits,
		}
		if err := consumeFinishedStockBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, true, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_order_material_reservation_batches
			SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,
			    status=CASE WHEN consumed_g+$2>=reserved_g AND consumed_units+$3>=reserved_units THEN 'consumed' ELSE status END,
			    updated_at=now()
			WHERE id=$1
		`, schema), batch.bindingID, addG, addUnits); err != nil {
			return nil, true, err
		}
		allocations = append(allocations, allocation)
		remainingG -= addG
		remainingUnits -= addUnits
		if specG > 0 {
			remainingUnits = nonnegativeQuantity(remainingG / specG)
		}
	}
	if remainingG > 0 || remainingUnits > 0 {
		return nil, true, fmt.Errorf("reserved finished-product batch stock insufficient: missing %dg/%d units", remainingG, remainingUnits)
	}
	consumedG, consumedUnits := deductG, deductUnits
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET consumed_g=LEAST(reserved_g,consumed_g+$2),
		    consumed_units=LEAST(reserved_units,consumed_units+$3),updated_at=now()
		WHERE id=$1
	`, schema), reservationID, consumedG, consumedUnits); err != nil {
		return nil, true, err
	}
	_ = workOrderID
	return allocations, true, nil
}

func productionPlanUsesTypedOutputBindingsTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s.production_plan_items item
			JOIN %s.production_bom_output_bindings binding
			  ON binding.output_type=item.output_type
			 AND binding.output_id=CASE WHEN item.output_type='material' THEN item.output_material_id ELSE item.output_product_id END
			 AND binding.bom_version_id=item.bom_version_id
			WHERE item.production_plan_id=$1 AND binding.is_default=true
		)
	`, schema, schema), planID).Scan(&exists)
	return exists, err
}

func createMultilevelWorkOrderReservationsTx(ctx context.Context, tx pgx.Tx, schema string, items []productionapp.ProductionPlanItem, workOrderByPlanItem map[int64]int64) error {
	for _, item := range items {
		workOrderID := workOrderByPlanItem[item.ID]
		if workOrderID <= 0 {
			continue
		}
		plan := plannedFinishedInventoryAddition(item.SpecG, item.PlannedOutputG)
		if item.OutputType == "material" {
			outputG, outputUnits := canonicalFromManufacturingQty(item.OutputQty, item.OutputUnit)
			plan = InvQty{Units: outputUnits, LooseG: outputG}
		} else if item.SalesSpecCount > 0 {
			plan.Units = int64(item.SalesSpecCount)
			if item.SpecG > 0 {
				plan.LooseG = nonnegativeQuantity(item.PlannedOutputG - plan.Units*item.SpecG)
			}
		}
		run := ProduceRunRow{
			Product: item.ProductName, ProductID: item.ProductID, SpecG: item.SpecG,
			NeedG: item.PlannedOutputG, InputG: item.PlannedG,
			PlanUnits: plan.Units, PlanLooseG: plan.LooseG, MaterialSnapshot: item.MaterialSnapshot,
		}
		needs, ok, err := materialSnapshotNeedsTx(run, plan)
		if err != nil {
			return err
		}
		if !ok || len(needs) == 0 {
			continue
		}
		needs = aggregateManufacturingConsumptionNeeds(needs)
		for _, need := range needs {
			componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
			requiredG, requiredUnits := manufacturingNeedCanonicalQuantities(need)
			availableG, availableUnits := int64(0), int64(0)
			if componentType == "product" {
				availableG, availableUnits, err = finishedProductAvailableForPlanningTx(ctx, tx, schema, componentID, componentSpecG, workOrderID)
			} else {
				coverage, coverageErr := workOrderWIPCoverageForNeedsTx(ctx, tx, schema, workOrderID, []materialConsumptionNeed{need})
				err = coverageErr
				if len(coverage) > 0 {
					availableG, availableUnits = coverage[0].AvailableG, coverage[0].AvailableUnits
				}
			}
			if err != nil {
				return err
			}
			reservedG := minInt64(requiredG, availableG)
			reservedUnits := minInt64(requiredUnits, availableUnits)
			materialID := int64(0)
			if componentType == "material" {
				materialID = componentID
			}
			var reservationID int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.work_order_material_reservations(
					work_order_id,running_item_id,material_id,material_name,unit,component_type,component_id,component_spec_g,
					required_g,required_units,reserved_g,reserved_units,status,created_at,updated_at
				) VALUES($1,0,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'reserved',now(),now())
				RETURNING id
			`, schema), workOrderID, materialID, need.MaterialName, strings.TrimSpace(need.Unit), componentType, componentID, componentSpecG, requiredG, requiredUnits, reservedG, reservedUnits).Scan(&reservationID); err != nil {
				return err
			}
			if componentType == "product" {
				if err := bindFinishedProductReservationBatchesTx(ctx, tx, schema, reservationID, workOrderID, componentID, componentSpecG, reservedG, reservedUnits); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ensureWorkOrderDependenciesCompletedTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) error {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT dependency.depends_on_work_order_id,upstream.work_order_no,upstream.output_name,upstream.status
		FROM %s.work_order_dependencies dependency
		JOIN %s.work_orders upstream ON upstream.id=dependency.depends_on_work_order_id
		WHERE dependency.work_order_id=$1
		ORDER BY dependency.depends_on_work_order_id
		FOR UPDATE OF upstream
	`, schema, schema), workOrderID)
	if err != nil {
		return err
	}
	blockers := make([]string, 0)
	for rows.Next() {
		var upstreamID int64
		var workOrderNo, outputName, status string
		if err := rows.Scan(&upstreamID, &workOrderNo, &outputName, &status); err != nil {
			return err
		}
		if status != "completed" {
			blockers = append(blockers, fmt.Sprintf("%s(%s)", firstNonEmpty(outputName, workOrderNo, fmt.Sprintf("工单%d", upstreamID)), status))
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	reservationRows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT r.component_id,r.component_spec_g,r.material_name,r.required_g,r.required_units,r.reserved_g,r.reserved_units,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		                         THEN LEAST(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g),GREATEST(0,b.remaining_g)) ELSE 0 END),0)::bigint,
		       COALESCE(SUM(CASE WHEN COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		                         THEN LEAST(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units),GREATEST(0,b.remaining_units)) ELSE 0 END),0)::bigint
		FROM %s.work_order_material_reservations r
		LEFT JOIN %s.work_order_material_reservation_batches rb
		  ON rb.reservation_id=r.id AND rb.component_type='product' AND rb.status='reserved'
		LEFT JOIN %s.stock_batches b
		  ON b.id=rb.stock_batch_id AND b.item_type='finished_product'
		WHERE r.work_order_id=$1 AND r.component_type='product' AND r.status='reserved'
		GROUP BY r.id,r.component_id,r.component_spec_g,r.material_name,r.required_g,r.required_units,r.reserved_g,r.reserved_units
		ORDER BY r.id
	`, schema, schema, schema), workOrderID)
	if err != nil {
		return err
	}
	for reservationRows.Next() {
		var productID, specG, requiredG, requiredUnits, reservedG, reservedUnits, boundG, boundUnits int64
		var name string
		if err := reservationRows.Scan(&productID, &specG, &name, &requiredG, &requiredUnits, &reservedG, &reservedUnits, &boundG, &boundUnits); err != nil {
			reservationRows.Close()
			return err
		}
		if reservedG < requiredG || reservedUnits < requiredUnits || boundG < requiredG || boundUnits < requiredUnits {
			blockers = append(blockers, fmt.Sprintf("%s产出批次预留不足", firstNonEmpty(name, fmt.Sprintf("商品%d", productID))))
		}
	}
	if err := reservationRows.Err(); err != nil {
		reservationRows.Close()
		return err
	}
	reservationRows.Close()
	if len(blockers) > 0 {
		return fmt.Errorf("上游依赖工单尚未完成: %s", strings.Join(blockers, ", "))
	}
	return nil
}
