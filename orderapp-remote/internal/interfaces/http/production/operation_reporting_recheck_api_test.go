package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	pg "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"sync"
	"testing"
)

func TestReportingRecheckPreviewConfirmAndStartedGuard(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	frozen := `{"id":77,"operations":[{"seq":1,"operation_id":77,"operation":"分选","execution_json":"{\"version\":1,\"node_key\":\"s1\",\"output_unit\":\"kg\"}"}]}`
	current := `{"version":1,"node_key":"s1","output_unit":"kg","reporting_type":"incoming"}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(77,'分选');
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(77,'路线','active');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation_id,operation,execution_json) VALUES(77,1,77,'分选','%[2]s');
 INSERT INTO %[1]s.production_plans(id,plan_no,status) VALUES(77,'RECHECK','draft');
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,process_route_id,process_route_snapshot_json) VALUES(77,77,1,77,'%[3]s');`, schema, current, frozen))
	e := newProductionFlowTestEcho(pool, schema)
	registerReportingRecheckAPI(e, app.NewService(pg.NewRepository(pool, schema)))
	path := "/api/production-plan-items/77/reporting-recheck"
	r := serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if r.Code != 200 {
		t.Fatalf("preview %d %s", r.Code, r.Body.String())
	}
	var p app.ReportingRecheck
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil || len(p.Changes) != 1 {
		t.Fatalf("preview=%s err=%v", r.Body.String(), err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", path, map[string]any{"version": "stale"})
	if r.Code == 200 {
		t.Fatal("stale accepted")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation_id,planned_qty) VALUES(77,77,1,77,50)`, schema))
	r = serveMultilevelProductionJSON(t, e, "POST", path, map[string]any{"version": p.Version})
	if r.Code == 200 || !strings.Contains(r.Body.String(), "已变化") {
		t.Fatalf("changed split did not invalidate preview: %d %s", r.Code, r.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %s.production_plan_operation_splits WHERE production_plan_item_id=77`, schema))
	r = serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_recheck_log() RETURNS trigger AS $body$ BEGIN IF NEW.action='reporting_recheck' THEN RAISE EXCEPTION 'forced audit failure'; END IF; RETURN NEW; END $body$ LANGUAGE plpgsql; CREATE TRIGGER reject_recheck_log BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_recheck_log();`, schema))
	r = serveMultilevelProductionJSON(t, e, "POST", path, map[string]any{"version": p.Version})
	if r.Code == 200 || !strings.Contains(r.Body.String(), "forced audit failure") {
		t.Fatalf("audit failure was not surfaced: %d %s", r.Code, r.Body.String())
	}
	var before string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT process_route_snapshot_json::text FROM %s.production_plan_items WHERE id=77`, schema)).Scan(&before); err != nil || strings.Contains(before, "incoming") {
		t.Fatalf("audit failure did not roll back: %s %v", before, err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DROP TRIGGER reject_recheck_log ON %s.audit_logs`, schema))
	requestBody, _ := json.Marshal(map[string]any{"version": p.Version})
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", path, bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec
		}()
	}
	wg.Wait()
	close(results)
	successful := 0
	for rec := range results {
		if rec.Code == 200 {
			successful++
		} else if !strings.Contains(rec.Body.String(), "已变化") {
			t.Fatalf("concurrent confirm: %d %s", rec.Code, rec.Body.String())
		}
	}
	if successful != 1 {
		t.Fatalf("concurrent successes %d", successful)
	}
	var saved string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT process_route_snapshot_json::text FROM %s.production_plan_items WHERE id=77`, schema)).Scan(&saved); err != nil || !strings.Contains(saved, "incoming") {
		t.Fatalf("snapshot %s err=%v", saved, err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='plan_item' AND entity_id=77 AND action='reporting_recheck'`, schema)).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit=%d err=%v", auditCount, err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET status='released' WHERE id=77`, schema))
	r = serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if r.Code == 200 {
		t.Fatal("non draft accepted")
	}
}

