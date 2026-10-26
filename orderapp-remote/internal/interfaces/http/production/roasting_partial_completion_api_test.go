package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type roastingPartialFixture struct {
	app                        *echo.Echo
	pool                       *pgxpool.Pool
	schema                     string
	workOrderID, cardID, batch int64
}

func newRoastingPartialFixture(t *testing.T) roastingPartialFixture {
	t.Helper()
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, workID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87601)
	const batchID int64 = 87603
	seedProductionFlowWIPBatch(t, ctx, pool, schema, batchID, 30, "ROAST-PARTIAL-INPUT", "分次烘焙生豆", 10000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.materials SET onhand_g=10000 WHERE id=30;
		UPDATE %[1]s.material_batches SET unit_cost=100 WHERE id=%[3]d;
		DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[2]d;
		DELETE FROM %[1]s.work_order_material_reservations WHERE work_order_id=%[2]d;
		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=200;
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,ratio_pct)
		VALUES (200,30,'material','ratio_pct',100);
		UPDATE %[1]s.work_orders SET bom_version_id=200,
			planned_g=6000,planned_output_g=5000,planned_inventory_qty=5,
			output_qty=5,output_unit='kg',inventory_unit='kg',target_warehouse='finished_goods',status='released',
			material_snapshot='[{"material_id":30,"material_name":"分次烘焙生豆","unit":"kg","ratio_pct":100,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]',
			process_snapshot_json='{"operations":[{"seq":1,"operation":"咖啡烘焙+除石","execution_json":"{\"version\":1,\"node_key\":\"roast\",\"source_warehouse\":\"wip\",\"receipt_enabled\":true,\"receipt_warehouse\":\"finished_goods\",\"output_unit\":\"kg\"}"}]}',
			operation_material_bindings='{"material:30:0:0":{"node_key":"roast","source_warehouse":"wip"}}'
		WHERE id=%[2]d;
		UPDATE %[1]s.job_cards SET sequence_no=1,operation='咖啡烘焙+除石',status='pending',
			planned_input_qty=6000,actual_input_qty=0,actual_output_qty=0,started_at=NULL,
			completed_at=NULL,cost_method='piece',piece_rate=0,hourly_rate=0,planned_operation_cost=0
		WHERE work_order_id=%[2]d;
	`, schema, workID, batchID))
	f := roastingPartialFixture{app: e, pool: pool, schema: schema, workOrderID: workID, batch: batchID}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, schema), workID).Scan(&f.cardID); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f roastingPartialFixture) preview(t *testing.T) productionapp.OperationCompletionPreview {
	t.Helper()
	res := serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", f.cardID), nil)
	if res.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", res.Code, res.Body.String())
	}
	var p productionapp.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Enabled || p.BlockedReason != "" || len(p.Materials) != 1 {
		t.Fatalf("single roasting card must be executable: %+v", p)
	}
	return p
}

type roastingPartialState struct {
	ConsumedG, WIPG, ReceiptG, InventoryG, ReservationConsumedG int64
	Events, Entries, Audits                                     int64
	MaterialCost, WorkOrderCost, InputQty, OutputQty            float64
	CardStatus, WorkOrderStatus                                 string
}

func (f roastingPartialFixture) state(t *testing.T) roastingPartialState {
	t.Helper()
	var s roastingPartialState
	err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT
		(SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.purpose='material_consumption_for_manufacture' AND e.status='submitted'),
		(SELECT qty_g FROM %[1]s.material_batch_locations WHERE material_batch_id=$3 AND warehouse='wip'),
		(SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %[1]s.stock_entry_items i JOIN %[1]s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.entry_type='finished_receipt' AND e.status='submitted'),
		(SELECT COALESCE(SUM(onhand_units*spec_g+onhand_loose_g),0)::bigint FROM %[1]s.finished_inventory WHERE product_id=1 AND warehouse='finished_goods'),
		(SELECT COALESCE(SUM(consumed_g),0)::bigint FROM %[1]s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=30),
		(SELECT COUNT(*) FROM %[1]s.production_operation_events WHERE work_order_id=$1),
		(SELECT COUNT(*) FROM %[1]s.stock_entries WHERE work_order_id=$1),
		(SELECT COUNT(*) FROM %[1]s.audit_logs),
		(SELECT COALESCE(SUM(material_cost),0)::float8 FROM %[1]s.production_operation_events WHERE work_order_id=$1 AND status='posted'),
		w.actual_cost::float8,j.actual_input_qty::float8,j.actual_output_qty::float8,j.status,w.status
		FROM %[1]s.work_orders w JOIN %[1]s.job_cards j ON j.work_order_id=w.id WHERE w.id=$1 AND j.id=$2`, f.schema), f.workOrderID, f.cardID, f.batch).Scan(
		&s.ConsumedG, &s.WIPG, &s.ReceiptG, &s.InventoryG, &s.ReservationConsumedG,
		&s.Events, &s.Entries, &s.Audits, &s.MaterialCost, &s.WorkOrderCost, &s.InputQty, &s.OutputQty, &s.CardStatus, &s.WorkOrderStatus)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRoastingPartialCompletionAPIConsumesOnlyCurrentInput(t *testing.T) {
	for _, tc := range []struct {
		name        string
		firstInput  float64
		firstOutput float64
		defaultUI   bool
	}{
		{name: "one_then_five", firstInput: 1, firstOutput: .8},
		{name: "three_then_three", firstInput: 3, firstOutput: 2.5},
		{name: "existing_partial_then_new_UI_default_input", firstInput: 1, firstOutput: .8, defaultUI: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoastingPartialFixture(t)
			p := f.preview(t)
			first := map[string]any{"request_id": "partial-first", "expected_version": p.Version,
				"input_unit": p.InputUnit, "output_unit": p.OutputUnit, "actual_input_qty": tc.firstInput,
				"actual_output_qty": tc.firstOutput, "finish_operation": false}
			path := fmt.Sprintf("/api/job-cards/%d/complete", f.cardID)
			res := serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, first)
			if res.Code != http.StatusOK {
				t.Fatalf("first partial status=%d body=%s", res.Code, res.Body.String())
			}
			beforeReplay := f.state(t)
			res = serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, first)
			if res.Code != http.StatusOK || f.state(t) != beforeReplay {
				t.Fatalf("first partial replay must not duplicate inventory, cost, or audit: status=%d body=%s", res.Code, res.Body.String())
			}
			p = f.preview(t)
			remainingInput := 6 - tc.firstInput
			if math.Abs(p.InputQty-remainingInput) > 1e-9 || math.Abs(p.Materials[0].ConsumedQty-tc.firstInput) > 1e-9 {
				t.Fatalf("preview must distinguish remaining input from historic consumption: %+v", p)
			}
			if math.Abs(p.Materials[0].Qty-remainingInput) > 1e-9 {
				t.Errorf("remaining input %g kg must need %g kg, got %g", p.InputQty, remainingInput, p.Materials[0].Qty)
			}
			second := map[string]any{"request_id": "partial-second", "expected_version": p.Version,
				"output_unit": p.OutputUnit, "actual_output_qty": p.PlannedQty, "finish_operation": true}
			if !tc.defaultUI {
				second["input_unit"], second["actual_input_qty"] = p.InputUnit, remainingInput
			}
			res = serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, second)
			if res.Code != http.StatusOK {
				t.Fatalf("final partial status=%d body=%s", res.Code, res.Body.String())
			}
			got := f.state(t)
			if got.ConsumedG != 6000 || got.WIPG != 4000 || got.ReservationConsumedG != 6000 || got.ReceiptG != 5000 || got.InventoryG != 5000 || got.MaterialCost != 600 || got.WorkOrderCost != 600 || got.InputQty != 6 || got.OutputQty != 5 || got.Events != 2 || got.CardStatus != "completed" || got.WorkOrderStatus != "completed" {
				t.Errorf("two partial completions must post 6kg input, 5kg output and cost 600 exactly once: %+v", got)
			}
			res = serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, second)
			if res.Code != http.StatusOK || f.state(t) != got {
				t.Fatalf("final replay changed inventory, cost or audit: status=%d body=%s", res.Code, res.Body.String())
			}
			assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", fmt.Sprintf("entity_type='job_card' AND entity_id=%d AND action='complete'", f.cardID), 2)
			var auditInput, auditOutput, auditConsumption float64
			if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`
				SELECT SUM((meta->>'actual_input_qty')::numeric)::float8,
				       SUM((meta->>'actual_output_qty')::numeric)::float8,
				       SUM((meta->'materials'->0->>'qty')::numeric)::float8
				FROM %s.audit_logs WHERE entity_type='job_card' AND entity_id=$1 AND action='complete'
			`, f.schema), f.cardID).Scan(&auditInput, &auditOutput, &auditConsumption); err != nil {
				t.Fatal(err)
			}
			if auditInput != 6 || auditOutput != 5 || auditConsumption != 6 {
				t.Fatalf("completion audit must agree with the ledger: input=%g output=%g consumption=%g", auditInput, auditOutput, auditConsumption)
			}
			assertProductionFlowCount(t, f.pool, f.schema, "production_batch_costs", fmt.Sprintf("running_item_id=(SELECT running_item_id FROM %s.work_orders WHERE id=%d) AND total_cost=600", f.schema, f.workOrderID), 1)
		})
	}
}

