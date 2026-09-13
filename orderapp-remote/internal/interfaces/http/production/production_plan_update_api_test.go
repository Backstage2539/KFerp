package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestProductionPlanDraftCanRecalculateInPlace(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 90010, 10, "RECALCULATE-WIP", "计划生豆", 10000)
	app := newProductionFlowTestEcho(pool, schema)

	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"source_type": "erp_order", "selected": []string{"1-227"}, "request_id": "create-plan",
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var before productionapp.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	if before.Revision != 1 {
		t.Fatalf("initial revision=%d want=1", before.Revision)
	}
	selectPlanningTestSources(t, app, before.ID)
	selectedDetail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", before.ID), nil)
	if err := json.Unmarshal(selectedDetail.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	selectedSourceCount := 0
	for _, source := range before.ComponentSources {
		if source.Selected {
			selectedSourceCount++
		}
	}
	if selectedSourceCount == 0 {
		t.Fatal("test fixture must select at least one compatible component source")
	}
	seedProductionPlanLifecycleOperationSplits(t, ctx, pool, schema, before)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_items SET target_warehouse='wip' WHERE production_plan_id=%d`, schema, before.ID))

	updated := serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", before.ID), map[string]any{
		"selected": []string{"1-227"}, "request_id": "update-plan", "revision": before.Revision,
	})
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	var after productionapp.ProductionPlanDetail
	if err := json.Unmarshal(updated.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.PlanNo != before.PlanNo || after.Revision != 2 || len(after.Items) == 0 {
		t.Fatalf("updated plan=%+v before=%+v", after, before)
	}
	if len(after.OperationSplits) != 2 || after.Items[0].TargetWarehouse != "wip" {
		t.Fatalf("unchanged draft arrangements were not preserved: items=%+v splits=%+v", after.Items, after.OperationSplits)
	}
	preservedSourceCount := 0
	for _, source := range after.ComponentSources {
		if source.Selected {
			preservedSourceCount++
		}
	}
	if preservedSourceCount != selectedSourceCount {
		t.Fatalf("compatible source selections=%d want=%d: %+v", preservedSourceCount, selectedSourceCount, after.ComponentSources)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='recalculate'", before.ID), 1)

	retry := serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", before.ID), map[string]any{
		"selected": []string{"1-227"}, "request_id": "update-plan", "revision": before.Revision,
	})
	if retry.Code != http.StatusOK {
		t.Fatalf("idempotent retry status=%d body=%s", retry.Code, retry.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='recalculate'", before.ID), 1)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES (2,'SO-PLAN-2','2026-06-11',false,(SELECT id FROM %s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total)
		VALUES (2,1,'计划拼配',1,'袋','227g',1,50,50);
	`, schema, schema, schema))
	changed := serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", before.ID), map[string]any{
		"selected": []string{"1-227"}, "request_id": "changed-update", "revision": after.Revision,
	})
	if changed.Code != http.StatusOK {
		t.Fatalf("changed update status=%d body=%s", changed.Code, changed.Body.String())
	}
	var changedPlan productionapp.ProductionPlanDetail
	if err := json.Unmarshal(changed.Body.Bytes(), &changedPlan); err != nil {
		t.Fatal(err)
	}
	if changedPlan.ID != before.ID || changedPlan.Revision != 3 || len(changedPlan.OperationSplits) != 0 {
		t.Fatalf("changed plan should keep identity and clear stale splits: %+v", changedPlan)
	}
	if changedPlan.Items[0].TargetWarehouse != "finished_goods" {
		t.Fatalf("changed item target warehouse=%q want default finished_goods", changedPlan.Items[0].TargetWarehouse)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='production_plan' AND entity_id=%d AND action='recalculate'", before.ID), 2)

	stale := serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", before.ID), map[string]any{
		"selected": []string{"1-227"}, "request_id": "stale-update", "revision": before.Revision,
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revision status=%d body=%s", stale.Code, stale.Body.String())
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET status='submitted' WHERE id=%d`, schema, before.ID))
	submitted := serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d", before.ID), map[string]any{
		"selected": []string{"1-227"}, "request_id": "submitted-update", "revision": changedPlan.Revision,
	})
	if submitted.Code != http.StatusBadRequest {
		t.Fatalf("submitted plan update status=%d body=%s", submitted.Code, submitted.Body.String())
	}
}
