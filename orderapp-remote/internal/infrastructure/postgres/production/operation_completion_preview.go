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

func isPackagingOperationName(value string) bool {
	return strings.Contains(value, "包装") || strings.Contains(value, "分装") || strings.Contains(value, "装袋") || strings.Contains(value, "装盒")
}

func isRoastingOperationName(value string) bool {
	return strings.Contains(value, "烘焙") || strings.Contains(value, "烘烤") || strings.Contains(value, "除石")
}

const invalidFrozenExecutionReason = "冻结工序报工类型的执行版本缺失或无效，请核对工艺路线配置"

func decodeFrozenOperationExecution(input string) (app.OperationExecution, bool, error) {
	var cfg app.OperationExecution
	if strings.TrimSpace(input) == "" {
		return cfg, false, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		return cfg, false, err
	}
	var reported string
	_ = json.Unmarshal(raw["reporting_type"], &reported)
	reported = strings.TrimSpace(reported)
	modern := reported != "" && reported != "legacy"
	if err := json.Unmarshal([]byte(input), &cfg); err != nil {
		if !modern {
			return cfg, false, err
		}
		cfg.ReportingType = reported
		return cfg, true, nil
	}
	cfg.ReportingType = reported
	return cfg, modern && (cfg.Version != 1 || (reported != "batch" && reported != "incoming" && reported != "piece")), nil
}

func incompleteRoastingBOMReason(totalRatioPct float64) string {
	return app.IncompleteRoastingBOMReason(totalRatioPct)
}

func resolveRoastingMaterialUses(expected []app.OperationMaterialUse, actualInputQty float64, inputUnit string) ([]app.OperationMaterialUse, error) {
	return app.ResolveRoastingMaterialUses(expected, actualInputQty, inputUnit)
}

func shouldRefreshIncompleteRoastingRecipe(frozenRatio, publishedRatio float64, eligible bool) bool {
	return eligible && frozenRatio < 99.999 && publishedRatio >= 99.999
}

// Keep the whole work-order demand visible while reserving only what is needed
// for the current completion batch. This allows available batches to proceed
// without requiring the warehouse to cover every future split up front.
func roastingReservationTargets(plannedG, reservedG, consumedG, currentBatchG int64) (requiredG, reservedTargetG int64) {
	currentTotalG := consumedG + currentBatchG
	requiredG = maxInt64(plannedG, currentTotalG)
	reservedTargetG = maxInt64(reservedG, currentTotalG)
	return requiredG, reservedTargetG
}

func recipeRatioTotal(rows []materialSnapshotRow) float64 {
	total := 0.0
	for _, row := range rows {
		total += row.RatioPct
	}
	return total
}

func roastingMaterialSnapshotKey(row materialSnapshotRow) string {
	kind := strings.TrimSpace(row.ComponentType)
	component := row.MaterialID
	if kind == "finished_product" || kind == "product" {
		kind = "product"
		component = row.ComponentProductID
	} else if kind == "" {
		kind = "material"
	}
	return operationMaterialKey(kind, component, row.ComponentBomSpecID, row.ComponentSpecG)
}

func effectiveRoastingMaterialKeys(rows []materialSnapshotRow) map[string]struct{} {
	keys := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		keys[roastingMaterialSnapshotKey(row)] = struct{}{}
	}
	return keys
}

func shouldReleaseStaleRoastingReservation(key, nodeKey, roastingNodeKey string, effectiveKeys map[string]struct{}) bool {
	if strings.TrimSpace(nodeKey) != strings.TrimSpace(roastingNodeKey) {
		return false
	}
	_, exists := effectiveKeys[key]
	return !exists
}

func inferredRoastingBinding(route processTemplateSnapshot) (app.OperationMaterialBinding, bool) {
	var found app.OperationMaterialBinding
	count := 0
	for _, operation := range route.Operations {
		if !isRoastingOperationName(operation.Operation) || strings.TrimSpace(operation.ExecutionJSON) == "" {
			continue
		}
		var execution app.OperationExecution
		if json.Unmarshal([]byte(operation.ExecutionJSON), &execution) != nil || execution.Version != 1 || strings.TrimSpace(execution.NodeKey) == "" {
			continue
		}
		count++
		found = app.OperationMaterialBinding{NodeKey: execution.NodeKey, SourceWarehouse: execution.SourceWarehouse}
	}
	return found, count == 1
}

func roastingRecipeBinding(key string, mapped map[string]app.OperationMaterialBinding, route processTemplateSnapshot) app.OperationMaterialBinding {
	if binding := mapped[key]; strings.TrimSpace(binding.NodeKey) != "" {
		return binding
	}
	if binding, ok := inferredRoastingBinding(route); ok {
		return binding
	}
	return mapped[key]
}

func (r Repository) workOrderBOMRefreshEligibleTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow) (bool, error) {
	hasConsumptionLogs, err := schemaColumnExistsTx(ctx, tx, r.schema, "material_consumption_logs", "running_item_id")
	if err != nil {
		return false, err
	}
	logCheck := "false"
	if hasConsumptionLogs && wo.RunningItemID > 0 {
		logCheck = fmt.Sprintf("EXISTS(SELECT 1 FROM %s.material_consumption_logs WHERE running_item_id=$2)", r.schema)
	}
	var used bool
	query := fmt.Sprintf(`SELECT
		EXISTS(SELECT 1 FROM %[1]s.job_cards WHERE work_order_id=$1 AND (actual_input_qty>0 OR actual_output_qty>0))
		OR EXISTS(SELECT 1 FROM %[1]s.production_operation_events WHERE work_order_id=$1 AND status IN('posting','posted'))
		OR EXISTS(SELECT 1 FROM %[1]s.production_operation_balances WHERE work_order_id=$1)
		OR EXISTS(SELECT 1 FROM %[1]s.production_material_receipts WHERE work_order_id=$1)
		OR EXISTS(SELECT 1 FROM %[1]s.stock_entries WHERE work_order_id=$1 AND status='submitted' AND purpose='material_consumption_for_manufacture')
		OR EXISTS(SELECT 1 FROM %[1]s.stock_entries WHERE work_order_id=$1 AND status='submitted' AND entry_type='finished_receipt')
		OR EXISTS(SELECT 1 FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1 AND (consumed_g>0 OR consumed_units>0))
		OR EXISTS(SELECT 1 FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=$1 AND (consumed_g>0 OR consumed_units>0))
		OR %s`, r.schema, logCheck)
	args := []any{wo.ID}
	if hasConsumptionLogs && wo.RunningItemID > 0 {
		args = append(args, wo.RunningItemID)
	}
	if err := tx.QueryRow(ctx, query, args...).Scan(&used); err != nil {
		return false, err
	}
	return !used && wo.Status != "completed" && wo.Status != "cancelled", nil
}