func TestRoastingPartialCompletionAPIFailureRollsBackAndSameRequestCanRetry(t *testing.T) {
	f := newRoastingPartialFixture(t)
	p := f.preview(t)
	path := fmt.Sprintf("/api/job-cards/%d/complete", f.cardID)
	first := map[string]any{"request_id": "rollback-first", "expected_version": p.Version,
		"input_unit": p.InputUnit, "output_unit": p.OutputUnit, "actual_input_qty": 1,
		"actual_output_qty": .8, "finish_operation": false}
	res := serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, first)
	if res.Code != http.StatusOK {
		t.Fatalf("first partial status=%d body=%s", res.Code, res.Body.String())
	}
	p = f.preview(t)
	before := f.state(t)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`
		CREATE FUNCTION %[1]s.reject_roasting_completion_audit() RETURNS trigger AS $body$
		BEGIN
			IF NEW.entity_type='job_card' AND NEW.action='complete' THEN
				RAISE EXCEPTION 'forced roasting completion audit failure';
			END IF;
			RETURN NEW;
		END
		$body$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_roasting_completion_audit BEFORE INSERT ON %[1]s.audit_logs
		FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_roasting_completion_audit();
	`, f.schema))
	second := map[string]any{"request_id": "rollback-second", "expected_version": p.Version,
		"output_unit": p.OutputUnit, "actual_output_qty": p.PlannedQty, "finish_operation": true}
	res = serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, second)
	if res.Code != http.StatusBadRequest || !strings.Contains(res.Body.String(), "forced roasting completion audit failure") {
		t.Fatalf("audit failure status=%d body=%s", res.Code, res.Body.String())
	}
	if after := f.state(t); after != before {
		t.Fatalf("failed completion must roll back inventory, consumption, output, cost and audit: before=%+v after=%+v", before, after)
	}
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`DROP TRIGGER reject_roasting_completion_audit ON %s.audit_logs`, f.schema))
	res = serveMultilevelProductionJSON(t, f.app, http.MethodPost, path, second)
	if res.Code != http.StatusOK {
		t.Fatalf("same request after rollback status=%d body=%s", res.Code, res.Body.String())
	}
	if got := f.state(t); got.ConsumedG != 6000 || got.WIPG != 4000 || got.MaterialCost != 600 || got.WorkOrderCost != 600 || got.ReceiptG != 5000 || got.Events != 2 {
		t.Fatalf("retry must consume remaining 5kg once and finish with total cost 600: %+v", got)
	}
}