func TestWorkOrderReportingRecheckRequiresReplanAndProtectsStarted(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	frozen := `{"id":77,"operations":[{"seq":1,"operation_id":77,"operation":"分选","execution_json":"{\"version\":1,\"node_key\":\"s1\",\"output_unit\":\"kg\"}"}]}`
	current := `{"version":1,"node_key":"s1","output_unit":"kg","reporting_type":"incoming"}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(77,'分选');
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(77,'路线','active');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation_id,operation,execution_json) VALUES(77,1,77,'分选','%[2]s');
 INSERT INTO %[1]s.production_plans(id,plan_no,status) VALUES(77,'RECHECK-WO','submitted');
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,process_route_id,process_route_snapshot_json) VALUES(77,77,1,77,'%[3]s');
 INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,product_id,process_template_id,process_snapshot_json,status,material_consumption_policy) VALUES(77,'WO-RECHECK',77,77,1,77,'%[3]s','released','bom_start_v1');
 INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,operation_id,status,planned_input_qty) VALUES(77,77,1,'分选',77,'pending',100);
 INSERT INTO %[1]s.production_plans(id,plan_no,status) VALUES(78,'RECHECK-OTHER','submitted');
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,process_route_id,process_route_snapshot_json) VALUES(78,78,1,77,'%[3]s');
 INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,product_id,process_template_id,process_snapshot_json,status,material_consumption_policy) VALUES(78,'WO-OTHER',78,78,1,77,'%[3]s','running','bom_start_v1');
 INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,operation_id,status,planned_input_qty) VALUES(78,78,1,'分选',77,'running',100);
 INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation_id,planned_qty,planned_batch_count) VALUES(77,77,1,77,100,1);`, schema, current, frozen))
	e := newProductionFlowTestEcho(pool, schema)
	registerReportingRecheckAPI(e, app.NewService(pg.NewRepository(pool, schema)))
	path := "/api/work-orders/77/reporting-recheck"
	r := serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if r.Code != 200 {
		t.Fatalf("preview %d %s", r.Code, r.Body.String())
	}
	var p app.ReportingRecheck
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil || len(p.Changes) != 1 || !p.RequiresReplan {
		t.Fatalf("preview=%s err=%v", r.Body.String(), err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", path, map[string]any{"version": p.Version})
	if r.Code != 200 {
		t.Fatalf("confirm %d %s", r.Code, r.Body.String())
	}
	var flag bool
	var splits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((metrics_json->>'reporting_replan_required')::boolean,false) FROM %s.job_cards WHERE id=77`, schema)).Scan(&flag); err != nil || !flag {
		t.Fatalf("flag=%v err=%v", flag, err)
	}
	var other string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT process_snapshot_json::text FROM %s.work_orders WHERE id=78`, schema)).Scan(&other); err != nil || strings.Contains(other, "incoming") {
		t.Fatalf("other work order changed: %s %v", other, err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_plan_operation_splits WHERE production_plan_item_id=77`, schema)).Scan(&splits); err != nil || splits != 0 {
		t.Fatalf("splits=%d err=%v", splits, err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", "/api/job-cards/77/start", map[string]any{"request_id": "must-replan", "expected_version": "not-yet", "actual_input_qty": 100, "input_unit": "kg"})
	if r.Code == 200 || !strings.Contains(r.Body.String(), "重新安排") {
		t.Fatalf("unplanned start was not blocked: %d %s", r.Code, r.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plan_items SET planned_g=100000,planned_output_g=100000 WHERE id=77;
 UPDATE %[1]s.work_orders SET planned_g=100000,planned_output_g=100000 WHERE id=77;
 INSERT INTO %[1]s.manufacturing_workstations(id,name,status) VALUES(77,'分选站','active');
 INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_unit,scheduling_mode,throughput_per_hour) VALUES(77,77,'按量分选','active','kg','quantity',100);
 INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(77,77);`, schema))
	r = serveMultilevelProductionJSON(t, e, "POST", "/api/work-orders/77/operation-splits", map[string]any{"items": []map[string]any{{"operation_seq": 1, "operation_id": 77, "operation": "分选", "workstation_capacity_id": 77, "planned_qty": 100}}})
	if r.Code != 200 {
		t.Fatalf("replan failed: %d %s", r.Code, r.Body.String())
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.job_cards WHERE work_order_id=77 AND COALESCE((metrics_json->>'reporting_replan_required')::boolean,false)`, schema)).Scan(&splits); err != nil || splits != 0 {
		t.Fatalf("replan flag retained: %d %v", splits, err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='running' WHERE work_order_id=77`, schema))
	r = serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if r.Code == 200 {
		t.Fatal("started work order accepted")
	}
}

