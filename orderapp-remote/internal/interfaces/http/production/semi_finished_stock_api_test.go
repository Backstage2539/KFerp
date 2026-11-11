package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestSemiFinishedStockChoicePreviewLeavesDraftUnchanged(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "SELECTABLE-SEMIFINISHED", "在制熟豆", 10000)
	e := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	got := serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
	if got.Code != 200 {
		t.Fatalf("candidates %d %s", got.Code, got.Body.String())
	}
	var candidates app.SemiFinishedStockResult
	if err := json.Unmarshal(got.Body.Bytes(), &candidates); err != nil {
		t.Fatal(err)
	}
	if len(candidates.Items) != 1 || candidates.Items[0].StockCoveredQty != 0 || candidates.Items[0].ManufactureQty != 22.7 || len(candidates.Items[0].Batches) == 0 {
		t.Fatalf("unexpected new-plan candidates %s", got.Body.String())
	}
	selection := map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 10}
	preview := serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": []any{selection}})
	if preview.Code != 200 {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var result app.SemiFinishedStockResult
	if err := json.Unmarshal(preview.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].StockCoveredQty != 10 || result.Items[0].ManufactureQty != 12.7 {
		t.Fatalf("preview wrong: %s", preview.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_semi_finished_selections", fmt.Sprintf("production_plan_id=%d", plan.ID), 0)
}

func TestSemiFinishedFrozenRecipeSurvivesFullStockCoverage(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "FROZEN-SEMIFINISHED", "在制熟豆", 22700)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatalf("create: %s", rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g) VALUES(%[2]d,'material:10',10,'wip',3010,22700)`, schema, plan.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), nil)
	if rec.Code != 200 {
		t.Fatalf("full cover: %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=10", plan.ID), 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=200; INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES(201,200,'V002','published',1,0,1,'kg',32,now()); INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES(201,30,'material','g',2000,0);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=201 WHERE output_type='material' AND output_id=10;DELETE FROM %[1]s.production_plan_semi_finished_selections WHERE production_plan_id=%[2]d`, schema, plan.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), nil)
	if rec.Code != 200 {
		t.Fatalf("restore: %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=10 AND bom_version_id=200 AND planned_g=28375", plan.ID), 1)
}

func TestSemiFinishedConfirmRestoresFrozenRecipeAfterFullCover(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "FROZEN-COMMIT", "在制熟豆", 22700)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	var upstreamID int64
	for _, item := range plan.Items {
		if item.OutputType == "material" && item.OutputMaterialID == 10 {
			upstreamID = item.ID
		}
	}
	if upstreamID == 0 {
		t.Fatal("missing upstream task")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'批次测试工位','active',102); INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes) VALUES(719,719,'20kg','active',20,'kg',25); INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'烘焙','active'); INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719);`, schema))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": []any{map[string]any{"production_plan_item_id": upstreamID, "operation_seq": 1, "operation": "烘焙", "workstation_capacity_id": 719, "planned_batch_count": 2}}})
	if rec.Code != 200 {
		t.Fatalf("splits: %d %s", rec.Code, rec.Body.String())
	}
	rec = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &plan) != nil {
		t.Fatalf("read: %s", rec.Body.String())
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	body := map[string]any{"draft_token": plan.DraftToken, "selections": []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 22.7}}}
	body["check_only"] = true
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var preview app.SemiFinishedStockResult
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) == 0 || len(preview.Changes[0].Operations) == 0 || preview.Changes[0].Operations[0].Workstation != "批次测试工位" {
		t.Fatalf("missing capacity preview: %s", rec.Body.String())
	}
	if preview.PreviewToken == "" {
		t.Fatal("preview token missing")
	}
	changedChoices := []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 22.6}}
	staleChoice := map[string]any{"draft_token": plan.DraftToken, "preview_token": preview.PreviewToken, "selections": changedChoices}
	if stale := serveMultilevelProductionJSON(t, e, http.MethodPost, url, staleChoice); stale.Code == 200 {
		t.Fatal("changed selection must reject stale preview")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET standard_minutes=30 WHERE id=719`, schema))
	body["check_only"] = false
	body["preview_token"] = preview.PreviewToken
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code == 200 {
		t.Fatal("changed capacity must reject stale preview")
	}
	body["check_only"] = true
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil {
		t.Fatalf("new preview: %s", rec.Body.String())
	}
	body["preview_token"] = preview.PreviewToken
	body["check_only"] = false
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 {
		t.Fatalf("select: %d %s", rec.Code, rec.Body.String())
	}
	var result app.SemiFinishedStockResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plan == nil {
		t.Fatal("missing updated plan")
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", plan.ID), 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=200;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES(201,200,'V002','published',1,0,1,'kg',32,now());INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES(201,30,'material','g',2000,0);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=201 WHERE output_type='material' AND output_id=10`, schema))
	clearBody := map[string]any{"draft_token": result.Plan.DraftToken, "selections": []any{}, "check_only": true}
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, clearBody)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil {
		t.Fatalf("clear preview: %s", rec.Body.String())
	}
	clearBody["check_only"] = false
	clearBody["preview_token"] = preview.PreviewToken
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, clearBody)
	if rec.Code != 200 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND bom_version_id=200 AND planned_g=28375", plan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_operation_splits", fmt.Sprintf("production_plan_id=%d AND workstation_capacity_id=719", plan.ID), 1)
}

