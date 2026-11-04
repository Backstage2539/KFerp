package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	app "orderapp/internal/application/production"
	productionrepo "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestBOMStartReversalRestoresExactInventoryAndCanRestart(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74750)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1' WHERE id=%[2]d; UPDATE %[1]s.job_cards SET status='pending' WHERE work_order_id=%[2]d; UPDATE %[1]s.material_batches SET unit_cost=10 WHERE material_id IN(10,20)`, schema, wo))
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1 WHERE work_order_id=%d`, schema, wo))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20); UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20); UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20); UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20)`, schema))
	var card, before, after int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card)
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(remaining_g) FROM %s.material_batches`, schema)).Scan(&before)
	preview := func() app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
		var p app.OperationCompletionPreview
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview()
	start := func(p app.OperationCompletionPreview, key string) {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": key, "materials": p.Materials})
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	start(p, "adjust-start")
	blockedCancel := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/work-orders/%d/cancel", wo), map[string]any{"note": "已开工不能直接取消"})
	if blockedCancel.Code == 200 {
		t.Fatal("started work order was cancelled without reversing its input")
	}
	p = preview()
	extra := append([]app.OperationMaterialUse(nil), p.Materials...)
	for i := range extra {
		extra[i].Qty /= 10
	}
	add := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_add", "actual_input_qty": p.InputQty / 10, "input_unit": p.InputUnit, "materials": extra, "reason": "追加投入", "request_id": "add-747", "expected_version": p.Version})
	if add.Code != 200 {
		t.Fatal(add.Body.String())
	}
	p = preview()
	if len(p.StartedInputs) != 2 {
		t.Fatalf("missing balances: %+v", p.StartedInputs)
	}
	input := p.StartedInputs[1]
	returns := []map[string]any{}
	for _, c := range input.Components {
		returns = append(returns, map[string]any{"id": c.ID, "qty": c.RemainingQty})
	}
	ret := map[string]any{"kind": "material_return", "source_input_id": input.ID, "actual_input_qty": input.RemainingQty, "returns": returns, "unprocessed_confirmed": true, "reason": "追加原料尚未使用，退回", "request_id": "return-747", "expected_version": p.Version}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), ret)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
	}
	p = preview()
	payload := map[string]any{"expected_version": p.Version, "request_id": "reverse-start", "reason": "误选任务，原料尚未投机", "unprocessed_confirmed": true}
	for i := 0; i < 2; i++ {
		r := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", card), payload)
		if r.Code != 200 {
			t.Fatalf("reverse: %d %s", r.Code, r.Body.String())
		}
	}
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(remaining_g) FROM %s.material_batches`, schema)).Scan(&after)
	if before != after {
		t.Fatalf("reversal did not restore stock: %d -> %d", before, after)
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND status='pending'", card), 1)
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d AND status='reversed' AND remaining_cost=0", card), 2)
	var net int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(deduct_g+deduct_units),0) FROM %[1]s.material_consumption_logs WHERE running_item_id=(SELECT running_item_id FROM %[1]s.work_orders WHERE id=$1)`, schema), wo).Scan(&net); err != nil {
		t.Fatal(err)
	}
	if net != 0 {
		t.Fatalf("consumption history must reflect returns, got %d", net)
	}
	ledger, err := productionrepo.NewRepository(pool, schema).ListWorkOrderLedgerEntries(ctx, app.WorkOrderLedgerQuery{WorkOrderID: wo, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	returnsFound := 0
	for _, row := range ledger {
		if row.SourceDocType == "production_input_return" {
			returnsFound++
		}
	}
	if returnsFound < 4 {
		t.Fatalf("return trace missing from work order ledger: %+v", ledger)
	}
	p = preview()
	start(p, "restart")
	p = preview()
	complete := func(key string, qty float64, finish bool) {
		t.Helper()
		p = preview()
		r := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": qty, "actual_output_qty": qty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": key, "finish_operation": finish, "actual_minutes": 1, "difference_reason": "分次生产，余料退回"})
		if r.Code != 200 {
			t.Fatal(r.Body.String())
		}
	}
	complete("half-report", 50, false)
	p = preview()
	input = p.StartedInputs[0]
	returns = nil
	for _, component := range input.Components {
		returns = append(returns, map[string]any{"id": component.ID, "qty": component.RemainingQty / 2})
	}
	returned := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_return", "source_input_id": input.ID, "actual_input_qty": 25, "returns": returns, "unprocessed_confirmed": true, "reason": "订单减量，未投入余料退回", "request_id": "partial-return", "expected_version": p.Version})
	if returned.Code != 200 {
		t.Fatal(returned.Body.String())
	}
	p = preview()
	blocked := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", card), map[string]any{"reason": "已有报工不得撤销", "unprocessed_confirmed": true, "request_id": "bad-reverse", "expected_version": p.Version})
	if blocked.Code == 200 {
		t.Fatal("reported batch reversed")
	}
	complete("finish-report", 25, true)
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d AND remaining_qty=0 AND remaining_cost=0", card), 3)
	var outputCost, netCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.total_cost),0)::float8 FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.source_type='operation_completion' AND e.entry_type='finished_receipt'`, schema)).Scan(&outputCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(material_cost+operation_cost)::float8 FROM %s.production_operation_events WHERE work_order_id=$1 AND status='posted'`, schema), wo).Scan(&netCost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(outputCost-netCost) > 0.0001 {
		t.Fatalf("adjusted output cost=%v net input/operation cost=%v", outputCost, netCost)
	}
}
