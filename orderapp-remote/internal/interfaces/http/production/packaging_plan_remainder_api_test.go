package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	postgres "orderapp/internal/infrastructure/postgres/production"
	"sync"
	"testing"
)

func TestPlanPackagingRemainderDoesNotRepeatReturnedStock(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88991)
	registerPlanRemainderAPI(e, app.NewService(postgres.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	var card, plan, product int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT j.id,w.production_plan_id,COALESCE(NULLIF(w.parent_product_id,0),w.product_id) FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE w.id=$1 ORDER BY j.id LIMIT 1`, schema, schema), wo).Scan(&card, &plan, &product); err != nil {
		t.Fatal(err)
	}
	preview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview %d %s", res.Code, res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview("start-preview")
	materials := p.Materials
	for i := range materials {
		if app.OperationUnitFactor(materials[i].Unit) > 0 {
			materials[i].Qty++
		}
	}
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "group-start", "difference_reason": "测试多领", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	p = preview("completion-preview")
	complete := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "group-complete", "finish_operation": true, "actual_minutes": 1, "remainder_mode": "plan_transfer"})
	if complete.Code != 200 {
		t.Fatalf("complete %d %s", complete.Code, complete.Body.String())
	}
	path := fmt.Sprintf("/api/production-plans/%d/packaging-remainders?product_id=%d", plan, product)
	get := func() app.PlanRemainderDetail {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", path, nil)
		if res.Code != 200 {
			t.Fatalf("get %d %s", res.Code, res.Body.String())
		}
		var out app.PlanRemainderDetail
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	d := get()
	// Another unfinished plan links the same source batch. Its physical
	// availability must never be treated as this plan's excess.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
      INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,product_id,product_name,status) VALUES(999991,'WO-FOREIGN-REMAINDER',%[2]d,1,'foreign','running');
      INSERT INTO %[1]s.production_started_inputs(id,work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,operator) VALUES(999992,999991,999993,'foreign',1,1,'袋','test');
      INSERT INTO %[1]s.production_started_input_components(input_id,component_key,component_type,component_id,bom_spec_id,bom_variant_id,spec_g,stock_entry_item_id,material_batch_id,stock_batch_id,warehouse,owner_customer_id,qty,remaining_qty,unit,cost,remaining_cost)
      SELECT 999992,component_key,component_type,component_id,bom_spec_id,bom_variant_id,spec_g,stock_entry_item_id,material_batch_id,stock_batch_id,warehouse,owner_customer_id,1,1,unit,0,0 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=%[3]d AND c.unit='g' LIMIT 1`, schema, plan+1000000, card))
	blocked := get()
	if blocked.CanConfirm || blocked.BlockedReason == "" {
		t.Fatalf("shared batch must block with reason: %+v", blocked)
	}
	blockedCmd := map[string]any{"product_id": product, "expected_version": blocked.Version, "request_id": "foreign-block", "target_warehouse": "wip", "sources": []any{}}
	if response := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan), blockedCmd); response.Code == 200 {
		t.Fatalf("shared foreign stock confirmation succeeded: %s", response.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET status='completed' WHERE id=999991`, schema))
	blocked = get()
	if blocked.CanConfirm || blocked.BlockedReason == "" {
		t.Fatalf("historical ownership ambiguity must block: %+v", blocked)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_started_input_components WHERE input_id=999992; DELETE FROM %[1]s.production_started_inputs WHERE id=999992; DELETE FROM %[1]s.work_orders WHERE id=999991`, schema))
	d = get()
	zeros := []app.PlanRemainderSelection{}
	var expectedPending int64
	for _, source := range d.Sources {
		zeros = append(zeros, app.PlanRemainderSelection{SourceID: source.SourceID, QuantityG: 0})
		expectedPending += source.AvailableG
	}
	zeroCmd := map[string]any{"product_id": product, "expected_version": d.Version, "request_id": "zero-with-sources", "target_warehouse": "wip", "sources": zeros}
	zeroResponse := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan), zeroCmd)
	if zeroResponse.Code != 200 {
		t.Fatalf("zero quantities: %d %s", zeroResponse.Code, zeroResponse.Body.String())
	}
	var zeroResult app.PlanRemainderResult
	if err := json.Unmarshal(zeroResponse.Body.Bytes(), &zeroResult); err != nil {
		t.Fatal(err)
	}
	if zeroResult.OutstandingG != expectedPending || len(zeroResult.StockEntryIDs) != 0 {
		t.Fatalf("zero quantities should remain pending: %+v", zeroResult)
	}
	d = get()
	var beforeUnissued int64
	for _, s := range d.Sources {
		if s.SourceType == "unissued" {
			beforeUnissued += s.AvailableG
		}
	}
	var selections []app.PlanRemainderSelection
	var issued int64
	for _, s := range d.Sources {
		q := int64(0)
		if s.SourceType == "issued" {
			q = s.AvailableG
			issued += q
		}
		selections = append(selections, app.PlanRemainderSelection{SourceID: s.SourceID, QuantityG: q})
	}
	if issued != 1000 {
		t.Fatalf("issued=%d sources=%+v", issued, d.Sources)
	}
	cmd := map[string]any{"product_id": product, "expected_version": d.Version, "request_id": "return-to-wip", "target_warehouse": "wip", "sources": selections}
	url := fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := serveMultilevelProductionJSON(t, e, "POST", url, cmd)
			codes <- res.Code
		}()
	}
	wg.Wait()
	close(codes)
	ok := 0
	for code := range codes {
		if code == 200 {
			ok++
		} else if code != 409 {
			t.Fatalf("unexpected concurrent status %d", code)
		}
	}
	if ok == 0 {
		t.Fatal("both concurrent attempts failed")
	}
	// A later retry with the same key must return the original result.
	if res := serveMultilevelProductionJSON(t, e, "POST", url, cmd); res.Code != 200 {
		t.Fatalf("retry %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_packaging_remainders", "request_id='return-to-wip'", 1)
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	after := get()
	var afterUnissued int64
	for _, s := range after.Sources {
		if s.SourceType == "unissued" {
			afterUnissued += s.AvailableG
		}
	}
	if afterUnissued > beforeUnissued {
		t.Fatalf("returned stock became claimable again: before=%d after=%d", beforeUnissued, afterUnissued)
	}
	// The unresolved portion can be confirmed in a later request. Only then
	// may this round disappear from the work area.
	nextSources := []app.PlanRemainderSelection{}
	for _, source := range after.Sources {
		nextSources = append(nextSources, app.PlanRemainderSelection{SourceID: source.SourceID, QuantityG: source.AvailableG})
	}
	next := map[string]any{"product_id": product, "expected_version": after.Version, "request_id": "finish-round", "target_warehouse": "wip", "sources": nextSources}
	if response := serveMultilevelProductionJSON(t, e, "POST", url, next); response.Code != 200 {
		t.Fatalf("finish round %d %s", response.Code, response.Body.String())
	}
	if response := serveMultilevelProductionJSON(t, e, "GET", path, nil); response.Code == 200 {
		t.Fatalf("resolved round still visible: %s", response.Body.String())
	}
}

