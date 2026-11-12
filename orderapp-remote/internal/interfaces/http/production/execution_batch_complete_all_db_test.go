package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestExecutionBatchesCompleteAllAtomicReplay(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 94837)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 94838, 10, "TEST-59KG", "日晒生豆", 60000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.materials SET onhand_g=82700 WHERE id=10;
 UPDATE %[1]s.material_batches SET unit_cost=10 WHERE material_id=10;
 UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',bom_version_id=0,operation_material_bindings='{}',output_qty=48,output_unit='kg',inventory_unit='kg',planned_inventory_qty=48,planned_output_g=48000,planned_g=59962,
 material_snapshot='[{"material_id":10,"material_name":"日晒生豆","unit":"kg","ratio_pct":100,"source":"bom","component_type":"material"}]',
 process_snapshot_json='{"operations":[{"seq":10,"operation":"首道通用工序","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"receipt_warehouse\":\"wip\"}"}]}' WHERE id=%[2]d;
 DELETE FROM %[1]s.production_execution_batches WHERE job_card_id IN (SELECT id FROM %[1]s.job_cards WHERE work_order_id=%[2]d);
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=10,planned_input_qty=59962,actual_input_qty=0,actual_output_qty=0,completed_at=NULL,scheduling_mode='batch',batch_size_qty=15,batch_size_unit='kg',planned_batch_count=4 WHERE work_order_id=%[2]d;
 UPDATE %[1]s.work_order_material_reservations SET status='released' WHERE work_order_id=%[2]d AND material_id<>10;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("/api/job-cards/%d", card)
	// Opening a legacy task previews its frozen batch arrangement without
	// creating execution records or posting any stock movement.
	prestart := serveMultilevelProductionJSON(t, e, "GET", prefix+"/execution-batches", nil)
	if prestart.Code != 200 { t.Fatal(prestart.Body.String()) }
	var prestartList app.ExecutionBatchList
	if err := json.Unmarshal(prestart.Body.Bytes(), &prestartList); err != nil || len(prestartList.PreviewBatches) != 4 || prestartList.PreviewBatches[3].PlannedInputQty != 14.962 {
		t.Fatalf("pre-start preview: %s %v", prestart.Body.String(), err)
	}
	assertProductionFlowCount(t, pool, schema, "production_execution_batches", fmt.Sprintf("job_card_id=%d", card), 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d", card), 0)
	preview := func(kind string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", prefix+"/"+kind+"-preview", nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.BlockedReason != "" {
			t.Fatal(p.BlockedReason)
		}
		return p
	}
	p := preview("start")
	start := serveMultilevelProductionJSON(t, e, "POST", prefix+"/start", map[string]any{"request_id": "start-batches", "expected_version": p.Version, "input_unit": p.InputUnit, "actual_input_qty": p.InputQty, "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("start %s", start.Body.String())
	}
	var consumed int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(consumed_g) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), wo).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	var initialCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(material_cost)::float8 FROM %s.production_started_inputs WHERE job_card_id=$1`, schema), card).Scan(&initialCost); err != nil {
		t.Fatal(err)
	}
	if initialCost <= 0 {
		t.Fatal("expected nonzero started input cost")
	}
	res := serveMultilevelProductionJSON(t, e, "GET", prefix+"/execution-batches", nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var list app.ExecutionBatchList
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches", map[string]any{"expected_version": list.Version, "request_id": "initialize-batches"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil || len(list.Batches) != 4 {
		t.Fatalf("%s %v", res.Body.String(), err)
	}
	p = preview("completion")
	batches := make([]map[string]any, 4)
	for i, b := range list.Batches {
		batches[i] = map[string]any{"execution_batch_id": b.ID, "actual_input_qty": b.PlannedInputQty, "actual_output_qty": 12.0, "actual_minutes": 0, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "difference_reason": "现场数量"}
	}
	batches[1]["actual_output_qty"] = 0.0
	batches[1]["difference_reason"] = ""
	request := map[string]any{"expected_version": p.Version, "request_id": "complete-all-atomic", "batches": batches}
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request)
	if res.Code == 200 {
		t.Fatal("invalid second batch accepted")
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 0)
	batches[1]["actual_output_qty"] = 12.0
	// A bulk child's deterministic key must never overwrite an existing single
	// completion receipt, even when a client deliberately reuses the prefix.
	childKey := fmt.Sprintf("complete-all-conflict:%d", list.Batches[0].ID)
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_request_keys(action,actor,request_id,payload_hash,result_id,result_json) VALUES('operation_complete','测试员',$1,'old-payload',999,'{"old":true}')`, schema), childKey)
	if err != nil {
		t.Fatal(err)
	}
	request["request_id"] = "complete-all-conflict"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request)
	if res.Code == 200 {
		t.Fatal("bulk accepted colliding individual completion key")
	}
	var oldResult int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT result_id FROM %s.production_request_keys WHERE action='operation_complete' AND actor='测试员' AND request_id=$1`, schema), childKey).Scan(&oldResult); err != nil || oldResult != 999 {
		t.Fatalf("existing receipt changed: %d %v", oldResult, err)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 0)
	request["request_id"] = "complete-all-success"
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request)
		if res.Code != 200 {
			t.Fatalf("complete all attempt %d: %s", i, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 4)
	var seconds int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM((payload->>'actual_seconds')::int) FROM %s.production_operation_events WHERE job_card_id=$1 AND kind='complete'`, schema), card).Scan(&seconds); err != nil || seconds != 0 {
		t.Fatalf("explicit zero seconds=%d err=%v", seconds, err)
	}
	var after int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(consumed_g) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), wo).Scan(&after); err != nil || after != consumed {
		t.Fatalf("consumption changed %d -> %d err=%v", consumed, after, err)
	}
}

