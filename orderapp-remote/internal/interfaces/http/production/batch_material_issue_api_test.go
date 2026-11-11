package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"sync"
	"testing"
)

func batchIssueFixture(t *testing.T, two bool) (bulkPickingFixture, []int64) {
	t.Helper()
	f := newBulkPickingFixture(t)
	ids := []int64{f.workOrderID}
	if two {
		ids = append(ids, f.createWorkOrder(t, 8))
	}
	registerProductionWorkstationAPI(f.app, productionapp.NewService(postgresproduction.NewRepository(f.pool, f.schema)))
	cards := []int64{}
	for _, id := range ids {
		var card int64
		if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`UPDATE %s.job_cards SET workstation='包装台' WHERE work_order_id=$1 RETURNING id`, f.schema), id).Scan(&card); err != nil {
			t.Fatal(err)
		}
		cards = append(cards, card)
	}
	return f, cards
}

func batchPreview(t *testing.T, f bulkPickingFixture, ids []int64) productionapp.BatchMaterialIssuePreview {
	t.Helper()
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/preview", map[string]any{"job_card_ids": ids})
	if r.Code != 200 {
		t.Fatalf("preview: %d %s", r.Code, r.Body.String())
	}
	var p productionapp.BatchMaterialIssuePreview
	if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func batchCommand(p productionapp.BatchMaterialIssuePreview, id string, qty float64) productionapp.BatchMaterialIssueCommand {
	return productionapp.BatchMaterialIssueCommand{JobCardIDs: p.JobCardIDs, PreviewVersion: p.Version, RequestID: id, Groups: []productionapp.BatchMaterialIssueSelection{{Key: p.Groups[0].Key, Version: p.Groups[0].Version, Quantity: qty}}}
}

func TestBatchMaterialIssueMergedPostingAndRetry(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 1 || p.Groups[0].Remaining != 25570 {
		t.Fatalf("merged remaining: %+v", p)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
	cmd := batchCommand(p, "batch-retry", 60)
	results := make(chan string, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", cmd)
			if r.Code != 200 {
				results <- r.Body.String()
				return
			}
			var got productionapp.BatchMaterialIssueResult
			_ = json.Unmarshal(r.Body.Bytes(), &got)
			if got.SuccessCount != 1 {
				results <- r.Body.String()
			}
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
	f.quantities(t, 60000, 60000, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted'", 2)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservations", "consumed_g>0", 0)
	assertProductionFlowCount(t, f.pool, f.schema, "audit_logs", "entity_type='workstation_material_issue' AND action='submit'", 1)
	cmd.Groups[0].Quantity = 61
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", cmd)
	if r.Code != 400 || !strings.Contains(r.Body.String(), "请求标识") {
		t.Fatalf("changed retry: %d %s", r.Code, r.Body.String())
	}
	fresh := batchPreview(t, f, ids)
	for _, g := range fresh.Groups {
		if g.Remaining > 0 {
			t.Fatalf("already issued: %+v", g)
		}
	}
}

func TestBatchMaterialIssueDraftAndStaleStock(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	p := batchPreview(t, f, ids)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=119000 WHERE material_batch_id=72230`, f.schema))
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "stale", 15.57))
	if r.Code != 200 || !strings.Contains(r.Body.String(), "刷新") {
		t.Fatalf("stale: %d %s", r.Code, r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
	f.draft(t, 60000)
	p = batchPreview(t, f, ids)
	if len(p.Groups) != 0 || len(p.Blocked) == 0 || p.Blocked[0].DraftID == 0 {
		t.Fatalf("draft should be explicit: %+v", p)
	}
}

func TestBatchMaterialIssueSameWorkOrderTasksDoNotRepeat(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	var second int64
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.job_cards SET planned_input_qty=1 WHERE id=%d`, f.schema, ids[0]))
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty) VALUES($1,1,'烘焙','包装台',1) RETURNING id`, f.schema), f.workOrderID).Scan(&second); err != nil {
		t.Fatal(err)
	}
	p := batchPreview(t, f, []int64{second})
	if p.Groups[0].Remaining != 7785 {
		t.Fatalf("task slice: %+v", p)
	}
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "second-first", 7.785))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("post: %d %s", r.Code, r.Body.String())
	}
	p = batchPreview(t, f, []int64{ids[0], second})
	if len(p.Groups) != 1 || p.Groups[0].Remaining != 7785 {
		t.Fatalf("remaining after later task: %+v", p)
	}
}

func TestBatchMaterialIssueSameWorkOrderExtraAfterAllTaskShares(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	var second int64
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.job_cards SET planned_input_qty=1 WHERE id=%d`, f.schema, ids[0]))
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty) VALUES($1,1,'烘焙','包装台',1) RETURNING id`, f.schema), f.workOrderID).Scan(&second); err != nil {
		t.Fatal(err)
	}
	p := batchPreview(t, f, []int64{ids[0], second})
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "split-extra", 60))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("all task shares then extra: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 60000, 60000, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted'", 2)
}

func TestBatchMaterialIssueScarceStockAndReadOnlyPreview(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=72230`, f.schema))
	p := batchPreview(t, f, ids)
	if p.Groups[0].Remaining != 15570 || p.Groups[0].Available != 10000 || p.Groups[0].MaxQuantity != 10000 {
		t.Fatalf("scarce preview: %+v", p.Groups)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "scarce", 10))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("scarce post: %s", r.Body.String())
	}
	f.quantities(t, 0, 10000, 5570, 10000)
	p = batchPreview(t, f, ids)
	if p.Groups[0].Remaining != 5570 || p.Groups[0].Available != 0 {
		t.Fatalf("remaining scarce: %+v", p.Groups)
	}
}

