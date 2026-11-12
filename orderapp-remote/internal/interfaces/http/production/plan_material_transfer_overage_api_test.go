package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
)

func TestPlanTransferAllowsMoreThanRequirementWithinAvailableStock(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "OVERAGE-GREEN", "生产生豆", 150000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.materials SET onhand_g=150000 WHERE id=30;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=1030;`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 52.307, "target_warehouse": "wip"}}})
	var plan app.ProductionPlanDetail
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &plan) != nil || len(plan.ComponentSources) != 1 {
		t.Fatalf("plan %d %s", rec.Code, rec.Body.String())
	}
	source := plan.ComponentSources[0]
	if source.RequiredG != 65384 {
		t.Fatalf("required=%d want 65384", source.RequiredG)
	}
	endpoint := fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID)
	assertOptions := func(pending, transferable float64) {
		t.Helper()
		rec := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d/material-transfer-options?source_id=%d&warehouse=raw_materials", plan.ID, source.ID), nil)
		var body struct {
			Options []map[string]any `json:"options"`
		}
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil || len(body.Options) != 1 {
			t.Fatalf("options %d %s", rec.Code, rec.Body.String())
		}
		row := body.Options[0]
		if row["available_g"] != float64(150000) || row["pending_plan_g"] != pending || row["transferable_g"] != transferable || row["pending_plan_units"] != float64(0) || row["transferable_units"] != float64(0) {
			t.Fatalf("options expected original 150000, pending %v, transferable %v: %+v", pending, transferable, row)
		}
	}
	assertOptions(0, 150000)
	command := func(key string, qty int64) map[string]any {
		return map[string]any{"draft_token": plan.DraftToken, "idempotency_key": key, "items": []map[string]any{{"source_id": source.ID, "warehouse": "raw_materials", "batches": []map[string]any{{"batch_code": "OVERAGE-GREEN", "qty_g": qty}}}}}
	}
	wrongOwner := command("wrong-owner", 120000)
	wrongOwner["items"].([]map[string]any)[0]["owner_customer_id"] = 999
	if denied := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, wrongOwner); denied.Code == 200 {
		t.Fatalf("different owner accepted: %s", denied.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE id=1030`, schema))
	if denied := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command("held-quality", 120000)); denied.Code == 200 {
		t.Fatalf("held batch accepted: %s", denied.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='pass' WHERE id=1030`, schema))
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command("overage-120", 120000))
	var result app.PlanMaterialTransferResult
	if create.Code != 200 || json.Unmarshal(create.Body.Bytes(), &result) != nil || len(result.Documents) != 1 {
		t.Fatalf("120 kg draft should be allowed: %d %s", create.Code, create.Body.String())
	}
	assertOptions(120000, 30000)
	// A shared physical batch remains pending even when its draft belongs to
	// a different component source in the same plan.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_material_transfer_links SET component_source_id=999999 WHERE stock_entry_id=%d`, schema, result.Documents[0].ID))
	assertOptions(120000, 30000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_material_transfer_links SET component_source_id=%d WHERE stock_entry_id=%d`, schema, source.ID, result.Documents[0].ID))
	again := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command("overage-120", 120000))
	var repeat app.PlanMaterialTransferResult
	if again.Code != 200 || json.Unmarshal(again.Body.Bytes(), &repeat) != nil || len(repeat.Documents) != 1 || repeat.Documents[0].ID != result.Documents[0].ID {
		t.Fatalf("replay %d %s", again.Code, again.Body.String())
	}
	second := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command("extra-20", 20000))
	var extra app.PlanMaterialTransferResult
	if second.Code != 200 || json.Unmarshal(second.Body.Bytes(), &extra) != nil || len(extra.Documents) != 1 {
		t.Fatalf("second draft %d %s", second.Code, second.Body.String())
	}
	assertOptions(140000, 10000)
	excessive := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command("exceed-stock", 10001))
	if excessive.Code == 200 {
		t.Fatalf("overpromise accepted: %s", excessive.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='raw_materials' AND qty_g=150000", 1)
	post := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", result.Documents[0].ID), map[string]any{})
	if post.Code != 200 {
		t.Fatalf("post extra stock %d %s", post.Code, post.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=120000", 1)
	detail := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if detail.Code != 200 || json.Unmarshal(detail.Body.Bytes(), &plan) != nil {
		t.Fatalf("detail %d %s", detail.Code, detail.Body.String())
	}
	source = plan.ComponentSources[0]
	if source.RequiredG != 65384 || source.WIPCoveredG != 65384 {
		t.Fatalf("overage must not increase need/coverage %+v", source)
	}
	// Resolve the unrelated unposted draft before submitting the production
	// plan; posting 120 kg must not reserve the extra against this task.
	cancel := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/cancel", extra.Documents[0].ID), map[string]any{})
	if cancel.Code != 200 {
		t.Fatalf("cancel extra draft %d %s", cancel.Code, cancel.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	submit := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submit.Code != 200 {
		t.Fatalf("submit plan %d %s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='wip' AND reserved_g=65384 AND status='reserved'", 1)
	options, err := pgstock.NewRepository(pool, schema).ListTransferOptions(ctx, stockapp.TransferOptionsQuery{Warehouse: "wip", ItemType: "material", ItemID: 30, Limit: 100})
	if err != nil || len(options.Rows) != 1 || options.Rows[0].AvailableG != 54616 {
		t.Fatalf("excess should remain freely available in WIP: %+v err=%v", options.Rows, err)
	}
}

func TestSelectedSemiFinishedManualOveragePreservesSelectedReservation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CHOSEN-OVERAGE", "生产生豆", 26000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2030, 30, "OTHER-OVERAGE", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=2030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',20000)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	created := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	var plan app.ProductionPlanDetail
	if created.Code != 200 || json.Unmarshal(created.Body.Bytes(), &plan) != nil {
		t.Fatalf("create plan %d %s", created.Code, created.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plans SET semi_finished_stock_policy='manual' WHERE id=%[2]d;
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(%[2]d,'material:30',30,'raw_materials',1030,15000,0),(%[2]d,'material:30',30,'wip',1030,6000,0);
 INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil || len(plan.ComponentSources) != 1 {
		t.Fatalf("get %d %s", get.Code, get.Body.String())
	}
	source := plan.ComponentSources[0]
	if source.AllocationMode != "semi_finished_selected" || source.WIPCoveredG != 6000 || source.TransferG != 15000 {
		t.Fatalf("selection %+v", source)
	}
	transfer := func(code string, qty int64, key string) app.PlanMaterialTransferResult {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": key, "items": []any{map[string]any{"source_id": source.ID, "warehouse": "raw_materials", "batches": []any{map[string]any{"batch_code": code, "qty_g": qty}}}}})
		var out app.PlanMaterialTransferResult
		if r.Code != 200 || json.Unmarshal(r.Body.Bytes(), &out) != nil || len(out.Documents) != 1 {
			t.Fatalf("manual transfer %s %d %s", code, r.Code, r.Body.String())
		}
		return out
	}
	other := transfer("OTHER-OVERAGE", 5000, "other-extra")
	posted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", other.Documents[0].ID), map[string]any{})
	if posted.Code != 200 {
		t.Fatalf("post unselected extra %d %s", posted.Code, posted.Body.String())
	}
	get = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil {
		t.Fatalf("get after extra %d %s", get.Code, get.Body.String())
	}
	source = plan.ComponentSources[0]
	if source.WIPCoveredG != 6000 || source.TransferG != 15000 {
		t.Fatalf("extra batch must not substitute selection %+v", source)
	}
	chosen := transfer("CHOSEN-OVERAGE", 20000, "selected-extra")
	direct := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/semi-finished-stock/transfer-to-wip", plan.ID), map[string]any{"draft_token": plan.DraftToken, "idempotency_key": "direct-overage-draft"})
	if direct.Code == 200 {
		t.Fatal("automatic direct transfer must not post an excessive manual draft")
	}
	posted = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", chosen.Documents[0].ID), map[string]any{})
	if posted.Code != 200 {
		t.Fatalf("post selected extra %d %s", posted.Code, posted.Body.String())
	}
	get = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 || json.Unmarshal(get.Body.Bytes(), &plan) != nil {
		t.Fatalf("get after transfer %d %s", get.Code, get.Body.String())
	}
	source = plan.ComponentSources[0]
	if source.WIPCoveredG != 21000 || source.TransferG != 0 {
		t.Fatalf("selected WIP must be capped at selection %+v", source)
	}
	submitted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submitted.Code != 200 {
		t.Fatalf("submit %d %s", submitted.Code, submitted.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='wip' AND reserved_g=21000 AND status='reserved'", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=2030", 0)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=26000", 1)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=2030 AND warehouse='wip' AND qty_g=5000", 1)
}