func TestLegacySemiFinishedRefreshDoesNotChangeExistingDeduction(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "LEGACY-SEMIFINISHED-A", "在制熟豆", 5000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	choiceURL := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	body := map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 5}}}
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, choiceURL, body)
	var preview app.SemiFinishedStockResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil {
		t.Fatalf("preview: %s", rec.Body.String())
	}
	body["check_only"] = false
	body["preview_token"] = preview.PreviewToken
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, choiceURL, body)
	if rec.Code != 200 {
		t.Fatalf("confirm: %s", rec.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_plan_semi_finished_selections WHERE production_plan_id=%[2]d; UPDATE %[1]s.production_plans SET semi_finished_stock_policy='' WHERE id=%[2]d`, schema, plan.ID))
	url := fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_qty=17.7", plan.ID), 1)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3011, 10, "LEGACY-SEMIFINISHED-B", "在制熟豆", 5000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"check_only": false})
	if rec.Code != 200 {
		t.Fatalf("refresh: %s", rec.Body.String())
	}
	var result app.ProductionPlanSupplySyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatalf("change should be reported: %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_qty=17.7", plan.ID), 1)
}

func TestSemiFinishedSixtyKilogramChoicesAndRefreshAreNotDoubleDeducted(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;UPDATE %[1]s.production_bom_version_items SET qty_per_unit=601.12 WHERE version_id=100 AND material_id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "SIXTY-KG-SEMI", "在制熟豆", 15000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	check := func(want float64) {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
		var got app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &got) != nil || len(got.Items) != 1 {
			t.Fatalf("read: %s", r.Body.String())
		}
		if math.Abs(got.Items[0].RequiredQty-60.112) > 0.000001 || math.Abs(got.Items[0].ManufactureQty-want) > 0.000001 {
			t.Fatalf("expected 60.112 / %v: %s", want, r.Body.String())
		}
	}
	check(60.112)
	for _, step := range []struct{ qty, want float64 }{{10, 50.112}, {15, 45.112}, {0, 60.112}} {
		choices := []any{}
		if step.qty > 0 {
			choices = append(choices, map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": step.qty})
		}
		body := map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": choices}
		r := serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
		var preview app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &preview) != nil || preview.PreviewToken == "" {
			t.Fatalf("preview: %d %s", r.Code, r.Body.String())
		}
		body["check_only"] = false
		body["preview_token"] = preview.PreviewToken
		r = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
		var result app.SemiFinishedStockResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &result) != nil || result.Plan == nil {
			t.Fatalf("confirm: %d %s", r.Code, r.Body.String())
		}
		plan = *result.Plan
		check(step.want)
		for i := 0; i < 2; i++ {
			r = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"draft_token": plan.DraftToken, "check_only": false})
			if r.Code != 200 {
				t.Fatalf("refresh: %d %s", r.Code, r.Body.String())
			}
			var refreshed app.ProductionPlanSupplySyncResult
			if err := json.Unmarshal(r.Body.Bytes(), &refreshed); err != nil {
				t.Fatal(err)
			}
			plan = refreshed.Plan
			check(step.want)
		}
	}
}

func TestSemiFinishedSelectionProtectsUnboundHistoricalReservations(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "UNBOUND-SEMI", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "UNBOUND-RAW", "生产生豆", 100000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "UNBOUND-BAG", "227g包装袋", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.work_orders(id,work_order_no,status) VALUES(900,'LEGACY-RESERVED','released');INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,material_name,unit,required_g,reserved_g,status,source_warehouse,source_owner_customer_id) VALUES(900,10,'在制熟豆','kg',7000,7000,'reserved','wip',0)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	rec = serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
	var state app.SemiFinishedStockResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &state) != nil || len(state.Items) != 1 || len(state.Items[0].Batches) != 1 {
		t.Fatalf("candidates: %d %s", rec.Code, rec.Body.String())
	}
	if state.Items[0].Batches[0].AvailableQty != 3 {
		t.Fatalf("unbound reservation not protected: %s", rec.Body.String())
	}
	makeBody := func(qty float64) map[string]any {
		return map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": qty}}}
	}
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, makeBody(4))
	if rec.Code == 200 {
		t.Fatal("must reject other order's unbound reservation")
	}
	body := makeBody(3)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &state) != nil {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	body["preview_token"] = state.PreviewToken
	body["check_only"] = false
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 {
		t.Fatalf("safe selection: %d %s", rec.Code, rec.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_order_material_reservations SET reserved_g=8000,required_g=8000 WHERE work_order_id=900;INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if rec.Code == 200 || !strings.Contains(rec.Body.String(), "未分配批次") {
		t.Fatalf("submit must protect new unbound reservation: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLegacySemiFinishedZeroCoverageDoesNotChangeAfterStockArrival(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='' WHERE id=%d`, schema, plan.ID))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "OLD-ZERO-SEMIFINISHED", "在制熟豆", 15000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"check_only": false})
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
	var result app.ProductionPlanSupplySyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed {
		t.Fatalf("must report changed supply: %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_qty=22.7", plan.ID), 1)
}

