package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
	"strings"
	"sync"
	"testing"
)

func TestOperationCompletionAPIEarlyHandoffAndConcurrentReplay(t *testing.T) {
	testOperationHandoffConcurrency(t, false)
}
func TestOperationCompletionAPICompetingDownstreamBatches(t *testing.T) {
	testOperationHandoffConcurrency(t, true)
}

func TestOperationCompletionUsesFrozenParentWhenLegacyOutputSKUWasRetired(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72800)
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"roast\",\"receipt_enabled\":true,\"receipt_warehouse\":\"wip\",\"output_unit\":\"kg\"}"}]}`

	// Model an old production order whose output points at a retired child SKU,
	// while the frozen parent product and BOM specification remain valid.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		VALUES(900,100,'SPEC-227','227g','227g袋装','袋');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)
		VALUES(901,100,900,'227g袋装','袋',true);
		UPDATE %[1]s.order_items SET bom_spec_id=900,bom_variant_id=901 WHERE order_id=1 AND product_id=1;
		UPDATE %[1]s.production_plan_items SET bom_spec_id=900,bom_variant_id=901 WHERE product_id=1;
		UPDATE %[1]s.work_orders SET output_product_id=765,product_id=1,bom_spec_id=900,bom_variant_id=901,spec_g=227,
			output_qty=22.7,output_unit='kg',inventory_unit='kg',planned_inventory_qty=22.7,planned_output_g=22700,
			target_warehouse='wip',sales_spec_snapshot_json='{"spec_label":"227g袋装"}'
		WHERE id=%[3]d;
		UPDATE %[1]s.produce_running_items SET product_id=1,bom_spec_id=900,bom_variant_id=901,spec_g=227 WHERE id=%[2]d;
		UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[3]d;
	`, schema, runID, woID))
	if err := postgresproductspecmigration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatalf("install product identity guards: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}

	var cardID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), woID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	previewResponse := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("completion preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview productionapp.OperationCompletionPreview
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Enabled || preview.BlockedReason != "" {
		t.Fatalf("valid frozen parent identity must keep completion enabled: %+v", preview)
	}
	request := map[string]any{
		"actual_input_qty": preview.InputQty, "actual_output_qty": 22.7,
		"input_unit": preview.InputUnit, "output_unit": preview.OutputUnit,
		"expected_version": preview.Version, "request_id": "retired-output-sku-completion",
		"materials": preview.Materials,
	}
	for attempt := 0; attempt < 2; attempt++ {
		response := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", cardID), request)
		if response.Code != http.StatusOK {
			t.Fatalf("completion attempt %d status=%d body=%s", attempt+1, response.Code, response.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt' AND status='submitted'", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND spec_g=227 AND warehouse='wip' AND onhand_units=0 AND onhand_loose_g=22700", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=765", 0)
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", "product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND qty_g=22700", 1)
	assertProductionFlowCount(t, pool, schema, "production_logs", "product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND finished_loose_g=22700", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND consumed_g=22700", woID), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", woID), 1)
}

func testOperationHandoffConcurrency(t *testing.T, competing bool) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72811)
	execution := func(key, unit string, receipt bool) string {
		raw, _ := json.Marshal(map[string]any{"version": 1, "node_key": key, "receipt_enabled": receipt, "receipt_warehouse": map[string]string{"roast": "raw_materials", "pack": "finished_goods"}[key], "output_unit": unit})
		return string(raw)
	}
	snapshot, _ := json.Marshal(map[string]any{"operations": []any{map[string]any{"seq": 1, "execution_json": execution("roast", "g", true)}, map[string]any{"seq": 2, "execution_json": execution("pack", "袋", true)}}})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings='{"material:10:0:0":{"node_key":"roast"},"material:20:0:0":{"node_key":"pack"}}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, string(snapshot)); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,operation='前序',planned_input_qty=11350,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d;
 INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%d,1,'前序','pending',11350),(%d,2,'包装','pending',11350),(%d,2,'包装','pending',11350)`, schema, woID, schema, woID, woID, woID))
	var first, last int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FILTER(WHERE sequence_no=1),MIN(id) FILTER(WHERE sequence_no=2) FROM %s.job_cards WHERE work_order_id=$1`, schema), woID).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	read := func(id int64) productionapp.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", id), nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var p productionapp.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := read(first)
	if p.BlockedReason != "" || len(p.Materials) != 1 || p.Materials[0].MaterialID != 10 {
		t.Fatalf("first %+v", p)
	}
	res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", first), map[string]any{"actual_input_qty": 11350, "actual_output_qty": 11350, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "first-batch", "materials": p.Materials})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 0)
	p = read(last)
	if len(p.PredecessorBatches) != 1 || p.PredecessorBatches[0].Warehouse != "raw_materials" {
		t.Fatalf("actual intermediate location lost: %+v", p.PredecessorBatches)
	}
	if p.BlockedReason != "" || len(p.Materials) != 1 || p.Materials[0].MaterialID != 20 || p.InputQty != 11350 {
		t.Fatalf("next batch %+v", p)
	}
	payload := map[string]any{"actual_input_qty": 11350, "actual_output_qty": 50, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "last-batch", "materials": p.Materials}
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		id := last
		version := p.Version
		if competing && i == 1 {
			id = last + 1
			version = read(id).Version
		}
		request := map[string]any{}
		for key, value := range payload {
			request[key] = value
		}
		if competing {
			request["request_id"] = fmt.Sprintf("competing-%d", id)
			request["expected_version"] = version
		}
		wg.Add(1)
		go func(id int64, request map[string]any) {
			defer wg.Done()
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", id), request)
			statuses <- res.Code
		}(id, request)
	}
	wg.Wait()
	close(statuses)
	successes, rejected := 0, 0
	for code := range statuses {
		if code == 200 {
			successes++
		} else if code == 400 {
			rejected++
		} else {
			t.Errorf("unexpected concurrency status %d", code)
		}
	}
	if competing {
		if successes != 1 || rejected != 1 {
			t.Fatalf("shared batch must post once: success=%d rejected=%d", successes, rejected)
		}
	} else if successes != 2 {
		t.Fatalf("replay successes=%d", successes)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 2)
	assertProductionFlowCount(t, pool, schema, "production_operation_balances", "remaining_qty=0 AND warehouse='raw_materials'", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_movements", "warehouse='raw_materials' AND qty=11350", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=10 AND consumed_g=11350", woID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=20 AND consumed_units=50", woID), 1)
	rows := serveMultilevelProductionJSON(t, e, "GET", "/api/production/records", nil)
	var records struct {
		Rows []productionapp.OperationRecord `json:"rows"`
	}
	if err := json.Unmarshal(rows.Body.Bytes(), &records); err != nil {
		t.Fatal(err)
	}
	for _, row := range records.Rows {
		if row.JobCardID == first {
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", first), map[string]any{"actual_output_qty": 10000, "reason": "复核", "expected_version": row.Version, "request_id": "used-upstream-correct"})
			if res.Code != 400 {
				t.Fatalf("used output correction must be rejected: %s", res.Body.String())
			}
		}
	}
}

