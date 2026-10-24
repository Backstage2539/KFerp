package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
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
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT completion_draft,GREATEST(actual_minutes,CEIL((COALESCE((metrics_json->>'timer_elapsed_seconds')::numeric,0)+CASE WHEN status='running' THEN GREATEST(0,EXTRACT(EPOCH FROM now()-COALESCE(resumed_at,started_at,now()))) ELSE 0 END)/60)::int) FROM %s.job_cards WHERE id=$1`, r.schema), id).Scan(&p.Draft, &p.ActualMinutes); err != nil {
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
		} else {
			if totalInput > 0 {
				m.Qty = float64(taskQuantitySlice(n, beforeInput, input, totalInput))
			} else {
				m.Qty = math.Ceil(m.Qty)
			}
		}
		p.Materials = append(p.Materials, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return p, wo, err
	}
	for i := range p.Materials {
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

func operationPlannedTotal(wo app.WorkOrderRow) float64 {
	if wo.OutputQty > 0 {
		return wo.OutputQty
	}
	return wo.PlannedInventoryQty
}
