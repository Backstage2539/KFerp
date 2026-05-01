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
	return out, rows.Err()
}

func (repo Repository) Finish(ctx context.Context, cmd productionapp.FinishCommand) error {
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
	r.BomYieldRate = normalizeYieldRate(r.BomYieldRate)
	if r.InputG <= 0 {
		r.InputG = defaultProductionInputG(r.NeedG, r.BomYieldRate)
	}

	var unitsBefore, looseBefore int64
	warehouse := strings.TrimSpace(cmd.Warehouse)
	if warehouse == "" {
		warehouse = "finished_goods"
	}
	_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=$1 AND spec_g=$2 AND warehouse=$3 FOR UPDATE`, schema), r.ProductID, r.SpecG, warehouse).Scan(&unitsBefore, &looseBefore)
	cur := InvQty{Units: unitsBefore, LooseG: looseBefore}
	add := runningInventoryPlan(r.SpecG, r.NeedG, r.InputG, r.BomYieldRate)
	if cmd.HasFinishedInput {
		add, err = normalizeFinishedInventoryAddition(r.SpecG, cmd.FinishedUnits, cmd.FinishedLooseG)
		if err != nil {
			return err
		}
	}
	if add.Units <= 0 && add.LooseG <= 0 {
		return fmt.Errorf("请填写完成件数或散装余量")
	}
	finishedTotal := finishedTotalG(r.SpecG, add.Units, add.LooseG)
	completionNo, err := nextCompletionNoForRunningItemTx(ctx, tx, schema, r.ID)
	if err != nil {
		return err
	}
	consumedInputG, partial, err := resolveFinishConsumedInput(r, cmd, finishedTotal)
	if err != nil {
		return err
	}
	actualYield, err := actualYieldRate(r.SpecG, add.Units, add.LooseG, consumedInputG)
	if err != nil {
		return err
	}
	nowQty := InvQty{Units: cur.Units + add.Units, LooseG: cur.LooseG + add.LooseG}
	norm, err := invNormalize(r.SpecG, nowQty)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g,updated_at=now()`, schema), r.ProductID, r.SpecG, warehouse, norm.Units, norm.LooseG); err != nil {
		return err
	}
	if err := recordFinishedProductStockMovementTx(ctx, tx, schema, r, cur, add, norm, finishedTotal, warehouse, operator); err != nil {
		return err
	}
	consumeRun := r
	consumeRun.InputG = consumedInputG
	if partial {
		consumeRun.NeedG = finishedTotal
		consumeRun.PlanUnits = add.Units
		consumeRun.PlanLooseG = add.LooseG
	}
	if err := deductMaterialsForRunningItemTx(ctx, tx, schema, consumeRun, add, operator); err != nil {
		return err
	}
	materialSummary, err := listMaterialConsumptionSummaryTx(ctx, tx, schema, r.ID)
	if err != nil {
		return err
	}
	materialSummaryJSON, err := marshalMaterialConsumptionSummary(materialSummary)
	if err != nil {
		return err
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
		return err
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
				return err
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET planned_g=$2 WHERE running_item_id=$1`, schema), id, remainingInputG); err != nil {
				return err
			}
			if err := tx.Commit(ctx); err != nil {
				return err
			}
			postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &id, "partial_finish", postgresinfra.StrPtr("material_consumption"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG, "input_g": consumedInputG, "remaining_need_g": remainingNeedG, "remaining_input_g": remainingInputG, "bom_yield_rate": r.BomYieldRate, "finished_units": add.Units, "finished_loose_g": add.LooseG, "finished_total_g": finishedTotal, "actual_yield_rate": actualYield})
			return nil
		}
	}
	totalFinishedG, err := cumulativeFinishedTotalGForRunningItemTx(ctx, tx, schema, r.ID)
	if err != nil {
		return err
	}
	actualCost, err := recordBatchCostForRunningItemTx(ctx, tx, schema, r, totalFinishedG)
	if err != nil {
		return err
	}
	if err := completeMaterialReservationsForRunningItemTx(ctx, tx, schema, r.ID); err != nil {
		return err
	}
	if err := completeWorkOrderForRunningItemTx(ctx, tx, schema, r.ID, actualCost, operator); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.produce_running_items SET status='done',finished_by=$2,finished_at=$3 WHERE id=$1`, schema), id, operator, finishedAt); err != nil {
		return err
	}
	for _, no := range splitOrderNos(r.OrderNos) {
		if err := completeOrderIfAllRunningDone(ctx, tx, schema, no); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	postgresinfra.AuditInsert(ctx, repo.pool, schema, operator, "produce_running", &id, "finish", postgresinfra.StrPtr("material_consumption"), nil, postgresinfra.StrPtr("deducted"), postgresinfra.AuditMeta{"running_item_id": id, "product_id": r.ProductID, "spec_g": r.SpecG, "need_g": r.NeedG, "input_g": consumedInputG, "bom_yield_rate": r.BomYieldRate, "finished_units": add.Units, "finished_loose_g": add.LooseG, "finished_total_g": finishedTotal, "actual_yield_rate": actualYield})
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
	restoredG, err := restoreRunningAllocationTx(ctx, tx, schema, r)
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

func restoreRunningAllocationTx(ctx context.Context, tx pgx.Tx, schema string, r ProduceRunRow) (int64, error) {
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

func completeOrderIfAllRunningDone(ctx context.Context, tx pgx.Tx, schema, orderNo string) error {
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
	statusID, err := lookupProcessStatusIDTx(ctx, tx, schema, "生产完成", "已生产完成")
	if err != nil {
		return err
	}
	if statusID <= 0 {
		return nil
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET process_status_id=$2 WHERE order_no=$1`, schema), orderNo, statusID)
	return err
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
