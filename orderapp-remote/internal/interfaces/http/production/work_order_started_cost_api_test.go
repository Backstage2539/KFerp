package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"

	app "orderapp/internal/application/production"
)

// Exercise the actual detail endpoint after the production start API posts
// component costs. This must not substitute a zero cost for an absent query.
func TestWorkOrderDetailReturnsActuallyPostedInputCost(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74889)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=CASE WHEN id=74889 THEN 52 ELSE 2 END WHERE id IN (74889,74890)`, schema))
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("preview: %s", res.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "detail-cost-start", "materials": p.Materials})
	if res.Code != http.StatusOK {
		t.Fatalf("start: %s", res.Body.String())
	}
	var expected float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(material_cost),0)::float8 FROM %s.production_started_inputs WHERE work_order_id=$1 AND status='posted'`, schema), wo).Scan(&expected); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", wo), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("detail: %s", res.Body.String())
	}
	var detail struct {
		StartedMaterialCost *float64 `json:"started_material_cost"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.StartedMaterialCost == nil || math.Abs(*detail.StartedMaterialCost-expected) > 1e-6 {
		t.Fatalf("detail cost %v, want %v", detail.StartedMaterialCost, expected)
	}
	if expected <= 0 {
		t.Fatal("fixture did not post a positive material cost")
	}
	// Return half of the actual posted components through the real adjustment
	// endpoint; detail must show net posted cost, including allocated history.
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("return preview: %s", res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.StartedInputs) != 1 {
		t.Fatalf("expected one input, got %+v", p.StartedInputs)
	}
	input := p.StartedInputs[0]
	returns := []map[string]any{}
	for _, component := range input.Components {
		returns = append(returns, map[string]any{"id": component.ID, "qty": component.RemainingQty / 2})
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_return", "source_input_id": input.ID, "actual_input_qty": input.RemainingQty / 2, "returns": returns, "unprocessed_confirmed": true, "reason": "隔离测试退料", "request_id": "detail-cost-return", "expected_version": p.Version})
	if res.Code != http.StatusOK {
		t.Fatalf("return: %s", res.Body.String())
	}
	var returned float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(r.cost),0)::float8 FROM %[1]s.production_started_input_returns r JOIN %[1]s.production_started_input_components c ON c.id=r.component_id JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.work_order_id=$1`, schema), wo).Scan(&returned); err != nil {
		t.Fatal(err)
	}
	if returned <= 0 {
		t.Fatal("return did not record positive cost")
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", wo), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("detail after return: %s", res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.StartedMaterialCost == nil || math.Abs(*detail.StartedMaterialCost-(expected-returned)) > 1e-6 {
		t.Fatalf("net cost %v, want %v", detail.StartedMaterialCost, expected-returned)
	}
	// A real input with any missing source cost must never look like a fully
	// known total in the HTTP response.
	for _, item := range []struct{ table, batch string }{{"material_batches", "material_batch_id"}, {"stock_batches", "stock_batch_id"}} {
		statement := fmt.Sprintf(`UPDATE %[1]s.%[2]s SET cost_unavailable=true WHERE id IN (SELECT c.%[3]s FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.work_order_id=$1 AND c.%[3]s>0)`, schema, item.table, item.batch)
		if _, err := pool.Exec(ctx, statement, wo); err != nil {
			t.Fatal(err)
		}
	}
	res = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", wo), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("unavailable detail: %s", res.Body.String())
	}
	detail.StartedMaterialCost = nil
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.StartedMaterialCost != nil {
		t.Fatalf("unavailable source cost was reported as known: %v", *detail.StartedMaterialCost)
	}
}
