package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strings"
)

func (r Repository) PreviewOperationCompletion(ctx context.Context, id int64) (app.OperationCompletionPreview, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return app.OperationCompletionPreview{}, err
	}
	defer tx.Rollback(ctx)
	p, _, err := r.operationCompletionContextTx(ctx, tx, id, false)
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}

func operationMaterialKey(kind string, id, spec, grams int64) string {
	return fmt.Sprintf("%s:%d:%d:%d", kind, id, spec, grams)
}

func (r Repository) operationCompletionContextTx(ctx context.Context, tx pgx.Tx, id int64, lock bool) (app.OperationCompletionPreview, app.WorkOrderRow, error) {
	p := app.OperationCompletionPreview{JobCardID: id, Materials: []app.OperationMaterialUse{}, PredecessorBatches: []app.OperationBatchBalance{}}
	var seq, first, last int
	var input, totalInput, beforeInput float64
	var snapshot, bindings, version, status, unit, salesSpec string
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF w,j"
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,j.sequence_no,j.status,j.planned_input_qty::float8,
 (SELECT MIN(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=w.id AND status<>'cancelled'),
 (SELECT MAX(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=w.id AND status<>'cancelled'),
 (SELECT COALESCE(SUM(planned_input_qty),0)::float8 FROM %[1]s.job_cards WHERE work_order_id=w.id AND sequence_no=j.sequence_no AND status<>'cancelled'),
 (SELECT COALESCE(SUM(planned_input_qty),0)::float8 FROM %[1]s.job_cards WHERE work_order_id=w.id AND sequence_no=j.sequence_no AND id<j.id AND status<>'cancelled'),
 w.process_snapshot_json::text,w.operation_material_bindings::text,w.xmin::text||':'||j.xmin::text,
 COALESCE(NULLIF(w.output_unit,''),NULLIF(w.inventory_unit,''),'袋'),w.sales_spec_snapshot_json::text
 FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`+suffix, r.schema), id).Scan(&p.WorkOrderID, &seq, &status, &input, &first, &last, &totalInput, &beforeInput, &snapshot, &bindings, &version, &unit, &salesSpec)
	if err != nil {
		return p, app.WorkOrderRow{}, err
	}
	wo, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, p.WorkOrderID)
	if err != nil {
		return p, wo, err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT completion_draft,GREATEST(0,GREATEST(actual_minutes,CEIL((COALESCE((metrics_json->>'timer_elapsed_seconds')::numeric,0)+CASE WHEN status='running' THEN GREATEST(0,EXTRACT(EPOCH FROM now()-COALESCE(resumed_at,started_at,now()))) ELSE 0 END)/60)::int)-COALESCE((SELECT SUM(COALESCE(NULLIF(e.payload->>'actual_minutes','')::int,NULLIF(e.payload->>'ActualMinutes','')::int,0)) FROM %s.production_operation_events e WHERE e.job_card_id=$1 AND e.kind='complete' AND e.status='posted'),0)) FROM %s.job_cards WHERE id=$1`, r.schema, r.schema), id).Scan(&p.Draft, &p.ActualMinutes); err != nil {
		return p, wo, err
	}
	var route processTemplateSnapshot
	if err = json.Unmarshal([]byte(snapshot), &route); err != nil {
		return p, wo, err
	}
	for _, op := range route.Operations {
		if op.Seq == seq {
			if op.ExecutionJSON != "" {
				if err = json.Unmarshal([]byte(op.ExecutionJSON), &p.Execution); err != nil {
					return p, wo, err
				}
			}
			break
		}
	}
	p.Enabled = p.Execution.Version == 1
	p.Final = seq == last
	if !p.Enabled {
		p.BlockedReason = "旧工单需先核对用料归属与完成数量，再启用自动记账"
	}
	p.ProductName = firstNonEmpty(wo.OutputName, wo.ProductName)
	p.OutputSpecG = wo.SpecG
	var spec struct {
		Label string `json:"spec_label"`
	}
	_ = json.Unmarshal([]byte(salesSpec), &spec)
	p.PackageSpecLabel = spec.Label
	if p.PackageSpecLabel == "" && wo.SpecG > 0 {
		p.PackageSpecLabel = fmt.Sprintf("%dg/%s", wo.SpecG, unit)
	}
	p.OutputUnit = firstNonEmpty(p.Execution.OutputUnit, unit)
	p.InputUnit = p.OutputUnit
	if p.Enabled && p.Final && p.OutputUnit != unit {
		p.BlockedReason = "末道产出单位与冻结商品单位不一致，请核对工单配置"
	}
	share := 1.0
	if totalInput > 0 {
		share = input / totalInput
	}
	p.PlannedQty = wo.OutputQty * share
	if p.PlannedQty <= 0 {
		p.PlannedQty = wo.PlannedInventoryQty * share
	}
	if app.OperationUnitFactor(p.OutputUnit) > 0 {
		p.PlannedQty = float64(wo.PlannedOutputG) * share / app.OperationUnitFactor(p.OutputUnit)
	}
	if app.OperationUnitFactor(p.OutputUnit) == 0 {
		if totalInput > 0 {
			p.PlannedQty = float64(taskQuantitySlice(int64(math.Round(operationPlannedTotal(wo))), beforeInput, input, totalInput))
		} else {
			p.PlannedQty = math.Round(p.PlannedQty)
		}
	}
	p.TotalPlannedQty = p.PlannedQty
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(output_qty),0)::float8 FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted'`, r.schema), id).Scan(&p.CompletedOutputQty); err != nil {
		return p, wo, err
	}
	p.PlannedQty = math.Max(0, p.TotalPlannedQty-p.CompletedOutputQty)
	p.InputQty = input
	if factor := app.OperationUnitFactor(p.OutputUnit); factor > 0 {
		p.InputQty = input / factor
	}
	p.Warehouse = firstNonEmpty(p.Execution.ReceiptWarehouse, wo.TargetWarehouse, "finished_goods")
	if !p.Execution.ReceiptEnabled {
		p.Warehouse = "wip"
	}
	if p.Final && !p.Execution.ReceiptEnabled && p.Enabled {
		p.BlockedReason = "最终产出工序请配置完成后入库"
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), p.Warehouse).Scan(&p.WarehouseName); err != nil {
		return p, wo, fmt.Errorf("入库仓不存在或已停用：%s", p.Warehouse)
	}
	var mapped map[string]app.OperationMaterialBinding
	if err = json.Unmarshal([]byte(bindings), &mapped); err != nil {
		return p, wo, err
	}
	consumedByReservation := map[int64]float64{}
	priorEvents, err := tx.Query(ctx, fmt.Sprintf(`SELECT COALESCE(payload->'materials','[]'::jsonb) FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted' ORDER BY id`, r.schema), id)
	if err != nil {
		return p, wo, err
	}
	for priorEvents.Next() {
		var raw []byte
		if err = priorEvents.Scan(&raw); err != nil {
			priorEvents.Close()
			return p, wo, err
		}
		var used []app.OperationMaterialUse
		if err = json.Unmarshal(raw, &used); err != nil {
			priorEvents.Close()
			return p, wo, err
		}
		for _, material := range used {
			consumedByReservation[material.ReservationID] += operationLedgerQuantity(material.Qty, material.Unit)
		}
	}
	if err = priorEvents.Err(); err != nil {
		priorEvents.Close()
		return p, wo, err
	}
	priorEvents.Close()
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT r.id,r.component_type,CASE WHEN r.component_id>0 THEN r.component_id ELSE r.material_id END,r.component_bom_spec_id,r.component_bom_variant_id,r.component_spec_g,r.material_name,r.unit,r.source_owner_customer_id,r.required_g,r.required_units
 FROM %s.work_order_material_reservations r WHERE r.work_order_id=$1 AND r.status IN('reserved','consumed') ORDER BY r.id`, r.schema), p.WorkOrderID)
	if err != nil {
		return p, wo, err
	}
	for rows.Next() {
		var m app.OperationMaterialUse
		var kind string
		var component, g, n int64
		if err = rows.Scan(&m.ReservationID, &kind, &component, &m.BomSpecID, &m.BomVariantID, &m.SpecG, &m.Name, &m.Unit, &m.OwnerCustomerID, &g, &n); err != nil {
			rows.Close()
			return p, wo, err
		}
		if kind == "finished_product" {
			kind = "product"
		}
		if kind == "" {
			kind = "material"
		}
		m.Key = operationMaterialKey(kind, component, m.BomSpecID, m.SpecG)
		m.NodeKey = mapped[m.Key].NodeKey
		if kind == "product" {
			m.ProductID = component
		} else {
			m.MaterialID = component
		}
		if m.NodeKey == "" && first == last {
			m.NodeKey = p.Execution.NodeKey
		}
		if m.NodeKey == "" && p.Enabled {
			p.BlockedReason = "请核对BOM组件使用工序：" + m.Name
		}
		if m.NodeKey != p.Execution.NodeKey {
			continue
		}
		m.Warehouse = firstNonEmpty(mapped[m.Key].SourceWarehouse, p.Execution.SourceWarehouse, "wip")
		m.Qty = float64(n) * share
		if g > 0 {
			factor := app.OperationUnitFactor(m.Unit)
			if factor == 0 {
				rows.Close()
				return p, wo, fmt.Errorf("物料%s缺少重量单位", m.Name)
			}
			m.Qty = float64(g) * share / factor
			if totalInput > 0 {
				m.Qty = float64(taskQuantitySlice(g, beforeInput, input, totalInput)) / factor
			}
		} else {
			if totalInput > 0 {
				m.Qty = float64(taskQuantitySlice(n, beforeInput, input, totalInput))
			} else {
				m.Qty = math.Ceil(m.Qty)
			}
		}
		m.TotalQty = m.Qty
		m.ConsumedQty = consumedByReservation[m.ReservationID]
		m.Qty = operationLedgerQuantity(math.Max(0, m.TotalQty-m.ConsumedQty), m.Unit)
		p.Materials = append(p.Materials, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, wo, err
	}
	for i := range p.Materials {
		p.Materials[i].AvailableQty, err = operationMaterialAvailableTx(ctx, tx, r.schema, p.Materials[i].ReservationID, p.Materials[i].Warehouse, p.Materials[i].OwnerCustomerID, p.Materials[i].Unit)
		if err != nil {
			return p, wo, err
		}
		p.Materials[i].ShortageQty = math.Max(0, p.Materials[i].Qty-p.Materials[i].AvailableQty)
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), p.Materials[i].Warehouse).Scan(&p.Materials[i].WarehouseName); err != nil {
			return p, wo, fmt.Errorf("来源仓不存在或已停用")
		}
	}
	if seq == first {
		var grams float64
		for _, m := range p.Materials {
			grams += m.Qty * app.OperationUnitFactor(m.Unit)
		}
		if grams > 0 {
			p.InputUnit = "g"
			p.InputQty = grams
		}
	}
	if seq > first {
		p.PredecessorRequired = true
		suffix = ""
		if lock {
			suffix = " FOR UPDATE OF b"
		}
		rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.work_order_id,b.job_card_id,b.owner_customer_id,b.remaining_qty::float8,b.unit,b.warehouse,wh.name,j.operation,b.remaining_cost::float8
 FROM %[1]s.production_operation_balances b JOIN %[1]s.job_cards j ON j.id=b.job_card_id JOIN %[1]s.warehouses wh ON wh.code=b.warehouse
 WHERE b.work_order_id=$1 AND b.sequence_no=(SELECT MAX(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=$1 AND sequence_no<$2 AND status<>'cancelled') AND b.remaining_qty>0 AND b.quality_status NOT IN('hold','reject') AND NOT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(q.job_card_id,q.scope,q.reference_type,q.reference_no) q.result FROM %[1]s.quality_inspections q WHERE (q.work_order_id=b.work_order_id AND q.job_card_id=0) OR q.job_card_id=b.job_card_id ORDER BY q.job_card_id,q.scope,q.reference_type,q.reference_no,q.created_at DESC,q.id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked')) ORDER BY b.id`+suffix, r.schema), p.WorkOrderID, seq)
		if err != nil {
			return p, wo, err
		}
		for rows.Next() {
			var b app.OperationBatchBalance
			if err = rows.Scan(&b.ID, &b.WorkOrderID, &b.JobCardID, &b.OwnerCustomerID, &b.Remaining, &b.Unit, &b.Warehouse, &b.WarehouseName, &b.Operation, &b.TotalCost); err != nil {
				rows.Close()
				return p, wo, err
			}
			p.PredecessorBatches = append(p.PredecessorBatches, b)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return p, wo, err
		}
		p.InputQty = 0
		if len(p.PredecessorBatches) > 0 {
			p.InputUnit = p.PredecessorBatches[0].Unit
			for _, b := range p.PredecessorBatches {
				if b.Unit == p.InputUnit {
					p.InputQty += b.Remaining
				}
			}
			if !p.Execution.ReceiptEnabled {
				p.Warehouse = p.PredecessorBatches[0].Warehouse
				p.WarehouseName = p.PredecessorBatches[0].WarehouseName
			}
		} else if p.Enabled {
			var unfinished bool
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no<$2 AND status NOT IN('completed','cancelled'))`, r.schema), p.WorkOrderID, seq).Scan(&unfinished); err != nil {
				return p, wo, err
			}
			if unfinished {
				p.BlockedReason = "前序尚无可用的完成批次"
			}
		}
	}
	p.MaxOutputQty = p.PlannedQty
	for _, material := range p.Materials {
		if material.Qty <= 0 || material.AvailableQty >= material.Qty {
			continue
		}
		possible := p.PlannedQty * material.AvailableQty / material.Qty
		if app.OperationUnitFactor(p.OutputUnit) == 0 {
			possible = math.Floor(possible + 1e-9)
		}
		p.MaxOutputQty = math.Min(p.MaxOutputQty, math.Max(0, possible))
	}
	stockVersion, err := r.operationInventoryVersionTx(ctx, tx, p.WorkOrderID)
	if err != nil {
		return p, wo, err
	}
	raw, _ := json.Marshal([]any{version, stockVersion, p.Materials, p.PredecessorBatches, p.Execution})
	p.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	if strings.TrimSpace(status) == "completed" {
		p.BlockedReason = "本工序已完成，请从生产记录查看或更正"
	}
	if strings.TrimSpace(status) == "cancelled" {
		p.BlockedReason = "任务已取消"
	}
	return p, wo, nil
}

func operationMaterialAvailableTx(ctx context.Context, tx pgx.Tx, schema string, reservationID int64, warehouse string, ownerCustomerID int64, unit string) (float64, error) {
	var componentType string
	var workOrderID, materialID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT component_type,work_order_id,material_id FROM %s.work_order_material_reservations WHERE id=$1`, schema), reservationID).Scan(&componentType, &workOrderID, &materialID); err != nil {
		return 0, err
	}
	if componentType == "product" {
		return operationReservedProductAvailableTx(ctx, tx, schema, reservationID, warehouse, ownerCustomerID, unit)
	}
	type batchAvailability struct {
		id, physicalG, physicalUnits, ownG, ownUnits, reservedG, reservedUnits int64
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT mb.id,l.qty_g,l.qty_units,
		       COALESCE(own.g,0)::bigint,COALESCE(own.units,0)::bigint,
		       COALESCE(all_reserved.g,0)::bigint,COALESCE(all_reserved.units,0)::bigint
		FROM %[1]s.work_order_material_reservations target
		JOIN %[1]s.material_batches mb ON mb.material_id=target.material_id
		  AND mb.status='active' AND mb.quality_status NOT IN('hold','reject')
		  AND COALESCE(mb.owner_customer_id,0)=$3
		JOIN %[1]s.material_batch_locations l ON l.material_batch_id=mb.id AND l.warehouse=$2
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)) g,
			       SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)) units
			FROM %[1]s.work_order_material_reservation_batches b
			JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id AND r.status='reserved'
			WHERE b.reservation_id=$1 AND b.material_batch_id=mb.id AND b.warehouse=$2
			  AND b.owner_customer_id=$3 AND b.status='reserved'
		) own ON true
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g)) g,
			       SUM(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units)) units
			FROM %[1]s.work_order_material_reservation_batches b
			JOIN %[1]s.work_order_material_reservations r ON r.id=b.reservation_id AND r.status='reserved'
			WHERE b.material_batch_id=mb.id AND b.warehouse=$2 AND b.owner_customer_id=$3 AND b.status='reserved'
		) all_reserved ON true
		WHERE target.id=$1 AND target.status IN('reserved','consumed')
		  AND target.source_owner_customer_id=$3 AND (l.qty_g>0 OR l.qty_units>0)
		ORDER BY mb.received_at,mb.id`, schema), reservationID, warehouse, ownerCustomerID)
	if err != nil {
		return 0, err
	}
	batches := []batchAvailability{}
	for rows.Next() {
		var row batchAvailability
		if err = rows.Scan(&row.id, &row.physicalG, &row.physicalUnits, &row.ownG, &row.ownUnits, &row.reservedG, &row.reservedUnits); err != nil {
			rows.Close()
			return 0, err
		}
		batches = append(batches, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	var grams, units int64
	for _, batch := range batches {
		processingG, processingUnits, e := stockrepo.OperationOtherProcessingReservedTx(ctx, tx, schema, workOrderID, batch.id, materialID, ownerCustomerID, warehouse, false)
		if e != nil {
			return 0, e
		}
		otherReservedG := maxInt64(0, batch.reservedG-batch.ownG)
		otherReservedUnits := maxInt64(0, batch.reservedUnits-batch.ownUnits)
		ownAvailableG := minInt64(batch.ownG, maxInt64(0, batch.physicalG-otherReservedG-processingG))
		ownAvailableUnits := minInt64(batch.ownUnits, maxInt64(0, batch.physicalUnits-otherReservedUnits-processingUnits))
		grams += ownAvailableG + maxInt64(0, batch.physicalG-batch.reservedG-processingG)
		units += ownAvailableUnits + maxInt64(0, batch.physicalUnits-batch.reservedUnits-processingUnits)
	}
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		return float64(grams) / factor, nil
	}
	return float64(units), nil
}

func operationReservedProductAvailableTx(ctx context.Context, tx pgx.Tx, schema string, reservationID int64, warehouse string, ownerCustomerID int64, unit string) (float64, error) {
	var grams, units int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(available_g),0)::bigint,COALESCE(SUM(available_units),0)::bigint
		FROM (
			SELECT LEAST(GREATEST(0,b.reserved_g-b.consumed_g-b.returned_g),GREATEST(0,sb.remaining_g)) AS available_g,
			       LEAST(GREATEST(0,b.reserved_units-b.consumed_units-b.returned_units),GREATEST(0,sb.remaining_units)) AS available_units
			FROM %[1]s.work_order_material_reservation_batches b
			JOIN %[1]s.stock_batches sb ON sb.id=b.stock_batch_id AND sb.quality_status NOT IN('hold','reject')
			WHERE b.reservation_id=$1 AND b.status='reserved' AND b.warehouse=$2 AND b.owner_customer_id=$3
			  AND COALESCE(sb.owner_customer_id,0)=$3
		) bound`, schema), reservationID, warehouse, ownerCustomerID).Scan(&grams, &units)
	if err != nil {
		return 0, err
	}
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		return float64(grams) / factor, nil
	}
	return float64(units), nil
}

func operationPlannedTotal(wo app.WorkOrderRow) float64 {
	if wo.OutputQty > 0 {
		return wo.OutputQty
	}
	return wo.PlannedInventoryQty
}
