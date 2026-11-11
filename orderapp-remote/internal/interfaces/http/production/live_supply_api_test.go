package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestLiveSupplyLegacyEmptyRequestsReturnPlan(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if created.Code != 200 {
		t.Fatal(created.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	for _, body := range []any{nil, map[string]any{}} {
		rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), body)
		var got app.ProductionPlanDetail
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.ID != plan.ID {
			t.Fatalf("legacy body %#v: %d %s", body, rec.Code, rec.Body.String())
		}
	}
}

func TestLiveSupplySummaryNamesNewOperations(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_plan_item_dependencies WHERE production_plan_id=%[2]d;
DELETE FROM %[1]s.production_plan_component_sources WHERE production_plan_item_id IN (SELECT id FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d AND output_type='material');
DELETE FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d AND output_type='material';
UPDATE %[1]s.production_plans SET supply_graph_json=NULL WHERE id=%[2]d;`, schema, p.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", p.ID), map[string]any{"check_only": false})
	var out struct {
		Summary string `json:"summary"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil || !strings.Contains(out.Summary, "新增") || !strings.Contains(out.Summary, "烘焙") {
		t.Fatalf("summary: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLiveSupplyManualSourcesStayOnExactTask(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id) SELECT 2,'SO-DIFF-SPEC','2026-08-11',false,process_status_id FROM %[1]s.orders WHERE id=1;
INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total,price_source_json) VALUES(2,1,'454g包装熟豆',30,'袋','454g',1,50,1500,'{"production_quantity_snapshot":{"sku_id":1,"parent_product_id":1,"spec_label":"454g","sales_unit":"袋","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454,"conversion_source":"published"}}');
INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active) VALUES('raw_backup','备用原料仓','raw_materials',90,true);`, schema))
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 3020, 20, "TWO-SOURCE-BAG", "227g包装袋", 300)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials',qty_units=150 WHERE material_batch_id=3020;
INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_units) VALUES (3020,20,'raw_backup',150)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227", "1-454"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	var roots []int64
	for _, item := range p.Items {
		if item.OutputType == "product" {
			roots = append(roots, item.ID)
		}
	}
	if len(roots) != 2 {
		t.Fatalf("expected distinct order tasks %+v", p.Items)
	}
	for i, id := range roots {
		warehouse := []string{"raw_materials", "raw_backup"}[i]
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plan_component_sources SET allocation_mode='manual',allocations_json='[{"warehouse":"%[3]s","owner_customer_id":0,"qty_units":1}]'::jsonb WHERE production_plan_item_id=%[2]d AND component_id=20`, schema, id, warehouse))
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "FORCE-SYNC", "在制熟豆", 1000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", p.ID), map[string]any{"check_only": false})
	if rec.Code != 200 {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	for i, id := range roots {
		var raw []byte
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT allocations_json FROM %s.production_plan_component_sources WHERE production_plan_item_id=$1 AND component_id=20`, schema), id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		want := []string{"raw_materials", "raw_backup"}[i]
		if !strings.Contains(string(raw), want) {
			t.Fatalf("task %d expected %s, got %s", id, want, raw)
		}
	}
}

func TestLiveSupplySummaryShowsQuantityAndRemovedOperations(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	for i, step := range []struct {
		qty      int64
		fragment string
	}{{5000, "数量变化"}, {17700, "减少工序"}} {
		seedProductionFlowWIPBatch(t, ctx, pool, schema, int64(9000+i), 10, fmt.Sprintf("SUMMARY-%d", i), "在制熟豆", step.qty)
		rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", p.ID), map[string]any{"draft_token": p.DraftToken})
		var out struct {
			Plan    app.ProductionPlanDetail `json:"plan"`
			Summary string                   `json:"summary"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil || !strings.Contains(out.Summary, step.fragment) {
			t.Fatalf("summary %d: %d %s", i, rec.Code, rec.Body.String())
		}
		if i == 0 && (!strings.Contains(out.Summary, "22.7") || !strings.Contains(out.Summary, "17.7")) {
			t.Fatalf("missing quantities: %s", out.Summary)
		}
		if i == 1 && !strings.Contains(out.Summary, "烘焙") {
			t.Fatalf("missing operation: %s", out.Summary)
		}
		p = out.Plan
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_id=%d AND entity_type='production_plan' AND action='refresh_supply' AND meta ? 'changes'", p.ID), 2)
}

func TestLiveSupplyReplacesEqualQuantityInflightSupplier(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 8030, 30, "REPLACE-GREEN", "生产生豆", 100000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 8020, 20, "REPLACE-BAG", "227g包装袋", 1000)
	e := newProductionFlowTestEcho(pool, schema)
	var suppliers []int64
	for i := 0; i < 2; i++ {
		rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 30, "target_warehouse": "wip"}}})
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		var p app.ProductionPlanDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		preparePlanningTest(t, ctx, pool, schema, e, p.ID)
		rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", p.ID), nil)
		if rec.Code != 200 {
			t.Fatalf("supplier submit %s", rec.Body.String())
		}
		var id int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), p.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		suppliers = append(suppliers, id)
	}
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_supply_allocations", fmt.Sprintf("production_plan_id=%d AND supplier_work_order_id=%d", p.ID, suppliers[0]), 1)
	preparePlanningTest(t, ctx, pool, schema, e, p.ID)
	before := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", p.ID), nil)
	if err := json.Unmarshal(before.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	check := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", p.ID), map[string]any{"draft_token": p.DraftToken, "check_only": true})
	var unchanged struct {
		Changed bool `json:"changed"`
	}
	if check.Code != 200 || json.Unmarshal(check.Body.Bytes(), &unchanged) != nil || unchanged.Changed {
		t.Fatalf("unchanged supplier: %d %s", check.Code, check.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET status='cancelled' WHERE id=%d`, schema, suppliers[0]))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", p.ID), map[string]any{"draft_token": p.DraftToken})
	var out struct {
		Changed bool `json:"changed"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &out) != nil || !out.Changed {
		t.Fatalf("supplier identity must trigger sync: %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_supply_allocations", fmt.Sprintf("production_plan_id=%d AND supplier_work_order_id=%d AND warehouse='wip' AND owner_customer_id=0 AND required_g=22700", p.ID, suppliers[1]), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_operation_splits", fmt.Sprintf("production_plan_id=%d", p.ID), 1)
}

func TestLiveSupplyDraftSaveRequiresExplicitScheduleReview(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"selected": []string{"1-227"}})
	if create.Code != 200 {
		t.Fatal(create.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(create.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'批次测试工位','active',102);
INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes) VALUES(719,719,'20kg','active',20,'kg',25);
INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'烘焙','active');
INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719);`, schema))
	rows := []map[string]any{{"production_plan_item_id": plan.Items[0].ID, "operation_seq": 1, "operation": "烘焙", "workstation_capacity_id": 719, "planned_batch_count": 6}}
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": rows})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_operation_splits SET note='[[SUPPLY_RECHECK_REQUIRED]]' WHERE production_plan_id=%d`, schema, plan.ID))
	read := func() app.ProductionPlanDetail {
		rec := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
		var d app.ProductionPlanDetail
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &d) != nil {
			t.Fatal(rec.Body.String())
		}
		return d
	}
	plan = read()
	for _, ack := range []bool{false, true} {
		rec = serveMultilevelProductionJSON(t, e, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d/draft", plan.ID), map[string]any{"draft_token": plan.DraftToken, "operation_splits": rows, "ack_supply_recheck": ack})
		if rec.Code != 200 {
			t.Fatal(rec.Body.String())
		}
		plan = read()
		has := len(plan.OperationSplits) > 0 && strings.Contains(plan.OperationSplits[0].Note, "[[SUPPLY_RECHECK_REQUIRED]]")
		if has == ack {
			t.Fatalf("ack %v: split %+v", ack, plan.OperationSplits)
		}
	}
}