func TestOperationCompletionAPIAtomicReceiptAndReplay(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72801)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.order_process_statuses(name,sort,active) VALUES('生产完成',30,true) ON CONFLICT(name) DO NOTHING`, schema))
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,sales_spec_snapshot_json='{"spec_label":"227g袋装"}' WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, woID))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if res.Code != 200 {
		t.Fatalf("preview %d %s", res.Code, res.Body.String())
	}
	var preview productionapp.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Enabled || preview.PackageSpecLabel != "227g袋装" || preview.WarehouseName != "成品仓" || len(preview.Materials) != 2 {
		t.Fatalf("preview %+v", preview)
	}
	payload := map[string]any{"actual_input_qty": preview.InputQty, "actual_output_qty": 100, "input_unit": preview.InputUnit, "output_unit": preview.OutputUnit, "expected_version": preview.Version, "request_id": "completion-728", "materials": preview.Materials}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
		if res.Code != 200 {
			t.Fatalf("complete/replay %d %s", res.Code, res.Body.String())
		}
	}
	state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedReceiptCount != 1 || state.FinishedInventoryUnits != 100 || state.ReservationConsumedG != 22700 || state.ReservationConsumedUnits != 100 {
		t.Fatalf("atomic receipt %+v", state)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", woID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='job_card' AND action='complete'", 1)
	assertProductionFlowCount(t, pool, schema, "orders", fmt.Sprintf("order_no='SO-ML-227' AND process_status_id=(SELECT id FROM %s.order_process_statuses WHERE name='生产完成')", schema), 1)
	payload["actual_output_qty"] = 99
	if res = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), payload); res.Code != 400 {
		t.Fatalf("changed retry %d %s", res.Code, res.Body.String())
	}
	records := serveMultilevelProductionJSON(t, e, "GET", "/api/production/records", nil)
	if records.Code != 200 {
		t.Fatalf("records %d %s", records.Code, records.Body.String())
	}
	var recordBody struct {
		Rows []struct {
			Version string `json:"version"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(records.Body.Bytes(), &recordBody); err != nil {
		t.Fatal(err)
	}
	if len(recordBody.Rows) != 1 {
		t.Fatal(records.Body.String())
	}
	// A processing reservation can exist without a generic reservation row.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.customer_processing_material_reservations(id BIGSERIAL PRIMARY KEY,finished_stock_batch_id BIGINT DEFAULT 0,material_id BIGINT DEFAULT 0,component_product_id BIGINT DEFAULT 0,reserved_g BIGINT DEFAULT 0,reserved_units BIGINT DEFAULT 0,consumed_g BIGINT DEFAULT 0,consumed_units BIGINT DEFAULT 0,returned_g BIGINT DEFAULT 0,returned_units BIGINT DEFAULT 0,status TEXT DEFAULT 'reserved'); INSERT INTO %[1]s.customer_processing_material_reservations(finished_stock_batch_id,reserved_units) SELECT id,1 FROM %[1]s.stock_batches WHERE qty_units=100`, schema))
	blocked := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", card), map[string]any{"actual_output_qty": 98, "reason": "复点", "expected_version": recordBody.Rows[0].Version, "request_id": "processing-blocked-correction"})
	if blocked.Code != 400 || !strings.Contains(blocked.Body.String(), "加工预留") {
		t.Fatalf("processing reservation correction allowed: %d %s", blocked.Code, blocked.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DROP TABLE %s.customer_processing_material_reservations`, schema))
	var completedBefore string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT completed_at::text FROM %s.work_orders WHERE id=$1`, schema), woID).Scan(&completedBefore); err != nil {
		t.Fatal(err)
	}
	correction := map[string]any{"actual_output_qty": 98, "reason": "现场复点为98袋", "expected_version": recordBody.Rows[0].Version, "request_id": "output-correction-728"}
	for i := 0; i < 2; i++ {
		res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/produce/job-cards/%d/correct-output", card), correction)
		if res.Code != 200 {
			t.Fatalf("correction %d %s", res.Code, res.Body.String())
		}
	}
	var completedAfter string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT completed_at::text FROM %s.work_orders WHERE id=$1`, schema), woID).Scan(&completedAfter); err != nil {
		t.Fatal(err)
	}
	if completedBefore != completedAfter {
		t.Errorf("correction changed completion time: %s -> %s", completedBefore, completedAfter)
	}
	state = loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedInventoryUnits != 98 || state.ReservationConsumedG != 22700 || state.ReservationConsumedUnits != 100 {
		t.Fatalf("correction only adjusts output difference %+v", state)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND output_qty=100", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='correct' AND output_qty=98", 1)
	var reported int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(finished_units),0)::bigint FROM %s.production_logs WHERE running_item_id=$1`, schema), runID).Scan(&reported); err != nil {
		t.Fatal(err)
	}
	if reported != 98 {
		t.Fatalf("correction stock and production history differ: %d", reported)
	}

}

