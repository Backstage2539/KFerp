package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestPlanTransferRejectsUnscopedWorkOrdersBeforeAnyWrite(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88981)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.materials SET onhand_g=onhand_g*2,onhand_units=onhand_units*2 WHERE id IN(10,20);
 UPDATE %[1]s.material_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.material_batch_locations SET qty_g=qty_g*2,qty_units=qty_units*2 WHERE material_id IN(10,20);
 UPDATE %[1]s.stock_batches SET remaining_g=remaining_g*2,qty_g=qty_g*2,remaining_units=remaining_units*2,qty_units=qty_units*2 WHERE item_type='material' AND item_id IN(10,20);`, schema, wo))
	var card, plan, product, parent int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT j.id,w.production_plan_id,w.product_id,w.parent_product_id FROM %s.job_cards j JOIN %s.work_orders w ON w.id=j.work_order_id WHERE w.id=$1 ORDER BY j.id LIMIT 1`, schema, schema), wo).Scan(&card, &plan, &product, &parent); err != nil {
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
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "unscoped-start", "difference_reason": "测试多领", "materials": materials})
	if start.Code != 200 {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	cases := []struct {
		name                  string
		plan, product, parent int64
	}{{"no plan", 0, product, parent}, {"missing product", plan, 0, 0}, {"dangling plan", plan + 999999, product, parent}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET production_plan_id=%d,product_id=%d,parent_product_id=%d WHERE id=%d`, schema, tc.plan, tc.product, tc.parent, wo))
			p := preview("completion-preview")
			if p.PlanRemainderEligible {
				t.Fatal("unscoped task advertised as eligible")
			}
			cmd := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "reject-" + tc.name, "finish_operation": true, "actual_minutes": 1, "remainder_mode": "plan_transfer"}
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), cmd)
			if res.Code == 200 || !strings.Contains(res.Body.String(), "余料") {
				t.Fatalf("unsafe completion %d %s", res.Code, res.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
			assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty>0", 0)
		})
	}
	// Existing explicit weighed-remainder clients retain their old transaction.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET production_plan_id=0,product_id=%d,parent_product_id=%d WHERE id=%d`, schema, product, parent, wo))
	p = preview("completion-preview")
	legacy := map[string]any{"actual_input_qty": 100, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "legacy-unscoped", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 1000, "remainder_warehouse": "wip"}
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), legacy)
	if res.Code != 200 {
		t.Fatalf("legacy completion %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_input_returns", "qty=1000", 1)
}