func TestBatchMaterialIssueGroupFailureDoesNotUndoOtherMaterial(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	ctx := context.Background()
	seedProductionFlowWIPBatch(t, ctx, f.pool, f.schema, 72430, 30, "PR724-SECOND", "生产生豆", 20000)
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`
 UPDATE %[1]s.material_batch_locations SET warehouse='finished_goods' WHERE material_batch_id=72430;
 UPDATE %[1]s.work_order_material_reservation_batches SET material_batch_id=72430,batch_code='PR724-SECOND',warehouse='finished_goods' WHERE work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d);
 `, f.schema, ids[1]))
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 2 {
		t.Fatalf("different sources must retain separate posting groups: %+v", p)
	}
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE id=72430`, f.schema))
	cmd := productionapp.BatchMaterialIssueCommand{JobCardIDs: ids, RequestID: "partial-groups", PreviewVersion: p.Version}
	for _, g := range p.Groups {
		cmd.Groups = append(cmd.Groups, productionapp.BatchMaterialIssueSelection{Key: g.Key, Version: g.Version, Quantity: float64(g.Available) / 1000})
	}
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", cmd)
	var got productionapp.BatchMaterialIssueResult
	_ = json.Unmarshal(r.Body.Bytes(), &got)
	if r.Code != 200 || got.SuccessCount != 1 || got.FailedCount != 1 {
		t.Fatalf("partial groups: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 104430, 15570, 0, 15570)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted'", 1)
}

func TestBatchMaterialIssueGroupRollsBackEveryTaskOnPostingError(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	p := batchPreview(t, f, ids)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`
 CREATE FUNCTION %[1]s.reject_second_batch_doc() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d) THEN RAISE EXCEPTION 'isolated posting failure'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_second_batch_doc BEFORE INSERT ON %[1]s.stock_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_second_batch_doc();`, f.schema, ids[1]))
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "rollback-group", 25.57))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"failed_count":1`) {
		t.Fatalf("rollback: %d %s", r.Code, r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
	f.quantities(t, 120000, 0, 15570, 0)
}

func TestBatchMaterialIssueMultipleBatchesAndPriority(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	ctx := context.Background()
	seedProductionFlowWIPBatch(t, ctx, f.pool, f.schema, 72431, 30, "PR724-THIRD", "生产生豆", 20000)
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`
 UPDATE %[1]s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=72431;
 UPDATE %[1]s.job_cards SET priority=10 WHERE id=%[2]d;
 UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=10000 WHERE work_order_id=%[3]d AND material_batch_id=72230;
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,material_batch_id,owner_customer_id,warehouse,batch_code,reserved_g,status)
 SELECT reservation_id,work_order_id,material_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,72431,owner_customer_id,warehouse,'PR724-THIRD',5570,status FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[3]d AND material_batch_id=72230;
 `, f.schema, ids[1], f.workOrderID))
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 1 || len(p.Groups[0].Allocations) != 3 || p.Groups[0].Remaining != 25570 {
		t.Fatalf("multiple batches: %+v", p)
	}
	if p.Groups[0].Allocations[0].JobCardID != ids[1] || p.Groups[0].Allocations[0].ExtraAvailable == 0 {
		t.Fatalf("priority extra: %+v", p.Groups[0].Allocations)
	}
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "multi-batch", 25.57))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("multi-batch post: %s", r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entry_items", "batch_code='PR724-THIRD' AND qty_g=5570", 1)
}