func TestPlanPackagingZeroRemainderRecordsWithoutStockMovement(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88993)
	registerPlanRemainderAPI(e, app.NewService(postgres.NewRepository(pool, schema)))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;`, schema, wo))
	var card, plan, product int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT j.id,w.production_plan_id,COALESCE(NULLIF(w.parent_product_id,0),w.product_id) FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE w.id=$1 ORDER BY j.id LIMIT 1`, schema, schema), wo).Scan(&card, &plan, &product); err != nil {
		t.Fatal(err)
	}
	preview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview %d %s", res.Code, res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview("start-preview")
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "zero-start", "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	p = preview("completion-preview")
	complete := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "zero-complete", "finish_operation": true, "actual_minutes": 1, "remainder_mode": "plan_transfer"})
	if complete.Code != 200 {
		t.Fatalf("complete %d %s", complete.Code, complete.Body.String())
	}
	url := fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan)
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil)
	if res.Code != 200 {
		t.Fatalf("get %d %s", res.Code, res.Body.String())
	}
	var d app.PlanRemainderDetail
	if err := json.Unmarshal(res.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Sources) != 0 {
		t.Fatalf("unexpected sources %+v", d.Sources)
	}
	cmd := map[string]any{"product_id": product, "expected_version": d.Version, "request_id": "zero-remainder", "target_warehouse": "wip", "sources": []any{}}
	res = serveMultilevelProductionJSON(t, e, "POST", url, cmd)
	if res.Code != 200 {
		t.Fatalf("zero confirmation %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_packaging_remainders", "request_id='zero-remainder'", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type='packaging_plan_remainder'", 0)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "action='packaging_remainder'", 1)
	res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil)
	if res.Code == 200 {
		t.Fatalf("settled round should not be presented again: %s", res.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.job_cards(id,work_order_id,sequence_no,operation,status) VALUES(999998,%d,1,'generic','completed')`, schema, wo))
	if res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil); res.Code == 200 {
		t.Fatalf("historical completed task without plan-transfer event entered new round: %s", res.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.job_cards(id,work_order_id,sequence_no,operation,status) VALUES(999999,%d,1,'generic','pending')`, schema, wo))
	res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil)
	if res.Code != 200 {
		t.Fatalf("new round read %d %s", res.Code, res.Body.String())
	}
	var next app.PlanRemainderDetail
	if err := json.Unmarshal(res.Body.Bytes(), &next); err != nil {
		t.Fatal(err)
	}
	if next.CanConfirm || next.TotalTasks != 1 || len(next.Sources) != 0 || next.Tasks[0].JobCardID != 999999 {
		t.Fatalf("new round reused old tasks or stock: %+v", next)
	}
}
