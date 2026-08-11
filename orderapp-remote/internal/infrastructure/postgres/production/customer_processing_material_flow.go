package production

import (
	"context"
	"fmt"
	"sort"
	"strings"

	stockdomain "orderapp/internal/domain/stock"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

// customerProcessingMaterialReservation keeps the frozen ownership boundary
// from the customer request all the way to the physical batch consumed by the
// work order. A reservation stays reserved while material is merely in WIP;
// consumed quantities are recorded only when production actually deducts it.
type customerProcessingMaterialReservation struct {
	ID                   int64
	RequestID            int64
	RequestItemID        int64
	CustomerID           int64
	MaterialID           int64
	ComponentType        string
	ComponentProductID   int64
	ComponentSpecG       int64
	RequiredG            int64
	RequiredUnits        int64
	ReservedG            int64
	ReservedUnits        int64
	ConsumedG            int64
	ConsumedUnits        int64
	ReturnedG            int64
	ReturnedUnits        int64
	SourceOwnerType      string
	SourceCustomerID     int64
	SourceWarehouseCode  string
	MaterialBatchID      int64
	FinishedStockBatchID int64
	ProductionPlanID     int64
	ProductionPlanItemID int64
	WorkOrderID          int64
}

func sortCustomerProcessingReservationsForIssue(rows []customerProcessingMaterialReservation) {
	sort.SliceStable(rows, func(i, j int) bool {
		ownerRank := func(row customerProcessingMaterialReservation) int {
			if strings.EqualFold(strings.TrimSpace(row.SourceOwnerType), "customer") && row.SourceCustomerID > 0 {
				return 0
			}
			return 1
		}
		left, right := ownerRank(rows[i]), ownerRank(rows[j])
		if left != right {
			return left < right
		}
		return rows[i].ID < rows[j].ID
	})
}

func customerProcessingReservationRemainingG(row customerProcessingMaterialReservation) int64 {
	return nonnegativeQuantity(row.ReservedG - row.ConsumedG - row.ReturnedG)
}

func customerProcessingReservationRemainingUnits(row customerProcessingMaterialReservation) int64 {
	return nonnegativeQuantity(row.ReservedUnits - row.ConsumedUnits - row.ReturnedUnits)
}

type customerProcessingBatchAvailability struct {
	BatchID        int64
	BatchCode      string
	AvailableG     int64
	AvailableUnits int64
	ReceivedOrder  int64
}

type customerProcessingBatchAllocation struct {
	BatchID       int64
	BatchCode     string
	QtyG          int64
	QtyUnits      int64
	ReservationID int64
}

type customerProcessingFinishedBatchAvailability struct {
	StockBatchID   int64
	BatchCode      string
	Warehouse      string
	AvailableG     int64
	AvailableUnits int64
}

type customerProcessingFinishedBatchAllocation struct {
	StockBatchID  int64
	BatchCode     string
	Warehouse     string
	QtyG          int64
	QtyUnits      int64
	ReservationID int64
}

func normalizeCustomerProcessingFinishedQuantity(specG, qtyG, qtyUnits int64) (int64, int64) {
	qtyG, qtyUnits = nonnegativeQuantity(qtyG), nonnegativeQuantity(qtyUnits)
	if specG <= 0 {
		return qtyG, qtyUnits
	}
	if qtyG <= 0 && qtyUnits > 0 {
		qtyG = qtyUnits * specG
	}
	// Grams are the authoritative quantity. remaining_units represents only
	// complete sale units contained in those grams, never an independent second
	// balance that can survive after the grams have been split or consumed.
	qtyUnits = qtyG / specG
	return qtyG, qtyUnits
}

func allocateCustomerProcessingFinishedBatches(rows []customerProcessingFinishedBatchAvailability, requiredG, requiredUnits int64) ([]customerProcessingFinishedBatchAllocation, error) {
	remainingG, remainingUnits := nonnegativeQuantity(requiredG), nonnegativeQuantity(requiredUnits)
	out := make([]customerProcessingFinishedBatchAllocation, 0, len(rows))
	for _, row := range rows {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		qtyG := minInt64(remainingG, nonnegativeQuantity(row.AvailableG))
		qtyUnits := minInt64(remainingUnits, nonnegativeQuantity(row.AvailableUnits))
		if qtyG <= 0 && qtyUnits <= 0 {
			continue
		}
		out = append(out, customerProcessingFinishedBatchAllocation{
			StockBatchID: row.StockBatchID, BatchCode: row.BatchCode, Warehouse: row.Warehouse,
			QtyG: qtyG, QtyUnits: qtyUnits,
		})
		remainingG -= qtyG
		remainingUnits -= qtyUnits
	}
	if remainingG > 0 || remainingUnits > 0 {
		return nil, fmt.Errorf("reserved finished-product batch stock insufficient: missing %dg/%d units", remainingG, remainingUnits)
	}
	return out, nil
}

func allocateCustomerProcessingBatches(rows []customerProcessingBatchAvailability, requiredG, requiredUnits int64) ([]customerProcessingBatchAllocation, error) {
	remainingG := nonnegativeQuantity(requiredG)
	remainingUnits := nonnegativeQuantity(requiredUnits)
	out := make([]customerProcessingBatchAllocation, 0, len(rows))
	for _, row := range rows {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		qtyG := minInt64(remainingG, nonnegativeQuantity(row.AvailableG))
		qtyUnits := minInt64(remainingUnits, nonnegativeQuantity(row.AvailableUnits))
		if qtyG <= 0 && qtyUnits <= 0 {
			continue
		}
		out = append(out, customerProcessingBatchAllocation{BatchID: row.BatchID, BatchCode: row.BatchCode, QtyG: qtyG, QtyUnits: qtyUnits})
		remainingG -= qtyG
		remainingUnits -= qtyUnits
	}
	if remainingG > 0 || remainingUnits > 0 {
		return nil, fmt.Errorf("reserved material batch stock insufficient: missing %dg/%d units", remainingG, remainingUnits)
	}
	return out, nil
}

func loadCustomerProcessingReservationsForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) ([]customerProcessingMaterialReservation, error) {
	hasReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil || !hasReservations {
		return nil, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,request_id,request_item_id,customer_id,material_id,component_type,component_product_id,
		       component_spec_g,required_g,required_units,reserved_g,reserved_units,
		       consumed_g,consumed_units,returned_g,returned_units,
		       source_owner_type,source_customer_id,source_warehouse_code,material_batch_id,finished_stock_batch_id,
		       production_plan_id,production_plan_item_id,work_order_id
		FROM %s.customer_processing_material_reservations
		WHERE work_order_id=$1 AND status='reserved'
		ORDER BY id
		FOR UPDATE
	`, schema), workOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerProcessingMaterialReservation, 0)
	for rows.Next() {
		var row customerProcessingMaterialReservation
		if err := rows.Scan(
			&row.ID, &row.RequestID, &row.RequestItemID, &row.CustomerID, &row.MaterialID,
			&row.ComponentType, &row.ComponentProductID, &row.ComponentSpecG,
			&row.RequiredG, &row.RequiredUnits, &row.ReservedG, &row.ReservedUnits,
			&row.ConsumedG, &row.ConsumedUnits, &row.ReturnedG, &row.ReturnedUnits,
			&row.SourceOwnerType, &row.SourceCustomerID, &row.SourceWarehouseCode, &row.MaterialBatchID, &row.FinishedStockBatchID,
			&row.ProductionPlanID, &row.ProductionPlanItemID, &row.WorkOrderID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortCustomerProcessingReservationsForIssue(out)
	return out, nil
}

func availableCustomerProcessingMaterialBatchesTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation) ([]customerProcessingBatchAvailability, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,
		       GREATEST(0,l.qty_g-CASE WHEN l.warehouse=$4 THEN COALESCE(bound.reserved_g,0) ELSE 0 END)::bigint,
		       GREATEST(0,l.qty_units-CASE WHEN l.warehouse=$4 THEN COALESCE(bound.reserved_units,0) ELSE 0 END)::bigint,
		       COALESCE(EXTRACT(EPOCH FROM b.received_at)::bigint,b.id)
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
			FROM %s.customer_processing_material_reservations r
			WHERE r.material_batch_id=b.id AND r.status='reserved' AND r.id<>$3
		) bound ON true
		WHERE l.material_id=$1 AND l.warehouse=$2
		  AND (l.qty_g>0 OR l.qty_units>0)
		  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.received_at,b.id
		FOR UPDATE OF l,b
	`, schema, schema, schema), reservation.MaterialID, reservation.SourceWarehouseCode, reservation.ID, stockdomain.WarehouseWIP)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerProcessingBatchAvailability, 0)
	for rows.Next() {
		var row customerProcessingBatchAvailability
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.AvailableG, &row.AvailableUnits, &row.ReceivedOrder); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func customerProcessingFinishedProductID(reservation customerProcessingMaterialReservation) int64 {
	if reservation.ComponentProductID > 0 {
		return reservation.ComponentProductID
	}
	return reservation.MaterialID
}

func availableCustomerProcessingFinishedBatchesTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation) ([]customerProcessingFinishedBatchAvailability, error) {
	productID := customerProcessingFinishedProductID(reservation)
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,
		       GREATEST(0,b.remaining_g-CASE WHEN $5=$6 THEN COALESCE(bound.reserved_g,0) ELSE 0 END)::bigint,
		       GREATEST(0,b.remaining_units-CASE WHEN $5=$6 THEN COALESCE(bound.reserved_units,0) ELSE 0 END)::bigint
		FROM %s.stock_batches b
		LEFT JOIN LATERAL (
			SELECT l.warehouse
			FROM %s.stock_ledger_entries l
			WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.spec_g=b.spec_g
			  AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code)
			ORDER BY l.id DESC LIMIT 1
		) location ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
			FROM %s.customer_processing_material_reservations r
			WHERE r.finished_stock_batch_id=b.id AND r.status='reserved' AND r.id<>$4
		) bound ON true
		WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.spec_g=$2
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		  AND COALESCE(location.warehouse,$3)=$5
		ORDER BY b.created_at,b.id
		FOR UPDATE OF b
	`, schema, schema, schema), productID, reservation.ComponentSpecG, stockdomain.WarehouseFinishedGoods,
		reservation.ID, reservation.SourceWarehouseCode, stockdomain.WarehouseFinishedGoods)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]customerProcessingFinishedBatchAvailability, 0)
	for rows.Next() {
		var row customerProcessingFinishedBatchAvailability
		if err := rows.Scan(&row.StockBatchID, &row.BatchCode, &row.AvailableG, &row.AvailableUnits); err != nil {
			return nil, err
		}
		row.AvailableG, row.AvailableUnits = normalizeCustomerProcessingFinishedQuantity(reservation.ComponentSpecG, row.AvailableG, row.AvailableUnits)
		out = append(out, row)
	}
	return out, rows.Err()
}

// A finished-product stock batch has one derived warehouse in the central
// stock model. When only part of it is issued, create a traceable child batch
// for the issued quantity so the untouched remainder keeps its original
// warehouse and owner instead of being made globally available.
func splitCustomerProcessingFinishedBatchForIssueTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation, allocation customerProcessingFinishedBatchAllocation, workOrderID int64, operator string) (customerProcessingFinishedBatchAllocation, error) {
	allocation.QtyG, allocation.QtyUnits = normalizeCustomerProcessingFinishedQuantity(reservation.ComponentSpecG, allocation.QtyG, allocation.QtyUnits)
	var remainingG, storedRemainingUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT remaining_g,remaining_units
		FROM %s.stock_batches
		WHERE id=$1
		FOR UPDATE
	`, schema), allocation.StockBatchID).Scan(&remainingG, &storedRemainingUnits); err != nil {
		return customerProcessingFinishedBatchAllocation{}, err
	}
	_, remainingUnits := normalizeCustomerProcessingFinishedQuantity(reservation.ComponentSpecG, remainingG, storedRemainingUnits)
	if remainingG < allocation.QtyG || remainingUnits < allocation.QtyUnits {
		return customerProcessingFinishedBatchAllocation{}, fmt.Errorf("finished-product batch %s stock insufficient", allocation.BatchCode)
	}
	partial := (allocation.QtyG > 0 && allocation.QtyG < remainingG) || (allocation.QtyUnits > 0 && allocation.QtyUnits < remainingUnits)
	if !partial {
		if storedRemainingUnits != remainingUnits {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET remaining_units=$2 WHERE id=$1`, schema), allocation.StockBatchID, remainingUnits); err != nil {
				return customerProcessingFinishedBatchAllocation{}, err
			}
		}
		return allocation, nil
	}
	newRemainingG := remainingG - allocation.QtyG
	newRemainingUnits := remainingUnits - allocation.QtyUnits
	if reservation.ComponentSpecG > 0 {
		newRemainingUnits = newRemainingG / reservation.ComponentSpecG
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches
		SET remaining_g=$2,remaining_units=$3
		WHERE id=$1 AND remaining_g>=$4
	`, schema), allocation.StockBatchID, newRemainingG, newRemainingUnits, allocation.QtyG)
	if err != nil {
		return customerProcessingFinishedBatchAllocation{}, err
	}
	if tag.RowsAffected() != 1 {
		return customerProcessingFinishedBatchAllocation{}, fmt.Errorf("finished-product batch %s stock insufficient", allocation.BatchCode)
	}
	childCode := fmt.Sprintf("%s-CP-%d-%d", allocation.BatchCode, workOrderID, reservation.ID)
	var childID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,
			qty_g,qty_units,operator,created_at,remaining_g,remaining_units,unit_cost,quality_status
		)
		SELECT $2,item_type,item_id,item_name,spec_g,'',0,batch_code,
		       $3,$4,$5,created_at,$3,$4,unit_cost,quality_status
		FROM %s.stock_batches
		WHERE id=$1
		RETURNING id
	`, schema, schema), allocation.StockBatchID, childCode, allocation.QtyG, allocation.QtyUnits, strings.TrimSpace(operator)).Scan(&childID); err != nil {
		return customerProcessingFinishedBatchAllocation{}, err
	}
	allocation.StockBatchID = childID
	allocation.BatchCode = childCode
	return allocation, nil
}

func finishedInventoryForUpdateTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG int64, warehouse string) (InvQty, error) {
	var qty InvQty
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 FOR UPDATE
	`, schema), productID, specG, warehouse).Scan(&qty.Units, &qty.LooseG)
	if err == pgx.ErrNoRows {
		return InvQty{}, nil
	}
	return qty, err
}

func moveCustomerProcessingFinishedBatchTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation, allocation customerProcessingFinishedBatchAllocation, fromWarehouse, toWarehouse string, sourceID int64, operator, sourceType string) error {
	fromWarehouse, toWarehouse = strings.TrimSpace(fromWarehouse), strings.TrimSpace(toWarehouse)
	if allocation.StockBatchID <= 0 || (allocation.QtyG <= 0 && allocation.QtyUnits <= 0) || fromWarehouse == toWarehouse {
		return nil
	}
	productID := customerProcessingFinishedProductID(reservation)
	specG := reservation.ComponentSpecG
	moveG := allocation.QtyG
	if moveG <= 0 && allocation.QtyUnits > 0 && specG > 0 {
		moveG = allocation.QtyUnits * specG
	}
	from, err := finishedInventoryForUpdateTx(ctx, tx, schema, productID, specG, fromWarehouse)
	if err != nil {
		return err
	}
	beforeFromG := finishedComponentTotalG(specG, from.Units, from.LooseG)
	if beforeFromG < moveG {
		return fmt.Errorf("finished product stock insufficient in %s", fromWarehouse)
	}
	afterFromUnits, afterFromLoose, err := deductFinishedComponentQty(specG, from.Units, from.LooseG, moveG)
	if err != nil {
		return err
	}
	to, err := finishedInventoryForUpdateTx(ctx, tx, schema, productID, specG, toWarehouse)
	if err != nil {
		return err
	}
	toAdded := plannedFinishedInventoryAddition(specG, moveG)
	afterTo, err := invNormalize(specG, InvQty{Units: to.Units + toAdded.Units, LooseG: to.LooseG + toAdded.LooseG})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT(product_id,spec_g,warehouse) DO UPDATE SET
			onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, schema), productID, specG, fromWarehouse, afterFromUnits, afterFromLoose); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
		VALUES($1,$2,$3,$4,$5,now())
		ON CONFLICT(product_id,spec_g,warehouse) DO UPDATE SET
			onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
	`, schema), productID, specG, toWarehouse, afterTo.Units, afterTo.LooseG); err != nil {
		return err
	}
	itemName := ""
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.products WHERE id=$1`, schema), productID).Scan(&itemName)
	beforeToG := finishedComponentTotalG(specG, to.Units, to.LooseG)
	sourceBatchID := fmt.Sprintf("CP-WO-%d", sourceID)
	if err := insertStockLedgerEntryTx(ctx, tx, schema, stockItemTypeFinishedProduct, productID, itemName, specG, fromWarehouse,
		sourceType, sourceID, allocation.BatchCode, sourceBatchID, stockLedgerQty{
			BeforeG: beforeFromG, ChangeG: -moveG, AfterG: beforeFromG - moveG,
			BeforeUnits: from.Units, ChangeUnits: afterFromUnits - from.Units, AfterUnits: afterFromUnits,
		}, operator); err != nil {
		return err
	}
	return insertStockLedgerEntryTx(ctx, tx, schema, stockItemTypeFinishedProduct, productID, itemName, specG, toWarehouse,
		sourceType, sourceID, allocation.BatchCode, sourceBatchID, stockLedgerQty{
			BeforeG: beforeToG, ChangeG: moveG, AfterG: beforeToG + moveG,
			BeforeUnits: to.Units, ChangeUnits: afterTo.Units - to.Units, AfterUnits: afterTo.Units,
		}, operator)
}