func TestBatchMaterialIssueRejectsUnboundEmployee(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	p := batchPreview(t, f, ids)
	app := echo.New()
	registerProductionWorkstationAPI(app, productionapp.NewService(postgresproduction.NewRepository(f.pool, f.schema)))
	r := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "unbound", 15.57))
	if r.Code != 400 {
		t.Fatalf("unbound: %d %s", r.Code, r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "true", 0)
}

func TestBatchMaterialIssuePackagingCountAndCustomerOwner(t *testing.T) {
	f := newBulkPickingFixtureOwner(t, "个", 77)
	registerProductionWorkstationAPI(f.app, productionapp.NewService(postgresproduction.NewRepository(f.pool, f.schema)))
	var card int64
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, f.schema), f.workOrderID).Scan(&card); err != nil {
		t.Fatal(err)
	}
	p := batchPreview(t, f, []int64{card})
	if len(p.Groups) != 1 || p.Groups[0].Unit != "个" || p.Groups[0].OwnerCustomerID != 77 || p.Groups[0].Available != 12 || p.Groups[0].QuantityBasis != "count" {
		t.Fatalf("packaging: %+v", p)
	}
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "count-owner", 30))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("count submit: %s", r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entry_items", "qty_units=30 AND qty_g=0 AND owner_customer_id=77", 1)
	assertProductionFlowCount(t, f.pool, f.schema, "work_order_material_reservations", "consumed_units>0 OR consumed_g>0", 0)
}

