package production

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	financeapp "orderapp/internal/application/finance"
	app "orderapp/internal/application/production"
	financepg "orderapp/internal/infrastructure/postgres/finance"
)

// Partial receipts are real production and must not be omitted merely because
// the job card has not reached the state that creates the batch cost row.
func TestPartialUnknownProductionBlocksMonthlyClosingAcrossMonths(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88989)
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS grand_total numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS discount_amount numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS shipping_amount numeric NOT NULL DEFAULT 0; ALTER TABLE %[1]s.orders ADD COLUMN IF NOT EXISTS total_amount numeric NOT NULL DEFAULT 0;
 UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"operation":"generic","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic' WHERE work_order_id=%[2]d;
 UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE material_id=10; UPDATE %[1]s.stock_batches SET cost_unavailable=true WHERE item_type='material' AND item_id=10;`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
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
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "partial-unknown-start", "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("start %d %s", start.Code, start.Body.String())
	}
	p = preview("completion-preview")
	first := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "partial-unknown-first", "finish_operation": false, "actual_minutes": 1, "actual_remainder_g": 11350, "remainder_warehouse": "wip"})
	if first.Code != 200 {
		t.Fatalf("partial %d %s", first.Code, first.Body.String())
	}
	var current, previous string
	if err := pool.QueryRow(ctx, `SELECT to_char(now(),'YYYY-MM'),to_char(date_trunc('month',now())-interval '1 day','YYYY-MM')`).Scan(&current, &previous); err != nil {
		t.Fatal(err)
	}
	finance := financeapp.NewService(financepg.NewRepository(pool, schema))
	report, err := finance.DraftReport(ctx, financeapp.ReportFilter{Month: current})
	if err != nil || !report.CostUnavailable {
		t.Fatalf("partial not marked unknown: %+v %v", report, err)
	}
	if _, err = finance.CloseMonth(ctx, financeapp.CloseMonthCommand{Month: current, Actor: "test"}); err == nil || !strings.Contains(err.Error(), "成本待核算") {
		t.Fatalf("partial closing not blocked: %v", err)
	}
	// Make the partial event belong to the preceding month, then complete the
	// same job in the current month. The preceding month must remain unknown.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_operation_events SET created_at=date_trunc('month',now())-interval '1 day' WHERE job_card_id=$1 AND kind='complete' AND status='posted'`, schema), card); err != nil {
		t.Fatal(err)
	}
	p = preview("completion-preview")
	final := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": 50, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "partial-unknown-final", "finish_operation": true, "actual_minutes": 1, "actual_remainder_g": 0, "remainder_warehouse": "wip"})
	if final.Code != 200 {
		t.Fatalf("final %d %s", final.Code, final.Body.String())
	}
	old, err := finance.DraftReport(ctx, financeapp.ReportFilter{Month: previous})
	if err != nil || !old.CostUnavailable {
		t.Fatalf("previous month lost unknown partial: %+v %v", old, err)
	}
	now, err := finance.DraftReport(ctx, financeapp.ReportFilter{Month: current})
	if err != nil || !now.CostUnavailable {
		t.Fatalf("final month unknown: %+v %v", now, err)
	}
	var sum float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(total_cost),0)::float8 FROM %s.production_batch_costs WHERE created_at>=date_trunc('month',now()) AND created_at<date_trunc('month',now())+interval '1 month'`, schema)).Scan(&sum); err != nil {
		t.Fatal(err)
	}
	if float64(now.MainBusinessCost) != sum {
		t.Fatalf("cost duplicated: report=%v batch=%v", now.MainBusinessCost, sum)
	}
}