func TestOperationCompletionAPIPartialOutputCanContinueAndFinish(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72809)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.order_process_statuses(name,sort,active) VALUES('生产完成',30,true) ON CONFLICT(name) DO NOTHING`, schema))
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,output_qty=12,output_unit='袋',inventory_unit='袋',planned_inventory_qty=12,sales_spec_snapshot_json='{"spec_label":"227g袋装"}' WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards SET status='pending',started_at=NULL,resumed_at=NULL,actual_minutes=0,completion_draft='{"actual_output_qty":1,"actual_minutes":1}',assigned_employee_id=999,assigned_to='旧排班',sequence_no=1,planned_input_qty=2724,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d;
		UPDATE %s.work_order_material_reservations SET required_g=CASE WHEN material_id=10 THEN 2724 ELSE required_g END,reserved_g=CASE WHEN material_id=10 THEN 227 ELSE reserved_g END,required_units=CASE WHEN material_id=20 THEN 12 ELSE required_units END,reserved_units=CASE WHEN material_id=20 THEN 1 ELSE reserved_units END WHERE work_order_id=%d;
		UPDATE %s.work_order_material_reservation_batches SET reserved_g=CASE WHEN material_id=10 THEN 227 ELSE reserved_g END,reserved_units=CASE WHEN material_id=20 THEN 1 ELSE reserved_units END WHERE work_order_id=%d`, schema, woID, schema, woID, schema, woID))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	read := func() productionapp.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
		if res.Code != http.StatusOK {
			t.Fatalf("preview status=%d body=%s", res.Code, res.Body.String())
		}
		var p productionapp.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := read()
	if first.PlannedQty != 12 || first.CompletedOutputQty != 0 || first.MaxOutputQty != 12 {
		t.Fatalf("initial remaining/max output = %g/%g completed=%g", first.PlannedQty, first.MaxOutputQty, first.CompletedOutputQty)
	}
	for _, material := range first.Materials {
		if material.AvailableQty < material.Qty {
			t.Fatalf("same-owner free stock in the configured warehouse should cover the previewed material: %+v", material)
		}
	}
	partial := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{
		"actual_input_qty": 227, "actual_output_qty": 1, "input_unit": first.InputUnit, "output_unit": first.OutputUnit,
		"expected_version": first.Version, "request_id": "partial-one-of-twelve", "finish_operation": false, "actual_minutes": 1,
	})
	if partial.Code != http.StatusOK {
		t.Fatalf("partial completion status=%d body=%s", partial.Code, partial.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND status='running' AND actual_output_qty=1", card), 1)
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND assigned_employee_id=1 AND assigned_to<>'旧排班'", card), 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted' AND output_qty=1", 1)
	state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedInventoryUnits != 1 || state.ReservationConsumedG != 227 || state.ReservationConsumedUnits != 1 || state.WorkOrderStatus != "partially_completed" {
		t.Fatalf("partial completion must receive and consume only one bag: %+v", state)
	}
	second := read()
	if second.ActualMinutes != 0 {
		t.Fatalf("next batch must not suggest already posted minutes: %d", second.ActualMinutes)
	}
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND completion_draft='{}'::jsonb", card), 1)
	if second.CompletedOutputQty != 1 || second.PlannedQty != 11 {
		t.Fatalf("next completion preview must retain 11 bags: planned=%g completed=%g", second.PlannedQty, second.CompletedOutputQty)
	}
	var suggestedG, suggestedUnits float64
	for _, material := range second.Materials {
		if factor := productionapp.OperationUnitFactor(material.Unit); factor > 0 {
			suggestedG += material.Qty * factor
		} else {
			suggestedUnits += material.Qty
		}
	}
	if suggestedG < 2497-1e-6 || suggestedG > 2497+1e-6 || suggestedUnits != 11 {
		t.Fatalf("next batch suggestion repeated or lost consumption: g=%g units=%g materials=%+v", suggestedG, suggestedUnits, second.Materials)
	}
	final := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{
		"actual_input_qty": 2497, "actual_output_qty": 11, "input_unit": second.InputUnit, "output_unit": second.OutputUnit,
		"expected_version": second.Version, "request_id": "finish-remaining-eleven", "finish_operation": true,
	})
	if final.Code != http.StatusOK {
		t.Fatalf("final completion status=%d body=%s", final.Code, final.Body.String())
	}
	state = loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
	if state.FinishedInventoryUnits != 12 || state.ReservationConsumedG != 2724 || state.ReservationConsumedUnits != 12 || state.WorkOrderStatus != "completed" {
		t.Fatalf("final cumulative output and inventory = %+v", state)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 2)
	assertProductionFlowCount(t, pool, schema, "orders", fmt.Sprintf("order_no='SO-ML-227' AND process_status_id=(SELECT id FROM %s.order_process_statuses WHERE name='生产完成')", schema), 0)
}