func TestBOMStartAfterIssuingMoreThanFrozenReservation(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	ctx := context.Background()
	snapshot := `{"operations":[{"seq":1,"operation":"烘焙","execution_json":"{\"version\":1,\"node_key\":\"roast\",\"receipt_enabled\":true,\"output_unit\":\"kg\",\"receipt_warehouse\":\"wip\",\"source_warehouse\":\"raw_materials\"}"}]}`
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1', process_snapshot_json='%s'::jsonb, material_snapshot='[{"material_id":30,"material_name":"生产生豆","unit":"kg","ratio_pct":100,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]'::jsonb WHERE id=%d`, f.schema, snapshot, f.workOrderID))
	first := batchPreview(t, f, ids)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(first, "partial-before-start", 10))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("partial issue: %d %s", r.Code, r.Body.String())
	}
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 1 || p.Groups[0].Remaining != 5570 || p.Groups[0].MaxQuantity < 6043 {
		t.Fatalf("expected extra stock: %+v", p)
	}
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/submit", batchCommand(p, "issue-extra-start", 6.043))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"success_count":1`) {
		t.Fatalf("issue extra: %d %s", r.Code, r.Body.String())
	}
	f.quantities(t, 103957, 16043, 0, 15570)
	res := serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/start-preview", ids[0]), nil)
	var preview productionapp.OperationCompletionPreview
	if res.Code != 200 {
		t.Fatalf("start preview: %d %s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.BlockedReason != "" || len(preview.Materials) != 1 || preview.Materials[0].Warehouse != "wip" || preview.Materials[0].AvailableQty != 16.043 {
		t.Fatalf("issued extra must be available to start in WIP: %+v", preview)
	}
	preview.Materials[0].Qty = 16.043
	payload := map[string]any{"actual_input_qty": 16.043, "input_unit": preview.InputUnit, "materials": preview.Materials, "expected_version": preview.Version, "request_id": "issue-extra-start-once", "difference_reason": "按实际投入补充"}
	for i := 0; i < 2; i++ {
		r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/start", ids[0]), payload)
		if r.Code != 200 {
			t.Fatalf("start attempt %d: %d %s", i, r.Code, r.Body.String())
		}
	}
	assertProductionFlowCount(t, f.pool, f.schema, "job_cards", fmt.Sprintf("id=%d AND status='running'", ids[0]), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "production_started_inputs", fmt.Sprintf("job_card_id=%d AND status='posted'", ids[0]), 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", fmt.Sprintf("job_card_id=%d AND source_type='operation_start' AND status='submitted'", ids[0]), 1)
}

func TestBatchReplenishmentDraftAndConfirmAreSeparateAndIdempotent(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	p := batchPreview(t, f, ids)
	cmd := batchCommand(p, "replenishment-test", 15.57)
	draft := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if draft.Code != http.StatusOK {
		t.Fatalf("draft: %d %s", draft.Code, draft.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 0)
	recovered := serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/production/workstation/material-issue/draft?job_card_id=%d", ids[0]), nil)
	var recovery productionapp.BatchReplenishmentRecovery
	if err := json.Unmarshal(recovered.Body.Bytes(), &recovery); err != nil || recovered.Code != 200 || !recovery.Found || recovery.Command == nil || recovery.Command.RequestID != "replenishment-test" {
		t.Fatalf("recover: %d %s %v", recovered.Code, recovered.Body.String(), err)
	}
	retry := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if retry.Code != http.StatusOK || retry.Body.String() != draft.Body.String() {
		t.Fatalf("draft retry: %d %s", retry.Code, retry.Body.String())
	}
	confirm := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "replenishment-test"})
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", confirm.Code, confirm.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 1)
	recovered = serveMultilevelProductionJSON(t, f.app, http.MethodGet, fmt.Sprintf("/api/production/workstation/material-issue/draft?job_card_id=%d", ids[0]), nil)
	if err := json.Unmarshal(recovered.Body.Bytes(), &recovery); err != nil || recovered.Code != 200 || recovery.Found {
		t.Fatalf("confirmed recovery: %d %s %v", recovered.Code, recovered.Body.String(), err)
	}
	var confirmedID int64
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT id FROM %s.stock_entries WHERE status='submitted' AND return_source='workstation_replenishment'`, f.schema)).Scan(&confirmedID); err != nil {
		t.Fatal(err)
	}
	if _, err := postgresstock.NewRepository(f.pool, f.schema).CancelStockDocument(context.Background(), confirmedID, "operator"); err == nil {
		t.Fatal("individual cancellation bypassed group integrity")
	}
	again := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "replenishment-test"})
	if again.Code != http.StatusOK {
		t.Fatalf("confirm retry: %d %s", again.Code, again.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 1)
}

func TestBatchReplenishmentRejectsBypassAndChangedStock(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	p := batchPreview(t, f, ids)
	cmd := batchCommand(p, "replenish-stale", 15.57)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("draft: %d %s", r.Code, r.Body.String())
	}
	var id int64
	if err := f.pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT id FROM %s.stock_entries WHERE return_source='workstation_replenishment'`, f.schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	stock := postgresstock.NewRepository(f.pool, f.schema)
	if _, err := stock.SubmitStockDocument(context.Background(), id, "operator"); err == nil {
		t.Fatal("generic submission bypassed replenishment validation")
	}
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=119000 WHERE material_batch_id=72230`, f.schema))
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "replenish-stale"})
	if r.Code == 200 {
		t.Fatalf("changed stock accepted: %s", r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 1)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 0)
}

func TestBatchReplenishmentConfirmRollsBackAllDocumentsOnFailure(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	p := batchPreview(t, f, ids)
	cmd := batchCommand(p, "replenish-rollback", 25.57)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("draft: %d %s", r.Code, r.Body.String())
	}
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_replenishment_post() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='submitted' AND NEW.work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d) THEN RAISE EXCEPTION 'posting failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_replenishment_post BEFORE UPDATE ON %[1]s.stock_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_replenishment_post();`, f.schema, ids[1]))
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "replenish-rollback"})
	if r.Code == 200 {
		t.Fatalf("failed post accepted: %s", r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 2)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 0)
}

func TestBatchReplenishmentConcurrentDraftsDoNotDuplicate(t *testing.T) {
	f, ids := batchIssueFixture(t, false)
	p := batchPreview(t, f, ids)
	cmd := batchCommand(p, "replenish-concurrent", 15.57)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
			codes <- r.Code
		}()
	}
	wg.Wait()
	close(codes)
	successes := 0
	for code := range codes {
		if code == 200 {
			successes++
		}
	}
	if successes != 2 {
		t.Fatalf("both retries should succeed, got %d", successes)
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 1)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("retry: %d %s", r.Code, r.Body.String())
	}
}