func TestLegacyPlanWithoutSupplyGraphExplainsChoiceUnavailable(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='',supply_graph_json=NULL WHERE id=%d`, schema, plan.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID), nil)
	if rec.Code == 200 || !strings.Contains(rec.Body.String(), "历史草稿") || !strings.Contains(rec.Body.String(), "生产计划") {
		t.Fatalf("must explain missing history: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLegacyStockPlanMayRefreshSemiFinishedComponents(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id IN (10,30)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []any{map[string]any{"output_type": "material", "output_material_id": 10, "output_qty": 10, "target_warehouse": "wip"}}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 6030, 30, "STOCK-REFRESH-COMPONENT", "生产生豆", 10000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"check_only": false})
	if rec.Code != 200 {
		t.Fatalf("refresh: %s", rec.Body.String())
	}
	var result app.ProductionPlanSupplySyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Plan.Revision <= plan.Revision {
		t.Fatalf("stock plan must refresh: %s", rec.Body.String())
	}
}

func TestLegacyPurchasedSemiFinishedRetainsNormalSupplyRefresh(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;DELETE FROM %[1]s.production_bom_output_bindings WHERE output_type='material' AND output_id=10`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='' WHERE id=%d`, schema, plan.ID))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "PURCHASED-SEMI", "外购半成品", 5000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), map[string]any{"check_only": false})
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
	}
	var result app.ProductionPlanSupplySyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Plan.Revision <= plan.Revision {
		t.Fatalf("purchased material must use normal refresh: %s", rec.Body.String())
	}
}

func TestLegacySupplyRefreshDoesNotFreezeUnrelatedUpstreamRecipes(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		stock    bool
		quantity int64
	}{{"legacy order without self-made semi-finished policy", false, 1000}, {"independent stock production", true, 6000}} {
		t.Run(scenario.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			seedSharedUpstreamMaterialFlow(t, ctx, pool, schema, true)
			e := newProductionFlowTestEcho(pool, schema)
			body := map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}}
			if scenario.stock {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
				body = map[string]any{"source_type": "stock", "items": []any{map[string]any{"output_type": "material", "output_material_id": 10, "output_qty": 1, "target_warehouse": "wip"}}}
			}
			rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", body)
			if rec.Code != 200 {
				t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
			}
			var plan app.ProductionPlanDetail
			if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if !scenario.stock {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='' WHERE id=%d`, schema, plan.ID))
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=30 AND bom_version_id=300", plan.ID), 1)
			seedProductionFlowWIPBatch(t, ctx, pool, schema, 9030, 30, "LEGACY-NO-FREEZE", "共用组件", scenario.quantity)
			endpoint := fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID)
			rec = serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, nil)
			if rec.Code != 200 {
				t.Fatalf("cover: %d %s", rec.Code, rec.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=30", plan.ID), 0)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=300;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES(301,300,'V002','published',1,0,0.001,'kg',32,now());INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES(301,40,'material','g',2,0);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=301 WHERE output_type='material' AND output_id=30;UPDATE %[1]s.material_batch_locations SET qty_g=0 WHERE material_batch_id=9030`, schema))
			rec = serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, nil)
			if rec.Code != 200 {
				t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=30 AND bom_version_id=301", plan.ID), 1)
		})
	}
}

