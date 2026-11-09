package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	app "orderapp/internal/application/production"
)

func TestPieceCompletionReturnsWeighedSurplusAtomically(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88911)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceCompletionCanReportPartialThenReturnSurplus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88912)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	first := map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-partial", "finish_operation": false, "actual_minutes": 1, "actual_remainder_g": 12350, "remainder_warehouse": "wip"}
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), first)
	if res.Code != 200 {
		t.Fatalf("partial %d: %s", res.Code, res.Body.String())
	}
	p = preview("completion-preview")
	command := map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 2)
}

func TestPieceCompletionReturnsMultipleInputSurplus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88913)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	materials := append([]app.OperationMaterialUse(nil), p.Materials...)
	found := false
	for i := range materials {
		materials[i].Qty /= 2
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture missing weight component")
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": 50, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weighed-start", "difference_reason": "称重余料验证", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d: %s", start.Code, start.Body.String())
	}
	p = preview("start-preview")
	extra := append([]app.OperationMaterialUse(nil), materials...)
	for i := range extra {
		if app.OperationUnitFactor(extra[i].Unit) > 0 {
			extra[i].Qty += 1
		}
	}
	add := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_add", "actual_input_qty": 50, "input_unit": p.InputUnit, "materials": extra, "reason": "追加投入", "request_id": "weighed-add", "expected_version": p.Version})
	if add.Code != 200 {
		t.Fatalf("add %d: %s", add.Code, add.Body.String())
	}
	p = preview("completion-preview")
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceCompletionTransfersWeighedSurplus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88914)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	var beforeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&beforeG); err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "raw_materials"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	var afterG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&afterG); err != nil {
		t.Fatal(err)
	}
	if afterG-beforeG != 1000 {
		t.Fatalf("transfer delta %d", afterG-beforeG)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type='packaging_remainder' AND status='submitted'", 1)
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceCompletionTransfersMultipleInputSurplus(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88917)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	materials := append([]app.OperationMaterialUse(nil), p.Materials...)
	found := false
	for i := range materials {
		materials[i].Qty /= 2
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("fixture missing weight component")
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": 50, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weighed-start", "difference_reason": "称重余料验证", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d: %s", start.Code, start.Body.String())
	}
	p = preview("start-preview")
	extra := append([]app.OperationMaterialUse(nil), materials...)
	for i := range extra {
		if app.OperationUnitFactor(extra[i].Unit) > 0 {
			extra[i].Qty += 1
		}
	}
	add := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_add", "actual_input_qty": 50, "input_unit": p.InputUnit, "materials": extra, "reason": "追加投入", "request_id": "weighed-add", "expected_version": p.Version})
	if add.Code != 200 {
		t.Fatalf("add %d: %s", add.Code, add.Body.String())
	}
	p = preview("completion-preview")
	var beforeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&beforeG); err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "raw_materials"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	var afterG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&afterG); err != nil {
		t.Fatal(err)
	}
	if afterG-beforeG != 1000 {
		t.Fatalf("transfer delta %d", afterG-beforeG)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceCompletionRejectsChangedOwnerAtomically(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88918)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_started_input_components SET owner_customer_id=999 WHERE input_id IN(SELECT id FROM %[1]s.production_started_inputs WHERE job_card_id=%[2]d) AND unit='g'`, schema, card))
	p = preview("completion-preview")
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}

	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
	if res.Code == 200 {
		t.Fatal("changed owner accepted")
	}
	if !strings.Contains(res.Body.String(), "货主") {
		t.Fatalf("wrong failure: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_started_input_component_allocations", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
}

func TestPieceCompletionPreservesMultipleBatches(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88919)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET qty_g=22700,remaining_g=22700 WHERE material_id=10; UPDATE %[1]s.material_batch_locations SET qty_g=22700 WHERE material_id=10 AND warehouse='wip'; UPDATE %[1]s.stock_batches SET qty_g=22700,remaining_g=22700 WHERE item_type='material' AND item_id=10; UPDATE %[1]s.materials SET onhand_g=24700 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 889199, 10, "MB-PIECE-SECOND", "熟豆", 2000)
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
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	var remaining int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=889199 AND warehouse='wip'`, schema)).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1000 {
		t.Fatalf("second batch remaining=%d", remaining)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceCompletionMovesOnlyExplicitSharedBatchQuantity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88920)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	if len(p.PackagingUnissuedCandidates) == 0 {
		t.Fatal("missing traceable shared batch candidate")
	}
	candidate := p.PackagingUnissuedCandidates[0]
	if candidate.AvailableG < 500 {
		t.Fatalf("candidate unavailable: %+v", candidate)
	}
	var beforeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&beforeG); err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "raw_materials"}
	command["selected_unissued_remainders"] = []map[string]any{{"batch_code": candidate.BatchCode, "warehouse": candidate.Warehouse, "owner_customer_id": candidate.OwnerCustomerID, "quantity_g": 500}}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	var afterG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&afterG); err != nil {
		t.Fatal(err)
	}
	if afterG-beforeG != 1500 {
		t.Fatalf("transfer delta %d", afterG-beforeG)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type IN ('packaging_remainder','packaging_unissued_remainder') AND status='submitted'", 2)
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
}

