package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	app "orderapp/internal/application/production"
	"testing"
)

func TestModernOperationTimerBaselineSeparatesManualAndAutomatic(t *testing.T) {
	cases := []struct {
		name                             string
		firstManual, secondManual        int
		wantFirst, wantSecond, wantThird int
		stripMarker                      bool
		mode, scheduling                 string
	}{{"manual-then-auto", 10, 0, 600, 60, 30, false, "piece", "quantity"}, {"auto-then-manual", 0, 10, 30, 600, 30, false, "piece", "quantity"}, {"legacy-keeps-existing-semantics", 10, 0, 600, 0, 0, false, "", "batch"}, {"existing-unmarked-modern-events", 10, 0, 600, 0, 0, true, "piece", "quantity"}}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, int64(89001+index*10))
			route := fmt.Sprintf(`{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"reporting_type\":\"%s\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`, tc.mode)
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, route); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,operation='generic',cost_method='time',hourly_rate=60,scheduling_mode='%s',throughput_per_hour=100,batch_size_unit='袋' WHERE work_order_id=%d`, schema, tc.scheduling, wo))
			var card int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
				t.Fatal(err)
			}
			preview := func(path string) app.OperationCompletionPreview {
				t.Helper()
				r := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
				if r.Code != 200 {
					t.Fatal(r.Body.String())
				}
				var p app.OperationCompletionPreview
				if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
					t.Fatal(err)
				}
				return p
			}
			p := preview("start-preview")
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": 100, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "timer-start", "materials": p.Materials})
			if res.Code != 200 {
				t.Fatalf("start: %s", res.Body.String())
			}
			complete := func(timer int, minutes int, qty int, remaining int, finished bool, key string) {
				t.Helper()
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='paused',metrics_json=COALESCE(metrics_json,'{}')||jsonb_build_object('timer_elapsed_seconds',%d) WHERE id=%d`, schema, timer, card))
				p := preview("completion-preview")
				if key == "timer-second" {
					want := 0
					if tc.mode == "piece" && !tc.stripMarker {
						want = 1
					}
					if p.ActualMinutes != want {
						t.Fatalf("preview incremental minutes=%d want=%d", p.ActualMinutes, want)
					}
				}
				payload := map[string]any{"actual_input_qty": qty, "actual_output_qty": qty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "actual_minutes": minutes, "expected_version": p.Version, "request_id": key, "finish_operation": finished, "difference_reason": "分次处理"}
				if tc.mode == "piece" {
					payload["actual_remainder_g"] = 227 * remaining
					if finished {
						payload["actual_remainder_g"] = 0
					}
					payload["remainder_warehouse"] = "wip"
				}
				for n := 0; n < 2; n++ {
					res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
					if res.Code != 200 {
						t.Fatalf("complete %s/%d: %s", key, n, res.Body.String())
					}
				}
			}
			complete(30, tc.firstManual, 25, 75, false, "timer-first")
			if tc.stripMarker {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_operation_events SET payload=payload-'timer_baseline_seconds' WHERE request_id='timer-first'`, schema))
			}
			complete(90, tc.secondManual, 25, 50, false, "timer-second")
			complete(120, 0, 50, 0, true, "timer-third")
			var first, second, third int
			var firstCost, secondCost, thirdCost float64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT (payload->>'actual_seconds')::int FROM %[1]s.production_operation_events WHERE request_id='timer-first'),(SELECT (payload->>'actual_seconds')::int FROM %[1]s.production_operation_events WHERE request_id='timer-second'),(SELECT operation_cost::float8 FROM %[1]s.production_operation_events WHERE request_id='timer-first'),(SELECT operation_cost::float8 FROM %[1]s.production_operation_events WHERE request_id='timer-second'),(SELECT (payload->>'actual_seconds')::int FROM %[1]s.production_operation_events WHERE request_id='timer-third'),(SELECT operation_cost::float8 FROM %[1]s.production_operation_events WHERE request_id='timer-third')`, schema)).Scan(&first, &second, &firstCost, &secondCost, &third, &thirdCost); err != nil {
				t.Fatal(err)
			}
			if first != tc.wantFirst || second != tc.wantSecond || third != tc.wantThird || math.Abs(firstCost-float64(tc.wantFirst)/60) > 0.0001 || math.Abs(secondCost-float64(tc.wantSecond)/60) > 0.0001 || math.Abs(thirdCost-float64(tc.wantThird)/60) > 0.0001 {
				t.Fatalf("seconds/cost got %d/%d/%d %.4f/%.4f/%.4f want %d/%d/%d", first, second, third, firstCost, secondCost, thirdCost, tc.wantFirst, tc.wantSecond, tc.wantThird)
			}
			assertProductionFlowCount(t, pool, schema, "production_operation_events", "request_id IN('timer-first','timer-second','timer-third') AND kind='complete'", 3)
		})
	}
}
