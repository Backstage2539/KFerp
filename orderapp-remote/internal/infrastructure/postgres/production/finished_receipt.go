package production

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
	infra "orderapp/internal/infrastructure/postgres"
	"orderapp/internal/infrastructure/postgres/customerstock"
	"strconv"
	"strings"
)

// All quantity-changing paths acquire this lock before work-order/card row locks.
func lockReceiptWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, id int64) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("%s:finished-receipt:%d", schema, id))
	return err
}

func (r Repository) EnrichFinishedReceiptTask(ctx context.Context, previous app.FinishedReceiptTask) (app.FinishedReceiptTask, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return previous, err
	}
	defer tx.Rollback(ctx)
	task, _, err := loadReceiptContextTx(ctx, tx, r.schema, previous.WorkOrderID)
	if err != nil {
		return previous, err
	}
	task.ReceiptStatus = previous.ReceiptStatus
	task.QualityStatus = previous.QualityStatus
	task.QualityBlocked = previous.QualityBlocked || task.QualityBlocked
	return task, tx.Commit(ctx)
}

func isCountReceipt(unit string) bool {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "g", "克", "kg", "千克", "公斤":
		return false
	}
	return true
}
func receiptWeightQuantity(g float64, unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		return g / 1000
	}
	return g
}

// The same snapshot supplies display, correction versions and posting limits.
func loadReceiptContextTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (app.FinishedReceiptTask, app.WorkOrderRow, error) {
	var task app.FinishedReceiptTask
	wo, err := loadWorkOrderExecutionRowTx(ctx, tx, schema, id)
	if err != nil {
		return task, wo, err
	}
	var inventoryUnit, snapshot, woVersion, runVersion string
	var planned, remainingUnits float64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT inventory_unit,planned_inventory_qty::float8,sales_spec_snapshot_json::text,w.xmin::text,COALESCE(r.xmin::text,''),COALESCE(r.planned_units,0)::float8 FROM %[1]s.work_orders w LEFT JOIN %[1]s.produce_running_items r ON r.id=w.running_item_id WHERE w.id=$1`, schema), id).Scan(&inventoryUnit, &planned, &snapshot, &woVersion, &runVersion, &remainingUnits)
	if err != nil {
		return task, wo, err
	}
	unit := strings.TrimSpace(wo.OutputUnit)
	if unit == "" {
		unit = inventoryUnit
	}
	if unit == "" {
		unit = "袋"
	}
	task = app.FinishedReceiptTask{WorkOrderID: wo.ID, WorkOrderNo: wo.WorkOrderNo, OutputType: wo.OutputType, OutputProductID: wo.OutputProductID, ProductID: wo.ProductID, OutputMaterialID: wo.OutputMaterialID, BomSpecID: wo.BomSpecID, BomVariantID: wo.BomVariantID, CustomerID: wo.CustomerID, OutputName: wo.OutputName, OutputUnit: unit, SpecG: wo.SpecG, TargetWarehouse: wo.TargetWarehouse}
	if task.OutputName == "" {
		task.OutputName = wo.ProductName
	}
	var frozen struct {
		SpecLabel string `json:"spec_label"`
	}
	_ = json.Unmarshal([]byte(snapshot), &frozen)
	task.PackageSpecLabel = strings.TrimSpace(frozen.SpecLabel)
	if task.PackageSpecLabel == "" && wo.SpecG > 0 {
		task.PackageSpecLabel = fmt.Sprintf("%dg/%s", wo.SpecG, unit)
	}
	if planned <= 0 {
		planned = wo.OutputQty
	}
	if planned <= 0 && !isCountReceipt(unit) {
		planned = receiptWeightQuantity(float64(wo.PlannedOutputG), unit)
	}
	if planned <= 0 && wo.SpecG > 0 {
		planned = float64(wo.PlannedOutputG) / float64(wo.SpecG)
	}
	task.PlannedQty = planned
	warehouse, err := completionWarehouseForWorkOrder(wo, "")
	if err != nil {
		task.BlockedReason = "目标仓库未确认"
	} else {
		task.TargetWarehouse = warehouse
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name,COALESCE(customer_id,0) FROM %s.warehouses WHERE code=$1 AND active=true`, schema), warehouse).Scan(&task.TargetWarehouseName, &task.InventoryOwnerCustomerID)
		if err == pgx.ErrNoRows {
			task.BlockedReason = "目标仓库不存在或已停用"
		} else if err != nil {
			return task, wo, err
		}
		if wo.OutputType == "material" {
			var owner int64
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(owner_customer_id,0) FROM %s.materials WHERE id=$1`, schema), wo.OutputMaterialID).Scan(&owner); err != nil {
				return task, wo, err
			}
			if owner != task.InventoryOwnerCustomerID && !(task.InventoryOwnerCustomerID == 0 && warehouse == "wip") {
				task.BlockedReason = "入库仓与产出物料货主不匹配"
			}
			task.InventoryOwnerCustomerID = owner
		}
	}
	var receivedG, receivedUnits float64
	var receiptCount int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT e.id),COALESCE(SUM(i.qty_g),0)::float8,COALESCE(SUM(i.qty_units),0)::float8 FROM %[1]s.stock_entries e LEFT JOIN %[1]s.stock_entry_items i ON i.stock_entry_id=e.id WHERE e.work_order_id=$1 AND e.entry_type='finished_receipt' AND e.status='submitted'`, schema), id).Scan(&receiptCount, &receivedG, &receivedUnits)
	if err != nil {
		return task, wo, err
	}
	task.ReceivedQty = receivedUnits
	if !isCountReceipt(unit) {
		task.ReceivedQty = receiptWeightQuantity(receivedG, unit)
	}
	task.RemainingPlanQty = math.Max(0, planned-task.ReceivedQty)
	if wo.BomSpecID > 0 && isCountReceipt(unit) && wo.RunningItemID > 0 {
		task.RemainingPlanQty = remainingUnits
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT id,sequence_no,operation,status,actual_output_qty::float8,metrics_json::text,xmin::text,COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),'') FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id`, schema), id)
	if err != nil {
		return task, wo, err
	}
	type card struct {
		ID                          int64
		Seq                         int
		Operation, Status           string
		Qty                         float64
		Metrics, Version, Completed string
	}
	cards := []card{}
	last := 0
	allComplete := true
	versions := []string{woVersion, runVersion, snapshot}
	for rows.Next() {
		var c card
		if err = rows.Scan(&c.ID, &c.Seq, &c.Operation, &c.Status, &c.Qty, &c.Metrics, &c.Version, &c.Completed); err != nil {
			rows.Close()
			return task, wo, err
		}
		cards = append(cards, c)
		if c.Seq > last {
			last = c.Seq
		}
		versions = append(versions, fmt.Sprintf("%d:%s", c.ID, c.Version))
		if c.Status != "completed" && c.Status != "cancelled" {
			allComplete = false
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return task, wo, err
	}
	for _, c := range cards {
		if c.Seq != last || c.Status != "completed" {
			continue
		}
		task.ReportedQty += c.Qty
		task.CorrectionCards = append(task.CorrectionCards, app.ReceiptCorrectionCard{ID: c.ID, Operation: c.Operation, ActualOutputQty: c.Qty})
		if c.Completed > task.LastReportedAt {
			task.LastReportedAt = c.Completed
		}
	}
	if wo.Status == "completed" && task.ReportedQty <= 0 {
		task.ReportedQty = task.ReceivedQty
	}
	task.AvailableQty = math.Max(0, task.ReportedQty-task.ReceivedQty)
	if isCountReceipt(unit) {
		switch {
		case planned <= 0:
			task.BlockedReason = "工单冻结计划数量缺失，请先核对工单"
		case math.Abs(task.ReportedQty-math.Round(task.ReportedQty)) > 0.000001:
			task.BlockedReason = "按袋/件报工必须为整数，请更正报工"
		case task.ReportedQty > planned+0.000001:
			task.BlockedReason = fmt.Sprintf("报工 %g %s，计划 %g %s，需更正报工", task.ReportedQty, unit, planned, unit)
		case task.ReceivedQty > task.ReportedQty+0.000001:
			task.BlockedReason = "累计已入库超过报工数量，请核对历史单据"
		case task.AvailableQty > task.RemainingPlanQty+0.000001:
			task.BlockedReason = "报工与运行单剩余数量不一致，请核对工单"
		}
		if wo.OutputType != "material" && task.PackageSpecLabel == "" && task.BlockedReason == "" {
			task.BlockedReason = "每袋规格未确认，请核对工单冻结规格"
		}
	}
	if !allComplete && task.BlockedReason == "" {
		task.BlockedReason = "工序尚未完成报工"
	}
	var quality string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT lower(result) FROM %s.quality_inspections WHERE work_order_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1),'')`, schema), id).Scan(&quality); err != nil {
		return task, wo, err
	}
	task.QualityBlocked = taskQualityResultBlocks(quality)
	if task.QualityBlocked && task.BlockedReason == "" {
		task.BlockedReason = "质检阻塞，请先完成质量处理"
	}
	if wo.Status == "completed" || wo.Status == "cancelled" {
		task.AvailableQty = 0
		task.CorrectionBlockedReason = "工单已结束，不能更正报工"
	}
	var used bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.production_logs WHERE running_item_id=$2 AND $2>0) OR EXISTS(SELECT 1 FROM %[1]s.production_material_receipts WHERE work_order_id=$1) OR EXISTS(SELECT 1 FROM %[1]s.work_order_dependencies d JOIN %[1]s.work_orders consumer ON consumer.id=d.work_order_id WHERE d.depends_on_work_order_id=$1 AND (d.delivered_g>0 OR d.delivered_units>0 OR consumer.status IN ('running','partially_completed','completed')))`, schema), id, wo.RunningItemID).Scan(&used)
	if err != nil {
		return task, wo, err
	}
	if receiptCount > 0 || used {
		task.CorrectionBlockedReason = "已有入库或后道使用记录，请通过原单据处理，不能直接更正报工"
	}
	if !allComplete || len(task.CorrectionCards) == 0 {
		task.CorrectionBlockedReason = "仅已完成的末工序可在此更正"
	}
	if task.BlockedReason != "" {
		task.AvailableQty = 0
	}
	raw, _ := json.Marshal(struct {
		Task     app.FinishedReceiptTask
		Versions []string
	}{task, versions})
	task.ReceiptVersion = fmt.Sprintf("%x", sha256.Sum256(raw))
	return task, wo, nil
}

func validateReceiptQuantityTx(ctx context.Context, tx pgx.Tx, schema string, id int64, version string, quantity float64) error {
	task, _, err := loadReceiptContextTx(ctx, tx, schema, id)
	if err != nil {
		return err
	}
	if version != "" && version != task.ReceiptVersion {
		return fmt.Errorf("入库数据已变化，请刷新后重试")
	}
	if task.BlockedReason != "" {
		return fmt.Errorf("%s", task.BlockedReason)
	}
	if quantity <= 0 || quantity > task.AvailableQty+0.000001 {
		return fmt.Errorf("本次入库数量不能超过已确认可入库量 %g %s", task.AvailableQty, task.OutputUnit)
	}
	return nil
}

func (r Repository) CorrectJobCardOutput(ctx context.Context, cmd app.CorrectJobCardOutputCommand) (app.FinishedReceiptTask, error) {
	var empty app.FinishedReceiptTask
	var automatic bool
	if err := r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted')`, r.schema), cmd.ID).Scan(&automatic); err != nil {
		return empty, err
	}
	if automatic {
		return r.correctOperationOutput(ctx, cmd)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	var workOrderID, runID int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT j.work_order_id,w.running_item_id FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`, r.schema), cmd.ID).Scan(&workOrderID, &runID); err != nil {
		return empty, err
	}
	if runID > 0 {
		if err = customerstock.LockRunningFulfillmentCustomersTx(ctx, tx, r.schema, runID); err != nil {
			return empty, err
		}
	}
	if err = lockReceiptWorkOrderTx(ctx, tx, r.schema, workOrderID); err != nil {
		return empty, err
	}
	replayID, replay, err := requestProductionKeyTx(ctx, tx, r.schema, "correct_output", cmd.Operator, cmd.RequestID, cmd)
	if err != nil {
		return empty, err
	}
	if replayID > 0 {
		err = json.Unmarshal(replay, &empty)
		return empty, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE id=$1 FOR UPDATE`, r.schema), workOrderID); err != nil {
		return empty, err
	}
	task, _, err := loadReceiptContextTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return empty, err
	}
	if task.ReceiptVersion != cmd.ExpectedVersion {
		return empty, fmt.Errorf("报工或入库数据已变化，请刷新后重试")
	}
	if task.CorrectionBlockedReason != "" {
		return empty, fmt.Errorf("%s", task.CorrectionBlockedReason)
	}
	found := false
	previous := float64(0)
	for _, c := range task.CorrectionCards {
		if c.ID == cmd.ID {
			found = true
			previous = c.ActualOutputQty
		}
	}
	if !found {
		return empty, fmt.Errorf("只能选择已完成的末工序卡更正")
	}
	if isCountReceipt(task.OutputUnit) && math.Trunc(cmd.ActualOutputQty) != cmd.ActualOutputQty {
		return empty, fmt.Errorf("袋数必须为非负整数")
	}
	if task.PlannedQty <= 0 || task.ReportedQty-previous+cmd.ActualOutputQty > task.PlannedQty+0.000001 {
		return empty, fmt.Errorf("更正后累计产出不能超过冻结计划 %g %s", task.PlannedQty, task.OutputUnit)
	}
	var input float64
	var metrics string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT actual_input_qty::float8,metrics_json::text FROM %s.job_cards WHERE id=$1 FOR UPDATE`, r.schema), cmd.ID).Scan(&input, &metrics); err != nil {
		return empty, err
	}
	loss, rate, err := domain.ActualLossMetrics(input, cmd.ActualOutputQty)
	if err != nil {
		return empty, err
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(metrics), &m)
	if m == nil {
		m = map[string]any{}
	}
	m["inventory_unit"] = task.OutputUnit
	m["quantity_basis"] = "inventory_unit"
	if isCountReceipt(task.OutputUnit) {
		m["finished_units"] = cmd.ActualOutputQty
	}
	raw, _ := json.Marshal(m)
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=$2,actual_loss_qty=$3,actual_loss_rate=$4,metrics_json=$5::jsonb,actual_operation_cost=CASE WHEN cost_method='piece' THEN ROUND($2::numeric*COALESCE(piece_rate,0),4) ELSE actual_operation_cost END WHERE id=$1`, r.schema), cmd.ID, cmd.ActualOutputQty, loss, rate, string(raw))
	if err != nil {
		return empty, err
	}
	summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return empty, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, r.schema), workOrderID, summary); err != nil {
		return empty, err
	}
	oldValue, newValue := strconv.FormatFloat(previous, 'f', -1, 64), strconv.FormatFloat(cmd.ActualOutputQty, 'f', -1, 64)
	if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.ID, "correct_output", infra.StrPtr("actual_output_qty"), &oldValue, &newValue, infra.AuditMeta{"work_order_id": workOrderID, "work_order_no": task.WorkOrderNo, "unit": task.OutputUnit, "reason": cmd.Reason, "request_id": cmd.RequestID, "previous_version": cmd.ExpectedVersion}); err != nil {
		return empty, err
	}
	task, _, err = loadReceiptContextTx(ctx, tx, r.schema, workOrderID)
	if err != nil {
		return empty, err
	}
	if err = finishProductionKeyTx(ctx, tx, r.schema, "correct_output", cmd.Operator, cmd.RequestID, cmd.ID, task); err != nil {
		return empty, err
	}
	return task, tx.Commit(ctx)
}
