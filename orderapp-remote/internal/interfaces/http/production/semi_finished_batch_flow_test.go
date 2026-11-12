package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
)

// The selection record is inserted directly because the selection command is
// developed independently; all subsequent transfer and submit actions use APIs.
func TestSelectedBatchTransferAndWorkOrderReservationFlow(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CHOSEN", "生产生豆", 21000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2030, 30, "OTHER", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=2030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',15000);`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plans SET semi_finished_stock_policy='manual' WHERE id=%[2]d;
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'raw_materials',1030,15000,0),(%[2]d,'material:30',30,'wip',1030,6000,0);
 INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 {
		t.Fatalf("get %d %s", get.Code, get.Body.String())
	}
	if err := json.Unmarshal(get.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.ComponentSources) != 1 {
		t.Fatalf("sources %+v", plan.ComponentSources)
	}
	source := plan.ComponentSources[0]
	if source.WIPCoveredG != 6000 || source.TransferG != 15000 {
		t.Fatalf("expected chosen 6000 WIP and 15000 raw: %+v", source)
	}
	transfer := func(code string, qty int64, key string) *httptest.ResponseRecorder {
		return serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": key, "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": code, "qty_g": qty}}}}})
	}
	// An explicit extra transfer may use another available batch; cancelling
	// this draft leaves the frozen selected batch and reservation unchanged.
	extra := transfer("OTHER", 1000, "extra-batch")
	var extraDoc app.PlanMaterialTransferResult
	if extra.Code != 200 || json.Unmarshal(extra.Body.Bytes(), &extraDoc) != nil || len(extraDoc.Documents) != 1 {
		t.Fatalf("explicit extra batch rejected: %d %s", extra.Code, extra.Body.String())
	}
	cancel := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/cancel", extraDoc.Documents[0].ID), map[string]any{})
	if cancel.Code != 200 {
		t.Fatalf("cancel extra draft: %d %s", cancel.Code, cancel.Body.String())
	}
	valid := transfer("CHOSEN", 15000, "chosen-batch")
	if valid.Code != 200 {
		t.Fatalf("transfer %d %s", valid.Code, valid.Body.String())
	}
	var doc app.PlanMaterialTransferResult
	if err := json.Unmarshal(valid.Body.Bytes(), &doc); err != nil || len(doc.Documents) != 1 {
		t.Fatalf("document %s %v", valid.Body.String(), err)
	}
	posted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", doc.Documents[0].ID), map[string]any{})
	if posted.Code != 200 {
		t.Fatalf("post %d %s", posted.Code, posted.Body.String())
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'"), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
}