func TestRecheckedWorkOrderReplanThenStartsWithInventory(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, plan, wo := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 99701, "bom_start_v1")
	registerReportingRecheckAPI(e, app.NewService(pg.NewRepository(pool, schema)))
	frozen := `{"id":77,"operations":[{"seq":1,"operation_id":77,"operation":"加工","execution_json":"{\"version\":1,\"node_key\":\"p\",\"receipt_enabled\":true,\"output_unit\":\"kg\"}"}]}`
	current := `{"version":1,"node_key":"p","receipt_enabled":true,"output_unit":"kg","reporting_type":"batch"}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.manufacturing_operations(id,name) VALUES(77,'加工');
 INSERT INTO %[1]s.process_routes(id,name,status) VALUES(77,'重核路线','active');
 INSERT INTO %[1]s.process_route_operations(route_id,seq,operation_id,operation,execution_json) VALUES(77,1,77,'加工','%[2]s');
 UPDATE %[1]s.work_orders SET process_template_id=77,process_snapshot_json='%[3]s'::jsonb,operation_material_bindings='{}',material_consumption_policy='bom_start_v1' WHERE id=%[4]d;
 UPDATE %[1]s.production_plan_items SET process_route_id=77,process_route_snapshot_json='%[3]s'::jsonb WHERE production_plan_id=%[5]d;
 UPDATE %[1]s.job_cards SET operation='加工',operation_id=77 WHERE work_order_id=%[4]d;
 UPDATE %[1]s.materials SET onhand_g=22700 WHERE id=10;
 INSERT INTO %[1]s.manufacturing_workstations(id,name,status) VALUES(77,'工作站','active');
 INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes,scheduling_mode) VALUES(77,77,'整批','active',22.7,'kg',15,'batch');
 INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(77,77);`, schema, current, frozen, wo, plan))
	path := fmt.Sprintf("/api/work-orders/%d/reporting-recheck", wo)
	rec := serveMultilevelProductionJSON(t, e, "GET", path, nil)
	if rec.Code != 200 {
		t.Fatalf("preview %d %s", rec.Code, rec.Body.String())
	}
	var recheck app.ReportingRecheck
	if err := json.Unmarshal(rec.Body.Bytes(), &recheck); err != nil {
		t.Fatal(err)
	}
	rec = serveMultilevelProductionJSON(t, e, "POST", path, map[string]any{"version": recheck.Version})
	if rec.Code != 200 {
		t.Fatalf("recheck %d %s", rec.Code, rec.Body.String())
	}
	rec = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/work-orders/%d/operation-splits", wo), map[string]any{"items": []map[string]any{{"operation_seq": 1, "operation_id": 77, "operation": "加工", "workstation_capacity_id": 77, "planned_qty": 22.7}}})
	if rec.Code != 200 {
		t.Fatalf("replan %d %s", rec.Code, rec.Body.String())
	}
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	rec = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if rec.Code != 200 {
		t.Fatalf("start preview %d %s", rec.Code, rec.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" {
		t.Fatalf("blocked: %+v", p)
	}
	rec = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "rechecked-start"})
	if rec.Code != 200 {
		t.Fatalf("start %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 1)
}
