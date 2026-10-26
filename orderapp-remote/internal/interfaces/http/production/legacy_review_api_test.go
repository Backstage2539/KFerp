package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	stockrepo "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestLegacyReviewAPIRequiresActualQuantityAndPostsOnce(t *testing.T) {
	testLegacyReview(t, false, false)
}

func TestRecordedOutputReceiptUsesSavedQuantityAndReplaysAfterItLeavesPendingList(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72890)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=1,output_unit='袋',inventory_unit='袋',planned_inventory_qty=1 WHERE id=%[2]d;UPDATE %[1]s.job_cards SET actual_output_qty=14,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"袋"}' WHERE work_order_id=%[2]d`, schema, woID))
	preview := serveMultilevelProductionJSON(t, e, "POST", "/api/production/legacy-review/preview", map[string]any{"work_order_ids": []int64{woID}})
	if preview.Code != http.StatusOK {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var body struct {
		Rows []app.LegacyReview `json:"rows"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 || body.Rows[0].ReportedQty != 14 {
		t.Fatalf("recorded output preview %s (%v)", preview.Body.String(), err)
	}
	payload := map[string]any{"expected_version": body.Rows[0].Version, "request_id": "recorded-output-direct-72890"}
	for i := 0; i < 2; i++ {
		result := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/work-orders/%d/receive-completed-output", woID), payload)
		if result.Code != http.StatusOK {
			t.Fatalf("direct receipt/replay %d: %s", result.Code, result.Body.String())
		}
	}
	var actual float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT actual_output_qty FROM %s.job_cards WHERE work_order_id=$1`, schema), woID).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != 14 {
		t.Fatalf("recorded output was overwritten: got %g want 14", actual)
	}
	state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedInventoryUnits != 14 || state.FinishedReceiptCount != 1 || state.ReservationConsumedG != 0 || state.ReservationConsumedUnits != 0 {
		t.Fatalf("direct receipt state %+v", state)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "action='recorded_output_receipt'", 1)
}