func TestOperationCompletionAPIVariancesDraftAndRollback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		output     float64
		stockError bool
	}{{"overproduction", 128, false}, {"underproduction", 98, false}, {"zero", 0, false}, {"fraction", 1.5, true}, {"shortage", 100, true}} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72851)
			if tc.name == "shortage" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
					UPDATE %s.material_batch_locations SET qty_g=10000 WHERE material_id=10 AND warehouse='wip';
					UPDATE %s.material_batches b SET remaining_g=10000
					WHERE EXISTS(SELECT 1 FROM %s.material_batch_locations l WHERE l.material_batch_id=b.id AND l.material_id=10 AND l.warehouse='wip');
				`, schema, schema, schema))
			}
			snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, woID))
			var card int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card); err != nil {
				t.Fatal(err)
			}
			read := func() productionapp.OperationCompletionPreview {
				res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
				if res.Code != 200 {
					t.Fatal(res.Body.String())
				}
				var p productionapp.OperationCompletionPreview
				if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
					t.Fatal(err)
				}
				return p
			}
			if tc.name == "underproduction" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET started_at=now()-interval '5 minutes',status='paused',metrics_json='{"timer_elapsed_seconds":180}',cost_method='hour',hourly_rate=60 WHERE id=%d`, schema, card))
			}
			p := read()
			if tc.name == "overproduction" {
				if len(p.Materials) != 2 || p.TotalPlannedQty <= 0 {
					t.Fatalf("overproduction preview lacks the bean and packaging rows: %+v", p)
				}
				for _, material := range p.Materials {
					if factor := productionapp.OperationUnitFactor(material.Unit); factor > 0 {
						targetG := int64(math.Round(material.TotalQty * tc.output / p.TotalPlannedQty * factor))
						plannedG := int64(math.Round(material.TotalQty * factor))
						extraG := targetG - plannedG
						if extraG <= 0 {
							continue
						}
						seedProductionFlowWIPBatch(t, ctx, pool, schema, 72854, material.MaterialID, "MB-ROASTED-EXTRA-72851", material.Name, extraG)
						if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=onhand_g+$2 WHERE id=$1`, schema), material.MaterialID, extraG); err != nil {
							t.Fatalf("increase bean master balance for seeded overproduction stock: %v", err)
						}
						if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET reserved_g=$2 WHERE id=(SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=$3 AND status='reserved' ORDER BY id LIMIT 1)`, schema, schema), woID, targetG, material.MaterialID); err != nil {
							t.Fatalf("extend bean reservation for physically available overproduction: %v", err)
						}
						if _, err := pool.Exec(ctx, fmt.Sprintf(`
							INSERT INTO %[1]s.work_order_material_reservation_batches(
								reservation_id,work_order_id,material_id,component_type,component_id,
								material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,status
							)
							SELECT r.id,r.work_order_id,$2,'material',$2,72854,'MB-ROASTED-EXTRA-72851','wip',0,$3,'reserved'
							FROM %[1]s.work_order_material_reservations r
							WHERE r.work_order_id=$1 AND r.material_id=$2 AND r.status='reserved'
							ORDER BY r.id LIMIT 1
						`, schema), woID, material.MaterialID, extraG); err != nil {
							t.Fatalf("extend bean reservation for physically available overproduction: %v", err)
						}
						continue
					}
					targetUnits := int64(math.Ceil(material.TotalQty*tc.output/p.TotalPlannedQty - 1e-9))
					plannedUnits := int64(math.Ceil(material.TotalQty - 1e-9))
					extraUnits := targetUnits - plannedUnits
					if extraUnits <= 0 {
						continue
					}
					seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 72855, material.MaterialID, "MB-PACK-EXTRA-72851", material.Name, extraUnits)
					if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_units=onhand_units+$2 WHERE id=$1`, schema), material.MaterialID, extraUnits); err != nil {
						t.Fatalf("increase packaging master balance for seeded overproduction stock: %v", err)
					}
					if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET reserved_units=$2 WHERE id=(SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=$3 AND status='reserved' ORDER BY id LIMIT 1)`, schema, schema), woID, targetUnits, material.MaterialID); err != nil {
						t.Fatalf("extend pack reservation for physically available overproduction: %v", err)
					}
					if _, err := pool.Exec(ctx, fmt.Sprintf(`
						INSERT INTO %[1]s.work_order_material_reservation_batches(
							reservation_id,work_order_id,material_id,component_type,component_id,
							material_batch_id,batch_code,warehouse,owner_customer_id,reserved_units,status
						)
						SELECT r.id,r.work_order_id,$2,'material',$2,72855,'MB-PACK-EXTRA-72851','wip',0,$3,'reserved'
						FROM %[1]s.work_order_material_reservations r
						WHERE r.work_order_id=$1 AND r.material_id=$2 AND r.status='reserved'
						ORDER BY r.id LIMIT 1
						`, schema), woID, material.MaterialID, extraUnits); err != nil {
						t.Fatalf("extend pack reservation for physically available overproduction: %v", err)
					}
				}
				p = read()
			}
			payload := map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": tc.output, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "actual_minutes": 0, "difference_reason": "现场实际已做完", "expected_version": p.Version, "request_id": "variance", "materials": p.Materials}
			if tc.name == "zero" {
				for i := range p.Materials {
					p.Materials[i].Qty = 0
				}
				payload["materials"] = p.Materials
				payload["actual_input_qty"] = 0
			}
			res := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/progress", card), payload)
			if res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_operation_events", "true", 0)
			state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
			if state.ReservationConsumedG != 0 || state.FinishedReceiptCount != 0 {
				t.Fatalf("draft changed inventory %+v", state)
			}
			p = read()
			payload["expected_version"] = p.Version
			res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
			if tc.stockError {
				if res.Code != 400 {
					t.Fatalf("expected rollback: %d %s", res.Code, res.Body.String())
				}
				assertProductionFlowCount(t, pool, schema, "production_operation_events", "true", 0)
				state = loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
				if state.ReservationConsumedG != 0 || state.FinishedReceiptCount != 0 {
					t.Fatalf("failed completion changed inventory %+v", state)
				}
				return
			}
			if res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", woID), 1)
			state = loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
			if tc.name == "underproduction" {
				assertProductionFlowCount(t, pool, schema, "production_operation_events", "estimated_cost=false AND operation_cost=3", 1)
			}
			if state.FinishedInventoryUnits != int64(tc.output) {
				t.Fatalf("actual output %+v", state)
			}
			p = read()
			if p.BlockedReason == "" {
				t.Fatal("completed card must not offer another completion")
			}
		})
	}
}

