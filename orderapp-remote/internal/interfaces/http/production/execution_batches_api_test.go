package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"math"
	app "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"sync"
	"testing"
)

func TestExecutionBatchesInitializeAndReplay(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, workID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 94817)
	registerProductionWorkstationAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), workID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	// New task creation creates exactly its own batch, even when the
	// surrounding order has multiple independently scheduled cards.
	var autoCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.production_execution_batches WHERE job_card_id=$1`, schema), id).Scan(&autoCount); err != nil || autoCount != 1 {
		t.Fatalf("automatic batch count=%d err=%v", autoCount, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_execution_batches WHERE job_card_id=$1`, schema), id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET scheduling_mode='batch',planned_input_qty=59962,batch_size_qty=15,batch_size_unit='kg',planned_batch_count=4 WHERE id=$1`, schema), id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1' WHERE id=$1`, schema), workID); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("/api/job-cards/%d/execution-batches", id)
	// A legacy aggregate without event rows must remain in its original mode.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=1 WHERE id=$1`, schema), id); err != nil {
		t.Fatal(err)
	}
	old := serveMultilevelProductionJSON(t, e, "GET", url, nil)
	var legacy app.ExecutionBatchList
	if old.Code != 200 {
		t.Fatal(old.Body.String())
	}
	if err := json.Unmarshal(old.Body.Bytes(), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.CanInitialize || legacy.Reason == "" {
		t.Fatalf("legacy aggregate not preserved: %+v", legacy)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET actual_output_qty=0 WHERE id=$1`, schema), id); err != nil {
		t.Fatal(err)
	}

	res := serveMultilevelProductionJSON(t, e, "GET", url, nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var list app.ExecutionBatchList
	if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if !list.CanInitialize {
		t.Fatalf("%+v", list)
	}
	payload := map[string]any{"expected_version": list.Version, "request_id": "batch-init-test"}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, "POST", url, payload)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Batches) != 4 {
			t.Fatalf("%+v", list)
		}
	}
	unauthorized := echo.New()
	registerProductionWorkstationAPI(unauthorized, app.NewService(postgresproduction.NewRepository(pool, schema)))
	denied := serveMultilevelProductionJSON(t, unauthorized, "POST", url, payload)
	if denied.Code == 200 {
		t.Fatal("unbound user initialized execution batches")
	}
	for i, want := range []float64{15, 15, 15, 14.962} {
		if math.Abs(list.Batches[i].PlannedInputQty-want) > 0.000001 {
			t.Fatalf("batch %d input=%v want=%v", i+1, list.Batches[i].PlannedInputQty, want)
		}
	}
	conflict := serveMultilevelProductionJSON(t, e, "POST", url, map[string]any{"expected_version": "different", "request_id": "batch-init-test"})
	if conflict.Code == 200 {
		t.Fatal("same idempotency key with different version accepted")
	}
	assertProductionFlowCount(t, pool, schema, "production_execution_batches", fmt.Sprintf("job_card_id=%d", id), 4)
}

func TestExecutionBatchesCompleteUsesStartedInputOnce(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 94827)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 94828, 10, "TEST-59KG", "日晒生豆", 60000)
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
	payload := map[string]any{"expected_version": p.Version, "request_id": "missing-output", "actual_input_qty": 15, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "execution_batch_id": list.Batches[0].ID}
	noBatch := map[string]any{"expected_version": p.Version, "request_id": "legacy-bypass", "actual_input_qty": 15, "actual_output_qty": 12, "input_unit": p.InputUnit, "output_unit": p.OutputUnit}
	if attempt := serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", noBatch); attempt.Code == 200 {
		t.Fatal("old completion bypassed execution batch")
	}

	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code == 200 {
		t.Fatal("missing output accepted")
	}
	payload["actual_output_qty"] = 0
	payload["request_id"] = "zero-output"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code == 200 {
		t.Fatal("zero without reason accepted")
	}
	payload["difference_reason"] = "全部损耗"
	payload["request_id"] = "zero-output-reason"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code != 200 {
		t.Fatalf("zero with reason: %s", res.Body.String())
	}
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code != 200 {
		t.Fatalf("idempotent batch replay: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 1)
	p = preview("completion")
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q := map[string]any{"expected_version": p.Version, "execution_batch_id": list.Batches[1].ID, "actual_input_qty": 15, "actual_output_qty": 12, "request_id": fmt.Sprintf("concurrent-%d", i), "difference_reason": "分批生产", "input_unit": p.InputUnit, "output_unit": p.OutputUnit}
			statuses <- serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", q).Code
		}(i)
	}
	wg.Wait()
	close(statuses)
	success := 0
	for status := range statuses {
		if status == 200 {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent success count=%d", success)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='complete'", card), 2)
	p = preview("completion")
	payload["expected_version"] = p.Version
	payload["execution_batch_id"] = list.Batches[2].ID
	payload["actual_input_qty"] = 15
	payload["actual_output_qty"] = 12
	payload["request_id"] = "batch-3"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code != 200 {
		t.Fatalf("batch 3: %s", res.Body.String())
	}

	p = preview("completion")
	payload["expected_version"] = p.Version
	payload["execution_batch_id"] = list.Batches[3].ID
	payload["actual_input_qty"] = 14.462
	payload["actual_output_qty"] = 1.6
	payload["request_id"] = "last-with-residue"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code == 200 {
		t.Fatal("last batch silently left input balance")
	}
	payload["actual_input_qty"] = 14.962
	payload["request_id"] = "last-batch"
	res = serveMultilevelProductionJSON(t, e, "POST", prefix+"/complete", payload)
	if res.Code != 200 {
		t.Fatalf("last batch: %s", res.Body.String())
	}
	var after int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(consumed_g) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), wo).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != consumed {
		t.Fatalf("consumed twice: %d -> %d", consumed, after)
	}
	var allocatedCost, remainingCost, remainingQty float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(a.cost),0)::float8 FROM %[1]s.production_started_input_allocations a JOIN %[1]s.production_started_inputs i ON i.id=a.input_id WHERE i.job_card_id=$1`, schema), card).Scan(&allocatedCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(remaining_cost)::float8,SUM(remaining_qty)::float8 FROM %s.production_started_inputs WHERE job_card_id=$1`, schema), card).Scan(&remainingCost, &remainingQty); err != nil {
		t.Fatal(err)
	}
	if math.Abs(allocatedCost-initialCost) > 0.00001 || remainingCost != 0 || remainingQty != 0 {
		t.Fatalf("cost mismatch: initial=%v allocated=%v remaining=%v qty=%v", initialCost, allocatedCost, remainingCost, remainingQty)
	}
	assertProductionFlowCount(t, pool, schema, "production_execution_batches", fmt.Sprintf("job_card_id=%d AND status='completed'", card), 4)
}
