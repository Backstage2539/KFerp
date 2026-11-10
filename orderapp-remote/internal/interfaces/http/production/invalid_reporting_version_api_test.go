package production

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	app "orderapp/internal/application/production"
)

func TestExplicitReportingTypeWithInvalidVersionCannotFallbackToLegacy(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, wo := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 88950)
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{"legacy", "bom_start_v1"} {
		for _, tc := range []struct{ name, execution string }{{"missing", `{"reporting_type":"piece"}`}, {"zero", `{"version":0,"reporting_type":"batch"}`}, {"unsupported", `{"version":2,"reporting_type":"incoming"}`}, {"invalid-type", `{"version":"bad","reporting_type":"piece"}`}} {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				executionJSON, _ := json.Marshal(tc.execution)
				route := fmt.Sprintf(`{"operations":[{"seq":1,"operation":"包装","execution_json":%s}]}`, executionJSON)
				if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET status='released',material_consumption_policy=$2,process_snapshot_json=$3::jsonb WHERE id=$1`, schema), wo, policy, route); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1,operation='包装',status='pending',planned_input_qty=0 WHERE id=$1`, schema), card); err != nil {
					t.Fatal(err)
				}
				res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
				if res.Code != 200 {
					t.Fatalf("preview %d %s", res.Code, res.Body.String())
				}
				var p app.OperationCompletionPreview
				if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
					t.Fatal(err)
				}
				if p.Enabled || p.LegacyExecution || p.BlockedCode != "frozen_route_invalid" || !strings.Contains(p.BlockedReason, "版本") {
					t.Fatalf("invalid modern config accepted: %+v", p)
				}
				res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"request_id": "invalid-version", "expected_version": p.Version, "actual_output_qty": 1, "output_unit": "袋", "difference_reason": "test"})
				if res.Code == 200 {
					t.Fatal("invalid config completed")
				}
				assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
			})
		}
	}
	trimmed := `{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"version\":1,\"reporting_type\":\" piece \"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='legacy',process_snapshot_json=$2::jsonb WHERE id=$1`, schema), wo, trimmed); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var p app.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Execution.ReportingType != "piece" || p.LegacyExecution {
		t.Fatalf("explicit type was not normalized: %+v", p)
	}
}
