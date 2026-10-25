package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strconv"
	"strings"
)

func (r Repository) completeOperation(ctx context.Context, cmd app.JobCardActionCommand) (app.JobCardActionResult, error) {
	return retryOperationTransaction(ctx, func() (app.JobCardActionResult, error) { return r.completeOperationOnce(ctx, cmd) })
}

func (r Repository) completeOperationOnce(ctx context.Context, cmd app.JobCardActionCommand) (app.JobCardActionResult, error) {
	var out app.JobCardActionResult
	if cmd.EmployeeID <= 0 {
		return out, fmt.Errorf("请先绑定员工")
	}
	if strings.TrimSpace(cmd.RequestID) == "" || strings.TrimSpace(cmd.ExpectedVersion) == "" {
		return out, fmt.Errorf("请刷新完成建议后提交")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var workID, runID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), cmd.ID).Scan(&workID, &runID); err != nil {
		return out, err
	}
	if runID > 0 {
		if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, runID); err != nil {
			return out, err
		}
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, workID); err != nil {
		return out, err
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "operation_complete", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return out, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &out)
		return out, err
	}
	out, err = r.completeOperationLockedTx(ctx, tx, cmd, workID, runID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (r Repository) completeOperationLockedTx(ctx context.Context, tx pgx.Tx, cmd app.JobCardActionCommand, workID, runID int64) (app.JobCardActionResult, error) {
	var out app.JobCardActionResult
	p, wo, err := r.operationCompletionContextTx(ctx, tx, cmd.ID, true)
	if err != nil {
		return out, err
	}
	if p.Version != cmd.ExpectedVersion {
		return out, fmt.Errorf("任务或库存已变化，请刷新；本次未扣料或入库")
	}
	if p.BlockedReason != "" {
		return out, fmt.Errorf("%s", p.BlockedReason)
	}
	if wo.Status == "completed" || wo.Status == "cancelled" {
		return out, fmt.Errorf("工单已结束")
	}
	var status, station, costMethod string
	var stationID int64
	var seq, plannedMinutes int
	var rate, pieceRate, plannedCost float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status,workstation,workstation_id,sequence_no,cost_method,hourly_rate::float8,piece_rate::float8,planned_minutes,planned_operation_cost::float8 FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&status, &station, &stationID, &seq, &costMethod, &rate, &pieceRate, &plannedMinutes, &plannedCost); err != nil {
		return out, err
	}
	if status == "completed" || status == "cancelled" {
		return out, fmt.Errorf("本工序已结束，请从记录更正")
	}
	owner, roster, err := resolveProductionWorkstationOwnerTx(ctx, tx, r.schema, stationID, station, productionWorkDate())
	if err != nil {
		return out, err
	}
	if roster && owner.EmployeeID != cmd.EmployeeID {
		return out, fmt.Errorf("请由今日工位负责人完成本任务")
	}
	var qualityBlocked bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(job_card_id,scope,reference_type,reference_no) result FROM %s.quality_inspections WHERE (work_order_id=$1 AND job_card_id=0) OR job_card_id=$2 ORDER BY job_card_id,scope,reference_type,reference_no,created_at DESC,id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked'))`, r.schema), wo.ID, cmd.ID).Scan(&qualityBlocked); err != nil {
		return out, err
	}
	if qualityBlocked {
		return out, fmt.Errorf("本任务被质检冻结，请先处理质检记录")
	}
	if cmd.InputUnit != p.InputUnit || cmd.OutputUnit != p.OutputUnit {
		return out, fmt.Errorf("完成数量单位与冻结工序不一致")
	}
	if p.LegacyPackaging {
		if err = r.freezeLegacyPackagingConfigTx(ctx, tx, wo, seq, p, cmd.Operator); err != nil {
			return out, err
		}
	}
	if err = app.ValidateOperationQuantity(cmd.ActualInputQty, cmd.InputUnit); err != nil {
		return out, err
	}
	plan, err := app.PlanOperationCompletion(app.OperationCompletionInput{PlannedQty: p.PlannedQty, ActualQty: cmd.ActualOutputQty, Unit: p.OutputUnit, TargetWarehouse: p.Warehouse, CurrentWarehouse: p.Warehouse, DifferenceReason: cmd.DifferenceReason, Final: p.Final, ReceiptEnabled: p.Execution.ReceiptEnabled, FinishOperation: cmd.FinishOperation})
	if err != nil {
		return out, err
	}
	handoffs := []app.OperationHandoffAllocation{}
	if p.PredecessorRequired {
		if cmd.ActualOutputQty > 0 && cmd.ActualInputQty <= 0 {
			return out, fmt.Errorf("请填写实际使用的前序产出数量")
		}
		handoffs, err = app.AllocateOperationHandoff(p.PredecessorBatches, cmd.ActualInputQty, cmd.InputUnit)
		if err != nil {
			return out, err
		}
	}
	if runID <= 0 {
		started, e := r.startWorkOrderTx(ctx, tx, app.WorkOrderStartCommand{ID: wo.ID, Operator: cmd.Operator}, false)
		if e != nil {
			return out, e
		}
		runID = started.RunningItemID
		wo = started.WorkOrder
	}
	materials, err := resolveOperationMaterialUses(p.Materials, cmd.Materials, cmd.ActualOutputQty, p.PlannedQty, p.TotalPlannedQty, p.CompletedOutputQty)
	if err != nil {
		return out, err
	}
	// Persist the material quantities actually applied by this completion event.
	// The next partial completion preview uses these event rows to avoid
	// suggesting the already-consumed portion again.
	cmd.Materials = materials
	var timerMinutes int
	var timed bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT started_at IS NOT NULL,CEIL((COALESCE((metrics_json->>'timer_elapsed_seconds')::numeric,0)+CASE WHEN status='running' THEN GREATEST(0,EXTRACT(EPOCH FROM now()-COALESCE(resumed_at,started_at,now()))) ELSE 0 END)/60)::int FROM %s.job_cards WHERE id=$1`, r.schema), cmd.ID).Scan(&timed, &timerMinutes); err != nil {
		return out, err
	}
	var priorEventMinutes int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(COALESCE(NULLIF(payload->>'actual_minutes','')::int,NULLIF(payload->>'ActualMinutes','')::int,0)),0) FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted'`, r.schema), cmd.ID).Scan(&priorEventMinutes); err != nil {
		return out, err
	}
	actualMinutes := cmd.ActualMinutes
	if actualMinutes == 0 && timed {
		actualMinutes = timerMinutes - priorEventMinutes
		if actualMinutes < 0 {
			actualMinutes = 0
		}
	}
	estimated := actualMinutes == 0 && !timed && costMethod != "piece"
	opCost := float64(actualMinutes) * rate / 60
	if costMethod == "piece" {
		opCost = cmd.ActualOutputQty * pieceRate
	} else if estimated {
		totalPlanned := p.TotalPlannedQty
		if totalPlanned <= 0 {
			totalPlanned = p.PlannedQty
		}
		if totalPlanned > 0 {
			opCost = plannedCost * cmd.ActualOutputQty / totalPlanned
		} else {
			opCost = 0
			if plannedCost > 0 {
				opCost = plannedCost
			} else if plannedMinutes > 0 {
				opCost = float64(plannedMinutes) * rate / 60
			}
		}
	}
	cmd.ActualMinutes = actualMinutes
	payload, _ := json.Marshal(cmd)
	receipt := app.OperationCompletionReceipt{OutputWeightG: app.OperationOutputWeight(cmd.ActualOutputQty, cmd.OutputUnit, wo.SpecG, wo.PlannedOutputG, operationPlannedTotal(wo)), OutputQty: cmd.ActualOutputQty, OutputUnit: cmd.OutputUnit, Warehouse: plan.Warehouse, WarehouseName: p.WarehouseName, StockEntryIDs: []int64{}, Handoffs: handoffs, EstimatedCost: estimated}
	err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(work_order_id,job_card_id,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,planned_qty,difference_reason,warehouse,receipt_enabled,final_output,operation_cost,estimated_cost,payload)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17::jsonb) RETURNING id`, r.schema), wo.ID, cmd.ID, cmd.RequestID, cmd.ExpectedVersion, cmd.Operator, cmd.ActualInputQty, cmd.InputUnit, cmd.ActualOutputQty, cmd.OutputUnit, p.PlannedQty, cmd.DifferenceReason, plan.Warehouse, p.Execution.ReceiptEnabled, p.Final, opCost, estimated, string(payload)).Scan(&receipt.EventID)
	if err != nil {
		return out, err
	}
	stockCmd := stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: wo.ID, JobCardID: cmd.ID, RunningItemID: runID, SourceType: "operation_completion", SourceID: receipt.EventID, Operator: cmd.Operator, IdempotencyKey: fmt.Sprintf("operation:%d:consume", receipt.EventID), Note: "工序完成自动用料；来源工单 " + wo.WorkOrderNo}
	for _, m := range materials {
		if m.Qty == 0 {
			continue
		}
		item := stockapp.StockDocumentItemCommand{MaterialID: m.MaterialID, ProductID: m.ProductID, BomSpecID: m.BomSpecID, BomVariantID: m.BomVariantID, SpecG: m.SpecG, ItemName: m.Name, InventoryUnit: m.Unit, FromWarehouse: m.Warehouse, OwnerCustomerID: m.OwnerCustomerID, ItemType: "material"}
		if m.ProductID > 0 {
			item.ItemType = "finished_product"
		}
		if factor := app.OperationUnitFactor(m.Unit); factor > 0 {
			item.QtyG = int64(math.Round(m.Qty * factor))
		} else {
			item.QtyUnits = int64(m.Qty)
		}
		stockCmd.Items = append(stockCmd.Items, item)
	}
	materialCost := 0.0
	if len(stockCmd.Items) > 0 {
		doc, e := stockrepo.NewRepository(r.pool, r.schema).ConsumeOperationMaterialsTx(ctx, tx, stockCmd)
		if e != nil {
			return out, e
		}
		materialCost = doc.TotalCost
		receipt.StockEntryIDs = append(receipt.StockEntryIDs, doc.ID)
		if err = r.recordOperationConsumptionLogsTx(ctx, tx, wo, doc.ID, cmd.Operator); err != nil {
			return out, err
		}
	}
	carriedCost := 0.0
	for _, a := range handoffs {
		tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_balances SET remaining_qty=remaining_qty-$2,remaining_cost=remaining_cost-$3 WHERE id=$1 AND remaining_qty>=$2`, r.schema), a.BatchID, a.Qty, a.Cost)
		if e != nil {
			return out, e
		}
		if tag.RowsAffected() != 1 {
			return out, fmt.Errorf("前序产出已被使用，请刷新")
		}
		if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_movements(event_id,source_balance_id,qty,unit,warehouse,cost) VALUES($1,$2,$3,$4,$5,$6)`, r.schema), receipt.EventID, a.BatchID, a.Qty, a.Unit, a.Warehouse, a.Cost); e != nil {
			return out, e
		}
		carriedCost += a.Cost
	}
	var ownerID int64
	for _, m := range materials {
		if m.OwnerCustomerID > 0 && m.Qty > 0 {
			if ownerID > 0 && ownerID != m.OwnerCustomerID {
				return out, fmt.Errorf("不同货主的物料不能混用")
			}
			ownerID = m.OwnerCustomerID
		}
	}
	for _, b := range p.PredecessorBatches {
		for _, allocation := range handoffs {
			if allocation.BatchID == b.ID && b.OwnerCustomerID > 0 {
				if ownerID > 0 && b.OwnerCustomerID != ownerID {
					return out, fmt.Errorf("不同货主的工序产出不能混用")
				}
				ownerID = b.OwnerCustomerID
			}
		}
	}
	warehouseOwner, err := warehouseCustomerID(ctx, tx, r.schema, plan.Warehouse)
	if err != nil {
		return out, err
	}
	if (p.Final && ownerID > 0 && warehouseOwner != ownerID) || (warehouseOwner > 0 && ownerID > 0 && warehouseOwner != ownerID) {
		return out, fmt.Errorf("产出货主与入库仓不匹配")
	}
	if p.Final {
		ownerID = warehouseOwner
	}
	if !p.Final && cmd.ActualOutputQty > 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,owner_customer_id,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8,$9,$9)`, r.schema), receipt.EventID, wo.ID, cmd.ID, seq, ownerID, plan.Warehouse, cmd.OutputUnit, cmd.ActualOutputQty, carriedCost+materialCost+opCost); err != nil {
			return out, err
		}
	}
	metrics := map[string]any{}
	_ = json.Unmarshal([]byte(cmd.MetricsJSON), &metrics)
	metrics["quantity_basis"] = "inventory_unit"
	metrics["inventory_unit"] = cmd.OutputUnit
	metrics["input_unit"] = cmd.InputUnit
	metrics["output_weight_g"] = receipt.OutputWeightG
	metrics["cost_estimated"] = estimated
	metrics["completion_event_id"] = receipt.EventID
	metricsRaw, _ := json.Marshal(metrics)
	loss := 0.0
	if cmd.InputUnit == cmd.OutputUnit {
		loss = math.Max(0, cmd.ActualInputQty-cmd.ActualOutputQty)
	}
	nextStatus := status
	if plan.CloseOperation {
		nextStatus = "completed"
	} else if status == "pending" || status == "ready" {
		nextStatus = "running"
	}
	var priorInput, priorOutput, priorLoss, priorCost float64
	var priorMinutes int
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT actual_input_qty::float8,actual_output_qty::float8,actual_loss_qty::float8,actual_minutes,actual_operation_cost::float8 FROM %s.job_cards WHERE id=$1`, r.schema), cmd.ID).Scan(&priorInput, &priorOutput, &priorLoss, &priorMinutes, &priorCost); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET assigned_employee_id=CASE WHEN status IN ('pending','ready') THEN $11 ELSE assigned_employee_id END,assigned_to=CASE WHEN status IN ('pending','ready') THEN $12 ELSE assigned_to END,assigned_employee_name=CASE WHEN status IN ('pending','ready') THEN $12 ELSE assigned_employee_name END,status=$10,completed_at=CASE WHEN $10='completed' THEN now() ELSE NULL END,operator=$2,actual_input_qty=$3,actual_output_qty=$4,actual_loss_qty=$5,actual_minutes=$6,actual_operation_cost=$7,exception_reason=$8,metrics_json=metrics_json||$9::jsonb,completion_draft='{}'::jsonb WHERE id=$1`, r.schema), cmd.ID, cmd.Operator, priorInput+cmd.ActualInputQty, priorOutput+cmd.ActualOutputQty, priorLoss+loss, priorMinutes+actualMinutes, priorCost+opCost, cmd.DifferenceReason, string(metricsRaw), nextStatus, cmd.EmployeeID, firstNonEmpty(cmd.AssignedEmployeeName, cmd.Operator)); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET material_cost=$2 WHERE id=$1`, r.schema), receipt.EventID, materialCost); err != nil {
		return out, err
	}
	if p.Final && cmd.ActualOutputQty > 0 {
		entryID, e := r.receiveOperationOutputTx(ctx, tx, wo, receipt, carriedCost+materialCost+opCost, cmd.Operator)
		if e != nil {
			return out, e
		}
		receipt.StockEntryIDs = append(receipt.StockEntryIDs, entryID)
	}
	receipt.Message = "已交下一工序"
	if !plan.CloseOperation {
		receipt.Message = "本批已完成，剩余数量保留待办"
	}
	if p.Execution.ReceiptEnabled {
		receipt.Message = "已入" + p.WarehouseName
		if !plan.CloseOperation {
			receipt.Message += "，本批完成，剩余数量保留待办"
		}
		if cmd.ActualOutputQty == 0 {
			receipt.Message = "本次无产出"
			if plan.CloseOperation {
				receipt.Message = "工序已结束，本次无产出"
			} else {
				receipt.Message += "，工序保留待办"
			}
		}
	}
	if plan.CloseOperation {
		if err = r.finishOperationWorkOrderTx(ctx, tx, wo, cmd.Operator); err != nil {
			return out, err
		}
	} else {
		summary, e := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, wo.ID)
		if e != nil {
			return out, e
		}
		var currentCost float64
		if e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(material_cost+operation_cost),0)::float8 FROM %s.production_operation_events WHERE work_order_id=$1 AND kind='complete' AND status IN('posting','posted')`, r.schema), wo.ID).Scan(&currentCost); e != nil {
			return out, e
		}
		if _, e = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='partially_completed',actual_cost=$2,operation_summary_json=$3::jsonb WHERE id=$1`, r.schema), wo.ID, currentCost, summary); e != nil {
			return out, e
		}
	}
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.ID, "complete", infra.StrPtr("status"), infra.StrPtr(status), infra.StrPtr("completed"), infra.AuditMeta{"event_id": receipt.EventID, "request_id": cmd.RequestID, "actual_input_qty": cmd.ActualInputQty, "input_unit": cmd.InputUnit, "actual_output_qty": cmd.ActualOutputQty, "output_unit": cmd.OutputUnit, "difference_reason": cmd.DifferenceReason, "materials": materials, "handoffs": handoffs, "stock_entry_ids": receipt.StockEntryIDs, "warehouse": plan.Warehouse, "cost_estimated": estimated}); err != nil {
		return out, err
	}
	out.JobCard, err = loadJobCardRowTx(ctx, tx, r.schema, cmd.ID)
	if err != nil {
		return out, err
	}
	out.WorkOrder, err = loadWorkOrderExecutionRowTx(ctx, tx, r.schema, wo.ID)
	if err != nil {
		return out, err
	}
	out.Completion = &receipt
	result, _ := json.Marshal(receipt)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET status='posted',result=$2::jsonb WHERE id=$1`, r.schema), receipt.EventID, string(result)); err != nil {
		return out, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "operation_complete", cmd.Operator, cmd.RequestID, receipt.EventID, out); err != nil {
		return out, err
	}
	return out, nil
}

func (r Repository) freezeLegacyPackagingConfigTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, sequence int, preview app.OperationCompletionPreview, actor string) error {
	var beforeSnapshot, beforeBindings []byte
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT process_snapshot_json::text,operation_material_bindings::text FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), wo.ID).Scan(&beforeSnapshot, &beforeBindings); err != nil {
		return err
	}
	var snapshot map[string]any
	if err := json.Unmarshal(beforeSnapshot, &snapshot); err != nil {
		return fmt.Errorf("旧包装工艺快照格式异常，不能自动接入")
	}
	operations, ok := snapshot["operations"].([]any)
	if !ok || len(operations) != 1 {
		return fmt.Errorf("旧包装工艺路线已变化，请刷新核对")
	}
	cfgRaw, _ := json.Marshal(preview.Execution)
	matched := false
	for _, raw := range operations {
		operation, ok := raw.(map[string]any)
		operationSeq, _ := strconv.Atoi(fmt.Sprint(operation["seq"]))
		if !ok || operationSeq != sequence || !isPackagingOperationName(fmt.Sprint(operation["operation"])) {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(operation["execution_json"])) != "" && strings.TrimSpace(fmt.Sprint(operation["execution_json"])) != "<nil>" {
			return fmt.Errorf("工艺路线配置已变化，请刷新核对")
		}
		operation["execution_json"] = string(cfgRaw)
		matched = true
	}
	if !matched {
		return fmt.Errorf("旧包装工艺路线已变化，请刷新核对")
	}
	bindings := map[string]app.OperationMaterialBinding{}
	if len(beforeBindings) > 0 {
		_ = json.Unmarshal(beforeBindings, &bindings)
	}
	for _, material := range preview.Materials {
		bindings[material.Key] = app.OperationMaterialBinding{NodeKey: preview.Execution.NodeKey, SourceWarehouse: material.Warehouse}
	}
	snapshotRaw, _ := json.Marshal(snapshot)
	bindingsRaw, _ := json.Marshal(bindings)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings=$3::jsonb,updated_at=now() WHERE id=$1`, r.schema), wo.ID, string(snapshotRaw), string(bindingsRaw)); err != nil {
		return err
	}
	oldSnapshot, newSnapshot := string(beforeSnapshot), string(snapshotRaw)
	return infra.AuditInsertTx(ctx, tx, r.schema, actor, "work_order", &wo.ID, "freeze_legacy_packaging_execution", infra.StrPtr("process_snapshot_json"), &oldSnapshot, &newSnapshot, infra.AuditMeta{"job_card_id": preview.JobCardID, "source_warehouse": "wip", "target_warehouse": preview.Warehouse, "reason": "单卡冻结资料完整的旧包装工单首次自动入库"})
}

// Material stock is posted in whole grams. Keep previews, event payloads and costs
// at the same precision as the ledger, including user-adjusted quantities.
func operationLedgerQuantity(qty float64, unit string) float64 {
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		return math.Round(qty*factor) / factor
	}
	return qty
}

func resolveOperationMaterialUses(expected, submitted []app.OperationMaterialUse, actual, planned, totalPlanned, completed float64) ([]app.OperationMaterialUse, error) {
	out := append([]app.OperationMaterialUse(nil), expected...)
	if submitted == nil {
		for i := range out {
			if out[i].TotalQty > 0 && totalPlanned > 0 {
				// A confirmed completion may exceed plan. Planned quantities are a
				// comparison baseline; consumption follows actual cumulative output.
				cumulative := math.Max(0, completed+actual)
				target := out[i].TotalQty * cumulative / totalPlanned
				if app.OperationUnitFactor(out[i].Unit) == 0 {
					target = math.Ceil(target - 1e-9)
				}
				out[i].Qty = math.Max(0, operationLedgerQuantity(target, out[i].Unit)-out[i].ConsumedQty)
			} else {
				factor := 1.0
				if planned > 0 {
					factor = actual / planned
				}
				out[i].Qty *= factor
				if app.OperationUnitFactor(out[i].Unit) == 0 {
					out[i].Qty = math.Ceil(out[i].Qty)
				}
			}
			out[i].Qty = operationLedgerQuantity(out[i].Qty, out[i].Unit)
		}
		return out, nil
	}
	if len(submitted) != len(expected) {
		return nil, fmt.Errorf("用料清单已变化，请刷新")
	}
	seen := map[int64]bool{}
	for i, row := range out {
		found := false
		for _, given := range submitted {
			if given.ReservationID == row.ReservationID {
				if seen[given.ReservationID] {
					return nil, fmt.Errorf("用料行重复")
				}
				seen[given.ReservationID] = true
				if given.Key != row.Key || given.Warehouse != row.Warehouse || given.Unit != row.Unit || given.OwnerCustomerID != row.OwnerCustomerID {
					return nil, fmt.Errorf("物料身份或来源仓已变化")
				}
				if err := app.ValidateOperationQuantity(given.Qty, row.Unit); err != nil {
					return nil, err
				}
				out[i].Qty = operationLedgerQuantity(given.Qty, row.Unit)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("用料清单不完整")
		}
	}
	return out, nil
}

func (r Repository) finishOperationWorkOrderTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, actor string) error {
	summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, wo.ID)
	if err != nil {
		return err
	}
	var remaining int
	var materialCost, operationCost float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.job_cards WHERE work_order_id=$1 AND status NOT IN('completed','cancelled')`, r.schema), wo.ID).Scan(&remaining); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(material_cost),0)::float8,COALESCE(SUM(operation_cost),0)::float8 FROM %s.production_operation_events WHERE work_order_id=$1`, r.schema), wo.ID).Scan(&materialCost, &operationCost); err != nil {
		return err
	}
	state := "partially_completed"
	if remaining == 0 {
		state = "completed"
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status=$2,actual_cost=$3,operation_summary_json=$4::jsonb,completed_at=CASE WHEN $2='completed' THEN COALESCE(completed_at,now()) ELSE completed_at END WHERE id=$1`, r.schema), wo.ID, state, materialCost+operationCost, summary); err != nil {
		return err
	}
	var finishedG int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.entry_type='finished_receipt' AND e.status='submitted'`, r.schema), wo.ID).Scan(&finishedG); err != nil {
		return err
	}
	unitCost := 0.0
	if finishedG > 0 {
		unitCost = (materialCost + operationCost) * 1000 / float64(finishedG)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_batch_costs(running_item_id,batch_id,product_name,material_cost,operation_cost,total_cost,finished_g,unit_cost_per_kg) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(running_item_id) DO UPDATE SET material_cost=EXCLUDED.material_cost,operation_cost=EXCLUDED.operation_cost,total_cost=EXCLUDED.total_cost,finished_g=EXCLUDED.finished_g,unit_cost_per_kg=EXCLUDED.unit_cost_per_kg`, r.schema), wo.RunningItemID, wo.BatchID, wo.ProductName, materialCost, operationCost, materialCost+operationCost, finishedG, unitCost); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_at=COALESCE(finished_at,now()),finished_by=CASE WHEN COALESCE(finished_by,'')='' THEN $2 ELSE finished_by END WHERE id=$1`, r.schema), wo.RunningItemID, actor); err != nil {
			return err
		}
		if err = releaseMaterialReservationsForRunningItemTx(ctx, tx, r.schema, wo.RunningItemID); err != nil {
			return err
		}
		if has, e := schemaColumnExistsTx(ctx, tx, r.schema, "customer_processing_material_reservations", "id"); e != nil {
			return e
		} else if has {
			tag, e := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_processing_material_reservations SET status=CASE WHEN consumed_g>0 OR consumed_units>0 THEN 'consumed' ELSE 'released' END,updated_at=now() WHERE work_order_id=$1 AND status='reserved'`, r.schema), wo.ID)
			if e != nil {
				return e
			}
			if tag.RowsAffected() > 0 {
				if e = infra.AuditInsertTx(ctx, tx, r.schema, actor, "work_order", &wo.ID, "release_unused_reservations", nil, nil, nil, infra.AuditMeta{"count": tag.RowsAffected(), "physical_movement": false}); e != nil {
					return e
				}
			}
		}
		if err = completeProductionPlanIfAllWorkOrdersDoneTx(ctx, tx, r.schema, wo.ProductionPlanID, actor); err != nil {
			return err
		}
		for _, orderNo := range splitOrderNos(wo.OrderNos) {
			order, changed, e := completeOrderIfAllRunningDone(ctx, tx, r.schema, orderNo)
			if e != nil {
				return e
			}
			if changed {
				if e = infra.AuditInsertTx(ctx, tx, r.schema, actor, "order", &order.OrderID, "production_complete", infra.StrPtr("process_status"), nil, infra.StrPtr("生产完成"), infra.AuditMeta{"work_order_id": wo.ID, "order_no": order.OrderNo}); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
