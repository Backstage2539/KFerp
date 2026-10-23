package stock

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5"
)

// The physical allocation and the reservation movement are different facts.
// NULL is an old document; an empty versioned Moves list is a new free-stock
// movement. Never infer either from today's aggregate reservation balance.
type pickingReservationSnapshot struct {
	Version int                      `json:"version"`
	Moves   []pickingReservationMove `json:"moves"`
}

type pickingReservationMove struct {
	FromBindingID int64 `json:"from_binding_id"`
	ToBindingID   int64 `json:"to_binding_id"`
	QtyG          int64 `json:"qty_g"`
	QtyUnits      int64 `json:"qty_units"`
}

func loadPickingReservationSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, itemID int64) (*pickingReservationSnapshot, error) {
	var raw []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT picking_reservation_snapshot FROM %s.stock_entry_items WHERE id=$1`, schema), itemID).Scan(&raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	var snapshot pickingReservationSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil || snapshot.Version != 1 || snapshot.Moves == nil {
		return nil, fmt.Errorf("领料预留移动记录无效，不能撤销")
	}
	for _, move := range snapshot.Moves {
		if move.FromBindingID <= 0 || move.ToBindingID <= 0 || move.FromBindingID == move.ToBindingID || move.QtyG < 0 || move.QtyUnits < 0 || (move.QtyG == 0 && move.QtyUnits == 0) {
			return nil, fmt.Errorf("领料预留移动记录无效，不能撤销")
		}
	}
	return &snapshot, nil
}

func reversePickingReservationSnapshotTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64, item stockapp.StockDocumentItemRow, snapshot pickingReservationSnapshot) error {
	for _, move := range snapshot.Moves {
		var valid bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(
 SELECT 1 FROM %[1]s.work_order_material_reservation_batches s
 JOIN %[1]s.work_order_material_reservation_batches t ON t.reservation_id=s.reservation_id
 WHERE s.id=$1 AND t.id=$2 AND s.work_order_id=$3 AND t.work_order_id=$3
 AND s.warehouse=$4 AND t.warehouse=$5 AND s.owner_customer_id=$6 AND t.owner_customer_id=$6
 AND s.component_type=t.component_type AND s.component_id=t.component_id
 AND s.component_bom_spec_id=t.component_bom_spec_id AND s.component_bom_variant_id=t.component_bom_variant_id
 AND s.component_spec_g=t.component_spec_g)`, schema), move.FromBindingID, move.ToBindingID, workOrderID, item.FromWarehouse, item.ToWarehouse, item.OwnerCustomerID).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("领料预留移动记录与原单据不一致，不能撤销")
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=reserved_g-$2,reserved_units=reserved_units-$3,updated_at=now()
 WHERE id=$1 AND status='reserved' AND reserved_g-consumed_g-returned_g>=$2 AND reserved_units-consumed_units-returned_units>=$3`, schema), move.ToBindingID, move.QtyG, move.QtyUnits)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("本次领料对应的预留已耗用或转出，不能撤销")
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=reserved_g+$2,reserved_units=reserved_units+$3,status='reserved',updated_at=now() WHERE id=$1`, schema), move.FromBindingID, move.QtyG, move.QtyUnits); err != nil {
			return err
		}
	}
	return nil
}

type pickingStockLocation struct {
	Product   bool
	BatchID   int64
	Warehouse string
	Owner     int64
}

