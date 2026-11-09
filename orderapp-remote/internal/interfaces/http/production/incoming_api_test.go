package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	"sync"
	"testing"
)

func TestIncomingModeMultiStationAndWasteAPI(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 78001)
	route := `{"operations":[{"seq":1,"operation":"加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"output_unit\":\"kg\"}"},{"seq":2,"operation":"分选","execution_json":"{\"version\":1,\"node_key\":\"sort\",\"reporting_type\":\"incoming\",\"output_unit\":\"g\"}"},{"seq":3,"operation":"后处理","execution_json":"{\"version\":1,\"node_key\":\"last\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json='%[2]s'::jsonb WHERE id=%[3]d;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,workstation,status,planned_input_qty) VALUES(%[3]d,2,'分选','A','pending',60000),(%[3]d,2,'分选','B','pending',40000),(%[3]d,3,'后处理','C','pending',100000);
 INSERT INTO %[1]s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,warehouse,payload)
 SELECT %[3]d,id,'complete','posted','seed','seed','test',100,'kg',84,'kg','wip','{"cost_unavailable":true}' FROM %[1]s.job_cards WHERE work_order_id=%[3]d AND sequence_no=1;
 INSERT INTO %[1]s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost)
 SELECT id,work_order_id,job_card_id,1,'wip','kg',84,84,100,100 FROM %[1]s.production_operation_events WHERE work_order_id=%[3]d AND request_id='seed';`, schema, route, wo))
	var a, b int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id),MAX(id) FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&a, &b); err != nil {
		t.Fatal(err)
	}
	preview := func(id int64) app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", id), nil)
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview(a)
	if p.BlockedReason != "" || p.InputQty != 50.4 || len(p.IncomingAllocations) != 2 {
		t.Fatalf("preview %+v", p)
	}
	start := map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "start-sort-a"}
	startBody, _ := json.Marshal(start)
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/job-cards/%d/start", a), bytes.NewReader(startBody))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			results <- rec
		}()
	}
	wg.Wait()
	close(results)
	for rec := range results {
		if rec.Code != 200 {
			t.Fatalf("concurrent start: %s", rec.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_handoffs", fmt.Sprintf("job_card_id=%d", a), 1)
	p = preview(b)
	if p.InputQty != 33.6 {
		t.Fatalf("second allocation %+v", p)
	}
	r := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", b), map[string]any{"actual_input_qty": p.InputQty + 1, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "overclaim"})
	if r.Code == 200 {
		t.Fatal("overclaim accepted")
	}
	p = preview(a)
	missing := map[string]any{"actual_input_qty": 50.4, "input_unit": "kg", "output_unit": "g", "expected_version": p.Version, "request_id": "missing-waste"}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", a), missing)
	if r.Code == 200 {
		t.Fatal("missing waste accepted")
	}
	invalid := map[string]any{"actual_input_qty": 50.4, "input_unit": "kg", "output_unit": "g", "waste_qty": 51, "difference_reason": "废料", "expected_version": p.Version, "request_id": "bad-waste"}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", a), invalid)
	if r.Code == 200 {
		t.Fatal("excess waste accepted")
	}
	invalid["waste_qty"] = 1
	invalid["request_id"] = "valid-waste"
	for i := 0; i < 2; i++ {
		r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", a), invalid)
		if r.Code != 200 {
			t.Fatalf("complete %d: %s", i, r.Body.String())
		}
	}
	var output, loss float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT actual_output_qty::float8,actual_loss_qty::float8 FROM %s.job_cards WHERE id=$1`, schema), a).Scan(&output, &loss); err != nil {
		t.Fatal(err)
	}
	if output != 49400 || loss != 1 {
		t.Fatalf("got output=%v loss=%v", output, loss)
	}
	var inherited bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((payload->>'cost_unavailable')::boolean,false) FROM %s.production_operation_events WHERE request_id='valid-waste'`, schema)).Scan(&inherited); err != nil {
		t.Fatal(err)
	}
	if !inherited {
		t.Fatal("unknown upstream cost did not follow handoff")
	}
}