func TestLiveSupplyHistoricalRootBOMStillCreatesUpstream(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES (101,'NEW-PACK','新版包装','product',1,0,'active');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
VALUES (101,101,'V002','published',1,0,1,'kg',31,now());
UPDATE %[1]s.production_bom_output_bindings SET bom_id=101,bom_version_id=101 WHERE output_type='product' AND output_id=1;`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var p app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='product' AND bom_version_id=100", p.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=10", p.ID), 1)
}

func TestLiveSupplyUnchangedCheckAndApplyPreserveTask(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material'`, schema), before.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{true, false} {
		rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{"check_only": check, "draft_token": before.DraftToken})
		var result struct {
			Changed bool                     `json:"changed"`
			Plan    app.ProductionPlanDetail `json:"plan"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &result) != nil || result.Changed || result.Plan.ID != before.ID || result.Plan.DraftToken != before.DraftToken {
			t.Fatalf("check=%v: %d %s", check, rec.Code, rec.Body.String())
		}
		assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("id=%d", id), 1)
	}
}

func TestLiveSupplyQuantityChangeRetainsTaskAndCapacity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var id, beforeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id,planned_output_g FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material'`, schema), before.ID).Scan(&id, &beforeG); err != nil {
		t.Fatal(err)
	}
	preparePlanningTest(t, ctx, pool, schema, e, before.ID)
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", before.ID), nil)
	if err := json.Unmarshal(get.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2001, 10, "LIVE-WIP", "在制熟豆", 5000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{"draft_token": before.DraftToken, "check_only": false})
	if rec.Code != 200 {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Changed bool                     `json:"changed"`
		Plan    app.ProductionPlanDetail `json:"plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || !out.Changed {
		t.Fatalf("sync %s", rec.Body.String())
	}
	var afterG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT planned_output_g FROM %s.production_plan_items WHERE id=$1`, schema), id).Scan(&afterG); err != nil {
		t.Fatal(err)
	}
	if afterG != beforeG-5000 {
		t.Fatalf("planned output before=%d after=%d", beforeG, afterG)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_operation_splits", fmt.Sprintf("production_plan_item_id=%d", id), 1)
}