func TestOperationCompletionAPIProductComponentDirectWarehouse(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72871)
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings='{"product:90001:0:1000":{"node_key":"pack","source_warehouse":"finished_goods"}}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 INSERT INTO %[1]s.products(id,name) VALUES(90001,'现有大包装商品');
 INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(90001,1000,'finished_goods',30,0);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status,unit_cost) VALUES(90001,'PRODUCT-COMPONENT','finished_product',90001,'现有大包装商品',1000,30000,30,30000,30,'pass',50);
 UPDATE %[1]s.work_order_material_reservations SET component_type='product',material_id=0,component_id=90001,component_spec_g=1000,material_name='现有大包装商品',source_warehouse='finished_goods',reserved_g=22700 WHERE work_order_id=%[2]d AND material_id=10;
 DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[2]d AND material_id=10;
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_spec_g,stock_batch_id,batch_code,warehouse,reserved_g) SELECT id,work_order_id,0,'product',90001,1000,90001,'PRODUCT-COMPONENT','finished_goods',22700 FROM %[1]s.work_order_material_reservations WHERE work_order_id=%[2]d AND component_id=90001;
 `, schema, woID))
	var card int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card)
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var p productionapp.OperationCompletionPreview
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "product-component", "materials": p.Materials})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=90001 AND warehouse='finished_goods' AND onhand_units=7 AND onhand_loose_g=300", 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", "id=90001 AND remaining_g=7300 AND remaining_units=7", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND component_id=90001 AND consumed_g=22700", woID), 1)
}

func TestOperationCompletionAPIDirectCompleteWithoutStarting(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, woID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 72911)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=22700 WHERE id=10;UPDATE %s.materials SET onhand_units=100 WHERE id=20`, schema, schema))
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	var card int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card)
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var p productionapp.OperationCompletionPreview
	json.Unmarshal(res.Body.Bytes(), &p)
	res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "direct-complete", "materials": p.Materials})
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed' AND running_item_id>0 AND batch_id<>''", woID), 1)
	assertProductionFlowCount(t, pool, schema, "production_logs", "batch_id<>'' AND finished_units=100", 1)
}