func TestRecordedOutputReceiptDoesNotReconsumeMaterialsWhenWIPIsShort(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72892)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=5.6,output_unit='kg',inventory_unit='kg',planned_inventory_qty=5.6 WHERE id=%[2]d;
UPDATE %[1]s.job_cards SET actual_output_qty=5.6,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"kg"}' WHERE work_order_id=%[2]d;
UPDATE %[1]s.material_batch_locations SET qty_g=22520 WHERE material_id=10 AND warehouse='wip';
UPDATE %[1]s.materials SET onhand_g=22520 WHERE id=10`, schema, woID))
	before := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	preview := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/legacy-review/preview", map[string]any{"work_order_ids": []int64{woID}})
	if preview.Code != http.StatusOK {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var body struct {
		Rows []app.LegacyReview `json:"rows"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 {
		t.Fatalf("recorded output preview %s (%v)", preview.Body.String(), err)
	}
	payload := map[string]any{"expected_version": body.Rows[0].Version, "request_id": "recorded-output-short-wip-72892"}
	result := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/receive-completed-output", woID), payload)
	if result.Code != http.StatusOK {
		t.Fatalf("direct receipt must not require raw WIP stock after the job card is complete: %s", result.Body.String())
	}
	var receiptBody struct {
		Completion app.OperationCompletionReceipt `json:"completion"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &receiptBody); err != nil {
		t.Fatalf("decode direct receipt: %s (%v)", result.Body.String(), err)
	}
	if receiptBody.Completion.OutputQty != 5.6 || receiptBody.Completion.OutputUnit != "kg" || !strings.Contains(receiptBody.Completion.InventoryWarning, "没有再次扣料") {
		t.Fatalf("receipt must return saved output and historical material warning: %+v", receiptBody.Completion)
	}
	after := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if after.FinishedInventoryG-before.FinishedInventoryG != 5600 || after.FinishedReceiptCount-before.FinishedReceiptCount != 1 {
		t.Fatalf("saved completed output was not received exactly once: before=%+v after=%+v", before, after)
	}
	if after.MaterialLocationG != before.MaterialLocationG || after.ReservationConsumedG != before.ReservationConsumedG || after.ReservationConsumedUnits != before.ReservationConsumedUnits {
		t.Fatalf("receipt must not reconsume or rewrite WIP materials: before=%+v after=%+v", before, after)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "purpose='material_consumption_for_manufacture' AND status='submitted'", 0)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt' AND status='submitted'", 1)
}

func TestRecordedWeightOutputWithoutPackageSpecReceiptsAsGrams(t *testing.T) {
	for _, tc := range []struct {
		unit string
		qty  float64
	}{
		{unit: "kg", qty: 5.6},
		{unit: "g", qty: 5600},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72893)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
				UPDATE %[1]s.work_orders
				SET output_qty=%[4]g,output_unit='%[5]s',inventory_unit='%[5]s',planned_inventory_qty=%[4]g,
				    spec_g=0,bom_spec_id=0,bom_variant_id=0,target_warehouse='finished_goods'
				WHERE id=%[2]d;
				UPDATE %[1]s.produce_running_items SET spec_g=0,bom_spec_id=0,bom_variant_id=0 WHERE id=%[3]d;
				UPDATE %[1]s.job_cards SET actual_output_qty=%[4]g,actual_operation_cost=56,
				    metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"%[5]s"}'
				WHERE work_order_id=%[2]d;
				UPDATE %[1]s.material_batches SET unit_cost=90 WHERE material_id=10;
			`, schema, woID, runID, tc.qty, tc.unit))
			// Carry the already-posted 504 material cost and 56 operation cost
			// into the receipt without consuming the same materials again.
			_, err := stockrepo.NewRepository(pool, schema).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
				Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: woID, Operator: "历史现场",
				Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 10, FromWarehouse: "wip", QtyG: 5600}},
			})
			if err != nil {
				t.Fatal(err)
			}
			before := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
			preview := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/legacy-review/preview", map[string]any{"work_order_ids": []int64{woID}})
			if preview.Code != http.StatusOK {
				t.Fatalf("recorded weight output preview %d %s", preview.Code, preview.Body.String())
			}
			var body struct {
				Rows []app.LegacyReview `json:"rows"`
			}
			if err := json.Unmarshal(preview.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 || body.Rows[0].ReportedQty != tc.qty {
				t.Fatalf("recorded weight output preview %s (%v)", preview.Body.String(), err)
			}
			payload := map[string]any{"expected_version": body.Rows[0].Version, "request_id": "recorded-weight-output-no-spec-72893"}
			for attempt := 0; attempt < 2; attempt++ {
				result := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/receive-completed-output", woID), payload)
				if result.Code != http.StatusOK {
					t.Fatalf("direct weight receipt/replay %d: %s", result.Code, result.Body.String())
				}
				assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=0 AND bom_variant_id=0 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=0 AND onhand_loose_g=5600", 1)
				assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='production_operation_event' AND qty_before_g=0 AND qty_change_g=5600 AND qty_after_g=5600 AND qty_before_units=0 AND qty_change_units=0 AND qty_after_units=0", 1)
				assertProductionFlowCount(t, pool, schema, "stock_entries", "work_order_id="+fmt.Sprint(woID)+" AND entry_type='finished_receipt' AND status='submitted'", 1)
				assertProductionFlowCount(t, pool, schema, "stock_entry_items", fmt.Sprintf("qty_g=5600 AND qty_units=0 AND unit_cost=100 AND total_cost=560 AND stock_entry_id IN (SELECT id FROM %s.stock_entries WHERE work_order_id=%d AND entry_type='finished_receipt' AND status='submitted')", schema, woID), 1)
				assertProductionFlowCount(t, pool, schema, "stock_batches", "source_doc_type='production_operation_event' AND item_id=1 AND spec_g=0 AND qty_g=5600 AND remaining_g=5600 AND qty_units=0 AND remaining_units=0 AND unit_cost=100", 1)
				assertProductionFlowCount(t, pool, schema, "production_logs", fmt.Sprintf("running_item_id=%d AND finished_units=0 AND finished_loose_g=5600 AND finished_total_g=5600", runID), 1)
				assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=%d AND material_cost=504 AND operation_cost=56 AND total_cost=560 AND finished_g=5600 AND unit_cost_per_kg=100", runID), 1)
				assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed' AND actual_cost=560", woID), 1)
				assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted' AND material_cost=504 AND operation_cost=56", 1)
				assertProductionFlowCount(t, pool, schema, "audit_logs", "action='recorded_output_receipt'", 1)
				assertProductionFlowCount(t, pool, schema, "stock_entries", "purpose='material_consumption_for_manufacture' AND status='submitted'", 1)
				after := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
				if after.MaterialLocationG != before.MaterialLocationG || after.MaterialLocationUnits != before.MaterialLocationUnits || after.ReservationConsumedG != before.ReservationConsumedG || after.ReservationConsumedUnits != before.ReservationConsumedUnits {
					t.Fatalf("receipt/replay must not reconsume materials: before=%+v after=%+v", before, after)
				}
			}
		})
	}
}