// Lock physical batches before reservation bindings, matching plan submission.
// Across duplicate document rows, only this document's recorded reservation
// movement can be removed from the total reserved stock at the destination.
func validatePickingSnapshotCancellationTx(ctx context.Context, tx pgx.Tx, schema string, detail stockapp.StockDocumentDetail) error {
	var workOrderID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=$1 FOR UPDATE`, schema), detail.WorkOrderID).Scan(&workOrderID); err != nil {
		return err
	}
	physical := map[pickingStockLocation][2]int64{}
	batchLocks := map[pickingStockLocation]bool{}
	byBinding := map[int64][2]int64{}
	bindingIDs := map[int64]bool{}
	for _, item := range detail.Items {
		snapshot, err := loadPickingReservationSnapshotTx(ctx, tx, schema, item.ID)
		if err != nil {
			return err
		}
		if snapshot == nil {
			continue
		}
		var moved [2]int64
		for _, move := range snapshot.Moves {
			q := byBinding[move.ToBindingID]
			q[0] += move.QtyG
			q[1] += move.QtyUnits
			byBinding[move.ToBindingID] = q
			bindingIDs[move.FromBindingID], bindingIDs[move.ToBindingID] = true, true
			moved[0] += move.QtyG
			moved[1] += move.QtyUnits
		}
		g, n := pickingTransferQuantities(item)
		if moved[0] > g || moved[1] > n {
			return fmt.Errorf("领料预留移动记录超过单据数量，不能撤销")
		}
		if item.ItemType == itemTypeFinishedProduct {
			var sourceID, targetID int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT source_batch_id,target_batch_id FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_item_id=$1`, schema), item.ID).Scan(&sourceID, &targetID); err != nil {
				return err
			}
			batchLocks[pickingStockLocation{Product: true, BatchID: sourceID, Owner: item.OwnerCustomerID}] = true
			key := pickingStockLocation{true, targetID, item.ToWarehouse, item.OwnerCustomerID}
			q := physical[key]
			q[0], q[1] = q[0]+g, q[1]+n
			physical[key] = q
		} else {
			for _, allocation := range item.Allocations {
				key := pickingStockLocation{false, allocation.MaterialBatchID, item.ToWarehouse, item.OwnerCustomerID}
				q := physical[key]
				q[0], q[1] = q[0]+allocation.QtyG, q[1]+allocation.QtyUnits
				physical[key] = q
			}
		}
	}
	keys := make([]pickingStockLocation, 0, len(physical))
	for key := range physical {
		keys = append(keys, key)
		batchLocks[pickingStockLocation{Product: key.Product, BatchID: key.BatchID, Owner: key.Owner}] = true
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Product != b.Product {
			return !a.Product
		}
		if a.BatchID != b.BatchID {
			return a.BatchID < b.BatchID
		}
		return a.Warehouse < b.Warehouse
	})
	lockKeys := make([]pickingStockLocation, 0, len(batchLocks))
	for key := range batchLocks {
		lockKeys = append(lockKeys, key)
	}
	sort.Slice(lockKeys, func(i, j int) bool {
		if lockKeys[i].Product != lockKeys[j].Product {
			return !lockKeys[i].Product
		}
		return lockKeys[i].BatchID < lockKeys[j].BatchID
	})
	for _, key := range lockKeys {
		table := "material_batches"
		if key.Product {
			table = "stock_batches"
		}
		var id int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.%s WHERE id=$1 AND owner_customer_id=$2 FOR UPDATE`, schema, table), key.BatchID, key.Owner).Scan(&id); err != nil {
			return err
		}
	}
	balances := map[pickingStockLocation][2]int64{}
	for _, key := range keys {
		var g, n int64
		if key.Product {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units FROM %s.stock_batches WHERE id=$1 AND owner_customer_id=$2 FOR UPDATE`, schema), key.BatchID, key.Owner).Scan(&g, &n); err != nil {
				return err
			}
		} else {
			var id int64
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.material_batches WHERE id=$1 AND owner_customer_id=$2 FOR UPDATE`, schema), key.BatchID, key.Owner).Scan(&id); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g,qty_units FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse=$2 FOR UPDATE`, schema), key.BatchID, key.Warehouse).Scan(&g, &n); err != nil {
				return err
			}
		}
		balances[key] = [2]int64{g, n}
	}
	ids := make([]int64, 0, len(bindingIDs))
	for id := range bindingIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	removed := map[pickingStockLocation][2]int64{}
	for _, id := range ids {
		var product, status string
		var materialBatch, stockBatch, owner, remainingG, remainingN int64
		var warehouse string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT component_type,material_batch_id,stock_batch_id,owner_customer_id,warehouse,status,GREATEST(0,reserved_g-consumed_g-returned_g),GREATEST(0,reserved_units-consumed_units-returned_units)
 FROM %s.work_order_material_reservation_batches WHERE id=$1 AND work_order_id=$2 FOR UPDATE`, schema), id, detail.WorkOrderID).Scan(&product, &materialBatch, &stockBatch, &owner, &warehouse, &status, &remainingG, &remainingN); err != nil {
			return err
		}
		q, isTarget := byBinding[id]
		if !isTarget {
			continue
		}
		if status != "reserved" || q[0] > remainingG || q[1] > remainingN {
			return fmt.Errorf("本次领料对应的预留已耗用或转出，不能撤销")
		}
		key := pickingStockLocation{product == "product", materialBatch, warehouse, owner}
		if key.Product {
			key.BatchID = stockBatch
		}
		if _, ok := physical[key]; !ok {
			return fmt.Errorf("领料预留移动记录与原批次不一致，不能撤销")
		}
		current := removed[key]
		current[0], current[1] = current[0]+q[0], current[1]+q[1]
		removed[key] = current
	}
	for _, key := range keys {
		column := "material_batch_id"
		if key.Product {
			column = "stock_batch_id"
		}
		var reservedG, reservedN int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)),0)::bigint,COALESCE(SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)),0)::bigint
 FROM %[1]s.work_order_material_reservation_batches b JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id
 WHERE b.%[2]s=$1 AND b.warehouse=$2 AND b.owner_customer_id=$3 AND b.status='reserved' AND r.status='reserved'`, schema, column), key.BatchID, key.Warehouse, key.Owner).Scan(&reservedG, &reservedN); err != nil {
			return err
		}
		balance, wanted, releasing := balances[key], physical[key], removed[key]
		if balance[0]-wanted[0] < maxInt64(0, reservedG-releasing[0]) || balance[1]-wanted[1] < maxInt64(0, reservedN-releasing[1]) {
			return fmt.Errorf("已领库存不足，或已被其他任务预留，不能撤销本次领料")
		}
	}
	return nil
}