func TestPieceSharedStockHonorsOtherWorkOrderReservation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88922)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	if len(p.PackagingUnissuedCandidates) == 0 {
		t.Fatal("missing traceable shared batch candidate")
	}
	candidate := p.PackagingUnissuedCandidates[0]
	var other, res, batch int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.work_orders(work_order_no,status) VALUES('WO-OTHER','released') RETURNING id`, schema)).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.material_batches WHERE batch_code=$1`, schema), candidate.BatchCode).Scan(&batch); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,material_name,unit,component_type,component_id,source_warehouse,required_g,reserved_g,status) VALUES($1,10,'熟豆','kg','material',10,$2,500,500,'reserved') RETURNING id`, schema), other, candidate.Warehouse).Scan(&res); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,reserved_g,status) VALUES($1,$2,10,'material',10,$3,$4,$5,500,'reserved')`, schema), res, other, batch, candidate.BatchCode, candidate.Warehouse); err != nil {
		t.Fatal(err)
	}
	fresh := preview("completion-preview")
	if len(fresh.PackagingUnissuedCandidates) == 0 || fresh.PackagingUnissuedCandidates[0].AvailableG != candidate.AvailableG-500 {
		t.Fatalf("other reservation not protected: before=%+v after=%+v", candidate, fresh.PackagingUnissuedCandidates)
	}
	stale := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "stale-shared", "finish_operation": true, "actual_remainder_g": 1000, "remainder_warehouse": "raw_materials", "selected_unissued_remainders": []map[string]any{{"batch_code": candidate.BatchCode, "warehouse": candidate.Warehouse, "owner_customer_id": candidate.OwnerCustomerID, "quantity_g": candidate.AvailableG}}}
	failed := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), stale)
	if failed.Code == 200 {
		t.Fatal("stale shared stock was accepted")
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
	p = fresh
	candidate = p.PackagingUnissuedCandidates[0]
	if candidate.AvailableG < 500 {
		t.Fatalf("candidate unavailable: %+v", candidate)
	}
	var beforeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&beforeG); err != nil {
		t.Fatal(err)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "raw_materials"}
	command["selected_unissued_remainders"] = []map[string]any{{"batch_code": candidate.BatchCode, "warehouse": candidate.Warehouse, "owner_customer_id": candidate.OwnerCustomerID, "quantity_g": 500}}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	var afterG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE warehouse='raw_materials' AND material_id=10`, schema)).Scan(&afterG); err != nil {
		t.Fatal(err)
	}
	if afterG-beforeG != 1500 {
		t.Fatalf("transfer delta %d", afterG-beforeG)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type IN ('packaging_remainder','packaging_unissued_remainder') AND status='submitted'", 2)
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
	var held int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT reserved_g-consumed_g-returned_g FROM %s.work_order_material_reservation_batches WHERE reservation_id=$1`, schema), res).Scan(&held); err != nil {
		t.Fatal(err)
	}
	if held != 500 {
		t.Fatalf("other reservation changed: %d", held)
	}
}

