package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestNewPlanFreezesBOMStartPolicyWithoutPerMaterialNodes(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	server, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74790, "bom_start_v1")
	var policy string
	var consumed int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_consumption_policy FROM %s.work_orders WHERE id=$1`, schema), work).Scan(&policy); err != nil {
		t.Fatal(err)
	}
	if policy != "bom_start_v1" {
		t.Fatalf("new order policy=%s", policy)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(consumed_g+consumed_units),0) FROM %s.work_order_material_reservations WHERE work_order_id=$1`, schema), work).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 0 {
		t.Fatal("planning consumed stock")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=22700 WHERE id=10`, schema))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id LIMIT 1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	var p app.OperationCompletionPreview
	preview := func() {
		t.Helper()
		res := serveMultilevelProductionJSON(t, server, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
	}
	preview()
	if p.BlockedReason != "" || len(p.Materials) != 2 {
		t.Fatalf("bad preview %+v", p)
	}
	started := serveMultilevelProductionJSON(t, server, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "ordinary-plan-start"})
	if started.Code != 200 {
		t.Fatal(started.Body.String())
	}
	preview()
	done := serveMultilevelProductionJSON(t, server, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "ordinary-plan-complete"})
	if done.Code != 200 {
		t.Fatal(done.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("work_order_id=%d AND remaining_qty=0 AND remaining_cost=0", work), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt' AND status='submitted'", work), 1)

}
func TestBOMStartDirectCreationFreezesBOMRoute(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 74830, 10, "PR747-DIRECT-WIP", "在制熟豆", 22700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 74831, 20, "PR747-DIRECT-PACK", "包装袋", 100)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.product_production_configs(product_id,process_route_id) VALUES(1,32) ON CONFLICT(product_id) DO UPDATE SET process_route_id=32; UPDATE %[1]s.materials SET onhand_g=22700,onhand_units=100 WHERE id IN(10,20)`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	r := serveMultilevelProductionJSON(t, e, "POST", "/api/produce/start", map[string]any{"selected": []string{"1-227"}})
	if r.Code != 200 {
		t.Fatalf("direct creation: %d %s", r.Code, r.Body.String())
	}
	var route int64
	var policy string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (process_snapshot_json->>'route_id')::bigint,material_consumption_policy FROM %s.work_orders ORDER BY id DESC LIMIT 1`, schema)).Scan(&route, &policy); err != nil {
		t.Fatal(err)
	}
	if route != 31 || policy != "bom_start_v1" {
		t.Fatalf("direct creation must freeze BOM route 31, got route=%d policy=%s", route, policy)
	}
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards ORDER BY id DESC LIMIT 1`, schema)).Scan(&card); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 0)
	r = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.BlockedReason != "" || len(p.Materials) == 0 {
		t.Fatalf("direct start unavailable: %+v", p)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "direct-start"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	r = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	r = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "direct-complete"})
	if r.Code != 200 {
		t.Fatal(r.Body.String())
	}
}

func TestBOMStartOldCompletionClientIsToldToRefresh(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	server, _, work := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74990, "bom_start_v1")
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no,id LIMIT 1`, schema), work).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, server, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_output_qty": 1})
	if res.Code == 200 || !strings.Contains(res.Body.String(), "刷新页面") || !strings.Contains(res.Body.String(), "开工") {
		t.Fatalf("old completion client must get actionable refresh and start instructions: %d %s", res.Code, res.Body.String())
	}
	previewResult := serveMultilevelProductionJSON(t, server, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	var preview app.OperationCompletionPreview
	if previewResult.Code != 200 {
		t.Fatalf("completion preview: %d %s", previewResult.Code, previewResult.Body.String())
	}
	if err := json.Unmarshal(previewResult.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, server, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_output_qty": preview.PlannedQty, "expected_version": preview.Version, "request_id": "old-completion-versioned"})
	if res.Code == 200 || !strings.Contains(res.Body.String(), "刷新页面") || !strings.Contains(res.Body.String(), "开工") {
		t.Fatalf("versioned old completion client must get actionable refresh and start instructions: %d %s", res.Code, res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND status='pending'", card), 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d", card), 0)
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 0)
}