func moveCustomerProcessingBatchTx(ctx context.Context, tx pgx.Tx, schema string, batchID, materialID int64, batchCode, fromWarehouse, toWarehouse string, qtyG, qtyUnits, sourceID int64, operator, sourceType string) error {
	fromWarehouse = strings.TrimSpace(fromWarehouse)
	toWarehouse = strings.TrimSpace(toWarehouse)
	if batchID <= 0 || (qtyG <= 0 && qtyUnits <= 0) || fromWarehouse == toWarehouse {
		return nil
	}
	var beforeFromG, beforeFromUnits int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g,qty_units FROM %s.material_batch_locations
		WHERE material_batch_id=$1 AND warehouse=$2 FOR UPDATE
	`, schema), batchID, fromWarehouse).Scan(&beforeFromG, &beforeFromUnits); err != nil {
		return err
	}
	if beforeFromG < qtyG || beforeFromUnits < qtyUnits {
		return fmt.Errorf("material batch %s stock insufficient in %s", batchCode, fromWarehouse)
	}
	beforeToG, beforeToUnits := int64(0), int64(0)
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g,qty_units FROM %s.material_batch_locations
		WHERE material_batch_id=$1 AND warehouse=$2 FOR UPDATE
	`, schema), batchID, toWarehouse).Scan(&beforeToG, &beforeToUnits)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.material_batch_locations
		SET qty_g=qty_g-$3,qty_units=qty_units-$4,updated_at=now()
		WHERE material_batch_id=$1 AND warehouse=$2 AND qty_g>=$3 AND qty_units>=$4
	`, schema), batchID, fromWarehouse, qtyG, qtyUnits); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (material_batch_id,warehouse) DO UPDATE SET
			batch_code=excluded.batch_code,material_id=excluded.material_id,
			qty_g=material_batch_locations.qty_g+excluded.qty_g,
			qty_units=material_batch_locations.qty_units+excluded.qty_units,updated_at=now()
	`, schema), batchID, batchCode, materialID, toWarehouse, qtyG, qtyUnits); err != nil {
		return err
	}
	hasLedger, err := schemaColumnExistsTx(ctx, tx, schema, "stock_ledger_entries", "id")
	if err != nil {
		return err
	}
	if !hasLedger {
		return nil
	}
	materialName := ""
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.materials WHERE id=$1`, schema), materialID).Scan(&materialName)
	sourceBatchID := fmt.Sprintf("CP-WO-%d", sourceID)
	if err := insertStockLedgerEntryTx(ctx, tx, schema, stockItemTypeMaterial, materialID, materialName, 0, fromWarehouse, sourceType, sourceID, batchCode, sourceBatchID, stockLedgerQty{
		BeforeG: beforeFromG, ChangeG: -qtyG, AfterG: beforeFromG - qtyG,
		BeforeUnits: beforeFromUnits, ChangeUnits: -qtyUnits, AfterUnits: beforeFromUnits - qtyUnits,
	}, operator); err != nil {
		return err
	}
	return insertStockLedgerEntryTx(ctx, tx, schema, stockItemTypeMaterial, materialID, materialName, 0, toWarehouse, sourceType, sourceID, batchCode, sourceBatchID, stockLedgerQty{
		BeforeG: beforeToG, ChangeG: qtyG, AfterG: beforeToG + qtyG,
		BeforeUnits: beforeToUnits, ChangeUnits: qtyUnits, AfterUnits: beforeToUnits + qtyUnits,
	}, operator)
}

func bindCustomerProcessingReservationAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation, allocations []customerProcessingBatchAllocation) error {
	for index, allocation := range allocations {
		if index == 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.customer_processing_material_reservations
				SET required_g=$2,required_units=$3,reserved_g=$2,reserved_units=$3,
				    material_batch_id=$4,updated_at=now()
				WHERE id=$1 AND status='reserved'
			`, schema), reservation.ID, allocation.QtyG, allocation.QtyUnits, allocation.BatchID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_material_reservations(
				request_id,request_item_id,customer_id,material_id,component_type,component_product_id,
				component_spec_g,required_g,required_units,reserved_g,reserved_units,
				consumed_g,consumed_units,returned_g,returned_units,
				source_owner_type,source_customer_id,source_warehouse_code,material_batch_id,
				production_plan_id,production_plan_item_id,work_order_id,status,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$8,$9,0,0,0,0,$10,$11,$12,$13,$14,$15,$16,'reserved',now(),now())
		`, schema), reservation.RequestID, reservation.RequestItemID, reservation.CustomerID,
			reservation.MaterialID, reservation.ComponentType, reservation.ComponentProductID,
			reservation.ComponentSpecG, allocation.QtyG, allocation.QtyUnits,
			reservation.SourceOwnerType, reservation.SourceCustomerID, reservation.SourceWarehouseCode,
			allocation.BatchID, reservation.ProductionPlanID, reservation.ProductionPlanItemID, reservation.WorkOrderID); err != nil {
			return err
		}
	}
	return nil
}

func bindCustomerProcessingFinishedAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, reservation customerProcessingMaterialReservation, allocations []customerProcessingFinishedBatchAllocation) error {
	for index, allocation := range allocations {
		if index == 0 {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.customer_processing_material_reservations
				SET required_g=$2,required_units=$3,reserved_g=$2,reserved_units=$3,
				    finished_stock_batch_id=$4,updated_at=now()
				WHERE id=$1 AND status='reserved'
			`, schema), reservation.ID, allocation.QtyG, allocation.QtyUnits, allocation.StockBatchID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.customer_processing_material_reservations(
				request_id,request_item_id,customer_id,material_id,component_type,component_product_id,
				component_spec_g,required_g,required_units,reserved_g,reserved_units,
				consumed_g,consumed_units,returned_g,returned_units,
				source_owner_type,source_customer_id,source_warehouse_code,material_batch_id,finished_stock_batch_id,
				production_plan_id,production_plan_item_id,work_order_id,status,created_at,updated_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$8,$9,0,0,0,0,$10,$11,$12,0,$13,$14,$15,$16,'reserved',now(),now())
		`, schema), reservation.RequestID, reservation.RequestItemID, reservation.CustomerID,
			reservation.MaterialID, reservation.ComponentType, reservation.ComponentProductID,
			reservation.ComponentSpecG, allocation.QtyG, allocation.QtyUnits,
			reservation.SourceOwnerType, reservation.SourceCustomerID, reservation.SourceWarehouseCode,
			allocation.StockBatchID, reservation.ProductionPlanID, reservation.ProductionPlanItemID, reservation.WorkOrderID); err != nil {
			return err
		}
	}
	return nil
}

// issueCustomerProcessingReservationsToWIPTx performs the physical issue that
// the old path skipped. It binds each frozen owner reservation to concrete FIFO
// batches and moves those batches into the standard WIP warehouse atomically.
// Status deliberately remains reserved until actual production consumption.
func issueCustomerProcessingReservationsToWIPTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, operator string) (int64, error) {
	reservations, err := loadCustomerProcessingReservationsForWorkOrderTx(ctx, tx, schema, workOrderID)
	if err != nil {
		return 0, err
	}
	var bound int64
	for _, reservation := range reservations {
		if strings.TrimSpace(reservation.SourceWarehouseCode) == "" {
			return 0, fmt.Errorf("customer processing reservation %d has no source warehouse", reservation.ID)
		}
		if strings.EqualFold(strings.TrimSpace(reservation.ComponentType), "finished_product") {
			if reservation.FinishedStockBatchID > 0 {
				continue
			}
			candidates, err := availableCustomerProcessingFinishedBatchesTx(ctx, tx, schema, reservation)
			if err != nil {
				return 0, err
			}
			requiredG, requiredUnits := normalizeCustomerProcessingFinishedQuantity(
				reservation.ComponentSpecG,
				customerProcessingReservationRemainingG(reservation),
				customerProcessingReservationRemainingUnits(reservation),
			)
			allocations, err := allocateCustomerProcessingFinishedBatches(candidates, requiredG, requiredUnits)
			if err != nil {
				return 0, fmt.Errorf("customer processing finished reservation %d: %w", reservation.ID, err)
			}
			for index := range allocations {
				allocations[index].ReservationID = reservation.ID
				allocations[index], err = splitCustomerProcessingFinishedBatchForIssueTx(ctx, tx, schema, reservation, allocations[index], workOrderID, operator)
				if err != nil {
					return 0, err
				}
				if err := moveCustomerProcessingFinishedBatchTx(ctx, tx, schema, reservation, allocations[index],
					reservation.SourceWarehouseCode, stockdomain.WarehouseWIP, workOrderID, operator, "customer_processing_finished_issue"); err != nil {
					return 0, err
				}
			}
			if err := bindCustomerProcessingFinishedAllocationsTx(ctx, tx, schema, reservation, allocations); err != nil {
				return 0, err
			}
			bound += int64(len(allocations))
			continue
		}
		if reservation.MaterialBatchID > 0 {
			continue
		}
		candidates, err := availableCustomerProcessingMaterialBatchesTx(ctx, tx, schema, reservation)
		if err != nil {
			return 0, err
		}
		allocations, err := allocateCustomerProcessingBatches(candidates, customerProcessingReservationRemainingG(reservation), customerProcessingReservationRemainingUnits(reservation))
		if err != nil {
			return 0, fmt.Errorf("customer processing reservation %d: %w", reservation.ID, err)
		}
		for index := range allocations {
			allocation := &allocations[index]
			allocation.ReservationID = reservation.ID
			if err := moveCustomerProcessingBatchTx(ctx, tx, schema, allocation.BatchID, reservation.MaterialID,
				allocation.BatchCode, reservation.SourceWarehouseCode, stockdomain.WarehouseWIP,
				allocation.QtyG, allocation.QtyUnits, workOrderID, operator, "customer_processing_issue"); err != nil {
				return 0, err
			}
		}
		if err := bindCustomerProcessingReservationAllocationsTx(ctx, tx, schema, reservation, allocations); err != nil {
			return 0, err
		}
		bound += int64(len(allocations))
	}
	return bound, nil
}

func settleCustomerProcessingReservationsForWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, cancelled bool, operator string) (int64, error) {
	reservations, err := loadCustomerProcessingReservationsForWorkOrderTx(ctx, tx, schema, workOrderID)
	if err != nil {
		return 0, err
	}
	var settled int64
	for _, reservation := range reservations {
		remainingG := customerProcessingReservationRemainingG(reservation)
		remainingUnits := customerProcessingReservationRemainingUnits(reservation)
		if reservation.MaterialBatchID > 0 && (remainingG > 0 || remainingUnits > 0) {
			toWarehouse := strings.TrimSpace(reservation.SourceWarehouseCode)
			if toWarehouse == "" {
				toWarehouse = stockdomain.WarehouseWIP
			}
			var batchCode string
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.material_batches WHERE id=$1`, schema), reservation.MaterialBatchID).Scan(&batchCode); err != nil {
				return 0, err
			}
			if err := moveCustomerProcessingBatchTx(ctx, tx, schema, reservation.MaterialBatchID, reservation.MaterialID,
				batchCode, stockdomain.WarehouseWIP, toWarehouse, remainingG, remainingUnits,
				workOrderID, operator, "customer_processing_return"); err != nil {
				return 0, err
			}
		}
		if reservation.FinishedStockBatchID > 0 && (remainingG > 0 || remainingUnits > 0) {
			toWarehouse := strings.TrimSpace(reservation.SourceWarehouseCode)
			if toWarehouse == "" {
				toWarehouse = stockdomain.WarehouseFinishedGoods
			}
			var batchCode string
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.stock_batches WHERE id=$1`, schema), reservation.FinishedStockBatchID).Scan(&batchCode); err != nil {
				return 0, err
			}
			if err := moveCustomerProcessingFinishedBatchTx(ctx, tx, schema, reservation, customerProcessingFinishedBatchAllocation{
				StockBatchID: reservation.FinishedStockBatchID, BatchCode: batchCode, QtyG: remainingG, QtyUnits: remainingUnits,
			}, stockdomain.WarehouseWIP, toWarehouse, workOrderID, operator, "customer_processing_finished_return"); err != nil {
				return 0, err
			}
		}
		status := "consumed"
		consumedG, consumedUnits := reservation.ConsumedG, reservation.ConsumedUnits
		returnedG, returnedUnits := reservation.ReturnedG+remainingG, reservation.ReturnedUnits+remainingUnits
		if cancelled {
			status = "released"
		} else if reservation.MaterialBatchID == 0 && reservation.FinishedStockBatchID == 0 && strings.EqualFold(strings.TrimSpace(reservation.ComponentType), "finished_product") {
			// Finished-product BOM components are deducted by the finished inventory
			// engine; there is no material-batch row to bind, so completion freezes
			// their actual request quantity here without creating a material charge.
			consumedG, consumedUnits = reservation.ReservedG, reservation.ReservedUnits
			returnedG, returnedUnits = reservation.ReturnedG, reservation.ReturnedUnits
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_processing_material_reservations
			SET consumed_g=$2,consumed_units=$3,returned_g=$4,returned_units=$5,status=$6,updated_at=now()
			WHERE id=$1 AND status='reserved'
		`, schema), reservation.ID, consumedG, consumedUnits, returnedG, returnedUnits, status); err != nil {
			return 0, err
		}
		settled++
	}
	if settled > 0 {
		action, oldStatus, newStatus := "complete_for_work_order", "reserved", "consumed"
		if cancelled {
			action, newStatus = "release_for_work_order_cancel", "released"
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, schema, operator, "customer_processing_material_reservation", &workOrderID, action,
			postgresinfra.StrPtr("status"), postgresinfra.StrPtr(oldStatus), postgresinfra.StrPtr(newStatus),
			postgresinfra.AuditMeta{"work_order_id": workOrderID, "reservation_count": settled}); err != nil {
			return 0, err
		}
	}
	return settled, nil
}

func completeCustomerProcessingReservationsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, operator string) (int64, error) {
	var workOrderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE running_item_id=$1`, schema), runningItemID).Scan(&workOrderID)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return settleCustomerProcessingReservationsForWorkOrderTx(ctx, tx, schema, workOrderID, false, operator)
}

func consumeMaterialBatchAllocationTx(ctx context.Context, tx pgx.Tx, schema string, allocation customerProcessingBatchAllocation) error {
	if allocation.BatchID <= 0 || (allocation.QtyG <= 0 && allocation.QtyUnits <= 0) {
		return nil
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.material_batch_locations
		SET qty_g=qty_g-$2,qty_units=qty_units-$3,updated_at=now()
		WHERE material_batch_id=$1 AND warehouse=$4 AND qty_g>=$2 AND qty_units>=$3
	`, schema), allocation.BatchID, allocation.QtyG, allocation.QtyUnits, stockdomain.WarehouseWIP)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("WIP batch %s stock insufficient", allocation.BatchCode)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.material_batches
		SET remaining_g=remaining_g-$2,remaining_units=remaining_units-$3,
		    status=CASE WHEN remaining_g-$2<=0 AND remaining_units-$3<=0 THEN 'consumed' ELSE status END
		WHERE id=$1 AND remaining_g>=$2 AND remaining_units>=$3
	`, schema), allocation.BatchID, allocation.QtyG, allocation.QtyUnits); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches
		SET remaining_g=GREATEST(0,remaining_g-$2),remaining_units=GREATEST(0,remaining_units-$3)
		WHERE batch_code=$1
	`, schema), allocation.BatchCode, allocation.QtyG, allocation.QtyUnits); err != nil {
		return err
	}
	if allocation.ReservationID > 0 {
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_processing_material_reservations
			SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,updated_at=now()
			WHERE id=$1 AND status='reserved'
			  AND consumed_g+returned_g+$2<=reserved_g
			  AND consumed_units+returned_units+$3<=reserved_units
		`, schema), allocation.ReservationID, allocation.QtyG, allocation.QtyUnits)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("customer processing reservation %d no longer available", allocation.ReservationID)
		}
	}
	return nil
}

func customerProcessingBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, materialID, deductG, deductUnits int64) ([]customerProcessingBatchAllocation, bool, error) {
	var workOrderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE running_item_id=$1`, schema), runningItemID).Scan(&workOrderID)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,source_owner_type,source_customer_id,material_batch_id,
		       reserved_g,consumed_g,returned_g,reserved_units,consumed_units,returned_units
		FROM %s.customer_processing_material_reservations
		WHERE work_order_id=$1 AND material_id=$2 AND component_type<>'finished_product' AND status='reserved'
		ORDER BY CASE WHEN source_owner_type='customer' AND source_customer_id>0 THEN 0 ELSE 1 END,id
		FOR UPDATE
	`, schema), workOrderID, materialID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	type ownedReservation struct {
		id, sourceCustomerID, batchID               int64
		ownerType                                   string
		reservedG, consumedG, returnedG             int64
		reservedUnits, consumedUnits, returnedUnits int64
	}
	owned := make([]ownedReservation, 0)
	for rows.Next() {
		var row ownedReservation
		if err := rows.Scan(&row.id, &row.ownerType, &row.sourceCustomerID, &row.batchID,
			&row.reservedG, &row.consumedG, &row.returnedG,
			&row.reservedUnits, &row.consumedUnits, &row.returnedUnits); err != nil {
			return nil, false, err
		}
		owned = append(owned, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(owned) == 0 {
		return nil, false, nil
	}
	remainingG, remainingUnits := nonnegativeQuantity(deductG), nonnegativeQuantity(deductUnits)
	allocations := make([]customerProcessingBatchAllocation, 0, len(owned))
	for _, reservation := range owned {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		if reservation.batchID <= 0 {
			return nil, true, fmt.Errorf("customer processing reservation %d is not bound to a material batch", reservation.id)
		}
		availableG := nonnegativeQuantity(reservation.reservedG - reservation.consumedG - reservation.returnedG)
		availableUnits := nonnegativeQuantity(reservation.reservedUnits - reservation.consumedUnits - reservation.returnedUnits)
		qtyG := minInt64(remainingG, availableG)
		qtyUnits := minInt64(remainingUnits, availableUnits)
		if qtyG <= 0 && qtyUnits <= 0 {
			continue
		}
		var batchCode string
		var wipG, wipUnits int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT b.batch_code,l.qty_g,l.qty_units
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_batch_id=$1 AND l.warehouse=$2
			  AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			FOR UPDATE OF l,b
		`, schema, schema), reservation.batchID, stockdomain.WarehouseWIP).Scan(&batchCode, &wipG, &wipUnits); err != nil {
			return nil, true, err
		}
		if wipG < qtyG || wipUnits < qtyUnits {
			return nil, true, fmt.Errorf("reserved WIP batch %s stock insufficient", batchCode)
		}
		allocation := customerProcessingBatchAllocation{
			BatchID: reservation.batchID, BatchCode: batchCode, QtyG: qtyG, QtyUnits: qtyUnits, ReservationID: reservation.id,
		}
		if err := consumeMaterialBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, true, err
		}
		allocations = append(allocations, allocation)
		remainingG -= qtyG
		remainingUnits -= qtyUnits
	}
	if remainingG > 0 || remainingUnits > 0 {
		return nil, true, fmt.Errorf("customer processing reserved material insufficient: missing %dg/%d units", remainingG, remainingUnits)
	}
	return allocations, true, nil
}

func unreservedWIPBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, materialID, deductG, deductUnits int64) ([]customerProcessingBatchAllocation, error) {
	hasGenericBindings, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "id")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT b.id,b.batch_code,
		       GREATEST(0,l.qty_g-COALESCE(bound.reserved_g,0)-COALESCE(generic_bound.reserved_g,0))::bigint,
		       GREATEST(0,l.qty_units-COALESCE(bound.reserved_units,0)-COALESCE(generic_bound.reserved_units,0))::bigint,
		       COALESCE(EXTRACT(EPOCH FROM b.received_at)::bigint,b.id)
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
			FROM %s.customer_processing_material_reservations r
			WHERE r.material_batch_id=b.id AND r.status='reserved'
		) bound ON true
		%s
		WHERE l.material_id=$1 AND l.warehouse=$2
		  AND (l.qty_g>0 OR l.qty_units>0)
		  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.received_at,b.id
		FOR UPDATE OF l,b
	`, schema, schema, schema, "%s")
	genericJoin := `LEFT JOIN LATERAL (SELECT 0::bigint AS reserved_g,0::bigint AS reserved_units) generic_bound ON true`
	if hasGenericBindings {
		genericJoin = fmt.Sprintf(`
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
				FROM %s.work_order_material_reservation_batches r
				WHERE r.material_batch_id=b.id AND r.status='reserved'
			) generic_bound ON true
		`, schema)
	}
	query = fmt.Sprintf(query, genericJoin)
	rows, err := tx.Query(ctx, query, materialID, stockdomain.WarehouseWIP)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	available := make([]customerProcessingBatchAvailability, 0)
	for rows.Next() {
		var row customerProcessingBatchAvailability
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.AvailableG, &row.AvailableUnits, &row.ReceivedOrder); err != nil {
			return nil, err
		}
		available = append(available, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(available) == 0 {
		legacyAggregateOnly, err := materialBatchFallbackAllowedTx(ctx, tx, schema, materialID)
		if err != nil || legacyAggregateOnly {
			return nil, err
		}
	}
	allocations, err := allocateCustomerProcessingBatches(available, deductG, deductUnits)
	if err != nil {
		return nil, err
	}
	for _, allocation := range allocations {
		if err := consumeMaterialBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, err
		}
	}
	return allocations, nil
}

// workOrderBoundBatchConsumptionsTx consumes the concrete production batches
// assigned by completed upstream work orders before falling back to ordinary
// unbound FIFO stock. It returns the unfilled remainder because the same
// reservation may also be covered by pre-existing WIP inventory.
func workOrderBoundBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, materialID, deductG, deductUnits int64) ([]customerProcessingBatchAllocation, int64, int64, error) {
	hasBindings, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "id")
	if err != nil || !hasBindings {
		return nil, deductG, deductUnits, err
	}
	var workOrderID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE running_item_id=$1`, schema), runningItemID).Scan(&workOrderID)
	if err == pgx.ErrNoRows {
		return nil, deductG, deductUnits, nil
	}
	if err != nil {
		return nil, 0, 0, err
	}
	type boundBatch struct {
		id, batchID                                 int64
		batchCode                                   string
		reservedG, consumedG, returnedG             int64
		reservedUnits, consumedUnits, returnedUnits int64
		wipG, wipUnits                              int64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT binding.id,binding.material_batch_id,binding.batch_code,
		       binding.reserved_g,binding.consumed_g,binding.returned_g,
		       binding.reserved_units,binding.consumed_units,binding.returned_units,
		       location.qty_g,location.qty_units
		FROM %s.work_order_material_reservation_batches binding
		JOIN %s.material_batch_locations location
		  ON location.material_batch_id=binding.material_batch_id AND location.warehouse=$3
		JOIN %s.material_batches batch ON batch.id=binding.material_batch_id
		WHERE binding.work_order_id=$1 AND binding.material_id=$2 AND binding.status='reserved'
		  AND batch.status='active' AND COALESCE(batch.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY binding.id
		FOR UPDATE OF binding,location,batch
	`, schema, schema, schema), workOrderID, materialID, stockdomain.WarehouseWIP)
	if err != nil {
		return nil, 0, 0, err
	}
	bound := make([]boundBatch, 0)
	for rows.Next() {
		var row boundBatch
		if err := rows.Scan(&row.id, &row.batchID, &row.batchCode,
			&row.reservedG, &row.consumedG, &row.returnedG,
			&row.reservedUnits, &row.consumedUnits, &row.returnedUnits,
			&row.wipG, &row.wipUnits); err != nil {
			rows.Close()
			return nil, 0, 0, err
		}
		bound = append(bound, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, 0, err
	}
	rows.Close()
	remainingG, remainingUnits := nonnegativeQuantity(deductG), nonnegativeQuantity(deductUnits)
	allocations := make([]customerProcessingBatchAllocation, 0, len(bound))
	for _, row := range bound {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		availableG := minInt64(nonnegativeQuantity(row.reservedG-row.consumedG-row.returnedG), row.wipG)
		availableUnits := minInt64(nonnegativeQuantity(row.reservedUnits-row.consumedUnits-row.returnedUnits), row.wipUnits)
		allocation := customerProcessingBatchAllocation{
			BatchID: row.batchID, BatchCode: row.batchCode,
			QtyG: minInt64(remainingG, availableG), QtyUnits: minInt64(remainingUnits, availableUnits),
		}
		if allocation.QtyG <= 0 && allocation.QtyUnits <= 0 {
			continue
		}
		if err := consumeMaterialBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, 0, 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.work_order_material_reservation_batches
			SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,
			    status=CASE
			      WHEN consumed_g+$2+returned_g>=reserved_g AND consumed_units+$3+returned_units>=reserved_units THEN 'consumed'
			      ELSE status
			    END,
			    updated_at=now()
			WHERE id=$1 AND status='reserved'
		`, schema), row.id, allocation.QtyG, allocation.QtyUnits); err != nil {
			return nil, 0, 0, err
		}
		allocations = append(allocations, allocation)
		remainingG -= allocation.QtyG
		remainingUnits -= allocation.QtyUnits
	}
	return allocations, remainingG, remainingUnits, nil
}