func (r Repository) effectiveRoastingMaterialSnapshotTx(ctx context.Context, tx pgx.Tx, wo app.WorkOrderRow, frozen []materialSnapshotRow) ([]materialSnapshotRow, int64, bool, error) {
	frozenRatio := recipeRatioTotal(frozen)
	if frozenRatio >= 99.999 || wo.BomVersionID <= 0 {
		return frozen, wo.BomVersionID, false, nil
	}
	var bomID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id FROM %s.production_bom_versions WHERE id=$1`, r.schema), wo.BomVersionID).Scan(&bomID); err != nil {
		return frozen, wo.BomVersionID, false, err
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT candidate.id
		FROM %s.production_bom_versions candidate
		JOIN %s.production_bom_versions frozen ON frozen.id=$2 AND frozen.bom_id=candidate.bom_id
		WHERE candidate.bom_id=$1 AND candidate.status='published' AND candidate.id<>frozen.id
		  AND (COALESCE(candidate.published_at,candidate.created_at),candidate.id) >
		      (COALESCE(frozen.published_at,frozen.created_at),frozen.id)
		ORDER BY candidate.published_at DESC NULLS LAST,candidate.created_at DESC,candidate.id DESC
	`, r.schema, r.schema), bomID, wo.BomVersionID)
	if err != nil {
		return frozen, wo.BomVersionID, false, err
	}
	versionIDs := []int64{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return frozen, wo.BomVersionID, false, err
		}
		versionIDs = append(versionIDs, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return frozen, wo.BomVersionID, false, err
	}
	rows.Close()
	eligible, err := r.workOrderBOMRefreshEligibleTx(ctx, tx, wo)
	if err != nil {
		return frozen, wo.BomVersionID, false, err
	}
	if !eligible {
		return frozen, wo.BomVersionID, false, nil
	}
	inputIncludesLoss := false
	for _, row := range frozen {
		inputIncludesLoss = inputIncludesLoss || row.InputIncludesMaterialLoss
	}
	run := ProduceRunRow{ID: wo.RunningItemID, Product: firstNonEmpty(wo.ProductName, wo.OutputName), ProductID: wo.ProductID, SpecG: wo.SpecG, NeedG: wo.PlannedOutputG, InputG: wo.PlannedG}
	for _, versionID := range versionIDs {
		raw, buildErr := buildMaterialSnapshotForBomVersionVariantTx(ctx, tx, r.schema, run, versionID, wo.BomVariantID, inputIncludesLoss)
		if buildErr != nil {
			return frozen, wo.BomVersionID, false, buildErr
		}
		var candidate []materialSnapshotRow
		if err = json.Unmarshal(raw, &candidate); err != nil {
			return frozen, wo.BomVersionID, false, err
		}
		if shouldRefreshIncompleteRoastingRecipe(frozenRatio, recipeRatioTotal(candidate), eligible) {
			return candidate, versionID, true, nil
		}
	}
	return frozen, wo.BomVersionID, false, nil
}

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
	var snapshot, materialSnapshot, bindings, version, status, unit, salesSpec string
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF w,j"
	}
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT w.id,j.sequence_no,j.status,j.planned_input_qty::float8,
 (SELECT MIN(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=w.id AND status<>'cancelled'),
 (SELECT MAX(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=w.id AND status<>'cancelled'),
 (SELECT COALESCE(SUM(planned_input_qty),0)::float8 FROM %[1]s.job_cards WHERE work_order_id=w.id AND sequence_no=j.sequence_no AND status<>'cancelled'),
 (SELECT COALESCE(SUM(planned_input_qty),0)::float8 FROM %[1]s.job_cards WHERE work_order_id=w.id AND sequence_no=j.sequence_no AND id<j.id AND status<>'cancelled'),
 w.process_snapshot_json::text,COALESCE(w.material_snapshot,'[]'::jsonb)::text,w.operation_material_bindings::text,w.xmin::text||':'||j.xmin::text,
 COALESCE(NULLIF(w.output_unit,''),NULLIF(w.inventory_unit,''),'袋'),w.sales_spec_snapshot_json::text
 FROM %[1]s.job_cards j JOIN %[1]s.work_orders w ON w.id=j.work_order_id WHERE j.id=$1`+suffix, r.schema), id).Scan(&p.WorkOrderID, &seq, &status, &input, &first, &last, &totalInput, &beforeInput, &snapshot, &materialSnapshot, &bindings, &version, &unit, &salesSpec)
	if err != nil {
		return p, app.WorkOrderRow{}, err
	}
	wo, err := loadWorkOrderExecutionRowTx(ctx, tx, r.schema, p.WorkOrderID)
	if err != nil {
		return p, wo, err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT NULLIF(planned_minutes,0) FROM %s.job_cards WHERE id=$1`, r.schema), id).Scan(&p.PlannedMinutes); err != nil {
		return p, wo, err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT completion_draft,GREATEST(0,GREATEST(actual_minutes,CEIL((COALESCE((metrics_json->>'timer_elapsed_seconds')::numeric,0)+CASE WHEN status='running' THEN GREATEST(0,EXTRACT(EPOCH FROM now()-COALESCE(resumed_at,started_at,now()))) ELSE 0 END)/60)::int)-COALESCE((SELECT SUM(COALESCE(NULLIF(e.payload->>'actual_minutes','')::int,NULLIF(e.payload->>'ActualMinutes','')::int,0)) FROM %s.production_operation_events e WHERE e.job_card_id=$1 AND e.kind='complete' AND e.status='posted'),0)) FROM %s.job_cards WHERE id=$1`, r.schema, r.schema), id).Scan(&p.Draft, &p.ActualMinutes); err != nil {
		return p, wo, err
	}
	var route processTemplateSnapshot
	if err = json.Unmarshal([]byte(snapshot), &route); err != nil {
		return p, wo, err
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_consumption_policy FROM %s.work_orders WHERE id=$1`, r.schema), wo.ID).Scan(&p.MaterialConsumptionPolicy); err != nil {
		return p, wo, err
	}
	startPolicy := p.MaterialConsumptionPolicy == "bom_start_v1"
	if startPolicy {
		first, err = firstFrozenRouteSequence(route)
		last = 0
		for _, operation := range route.Operations {
			if operation.Seq > last {
				last = operation.Seq
			}
		}
		if err != nil {
			return p, wo, err
		}
	}
	var routeOperation string
	var invalidModernExecution bool
	for _, op := range route.Operations {
		if op.Seq == seq {
			routeOperation = op.Operation
			if op.ExecutionJSON != "" {
				p.Execution, invalidModernExecution, err = decodeFrozenOperationExecution(op.ExecutionJSON)
				if err != nil {
					return p, wo, err
				}
			}
			break
		}
	}
	p.OperationName = routeOperation
	p.Enabled = p.Execution.Version == 1 && !invalidModernExecution
	p.Final = seq == last
	if p.Execution.ReportingType == "batch" || p.Execution.ReportingType == "incoming" || p.Execution.ReportingType == "piece" {
		var timed bool
		var timer, baseline, unmarked int
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT j.started_at IS NOT NULL,ROUND(COALESCE((j.metrics_json->>'timer_elapsed_seconds')::numeric,0)+CASE WHEN j.status='running' THEN GREATEST(0,EXTRACT(EPOCH FROM now()-COALESCE(j.resumed_at,j.started_at,now()))) ELSE 0 END)::int,(SELECT COUNT(*) FROM %[1]s.production_operation_events e WHERE e.job_card_id=j.id AND e.kind='complete' AND e.status='posted' AND NOT e.payload ? 'timer_baseline_seconds')::int,COALESCE((SELECT (e.payload->>'timer_baseline_seconds')::int FROM %[1]s.production_operation_events e WHERE e.job_card_id=j.id AND e.kind='complete' AND e.status='posted' AND e.payload ? 'timer_baseline_seconds' ORDER BY e.id DESC LIMIT 1),0) FROM %[1]s.job_cards j WHERE j.id=$1`, r.schema), id).Scan(&timed, &timer, &unmarked, &baseline)
		if err != nil {
			return p, wo, err
		}
		if timed && unmarked == 0 {
			p.ActualMinutes = int(math.Ceil(float64(max(0, timer-baseline)) / 60))
		}
	}
	if invalidModernExecution {
		p.BlockedReason = invalidFrozenExecutionReason
		p.BlockedCode = "frozen_route_invalid"
	} else if !p.Enabled && startPolicy {
		p.BlockedReason = "冻结工艺路线的当前工序缺少执行配置，请核对工单"
		p.BlockedCode = "frozen_route_invalid"
	}
	if !p.Enabled && !startPolicy && !invalidModernExecution {
		p.BlockedReason = "旧工单暂不可自动完成，请核对冻结 BOM 与工艺路线"
		p.BlockedCode = "legacy_execution_unresolved"
		legacyRoute := len(route.Operations) == 1
		var cardCount, inputCardCount, eventCount int
		legacyPlannedQty := wo.PlannedInventoryQty
		if legacyPlannedQty <= 0 {
			legacyPlannedQty = wo.OutputQty
		}
		legacyOutputUnit := firstNonEmpty(wo.InventoryUnit, unit)
		if legacyRoute && seq == first && seq == last && legacyPlannedQty > 0 && strings.TrimSpace(legacyOutputUnit) != "" && (status == "pending" || status == "ready" || status == "released" || status == "running" || status == "paused") {
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*),COUNT(*) FILTER(WHERE planned_input_qty>0),(SELECT COUNT(*) FROM %s.production_operation_events WHERE work_order_id=$1 AND status IN('posting','posted')) FROM %s.job_cards WHERE work_order_id=$1 AND status<>'cancelled'`, r.schema, r.schema), wo.ID).Scan(&cardCount, &inputCardCount, &eventCount); err != nil {
				return p, wo, err
			}
		}
		legacyQuantityResolved := cardCount > 0 && inputCardCount == cardCount
		if isPackagingOperationName(routeOperation) {
			// A legacy packaging card is measured by the frozen work-order bag
			// quantity, not planned_input_qty (which is commonly zero for bag
			// operations). Only adopt this shortcut when there is exactly one
			// card, so we never guess how a multi-card quantity should split.
			legacyQuantityResolved = cardCount == 1
		}
		if legacyRoute && legacyQuantityResolved && eventCount == 0 && strings.TrimSpace(materialSnapshot) != "" && strings.TrimSpace(materialSnapshot) != "[]" {
			legacyClean, cleanErr := r.workOrderBOMRefreshEligibleTx(ctx, tx, wo)
			if cleanErr != nil {
				return p, wo, cleanErr
			}
			if legacyClean {
				p.Execution = app.OperationExecution{Version: 1, NodeKey: fmt.Sprintf("legacy-execution-%d", wo.ID), ReceiptEnabled: true, ReceiptWarehouse: firstNonEmpty(wo.TargetWarehouse, "wip"), SourceWarehouse: "wip", OutputUnit: unit}
				p.Enabled = true
				p.LegacyExecution = true
				p.LegacyPackaging = isPackagingOperationName(routeOperation)
				p.BlockedReason = ""
				p.BlockedCode = ""
			}
		}
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
	if wo.BomVersionID > 0 {
		p.BomVersionID = wo.BomVersionID
		p.OriginalBomVersionID = wo.BomVersionID
		p.EffectiveBomVersionID = wo.BomVersionID
		if e := tx.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id,version_no FROM %s.production_bom_versions WHERE id=$1`, r.schema), wo.BomVersionID).Scan(&p.BomID, &p.OriginalBomVersionNo); e != nil && e != pgx.ErrNoRows {
			return p, wo, e
		}
		p.EffectiveBomVersionNo = p.OriginalBomVersionNo
	}
	p.OutputUnit = firstNonEmpty(p.Execution.OutputUnit, unit)
	p.InputUnit = p.OutputUnit
	if p.Enabled && p.Final && p.OutputUnit != unit {
		p.BlockedReason = "末道产出单位与冻结商品单位不一致，请核对工单配置"
		p.BlockedCode = "output_unit_mismatch"
	}
	if p.Enabled && p.Final && wo.OutputType != "material" {
		if _, _, _, identityErr := resolveOperationOutputProductIdentityTx(ctx, tx, r.schema, wo); identityErr != nil {
			p.Enabled = false
			p.BlockedReason = identityErr.Error()
			p.BlockedCode = "output_identity_invalid"
		}
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
		factor := app.OperationUnitFactor(p.OutputUnit)
		p.PlannedQty = float64(roundedOperationWeightSlice(wo.PlannedOutputG, beforeInput, input, totalInput)) / factor
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
	if startPolicy && app.OperationUnitFactor(p.InputUnit) == 0 {
		p.InputQty = p.TotalPlannedQty
	}
	inputUnitFactor := app.OperationUnitFactor(p.InputUnit)
	if inputUnitFactor > 0 {
		p.InputQty = input / inputUnitFactor
	}
	inputEvents, err := tx.Query(ctx, fmt.Sprintf(`SELECT input_qty::float8,input_unit FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete' AND status='posted'`, r.schema), id)
	if err != nil {
		return p, wo, err
	}
	var alreadyInput float64
	for inputEvents.Next() {
		var qty float64
		var eventUnit string
		if err = inputEvents.Scan(&qty, &eventUnit); err != nil {
			inputEvents.Close()
			return p, wo, err
		}
		factor := app.OperationUnitFactor(eventUnit)
		if factor > 0 && inputUnitFactor > 0 {
			alreadyInput += qty * factor / inputUnitFactor
		} else if strings.EqualFold(eventUnit, p.InputUnit) {
			alreadyInput += qty
		}
	}
	if err = inputEvents.Err(); err != nil {
		inputEvents.Close()
		return p, wo, err
	}
	inputEvents.Close()
	p.InputQty = math.Max(0, p.InputQty-alreadyInput)
	p.Warehouse = firstNonEmpty(p.Execution.ReceiptWarehouse, wo.TargetWarehouse, "finished_goods")
	if !p.Execution.ReceiptEnabled {
		p.Warehouse = "wip"
	}
	if p.Final && !p.Execution.ReceiptEnabled && p.Enabled {
		p.BlockedReason = "最终产出工序请配置完成后入库"
		p.BlockedCode = "receipt_configuration"
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), p.Warehouse).Scan(&p.WarehouseName); err != nil {
		return p, wo, fmt.Errorf("入库仓不存在或已停用：%s", p.Warehouse)
	}
	var mapped map[string]app.OperationMaterialBinding
	if err = json.Unmarshal([]byte(bindings), &mapped); err != nil {
		return p, wo, err
	}
	var frozenMaterials []materialSnapshotRow
	if err = json.Unmarshal([]byte(materialSnapshot), &frozenMaterials); err != nil {
		return p, wo, err
	}
	if (!startPolicy && isRoastingOperationName(p.OperationName) || startPolicy && seq == first && recipeRatioTotal(frozenMaterials) > 0) && p.BomVersionID > 0 {
		resolved, versionID, refresh, resolveErr := r.effectiveRoastingMaterialSnapshotTx(ctx, tx, wo, frozenMaterials)
		if resolveErr != nil {
			return p, wo, resolveErr
		}
		if refresh {
			frozenMaterials = resolved
			p.EffectiveBomVersionID = versionID
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT version_no FROM %s.production_bom_versions WHERE id=$1`, r.schema), versionID).Scan(&p.EffectiveBomVersionNo); err != nil {
				return p, wo, err
			}
			p.BomRefreshPending = true
			p.BomRefreshReason = "仅在首次实际耗料前接入同一 BOM 最新已发布完整版本"
			raw, marshalErr := json.Marshal(resolved)
			err = marshalErr
			if err != nil {
				return p, wo, err
			}
			materialSnapshot = string(raw)
		} else if recipeRatioTotal(frozenMaterials) < 99.999 {
			p.BomRefreshReason = "工单已有实际执行记录，继续使用原冻结配方；请核对生产记录"
		}
	}
	p.EffectiveMaterialSnapshot = json.RawMessage(materialSnapshot)
	consumedByReservation := map[int64]float64{}
	effectiveRecipeKeys := map[string]struct{}{}
	isRoasting := isRoastingOperationName(p.OperationName)
	if startPolicy {
		isRoasting = seq == first && recipeRatioTotal(frozenMaterials) > 0
	}
	if isRoasting {
		effectiveRecipeKeys = effectiveRoastingMaterialKeys(frozenMaterials)
	}
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
		m.JobCardID = id
		if kind == "finished_product" {
			kind = "product"
		}
		if kind == "" {
			kind = "material"
		}
		m.Key = operationMaterialKey(kind, component, m.BomSpecID, m.SpecG)
		if isRoasting {
			if _, isCurrentRecipeMaterial := effectiveRecipeKeys[m.Key]; !isCurrentRecipeMaterial {
				continue
			}
		}
		for _, frozen := range frozenMaterials {
			frozenComponent := frozen.MaterialID
			if strings.EqualFold(frozen.ComponentType, "product") {
				frozenComponent = frozen.ComponentProductID
			}
			if frozenComponent == component && frozen.ComponentBomSpecID == m.BomSpecID && frozen.ComponentBomVariantID == m.BomVariantID && frozen.ComponentSpecG == m.SpecG {
				m.RatioPct = frozen.RatioPct
				break
			}
		}
		binding := mapped[m.Key]
		if startPolicy {
			if seq != first {
				continue
			}
			binding.NodeKey = p.Execution.NodeKey
		}
		if p.LegacyExecution {
			binding.NodeKey = p.Execution.NodeKey
			binding.SourceWarehouse = firstNonEmpty(binding.SourceWarehouse, "wip")
		} else if p.BomRefreshPending && !startPolicy {
			binding = roastingRecipeBinding(m.Key, mapped, route)
		}
		m.NodeKey = binding.NodeKey
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
			p.BlockedCode = "material_binding_missing"
		}
		if m.NodeKey != p.Execution.NodeKey {
			continue
		}
		m.Warehouse = firstNonEmpty(binding.SourceWarehouse, p.Execution.SourceWarehouse, "wip")
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
	if isRoasting {
		seen := map[string]bool{}
		for _, material := range p.Materials {
			seen[material.Key] = true
		}
		for _, frozen := range frozenMaterials {
			kind := strings.TrimSpace(frozen.ComponentType)
			component := frozen.MaterialID
			if kind == "finished_product" || kind == "product" {
				kind = "product"
				component = frozen.ComponentProductID
			} else if kind == "" {
				kind = "material"
			}
			key := operationMaterialKey(kind, component, frozen.ComponentBomSpecID, frozen.ComponentSpecG)
			if seen[key] {
				continue
			}
			m := app.OperationMaterialUse{JobCardID: id, Key: key, Name: frozen.MaterialName, Unit: frozen.Unit, RatioPct: frozen.RatioPct,
				BomSpecID: frozen.ComponentBomSpecID, BomVariantID: frozen.ComponentBomVariantID, SpecG: frozen.ComponentSpecG}
			if kind == "product" {
				m.ProductID = component
			} else {
				m.MaterialID = component
				if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(owner_customer_id,0) FROM %s.materials WHERE id=$1`, r.schema), component).Scan(&m.OwnerCustomerID); err != nil {
					return p, wo, err
				}
			}
			binding := roastingRecipeBinding(key, mapped, route)
			if startPolicy {
				binding = mapped[key]
				binding.NodeKey = p.Execution.NodeKey
			}
			if p.LegacyExecution {
				binding.NodeKey = p.Execution.NodeKey
				binding.SourceWarehouse = firstNonEmpty(binding.SourceWarehouse, "wip")
			}
			m.NodeKey = binding.NodeKey
			if m.NodeKey == "" && first == last {
				m.NodeKey = p.Execution.NodeKey
			}
			if m.NodeKey == "" && p.Enabled {
				p.BlockedReason = "请核对BOM组件使用工序：" + m.Name
				p.BlockedCode = "material_binding_missing"
				continue
			}
			if m.NodeKey != p.Execution.NodeKey {
				continue
			}
			m.Warehouse = firstNonEmpty(binding.SourceWarehouse, p.Execution.SourceWarehouse, "wip")
			m.TotalQty = 0
			m.AvailableQty, err = operationMaterialAvailableIdentityTx(ctx, tx, r.schema, m.MaterialID, m.Warehouse, m.OwnerCustomerID, m.Unit)
			if err != nil {
				return p, wo, err
			}
			m.ShortageQty = 0
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), m.Warehouse).Scan(&m.WarehouseName); err != nil {
				return p, wo, fmt.Errorf("来源仓不存在或已停用")
			}
			p.Materials = append(p.Materials, m)
		}
	}
	if isRoasting {
		for _, material := range p.Materials {
			p.RecipeRatioPct += material.RatioPct
		}
		if reason := app.IncompleteRoastingBOMReason(p.RecipeRatioPct); reason != "" {
			p.BlockedReason = reason
			p.BlockedCode = "bom_incomplete"
		} else {
			var resolved []app.OperationMaterialUse
			var e error
			if startPolicy {
				resolved, e = app.ResolveBOMStartMaterialUses(p.Materials, p.InputQty, p.InputUnit)
			} else {
				resolved, e = app.ResolveRoastingMaterialUses(p.Materials, p.InputQty, p.InputUnit)
			}
			if e != nil {
				p.BlockedReason = e.Error()
				p.BlockedCode = "bom_incomplete"
			} else {
				p.Materials = resolved
			}
		}
	}
	if startPolicy && seq == first {
		present := make(map[string]bool, len(p.Materials))
		for _, material := range p.Materials {
			present[material.Key] = true
		}
		for _, frozen := range frozenMaterials {
			kind, component := "material", frozen.MaterialID
			if frozen.ComponentType == "finished_product" || frozen.ComponentType == "product" {
				kind, component = "product", frozen.ComponentProductID
			}
			if !present[operationMaterialKey(kind, component, frozen.ComponentBomSpecID, frozen.ComponentSpecG)] {
				p.BlockedReason = "工单缺少冻结 BOM 投入：" + frozen.MaterialName
				p.BlockedCode = "bom_material_reservation_missing"
				break
			}
		}
	}
	if p.LegacyPackaging && len(p.Materials) == 0 {
		p.Enabled = false
		p.LegacyPackaging = false
		p.BlockedReason = "旧包装工单缺少完整冻结用料，不能自动接入"
		p.BlockedCode = "legacy_materials_missing"
	}
	for i := range p.Materials {
		if startPolicy {
			p.Materials[i].Warehouse, err = startedMaterialWarehouseTx(ctx, tx, r.schema, wo.ID, p.Materials[i])
			if err != nil {
				return p, wo, err
			}
		}
		if p.Materials[i].ReservationID > 0 {
			p.Materials[i].AvailableQty, err = operationMaterialAvailableTx(ctx, tx, r.schema, p.Materials[i].ReservationID, p.Materials[i].Warehouse, p.Materials[i].OwnerCustomerID, p.Materials[i].Unit)
		} else {
			p.Materials[i].AvailableQty, err = operationMaterialAvailableIdentityTx(ctx, tx, r.schema, p.Materials[i].MaterialID, p.Materials[i].Warehouse, p.Materials[i].OwnerCustomerID, p.Materials[i].Unit)
		}
		if err != nil {
			return p, wo, err
		}
		if startPolicy {
			p.Materials[i].EligibleBatches, err = startedMaterialBatchesTx(ctx, tx, r.schema, wo.ID, p.Materials[i])
			if err != nil {
				return p, wo, err
			}
			if p.Materials[i].ProductID > 0 {
				var hasBatchBindings bool
				if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.work_order_material_reservation_batches WHERE reservation_id=$1 AND status='reserved')`, r.schema), p.Materials[i].ReservationID).Scan(&hasBatchBindings); err != nil {
					return p, wo, err
				}
				if !hasBatchBindings {
					p.Materials[i].AvailableQty = 0
					for _, batch := range p.Materials[i].EligibleBatches {
						p.Materials[i].AvailableQty += batch.AvailableQty
					}
				}
			}
		}
		p.Materials[i].ShortageQty = math.Max(0, p.Materials[i].Qty-p.Materials[i].AvailableQty)
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.warehouses WHERE code=$1 AND active=true`, r.schema), p.Materials[i].Warehouse).Scan(&p.Materials[i].WarehouseName); err != nil {
			return p, wo, fmt.Errorf("来源仓不存在或已停用")
		}
	}
	if seq > first {
		p.PredecessorRequired = true
		suffix = ""
		if lock {
			suffix = " FOR UPDATE OF b"
		}
		predecessorSeq := 0
		if startPolicy {
			for _, op := range route.Operations {
				if op.Seq < seq && op.Seq > predecessorSeq {
					predecessorSeq = op.Seq
				}
			}
		}
		rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT b.id,b.work_order_id,b.job_card_id,b.owner_customer_id,b.remaining_qty::float8,b.unit,b.warehouse,wh.name,j.operation,b.remaining_cost::float8
 FROM %[1]s.production_operation_balances b JOIN %[1]s.job_cards j ON j.id=b.job_card_id JOIN %[1]s.warehouses wh ON wh.code=b.warehouse
 WHERE b.work_order_id=$1 AND b.sequence_no=CASE WHEN $3::int>0 THEN $3 ELSE (SELECT MAX(sequence_no) FROM %[1]s.job_cards WHERE work_order_id=$1 AND sequence_no<$2 AND status<>'cancelled') END AND b.remaining_qty>0 AND b.quality_status NOT IN('hold','reject') AND NOT EXISTS(SELECT 1 FROM (SELECT DISTINCT ON(q.job_card_id,q.scope,q.reference_type,q.reference_no) q.result FROM %[1]s.quality_inspections q WHERE (q.work_order_id=b.work_order_id AND q.job_card_id=0) OR q.job_card_id=b.job_card_id ORDER BY q.job_card_id,q.scope,q.reference_type,q.reference_no,q.created_at DESC,q.id DESC) latest WHERE lower(trim(result)) IN('hold','reject','failed','fail','blocked')) ORDER BY b.id`+suffix, r.schema), p.WorkOrderID, seq, predecessorSeq)
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
		if startPolicy {
			filtered := []app.OperationBatchBalance{}
			for _, b := range p.PredecessorBatches {
				var own, other float64
				if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_qty) FILTER(WHERE job_card_id=$2),0)::float8,COALESCE(SUM(remaining_qty) FILTER(WHERE job_card_id<>$2),0)::float8 FROM %s.production_started_handoffs WHERE source_balance_id=$1 AND status='posted'`, r.schema), b.ID, id).Scan(&own, &other); err != nil {
					return p, wo, err
				}
				available := math.Max(0, b.Remaining-other)
				if status == "running" || status == "paused" {
					available = math.Min(available, own)
					p.StartedInputQty += own
				}
				if available > 0 {
					b.TotalCost = b.TotalCost * available / b.Remaining
					b.Remaining = available
					filtered = append(filtered, b)
				}
			}
			p.PredecessorBatches = filtered
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
				p.BlockedCode = "predecessor_output_missing"
			}
		}
	}
	if p.Execution.ReportingType == "incoming" && p.PredecessorRequired {
		if predecessorErr := ensureJobCardPredecessorCompletedTx(ctx, tx, r.schema, p.WorkOrderID, seq); predecessorErr != nil {
			if !strings.HasPrefix(predecessorErr.Error(), "前序工序未完成") {
				return p, wo, predecessorErr
			}
			p.BlockedReason = predecessorErr.Error()
			p.BlockedCode = "predecessor_incomplete"
		}
	}
	if p.Execution.ReportingType == "incoming" && p.PredecessorRequired && p.BlockedCode != "predecessor_incomplete" && (status == "pending" || status == "ready" || status == "released") && len(p.PredecessorBatches) > 0 {
		allocations, allocErr := r.incomingAllocationsTx(ctx, tx, p, seq, lock)
		if allocErr != nil {
			return p, wo, allocErr
		}
		for _, a := range allocations {
			p.IncomingAllocations = append(p.IncomingAllocations, app.IncomingAllocation{JobCardID: a.JobCardID, Workstation: a.Workstation, Quantity: a.Quantity, Unit: a.Unit, Frozen: a.Frozen})
			if a.JobCardID == id {
				p.InputQty = a.Quantity
			}
		}
		if p.InputQty <= 0 {
			p.BlockedReason = "本工位实际来料分配为零，请取消该待开工任务"
			p.BlockedCode = "incoming_allocation_zero"
		}
	}
	if p.Execution.ReportingType == "incoming" && p.PredecessorRequired {
		if len(p.PredecessorBatches) == 0 && (status == "pending" || status == "ready" || status == "released") && p.BlockedReason == "" {
			p.BlockedReason = "前序无可用来料或来料已全部分配，请核对任务"
			p.BlockedCode = "incoming_allocation_empty"
		}
		if (status == "running" || status == "paused") && len(p.PredecessorBatches) > 0 {
			total := 0.0
			for _, b := range p.PredecessorBatches {
				factor := app.OperationUnitFactor(b.Unit)
				if factor <= 0 || app.OperationUnitFactor(p.InputUnit) <= 0 {
					return p, wo, fmt.Errorf("来料单位不可换算")
				}
				total += b.Remaining * factor / app.OperationUnitFactor(p.InputUnit)
			}
			p.StartedInputQty = total
			p.InputQty = total
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
	materialIDs := make([]int64, 0, len(p.Materials))
	for _, material := range p.Materials {
		if material.MaterialID > 0 {
			materialIDs = append(materialIDs, material.MaterialID)
		}
	}
	stockVersion, err := r.operationInventoryVersionForMaterialsTx(ctx, tx, p.WorkOrderID, materialIDs)
	if err != nil {
		return p, wo, err
	}
	raw, _ := json.Marshal([]any{version, stockVersion, p.EffectiveBomVersionID, p.Materials, p.PredecessorBatches, p.Execution, p.IncomingAllocations, p.InputQty})
	p.Version = fmt.Sprintf("%x", sha256.Sum256(raw))
	if strings.TrimSpace(status) == "completed" {
		p.BlockedReason = "本工序已完成，请从生产记录查看或更正"
		p.BlockedCode = "job_card_completed"
	}
	if strings.TrimSpace(status) == "cancelled" {
		p.BlockedReason = "任务已取消"
		p.BlockedCode = "job_card_cancelled"
	}
	if startPolicy && seq == first {
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_qty),0)::float8 FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted'`, r.schema), id).Scan(&p.StartedInputQty); err != nil {
			return p, wo, err
		}
		if status == "running" || status == "paused" {
			p.InputQty = p.StartedInputQty
			p.MaxOutputQty = p.PlannedQty
			inputRows, e := tx.Query(ctx, fmt.Sprintf(`SELECT material_snapshot FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted' ORDER BY id`, r.schema), id)
			if e != nil {
				return p, wo, e
			}
			for inputRows.Next() {
				var raw []byte
				if e = inputRows.Scan(&raw); e != nil {
					inputRows.Close()
					return p, wo, e
				}
				var materials []app.OperationMaterialUse
				if e = json.Unmarshal(raw, &materials); e != nil {
					inputRows.Close()
					return p, wo, e
				}
				p.StartedMaterials = append(p.StartedMaterials, materials...)
			}
			e = inputRows.Err()
			inputRows.Close()
			if e != nil {
				return p, wo, e
			}
			// Component-only WIP additions are independent audited events; keep
			// the original start snapshot immutable while displaying all issues.
			addRows, e := tx.Query(ctx, fmt.Sprintf(`SELECT payload->'materials' FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='material_add' AND status='posted' AND COALESCE((payload->>'component_only')::boolean,false) ORDER BY id`, r.schema), id)
			if e != nil {
				return p, wo, e
			}
			for addRows.Next() {
				var raw []byte
				if e = addRows.Scan(&raw); e != nil {
					addRows.Close()
					return p, wo, e
				}
				var materials []app.OperationMaterialUse
				if e = json.Unmarshal(raw, &materials); e != nil {
					addRows.Close()
					return p, wo, e
				}
				for _, added := range materials {
					found := false
					for i := range p.StartedMaterials {
						row := &p.StartedMaterials[i]
						if row.Key == added.Key && row.Unit == added.Unit && row.Warehouse == added.Warehouse && row.OwnerCustomerID == added.OwnerCustomerID && row.BomVariantID == added.BomVariantID {
							row.Qty = operationLedgerQuantity(row.Qty+added.Qty, row.Unit)
							found = true
							break
						}
					}
					if !found {
						p.StartedMaterials = append(p.StartedMaterials, added)
					}
				}
			}
			e = addRows.Err()
			addRows.Close()
			if e != nil {
				return p, wo, e
			}
		}
	}
	if p.MaterialConsumptionPolicy == "bom_start_v1" {
		p.StartedInputs, err = r.startedInputsTx(ctx, tx, id)
		if err != nil {
			return p, wo, err
		}
	}
	if p.Execution.ReportingType == "piece" && startPolicy && (status == "running" || status == "paused") {
		bean, e := app.ResolvePackagingWeightComponent(p.Materials)
		if e != nil || p.TotalPlannedQty <= 0 {
			p.BlockedCode = "packaging_material_invalid"
			p.BlockedReason = "请在冻结 BOM 中配置唯一可核对的重量半成品组件"
		} else {
			p.PackagingStandardGPerPiece = bean.TotalQty * app.OperationUnitFactor(bean.Unit) / p.TotalPlannedQty
			e = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(c.remaining_qty),0)::float8 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs b ON b.id=c.input_id WHERE b.job_card_id=$1 AND b.status='posted' AND c.component_key=$2 AND c.unit='g'`, r.schema), id, bean.Key).Scan(&p.PackagingAvailableG)
			if e != nil {
				return p, wo, e
			}
			p.PackagingUnissuedCandidates, e = r.packagingUnissuedCandidatesTx(ctx, tx, id, bean)
			if e != nil {
				return p, wo, e
			}
			versionWithCandidates, _ := json.Marshal([]any{p.Version, p.PackagingAvailableG, p.PackagingUnissuedCandidates})
			p.Version = fmt.Sprintf("%x", sha256.Sum256(versionWithCandidates))
		}
	}

	var replanRequired bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.job_cards WHERE work_order_id=$1 AND COALESCE((metrics_json->>'reporting_replan_required')::boolean,false))`, r.schema), wo.ID).Scan(&replanRequired); err != nil {
		return p, wo, err
	}
	if replanRequired {
		p.BlockedCode = "reporting_replan_required"
		p.BlockedReason = "工序报工类型已更新，请先重新安排工位产能"
	}
	if p.Execution.ReportingType == "piece" && p.MaterialConsumptionPolicy == "bom_start_v1" && wo.ProductionPlanID > 0 && (wo.ParentProductID > 0 || wo.ProductID > 0) {
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %[1]s.work_orders w JOIN %[1]s.production_plans pp ON pp.id=w.production_plan_id JOIN %[1]s.job_cards j ON j.work_order_id=w.id CROSS JOIN LATERAL jsonb_array_elements(COALESCE(w.process_snapshot_json->'operations','[]'::jsonb)) op WHERE w.id=$1 AND j.id=$2 AND w.production_plan_id>0 AND COALESCE(NULLIF(w.parent_product_id,0),w.product_id)>0 AND (op->>'seq')::int=j.sequence_no AND COALESCE(NULLIF(op->>'execution_json','')::jsonb->>'reporting_type','')='piece')`, r.schema), wo.ID, id).Scan(&p.PlanRemainderEligible); err != nil {
			return p, wo, err
		}
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

func operationMaterialAvailableIdentityTx(ctx context.Context, tx pgx.Tx, schema string, materialID int64, warehouse string, ownerCustomerID int64, unit string) (float64, error) {
	if materialID <= 0 {
		return 0, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,l.qty_g,l.qty_units,
		       COALESCE(bound.g,0)::bigint,COALESCE(bound.units,0)::bigint
		FROM %[1]s.material_batch_locations l
		JOIN %[1]s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)) g,
			       SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)) units
			FROM %[1]s.work_order_material_reservation_batches rb
			JOIN %[1]s.work_order_material_reservations reservation ON reservation.id=rb.reservation_id AND reservation.status='reserved'
			WHERE rb.material_batch_id=b.id AND rb.warehouse=l.warehouse AND rb.status='reserved'
		) bound ON true
		WHERE l.material_id=$1 AND l.warehouse=$2 AND COALESCE(b.owner_customer_id,0)=$3
		  AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN('hold','reject')
		  AND (l.qty_g>0 OR l.qty_units>0)
		ORDER BY b.received_at,b.id`, schema), materialID, warehouse, ownerCustomerID)
	if err != nil {
		return 0, err
	}
	type row struct{ id, g, units, reservedG, reservedUnits int64 }
	stock := []row{}
	for rows.Next() {
		var item row
		if err = rows.Scan(&item.id, &item.g, &item.units, &item.reservedG, &item.reservedUnits); err != nil {
			rows.Close()
			return 0, err
		}
		stock = append(stock, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	var availableG, availableUnits int64
	for _, item := range stock {
		processingG, processingUnits, e := stockrepo.OperationOtherProcessingReservedTx(ctx, tx, schema, 0, item.id, materialID, ownerCustomerID, warehouse, false)
		if e != nil {
			return 0, e
		}
		availableG += maxInt64(0, item.g-item.reservedG-processingG)
		availableUnits += maxInt64(0, item.units-item.reservedUnits-processingUnits)
	}
	var unboundG, unboundUnits int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g-COALESCE(bound.g,0))),0)::bigint,
		       COALESCE(SUM(GREATEST(0,reserved_units-consumed_units-returned_units-COALESCE(bound.units,0))),0)::bigint
		FROM %[1]s.work_order_material_reservations r
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,reserved_g-consumed_g-returned_g)) g,
			       SUM(GREATEST(0,reserved_units-consumed_units-returned_units)) units
			FROM %[1]s.work_order_material_reservation_batches b
			WHERE b.reservation_id=r.id AND b.status='reserved'
		) bound ON true
		WHERE r.material_id=$1 AND r.source_warehouse=$2 AND r.source_owner_customer_id=$3 AND r.status='reserved'
	`, schema), materialID, warehouse, ownerCustomerID).Scan(&unboundG, &unboundUnits); err != nil {
		return 0, err
	}
	availableG = maxInt64(0, availableG-unboundG)
	availableUnits = maxInt64(0, availableUnits-unboundUnits)
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		return float64(availableG) / factor, nil
	}
	return float64(availableUnits), nil
}

// operationMaterialAvailabilityPartsTx separates stock already reserved for
// one order from unreserved stock. Group previews sum their own reservations
// and count shared free inventory only once.
func operationMaterialAvailabilityPartsTx(ctx context.Context, tx pgx.Tx, schema string, reservationID int64, warehouse string, ownerCustomerID int64, unit string) (float64, float64, error) {
	var componentType string
	var workOrderID, materialID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT component_type,work_order_id,material_id FROM %s.work_order_material_reservations WHERE id=$1`, schema), reservationID).Scan(&componentType, &workOrderID, &materialID); err != nil {
		return 0, 0, err
	}
	if componentType == "product" {
		own, err := operationReservedProductAvailableTx(ctx, tx, schema, reservationID, warehouse, ownerCustomerID, unit)
		return own, 0, err
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
		return 0, 0, err
	}
	batches := []batchAvailability{}
	for rows.Next() {
		var row batchAvailability
		if err = rows.Scan(&row.id, &row.physicalG, &row.physicalUnits, &row.ownG, &row.ownUnits, &row.reservedG, &row.reservedUnits); err != nil {
			rows.Close()
			return 0, 0, err
		}
		batches = append(batches, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, err
	}
	var ownG, ownUnits, freeG, freeUnits int64
	for _, batch := range batches {
		processingG, processingUnits, e := stockrepo.OperationOtherProcessingReservedTx(ctx, tx, schema, workOrderID, batch.id, materialID, ownerCustomerID, warehouse, false)
		if e != nil {
			return 0, 0, e
		}
		otherReservedG := maxInt64(0, batch.reservedG-batch.ownG)
		otherReservedUnits := maxInt64(0, batch.reservedUnits-batch.ownUnits)
		ownG += minInt64(batch.ownG, maxInt64(0, batch.physicalG-otherReservedG-processingG))
		ownUnits += minInt64(batch.ownUnits, maxInt64(0, batch.physicalUnits-otherReservedUnits-processingUnits))
		freeG += maxInt64(0, batch.physicalG-batch.reservedG-processingG)
		freeUnits += maxInt64(0, batch.physicalUnits-batch.reservedUnits-processingUnits)
	}
	if factor := app.OperationUnitFactor(unit); factor > 0 {
		return float64(ownG) / factor, float64(freeG) / factor, nil
	}
	return float64(ownUnits), float64(freeUnits), nil
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

func roundedOperationWeightSlice(totalG int64, before, current, denominator float64) int64 {
	if totalG <= 0 {
		return 0
	}
	if denominator <= 0 || current <= 0 {
		return totalG
	}
	start := int64(math.Round(float64(totalG) * math.Max(0, before) / denominator))
	end := totalG
	if before+current < denominator-1e-9 {
		end = int64(math.Round(float64(totalG) * math.Min(denominator, math.Max(0, before)+current) / denominator))
	}
	return nonnegativeQuantity(end - start)
}
