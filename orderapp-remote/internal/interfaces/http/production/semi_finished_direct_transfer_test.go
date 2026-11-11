package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
	"testing"
)

func TestDirectSelectedBatchTransferAndReservationFlow(t *testing.T) {
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
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID)
	command := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-test"}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var result app.PlanMaterialTransferResult
	if transfer.Code != 200 || json.Unmarshal(transfer.Body.Bytes(), &result) != nil || result.Plan == nil || len(result.Documents) != 1 || result.Documents[0].Status != "submitted" {
		t.Fatalf("direct transfer %d %s", transfer.Code, transfer.Body.String())
	}
	retry := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var again app.PlanMaterialTransferResult
	if retry.Code != 200 || json.Unmarshal(retry.Body.Bytes(), &again) != nil || len(again.Documents) != 1 || again.Documents[0].ID != result.Documents[0].ID {
		t.Fatalf("retry %d %s", retry.Code, retry.Body.String())
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'"), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
}

func TestDirectSelectedBatchReusesDraft(t *testing.T) {
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
	createdDraft := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "preexisting", "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "CHOSEN", "qty_g": 15000}}}}})
	var draft app.PlanMaterialTransferResult
	if createdDraft.Code != 200 || json.Unmarshal(createdDraft.Body.Bytes(), &draft) != nil || len(draft.Documents) != 1 {
		t.Fatalf("draft %d %s", createdDraft.Code, createdDraft.Body.String())
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID)
	command := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-test"}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var result app.PlanMaterialTransferResult
	if transfer.Code != 200 || json.Unmarshal(transfer.Body.Bytes(), &result) != nil || result.Plan == nil || len(result.Documents) != 1 || result.Documents[0].Status != "submitted" {
		t.Fatalf("direct transfer %d %s", transfer.Code, transfer.Body.String())
	}
	if result.Documents[0].ID != draft.Documents[0].ID {
		t.Fatal("did not reuse draft")
	}
	retry := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var again app.PlanMaterialTransferResult
	if retry.Code != 200 || json.Unmarshal(retry.Body.Bytes(), &again) != nil || len(again.Documents) != 1 || again.Documents[0].ID != result.Documents[0].ID {
		t.Fatalf("retry %d %s", retry.Code, retry.Body.String())
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'"), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
}

func TestDirectSelectedBatchRejectsTamperedDraft(t *testing.T) {
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
	createdDraft := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "preexisting", "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "CHOSEN", "qty_g": 15000}}}}})
	var draft app.PlanMaterialTransferResult
	if createdDraft.Code != 200 || json.Unmarshal(createdDraft.Body.Bytes(), &draft) != nil || len(draft.Documents) != 1 {
		t.Fatalf("draft %d %s", createdDraft.Code, createdDraft.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.stock_entry_items SET batch_code='OTHER' WHERE stock_entry_id=%d`, schema, draft.Documents[0].ID))
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID)
	command := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-test"}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	if transfer.Code == 200 {
		t.Fatalf("tampered draft posted: %s", transfer.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("id=%d AND status='draft'", draft.Documents[0].ID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("source_type='production_plan_material_transfer' AND source_id=%d", plan.ID), 1)
}

func TestDirectSelectedBatchMultiWarehouseRollback(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CHOSEN", "生产生豆", 21000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2030, 30, "OTHER", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=2030;
 INSERT INTO %[1]s.warehouses(code,name,kind,active,customer_id) VALUES('raw2','第二原料仓','raw',true,0);
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',7000),(1030,30,'raw2',8000);`, schema))
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
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'raw_materials',1030,7000,0),(%[2]d,'material:30',30,'raw2',1030,8000,0),(%[2]d,'material:30',30,'wip',1030,6000,0);
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
	var documents []app.PlanMaterialTransferResult
	for index, choice := range []struct {
		wh  string
		qty int64
	}{{"raw_materials", 7000}, {"raw2", 8000}} {
		response := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": fmt.Sprintf("draft-%d", index), "items": []any{map[string]any{"source_id": source.ID, "warehouse": choice.wh, "batches": []any{map[string]any{"batch_code": "CHOSEN", "qty_g": choice.qty}}}}})
		var d app.PlanMaterialTransferResult
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &d) != nil || len(d.Documents) != 1 {
			t.Fatalf("create draft %d %s", response.Code, response.Body.String())
		}
		documents = append(documents, d)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_material_transfer_links SET source_snapshot='{}'::jsonb WHERE stock_entry_id=%d`, schema, documents[1].Documents[0].ID))
	response := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "rollback"})
	if response.Code == 200 {
		t.Fatalf("stale second draft should fail: %s", response.Body.String())
	}
	for _, d := range documents {
		assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("id=%d AND status='draft'", d.Documents[0].ID), 1)
	}
	var qty int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=1030 AND warehouse='wip'", schema)).Scan(&qty); err != nil || qty != 6000 {
		t.Fatalf("partial posting escaped rollback: %d %v", qty, err)
	}
}

func TestDirectSelectedBatchAllWIPIsNoOp(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CHOSEN", "生产生豆", 21000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2030, 30, "OTHER", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=21000 WHERE material_batch_id=1030;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=2030;
 `, schema))
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
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'wip',1030,21000,0);
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
	if source.WIPCoveredG != 21000 || source.TransferG != 0 {
		t.Fatalf("expected chosen 6000 WIP and 15000 raw: %+v", source)
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID)
	command := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-test"}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var result app.PlanMaterialTransferResult
	if transfer.Code != 200 || json.Unmarshal(transfer.Body.Bytes(), &result) != nil || result.Plan == nil || len(result.Documents) != 0 {
		t.Fatalf("direct transfer %d %s", transfer.Code, transfer.Body.String())
	}
	retry := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var again app.PlanMaterialTransferResult
	if retry.Code != 200 || json.Unmarshal(retry.Body.Bytes(), &again) != nil || len(again.Documents) != 0 {
		t.Fatalf("retry %d %s", retry.Code, retry.Body.String())
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'"), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
}

func TestDirectSelectedBatchReusesPartialDraftAndTransfersRemainder(t *testing.T) {
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
	createdDraft := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "preexisting", "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": "CHOSEN", "qty_g": 7000}}}}})
	var draft app.PlanMaterialTransferResult
	if createdDraft.Code != 200 || json.Unmarshal(createdDraft.Body.Bytes(), &draft) != nil || len(draft.Documents) != 1 {
		t.Fatalf("draft %d %s", createdDraft.Code, createdDraft.Body.String())
	}
	url := fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID)
	command := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-test"}
	transfer := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var result app.PlanMaterialTransferResult
	if transfer.Code != 200 || json.Unmarshal(transfer.Body.Bytes(), &result) != nil || result.Plan == nil || len(result.Documents) != 2 || result.Documents[0].Status != "submitted" || result.Documents[1].Status != "submitted" {
		t.Fatalf("direct transfer %d %s", transfer.Code, transfer.Body.String())
	}
	if result.Documents[0].ID != draft.Documents[0].ID {
		t.Fatal("did not reuse draft")
	}
	retry := serveMultilevelProductionJSON(t, e, http.MethodPost, url, command)
	var again app.PlanMaterialTransferResult
	if retry.Code != 200 || json.Unmarshal(retry.Body.Bytes(), &again) != nil || len(again.Documents) != 2 || again.Documents[0].ID != result.Documents[0].ID || again.Documents[1].ID != result.Documents[1].ID {
		t.Fatalf("retry %d %s", retry.Code, retry.Body.String())
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'"), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
}