func TestExecutionBatchesCompleteAllAfterSingle(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 94847)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 94848, 10, "TEST-59KG", "日晒生豆", 60000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.materials SET onhand_g=82700 WHERE id=10;
 UPDATE %[1]s.material_batches SET unit_cost=10 WHERE material_id=10;
 UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',bom_version_id=0,operation_material_bindings='{}',output_qty=48,output_unit='kg',inventory_unit='kg',planned_inventory_qty=48,planned_output_g=48000,planned_g=59962,
 material_snapshot='[{"material_id":10,"material_name":"日晒生豆","unit":"kg","ratio_pct":100,"source":"bom","component_type":"material"}]',
 process_snapshot_json='{"operations":[{"seq":10,"operation":"首道通用工序","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"receipt_warehouse\":\"wip\"}"}]}' WHERE id=%[2]d;
 DELETE FROM %[1]s.production_execution_batches WHERE job_card_id IN (SELECT id FROM %[1]s.job_cards WHERE work_order_id=%[2]d);
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=10,planned_input_qty=59962,actual_input_qty=0,actual_output_qty=0,completed_at=NULL,scheduling_mode='batch',batch_size_qty=15,batch_size_unit='kg',planned_batch_count=4 WHERE work_order_id=%[2]d;
 UPDATE %[1]s.work_order_material_reservations SET status='released' WHERE work_order_id=%[2]d AND material_id<>10;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("/api/job-cards/%d", card)
	preview := func(kind string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", prefix+"/"+kind+"-preview", nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.BlockedReason != "" {
			t.Fatal(p.BlockedReason)
		}
		return p
	}
	p := preview("start")
	start := serveMultilevelProductionJSON(t, e, "POST", prefix+"/start", map[string]any{"request_id": "start-batches", "expected_version": p.Version, "input_unit": p.InputUnit, "actual_input_qty": p.InputQty, "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("start %s", start.Body.String())
	}
	var consumed int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(consumed_g) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), wo).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	var initialCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(material_cost)::float8 FROM %s.production_started_inputs WHERE job_card_id=$1`, schema), card).Scan(&initialCost); err != nil {
		t.Fatal(err)
	}
	if initialCost <= 0 {
		t.Fatal("expected nonzero started input cost")
	}
	res := serveMultilevelProductionJSON(t, e, "GET", prefix+"/execution-batches", nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var list app.ExecutionBatchList
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches", map[string]any{"expected_version": list.Version, "request_id": "initialize-batches"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil || len(list.Batches) != 4 {
		t.Fatalf("%s %v", res.Body.String(), err)
	}
	p = preview("completion")
	single := map[string]any{"expected_version": p.Version, "request_id": "first-single", "execution_batch_id": list.Batches[0].ID, "actual_input_qty": 15, "actual_output_qty": 12, "actual_minutes": 0, "actual_minutes_explicit": true, "input_unit": p.InputUnit, "output_unit": p.OutputUnit}
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", single)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	p = preview("completion")
	batches := make([]map[string]any, 0, 3)
	for _, b := range list.Batches[1:] {
		batches = append(batches, map[string]any{"execution_batch_id": b.ID, "actual_input_qty": b.PlannedInputQty, "actual_output_qty": 12.0, "actual_minutes": 18, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "difference_reason": "现场数量"})
	}
	request := map[string]any{"expected_version": p.Version, "request_id": "incomplete-list", "batches": batches[:2]}
	if r := serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request); r.Code == 200 {
		t.Fatal("incomplete pending set accepted")
	}
	request["batches"] = batches
	request["request_id"] = "stale-version"
	request["expected_version"] = "stale"
	if r := serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request); r.Code == 200 {
		t.Fatal("stale version accepted")
	}
	request["request_id"] = "remaining-3"
	request["expected_version"] = p.Version
	if r := serveMultilevelProductionJSON(t, e, "POST", prefix+"/execution-batches/complete-all", request); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 4)
	var after int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(consumed_g) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), wo).Scan(&after); err != nil || after != consumed {
		t.Fatalf("consumption changed %d -> %d err=%v", consumed, after, err)
	}
}
