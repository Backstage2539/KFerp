package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestReceiptCorrectionMisreportAndAudit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,bom_spec_id,spec_g,need_g,input_g,planned_units,order_nos,status,started_by) VALUES(726,'PR726',7,'初晓',19,0,0,230,1,'PR726','running','测试');
 INSERT INTO %[1]s.work_orders(id,work_order_no,running_item_id,product_id,output_product_id,product_name,output_name,bom_spec_id,output_qty,output_unit,inventory_unit,planned_inventory_qty,status,target_warehouse,sales_spec_snapshot_json) VALUES(726,'WO-726',726,7,7,'初晓','初晓',19,1,'袋','袋',1,'partially_completed','finished_goods','{"spec_label":"227g袋装"}');
 INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,status,actual_input_qty,actual_output_qty,actual_minutes,completed_at,metrics_json) VALUES(726,726,1,'包装','completed',14,14,25,now(),'{"quantity_basis":"inventory_unit","inventory_unit":"袋"}');
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	read := func() map[string]any {
		t.Helper()
		res := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/finished-receipts?status=all", nil)
		if res.Code != 200 {
			t.Fatalf("list %d %s", res.Code, res.Body.String())
		}
		var body struct {
			Rows []map[string]any `json:"rows"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Rows) != 1 {
			t.Fatalf("rows %+v", body.Rows)
		}
		return body.Rows[0]
	}
	row := read()
	if row["available_qty"] != float64(0) || !strings.Contains(fmt.Sprint(row["blocked_reason"]), "报工") {
		t.Fatalf("misreport must be blocked before posting: %+v", row)
	}
	if row["package_spec_label"] != "227g袋装" || row["target_warehouse_name"] != "成品仓" || row["inventory_owner_customer_id"] != float64(0) {
		t.Fatalf("frozen display: %+v", row)
	}
	version := row["receipt_version"]
	for _, bad := range []map[string]any{
		{"reason": "误报", "expected_version": version, "request_id": "missing-quantity"},
		{"actual_output_qty": 1, "reason": "", "expected_version": version, "request_id": "missing-reason"},
		{"actual_output_qty": 1.5, "reason": "误报", "expected_version": version, "request_id": "fraction"},
		{"actual_output_qty": 2, "reason": "误报", "expected_version": version, "request_id": "over-plan"},
		{"actual_output_qty": 1, "reason": "误报", "expected_version": "stale", "request_id": "stale"},
	} {
		r := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/produce/job-cards/726/correct-output", bad)
		if r.Code != 400 {
			t.Fatalf("invalid correction accepted %d %s", r.Code, r.Body.String())
		}
	}
	payload := map[string]any{"actual_output_qty": 1, "reason": "现场核对原14袋误报", "expected_version": version, "request_id": "correction-726"}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/produce/job-cards/726/correct-output", payload)
		if res.Code != 200 {
			t.Fatalf("correct/replay %d %s", res.Code, res.Body.String())
		}
	}
	row = read()
	if row["available_qty"] != float64(1) || row["reported_qty"] != float64(1) {
		t.Fatalf("corrected context %+v", row)
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 0)
	assertProductionFlowCount(t, pool, schema, "job_cards", "id=726 AND actual_input_qty=14 AND actual_minutes=25 AND actual_output_qty=1", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='job_card' AND entity_id=726 AND action='correct_output' AND old_value='14' AND new_value='1'", 1)
	payload["actual_output_qty"] = 0
	if res := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/produce/job-cards/726/correct-output", payload); res.Code != 400 {
		t.Fatalf("same key different content %d", res.Code)
	}
	legacy := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/produce/job-cards/726/actuals", map[string]any{"actual_output_qty": 9, "actual_input_qty": 14})
	if legacy.Code != 400 {
		t.Fatalf("legacy bypass %d %s", legacy.Code, legacy.Body.String())
	}
}

func TestReceiptCorrectionPartialFinalAndReplay(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72600)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,sales_spec_snapshot_json='{"spec_label":"227g袋装"}' WHERE id=%[2]d; UPDATE %[1]s.job_cards SET actual_output_qty=100,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"袋","finished_units":100}' WHERE work_order_id=%[2]d;`, schema, woID))
	read := func() map[string]any {
		t.Helper()
		res := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/finished-receipts?status=all", nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var body struct {
			Rows []map[string]any `json:"rows"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		for _, row := range body.Rows {
			if row["work_order_id"] == float64(woID) {
				return row
			}
		}
		t.Fatal("missing row")
		return nil
	}
	path := fmt.Sprintf("/api/produce/work-orders/%d/complete", woID)
	first := read()
	payload := map[string]any{"finished_units": 40, "completion_mode": "partial", "request_id": "receipt-partial-726", "expected_receipt_version": first["receipt_version"]}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, app, http.MethodPost, path, payload)
		if res.Code != 200 {
			t.Fatalf("partial/replay %d %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt' AND status='submitted'", woID), 1)
	next := read()
	if next["available_qty"] != float64(60) || next["reported_qty"] != float64(100) {
		t.Fatalf("partial context %+v", next)
	}
	var cardID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no DESC LIMIT 1`, schema), woID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	correction := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/job-cards/%d/correct-output", cardID), map[string]any{"actual_output_qty": 80, "reason": "不得修改已入库历史", "request_id": "after-receipt", "expected_version": next["receipt_version"]})
	if correction.Code != 400 || !strings.Contains(correction.Body.String(), "已有入库") {
		t.Fatalf("correction after receipt %d %s", correction.Code, correction.Body.String())
	}
	payload["request_id"] = "receipt-stale-726"
	if res := serveMultilevelProductionJSON(t, app, http.MethodPost, path, payload); res.Code != 400 {
		t.Fatalf("stale posting accepted %d", res.Code)
	}
	final := map[string]any{"finished_units": 60, "completion_mode": "final", "request_id": "receipt-final-726", "expected_receipt_version": next["receipt_version"]}
	for i := 0; i < 2; i++ {
		res := serveMultilevelProductionJSON(t, app, http.MethodPost, path, final)
		if res.Code != 200 {
			t.Fatalf("final/replay %d %s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("work_order_id=%d AND actual_output_qty=100", woID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", woID), 1)
	state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedReceiptCount != 2 || state.FinishedInventoryUnits != 100 || state.FinishedInventoryG != 22700 || state.ReservationConsumedG != 22700 || state.ReservationConsumedUnits != 100 {
		t.Fatalf("receipts %+v", state)
	}
	var units int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(finished_units)::bigint FROM %s.production_logs WHERE running_item_id=$1`, schema), runID).Scan(&units); err != nil || units != 100 {
		t.Fatalf("logged units %d %v", units, err)
	}
}

func TestReceiptCorrectionConcurrency(t *testing.T) {
	for _, scenario := range []string{"same request", "different receipt requests", "correction versus receipt"} {
		t.Run(scenario, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			app, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72700)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=%[2]d; UPDATE %[1]s.job_cards SET actual_output_qty=100,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"袋","finished_units":100}' WHERE work_order_id=%[2]d;`, schema, woID))
			res := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/finished-receipts?status=all", nil)
			var body struct {
				Rows []map[string]any `json:"rows"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 {
				t.Fatalf("list %s %v", res.Body.String(), err)
			}
			version := body.Rows[0]["receipt_version"]
			paths := []string{fmt.Sprintf("/api/produce/work-orders/%d/complete", woID), fmt.Sprintf("/api/produce/work-orders/%d/complete", woID)}
			payloads := []map[string]any{{"finished_units": 20, "completion_mode": "partial", "request_id": "concurrent-1", "expected_receipt_version": version}, {"finished_units": 20, "completion_mode": "partial", "request_id": "concurrent-1", "expected_receipt_version": version}}
			if scenario == "different receipt requests" {
				payloads[1]["request_id"] = "concurrent-2"
			}
			if scenario == "correction versus receipt" {
				var cardID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY sequence_no DESC LIMIT 1`, schema), woID).Scan(&cardID); err != nil {
					t.Fatal(err)
				}
				paths[1] = fmt.Sprintf("/api/produce/job-cards/%d/correct-output", cardID)
				payloads[1] = map[string]any{"actual_output_qty": 80, "reason": "核实数量", "request_id": "correct-concurrent", "expected_version": version}
			}
			var wg sync.WaitGroup
			codes := make([]int, 2)
			texts := make([]string, 2)
			start := make(chan struct{})
			for i := range paths {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					r := serveMultilevelProductionJSON(t, app, http.MethodPost, paths[i], payloads[i])
					codes[i] = r.Code
					texts[i] = r.Body.String()
				}(i)
			}
			close(start)
			wg.Wait()
			success := 0
			for _, code := range codes {
				if code == 200 {
					success++
				} else if code != 400 {
					t.Fatalf("unexpected %v %v", codes, texts)
				}
			}
			wanted := 1
			if scenario == "same request" {
				wanted = 2
			}
			if success != wanted {
				t.Fatalf("success %v %v", codes, texts)
			}
			var receipts int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_entries WHERE work_order_id=$1 AND entry_type='finished_receipt' AND status='submitted'`, schema), woID).Scan(&receipts); err != nil {
				t.Fatal(err)
			}
			if receipts > 1 {
				t.Fatalf("duplicate inventory entries %d", receipts)
			}
		})
	}
}