func TestBatchReplenishmentCrossWarehouseFailureIsAtomic(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	ctx := context.Background()
	seedProductionFlowWIPBatch(t, ctx, f.pool, f.schema, 72431, 30, "REPL-SECOND", "生产生豆", 20000)
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET warehouse='finished_goods' WHERE material_batch_id=72431; UPDATE %[1]s.work_order_material_reservation_batches SET material_batch_id=72431,batch_code='REPL-SECOND',warehouse='finished_goods' WHERE work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d)`, f.schema, ids[1]))
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 2 {
		t.Fatalf("expected two sources: %+v", p.Groups)
	}
	cmd := productionapp.BatchMaterialIssueCommand{JobCardIDs: ids, RequestID: "replenish-cross-warehouse", PreviewVersion: p.Version}
	for _, g := range p.Groups {
		cmd.Groups = append(cmd.Groups, productionapp.BatchMaterialIssueSelection{Key: g.Key, Version: g.Version, Quantity: float64(g.Available) / 1000})
	}
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("draft: %d %s", r.Code, r.Body.String())
	}
	var draft productionapp.BatchMaterialIssueResult
	if err := json.Unmarshal(r.Body.Bytes(), &draft); err != nil {
		t.Fatal(err)
	}
	if len(draft.Groups) != 2 || len(draft.Groups[0].Documents) != 1 || len(draft.Groups[1].Documents) != 1 || draft.Groups[0].Documents[0].ID == draft.Groups[1].Documents[0].ID {
		t.Fatalf("documents not mapped by source: %+v", draft)
	}
	mustExecProductionFlowTestSQL(t, ctx, f.pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_cross_replenishment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='submitted' AND NEW.work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d) THEN RAISE EXCEPTION 'failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_cross_replenishment BEFORE UPDATE ON %[1]s.stock_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_cross_replenishment();`, f.schema, ids[1]))
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/confirm", map[string]any{"request_id": "replenish-cross-warehouse"})
	if r.Code == 200 {
		t.Fatalf("failure accepted: %s", r.Body.String())
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 2)
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='submitted' AND return_source='workstation_replenishment'", 0)
}

func TestBatchReplenishmentCancelGroupAtomicAndRetry(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	p := batchPreview(t, f, ids)
	cmd := batchCommand(p, "replenish-cancel", 25.57)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("draft: %d %s", r.Code, r.Body.String())
	}
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_cancel_replenishment() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='cancelled' AND NEW.work_order_id=(SELECT work_order_id FROM %[1]s.job_cards WHERE id=%[2]d) THEN RAISE EXCEPTION 'cancel failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_cancel_replenishment BEFORE UPDATE ON %[1]s.stock_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_cancel_replenishment();`, f.schema, ids[1]))
	body := map[string]any{"request_id": "replenish-cancel"}
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/cancel", body)
	if r.Code == 200 {
		t.Fatal("cancel should fail")
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='draft' AND return_source='workstation_replenishment'", 2)
	mustExecProductionFlowTestSQL(t, context.Background(), f.pool, fmt.Sprintf(`DROP TRIGGER reject_cancel_replenishment ON %s.stock_entries`, f.schema))
	for i := 0; i < 2; i++ {
		r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/cancel", body)
		if r.Code != 200 || !strings.Contains(r.Body.String(), `"cancelled":true`) {
			t.Fatalf("cancel: %d %s", r.Code, r.Body.String())
		}
	}
	assertProductionFlowCount(t, f.pool, f.schema, "stock_entries", "status='cancelled' AND return_source='workstation_replenishment'", 2)
	p = batchPreview(t, f, ids)
	cmd = batchCommand(p, "replenish-new-after-cancel", 25.57)
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("new draft: %d %s", r.Code, r.Body.String())
	}
}

func TestBatchReplenishmentSharedWIPAllocatedOnce(t *testing.T) {
	f, ids := batchIssueFixture(t, true)
	ctx := context.Background()
	seedProductionFlowWIPBatch(t, ctx, f.pool, f.schema, 72435, 30, "SHARED-WIP", "生产生豆", 10000)
	p := batchPreview(t, f, ids)
	if len(p.Groups) != 1 {
		t.Fatalf("preview: %+v", p.Groups)
	}
	cmd := batchCommand(p, "shared-wip-excess", 15.571)
	r := serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code == 200 {
		t.Fatalf("over-shared shortage accepted: %s", r.Body.String())
	}
	cmd = batchCommand(p, "shared-wip-exact", 15.57)
	r = serveMultilevelProductionJSON(t, f.app, http.MethodPost, "/api/production/workstation/material-issue/draft", cmd)
	if r.Code != 200 {
		t.Fatalf("shared shortage: %d %s", r.Code, r.Body.String())
	}
}