func TestLegacyExplicitSelectionFreezesExistingRecipeAndCapacity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "FROZEN-COMMIT", "在制熟豆", 22700)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_plan_frozen_manufacturing_outputs WHERE production_plan_id=%[2]d; UPDATE %[1]s.production_plans SET semi_finished_stock_policy='' WHERE id=%[2]d`, schema, plan.ID))
	var upstreamID int64
	for _, item := range plan.Items {
		if item.OutputType == "material" && item.OutputMaterialID == 10 {
			upstreamID = item.ID
		}
	}
	if upstreamID == 0 {
		t.Fatal("missing upstream task")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'批次测试工位','active',102); INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes) VALUES(719,719,'20kg','active',20,'kg',25); INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'烘焙','active'); INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719);`, schema))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/operation-splits", plan.ID), map[string]any{"items": []any{map[string]any{"production_plan_item_id": upstreamID, "operation_seq": 1, "operation": "烘焙", "workstation_capacity_id": 719, "planned_batch_count": 2}}})
	if rec.Code != 200 {
		t.Fatalf("splits: %d %s", rec.Code, rec.Body.String())
	}
	rec = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &plan) != nil {
		t.Fatalf("read: %s", rec.Body.String())
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	body := map[string]any{"draft_token": plan.DraftToken, "selections": []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 22.7}}}
	body["check_only"] = true
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var preview app.SemiFinishedStockResult
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) == 0 || len(preview.Changes[0].Operations) == 0 || preview.Changes[0].Operations[0].Workstation != "批次测试工位" {
		t.Fatalf("missing capacity preview: %s", rec.Body.String())
	}
	if preview.PreviewToken == "" {
		t.Fatal("preview token missing")
	}
	changedChoices := []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 22.6}}
	staleChoice := map[string]any{"draft_token": plan.DraftToken, "preview_token": preview.PreviewToken, "selections": changedChoices}
	if stale := serveMultilevelProductionJSON(t, e, http.MethodPost, url, staleChoice); stale.Code == 200 {
		t.Fatal("changed selection must reject stale preview")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET standard_minutes=30 WHERE id=719`, schema))
	body["check_only"] = false
	body["preview_token"] = preview.PreviewToken
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code == 200 {
		t.Fatal("changed capacity must reject stale preview")
	}
	body["check_only"] = true
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil {
		t.Fatalf("new preview: %s", rec.Body.String())
	}
	body["preview_token"] = preview.PreviewToken
	body["check_only"] = false
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, body)
	if rec.Code != 200 {
		t.Fatalf("select: %d %s", rec.Code, rec.Body.String())
	}
	var result app.SemiFinishedStockResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Plan == nil {
		t.Fatal("missing updated plan")
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", plan.ID), 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=200;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES(201,200,'V002','published',1,0,1,'kg',32,now());INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES(201,30,'material','g',2000,0);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=201 WHERE output_type='material' AND output_id=10`, schema))
	clearBody := map[string]any{"draft_token": result.Plan.DraftToken, "selections": []any{}, "check_only": true}
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, clearBody)
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &preview) != nil {
		t.Fatalf("clear preview: %s", rec.Body.String())
	}
	clearBody["check_only"] = false
	clearBody["preview_token"] = preview.PreviewToken
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, url, clearBody)
	if rec.Code != 200 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND bom_version_id=200 AND planned_g=28375", plan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_operation_splits", fmt.Sprintf("production_plan_id=%d AND workstation_capacity_id=719", plan.ID), 1)
}
