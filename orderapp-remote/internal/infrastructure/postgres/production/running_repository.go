package production

import (
	"context"
	"errors"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProduceRunRow struct {
	ID               int64
	BatchID          string
	Product          string
	ProductID        int64
	SpecG            int64
	NeedG            int64
	InputG           int64
	BomYieldRate     float64
	PlanUnits        int64
	PlanLooseG       int64
	OrderNos         string
	StartedBy        string
	StartedAt        string
	StartedAtTime    time.Time
	MaterialSnapshot string
	Outputs          []ProduceRunOutputRow
}

func listRunningItems(ctx context.Context, pool *pgxpool.Pool, schema string) ([]ProduceRunRow, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(material_snapshot,'[]'::jsonb)::text FROM %s.produce_running_items WHERE status='running' ORDER BY started_at DESC,id DESC`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProduceRunRow, 0)
	for rows.Next() {
		var r ProduceRunRow
		if err := rows.Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt, &r.MaterialSnapshot); err != nil {
			return nil, err
		}
		r.BomYieldRate = normalizeYieldRate(r.BomYieldRate)
		if r.InputG <= 0 {
			r.InputG = defaultProductionInputG(r.NeedG, r.BomYieldRate)
		}
		plan := runningInventoryPlan(r.SpecG, r.NeedG, r.InputG, r.BomYieldRate)
		r.PlanUnits = plan.Units
		r.PlanLooseG = plan.LooseG
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachRunningOutputs(ctx, pool, schema, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (repo Repository) Finish(ctx context.Context, cmd productionapp.FinishCommand) (productionapp.FinishResult, error) {
	id := cmd.ID
	schema := repo.schema
	operator := cmd.Operator
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	defer tx.Rollback(ctx)

	var r ProduceRunRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(material_snapshot,'[]'::jsonb)::text FROM %s.produce_running_items WHERE id=$1 AND status='running' FOR UPDATE`, schema), id).Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt, &r.MaterialSnapshot); err != nil {
		return productionapp.FinishResult{}, err
	}
	r.BomYieldRate = normalizeYieldRate(r.BomYieldRate)
	if r.InputG <= 0 {
		r.InputG = defaultProductionInputG(r.NeedG, r.BomYieldRate)
	}
	outputs, err := loadRunningOutputsForUpdateTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	if len(outputs) > 0 {
		return repo.finishRunningOutputs(ctx, tx, r, outputs, cmd)
	}

	var unitsBefore, looseBefore int64
	warehouse, err := finishWarehouseForRunningItemTx(ctx, tx, schema, r.ID, cmd.Warehouse)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 FOR UPDATE`, schema), r.ProductID, r.SpecG, warehouse).Scan(&unitsBefore, &looseBefore)
	cur := InvQty{Units: unitsBefore, LooseG: looseBefore}
	add := runningInventoryPlan(r.SpecG, r.NeedG, r.InputG, r.BomYieldRate)
	if cmd.HasFinishedInput {
		add, err = normalizeFinishedInventoryAddition(r.SpecG, cmd.FinishedUnits, cmd.FinishedLooseG)
		if err != nil {
			return productionapp.FinishResult{}, err
		}
	}
	if add.Units <= 0 && add.LooseG <= 0 {
		return productionapp.FinishResult{}, fmt.Errorf("请填写完成件数或散装余量")
	}
	finishedTotal := finishedTotalG(r.SpecG, add.Units, add.LooseG)
	completionNo, err := nextCompletionNoForRunningItemTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	consumedInputG, partial, err := resolveFinishConsumedInput(r, cmd, finishedTotal)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := validateFinishedOutputWithinConsumedInput(finishedTotal, consumedInputG); err != nil {
		return productionapp.FinishResult{}, err
	}
	actualYield, err := actualYieldRate(r.SpecG, add.Units, add.LooseG, consumedInputG)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	nowQty := InvQty{Units: cur.Units + add.Units, LooseG: cur.LooseG + add.LooseG}
	norm, err := invNormalize(r.SpecG, nowQty)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, warehouse, norm.Units, norm.LooseG); err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := recordFinishedProductStockMovementTx(ctx, tx, schema, r, cur, add, norm, finishedTotal, warehouse, operator); err != nil {
		return productionapp.FinishResult{}, err
	}
	consumeRun := r
	consumeRun.InputG = consumedInputG
	if partial {
		consumeRun.NeedG = finishedTotal
		consumeRun.PlanUnits = add.Units
		consumeRun.PlanLooseG = add.LooseG
	}
	if err := deductMaterialsForRunningItemTx(ctx, tx, schema, consumeRun, add, operator); err != nil {
		return productionapp.FinishResult{}, err
	}
	materialSummary, err := listMaterialConsumptionSummaryTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	materialSummaryJSON, err := marshalMaterialConsumptionSummary(materialSummary)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	finishedAt := time.Now()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_logs(
			running_item_id,completion_no,batch_id,product_id,product_name,spec_g,order_nos,
			planned_need_g,input_g,bom_yield_rate,
			finished_units,finished_loose_g,finished_total_g,actual_yield_rate,
			started_by,started_at,finished_by,finished_at,
			inventory_units_before,inventory_loose_g_before,
			inventory_units_after,inventory_loose_g_after,
			material_summary,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,now())
	`, schema),
		r.ID, completionNo, r.BatchID, r.ProductID, r.Product, r.SpecG, r.OrderNos,
		r.NeedG, consumedInputG, r.BomYieldRate,
		add.Units, add.LooseG, finishedTotal, actualYield,
		r.StartedBy, r.StartedAtTime, operator, finishedAt,
		unitsBefore, looseBefore, norm.Units, norm.LooseG,
		materialSummaryJSON,
	); err != nil {
		return productionapp.FinishResult{}, err
	}
	if partial {
		remainingNeedG := r.NeedG - finishedTotal
		remainingInputG := r.InputG - consumedInputG
		if remainingNeedG <= 0 || remainingInputG <= 0 {
			partial = false
		} else {
			remainingPlan := runningInventoryPlan(r.SpecG, remainingNeedG, remainingInputG, r.BomYieldRate)
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				UPDATE %s.produce_running_items
				SET need_g=$2,input_g=$3,planned_units=$4,planned_loose_g=$5
				WHERE id=$1
			`, schema), id, remainingNeedG, remainingInputG, remainingPlan.Units, remainingPlan.LooseG); err != nil {
				return productionapp.FinishResult{}, err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET planned_g=$2 WHERE running_item_id=$1`, schema), id, remainingInputG); err != nil {
				return productionapp.FinishResult{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return productionapp.FinishResult{}, err
			}
			postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &id, "partial_finish", postgresinfra.StrPtr("material_consumption"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG, "input_g": consumedInputG, "remaining_need_g": remainingNeedG, "remaining_input_g": remainingInputG, "bom_yield_rate": r.BomYieldRate, "finished_units": add.Units, "finished_loose_g": add.LooseG, "finished_total_g": finishedTotal, "actual_yield_rate": actualYield})
			return productionapp.FinishResult{RunningItemID: id}, nil
		}
	}
	totalFinishedG, err := cumulativeFinishedTotalGForRunningItemTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	actualCost, err := recordBatchCostForRunningItemTx(ctx, tx, schema, r, totalFinishedG)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := completeMaterialReservationsForRunningItemTx(ctx, tx, schema, r.ID); err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := completeWorkOrderForRunningItemTx(ctx, tx, schema, r.ID, actualCost, operator); err != nil {
		return productionapp.FinishResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, finishedAt); err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := markProcessingDemandsDoneTx(ctx, tx, schema, r.ID); err != nil {
		return productionapp.FinishResult{}, err
	}
	finishedOrders := make([]productionapp.FinishedOrder, 0)
	for _, no := range splitOrderNos(r.OrderNos) {
		order, changed, err := completeOrderIfAllRunningDone(ctx, tx, schema, no)
		if err != nil {
			return productionapp.FinishResult{}, err
		}
		if changed {
			finishedOrders = append(finishedOrders, order)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.FinishResult{}, err
	}
	postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &id, "finish", postgresinfra.StrPtr("material_consumption"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG, "input_g": consumedInputG, "bom_yield_rate": r.BomYieldRate, "finished_units": add.Units, "finished_loose_g": add.LooseG, "finished_total_g": finishedTotal, "actual_yield_rate": actualYield})
	return productionapp.FinishResult{RunningItemID: id, Completed: true, FinishedOrders: finishedOrders}, nil
}

