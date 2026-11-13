package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	app "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
)

func TestStartedPackagingCanUseWIPWithoutIncreasingPieces(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88950)
	registerWIPComponentTopUpAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	endpoint := fmt.Sprintf("/api/job-cards/%d/", card)
	getPreview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", endpoint+path, nil)
		if r.Code != 200 {
			t.Fatalf("preview %s: %d %s", path, r.Code, r.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := getPreview("start-preview")
	materials := p.Materials
	found := false
	for i := range materials {
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			materials[i].Qty = math.Round(materials[i].Qty*400) / 1000
			found = true
		}
	}
	if !found {
		t.Fatal("missing weight component")
	}
	r := serveMultilevelProductionJSON(t, e, "POST", endpoint+"start", map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "topup-start", "difference_reason": "先投部分", "materials": materials})
	if r.Code != 200 {
		t.Fatalf("start: %d %s", r.Code, r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "GET", endpoint+"wip-top-up", nil)
	if r.Code != 200 {
		t.Fatalf("top-up preview: %d %s", r.Code, r.Body.String())
	}
	var top struct {
		Version   string `json:"version"`
		Supported bool   `json:"supported"`
		Items     []struct {
			Key        string  `json:"key"`
			Shortage   float64 `json:"shortage_qty"`
			Candidates []struct {
				Batch           string  `json:"batch_code"`
				Warehouse       string  `json:"warehouse"`
				Owner           int64   `json:"owner_customer_id"`
				Available       float64 `json:"available_qty"`
				UnitCost        float64 `json:"unit_cost"`
				CostUnavailable bool    `json:"cost_unavailable"`
			} `json:"candidates"`
		} `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	if !top.Supported || len(top.Items) != 1 || len(top.Items[0].Candidates) == 0 || top.Items[0].Shortage <= 0 {
		t.Fatalf("missing eligible shortage: %+v", top)
	}
	item := top.Items[0]
	batch := item.Candidates[0]
	if batch.UnitCost != 0 || batch.CostUnavailable {
		t.Fatalf("explicit zero cost must not be treated as unknown: %+v", batch)
	}
	if batch.Available+1e-8 < item.Shortage {
		t.Fatalf("fixture insufficient available %v < %v", batch.Available, item.Shortage)
	}
	cmd := map[string]any{"expected_version": top.Version, "request_id": "topup-600", "materials": []map[string]any{{"key": item.Key, "batch_code": batch.Batch, "warehouse": batch.Warehouse, "owner_customer_id": batch.Owner, "qty": item.Shortage}}}
	for i := 0; i < 2; i++ {
		r = serveMultilevelProductionJSON(t, e, "POST", endpoint+"wip-top-up", cmd)
		if r.Code != 200 {
			t.Fatalf("top-up %d: %d %s", i, r.Code, r.Body.String())
		}
	}
	p = getPreview("completion-preview")
	if math.Abs(p.StartedInputQty-100) > 1e-8 {
		t.Fatalf("pieces changed: %v", p.StartedInputQty)
	}
	if p.PackagingAvailableG+1e-8 < 100*p.PackagingStandardGPerPiece {
		t.Fatalf("still short: %v < %v", p.PackagingAvailableG, 100*p.PackagingStandardGPerPiece)
	}
	seenMaterial := 0
	for _, m := range p.StartedMaterials {
		if m.Key == item.Key {
			seenMaterial++
			if math.Abs(m.Qty-item.Shortage/0.6) > 1e-6 {
				t.Fatalf("display does not include top-up: %+v", m)
			}
		}
	}
	if seenMaterial != 1 {
		t.Fatalf("expected one combined displayed material, got %d: %+v", seenMaterial, p.StartedMaterials)
	}
	var actualBatch string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT a.batch_code FROM %[1]s.stock_entry_batch_allocations a JOIN %[1]s.stock_entry_items i ON i.id=a.stock_entry_item_id JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id JOIN %[1]s.production_operation_events v ON v.id=e.source_id WHERE v.job_card_id=$1 AND v.kind='material_add'`, schema), card).Scan(&actualBatch); err != nil {
		t.Fatal(err)
	}
	if actualBatch != batch.Batch {
		t.Fatalf("selected batch %s, issued %s", batch.Batch, actualBatch)
	}
	complete := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "topup-complete", "finish_operation": true, "actual_minutes": 0, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"}
	r = serveMultilevelProductionJSON(t, e, "POST", endpoint+"complete", complete)
	if r.Code != 200 {
		t.Fatalf("completion: %d %s", r.Code, r.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='material_add' AND status='posted'", 1)
}

func TestStartedOneBagUsesOwnReservedAndFreeWIP(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88970)
	registerWIPComponentTopUpAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=1,output_unit='袋',inventory_unit='袋',planned_inventory_qty=1,planned_g=1000,planned_output_g=1000,spec_g=1000,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
	 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic',planned_input_qty=1000 WHERE work_order_id=%[2]d;
	 UPDATE %[1]s.work_order_material_reservations SET required_g=1000,reserved_g=450 WHERE work_order_id=%[2]d AND material_id=10;
	 UPDATE %[1]s.work_order_material_reservations SET required_units=1,reserved_units=1 WHERE work_order_id=%[2]d AND material_id=20;
	 UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=450 WHERE work_order_id=%[2]d AND material_id=10;
	 UPDATE %[1]s.work_order_material_reservation_batches SET reserved_units=1 WHERE work_order_id=%[2]d AND material_id=20;
	 UPDATE %[1]s.material_batches SET remaining_g=450,qty_g=450,unit_cost=80 WHERE id=88970;
	 UPDATE %[1]s.material_batch_locations SET qty_g=450 WHERE material_batch_id=88970 AND warehouse='wip';
	 UPDATE %[1]s.materials SET onhand_g=1450 WHERE id=10;`, schema, wo))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 88972, 10, "MB-EXTRA-88972", "在制熟豆", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,qty_g,remaining_g,cost_unavailable) VALUES('MB-EXTRA-88972','material',10,'在制熟豆',1000,1000,true)`, schema))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/api/job-cards/%d/", card)
	load := func(path string) app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", base+path, nil)
		if r.Code != 200 {
			t.Fatalf("%s %d: %s", path, r.Code, r.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load("start-preview")
	materials := p.Materials
	for i := range materials {
		if materials[i].MaterialID == 10 {
			materials[i].Qty = 0.4
		}
	}
	r := serveMultilevelProductionJSON(t, e, "POST", base+"start", map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "one-start", "difference_reason": "先投部分", "materials": materials})
	if r.Code != 200 {
		t.Fatalf("start %d: %s", r.Code, r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "GET", base+"wip-top-up", nil)
	if r.Code != 200 {
		t.Fatalf("top-up preview %d: %s", r.Code, r.Body.String())
	}
	var preview struct {
		Version string `json:"version"`
		Items   []struct {
			Key        string  `json:"key"`
			Shortage   float64 `json:"shortage_qty"`
			Candidates []struct {
				Batch           string  `json:"batch_code"`
				Warehouse       string  `json:"warehouse"`
				Owner           int64   `json:"owner_customer_id"`
				Available       float64 `json:"available_qty"`
				CostUnavailable bool    `json:"cost_unavailable"`
			} `json:"candidates"`
		} `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Items) != 1 || math.Abs(preview.Items[0].Shortage-.6) > 1e-8 {
		t.Fatalf("want 0.6 kg shortage: %s", r.Body.String())
	}
	item := preview.Items[0]
	selected := []map[string]any{}
	for _, b := range item.Candidates {
		if b.Batch == "MB-ROASTED-88970" {
			if math.Abs(b.Available-.05) > 1e-8 {
				t.Fatalf("own reservation: %+v", b)
			}
			selected = append(selected, map[string]any{"key": item.Key, "batch_code": b.Batch, "warehouse": b.Warehouse, "owner_customer_id": b.Owner, "qty": .05})
		}
		if b.Batch == "MB-EXTRA-88972" {
			if !b.CostUnavailable {
				t.Fatal("mirror unknown cost must be visible before use")
			}
			selected = append(selected, map[string]any{"key": item.Key, "batch_code": b.Batch, "warehouse": b.Warehouse, "owner_customer_id": b.Owner, "qty": .55})
		}
	}
	if len(selected) != 2 {
		t.Fatalf("want both source batches: %s", r.Body.String())
	}
	cmd := map[string]any{"expected_version": preview.Version, "request_id": "one-topup", "materials": selected}
	for i := 0; i < 2; i++ {
		r = serveMultilevelProductionJSON(t, e, "POST", base+"wip-top-up", cmd)
		if r.Code != 200 {
			t.Fatalf("top-up %d %d: %s", i, r.Code, r.Body.String())
		}
	}
	p = load("completion-preview")
	if math.Abs(p.StartedInputQty-1) > 1e-8 || math.Abs(p.PackagingAvailableG-1000) > 1e-8 {
		t.Fatalf("pieces=%v grams=%v", p.StartedInputQty, p.PackagingAvailableG)
	}
	for _, m := range p.StartedMaterials {
		if m.MaterialID == 10 && math.Abs(m.Qty-1) > 1e-8 {
			t.Fatalf("display total: %+v", m)
		}
	}
	complete := map[string]any{"actual_input_qty": 1, "actual_output_qty": 1, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "one-complete", "finish_operation": true, "actual_minutes": 1, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"}
	r = serveMultilevelProductionJSON(t, e, "POST", base+"complete", complete)
	if r.Code != 200 {
		t.Fatalf("complete %d: %s", r.Code, r.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='material_add' AND status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND COALESCE((payload->>'cost_unavailable')::boolean,false)", 1)
	var oldQty, newQty, actualCost, orderCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(a.qty) FILTER(WHERE c.material_batch_id=88970),0)::float8,COALESCE(SUM(a.qty) FILTER(WHERE c.material_batch_id=88972),0)::float8,COALESCE(SUM(a.cost),0)::float8,(SELECT actual_cost::float8 FROM %[1]s.work_orders WHERE id=$1) FROM %[1]s.production_started_input_component_allocations a JOIN %[1]s.production_started_input_components c ON c.id=a.component_id JOIN %[1]s.production_operation_events e ON e.id=a.event_id WHERE e.work_order_id=$1`, schema), wo).Scan(&oldQty, &newQty, &actualCost, &orderCost); err != nil {
		t.Fatal(err)
	}
	if oldQty != 450 || newQty != 550 || math.Abs(actualCost-36) > 1e-6 || math.Abs(orderCost-actualCost) > 1e-6 {
		t.Fatalf("source or cost mismatch: old=%v new=%v allocated=%v work=%v", oldQty, newQty, actualCost, orderCost)
	}
}

