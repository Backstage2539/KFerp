package stock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
)

type materialLocationRepairQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type materialLocationRepairSnapshot struct {
	app.MaterialBatchLocationRepairPreview
}

func (r Repository) PreviewMaterialBatchLocationRepair(ctx context.Context, id int64) (app.MaterialBatchLocationRepairPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.MaterialBatchLocationRepairPreview{}, err
	}
	defer tx.Rollback(ctx)
	snapshot, err := loadMaterialLocationRepairSnapshot(ctx, tx, r.schema, id, false)
	if err != nil {
		return app.MaterialBatchLocationRepairPreview{}, err
	}
	snapshot.Version = materialLocationRepairVersion(snapshot)
	return snapshot, tx.Commit(ctx)
}

func loadMaterialLocationRepairSnapshot(ctx context.Context, q materialLocationRepairQuerier, schema string, id int64, lock bool) (app.MaterialBatchLocationRepairPreview, error) {
	var out app.MaterialBatchLocationRepairPreview
	lockSuffix := ""
	if lock {
		lockSuffix = " FOR UPDATE OF b"
	}
	err := q.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,b.material_id,COALESCE(m.name,b.material_name,''),COALESCE(b.owner_customer_id,0),b.remaining_g,b.remaining_units
		FROM %s.material_batches b LEFT JOIN %s.materials m ON m.id=b.material_id WHERE b.id=$1`+lockSuffix, schema, schema), id).Scan(
		&out.MaterialBatchID, &out.BatchCode, &out.MaterialID, &out.MaterialName, &out.OwnerCustomerID, &out.RemainingG, &out.RemainingUnits)
	if err != nil {
		return out, err
	}
	locationLock := ""
	if lock {
		locationLock = " FOR UPDATE"
	}
	locations, err := q.Query(ctx, fmt.Sprintf(`SELECT warehouse,qty_g,qty_units FROM %s.material_batch_locations WHERE material_batch_id=$1 ORDER BY warehouse`+locationLock, schema), id)
	if err != nil {
		return out, err
	}
	out.Locations = []app.MaterialBatchLocationRepairTarget{}
	for locations.Next() {
		var row app.MaterialBatchLocationRepairTarget
		if err = locations.Scan(&row.Warehouse, &row.QtyG, &row.QtyUnits); err != nil {
			locations.Close()
			return out, err
		}
		out.Locations = append(out.Locations, row)
	}
	err = locations.Err()
	locations.Close()
	if err != nil {
		return out, err
	}
	reservationLock := ""
	if lock {
		reservationLock = " FOR UPDATE OF binding"
	}
	reservations, err := q.Query(ctx, fmt.Sprintf(`
		SELECT binding.id,binding.work_order_id,COALESCE(wo.work_order_no,''),COALESCE(reservation.material_name,''),binding.warehouse,
		       binding.reserved_g,binding.reserved_units,binding.consumed_g,binding.consumed_units,binding.returned_g,binding.returned_units
		FROM %s.work_order_material_reservation_batches binding
		LEFT JOIN %s.work_orders wo ON wo.id=binding.work_order_id
		LEFT JOIN %s.work_order_material_reservations reservation ON reservation.id=binding.reservation_id
		WHERE binding.material_batch_id=$1 AND binding.status='reserved'
		  AND (binding.reserved_g>binding.consumed_g+binding.returned_g OR binding.reserved_units>binding.consumed_units+binding.returned_units)
		ORDER BY binding.id`+reservationLock, schema, schema, schema), id)
	if err != nil {
		return out, err
	}
	out.Reservations = []app.MaterialBatchReservationBinding{}
	for reservations.Next() {
		var row app.MaterialBatchReservationBinding
		if err = reservations.Scan(&row.ReservationBatchID, &row.WorkOrderID, &row.WorkOrderNo, &row.MaterialName, &row.Warehouse,
			&row.ReservedG, &row.ReservedUnits, &row.ConsumedG, &row.ConsumedUnits, &row.ReturnedG, &row.ReturnedUnits); err != nil {
			reservations.Close()
			return out, err
		}
		out.Reservations = append(out.Reservations, row)
	}
	err = reservations.Err()
	reservations.Close()
	return out, err
}

func materialLocationRepairVersion(snapshot app.MaterialBatchLocationRepairPreview) string {
	snapshot.Version = ""
	raw, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func (r Repository) RepairMaterialBatchLocations(ctx context.Context, cmd app.MaterialBatchLocationRepairCommand) (app.MaterialBatchLocationRepairResult, error) {
	var out app.MaterialBatchLocationRepairResult
	if cmd.Actor == "" {
		return out, fmt.Errorf("操作人不能为空")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	requestBytes, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	requestDigest := sha256.Sum256(requestBytes)
	requestHash := hex.EncodeToString(requestDigest[:])
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_material_location_repairs(request_id,request_hash,created_by) VALUES($1,$2,$3) ON CONFLICT(request_id) DO NOTHING`, r.schema), cmd.RequestID, requestHash, cmd.Actor); err != nil {
		return out, err
	}
	var savedHash string
	var savedResult []byte
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT request_hash,result FROM %s.stock_material_location_repairs WHERE request_id=$1 FOR UPDATE`, r.schema), cmd.RequestID).Scan(&savedHash, &savedResult); err != nil {
		return out, err
	}
	if savedHash != requestHash {
		return out, fmt.Errorf("同一请求标识已用于不同库位修复内容")
	}
	if len(savedResult) > 0 && string(savedResult) != "null" {
		if err = json.Unmarshal(savedResult, &out); err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	snapshot, err := loadMaterialLocationRepairSnapshot(ctx, tx, r.schema, cmd.MaterialBatchID, true)
	if err != nil {
		return out, err
	}
	snapshot.Version = materialLocationRepairVersion(snapshot)
	if snapshot.Version != cmd.ExpectedVersion {
		return out, fmt.Errorf("批次库位或预留已变化，请刷新预览后重试")
	}
	targets := map[string]app.MaterialBatchLocationRepairTarget{}
	var targetG, targetUnits int64
	for _, target := range cmd.Locations {
		if target.Warehouse == "" || target.QtyG < 0 || target.QtyUnits < 0 {
			return out, fmt.Errorf("目标库位或数量无效")
		}
		if _, exists := targets[target.Warehouse]; exists {
			return out, fmt.Errorf("目标库位重复：%s", target.Warehouse)
		}
		targets[target.Warehouse] = target
		targetG += target.QtyG
		targetUnits += target.QtyUnits
	}
	if targetG != snapshot.RemainingG || targetUnits != snapshot.RemainingUnits {
		return out, fmt.Errorf("修复后库位合计必须等于批次剩余量：批次 %dg/%d件，目标 %dg/%d件", snapshot.RemainingG, snapshot.RemainingUnits, targetG, targetUnits)
	}
	for _, target := range cmd.Locations {
		var active bool
		var customerID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT active,customer_id FROM %s.warehouses WHERE code=$1`, r.schema), target.Warehouse).Scan(&active, &customerID); err != nil {
			return out, fmt.Errorf("目标仓库不存在：%s", target.Warehouse)
		}
		if !active || customerID != snapshot.OwnerCustomerID {
			return out, fmt.Errorf("仓库%s停用或货主不匹配", target.Warehouse)
		}
	}
	moves := map[int64]string{}
	for _, move := range cmd.ReservationMoves {
		if move.ReservationBatchID <= 0 || strings.TrimSpace(move.Warehouse) == "" {
			return out, fmt.Errorf("预留库位调整行无效")
		}
		if _, exists := moves[move.ReservationBatchID]; exists {
			return out, fmt.Errorf("预留批次#%d重复指定目标仓", move.ReservationBatchID)
		}
		moves[move.ReservationBatchID] = move.Warehouse
	}
	reservedByWarehouse := map[string]app.MaterialBatchLocationRepairTarget{}
	movedCount := 0
	seenBindingIDs := map[int64]bool{}
	for _, binding := range snapshot.Reservations {
		seenBindingIDs[binding.ReservationBatchID] = true
		warehouse := binding.Warehouse
		if target, ok := moves[binding.ReservationBatchID]; ok {
			warehouse = target
			if warehouse != binding.Warehouse {
				movedCount++
			}
		}
		if _, ok := targets[warehouse]; !ok {
			return out, fmt.Errorf("工单%s的预留批次#%d仍指向被移除仓位，请明确选择新仓", binding.WorkOrderNo, binding.ReservationBatchID)
		}
		reserved := reservedByWarehouse[warehouse]
		reserved.Warehouse = warehouse
		reserved.QtyG += maxZero(binding.ReservedG - binding.ConsumedG - binding.ReturnedG)
		reserved.QtyUnits += maxZero(binding.ReservedUnits - binding.ConsumedUnits - binding.ReturnedUnits)
		reservedByWarehouse[warehouse] = reserved
	}
	for id := range moves {
		if !seenBindingIDs[id] {
			return out, fmt.Errorf("预留批次#%d不属于该批次的有效预留", id)
		}
	}
	for warehouse, reserved := range reservedByWarehouse {
		target := targets[warehouse]
		if reserved.QtyG > target.QtyG || reserved.QtyUnits > target.QtyUnits {
			return out, fmt.Errorf("仓库%s修复后余额小于对应工单预留，请先核实预留分配", warehouse)
		}
	}
	before, _ := json.Marshal(snapshot.Locations)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.material_batch_locations WHERE material_batch_id=$1`, r.schema), cmd.MaterialBatchID); err != nil {
		return out, err
	}
	for _, target := range cmd.Locations {
		if target.QtyG == 0 && target.QtyUnits == 0 {
			continue
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at) VALUES($1,$2,$3,$4,$5,$6,now())`, r.schema), snapshot.MaterialBatchID, snapshot.BatchCode, snapshot.MaterialID, target.Warehouse, target.QtyG, target.QtyUnits); err != nil {
			return out, err
		}
	}
	for id, warehouse := range moves {
		if warehouse == "" {
			return out, fmt.Errorf("预留批次#%d目标仓为空", id)
		}
		tag, updateErr := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET warehouse=$2,updated_at=now() WHERE id=$1 AND material_batch_id=$3 AND status='reserved'`, r.schema), id, warehouse, cmd.MaterialBatchID)
		if updateErr != nil {
			err = updateErr
			return out, err
		}
		if tag.RowsAffected() != 1 {
			return out, fmt.Errorf("预留批次#%d已变化，请刷新预览后重试", id)
		}
	}
	sort.Slice(cmd.Locations, func(i, j int) bool { return cmd.Locations[i].Warehouse < cmd.Locations[j].Warehouse })
	out = app.MaterialBatchLocationRepairResult{MaterialBatchID: snapshot.MaterialBatchID, BatchCode: snapshot.BatchCode, Locations: cmd.Locations, MovedReservations: movedCount, RequestID: cmd.RequestID}
	after, _ := json.Marshal(cmd.Locations)
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Actor, "material_batch", &snapshot.MaterialBatchID, "reconcile_locations", infra.StrPtr("warehouse_locations"), infra.StrPtr(string(before)), infra.StrPtr(string(after)), infra.AuditMeta{"request_id": cmd.RequestID, "reason": strings.TrimSpace(cmd.Reason), "reservation_moves": cmd.ReservationMoves, "physical_movement": false, "batch_code": snapshot.BatchCode}); err != nil {
		return app.MaterialBatchLocationRepairResult{}, err
	}
	rawResult, _ := json.Marshal(out)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_material_location_repairs SET result=$2::jsonb WHERE request_id=$1`, r.schema), cmd.RequestID, string(rawResult)); err != nil {
		return app.MaterialBatchLocationRepairResult{}, err
	}
	return out, tx.Commit(ctx)
}

func maxZero(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
