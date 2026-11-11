package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	app "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"testing"
)

func TestFlexibleCapacityAPICustomRoundTripSubmitAndWorkOrderEdit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "FLEX-RAW", "计划生豆", 100000)
	repo := postgresproduction.NewRepository(pool, schema)
	plan, err := repo.CreateProductionPlan(ctx, app.CreateProductionPlanCommand{Selected: map[string]bool{"1-227": true}, InputByKey: map[string]int64{"1-227": 45113}, Operator: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	e := newProductionFlowTestEcho(pool, schema)
	selectPlanningTestSources(t, e, plan.ID)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'自定义测试','active',102);
INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes) VALUES(719,719,'15kg','active',15,'kg',18);
INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'烘焙','active'),(720,'包装','active');
INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719),(719,720);`, schema))
	rows := func(custom any, count int) []map[string]any {
		return []map[string]any{
			{"production_plan_item_id": plan.Items[0].ID, "operation_seq": 1, "operation": "烘焙", "workstation_capacity_id": 719, "custom_batch_size_qty": custom, "planned_batch_count": count},
			{"production_plan_item_id": plan.Items[0].ID, "operation_seq": 2, "operation": "包装", "workstation_capacity_id": 719, "planned_batch_count": 3},
		}
	}
	for _, size := range []float64{18.1, 18.5, 18.7} {
		for _, count := range []int{2, 3} {
			rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits/preview", plan.ID), map[string]any{"items": rows(size, count)})
			var preview app.ProductionPlanOperationSplitPreview
			if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil || len(preview.OperationCoverage) != 2 || preview.OperationCoverage[0].ArrangedG != int64(math.Round(size*float64(count)*1000)) {
				t.Fatalf("custom %v x %d: %d %s", size, count, rec.Code, rec.Body.String())
			}
			if len(preview.Warnings) == 0 {
				t.Fatalf("missing variance warnings: %s", rec.Body.String())
			}
		}
	}
	for _, invalid := range []any{0, -1, 0.00001, 18.1234} {
		rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": rows(invalid, 3)})
		if rec.Code != 400 {
			t.Fatalf("invalid custom %v accepted: %d %s", invalid, rec.Code, rec.Body.String())
		}
	}
	// Save an explicitly chosen 15 kg x 3, leaving the 0.113 kg gap visible.
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": rows(15, 3)})
	if rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	saved, err := repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Readiness.CanSubmit || saved.Readiness.WarningCount == 0 {
		t.Fatalf("short plan not ready: %+v", saved.Readiness)
	}
	raw, _ := json.Marshal(saved.OperationSplits[0])
	if saved.OperationSplits[0].PlannedQtyG != 45000 || !strings.Contains(string(raw), `"custom_batch_size_qty":15`) {
		t.Fatalf("custom persistence: %s", raw)
	}
	// Atomic draft saving must preserve the explicit marker too.
	rec = serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d/draft", plan.ID), map[string]any{"draft_token": saved.DraftToken, "operation_splits": rows(15, 3)})
	if rec.Code != 200 {
		t.Fatalf("draft: %d %s", rec.Code, rec.Body.String())
	}
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	var submitted app.ProductionPlanSubmitResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &submitted) != nil || len(submitted.WorkOrders) != 1 || len(submitted.JobCards) != 2 {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body.String())
	}
	card := submitted.JobCards[0]
	raw, _ = json.Marshal(card)
	if card.PlannedInputQty != 45000 || card.BatchSizeQty != 15 || card.PlannedBatchCount != 3 || !strings.Contains(string(raw), `"custom_batch_size_qty":15`) {
		t.Fatalf("card snapshot: %s", raw)
	}
	workID := submitted.WorkOrders[0].ID
	if !strings.Contains(submitted.WorkOrders[0].OperationSummaryJSON, `"custom_batch_size_qty":15`) {
		t.Fatalf("summary dropped custom: %s", submitted.WorkOrders[0].OperationSummaryJSON)
	}
	if !strings.Contains(submitted.WorkOrders[0].OperationSummaryJSON, `"standard_minutes":18`) || !strings.Contains(submitted.WorkOrders[0].OperationSummaryJSON, `"scheduling_mode":"batch"`) {
		t.Fatalf("summary lost frozen batch timing/mode: %s", submitted.WorkOrders[0].OperationSummaryJSON)
	}
	// Changing the catalog cannot erase a custom batch size on an unstarted work order.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET batch_size_qty=12 WHERE id=719`, schema))
	for _, count := range []int{2, 3} {
		rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/work-orders/%d/operation-splits", workID), map[string]any{"items": rows(18.7, count)})
		var changed app.WorkOrderOperationSplitsResult
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &changed) != nil || len(changed.JobCards) != 2 {
			t.Fatalf("work order: %d %s", rec.Code, rec.Body.String())
		}
		card = changed.JobCards[0]
		if card.BatchSizeQty != 18.7 || card.PlannedInputQty != math.Round(18700*float64(count)) || card.PlannedBatchCount != count || !strings.Contains(changed.WorkOrder.OperationSummaryJSON, `"custom_batch_size_qty":18.7`) {
			t.Fatalf("work-order custom snapshot: %+v", changed)
		}
	}
	var custom, effective float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT custom_batch_size_qty,batch_size_qty FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=1`, schema), workID).Scan(&custom, &effective); err != nil || custom != 18.7 || effective != 18.7 {
		t.Fatalf("job card persistence %v %v: %v", custom, effective, err)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='save_operation_splits'", workID), 2)
	for _, path := range []string{fmt.Sprintf("/api/produce/work-orders/%d", workID), "/api/produce/job-cards"} {
		rec = serveMultilevelProductionJSON(t, e, http.MethodGet, path, nil)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"custom_batch_size_qty":18.7`) {
			t.Fatalf("read path lost custom marker: %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	// Four independently arranged cards must each own one physical execution
	// batch, rather than multiplying the four cards by four a second time.
	manual := []map[string]any{}
	for i := 0; i < 4; i++ {
		manual = append(manual, rows(15, 1)[0])
	}
	manual = append(manual, rows(15, 1)[1])
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/work-orders/%d/operation-splits", workID), map[string]any{"items": manual})
	if rec.Code != 200 {
		t.Fatalf("manual split: %s", rec.Body.String())
	}
	var taskCount, batchCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(DISTINCT j.id),count(b.id) FROM %[1]s.job_cards j LEFT JOIN %[1]s.production_execution_batches b ON b.job_card_id=j.id WHERE j.work_order_id=$1 AND j.sequence_no=1`, schema), workID).Scan(&taskCount, &batchCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 4 || batchCount != 4 {
		t.Fatalf("manual cards=%d execution batches=%d", taskCount, batchCount)
	}

}

func TestFlexibleCapacityAPIQuantityModesAllowUnderAndOver(t *testing.T) {
	for _, tc := range []struct {
		reporting, unit string
		quantities      []float64
	}{{"incoming", "kg", []float64{45, 56.1}}, {"piece", "件", []float64{1, 3}}} {
		t.Run(tc.reporting, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			seedProductionPlanLifecycleData(t, ctx, pool, schema)
			seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "FLEX-QUANTITY-RAW", "计划生豆", 100000)
			repo := postgresproduction.NewRepository(pool, schema)
			plan, err := repo.CreateProductionPlan(ctx, app.CreateProductionPlanCommand{Selected: map[string]bool{"1-227": true}, InputByKey: map[string]int64{"1-227": 45113}, Operator: "tester"})
			if err != nil {
				t.Fatal(err)
			}
			e := newProductionFlowTestEcho(pool, schema)
			selectPlanningTestSources(t, e, plan.ID)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'数量测试','active',102);
INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes,scheduling_mode) VALUES(719,719,'按量','active',0,'%[2]s',0,'quantity');
INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'加工','active');
INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719);
UPDATE %[1]s.production_plan_items SET process_route_snapshot_json='{"operations":[{"seq":1,"operation":"加工","execution_json":"{\"reporting_type\":\"%[3]s\"}"}]}'::jsonb WHERE production_plan_id=%[4]d;
`, schema, tc.unit, tc.reporting, plan.ID))
			for _, qty := range tc.quantities {
				row := map[string]any{"production_plan_item_id": plan.Items[0].ID, "operation_seq": 1, "operation": "加工", "workstation_capacity_id": 719, "planned_qty": qty}
				for _, suffix := range []string{"/operation-splits/preview", "/operation-splits"} {
					rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d%s", plan.ID, suffix), map[string]any{"items": []any{row}})
					if rec.Code != 200 {
						t.Fatalf("%v %s: %d %s", qty, suffix, rec.Code, rec.Body.String())
					}
				}
				saved, err := repo.GetProductionPlan(ctx, plan.ID)
				if err != nil {
					t.Fatal(err)
				}
				if !saved.Readiness.CanSubmit || saved.Readiness.WarningCount != 1 || saved.OperationSplits[0].PlannedQty != qty {
					t.Fatalf("quantity mode ready/persistence: %+v %+v", saved.Readiness, saved.OperationSplits)
				}
				rec := serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d/draft", plan.ID), map[string]any{"draft_token": saved.DraftToken, "operation_splits": []any{row}})
				if rec.Code != 200 {
					t.Fatalf("draft %d %s", rec.Code, rec.Body.String())
				}
				row["custom_batch_size_qty"] = 18.7
				rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": []any{row}})
				if rec.Code != 400 {
					t.Fatalf("custom accepted for quantity mode: %d %s", rec.Code, rec.Body.String())
				}
			}
			rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
			if rec.Code != 200 {
				t.Fatalf("submit over quantity %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