func TestStartedWeightOperationCanUseWIPWithoutIncreasingInput(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88990)
	registerWIPComponentTopUpAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=22.7,output_unit='kg',inventory_unit='kg',planned_inventory_qty=22.7,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"reporting_type\":\"batch\"}"}]}' WHERE id=%[2]d;
	 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic',planned_input_qty=22700 WHERE work_order_id=%[2]d;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/api/job-cards/%d/", card)
	load := func(path string) app.OperationCompletionPreview {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, "GET", base+path, nil)
		if r.Code != 200 {
			t.Fatalf("%s: %d %s", path, r.Code, r.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load("start-preview")
	materials := p.Materials
	for i := range materials {
		if materials[i].MaterialID == 10 {
			materials[i].Qty = 9.08
		}
	}
	r := serveMultilevelProductionJSON(t, e, "POST", base+"start", map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "weight-start", "difference_reason": "先投部分", "materials": materials})
	if r.Code != 200 {
		t.Fatalf("start %d: %s", r.Code, r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "GET", base+"wip-top-up", nil)
	if r.Code != 200 {
		t.Fatalf("preview %d: %s", r.Code, r.Body.String())
	}
	var q struct {
		Version   string `json:"version"`
		Supported bool   `json:"supported"`
		Items     []struct {
			Key        string  `json:"key"`
			Shortage   float64 `json:"shortage_qty"`
			Candidates []struct {
				Batch     string `json:"batch_code"`
				Warehouse string `json:"warehouse"`
				Owner     int64  `json:"owner_customer_id"`
			} `json:"candidates"`
		} `json:"items"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if !q.Supported || len(q.Items) != 1 || math.Abs(q.Items[0].Shortage-13.62) > 1e-8 || len(q.Items[0].Candidates) == 0 {
		t.Fatalf("expected 13.62kg shortage: %s", r.Body.String())
	}
	i := q.Items[0]
	b := i.Candidates[0]
	r = serveMultilevelProductionJSON(t, e, "POST", base+"wip-top-up", map[string]any{"expected_version": q.Version, "request_id": "weight-topup", "materials": []map[string]any{{"key": i.Key, "batch_code": b.Batch, "warehouse": b.Warehouse, "owner_customer_id": b.Owner, "qty": i.Shortage}}})
	if r.Code != 200 {
		t.Fatalf("top-up %d: %s", r.Code, r.Body.String())
	}
	p = load("completion-preview")
	if math.Abs(p.StartedInputQty-22.7) > 1e-8 {
		t.Fatalf("input changed: %v", p.StartedInputQty)
	}
	var executionID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_execution_batches WHERE job_card_id=$1 ORDER BY id LIMIT 1`, schema), card).Scan(&executionID); err != nil {
		t.Fatal(err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", base+"complete", map[string]any{"actual_input_qty": 22.7, "actual_output_qty": 22.7, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "execution_batch_id": executionID, "request_id": "weight-complete", "finish_operation": true, "actual_minutes": 1, "actual_minutes_explicit": true})
	if r.Code != 200 {
		t.Fatalf("complete %d: %s", r.Code, r.Body.String())
	}
}

func TestWIPTopUpDoesNotChangePredecessorHandoffs(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 89020)
	registerWIPComponentTopUpAPI(e, app.NewService(postgresproduction.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"output_unit\":\"kg\",\"reporting_type\":\"batch\"}"},{"seq":2,"operation":"later","execution_json":"{\"version\":1,\"node_key\":\"later\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
	UPDATE %[1]s.job_cards SET status='running',sequence_no=2,operation='later',planned_input_qty=22700 WHERE work_order_id=%[2]d;
	INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'generic','completed',22700);`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=2`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	base := fmt.Sprintf("/api/job-cards/%d/", card)
	r := serveMultilevelProductionJSON(t, e, "GET", base+"wip-top-up", nil)
	if r.Code != 200 {
		t.Fatalf("preview %d: %s", r.Code, r.Body.String())
	}
	var preview struct {
		Supported     bool   `json:"supported"`
		BlockedReason string `json:"blocked_reason"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Supported || preview.BlockedReason == "" {
		t.Fatalf("must explain predecessor-only input: %s", r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "POST", base+"wip-top-up", map[string]any{"expected_version": "forged", "request_id": "blocked-predecessor", "materials": []map[string]any{{"key": "material:10:0:0", "batch_code": "MB-ROASTED-89020", "warehouse": "wip", "qty": 1}}})
	if r.Code == 200 {
		t.Fatalf("predecessor top-up must be blocked: %s", r.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='material_add'", 0)
	assertProductionFlowCount(t, pool, schema, "production_started_handoffs", "qty>0", 0)
}
