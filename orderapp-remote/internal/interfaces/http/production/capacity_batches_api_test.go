package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"strings"
	"testing"
)

func TestCapacityBatchAPIReadWriteCompatibility(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	repo := postgresproduction.NewRepository(pool, schema)
	plan, err := repo.CreateProductionPlan(ctx, productionapp.CreateProductionPlanCommand{Selected: map[string]bool{"1-227": true}, Operator: "test"})
	if err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.manufacturing_workstations(id,name,status,hourly_rate) VALUES(719,'批次测试工位','active',102);
 INSERT INTO %[1]s.manufacturing_workstation_capacities(id,workstation_id,name,status,batch_size_qty,batch_size_unit,standard_minutes) VALUES(719,719,'20kg','active',20,'kg',25);
 INSERT INTO %[1]s.manufacturing_operations(id,name,status) VALUES(719,'烘焙','active');
 INSERT INTO %[1]s.manufacturing_workstation_operations(workstation_id,operation_id) VALUES(719,719);
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, fmt.Sprintf("/api/production-plans/%d%s", plan.ID, path), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	rows := fmt.Sprintf(`[{"production_plan_item_id":%d,"operation_seq":1,"operation":"烘焙","workstation_capacity_id":719,"planned_batch_count":6}]`, plan.Items[0].ID)
	preview := request(http.MethodPost, "/operation-splits/preview", `{"items":`+rows+`}`, 200)
	var result productionapp.ProductionPlanOperationSplitPreview
	if err := json.Unmarshal(preview, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.OperationCoverage) == 0 || result.OperationCoverage[0].ArrangedG != 120000 {
		t.Fatalf("batch preview: %+v", result)
	}
	request(http.MethodPost, "/operation-splits", `{"items":`+rows+`}`, 200)
	saved, err := repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	check := func(p productionapp.ProductionPlanDetail, qty float64, batches int) {
		t.Helper()
		if len(p.OperationSplits) != 1 || p.OperationSplits[0].PlannedQty != qty || p.OperationSplits[0].PlannedBatchCount != batches {
			t.Fatalf("saved: %+v", p.OperationSplits)
		}
	}
	check(saved, 120, 6)
	if saved.OperationSplits[0].PlannedMinutes != 150 || saved.OperationSplits[0].PlannedOperationCost != 255 {
		t.Fatal(saved.OperationSplits)
	}
	rows = strings.Replace(rows, `"planned_batch_count":6`, `"planned_batch_count":5`, 1)
	request(http.MethodPatch, "/draft", fmt.Sprintf(`{"draft_token":%q,"operation_splits":%s}`, saved.DraftToken, rows), 200)
	saved, err = repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	check(saved, 100, 5)
	rows = strings.Replace(rows, `"planned_batch_count":5`, `"planned_qty":3.797`, 1)
	request(http.MethodPost, "/operation-splits", `{"items":`+rows+`}`, 200)
	saved, err = repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	check(saved, 3.797, 1)
	for _, bad := range []string{`"planned_batch_count":0`, `"planned_batch_count":-1`, `"planned_batch_count":1.5`} {
		invalid := strings.Replace(rows, `"planned_qty":3.797`, bad, 1)
		request(http.MethodPost, "/operation-splits/preview", `{"items":`+invalid+`}`, 400)
		request(http.MethodPost, "/operation-splits", `{"items":`+invalid+`}`, 400)
		request(http.MethodPatch, "/draft", fmt.Sprintf(`{"draft_token":%q,"operation_splits":%s}`, saved.DraftToken, invalid), 400)
	}
	// The server owns the batch size even if an old client sends stale projections.
	resizedRows := strings.Replace(rows, `"planned_qty":3.797`, `"planned_batch_count":6,"planned_qty_g":999999,"batch_size_qty":20`, 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET batch_size_qty=12 WHERE id=719`, schema))
	resizedPreview := request(http.MethodPost, "/operation-splits/preview", `{"items":`+resizedRows+`}`, 200)
	if err := json.Unmarshal(resizedPreview, &result); err != nil || result.OperationCoverage[0].ArrangedG != 72000 {
		t.Fatalf("current size preview: %s err=%v", resizedPreview, err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET batch_size_qty=20 WHERE id=719`, schema))
	// A global workstation change must invalidate batch-mode requests without replacing the saved rows.
	batchRows := strings.Replace(rows, `"planned_qty":3.797`, `"planned_batch_count":6`, 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstations SET status='inactive' WHERE id=719`, schema))
	request(http.MethodPost, "/operation-splits/preview", `{"items":`+batchRows+`}`, 400)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstations SET status='active' WHERE id=719; DELETE FROM %s.manufacturing_workstation_operations WHERE workstation_id=719`, schema, schema))
	request(http.MethodPost, "/operation-splits", `{"items":`+batchRows+`}`, 400)
	afterInvalid, err := repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	check(afterInvalid, 3.797, 1)
	var logs int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='production_plan' AND entity_id=$1`, schema), plan.ID).Scan(&logs); err != nil || logs < 3 {
		t.Fatalf("audit count=%d err=%v", logs, err)
	}
	// Submitted plans must retain the frozen split even if a client tries to edit it.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET status='submitted' WHERE id=%d`, schema, plan.ID))
	request(http.MethodPost, "/operation-splits", `{"items":`+rows+`}`, 400)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET batch_size_qty=12,standard_minutes=5 WHERE id=719`, schema))
	frozen, err := repo.GetProductionPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	check(frozen, 3.797, 1)
	if frozen.OperationSplits[0].BatchSizeQty != 20 || frozen.OperationSplits[0].StandardMinutes != 25 {
		t.Fatal("submitted capacity snapshot changed")
	}
}