func TestLegacyReviewAPICreditsExistingConsumption(t *testing.T) { testLegacyReview(t, true, false) }
func TestLegacyReviewAPICreditsLegacyLogsCost(t *testing.T)      { testLegacyReview(t, true, true) }
func testLegacyReview(t *testing.T, credit, logsOnly bool) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72891)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=1,output_unit='袋',inventory_unit='袋',planned_inventory_qty=1 WHERE id=%[2]d;UPDATE %[1]s.job_cards SET actual_output_qty=14,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"袋"}' WHERE work_order_id=%[2]d`, schema, woID))
	if credit {
		_, err := stockrepo.NewRepository(pool, schema).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: woID, Operator: "历史现场", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 10, FromWarehouse: "wip", QtyG: 227}, {ItemType: "material", MaterialID: 20, FromWarehouse: "wip", QtyUnits: 1}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	var creditedCost float64
	if logsOnly {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=100 WHERE material_id=10`, schema))
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.material_consumption_logs(running_item_id,batch_id,product_id,product_name,spec_g,material_id,material_name,unit,deduct_g,deduct_units,before_g,after_g,before_units,after_units,operator,material_batch_id,material_batch_code) SELECT %[2]d,0,0,'测试',227,l.item_id,l.item_name,'g',-l.qty_change_g,-l.qty_change_units,l.qty_before_g,l.qty_after_g,l.qty_before_units,l.qty_after_units,'历史',b.id,b.batch_code FROM %[1]s.stock_ledger_entries l JOIN %[1]s.material_batches b ON b.batch_code=l.source_batch_code AND b.material_id=l.item_id WHERE l.qty_change_g<0 OR l.qty_change_units<0`, schema, runID))
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(CASE WHEN l.deduct_g>0 THEN l.deduct_g*b.unit_cost/1000 ELSE l.deduct_units*b.unit_cost END)::float8 FROM %s.material_consumption_logs l JOIN %s.material_batches b ON b.id=l.material_batch_id WHERE l.running_item_id=$1`, schema, schema), runID).Scan(&creditedCost); err != nil {
			t.Fatal(err)
		}
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.stock_entries SET status='draft' WHERE work_order_id=%[2]d;DELETE FROM %[1]s.production_batch_costs WHERE running_item_id=%[3]d`, schema, woID, runID))
	}
	res := serveMultilevelProductionJSON(t, e, "POST", "/api/production/legacy-review/preview", map[string]any{"work_order_ids": []int64{woID}})
	if res.Code != 200 {
		t.Fatalf("preview %d %s", res.Code, res.Body.String())
	}
	var body struct {
		Rows []app.LegacyReview `json:"rows"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rows) != 1 || body.Rows[0].ReportedQty != 14 {
		t.Fatal(res.Body.String())
	}
	row := body.Rows[0]
	materials := []app.OperationMaterialUse{}
	for _, m := range row.Materials {
		use := m.OperationMaterialUse
		if use.MaterialID == 10 {
			use.Qty = .227
		} else {
			use.Qty = 1
		}
		materials = append(materials, use)
	}
	payload := map[string]any{"work_order_id": woID, "actual_input_g": 227, "materials": materials, "reason": "现场核实实际1袋，旧14袋误填", "request_id": "legacy-728", "expected_version": row.Version}
	rejected := serveMultilevelProductionJSON(t, e, "POST", "/api/production/legacy-review/submit", map[string]any{"items": []any{payload}})
	if rejected.Code != 200 {
		t.Fatal(rejected.Body.String())
	}
	var rejectedBody struct {
		Rows []app.LegacyReviewResult `json:"rows"`
	}
	_ = json.Unmarshal(rejected.Body.Bytes(), &rejectedBody)
	if len(rejectedBody.Rows) != 1 || rejectedBody.Rows[0].Succeeded {
		t.Fatal("missing quantity must not infer plan")
	}
	payload["actual_output_qty"] = 1
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, "POST", "/api/production/legacy-review/submit", map[string]any{"items": []any{payload}})
		var result struct {
			Rows []app.LegacyReviewResult `json:"rows"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || len(result.Rows) != 1 || !result.Rows[0].Succeeded {
			t.Fatalf("submit/replay %s %v", res.Body.String(), err)
		}
	}
	state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedInventoryUnits != 1 || state.ReservationConsumedG != 227 || state.ReservationConsumedUnits != 1 {
		t.Fatalf("legacy reconciliation %+v", state)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "action='legacy_reconcile'", 1)
	if logsOnly {
		var actualCost float64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_cost::float8 FROM %s.production_operation_events WHERE work_order_id=$1`, schema), woID).Scan(&actualCost); err != nil {
			t.Fatal(err)
		}
		if creditedCost <= 0 || actualCost != creditedCost {
			t.Fatalf("legacy cost lost: got %g want %g", actualCost, creditedCost)
		}
		return
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "purpose='material_consumption_for_manufacture' AND status='submitted'", 1)
}
