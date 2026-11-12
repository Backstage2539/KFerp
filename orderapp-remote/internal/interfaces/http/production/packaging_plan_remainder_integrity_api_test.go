package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/production"
	postgres "orderapp/internal/infrastructure/postgres/production"
)

// The stock and event queries below verify the same transaction as the HTTP request,
// using only the isolated schema created by newProductionFlowTestDB.
func TestPlanPackagingRemainderCostAndAuditAtomicity(t *testing.T) {
	for _, tc := range []struct {
		name         string
		auditFailure bool
		target       string
	}{{"return_cost_and_idempotency", false, "wip"}, {"audit_failure_rolls_back", true, "wip"}, {"transfer_to_raw_materials", false, "raw_materials"}, {"concurrent_different_keys", false, "wip"}, {"new_round_reissue", false, "wip"}, {"external_recheck", false, "wip"}, {"external_shortfall", false, "wip"}, {"second_outflow", false, "wip"}} {
		auditFailure := tc.auditFailure
		name := tc.name
		t.Run(name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88994)
			registerPlanRemainderAPI(e, app.NewService(postgres.NewRepository(pool, schema)))
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*3,onhand_units=onhand_units*3 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET unit_cost=10,remaining_g=remaining_g*3,qty_g=qty_g*3,remaining_units=remaining_units*3,qty_units=qty_units*3 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*3,qty_units=qty_units*3 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET unit_cost=10,remaining_g=remaining_g*3,qty_g=qty_g*3,remaining_units=remaining_units*3,qty_units=qty_units*3 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
			var card, plan, product int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT j.id,w.production_plan_id,COALESCE(NULLIF(w.parent_product_id,0),w.product_id) FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE w.id=$1 ORDER BY j.id LIMIT 1`, schema, schema), wo).Scan(&card, &plan, &product); err != nil {
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
			for i := range materials {
				if app.OperationUnitFactor(materials[i].Unit) > 0 {
					materials[i].Qty++
				} else {
					materials[i].Qty += 3
				}
			}
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "integrity-start", "difference_reason": "验证余料", "materials": materials})
			if res.Code != 200 {
				t.Fatalf("start %d: %s", res.Code, res.Body.String())
			}
			p = preview("completion-preview")
			res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "integrity-complete", "finish_operation": true, "actual_minutes": 0, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"})
			if res.Code != 200 {
				t.Fatalf("complete %d: %s", res.Code, res.Body.String())
			}
			var beforeCost, beforeRemaining, beforeStock float64
			err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT material_cost FROM %[1]s.production_batch_costs WHERE running_item_id=w.running_item_id),0)::float8,
 COALESCE((SELECT SUM(remaining_cost) FROM %[1]s.production_started_inputs WHERE work_order_id=w.id),0)::float8,
 COALESCE((SELECT SUM(qty_g) FROM %[1]s.material_batch_locations),0)::float8 FROM %[1]s.work_orders w WHERE w.id=$1`, schema), wo).Scan(&beforeCost, &beforeRemaining, &beforeStock)
			if err != nil {
				t.Fatal(err)
			}
			if beforeCost <= 0 || beforeRemaining <= 0 {
				t.Fatalf("expected non-zero cost and remainder: cost=%v remainder=%v", beforeCost, beforeRemaining)
			}
			url := fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan)
			res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil)
			if res.Code != 200 {
				t.Fatalf("get %d: %s", res.Code, res.Body.String())
			}
			var detail app.PlanRemainderDetail
			if err = json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
				t.Fatal(err)
			}
			var selections []app.PlanRemainderSelection
			var issued int64
			for _, s := range detail.Sources {
				q := int64(0)
				if s.SourceType == "issued" {
					q = s.AvailableG
					issued += q
				}
				selections = append(selections, app.PlanRemainderSelection{SourceID: s.SourceID, QuantityG: q})
			}
			if tc.name == "new_round_reissue" {
				for i := range selections {
					selections[i].QuantityG = detail.Sources[i].AvailableG
				}
			}
			if issued != 1000 {
				t.Fatalf("issued=%d sources=%+v", issued, detail.Sources)
			}
			cmd := map[string]any{"product_id": product, "expected_version": detail.Version, "request_id": "integrity-return", "target_warehouse": tc.target, "sources": selections}
			if auditFailure {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT integrity_reject_packaging_audit CHECK(action <> 'packaging_remainder')`, schema))
			}
			if tc.name == "concurrent_different_keys" {
				results := make(chan int, 2)
				ready := make(chan struct{})
				var wg sync.WaitGroup
				for i := 0; i < 2; i++ {
					wg.Add(1)
					go func(index int) {
						defer wg.Done()
						body := map[string]any{"product_id": product, "expected_version": detail.Version, "request_id": fmt.Sprintf("concurrent-%d", index), "target_warehouse": tc.target, "sources": selections}
						<-ready
						results <- serveMultilevelProductionJSON(t, e, "POST", url, body).Code
					}(i)
				}
				close(ready)
				wg.Wait()
				close(results)
				successes := 0
				for code := range results {
					if code == 200 {
						successes++
					}
				}
				if successes != 1 {
					t.Fatalf("concurrent requests: expected exactly one success, got %d", successes)
				}
			} else {
				res = serveMultilevelProductionJSON(t, e, "POST", url, cmd)
			}
			if auditFailure {
				if res.Code < 400 {
					t.Fatalf("expected failed audit, got %d %s", res.Code, res.Body.String())
				}
			} else if tc.name != "concurrent_different_keys" {
				if res.Code != 200 {
					t.Fatalf("confirm %d: %s", res.Code, res.Body.String())
				}
				again := serveMultilevelProductionJSON(t, e, "POST", url, cmd)
				if again.Code != 200 {
					t.Fatalf("retry %d: %s", again.Code, again.Body.String())
				}
			}
			var afterCost, afterRemaining, afterStock, returnEventCost float64
			err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((SELECT material_cost FROM %[1]s.production_batch_costs WHERE running_item_id=w.running_item_id),0)::float8,
 COALESCE((SELECT SUM(remaining_cost) FROM %[1]s.production_started_inputs WHERE work_order_id=w.id),0)::float8,
 COALESCE((SELECT SUM(qty_g) FROM %[1]s.material_batch_locations),0)::float8,
 COALESCE((SELECT SUM(material_cost) FROM %[1]s.production_operation_events WHERE work_order_id=w.id AND kind='material_return' AND status='posted'),0)::float8
 FROM %[1]s.work_orders w WHERE w.id=$1`, schema), wo).Scan(&afterCost, &afterRemaining, &afterStock, &returnEventCost)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(afterCost-beforeCost) > 0.00001 {
				t.Fatalf("historical production cost changed: %v -> %v", beforeCost, afterCost)
			}
			if auditFailure {
				if afterStock != beforeStock || afterRemaining != beforeRemaining || returnEventCost != 0 {
					t.Fatalf("failed transaction changed state: stock %v->%v remaining %v->%v return event %v", beforeStock, afterStock, beforeRemaining, afterRemaining, returnEventCost)
				}
				assertProductionFlowCount(t, pool, schema, "production_packaging_remainders", "true", 0)
				assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "true", 0)
			} else {
				if afterStock-beforeStock != 1000 || afterRemaining >= beforeRemaining || math.Abs((beforeRemaining-afterRemaining)+returnEventCost) > 0.00001 {
					t.Fatalf("cost/stock not conserved: stock %v->%v remaining cost %v->%v returned event %v", beforeStock, afterStock, beforeRemaining, afterRemaining, returnEventCost)
				}
				assertProductionFlowCount(t, pool, schema, "production_packaging_remainders", "true", 1)
				assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
				assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=3", 1)
				if tc.name == "new_round_reissue" {
					verifyPlanRemainderRealReissue(t, ctx, pool, schema, e, wo, plan, product)
				}
				if tc.name == "external_recheck" || tc.name == "external_shortfall" || tc.name == "second_outflow" {
					verifyRemainderExternalRecheck(t, ctx, pool, schema, e, wo, plan, product, cmd, tc.name != "external_recheck", tc.name == "second_outflow")
				}
				if tc.target == "raw_materials" {
					assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type='packaging_plan_remainder' AND status='submitted'", 1)
				} else {
					assertProductionFlowCount(t, pool, schema, "stock_entries", "source_type='packaging_plan_remainder'", 0)
				}
			}
		})
	}
}

