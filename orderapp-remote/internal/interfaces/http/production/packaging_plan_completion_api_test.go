package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	app "orderapp/internal/application/production"
	"testing"
)

func TestPieceCompletionDefersRemainderForPlan(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88921)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic',planned_minutes=18 WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET unit_cost=10,remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET unit_cost=10,remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	preview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview %d: %s", res.Code, res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview("start-preview")
	materials := p.Materials
	found := false
	for i := range materials {
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			materials[i].Qty += 1
			found = true
		}
	}
	if !found {
		t.Fatal("fixture missing weight component")
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weighed-start", "difference_reason": "称重余料验证", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d: %s", start.Code, start.Body.String())
	}
	p = preview("completion-preview")
	if !p.PlanRemainderEligible {
		t.Fatal("valid plan task should advertise unified remainder support")
	}
	if p.PlannedMinutes == nil || *p.PlannedMinutes != 18 {
		t.Fatalf("expected 18 planned minutes, got %v", p.PlannedMinutes)
	}
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 0, "remainder_mode": "plan_transfer", "actual_minutes_explicit": true}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 0)
	assertProductionFlowCount(t, pool, schema, "production_started_input_components", "remaining_qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND estimated_cost=false AND operation_cost=0", 1)
	var workCost, actualCost, outputCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT material_cost FROM %[1]s.production_batch_costs WHERE running_item_id=w.running_item_id),0)::float8, COALESCE((SELECT SUM(a.cost) FROM %[1]s.production_started_input_component_allocations a JOIN %[1]s.production_operation_events e ON e.id=a.event_id WHERE e.work_order_id=w.id),0)::float8, COALESCE((SELECT SUM(i.unit_cost*i.qty_g/1000.0) FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=w.id AND e.entry_type='finished_receipt'),0)::float8 FROM %[1]s.work_orders w WHERE w.id=$1`, schema), wo).Scan(&workCost, &actualCost, &outputCost); err != nil {
		t.Fatal(err)
	}
	if actualCost <= 0 || math.Abs(workCost-actualCost) > 0.00001 || math.Abs(outputCost-actualCost) > 0.01 {
		t.Fatalf("packaging costs work=%v allocated=%v output=%v", workCost, actualCost, outputCost)
	}
}
func TestPieceCompletionDefersRemainderAcrossPartialReports(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88922)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET unit_cost=10,remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET unit_cost=10,remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	preview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview %d: %s", res.Code, res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview("start-preview")
	materials := p.Materials
	found := false
	for i := range materials {
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			materials[i].Qty += 1
			found = true
		}
	}
	if !found {
		t.Fatal("fixture missing weight component")
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weighed-start", "difference_reason": "称重余料验证", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d: %s", start.Code, start.Body.String())
	}
	p = preview("completion-preview")
	first := map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-partial", "finish_operation": false, "actual_minutes": 0, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"}
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), first)
	if res.Code != 200 {
		t.Fatalf("partial %d: %s", res.Code, res.Body.String())
	}
	var workCost, allocatedCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT w.actual_cost::float8,COALESCE((SELECT SUM(a.cost) FROM %[1]s.production_started_input_component_allocations a JOIN %[1]s.production_operation_events e ON e.id=a.event_id WHERE e.work_order_id=w.id),0)::float8 FROM %[1]s.work_orders w WHERE w.id=$1`, schema), wo).Scan(&workCost, &allocatedCost); err != nil {
		t.Fatal(err)
	}
	if allocatedCost <= 0 || math.Abs(workCost-allocatedCost) > 0.00001 {
		t.Fatalf("partial cost work=%v allocated=%v", workCost, allocatedCost)
	}
	p = preview("completion-preview")
	command := map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 0, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 0)
	assertProductionFlowCount(t, pool, schema, "production_started_input_components", "remaining_qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 2)
}
