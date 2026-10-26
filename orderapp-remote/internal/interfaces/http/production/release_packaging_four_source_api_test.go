package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	productionapp "orderapp/internal/application/production"
	supporthttp "orderapp/internal/interfaces/http/support"
)

func TestReleaseFourSourcePackagingPartialRollbackAndReplay(t *testing.T) {
	pool, schema, app, _, cards := seedReleasePackagingBrowserFixture(t)
	ctx := context.Background()
	app.Use(supporthttp.OperationLogMiddleware(pool, schema))
	preview := func() productionapp.OperationCompletionGroupPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/completion/preview", map[string]any{"job_card_ids": cards})
		if res.Code != 200 {
			t.Fatalf("group preview: %d %s", res.Code, res.Body.String())
		}
		var p productionapp.OperationCompletionGroupPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := preview()
	if first.RemainingQty != 21 || len(first.Items) != 4 {
		t.Fatalf("four-source preview: %+v", first)
	}
	for i, want := range []float64{10, 6, 4, 1} {
		if first.Items[i].PlannedQty != want {
			t.Fatalf("source %d quantity %v, want %v", i, first.Items[i].PlannedQty, want)
		}
	}
	command := func(p productionapp.OperationCompletionGroupPreview, qty float64, requestID string) map[string]any {
		return map[string]any{"job_card_ids": cards, "actual_output_qty": qty, "output_unit": "袋", "actual_minutes": 0, "finish_group": false, "expected_version": p.Version, "request_id": requestID}
	}
	// Fail after earlier sources have already written within the transaction.
	// A rollback must remove their inventory, consumption, cost and audit rows.
	lastWorkID := first.Items[len(first.Items)-1].WorkOrderID
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
	CREATE FUNCTION %[1]s.reject_release_last_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN IF NEW.work_order_id=%[2]d AND NEW.entry_type='finished_receipt' THEN RAISE EXCEPTION 'isolated last-source receipt failure'; END IF; RETURN NEW; END $$;
	CREATE TRIGGER reject_release_last_receipt BEFORE INSERT ON %[1]s.stock_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_release_last_receipt();`, schema, lastWorkID))
	failed := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/completion/submit", command(first, 21, "four-source-forced-failure"))
	if failed.Code == 200 {
		t.Fatal("injected final-source failure unexpectedly succeeded")
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "status='submitted'", 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 0)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='job_card' AND action='complete'", 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DROP TRIGGER reject_release_last_receipt ON %s.stock_entries`, schema))
	for i, qty := range []float64{6, 15} {
		p := preview()
		if p.RemainingQty != 21-float64(i)*6 {
			t.Fatalf("remaining before part %d = %v", i, p.RemainingQty)
		}
		body := command(p, qty, fmt.Sprintf("four-source-part-%d", i))
		for retry := 0; retry < 2; retry++ {
			res := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/completion/submit", body)
			if res.Code != 200 {
				t.Fatalf("part %d retry %d: %d %s", i, retry, res.Code, res.Body.String())
			}
			var result productionapp.OperationCompletionGroupResult
			if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || result.CompletedQty != qty {
				t.Fatalf("part %d result %s err=%v", i, res.Body.String(), err)
			}
		}
		if path := os.Getenv("ORDERAPP_RELEASE_PACKAGING_DIAGNOSTICS"); i == 0 && path != "" {
			observations := map[string]json.RawMessage{}
			for name, route := range map[string]string{"material_issue_preview": "/api/production/workstation/material-issue/preview", "material_issue_shortage_task_preview": "/api/production/workstation/material-issue/preview", "completion_preview": "/api/production/workstation/completion/preview"} {
				selectedCards := cards
				if name == "material_issue_shortage_task_preview" {
					selectedCards = cards[:1]
				}
				response := serveMultilevelProductionJSON(t, app, "POST", route, map[string]any{"job_card_ids": selectedCards})
				if response.Code != 200 {
					t.Fatalf("partial diagnostic %s: %d %s", name, response.Code, response.Body.String())
				}
				observations[name] = append(json.RawMessage(nil), response.Body.Bytes()...)
			}
			data, err := json.MarshalIndent(observations, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, item := range first.Items {
		var received int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_units),0)::bigint FROM %s.stock_entry_items i JOIN %s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.entry_type='finished_receipt' AND e.status='submitted'`, schema, schema), item.WorkOrderID).Scan(&received); err != nil {
			t.Fatal(err)
		}
		if float64(received) != item.PlannedQty {
			t.Fatalf("source %s received %d, want %v", item.WorkOrderNo, received, item.PlannedQty)
		}
	}
	var beanG, bags int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g) FILTER(WHERE i.material_id=10),0)::bigint,COALESCE(SUM(i.qty_units) FILTER(WHERE i.material_id=20),0)::bigint FROM %s.stock_entry_items i JOIN %s.stock_entries e ON e.id=i.stock_entry_id WHERE e.purpose='material_consumption_for_manufacture' AND e.status='submitted'`, schema, schema)).Scan(&beanG, &bags); err != nil {
		t.Fatal(err)
	}
	if beanG != 4767 || bags != 21 {
		t.Fatalf("group consumption beans=%dg bags=%d, want 4767g/21", beanG, bags)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 5)
	assertProductionFlowCount(t, pool, schema, "work_orders", "status='completed'", 4)
	var cost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(material_cost),0)::float8 FROM %s.production_operation_events WHERE kind='complete' AND status='posted'`, schema)).Scan(&cost); err != nil {
		t.Fatal(err)
	}
	if math.Abs(cost-402.36) > 0.00001 {
		t.Fatalf("material cost=%v want 402.36", cost)
	}
	assertProductionFlowCount(t, pool, schema, "operation_logs", "method='POST' AND path='/api/production/workstation/completion/submit' AND status=200", 4)
	assertProductionFlowCount(t, pool, schema, "operation_logs", "method='POST' AND path='/api/production/workstation/completion/submit' AND status>=400", 1)
}