func TestOrderSemiFinishedChoiceThroughTransferToReservation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "CHOSEN-SEMI", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4010, 10, "OTHER-SEMI", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "RAW-MATERIAL", "生产生豆", 100000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "PACKAGING", "227g包装袋", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET qty_g=3000 WHERE material_batch_id=3010;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(3010,10,'raw_materials',7000);`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock", plan.ID)
	choices := []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 3}, map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "raw_materials", "qty": 7}}
	preview := serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"draft_token": plan.DraftToken, "check_only": true, "selections": choices})
	var previewResult app.SemiFinishedStockResult
	if preview.Code != 200 || json.Unmarshal(preview.Body.Bytes(), &previewResult) != nil || previewResult.PreviewToken == "" {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	result := serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"draft_token": plan.DraftToken, "check_only": false, "preview_token": previewResult.PreviewToken, "selections": choices})
	if result.Code != 200 {
		t.Fatalf("choose %d %s", result.Code, result.Body.String())
	}
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 {
		t.Fatalf("get %d %s", get.Code, get.Body.String())
	}
	if err := json.Unmarshal(get.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	var source app.ProductionPlanComponentSource
	for _, s := range plan.ComponentSources {
		if s.ComponentID == 10 {
			source = s
		}
	}
	if source.ID == 0 || source.WIPCoveredG != 3000 || source.TransferG != 7000 {
		t.Fatalf("source %+v, plan %s", source, get.Body.String())
	}
	// A separate stock operation moved 4 kg of the same batch after selection.
	// Reopening the chooser must preserve the total, now split 7/3 by location.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET qty_g=7000 WHERE material_batch_id=3010 AND warehouse='wip'; UPDATE %[1]s.material_batch_locations SET qty_g=3000 WHERE material_batch_id=3010 AND warehouse='raw_materials'`, schema))
	currentMoved := serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
	var movedChoices app.SemiFinishedStockResult
	if currentMoved.Code != 200 || json.Unmarshal(currentMoved.Body.Bytes(), &movedChoices) != nil {
		t.Fatalf("choices after external move: %d %s", currentMoved.Code, currentMoved.Body.String())
	}
	selectedLocations := map[string]float64{}
	for _, item := range movedChoices.Items {
		for _, batch := range item.Batches {
			if batch.BatchID == 3010 {
				selectedLocations[batch.Warehouse] += batch.SelectedQty
			}
		}
	}
	if selectedLocations["wip"] != 7 || selectedLocations["raw_materials"] != 3 {
		t.Fatalf("external move lost or duplicated selection: %s", currentMoved.Body.String())
	}
	movedSelection := []any{map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "wip", "qty": 7}, map[string]any{"key": "material:10", "batch_id": 3010, "warehouse": "raw_materials", "qty": 3}}
	checkedMoved := serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"draft_token": movedChoices.DraftToken, "check_only": true, "selections": movedSelection})
	var movedPreview app.SemiFinishedStockResult
	if checkedMoved.Code != 200 || json.Unmarshal(checkedMoved.Body.Bytes(), &movedPreview) != nil || movedPreview.PreviewToken == "" {
		t.Fatalf("recheck moved selection %d %s", checkedMoved.Code, checkedMoved.Body.String())
	}
	savedMoved := serveMultilevelProductionJSON(t, e, http.MethodPost, url, map[string]any{"draft_token": movedChoices.DraftToken, "check_only": false, "preview_token": movedPreview.PreviewToken, "selections": movedSelection})
	if savedMoved.Code != 200 {
		t.Fatalf("save moved selection %d %s", savedMoved.Code, savedMoved.Body.String())
	}
	get = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil {
		t.Fatalf("get after save %d %s", get.Code, get.Body.String())
	}
	for _, s := range plan.ComponentSources {
		if s.ComponentID == 10 {
			source = s
		}
	}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "order-selected-flow", "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "CHOSEN-SEMI", "qty_g": 3000}}}}})
	if transfer.Code != 200 {
		t.Fatalf("transfer %d %s", transfer.Code, transfer.Body.String())
	}
	var doc app.PlanMaterialTransferResult
	if err := json.Unmarshal(transfer.Body.Bytes(), &doc); err != nil || len(doc.Documents) != 1 {
		t.Fatalf("document %s %v", transfer.Body.String(), err)
	}
	posted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", doc.Documents[0].ID), map[string]any{})
	if posted.Code != 200 {
		t.Fatalf("post %d %s", posted.Code, posted.Body.String())
	}
	current := serveMultilevelProductionJSON(t, e, http.MethodGet, url, nil)
	var currentChoices app.SemiFinishedStockResult
	if current.Code != 200 || json.Unmarshal(current.Body.Bytes(), &currentChoices) != nil {
		t.Fatalf("choices after transfer: %d %s", current.Code, current.Body.String())
	}
	var selectedWIP float64
	for _, item := range currentChoices.Items {
		for _, batch := range item.Batches {
			if batch.BatchID == 3010 && batch.Warehouse == "wip" {
				selectedWIP = batch.SelectedQty
			}
		}
	}
	if selectedWIP != 10 {
		t.Fatalf("selection should follow posted batch into WIP: %s", current.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=3010 AND warehouse='wip' AND reserved_g=10000 AND status='reserved'", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=4010", 0)
}

func TestSelectedBatchCompetingPlansCannotBothSubmit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CONTENDED", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	ids := []int64{}
	for i := 0; i < 2; i++ {
		created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
		if created.Code != 200 {
			t.Fatalf("create %d %s", created.Code, created.Body.String())
		}
		var plan app.ProductionPlanDetail
		if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, plan.ID)
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plans SET semi_finished_stock_policy='manual' WHERE id=%[2]d;
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'wip',1030,21000,0);
 INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	}
	results := make(chan int, 2)
	start := make(chan struct{})
	for _, id := range ids {
		go func(id int64) {
			<-start
			rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", id), nil)
			results <- rec.Code
		}(id)
	}
	close(start)
	a, b := <-results, <-results
	if (a == http.StatusOK) == (b == http.StatusOK) {
		t.Fatalf("expected exactly one successful submit, got %d and %d", a, b)
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'", 1)
}