func TestPieceCompletionAuditFailureRollsBackEverything(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88923)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
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
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT reject_packaging_audit CHECK(action <> 'packaging_remainder')`, schema))
	p = preview("completion-preview")
	if p.PackagingAvailableG != 23700 || p.PackagingStandardGPerPiece != 227 {
		t.Fatalf("unexpected packaging preview: available=%v rate=%v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	command := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}

	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
	if res.Code == 200 {
		t.Fatal("audit failure accepted")
	}
	if !strings.Contains(res.Body.String(), "reject_packaging_audit") {
		t.Fatalf("wrong failure: %s", res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_started_input_component_allocations", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
}

func TestPieceCompletionUsesWeightAcrossInputSources(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88924)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=10 WHERE material_id=10`, schema))
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
	materials := append([]app.OperationMaterialUse(nil), p.Materials...)
	found := false
	for i := range materials {
		materials[i].Qty /= 2
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			materials[i].Qty = 0.1
			found = true
		}
	}
	if !found {
		t.Fatal("fixture missing weight component")
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": 50, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weighed-start", "difference_reason": "称重余料验证", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d: %s", start.Code, start.Body.String())
	}
	p = preview("start-preview")
	extra := append([]app.OperationMaterialUse(nil), materials...)
	for i := range extra {
		if app.OperationUnitFactor(extra[i].Unit) > 0 {
			extra[i].Qty = 23.6
		}
	}
	add := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_add", "actual_input_qty": 50, "input_unit": p.InputUnit, "materials": extra, "reason": "追加投入", "request_id": "weighed-add", "expected_version": p.Version})
	if add.Code != 200 {
		t.Fatalf("add %d: %s", add.Code, add.Body.String())
	}
	p = preview("completion-preview")
	first := map[string]any{"actual_input_qty": 2, "actual_output_qty": 2, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "cross-source-first", "finish_operation": false, "actual_remainder_g": 23246, "remainder_warehouse": "wip"}
	firstResult := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), first)
	if firstResult.Code != 200 {
		t.Fatalf("cross-source partial %d: %s", firstResult.Code, firstResult.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_allocations", "qty=0 AND cost>0", 1)
	p = preview("completion-preview")
	reversal := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start-reversal", card), map[string]any{"request_id": "cross-source-reverse", "expected_version": p.Version, "reason": "验证来源撤销保护", "unprocessed_confirmed": true})
	if reversal.Code == 200 || !strings.Contains(reversal.Body.String(), "已有报工") {
		t.Fatalf("invalid reversal response: %s", reversal.Body.String())
	}
	p = preview("completion-preview")
	command := map[string]any{"actual_input_qty": 98, "actual_output_qty": 98, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "weighed-complete", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), command)
		if res.Code != 200 {
			t.Fatalf("completion %d: %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 2)
	records := serveMultilevelProductionJSON(t, e, "GET", "/api/production/records", nil)
	var recordBody struct {
		Rows []app.OperationRecord `json:"rows"`
	}
	if err := json.Unmarshal(records.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	var firstEvent int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_operation_events WHERE request_id='cross-source-first'`, schema)).Scan(&firstEvent); err != nil {
		t.Fatal(err)
	}
	version := ""
	for _, r := range recordBody.Rows {
		if r.ID == firstEvent {
			version = r.Version
		}
	}
	if version == "" {
		t.Fatalf("missing source event: %s", records.Body.String())
	}
	correction := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", card), map[string]any{"event_id": firstEvent, "actual_output_qty": 0, "reason": "复核件数", "request_id": "cross-source-correct", "expected_version": version})
	if correction.Code == 200 || !strings.Contains(correction.Body.String(), "成本") {
		t.Fatalf("zero-output cost correction must fail: %s", correction.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='correct' AND status='posted'", 0)
	var qty, allocated float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT output_qty FROM %[1]s.production_operation_events WHERE id=$1)::float8,(SELECT COALESCE(SUM(cost),0) FROM %[1]s.production_started_input_allocations WHERE event_id=$1)::float8`, schema), firstEvent).Scan(&qty, &allocated); err != nil {
		t.Fatal(err)
	}
	if qty != 2 || allocated <= 0 {
		t.Fatalf("failed correction changed quantity/cost: %v %v", qty, allocated)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_allocations", "qty=0 AND cost>0", 1)
	for _, corrected := range []float64{1, 101} {
		correction = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", card), map[string]any{"event_id": firstEvent, "actual_output_qty": corrected, "reason": "复核件数", "request_id": fmt.Sprintf("cross-source-correct-%g", corrected), "expected_version": version})
		if correction.Code == 200 || !strings.Contains(correction.Body.String(), "冲销") {
			t.Fatalf("quantity correction should fail: %s", correction.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='correct' AND status='posted'", 0)
	var perKG float64
	var weight int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT b.unit_cost::float8,b.remaining_g FROM %[1]s.stock_batches b JOIN %[1]s.stock_entry_items i ON i.batch_code=b.batch_code JOIN %[1]s.stock_entries d ON d.id=i.stock_entry_id WHERE d.source_type='operation_completion' AND d.source_id=$1 LIMIT 1`, schema), firstEvent).Scan(&perKG, &weight); err != nil {
		t.Fatal(err)
	}
	if weight != 454 || math.Abs(perKG*float64(weight)/1000-allocated) > 0.001 {
		t.Fatalf("rejected correction changed receipt cost: weight=%d unit=%v source=%v", weight, perKG, allocated)
	}
}