// materialBatchConsumptionsForRunningItemTx protects every active customer
// reservation. Customer processing work orders may consume only their own
// bound batches; ordinary work orders may consume only the unreserved balance.
func materialBatchConsumptionsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, materialID, deductG, deductUnits int64) ([]customerProcessingBatchAllocation, error) {
	hasOwnershipReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil {
		return nil, err
	}
	if hasOwnershipReservations {
		allocations, owned, err := customerProcessingBatchConsumptionsTx(ctx, tx, schema, runningItemID, materialID, deductG, deductUnits)
		if err != nil || owned {
			return allocations, err
		}
	}
	bound, remainingG, remainingUnits, err := workOrderBoundBatchConsumptionsTx(ctx, tx, schema, runningItemID, materialID, deductG, deductUnits)
	if err != nil || (remainingG <= 0 && remainingUnits <= 0) {
		return bound, err
	}
	var unbound []customerProcessingBatchAllocation
	if hasOwnershipReservations {
		unbound, err = unreservedWIPBatchConsumptionsTx(ctx, tx, schema, materialID, remainingG, remainingUnits)
	} else {
		unbound, err = ordinaryWIPBatchConsumptionsTx(ctx, tx, schema, materialID, remainingG, remainingUnits)
	}
	return append(bound, unbound...), err
}

func ordinaryWIPBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, materialID, deductG, deductUnits int64) ([]customerProcessingBatchAllocation, error) {
	hasGenericBindings, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "id")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT b.id,b.batch_code,l.qty_g,l.qty_units,
		       COALESCE(EXTRACT(EPOCH FROM b.received_at)::bigint,b.id)
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		WHERE l.material_id=$1 AND l.warehouse=$2
		  AND (l.qty_g>0 OR l.qty_units>0)
		  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.received_at,b.id
		FOR UPDATE OF l,b
	`, schema, schema)
	if hasGenericBindings {
		query = fmt.Sprintf(`
			SELECT b.id,b.batch_code,
			       GREATEST(0,l.qty_g-COALESCE(bound.reserved_g,0))::bigint,
			       GREATEST(0,l.qty_units-COALESCE(bound.reserved_units,0))::bigint,
			       COALESCE(EXTRACT(EPOCH FROM b.received_at)::bigint,b.id)
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
				FROM %s.work_order_material_reservation_batches r
				WHERE r.material_batch_id=b.id AND r.status='reserved'
			) bound ON true
			WHERE l.material_id=$1 AND l.warehouse=$2
			  AND (l.qty_g>0 OR l.qty_units>0)
			  AND b.status='active' AND (b.remaining_g>0 OR b.remaining_units>0)
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			ORDER BY b.received_at,b.id
			FOR UPDATE OF l,b
		`, schema, schema, schema)
	}
	rows, err := tx.Query(ctx, query, materialID, stockdomain.WarehouseWIP)
	if err != nil {
		return nil, err
	}
	available := make([]customerProcessingBatchAvailability, 0)
	for rows.Next() {
		var row customerProcessingBatchAvailability
		if err := rows.Scan(&row.BatchID, &row.BatchCode, &row.AvailableG, &row.AvailableUnits, &row.ReceivedOrder); err != nil {
			rows.Close()
			return nil, err
		}
		available = append(available, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(available) == 0 {
		legacyAggregateOnly, err := materialBatchFallbackAllowedTx(ctx, tx, schema, materialID)
		if err != nil || legacyAggregateOnly {
			return nil, err
		}
	}
	allocations, err := allocateCustomerProcessingBatches(available, deductG, deductUnits)
	if err != nil {
		return nil, err
	}
	for _, allocation := range allocations {
		if err := consumeMaterialBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, err
		}
	}
	return allocations, nil
}

// Historical production rows may predate concrete material batches and keep
// only the aggregate material balance. Preserve that read/write compatibility,
// while still rejecting a real batch that exists but is quality-blocked.
func materialBatchFallbackAllowedTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64) (bool, error) {
	var concreteCount, qualityBlockedCount int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint,
		       COUNT(*) FILTER (WHERE COALESCE(batch.quality_status,'unchecked') IN ('hold','reject'))::bigint
		FROM %s.material_batch_locations location
		JOIN %s.material_batches batch ON batch.id=location.material_batch_id
		WHERE location.material_id=$1 AND location.warehouse=$2
		  AND (location.qty_g>0 OR location.qty_units>0)
		  AND (batch.remaining_g>0 OR batch.remaining_units>0)
	`, schema, schema), materialID, stockdomain.WarehouseWIP).Scan(&concreteCount, &qualityBlockedCount); err != nil {
		return false, err
	}
	if concreteCount == 0 {
		return true, nil
	}
	if qualityBlockedCount == concreteCount {
		return false, fmt.Errorf("WIP stock blocked by quality status: material %d", materialID)
	}
	return false, nil
}

func consumeFinishedStockBatchAllocationTx(ctx context.Context, tx pgx.Tx, schema string, allocation customerProcessingFinishedBatchAllocation) error {
	if allocation.StockBatchID <= 0 || (allocation.QtyG <= 0 && allocation.QtyUnits <= 0) {
		return nil
	}
	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.stock_batches
		SET remaining_g=remaining_g-$2,
		    remaining_units=CASE WHEN spec_g>0 THEN (remaining_g-$2)/spec_g ELSE remaining_units-$3 END
		WHERE id=$1 AND remaining_g>=$2 AND (spec_g>0 OR remaining_units>=$3)
	`, schema), allocation.StockBatchID, allocation.QtyG, allocation.QtyUnits)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("finished-product batch %s stock insufficient", allocation.BatchCode)
	}
	if allocation.ReservationID > 0 {
		tag, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.customer_processing_material_reservations
			SET consumed_g=consumed_g+$2,consumed_units=consumed_units+$3,updated_at=now()
			WHERE id=$1 AND status='reserved'
			  AND consumed_g+returned_g+$2<=reserved_g
			  AND consumed_units+returned_units+$3<=reserved_units
		`, schema), allocation.ReservationID, allocation.QtyG, allocation.QtyUnits)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("customer processing finished reservation %d no longer available", allocation.ReservationID)
		}
	}
	return nil
}

func customerProcessingFinishedBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, productID, specG, deductG, deductUnits int64) ([]customerProcessingFinishedBatchAllocation, bool, error) {
	var workOrderID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE running_item_id=$1`, schema), runningItemID).Scan(&workOrderID)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT r.id,r.source_owner_type,r.source_customer_id,r.finished_stock_batch_id,b.batch_code,
		       r.reserved_g,r.consumed_g,r.returned_g,r.reserved_units,r.consumed_units,r.returned_units
		FROM %s.customer_processing_material_reservations r
		LEFT JOIN %s.stock_batches b ON b.id=r.finished_stock_batch_id
		WHERE r.work_order_id=$1 AND r.component_type='finished_product' AND r.status='reserved'
		  AND COALESCE(NULLIF(r.component_product_id,0),r.material_id)=$2 AND r.component_spec_g=$3
		ORDER BY CASE WHEN r.source_owner_type='customer' AND r.source_customer_id>0 THEN 0 ELSE 1 END,r.id
		FOR UPDATE OF r
	`, schema, schema), workOrderID, productID, specG)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	type ownedFinished struct {
		id, sourceCustomerID, stockBatchID          int64
		ownerType, batchCode                        string
		reservedG, consumedG, returnedG             int64
		reservedUnits, consumedUnits, returnedUnits int64
	}
	owned := make([]ownedFinished, 0)
	for rows.Next() {
		var row ownedFinished
		if err := rows.Scan(&row.id, &row.ownerType, &row.sourceCustomerID, &row.stockBatchID, &row.batchCode,
			&row.reservedG, &row.consumedG, &row.returnedG,
			&row.reservedUnits, &row.consumedUnits, &row.returnedUnits); err != nil {
			return nil, false, err
		}
		owned = append(owned, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(owned) == 0 {
		return nil, false, nil
	}
	remainingG, remainingUnits := normalizeCustomerProcessingFinishedQuantity(specG, deductG, deductUnits)
	allocations := make([]customerProcessingFinishedBatchAllocation, 0, len(owned))
	for _, reservation := range owned {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		if reservation.stockBatchID <= 0 {
			return nil, true, fmt.Errorf("customer processing finished reservation %d is not bound to a stock batch", reservation.id)
		}
		qtyG := minInt64(remainingG, nonnegativeQuantity(reservation.reservedG-reservation.consumedG-reservation.returnedG))
		qtyUnits := minInt64(remainingUnits, nonnegativeQuantity(reservation.reservedUnits-reservation.consumedUnits-reservation.returnedUnits))
		if qtyG <= 0 && qtyUnits <= 0 {
			continue
		}
		allocation := customerProcessingFinishedBatchAllocation{
			StockBatchID: reservation.stockBatchID, BatchCode: reservation.batchCode,
			Warehouse: stockdomain.WarehouseWIP,
			QtyG:      qtyG, QtyUnits: qtyUnits, ReservationID: reservation.id,
		}
		if err := consumeFinishedStockBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, true, err
		}
		allocations = append(allocations, allocation)
		remainingG -= qtyG
		remainingUnits -= qtyUnits
	}
	if remainingG > 0 || remainingUnits > 0 {
		return nil, true, fmt.Errorf("customer processing reserved finished product insufficient: missing %dg/%d units", remainingG, remainingUnits)
	}
	return allocations, true, nil
}

func unreservedFinishedBatchConsumptionsTx(ctx context.Context, tx pgx.Tx, schema string, productID, specG, deductG, deductUnits int64) ([]customerProcessingFinishedBatchAllocation, error) {
	deductG, deductUnits = normalizeCustomerProcessingFinishedQuantity(specG, deductG, deductUnits)
	hasGenericReservations, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservation_batches", "component_type")
	if err != nil {
		return nil, err
	}
	genericJoin := ""
	genericReservedG, genericReservedUnits := "0", "0"
	if hasGenericReservations {
		genericJoin = fmt.Sprintf(`
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
			FROM %s.work_order_material_reservation_batches rb
			JOIN %s.work_order_material_reservations r ON r.id=rb.reservation_id AND r.status='reserved'
			WHERE rb.stock_batch_id=b.id AND rb.component_type='product' AND rb.status='reserved'
		) generic_bound ON true`, schema, schema)
		genericReservedG, genericReservedUnits = "COALESCE(generic_bound.reserved_g,0)", "COALESCE(generic_bound.reserved_units,0)"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,COALESCE(location.warehouse,$3),
		       GREATEST(0,b.remaining_g-COALESCE(bound.reserved_g,0)-%s)::bigint,
		       GREATEST(0,b.remaining_units-COALESCE(bound.reserved_units,0)-%s)::bigint
		FROM %s.stock_batches b
		LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.spec_g=b.spec_g
			  AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code)
			ORDER BY l.id DESC LIMIT 1
		) location ON true
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,r.reserved_g-r.consumed_g-r.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,r.reserved_units-r.consumed_units-r.returned_units)),0)::bigint AS reserved_units
			FROM %s.customer_processing_material_reservations r
			WHERE r.finished_stock_batch_id=b.id AND r.status='reserved'
		) bound ON true
		%s
		WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.spec_g=$2
		  AND COALESCE(location.warehouse,$3)=$3
		  AND (b.remaining_g>0 OR b.remaining_units>0)
		  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.created_at,b.id
		FOR UPDATE OF b
	`, genericReservedG, genericReservedUnits, schema, schema, schema, genericJoin), productID, specG, stockdomain.WarehouseFinishedGoods)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	available := make([]customerProcessingFinishedBatchAvailability, 0)
	for rows.Next() {
		var row customerProcessingFinishedBatchAvailability
		if err := rows.Scan(&row.StockBatchID, &row.BatchCode, &row.Warehouse, &row.AvailableG, &row.AvailableUnits); err != nil {
			return nil, err
		}
		row.AvailableG, row.AvailableUnits = normalizeCustomerProcessingFinishedQuantity(specG, row.AvailableG, row.AvailableUnits)
		available = append(available, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	allocations, err := allocateCustomerProcessingFinishedBatches(available, deductG, deductUnits)
	if err != nil {
		return nil, err
	}
	for _, allocation := range allocations {
		if err := consumeFinishedStockBatchAllocationTx(ctx, tx, schema, allocation); err != nil {
			return nil, err
		}
	}
	return allocations, nil
}

func finishedBatchConsumptionsForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID, productID, specG, deductG, deductUnits int64) ([]customerProcessingFinishedBatchAllocation, error) {
	hasStockBatches, err := schemaColumnExistsTx(ctx, tx, schema, "stock_batches", "id")
	if err != nil || !hasStockBatches {
		return nil, err
	}
	hasWorkOrders, err := schemaColumnExistsTx(ctx, tx, schema, "work_orders", "id")
	if err != nil {
		return nil, err
	}
	hasReservations, err := schemaColumnExistsTx(ctx, tx, schema, "customer_processing_material_reservations", "id")
	if err != nil {
		return nil, err
	}
	hasTypedWorkReservations, err := schemaColumnExistsTx(ctx, tx, schema, "work_order_material_reservations", "component_type")
	if err != nil {
		return nil, err
	}
	if hasWorkOrders && hasTypedWorkReservations {
		allocations, owned, err := consumeTypedFinishedProductReservationBatchesTx(ctx, tx, schema, runningItemID, productID, specG, deductG, deductUnits)
		if err != nil || owned {
			return allocations, err
		}
	}
	if hasWorkOrders && hasReservations {
		allocations, owned, err := customerProcessingFinishedBatchConsumptionsTx(ctx, tx, schema, runningItemID, productID, specG, deductG, deductUnits)
		if err != nil || owned {
			return allocations, err
		}
	}
	if !hasReservations {
		return nil, nil
	}
	allocations, err := unreservedFinishedBatchConsumptionsTx(ctx, tx, schema, productID, specG, deductG, deductUnits)
	if err != nil && strings.Contains(err.Error(), "reserved finished-product batch stock insufficient") {
		var batchCount int64
		if countErr := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE item_type='finished_product' AND item_id=$1 AND spec_g=$2`, schema), productID, specG).Scan(&batchCount); countErr == nil && batchCount == 0 {
			return nil, nil
		}
	}
	return allocations, err
}
