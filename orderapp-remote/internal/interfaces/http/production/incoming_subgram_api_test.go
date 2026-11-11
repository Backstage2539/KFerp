package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	app "orderapp/internal/application/production"
	"testing"
)

func TestIncomingSubgramPredecessorStartAndCompleteAPI(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 94881)
	route := `{"operations":[{"seq":1,"operation":"加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"reporting_type\":\"batch\",\"output_unit\":\"kg\"}"},{"seq":2,"operation":"分选","execution_json":"{\"version\":1,\"node_key\":\"sort\",\"reporting_type\":\"incoming\",\"output_unit\":\"kg\"}"},{"seq":3,"operation":"后处理","execution_json":"{\"version\":1,\"node_key\":\"last\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json='%[2]s'::jsonb WHERE id=%[3]d;
 UPDATE %[1]s.job_cards SET sequence_no=1,status='completed' WHERE work_order_id=%[3]d;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,workstation,status,planned_input_qty) VALUES(%[3]d,2,'分选','A','pending',59962),(%[3]d,3,'后处理','B','pending',59962);
 INSERT INTO %[1]s.production_operation_events(work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_qty,input_unit,output_qty,output_unit,warehouse,payload)
 SELECT %[3]d,j.id,'complete','posted','seed-subgram-'||v.n,'seed','test',v.qty,'kg',v.qty,'Kg','wip','{}' FROM %[1]s.job_cards j CROSS JOIN (VALUES(1,15::numeric),(2,15::numeric),(3,15::numeric),(4,14.961751::numeric)) AS v(n,qty) WHERE j.work_order_id=%[3]d AND j.sequence_no=1;
 INSERT INTO %[1]s.production_operation_balances(event_id,work_order_id,job_card_id,sequence_no,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost)
 SELECT id,work_order_id,job_card_id,1,'wip','Kg',output_qty,output_qty,output_qty,output_qty FROM %[1]s.production_operation_events WHERE request_id LIKE 'seed-subgram-%%' AND work_order_id=%[3]d;`, schema, route, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	preview := func() app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
		if r.Code != 200 {
			t.Fatalf("preview status %d: %s", r.Code, r.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview()
	if p.BlockedReason != "" || math.Abs(p.InputQty-59.961751) > 1e-8 || len(p.PredecessorBatches) != 4 {
		t.Fatalf("unexpected preview %+v", p)
	}
	var before int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_entries`, schema)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "start-subgram-sort"})
	if start.Code != 200 {
		t.Fatalf("start status %d: %s", start.Code, start.Body.String())
	}
	var handoff float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(qty),0)::float8 FROM %s.production_started_handoffs WHERE job_card_id=$1`, schema), card).Scan(&handoff); err != nil {
		t.Fatal(err)
	}
	if math.Abs(handoff-59.961751) > 1e-8 {
		t.Fatalf("handoff=%v", handoff)
	}
	p = preview()
	done := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 59.961751, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "waste_qty": 0, "difference_reason": "现场核对", "expected_version": p.Version, "request_id": "complete-subgram-sort"})
	if done.Code != 200 {
		t.Fatalf("complete status %d: %s", done.Code, done.Body.String())
	}
	var output, leftover float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_qty::float8 FROM %s.production_operation_events WHERE request_id='complete-subgram-sort' AND job_card_id=$1`, schema), card).Scan(&output); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(remaining_qty),0)::float8 FROM %s.production_started_handoffs WHERE job_card_id=$1`, schema), card).Scan(&leftover); err != nil {
		t.Fatal(err)
	}
	if math.Abs(output-59.961751) > 1e-8 || math.Abs(leftover) > 1e-8 {
		t.Fatalf("output=%v leftover=%v", output, leftover)
	}
	var upstreamRemaining, downstreamOutput float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(remaining_qty),0)::float8 FROM %s.production_operation_balances WHERE work_order_id=$1 AND sequence_no=1`, schema), wo).Scan(&upstreamRemaining); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(remaining_qty),0)::float8 FROM %s.production_operation_balances WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&downstreamOutput); err != nil {
		t.Fatal(err)
	}
	if math.Abs(upstreamRemaining) > 1e-8 || math.Abs(downstreamOutput-59.961751) > 1e-8 {
		t.Fatalf("upstreamRemaining=%v downstreamOutput=%v", upstreamRemaining, downstreamOutput)
	}
	// Existing balance costs follow the actual handoffs, and no cost remains
	// stranded upstream. The sort operation's own time cost is additive.
	var upstreamCost, movedCost, downstreamCost, operationCost, materialCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(remaining_cost),0)::float8 FROM %s.production_operation_balances WHERE work_order_id=$1 AND sequence_no=1`, schema), wo).Scan(&upstreamCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(m.cost),0)::float8 FROM %[1]s.production_operation_movements m JOIN %[1]s.production_operation_events e ON e.id=m.event_id WHERE e.request_id='complete-subgram-sort' AND e.job_card_id=$1`, schema), card).Scan(&movedCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_cost::float8 FROM %s.production_operation_balances WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&downstreamCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT operation_cost::float8,material_cost::float8 FROM %s.production_operation_events WHERE request_id='complete-subgram-sort' AND job_card_id=$1`, schema), card).Scan(&operationCost, &materialCost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(upstreamCost) > 1e-6 || math.Abs(movedCost-59.961751) > 1e-6 || math.Abs(downstreamCost-(movedCost+materialCost+operationCost)) > 1e-6 {
		t.Fatalf("cost upstream=%v moved=%v downstream=%v op=%v materials=%v", upstreamCost, movedCost, downstreamCost, operationCost, materialCost)
	}
	var next int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=3`, schema), wo).Scan(&next); err != nil {
		t.Fatal(err)
	}
	r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", next), nil)
	if r.Code != 200 {
		t.Fatalf("downstream preview status=%d body=%s", r.Code, r.Body.String())
	}
	var nextPreview app.OperationCompletionPreview
	if err := json.Unmarshal(r.Body.Bytes(), &nextPreview); err != nil {
		t.Fatal(err)
	}
	if len(nextPreview.PredecessorBatches) != 1 || math.Abs(nextPreview.PredecessorBatches[0].Remaining-59.961751) > 1e-8 {
		t.Fatalf("downstream handoff %+v", nextPreview.PredecessorBatches)
	}
	var after int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_entries`, schema)).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("unexpected stock entry: before=%d after=%d", before, after)
	}
}
