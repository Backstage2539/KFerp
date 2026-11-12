package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	productionrepo "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestWorkstationPostedInputsAreNotMissing(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74888)
	registerProductionWorkstationAPI(e, app.NewService(productionrepo.NewRepository(pool, schema)))
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "readiness-input", "materials": p.Materials}
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), payload)
	if res.Code != 200 {
		t.Fatalf("start: %s", res.Body.String())
	}
	before := 0
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_ledger_entries`, schema)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, "GET", "/api/production/workstation-overview?scope=all", nil)
	if res.Code != 200 {
		t.Fatalf("overview: %s", res.Body.String())
	}
	var overview app.ProductionWorkstationOverview
	if err := json.Unmarshal(res.Body.Bytes(), &overview); err != nil {
		t.Fatal(err)
	}
	for _, task := range overview.Tasks {
		if task.JobCardID != card {
			continue
		}
		if task.Status != "running" || task.MaterialReadinessState != "ready" {
			t.Fatalf("posted task marked missing: %+v", task)
		}
		if len(task.MaterialReadiness) == 0 {
			t.Fatal("no material rows")
		}
		for _, m := range task.MaterialReadiness {
			if m.ShortageG > 0 || m.ShortageUnits > 0 || m.IssuedG+m.IssuedUnits <= 0 {
				t.Fatalf("posted material not accounted for: %+v", m)
			}
		}
		after := 0
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_ledger_entries`, schema)).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatal("read wrote stock")
		}
		// Return half of each real posted component, then check that the
		// read model derives net issued from the persisted return records.
		res = serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if len(p.StartedInputs) != 1 {
			t.Fatalf("expected one input: %+v", p.StartedInputs)
		}
		input := p.StartedInputs[0]
		returns := []map[string]any{}
		for _, c := range input.Components {
			returns = append(returns, map[string]any{"id": c.ID, "qty": c.RemainingQty / 2})
		}
		returned := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/material-adjustments", card), map[string]any{"kind": "material_return", "source_input_id": input.ID, "actual_input_qty": input.RemainingQty / 2, "returns": returns, "unprocessed_confirmed": true, "reason": "隔离测试退回未加工物料", "request_id": "readiness-return", "expected_version": p.Version})
		if returned.Code != 200 {
			t.Fatalf("return: %s", returned.Body.String())
		}
		coverage, err := productionrepo.NewRepository(pool, schema).GetWorkOrderWIPCoverage(ctx, wo)
		if err != nil {
			t.Fatal(err)
		}
		if !coverage.StartedInputsKnown || len(coverage.StartedInputs) == 0 {
			t.Fatal("posted components missing")
		}
		for _, component := range coverage.StartedInputs {
			if component.JobCardID != card || component.IssuedQty <= 0 {
				t.Fatalf("wrong posted component %+v", component)
			}
			if component.IssuedQty != component.InProcessQty {
				t.Fatalf("returns not reflected in issued quantity %+v", component)
			}
		}
		res = serveMultilevelProductionJSON(t, e, "GET", "/api/production/workstation-overview?scope=all", nil)
		if res.Code != 200 {
			t.Fatalf("overview after return: %s", res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &overview); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range overview.Tasks {
			if row.JobCardID == card {
				found = true
				for _, m := range row.MaterialReadiness {
					if m.IssuedG+m.IssuedUnits <= 0 || m.RemainingRequiredG+m.RemainingRequiredUnits <= 0 {
						t.Fatalf("partial return not represented %+v", m)
					}
				}
			}
		}
		if !found {
			t.Fatal("missing after return")
		}
		return
	}
	t.Fatal("task missing")
}