func verifyPlanRemainderRealReissue(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, e http.Handler, oldWO, plan, product int64) {
	t.Helper()
	var wo, card int64
	err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.work_orders(
 work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,
 output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,
 product_id,parent_product_id,bom_spec_id,bom_variant_id,product_name,spec_g,inventory_unit,planned_inventory_qty,
 planned_g,planned_output_g,status,material_snapshot,bom_version_id,operation_template_id,
 process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
 production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
 sales_spec_snapshot_json,operation_material_bindings,material_consumption_policy)
 SELECT 'WO-PLAN-REISSUE',0,production_plan_id,production_plan_item_id,batch_id,
 output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,
 product_id,parent_product_id,bom_spec_id,bom_variant_id,product_name,spec_g,inventory_unit,planned_inventory_qty,
 planned_g,planned_output_g,'released',material_snapshot,bom_version_id,operation_template_id,
 process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
 production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
 sales_spec_snapshot_json,operation_material_bindings,material_consumption_policy FROM %[1]s.work_orders WHERE id=$1 RETURNING id`, schema), oldWO).Scan(&wo)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty,actual_input_qty,actual_output_qty,status)
 SELECT $2,sequence_no,operation,workstation,planned_input_qty,0,0,'pending' FROM %[1]s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1 RETURNING id`, schema), oldWO, wo).Scan(&card)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservations(
 work_order_id,running_item_id,material_id,material_name,unit,required_g,required_units,
 reserved_g,reserved_units,consumed_g,consumed_units,returned_g,returned_units,status,
 source_warehouse,source_owner_customer_id,component_type,component_id,
 component_bom_spec_id,component_bom_variant_id,component_spec_g)
 SELECT $2,0,material_id,material_name,unit,required_g,required_units,required_g,required_units,0,0,0,0,'reserved',
 source_warehouse,source_owner_customer_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g
 FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1`, schema), oldWO, wo)
	if err != nil {
		t.Fatal(err)
	}
	var oldBatch string
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(m.batch_code,b.batch_code,'') FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.work_order_id=$1 AND c.unit='g' LIMIT 1`, schema), oldWO).Scan(&oldBatch)
	if err != nil || oldBatch == "" {
		t.Fatalf("old batch: %q %v", oldBatch, err)
	}
	preview := func(path string) app.OperationCompletionPreview {
		t.Helper()
		rec := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if rec.Code != 200 {
			t.Fatalf("reissue preview %d %s", rec.Code, rec.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview("start-preview")
	mats := p.Materials
	for i := range mats {
		if app.OperationUnitFactor(mats[i].Unit) > 0 {
			mats[i].Qty++
		}
	}
	started := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "new-round-start", "difference_reason": "余料再领", "materials": mats})
	if started.Code != 200 {
		t.Fatalf("new round real issue %d %s", started.Code, started.Body.String())
	}
	var newBatch string
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(m.batch_code,b.batch_code,'') FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.work_order_id=$1 AND c.unit='g' LIMIT 1`, schema), wo).Scan(&newBatch)
	if err != nil || oldBatch != newBatch {
		t.Fatalf("reissue used different batch %q -> %q: %v", oldBatch, newBatch, err)
	}
	p = preview("completion-preview")
	completed := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "new-round-complete", "finish_operation": true, "actual_minutes": 0, "actual_minutes_explicit": true, "remainder_mode": "plan_transfer"})
	if completed.Code != 200 {
		t.Fatalf("new round completion %d %s", completed.Code, completed.Body.String())
	}
	get := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/production-plans/%d/packaging-remainders?product_id=%d", plan, product), nil)
	if get.Code != 200 {
		t.Fatalf("new round remainder %d %s", get.Code, get.Body.String())
	}
	var detail app.PlanRemainderDetail
	if err = json.Unmarshal(get.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	var issued int64
	for _, s := range detail.Sources {
		if s.SourceType == "issued" && s.BatchCode == oldBatch {
			issued += s.AvailableG
		}
	}
	if !detail.CanConfirm || issued != 1000 || detail.TotalTasks != 1 {
		t.Fatalf("expected one new round task and its actual 1000g remaining, got %+v", detail)
	}
	var oldCost, newCost, allocated float64
	q := fmt.Sprintf(`SELECT (SELECT actual_cost FROM %[1]s.work_orders WHERE id=$1)::float8,(SELECT actual_cost FROM %[1]s.work_orders WHERE id=$2)::float8,COALESCE((SELECT sum(a.cost) FROM %[1]s.production_started_input_component_allocations a JOIN %[1]s.production_operation_events e ON e.id=a.event_id WHERE e.work_order_id=$2),0)::float8`, schema)
	if err = pool.QueryRow(ctx, q, oldWO, wo).Scan(&oldCost, &newCost, &allocated); err != nil {
		t.Fatal(err)
	}
	if allocated <= 0 || math.Abs(allocated-newCost) > 0.00001 {
		t.Fatalf("new round actual cost=%v allocated=%v", newCost, allocated)
	}
	var selections []app.PlanRemainderSelection
	for _, source := range detail.Sources {
		selections = append(selections, app.PlanRemainderSelection{SourceID: source.SourceID, QuantityG: source.AvailableG})
	}
	url := fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan)
	command := map[string]any{"product_id": product, "expected_version": detail.Version, "request_id": "new-round-return", "target_warehouse": "wip", "sources": selections}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, e, "POST", url, command)
		if res.Code != 200 {
			t.Fatalf("new round return %d %s", res.Code, res.Body.String())
		}
	}
	var oldAfter, newAfter, allocatedAfter float64
	if err = pool.QueryRow(ctx, q, oldWO, wo).Scan(&oldAfter, &newAfter, &allocatedAfter); err != nil {
		t.Fatal(err)
	}
	if math.Abs(oldCost-oldAfter) > 0.00001 || math.Abs(newCost-newAfter) > 0.00001 || math.Abs(allocatedAfter-newAfter) > 0.00001 {
		t.Fatalf("round costs changed: old %v -> %v, new %v -> %v allocation=%v", oldCost, oldAfter, newCost, newAfter, allocatedAfter)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 2)
}

// A net-zero external movement still invalidates the plan's attribution. The
// extra rows represent external issue and return, not an inventory adjustment.
func verifyRemainderExternalRecheck(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, e http.Handler, wo, plan, product int64, oldCmd map[string]any, shortfall, secondOutflow bool) {
	t.Helper()
	url := fmt.Sprintf("/api/production-plans/%d/packaging-remainders", plan)
	get := func() (app.PlanRemainderDetail, map[string]any) {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("%s?product_id=%d", url, product), nil)
		if res.Code != 200 {
			t.Fatalf("read remainder %d %s", res.Code, res.Body.String())
		}
		var d app.PlanRemainderDetail
		var raw map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(res.Body.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		return d, raw
	}
	after, _ := get()
	if !after.CanConfirm {
		t.Fatalf("expected incomplete round still editable: %+v", after)
	}
	// Same item, different owner and source batch cannot invalidate this round.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,operator)
 SELECT CASE WHEN c.component_type='product' THEN 'finished_product' ELSE c.component_type END,c.component_id,'other',c.owner_customer_id+500,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.warehouse,'external_test',900,COALESCE(m.batch_code,b.batch_code,'')||'-other',-1,'test'
 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1`, schema, wo))
	after, _ = get()
	if !after.CanConfirm {
		t.Fatalf("unrelated ledger invalidated round: %+v", after)
	}
	oldVersion := after.Version
	deltaSQL := "(VALUES (-500),(500))"
	if shortfall {
		deltaSQL = "(VALUES (-500))"
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations l SET qty_g=qty_g-500 WHERE (l.material_batch_id,l.warehouse)=(SELECT c.material_batch_id,c.warehouse FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1)`, schema, wo))
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,operator)
 SELECT CASE WHEN c.component_type='product' THEN 'finished_product' ELSE c.component_type END,c.component_id,'same',c.owner_customer_id,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.warehouse,'external_test',901,COALESCE(m.batch_code,b.batch_code,''),v.delta,'test'
 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id CROSS JOIN %[3]s AS v(delta) WHERE c.id=(SELECT cc.id FROM %[1]s.production_started_input_components cc JOIN %[1]s.production_started_inputs ii ON ii.id=cc.input_id WHERE ii.work_order_id=%[2]d AND cc.unit='g' LIMIT 1)`, schema, wo, deltaSQL))
	blocked, raw := get()
	if blocked.CanConfirm || raw["recheck_required"] != true {
		t.Fatalf("net-zero external activity did not require recheck: %+v raw=%v", blocked, raw)
	}
	sources := make([]app.PlanRemainderSelection, 0, len(blocked.Sources))
	for _, s := range blocked.Sources {
		sources = append(sources, app.PlanRemainderSelection{SourceID: s.SourceID, QuantityG: s.AvailableG})
	}
	stale := map[string]any{"product_id": product, "expected_version": oldVersion, "request_id": "external-stale", "target_warehouse": "wip", "sources": sources}
	if res := serveMultilevelProductionJSON(t, e, "POST", url, stale); res.Code == 200 {
		t.Fatal("stale attribution accepted after external activity")
	}
	recheck := map[string]any{"product_id": product, "expected_version": blocked.Version, "request_id": "external-recheck"}
	for n := 0; n < 2; n++ {
		res := serveMultilevelProductionJSON(t, e, "POST", url+"/recheck", recheck)
		if res.Code != 200 {
			t.Fatalf("recheck %d: %d %s", n, res.Code, res.Body.String())
		}
	}
	unblocked, raw := get()
	if shortfall && raw["discrepancy_g"] != float64(500) {
		t.Fatalf("external shortfall must remain explicitly pending: %+v raw=%v", unblocked, raw)
	}
	if !unblocked.CanConfirm || raw["reweigh_required"] != true {
		t.Fatalf("recheck must preserve reweigh requirement after refresh: %+v raw=%v", unblocked, raw)
	}
	unblockedAgain, rawAgain := get()
	if rawAgain["reweigh_required"] != true || unblockedAgain.Version != unblocked.Version {
		t.Fatalf("reweigh requirement lost after second refresh: %+v raw=%v", unblockedAgain, rawAgain)
	}
	if secondOutflow {
		// A second movement occurs after explicit recheck but before submitting.
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations l SET qty_g=qty_g-200 WHERE (l.material_batch_id,l.warehouse)=(SELECT c.material_batch_id,c.warehouse FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1);
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,operator)
 SELECT CASE WHEN c.component_type='product' THEN 'finished_product' ELSE c.component_type END,c.component_id,'same',c.owner_customer_id,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.warehouse,'external_test',902,COALESCE(m.batch_code,b.batch_code,''),-200,'test'
 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1`, schema, wo))
		previous := unblocked.Version
		unblocked, raw = get()
		if unblocked.Version == previous || raw["reweigh_required"] != true {
			t.Fatalf("second outflow missed by snapshot: %+v %v", unblocked, raw)
		}
	}
	finalSources := make([]app.PlanRemainderSelection, 0, len(unblocked.Sources))
	for _, s := range unblocked.Sources {
		finalSources = append(finalSources, app.PlanRemainderSelection{SourceID: s.SourceID, QuantityG: s.AvailableG})
	}
	finish := map[string]any{"product_id": product, "expected_version": unblocked.Version, "request_id": "external-finish", "target_warehouse": "wip", "sources": finalSources}
	if r := serveMultilevelProductionJSON(t, e, "POST", url, finish); r.Code != 409 {
		t.Fatalf("missing explicit reweigh must fail 409, got %d %s", r.Code, r.Body.String())
	}
	finish["reweigh_confirmed"] = true
	if r := serveMultilevelProductionJSON(t, e, "POST", url, finish); r.Code != 200 {
		t.Fatalf("confirmed reweigh %d %s", r.Code, r.Body.String())
	}
	if r := serveMultilevelProductionJSON(t, e, "POST", url, finish); r.Code != 200 {
		t.Fatalf("confirmed reweigh retry %d %s", r.Code, r.Body.String())
	}
	for _, s := range unblocked.Sources {
		if s.SourceType == "issued" {
			t.Fatalf("prior issued remainder duplicated after recheck: %+v", s)
		}
	}
	// Original successful request remains a safe retry after recheck.
	if res := serveMultilevelProductionJSON(t, e, "POST", url, oldCmd); res.Code != 200 {
		t.Fatalf("prior retry %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	if shortfall {
		expectedDiscrepancy := float64(500)
		if secondOutflow {
			expectedDiscrepancy = 700
		}
		pending, raw := get()
		if raw["discrepancy_g"] != expectedDiscrepancy || len(pending.Discrepancies) == 0 {
			t.Fatalf("shortfall disappeared after other remainder processed: %v", raw)
		}
		if secondOutflow {
			// Later net increase is new stock and cannot silently clear old shortage.
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations l SET qty_g=qty_g+300 WHERE (l.material_batch_id,l.warehouse)=(SELECT c.material_batch_id,c.warehouse FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1);
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,operator)
 SELECT CASE WHEN c.component_type='product' THEN 'finished_product' ELSE c.component_type END,c.component_id,'same',c.owner_customer_id,c.bom_spec_id,c.bom_variant_id,c.spec_g,c.warehouse,'external_test',903,COALESCE(m.batch_code,b.batch_code,''),300,'test'
 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id LEFT JOIN %[1]s.material_batches m ON m.id=c.material_batch_id LEFT JOIN %[1]s.stock_batches b ON b.id=c.stock_batch_id WHERE i.work_order_id=%[2]d AND c.unit='g' LIMIT 1`, schema, wo))
			var reread map[string]any
			pending, reread = get()
			if reread["discrepancy_g"] != expectedDiscrepancy {
				t.Fatalf("net increase erased old discrepancy: %v", reread)
			}
			if pending.RecheckRequired {
				body := map[string]any{"product_id": product, "expected_version": pending.Version, "request_id": "second-stock-recheck"}
				r := serveMultilevelProductionJSON(t, e, "POST", url+"/recheck", body)
				if r.Code != 200 {
					t.Fatalf("recheck net increase %d %s", r.Code, r.Body.String())
				}
				pending, reread = get()
				if reread["discrepancy_g"] != expectedDiscrepancy {
					t.Fatalf("net increase recheck erased discrepancy %v", reread)
				}
			}
		}
		ids := make([]int64, 0, len(pending.Discrepancies))
		for _, d := range pending.Discrepancies {
			if d.ID <= 0 {
				t.Fatalf("invalid discrepancy: %+v", d)
			}
			ids = append(ids, d.ID)
		}
		query := fmt.Sprintf(`SELECT COALESCE((SELECT sum(qty_g) FROM %[1]s.material_batch_locations),0)::float8,(SELECT actual_cost FROM %[1]s.work_orders WHERE id=$1)::float8,(SELECT count(*) FROM %[1]s.stock_entries)`, schema)
		var stockBefore, costBefore float64
		var entriesBefore int64
		if err := pool.QueryRow(ctx, query, wo).Scan(&stockBefore, &costBefore, &entriesBefore); err != nil {
			t.Fatal(err)
		}
		resolveURL := url + "/resolve-discrepancies"
		resolve := map[string]any{"product_id": product, "expected_version": pending.Version, "request_id": "resolve-shortfall", "discrepancy_ids": ids, "reason": "查明历史库存差异"}
		noReason := map[string]any{"product_id": product, "expected_version": pending.Version, "request_id": "resolve-no-reason", "discrepancy_ids": ids, "reason": ""}
		if r := serveMultilevelProductionJSON(t, e, "POST", resolveURL, noReason); r.Code < 400 {
			t.Fatalf("blank reason accepted %d %s", r.Code, r.Body.String())
		}
		staleResolve := map[string]any{"product_id": product, "expected_version": "stale-version", "request_id": "resolve-stale", "discrepancy_ids": ids, "reason": "查明"}
		if r := serveMultilevelProductionJSON(t, e, "POST", resolveURL, staleResolve); r.Code < 400 {
			t.Fatalf("stale version accepted %d %s", r.Code, r.Body.String())
		}
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT reject_resolve_audit CHECK(action <> 'packaging_remainder_discrepancy_resolve')`, schema))
		if r := serveMultilevelProductionJSON(t, e, "POST", resolveURL, resolve); r.Code < 400 {
			t.Fatalf("audit failure accepted %d %s", r.Code, r.Body.String())
		}
		assertProductionFlowCount(t, pool, schema, "production_packaging_remainder_resolutions", "true", 0)
		assertProductionFlowCount(t, pool, schema, "production_packaging_remainder_discrepancies", "resolved_by=0", len(ids))
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs DROP CONSTRAINT reject_resolve_audit`, schema))
		for n := 0; n < 2; n++ {
			r := serveMultilevelProductionJSON(t, e, "POST", resolveURL, resolve)
			if r.Code != 200 {
				t.Fatalf("resolve retry %d: %d %s", n, r.Code, r.Body.String())
			}
		}
		assertProductionFlowCount(t, pool, schema, "production_packaging_remainder_resolutions", "true", 1)
		assertProductionFlowCount(t, pool, schema, "production_packaging_remainder_discrepancies", "resolved_by>0", len(ids))
		assertProductionFlowCount(t, pool, schema, "audit_logs", "action='packaging_remainder_discrepancy_resolve'", 1)
		resolved, resolvedRaw := get()
		if resolved.DiscrepancyG != 0 || resolvedRaw["reweigh_required"] != true {
			t.Fatalf("resolution must require recount: %+v %v", resolved, resolvedRaw)
		}
		reloaded, reloadedRaw := get()
		if reloaded.Version != resolved.Version || reloadedRaw["reweigh_required"] != true {
			t.Fatalf("recount lost after refresh: %+v", reloaded)
		}
		current := []app.PlanRemainderSelection{}
		for _, source := range reloaded.Sources {
			current = append(current, app.PlanRemainderSelection{SourceID: source.SourceID, QuantityG: source.AvailableG})
		}
		final := map[string]any{"product_id": product, "expected_version": reloaded.Version, "request_id": "after-resolution-finish", "target_warehouse": "wip", "sources": current}
		if r := serveMultilevelProductionJSON(t, e, "POST", url, final); r.Code != 409 {
			t.Fatalf("finished before recount: %d %s", r.Code, r.Body.String())
		}
		final["reweigh_confirmed"] = true
		for n := 0; n < 2; n++ {
			r := serveMultilevelProductionJSON(t, e, "POST", url, final)
			if r.Code != 200 {
				t.Fatalf("post-resolution finish %d: %d %s", n, r.Code, r.Body.String())
			}
		}
		var stockAfter, costAfter float64
		var entriesAfter int64
		if err := pool.QueryRow(ctx, query, wo).Scan(&stockAfter, &costAfter, &entriesAfter); err != nil {
			t.Fatal(err)
		}
		if stockBefore != stockAfter || costBefore != costAfter || entriesBefore != entriesAfter {
			t.Fatalf("resolution altered inventory/cost/documents: stock %v->%v cost %v->%v entries %v->%v", stockBefore, stockAfter, costBefore, costAfter, entriesBefore, entriesAfter)
		}
		assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
	}
}
