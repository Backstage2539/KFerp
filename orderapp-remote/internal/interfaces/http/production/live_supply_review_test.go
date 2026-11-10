package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	"strings"
	"sync"
	"testing"
	"time"
)

type reviewSyncResult struct {
	Changed bool                     `json:"changed"`
	Plan    app.ProductionPlanDetail `json:"plan"`
}

func TestLiveSupplyChangePreservesUpstreamIdentityAndRequiresSplitReview(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "REVIEW-ROAST-OLD", "在制熟豆", 10000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var child int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material'`, schema), before.ID).Scan(&child); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,workstation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙','原烘焙机',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE id=%[2]d`, schema, child))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1011, 10, "REVIEW-ROAST-NEW", "在制熟豆", 5000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{"check_only": false})
	var after reviewSyncResult
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &after) != nil {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	if !after.Changed {
		t.Fatal("expected changed supply")
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("id=%d AND planned_output_g=7700 AND planned_g=9625", child), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_component_sources", fmt.Sprintf("production_plan_item_id=%d AND component_id=30 AND required_g=9625", child), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf("depends_on_plan_item_id=%d AND material_id=10 AND required_g=7700", child), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_operation_splits", fmt.Sprintf("production_plan_item_id=%d AND workstation='原烘焙机'", child), 1)
	found := false
	for _, issue := range after.Plan.Readiness.Issues {
		if issue.ProductionPlanItemID == child && issue.Code == "operation_split_invalid" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changed split must require review: %+v", after.Plan.Readiness)
	}
}

func TestLiveSupplyReusesFrozenUpstreamBOMAfterDefaultChanges(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2010, 10, "REVIEW-BOM-OLD", "在制熟豆", 10000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var child int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material'`, schema), before.ID).Scan(&child); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,output_material_id,status) VALUES (201,'PBOM-ROAST-NEW','新熟豆 BOM','material',0,10,'active'); INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at) VALUES (201,201,'V002','published',1,0,1,'kg',32,now()); INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct) VALUES(201,30,'material','g',1500,0); UPDATE %[1]s.production_bom_output_bindings SET bom_id=201,bom_version_id=201 WHERE output_type='material' AND output_id=10;`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 2011, 10, "REVIEW-BOM-NEW", "在制熟豆", 5000)
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{})
	if rec.Code != 200 {
		t.Fatalf("should retain original frozen BOM: %d %s", rec.Code, rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("id=%d AND bom_version_id=200 AND planned_output_g=7700 AND planned_g=9625", child), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_component_sources", fmt.Sprintf("production_plan_item_id=%d AND component_id=30 AND required_g=9625", child), 1)
}

func TestLiveSupplyConcurrentNoChangeDoesNotRepeatOrModifyVersion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan string, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{"draft_token": before.DraftToken})
			if r.Code != 200 {
				results <- fmt.Sprintf("%d %s", r.Code, r.Body.String())
				return
			}
			var out reviewSyncResult
			if json.Unmarshal(r.Body.Bytes(), &out) != nil || out.Changed || out.Plan.DraftToken != before.DraftToken {
				results <- r.Body.String()
			}
		}()
	}
	wg.Wait()
	close(results)
	for s := range results {
		t.Error(s)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", before.ID), 1)
}

func TestSubmitRechecksGraphAfterWaitingForBatchLock(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3010, 10, "RACE-ROAST-OLD", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3030, 30, "RACE-GREEN", "生产生豆", 20000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 3020, 20, "RACE-BAG", "227g包装袋", 100)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	preparePlanningTest(t, ctx, pool, schema, e, before.ID)
	locker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer locker.Rollback(ctx)
	if _, err = locker.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.material_batches WHERE id=3010 FOR UPDATE`, schema)); err != nil {
		t.Fatal(err)
	}
	type response struct {
		code int
		body string
	}
	done := make(chan response, 1)
	go func() {
		r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", before.ID), nil)
		done <- response{r.Code, r.Body.String()}
	}()
	waiting := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE $1`, "%"+schema+".material_batches%").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			waiting = true
			break
		}
	}
	if !waiting {
		locker.Rollback(ctx)
		t.Fatalf("submit did not reach batch lock: %+v", <-done)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3011, 10, "RACE-ROAST-NEW", "在制熟豆", 5000)
	if err := locker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.code != http.StatusConflict || !strings.Contains(result.body, "供应已变化") {
		t.Fatalf("expected supply conflict after stock changes while waiting; got %d %s", result.code, result.body)
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", before.ID), 0)
}

func TestLiveSupplyConcurrentChangedApplyDoesNotDuplicateUpstream(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4010, 10, "CONCURRENT-BEFORE", "在制熟豆", 10000)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	var child int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plan_items WHERE production_plan_id=$1 AND output_type='material'`, schema), before.ID).Scan(&child); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4011, 10, "CONCURRENT-NEW", "在制熟豆", 5000)
	var wg sync.WaitGroup
	results := make(chan int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{"draft_token": before.DraftToken})
			results <- r.Code
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for code := range results {
		if code == 200 {
			success++
		} else if code == 409 {
			conflict++
		} else {
			t.Errorf("unexpected status %d", code)
		}
	}
	if success != 1 || conflict != 3 {
		t.Errorf("success=%d conflict=%d, want 1 and 3", success, conflict)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("id=%d AND production_plan_id=%d AND output_type='material' AND planned_output_g=7700", child, before.ID), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", before.ID), 1)
}

func TestLiveSupplyLegacyGraphlessMissingBOMShowsGap(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %[1]s.production_bom_output_bindings WHERE output_type='material' AND output_id=10; DELETE FROM %[1]s.production_bom_version_items WHERE version_id=200; DELETE FROM %[1]s.production_bom_versions WHERE id=200; DELETE FROM %[1]s.production_boms WHERE id=200;`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plans SET supply_graph_json=NULL WHERE id=%[2]d; DELETE FROM %[1]s.production_plan_supply_gaps WHERE production_plan_id=%[2]d;`, schema, before.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_supply_gaps", fmt.Sprintf("production_plan_id=%d AND item_id=10 AND reason='no_default_material_bom' AND status='unresolved'", before.ID), 1)
}

func TestLiveSupplyLegacyGraphlessFullStockNoSpuriousUpstream(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 5010, 10, "LEGACY-FULL", "在制熟豆", 22700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 5020, 20, "LEGACY-BAG", "227g包装袋", 100)
	e := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var before app.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &before); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET supply_graph_json=NULL WHERE id=%d`, schema, before.ID))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", before.ID), 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=5010 AND warehouse='wip'`, schema))
	rec = serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", before.ID), map[string]any{})
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND planned_output_g=12700", before.ID), 1)
}