func TestOperationCompletionAPIIsolationAndFailureRetry(t *testing.T) {
	for _, scenario := range []string{"quality", "owner", "other-reservation", "no-cross-warehouse", "stale", "late-failure"} {
		t.Run(scenario, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72931)
			snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',actual_output_qty=0,actual_input_qty=0,completed_at=NULL WHERE work_order_id=%d`, schema, woID))
			switch scenario {
			case "quality":
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE material_id=10`, schema))
			case "owner":
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET owner_customer_id=42 WHERE material_id=10`, schema))
			case "other-reservation":
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservations(id,work_order_id,material_id,material_name,unit,required_g,reserved_g,status) VALUES(90003,90003,10,'其他单熟豆','kg',22700,22700,'reserved');INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_id,material_batch_id,batch_code,warehouse,reserved_g) SELECT 90003,90003,10,10,id,batch_code,'wip',22700 FROM %[1]s.material_batches WHERE material_id=10`, schema))
			case "no-cross-warehouse":
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='elsewhere' WHERE material_id=10`, schema))
			case "late-failure":
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_completion_log() RETURNS trigger LANGUAGE plpgsql AS $body$ BEGIN RAISE EXCEPTION 'forced late failure'; END $body$;CREATE TRIGGER fail_completion_log BEFORE INSERT ON %[1]s.production_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_completion_log()`, schema))
			}
			var card int64
			pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card)
			res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
			if res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			var p productionapp.OperationCompletionPreview
			json.Unmarshal(res.Body.Bytes(), &p)
			if scenario == "other-reservation" && p.MaxOutputQty != 0 {
				t.Fatalf("preview must not claim another work order's reserved inventory: max output=%g materials=%+v", p.MaxOutputQty, p.Materials)
			}
			if scenario == "stale" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=unit_cost+1 WHERE material_id=10`, schema))
			}
			payload := map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "isolation", "materials": p.Materials}
			res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
			if res.Code != 400 {
				t.Fatalf("must reject %s: %d %s", scenario, res.Code, res.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "production_operation_events", "true", 0)
			assertProductionFlowCount(t, pool, schema, "stock_entries", "entry_type='finished_receipt'", 0)
			assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND (consumed_g>0 OR consumed_units>0)", woID), 0)
			if scenario == "late-failure" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DROP TRIGGER fail_completion_log ON %s.production_logs`, schema))
				res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), payload)
				if res.Code != 200 {
					t.Fatalf("same request retry %s", res.Body.String())
				}
				state := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
				if state.FinishedReceiptCount != 1 || state.ReservationConsumedG != 22700 {
					t.Fatalf("retry %+v", state)
				}
			}
		})
	}
}

func TestOperationCompletionAPIWeightSlicesUseLedgerGrams(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72981)
	snapshot := `{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET process_snapshot_json=$2::jsonb,operation_material_bindings='{"material:10:0:0":{"node_key":"pack"},"material:20:0:0":{"node_key":"pack"}}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), woID, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,planned_input_qty=3,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'包装','pending',1);
 UPDATE %[1]s.work_order_material_reservations SET unit='kg',required_g=3385,reserved_g=3385 WHERE work_order_id=%[2]d AND material_id=10;
 UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=3385 WHERE work_order_id=%[2]d AND material_id=10;
 UPDATE %[1]s.material_batches SET remaining_g=3385 WHERE material_id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=3385 WHERE material_id=10 AND warehouse='wip';`, schema, woID))
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id`, schema), woID)
	if err != nil {
		t.Fatal(err)
	}
	var cards []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		cards = append(cards, id)
	}
	rows.Close()
	for i, id := range cards {
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", id), nil)
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var p productionapp.OperationCompletionPreview
		if err = json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		expected := []float64{2.539, .846}[i]
		for _, m := range p.Materials {
			if m.MaterialID == 10 && (m.Qty != expected || m.ShortageQty != 0) {
				t.Fatalf("batch %d must use ledger grams without phantom shortage: %+v expected %g kg", i+1, m, expected)
			}
		}
		if p.MaxOutputQty != p.PlannedQty {
			t.Fatalf("batch %d incorrectly reduced output: %+v", i+1, p)
		}
		res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", id), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": fmt.Sprintf("gram-slice-%d", i), "materials": p.Materials, "finish_operation": true})
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d AND material_id=10 AND consumed_g=3385", woID), 1)
}
