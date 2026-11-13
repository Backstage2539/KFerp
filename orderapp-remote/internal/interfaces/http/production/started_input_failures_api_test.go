package production

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	app "orderapp/internal/application/production"
	"sync"
	"testing"
)

func TestBOMStartFailuresAreAtomic(t *testing.T) {
	for _, kind := range []string{"stock", "quality", "owner", "version", "audit", "rollback"} {
		t.Run(kind, func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74800)
			snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, snapshot); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1 WHERE work_order_id=%d`, schema, wo))
			if kind == "stock" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET remaining_g=0 WHERE material_id=10; UPDATE %[1]s.material_batch_locations SET qty_g=0 WHERE material_id=10;UPDATE %[1]s.materials SET onhand_g=0 WHERE id=10`, schema))
			}
			if kind == "quality" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE material_id=10`, schema))
			}
			if kind == "audit" {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_start_audit() RETURNS trigger LANGUAGE plpgsql AS $body$ BEGIN IF NEW.entity_type='job_card' AND NEW.action='start_consumption' THEN RAISE EXCEPTION 'forced start audit failure'; END IF;RETURN NEW; END $body$;CREATE TRIGGER fail_start_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_start_audit()`, schema))
			}
			if kind == "rollback" {
				t.Setenv("KFERP_BOM_START_READ_ONLY", "1")
			}
			var card int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
				t.Fatal(err)
			}
			var before int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(remaining_g+remaining_units) FROM %s.material_batches`, schema)).Scan(&before); err != nil {
				t.Fatal(err)
			}
			res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
			var p app.OperationCompletionPreview
			if res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if !p.Enabled || p.BlockedReason != "" {
				t.Fatalf("invalid failure fixture: %+v", p)
			}
			if kind == "owner" {
				p.Materials[0].OwnerCustomerID = 999
			}
			if kind == "version" {
				p.Version = "expired"
			}
			res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "fail-start"})
			if res.Code == 200 {
				t.Fatalf("%s must reject", kind)
			}
			var rejected struct {
				NotCommitted bool `json:"not_committed"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &rejected); err != nil || !rejected.NotCommitted {
				t.Fatalf("%s must identify the rolled-back start: %s", kind, res.Body.String())
			}
			var after int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(remaining_g+remaining_units) FROM %s.material_batches`, schema)).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatalf("partial inventory mutation %d -> %d", before, after)
			}
			assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("job_card_id=%d AND kind='start'", card), 0)
			assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 0)
			assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND status='pending'", card), 1)
			assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND source_type='operation_start'", wo), 0)
		})
	}
}
func TestBOMStartConcurrentRequestsConsumeOnce(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, wo, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74810)
	snapshot := `{"operations":[{"seq":1,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"first\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100 WHERE id=$1`, schema), wo, snapshot); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='pending',sequence_no=1 WHERE work_order_id=%d`, schema, wo))
	var card int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT MIN(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&card); err != nil {
		t.Fatal(err)
	}
	res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/start-preview", card), nil)
	var p app.OperationCompletionPreview
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "materials": p.Materials, "expected_version": p.Version, "request_id": "concurrent-same"}
	raw, _ := json.Marshal(body)
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/job-cards/%d/start", card), bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			r := httptest.NewRecorder()
			e.ServeHTTP(r, req)
			results <- r
		}()
	}
	wg.Wait()
	close(results)
	for r := range results {
		if r.Code != 200 {
			t.Fatalf("concurrent same-key retry: %s", r.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d", card), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND source_type='operation_start'", wo), 1)
	body["actual_input_qty"] = p.InputQty + 1
	changed := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), body)
	if changed.Code == 200 {
		t.Fatal("same key accepted changed payload")
	}
	var conflict struct {
		NotCommitted bool `json:"not_committed"`
	}
	if err := json.Unmarshal(changed.Body.Bytes(), &conflict); err != nil || conflict.NotCommitted {
		t.Fatalf("a conflicting key can refer to an earlier committed start: %s", changed.Body.String())
	}
}