func (repo Repository) finishRunningOutputs(ctx context.Context, tx pgx.Tx, r ProduceRunRow, outputs []ProduceRunOutputRow, cmd productionapp.FinishCommand) (productionapp.FinishResult, error) {
	if cmd.Partial {
		return productionapp.FinishResult{}, fmt.Errorf("合并多规格生产暂不支持部分完工")
	}
	schema := repo.schema
	operator := cmd.Operator
	warehouse, err := finishWarehouseForRunningItemTx(ctx, tx, schema, r.ID, cmd.Warehouse)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	finishedOutputs, totalFinishedG, err := normalizeFinishedOutputs(outputs, cmd.Outputs)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	consumedInputG := cmd.ConsumedInputG
	if consumedInputG <= 0 {
		consumedInputG = r.InputG
	}
	if consumedInputG <= 0 {
		return productionapp.FinishResult{}, fmt.Errorf("本次消耗投料必须大于0")
	}
	if err := validateFinishedOutputWithinConsumedInput(totalFinishedG, consumedInputG); err != nil {
		return productionapp.FinishResult{}, err
	}
	actualYield := math.Round((float64(totalFinishedG)/float64(consumedInputG))*10000) / 10000
	finishedAt := time.Now()
	completionNo, err := nextCompletionNoForRunningItemTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	type outputInventoryLog struct {
		unitsBefore int64
		looseBefore int64
		unitsAfter  int64
		looseAfter  int64
	}
	inventoryBySpec := map[int64]outputInventoryLog{}

	for _, output := range finishedOutputs {
		var unitsBefore, looseBefore int64
		_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 FOR UPDATE`, schema), output.ProductID, output.SpecG, warehouse).Scan(&unitsBefore, &looseBefore)
		cur := InvQty{Units: unitsBefore, LooseG: looseBefore}
		add := InvQty{Units: output.FinishedUnits, LooseG: output.FinishedLooseG}
		norm, err := invNormalize(output.SpecG, InvQty{Units: cur.Units + add.Units, LooseG: cur.LooseG + add.LooseG})
		if err != nil {
			return productionapp.FinishResult{}, err
		}
		finishedTotal := finishedTotalG(output.SpecG, add.Units, add.LooseG)
		if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), output.ProductID, output.SpecG, warehouse, norm.Units, norm.LooseG); err != nil {
			return productionapp.FinishResult{}, err
		}
		outputRun := r
		outputRun.ProductID = output.ProductID
		outputRun.Product = firstNonEmpty(output.Product, r.Product)
		outputRun.SpecG = output.SpecG
		outputRun.NeedG = output.NeedG
		outputRun.OrderNos = output.OrderNos
		if err := recordFinishedProductStockMovementWithBatchCodeTx(ctx, tx, schema, finishedProductionBatchCodeForSpec(r.ID, output.SpecG), outputRun, cur, add, norm, finishedTotal, warehouse, operator); err != nil {
			return productionapp.FinishResult{}, err
		}
		inventoryBySpec[output.SpecG] = outputInventoryLog{
			unitsBefore: unitsBefore,
			looseBefore: looseBefore,
			unitsAfter:  norm.Units,
			looseAfter:  norm.LooseG,
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_outputs SET finished_units=$2,finished_loose_g=$3 WHERE id=$1`, schema), output.ID, output.FinishedUnits, output.FinishedLooseG); err != nil {
			return productionapp.FinishResult{}, err
		}
	}

	r.Outputs = finishedOutputs
	if err := deductMaterialsForRunOutputsTx(ctx, tx, schema, r, finishedOutputs, operator); err != nil {
		return productionapp.FinishResult{}, err
	}
	materialSummary, err := listMaterialConsumptionSummaryTx(ctx, tx, schema, r.ID)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	materialSummaryJSON, err := marshalMaterialConsumptionSummary(materialSummary)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	for _, output := range finishedOutputs {
		finishedTotal := finishedTotalG(output.SpecG, output.FinishedUnits, output.FinishedLooseG)
		inventoryLog := inventoryBySpec[output.SpecG]
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_logs(
				running_item_id,completion_no,batch_id,product_id,product_name,spec_g,order_nos,
				planned_need_g,input_g,bom_yield_rate,
				finished_units,finished_loose_g,finished_total_g,actual_yield_rate,
				started_by,started_at,finished_by,finished_at,
				inventory_units_before,inventory_loose_g_before,
				inventory_units_after,inventory_loose_g_after,
				material_summary,created_at
			) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,now())
		`, schema),
			r.ID, completionNo, r.BatchID, output.ProductID, firstNonEmpty(output.Product, r.Product), output.SpecG, output.OrderNos,
			output.NeedG, consumedInputG, r.BomYieldRate,
			output.FinishedUnits, output.FinishedLooseG, finishedTotal, actualYield,
			r.StartedBy, r.StartedAtTime, operator, finishedAt,
			inventoryLog.unitsBefore, inventoryLog.looseBefore, inventoryLog.unitsAfter, inventoryLog.looseAfter,
			materialSummaryJSON,
		); err != nil {
			return productionapp.FinishResult{}, err
		}
	}
	actualCost, err := recordBatchCostForRunningItemTx(ctx, tx, schema, r, totalFinishedG)
	if err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := completeMaterialReservationsForRunningItemTx(ctx, tx, schema, r.ID); err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := completeWorkOrderForRunningItemTx(ctx, tx, schema, r.ID, actualCost, operator); err != nil {
		return productionapp.FinishResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), r.ID, operator, finishedAt); err != nil {
		return productionapp.FinishResult{}, err
	}
	if err := markProcessingDemandsDoneTx(ctx, tx, schema, r.ID); err != nil {
		return productionapp.FinishResult{}, err
	}
	finishedOrders := make([]productionapp.FinishedOrder, 0)
	for _, no := range splitOrderNos(r.OrderNos) {
		order, changed, err := completeOrderIfAllRunningDone(ctx, tx, schema, no)
		if err != nil {
			return productionapp.FinishResult{}, err
		}
		if changed {
			finishedOrders = append(finishedOrders, order)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.FinishResult{}, err
	}
	postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &r.ID, "finish", postgresinfra.StrPtr("material_consumption"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"running_item_id": r.ID, "product_id": r.ProductID, "spec_g": 0, "need_g": r.NeedG, "input_g": consumedInputG, "bom_yield_rate": r.BomYieldRate, "finished_total_g": totalFinishedG, "actual_yield_rate": actualYield})
	return productionapp.FinishResult{RunningItemID: r.ID, Completed: true, FinishedOrders: finishedOrders}, nil
}

func normalizeFinishedOutputs(outputs []ProduceRunOutputRow, commands []productionapp.FinishOutputCommand) ([]ProduceRunOutputRow, int64, error) {
	bySpec := map[int64]productionapp.FinishOutputCommand{}
	for _, cmd := range commands {
		if cmd.SpecG > 0 {
			bySpec[cmd.SpecG] = cmd
		}
	}
	out := make([]ProduceRunOutputRow, 0, len(outputs))
	total := int64(0)
	for _, output := range outputs {
		if output.SpecG <= 0 {
			continue
		}
		units := output.PlanUnits
		looseG := output.PlanLooseG
		if cmd, ok := bySpec[output.SpecG]; ok {
			units = cmd.FinishedUnits
			looseG = cmd.FinishedLooseG
		}
		norm, err := normalizeFinishedInventoryAddition(output.SpecG, units, looseG)
		if err != nil {
			return nil, 0, err
		}
		finishedTotal := finishedTotalG(output.SpecG, norm.Units, norm.LooseG)
		if finishedTotal <= 0 {
			return nil, 0, fmt.Errorf("请填写 %dg 完成件数或散装余量", output.SpecG)
		}
		output.FinishedUnits = norm.Units
		output.FinishedLooseG = norm.LooseG
		total += finishedTotal
		out = append(out, output)
	}
	if total <= 0 {
		return nil, 0, fmt.Errorf("请填写完成件数或散装余量")
	}
	return out, total, nil
}

func validateFinishedOutputWithinConsumedInput(finishedTotalG, consumedInputG int64) error {
	if finishedTotalG > consumedInputG {
		return fmt.Errorf("finished output cannot exceed consumed input")
	}
	return nil
}

func resolveFinishConsumedInput(r ProduceRunRow, cmd productionapp.FinishCommand, finishedTotal int64) (int64, bool, error) {
	partial := cmd.Partial && finishedTotal < r.NeedG
	if !partial {
		if cmd.ConsumedInputG > 0 {
			return cmd.ConsumedInputG, false, nil
		}
		if r.InputG > 0 {
			return r.InputG, false, nil
		}
		return 0, false, fmt.Errorf("投料数必须大于0")
	}

	consumedInputG := cmd.ConsumedInputG
	if consumedInputG <= 0 {
		consumedInputG = int64(math.Ceil(float64(finishedTotal) / r.BomYieldRate))
	}
	if consumedInputG <= 0 {
		return 0, false, fmt.Errorf("本次消耗投料必须大于0")
	}
	if consumedInputG > r.InputG {
		return 0, false, fmt.Errorf("本次消耗投料不能大于工单剩余投料")
	}
	return consumedInputG, true, nil
}

func nextCompletionNoForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) (int64, error) {
	var completionNo int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(MAX(completion_no),0)+1 FROM %s.production_logs WHERE running_item_id=$1`, schema), runningItemID).Scan(&completionNo)
	if err != nil {
		return 0, err
	}
	if completionNo <= 0 {
		completionNo = 1
	}
	return completionNo, nil
}

func cumulativeFinishedTotalGForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) (int64, error) {
	var totalG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(finished_total_g),0)::bigint FROM %s.production_logs WHERE running_item_id=$1`, schema), runningItemID).Scan(&totalG)
	if err != nil {
		return 0, err
	}
	return totalG, nil
}

func finishWarehouseForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64, requested string) (string, error) {
	warehouse := strings.TrimSpace(requested)
	if warehouse != "" {
		return warehouse, nil
	}
	warehouse, err := processingDemandTargetWarehouseForRunningItemTx(ctx, tx, schema, runningItemID)
	if err != nil {
		return "", err
	}
	if warehouse != "" {
		return warehouse, nil
	}
	return "finished_goods", nil
}

func processingDemandTargetWarehouseForRunningItemTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) (string, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT COALESCE(target_warehouse,'')
		FROM %s.customer_processing_production_demands
		WHERE linked_running_item_id=$1
		  AND status='running'
	`, schema), runningItemID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	warehouses := make([]string, 0, 1)
	for rows.Next() {
		var warehouse string
		if err := rows.Scan(&warehouse); err != nil {
			return "", err
		}
		warehouse = strings.TrimSpace(warehouse)
		if warehouse == "" {
			continue
		}
		warehouses = append(warehouses, warehouse)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(warehouses) != 1 {
		return "", nil
	}
	return warehouses[0], nil
}

func markProcessingDemandsDoneTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_production_demands
		SET status='done',
		    updated_at=now()
		WHERE linked_running_item_id=$1
		  AND status='running'
	`, schema), runningItemID)
	return err
}

func markProcessingDemandsPlannedTx(ctx context.Context, tx pgx.Tx, schema string, runningItemID int64) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.customer_processing_production_demands
		SET status='planned',
		    linked_batch_id='',
		    linked_running_item_id=0,
		    linked_work_order_id=0,
		    updated_at=now()
		WHERE linked_running_item_id=$1
		  AND status='running'
	`, schema), runningItemID)
	return err
}

func (repo Repository) Cancel(ctx context.Context, cmd productionapp.CancelCommand) error {
	id := cmd.ID
	schema := repo.schema
	operator := cmd.Operator
	tx, err := repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var r ProduceRunRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,batch_id,product_name,product_id,spec_g,need_g,COALESCE(input_g,0),COALESCE(bom_yield_rate,0.8),COALESCE(planned_units,0),COALESCE(planned_loose_g,0),order_nos,COALESCE(started_by,''),started_at,to_char(started_at,'YYYY-MM-DD HH24:MI'),COALESCE(material_snapshot,'[]'::jsonb)::text FROM %s.produce_running_items WHERE id=$1 AND status='running' FOR UPDATE`, schema), id).Scan(&r.ID, &r.BatchID, &r.Product, &r.ProductID, &r.SpecG, &r.NeedG, &r.InputG, &r.BomYieldRate, &r.PlanUnits, &r.PlanLooseG, &r.OrderNos, &r.StartedBy, &r.StartedAtTime, &r.StartedAt, &r.MaterialSnapshot); err != nil {
		return err
	}
	outputs, err := loadRunningOutputsForUpdateTx(ctx, tx, schema, r.ID)
	if err != nil {
		return err
	}
	restoredG, err := restoreRunningAllocationTx(ctx, tx, schema, r, outputs)
	if err != nil {
		return err
	}
	cancelledAt := time.Now()
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='cancelled',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, cancelledAt); err != nil {
		return err
	}
	if err := cancelWorkOrderForRunningItemTx(ctx, tx, schema, r.ID, operator); err != nil {
		return err
	}
	if err := markProcessingDemandsPlannedTx(ctx, tx, schema, r.ID); err != nil {
		return err
	}
	for _, no := range splitOrderNos(r.OrderNos) {
		if err := resetOrderIfNoRunningItemsTx(ctx, tx, schema, no); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &id, "cancel", postgresinfra.StrPtr("finished_allocation"), postgresinfra.StrPtr(fmt.Sprintf("%d", restoredG)), postgresinfra.StrPtr("restored"), postgresinfra.AuditMeta{"running_item_id": id, "batch_id": r.BatchID, "product_id": r.ProductID, "spec_g": r.SpecG, "restored_g": restoredG})
	return nil
}

func restoreRunningAllocationTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, outputs []ProduceRunOutputRow) (int64, error) {
	if len(outputs) > 0 {
		return restoreRunningOutputAllocationsTx(ctx, tx, schema, r, outputs)
	}
	var deductedG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT deducted_g FROM %s.finished_allocation_logs WHERE batch_id=$1 AND product_id=$2 AND spec_g=$3 ORDER BY id DESC LIMIT 1`, schema), r.BatchID, r.ProductID, r.SpecG).Scan(&deductedG)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	if deductedG <= 0 {
		return 0, nil
	}
	var units, loose int64
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods' FOR UPDATE`, schema), r.ProductID, r.SpecG).Scan(&units, &loose)
	norm, err := restoreAllocatedInventory(r.SpecG, InvQty{Units: units, LooseG: loose}, deductedG)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,'finished_goods',$3,$4,now()) ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, norm.Units, norm.LooseG); err != nil {
		return 0, err
	}
	return deductedG, nil
}

func restoreRunningOutputAllocationsTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow, outputs []ProduceRunOutputRow) (int64, error) {
	totalRestoredG := int64(0)
	for _, output := range outputs {
		if output.ProductID <= 0 || output.SpecG <= 0 {
			continue
		}
		var deductedG int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT deducted_g
			FROM %s.finished_allocation_logs
			WHERE batch_id=$1 AND product_id=$2 AND spec_g=$3
			ORDER BY id DESC
			LIMIT 1
		`, schema), r.BatchID, output.ProductID, output.SpecG).Scan(&deductedG)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return 0, err
		}
		if deductedG <= 0 {
			continue
		}
		var units, loose int64
		_ = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g
			FROM %s.finished_inventory
			WHERE product_id=$1 AND spec_g=$2 AND warehouse='finished_goods'
			FOR UPDATE
		`, schema), output.ProductID, output.SpecG).Scan(&units, &loose)
		norm, err := restoreAllocatedInventory(output.SpecG, InvQty{Units: units, LooseG: loose}, deductedG)
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at)
			VALUES($1,$2,'finished_goods',$3,$4,now())
			ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
			SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()
		`, schema), output.ProductID, output.SpecG, norm.Units, norm.LooseG); err != nil {
			return 0, err
		}
		totalRestoredG += deductedG
	}
	return totalRestoredG, nil
}

func splitOrderNos(v string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, x := range strings.Split(strings.TrimSpace(v), ",") {
		no := strings.TrimSpace(x)
		if no == "" || seen[no] {
			continue
		}
		seen[no] = true
		out = append(out, no)
	}
	return out
}

func completeOrderIfAllRunningDone(ctx context.Context, tx pgx.Tx, schema, orderNo string) (productionapp.FinishedOrder, bool, error) {
	if strings.TrimSpace(orderNo) == "" {
		return productionapp.FinishedOrder{}, false, nil
	}
	var hasRunning bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.produce_running_items WHERE status='running' AND $1 = ANY(string_to_array(replace(order_nos,' ',''),',')))`, schema), orderNo).Scan(&hasRunning); err != nil {
		return productionapp.FinishedOrder{}, false, err
	}
	if hasRunning {
		return productionapp.FinishedOrder{}, false, nil
	}
	hasGap, err := orderHasRemainingProductionGapTx(ctx, tx, schema, orderNo)
	if err != nil {
		return productionapp.FinishedOrder{}, false, err
	}
	if hasGap {
		return productionapp.FinishedOrder{}, false, nil
	}
	statusID, err := lookupProcessStatusIDTx(ctx, tx, schema, "生产完成", "已生产完成")
	if err != nil {
		return productionapp.FinishedOrder{}, false, err
	}
	if statusID <= 0 {
		return productionapp.FinishedOrder{}, false, nil
	}
	var order productionapp.FinishedOrder
	err = tx.QueryRow(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1 RETURNING id, order_no`, schema), orderNo, statusID).Scan(&order.OrderID, &order.OrderNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return productionapp.FinishedOrder{}, false, nil
	}
	if err != nil {
		return productionapp.FinishedOrder{}, false, err
	}
	return order, true, nil
}

func orderHasRemainingProductionGapTx(ctx context.Context, tx pgx.Tx, schema, orderNo string) (bool, error) {
	q := fmt.Sprintf(`
		WITH target_order AS (
			SELECT id
			FROM %[1]s.orders
			WHERE order_no=$1
			  AND is_void=false
			LIMIT 1
		),
		need AS (
			SELECT
				oi.product_id,
				spec.spec_g,
				(SUM(COALESCE(oi.qty,0))::bigint * spec.spec_g)::bigint AS need_g,
				BOOL_OR(COALESCE(osd.decision,'') = 'produce') AS force_produce
			FROM %[1]s.order_items oi
			JOIN target_order o ON o.id=oi.order_id
			LEFT JOIN %[1]s.order_stock_decisions osd ON osd.order_id=o.id
			CROSS JOIN LATERAL (
				SELECT COALESCE(NULLIF(regexp_replace(COALESCE(oi.spec,''), '[^0-9]', '', 'g'), ''), '0')::bigint AS spec_g
			) spec
			WHERE COALESCE(oi.product_id,0) > 0
			  AND spec.spec_g > 0
			  AND COALESCE(oi.qty,0) > 0
			GROUP BY oi.product_id, spec.spec_g
		),
		produced AS (
			SELECT
				pl.product_id,
				pl.spec_g,
				SUM(COALESCE(pl.finished_total_g,0))::bigint AS produced_g
			FROM %[1]s.production_logs pl
			WHERE $1 = ANY(string_to_array(replace(COALESCE(pl.order_nos,''),' ',''), ','))
			GROUP BY pl.product_id, pl.spec_g
		),
		reserved AS (
			SELECT
				a.product_id,
				a.spec_g,
				SUM(COALESCE(a.allocated_g,0))::bigint AS reserved_g
			FROM %[1]s.order_stock_batch_allocations a
			WHERE NOT EXISTS (
				SELECT 1
				FROM %[1]s.order_stock_deductions d
				WHERE d.order_id=a.order_id
				  AND d.product_id=a.product_id
				  AND d.spec_g=a.spec_g
				  AND d.batch_code=a.batch_code
			)
			GROUP BY a.product_id, a.spec_g
		)
		SELECT EXISTS (
			SELECT 1
			FROM need n
			LEFT JOIN produced p ON p.product_id=n.product_id AND p.spec_g=n.spec_g
			LEFT JOIN %[1]s.finished_inventory fi ON fi.product_id=n.product_id AND fi.spec_g=n.spec_g AND fi.warehouse='finished_goods'
			LEFT JOIN reserved r ON r.product_id=n.product_id AND r.spec_g=n.spec_g
			WHERE CASE
				WHEN n.force_produce THEN COALESCE(p.produced_g,0) < n.need_g
				ELSE GREATEST(
					COALESCE(p.produced_g,0),
					GREATEST(
						0,
						(COALESCE(fi.onhand_units,0) * n.spec_g + COALESCE(fi.onhand_loose_g,0)) - COALESCE(r.reserved_g,0)
					)
				) < n.need_g
			END
		)
	`, schema)
	var hasGap bool
	if err := tx.QueryRow(ctx, q, strings.TrimSpace(orderNo)).Scan(&hasGap); err != nil {
		return false, err
	}
	return hasGap, nil
}

func resetOrderIfNoRunningItemsTx(ctx context.Context, tx pgx.Tx, schema, orderNo string) error {
	if strings.TrimSpace(orderNo) == "" {
		return nil
	}
	var hasRunning bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s.produce_running_items WHERE status='running' AND $1 = ANY(string_to_array(replace(order_nos,' ',''),',')))`, schema), orderNo).Scan(&hasRunning); err != nil {
		return err
	}
	if hasRunning {
		return nil
	}
	_, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=NULL WHERE order_no=$1`, schema), orderNo)
	return err
}

func lookupProcessStatusIDTx(ctx context.Context, tx pgx.Tx, schema string, names ...string) (int64, error) {
	for _, name := range names {
		var id int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.order_process_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), name).Scan(&id)
		if err == nil && id > 0 {
			return id, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return 0, err
		}
	}
	return 0, nil
}

func setOrdersProcessStatusByNeeds(ctx context.Context, pool *pgxpool.Pool, schema string, rows []UnprodNeedRow, statusName string) error {
	statusID := int64(0)
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(id,0) FROM %s.order_process_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), statusName).Scan(&statusID)
	if statusID <= 0 {
		return nil
	}
	nos := map[string]bool{}
	for _, r := range rows {
		for _, x := range strings.Split(strings.TrimSpace(r.OrderNos), ",") {
			x = strings.TrimSpace(x)
			if x != "" {
				nos[x] = true
			}
		}
	}
	for no := range nos {
		_, _ = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1`, schema), no, statusID)
	}
	return nil
}

func setOrdersProcessStatusByNeedsTx(ctx context.Context, tx pgx.Tx, schema string, rows []UnprodNeedRow, statusName string) error {
	statusID := int64(0)
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(id,0) FROM %s.order_process_statuses WHERE name=$1 ORDER BY id LIMIT 1`, schema), statusName).Scan(&statusID)
	if statusID <= 0 {
		return nil
	}
	nos := map[string]bool{}
	for _, r := range rows {
		for _, x := range strings.Split(strings.TrimSpace(r.OrderNos), ",") {
			x = strings.TrimSpace(x)
			if x != "" {
				nos[x] = true
			}
		}
	}
	for no := range nos {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1`, schema), no, statusID); err != nil {
			return err
		}
	}
	return nil
}
