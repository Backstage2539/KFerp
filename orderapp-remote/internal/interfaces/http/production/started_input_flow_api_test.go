package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	"testing"
)

func TestBOMStartConsumesOnceAndCompletionOnlyAllocates(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74710)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=10 WHERE material_id IN(10,20)`, schema))
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, woID))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), woID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if res.Code != 200 {
		t.Fatalf("start preview: %d %s", res.Code, res.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" || len(p.Materials) == 0 {
		t.Fatalf("preview missing inputs: %+v", p)
	}
	half := append([]app.OperationMaterialUse(nil), p.Materials...)
	for i := range half {
		half[i].Qty /= 2
	}
	noReason := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty / 2, "input_unit": p.InputUnit, "materials": half, "expected_version": p.Version, "request_id": "different-input-without-reason"})
	if noReason.Code == 200 {
		t.Fatal("changed batch input accepted without reason")
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 0)
	payload := map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "start-747", "materials": p.Materials}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
	if res.Code == 200 {
		t.Fatal("new policy accepted completion before start")
	}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", card), payload)
		if res.Code != 200 {
			t.Fatalf("start %d: %d %s", i, res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 1)
	var consumedBefore int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(consumed_g),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), woID).Scan(&consumedBefore); err != nil {
		t.Fatal(err)
	}
	if consumedBefore <= 0 {
		t.Fatal("start did not consume")
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	payload["expected_version"] = p.Version
	payload["request_id"] = "finish-747"
	payload["actual_input_qty"] = p.InputQty
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
	if res.Code != 200 {
		t.Fatalf("complete: %d %s", res.Code, res.Body.String())
	}
	var consumedAfter int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(consumed_g),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), woID).Scan(&consumedAfter); err != nil {
		t.Fatal(err)
	}
	if consumedAfter != consumedBefore {
		t.Fatalf("completion consumed twice: %d -> %d", consumedBefore, consumedAfter)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d AND remaining_qty=0 AND remaining_cost=0", card), 1)
	var allocated, sourceCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT SUM(cost) FROM %s.production_started_input_allocations),0)::float8,COALESCE((SELECT SUM(material_cost) FROM %s.production_started_inputs),0)::float8`, schema, schema)).Scan(&allocated, &sourceCost); err != nil {
		t.Fatal(err)
	}
	if sourceCost <= 0 {
		t.Fatal("fixture must have nonzero input cost")
	}
	if allocated != sourceCost {
		t.Fatalf("cost not conserved: %v/%v", allocated, sourceCost)
	}
	records := serveMultilevelProductionJSON(t, e, "GET", "/api/production/records", nil)
	var recordBody struct {
		Rows []app.OperationRecord `json:"rows"`
	}
	if err := json.Unmarshal(records.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if len(recordBody.Rows) != 1 {
		t.Fatal(records.Body.String())
	}
	corrected := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", card), map[string]any{"actual_output_qty": 98, "reason": "复点", "expected_version": recordBody.Rows[0].Version, "request_id": "start-cost-correction"})
	if corrected.Code != 200 {
		t.Fatal(corrected.Body.String())
	}
	var batchCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8 FROM %s.stock_batches WHERE item_type='finished_product' AND qty_units=98 ORDER BY id DESC LIMIT 1`, schema)).Scan(&batchCost); err != nil {
		t.Fatal(err)
	}
	if batchCost <= 0 {
		t.Fatalf("correction lost started input cost: %v", batchCost)
	}

}

func TestBOMStartTwoBatchesUseFrozenSequenceAndPartialReports(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74720)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g) VALUES(747,'PR747-B','水洗生豆','bean','kg','kg',10000);
 UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',bom_version_id=0,operation_material_bindings='{}',output_qty=8,output_unit='kg',inventory_unit='kg',planned_inventory_qty=8,planned_output_g=8000,planned_g=10000,
 material_snapshot='[{"material_id":10,"material_name":"日晒生豆","unit":"kg","ratio_pct":30,"source":"bom","component_type":"material"},{"material_id":747,"material_name":"水洗生豆","unit":"kg","ratio_pct":70,"source":"bom","component_type":"material"}]',
 process_snapshot_json='{"operations":[{"seq":10,"operation":"首道通用工序","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":false,\"output_unit\":\"kg\"}"},{"seq":20,"operation":"色选","execution_json":"{\"version\":1,\"node_key\":\"second\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"receipt_warehouse\":\"wip\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=10,planned_input_qty=5000,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,10,'首道通用工序','pending',5000),(%[2]d,20,'色选','pending',10000);
 UPDATE %[1]s.work_order_material_reservations SET status='released' WHERE work_order_id=%[2]d AND material_id<>10;
 INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,component_type,component_id,material_name,unit,required_g,reserved_g,source_warehouse,status) VALUES(%[2]d,747,'material',747,'水洗生豆','kg',7000,0,'wip','reserved');
 `, schema, woID))
	// This scenario models pre-existing cards with historical split reports.
	// The fixture rewrites the newly created card's process and quantity by SQL;
	// discard the automatic records created for the original, different route.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_execution_batches WHERE job_card_id IN (SELECT id FROM %[1]s.job_cards WHERE work_order_id=%[2]d)`, schema, woID))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 74721, 747, "PR747-WASHED", "水洗生豆", 10000)
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id`, schema), woID)
	if err != nil {
		t.Fatal(err)
	}
	var cards []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		cards = append(cards, id)
	}
	rows.Close()
	preview := func(id int64, kind string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s-preview", id, kind), nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if p.BlockedReason != "" {
			t.Fatalf("blocked: %+v", p)
		}
		return p
	}
	post := func(id int64, action, key string, p app.OperationCompletionPreview, input, output float64, finish bool) {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/%s", id, action), map[string]any{"actual_input_qty": input, "actual_output_qty": output, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": key, "materials": p.Materials, "finish_operation": finish, "difference_reason": "分锅实测"})
		if res.Code != 200 {
			t.Fatalf("%s: %s", action, res.Body.String())
		}
	}
	for n, card := range cards[:2] {
		p := preview(card, "start")
		if len(p.Materials) != 2 || p.Materials[0].Qty != 1.5 || p.Materials[1].Qty != 3.5 {
			t.Fatalf("batch materials: %+v", p.Materials)
		}
		post(card, "start", fmt.Sprintf("pot-%d", n), p, 5, 4, false)
		assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=10 AND consumed_g=%d", woID, (n+1)*1500), 1)
		p = preview(card, "completion")
		post(card, "complete", fmt.Sprintf("pot-%d-part", n), p, 2, 1.6, false)
		p = preview(card, "completion")
		if p.InputQty != 3 {
			t.Fatalf("remaining input=%v", p.InputQty)
		}
		post(card, "complete", fmt.Sprintf("pot-%d-final", n), p, 3, 2.4, true)
	}
	p := preview(cards[2], "start")
	if len(p.Materials) != 0 || !p.PredecessorRequired {
		t.Fatalf("downstream cannot consume BOM: %+v", p)
	}
	post(cards[2], "start", "sort-start", p, 4, 4, false)
	p = preview(cards[2], "completion")
	if p.InputQty != 4 || p.StartedInputQty != 4 {
		t.Fatalf("downstream must process its own confirmed batch, got input=%v balance=%v", p.InputQty, p.StartedInputQty)
	}
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", cards[2]), map[string]any{"reason": "确认数量误填，尚未加工", "unprocessed_confirmed": true, "expected_version": p.Version, "request_id": "sort-reverse"})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	p = preview(cards[2], "start")
	post(cards[2], "start", "sort-restart", p, 8, 8, false)
	p = preview(cards[2], "completion")
	post(cards[2], "complete", "sort-complete", p, 8, 8, true)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=10 AND consumed_g=3000", woID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=747 AND consumed_g=7000", woID), 1)
}

func TestBOMStartRatioRoundingConsumesExactlyOneGram(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74730)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g) VALUES(747,'PR747-TINY','第二种生豆','bean','kg','kg',10000);
 UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',bom_version_id=0,operation_material_bindings='{}',output_qty=0.001,output_unit='kg',inventory_unit='kg',planned_inventory_qty=0.001,planned_output_g=1,planned_g=1,
 material_snapshot='[{"material_id":10,"material_name":"第一种生豆","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material"},{"material_id":747,"material_name":"第二种生豆","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material"}]',
 process_snapshot_json='{"operations":[{"seq":10,"operation":"加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"kg\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=10,planned_input_qty=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 UPDATE %[1]s.work_order_material_reservations SET status='released' WHERE work_order_id=%[2]d AND material_id<>10;
 INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,component_type,component_id,material_name,unit,required_g,reserved_g,source_warehouse,status) VALUES(%[2]d,747,'material',747,'第二种生豆','kg',1,0,'wip','reserved');
 `, schema, woID))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 74731, 747, "PR747-TINY-B", "第二种生豆", 10000)
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), woID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &p) != nil || p.BlockedReason != "" || len(p.Materials) != 2 {
		t.Fatalf("preview: %d %s", res.Code, res.Body.String())
	}
	if p.Materials[0].Qty+p.Materials[1].Qty != 0.001 {
		t.Fatalf("rounded preview does not total one gram: %+v", p.Materials)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "pr747-one-gram"})
	if res.Code != 200 {
		t.Fatalf("start: %d %s", res.Code, res.Body.String())
	}
	var grams int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(consumed_g),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), woID).Scan(&grams); err != nil || grams != 1 {
		t.Fatalf("physical stock consumed %d grams: %v", grams, err)
	}
}