func TestLiveSupplyRecursiveFrozenRecipeUpdatesFixedBasis(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedSharedUpstreamMaterialFlow(t, ctx, pool, schema, true)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	ids := map[int64]int64{}
	for _, item := range plan.Items {
		if item.OutputType == "material" {
			ids[item.OutputMaterialID] = item.ID
		}
	}
	if len(ids) != 3 {
		t.Fatalf("expected three intermediate tasks: %+v", plan.Items)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2020, 10, "RECURSIVE-STOCK", "半成品A", 50)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"draft_token": plan.DraftToken})
	if rec.Code != 200 {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	for material, want := range map[int64]int64{10: 50, 20: 100, 30: 700} {
		var got int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT planned_output_g FROM %s.production_plan_items WHERE id=$1`, schema), ids[material]).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("material %d: got %d want %d", material, got, want)
		}
	}
}

func TestLiveSupplyFrozenUnitRecipeRoundsAtNewQuantity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.order_items SET qty=5 WHERE order_id=1;
INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES (200,20,'material','fixed_qty',1,0);
INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES (201,'BAG-BOM','袋生产','material',0,20,'active');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES (201,201,'V001','published',1,0,1,'unit',32,now());
INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES (201,30,'material','g',1,0);
INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by) VALUES ('material',20,201,201,true,'test');`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2021, 10, "ROUND-STOCK", "在制熟豆", 135)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"draft_token": plan.DraftToken})
	if rec.Code != 200 {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	var qty float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_qty FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material' AND output_material_id=20`, schema), plan.ID).Scan(&qty); err != nil {
		t.Fatal(err)
	}
	if qty != 6 {
		t.Fatalf("packaging plus fixed recipe expected 6 units; got %v", qty)
	}
}
