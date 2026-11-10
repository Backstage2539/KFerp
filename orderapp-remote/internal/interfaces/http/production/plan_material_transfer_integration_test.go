package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	pgprod "orderapp/internal/infrastructure/postgres/production"
	pgstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestPlanTransferDraftAtomicReplayAndPosting(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "AUTO-GREEN", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',15000);`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	if create.Code != 200 {
		t.Fatal(create.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(create.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.ComponentSources) != 1 {
		t.Fatalf("source %+v", plan.ComponentSources)
	}
	preview := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers/preview", plan.ID), map[string]any{"draft_token": plan.DraftToken, "items": []any{}})
	if preview.Code != 200 {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	var result app.PlanMaterialTransferPreview
	if err := json.Unmarshal(preview.Body.Bytes(), &result); err != nil || len(result.Groups) == 0 {
		t.Fatalf("preview result %+v %v", result, err)
	}
	command := app.PlanMaterialTransferCommand{ProductionPlanID: plan.ID, DraftToken: plan.DraftToken, IdempotencyKey: "atomic-transfer-test", Operator: "tester", Items: []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "AUTO-GREEN", QtyG: 4000}}}}}
	repo := pgprod.NewRepository(pool, schema)
	endpoint := fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID)
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, command)
	if rec.Code != 200 {
		t.Fatalf("create transfer HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var one app.PlanMaterialTransferResult
	if err := json.Unmarshal(rec.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	var err error
	if len(one.Documents) != 1 {
		t.Fatalf("result %+v", one)
	}
	// Simulate another task in the same plan already holding a draft against this batch.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_material_transfer_links SET component_source_id=999999 WHERE stock_entry_id=%d`, schema, one.Documents[0].ID))
	cross := command
	cross.IdempotencyKey = "other-task-cross-batch"
	cross.Items = []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "AUTO-GREEN", QtyG: 12000}}}}
	if _, err := repo.CreatePlanMaterialTransfers(ctx, cross); err == nil {
		t.Fatal("separate task reused stock already promised in this plan")
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plan_material_transfer_links SET component_source_id=%d WHERE stock_entry_id=%d`, schema, plan.ComponentSources[0].ID, one.Documents[0].ID))
	again, err := repo.CreatePlanMaterialTransfers(ctx, command)
	if err != nil || len(again.Documents) != 1 || again.Documents[0].ID != one.Documents[0].ID {
		t.Fatalf("replay %+v %v", again, err)
	}
	changedToken := command
	changedToken.DraftToken = "different-draft-token"
	if _, err := repo.CreatePlanMaterialTransfers(ctx, changedToken); err == nil {
		t.Fatal("idempotency key accepted a different draft token")
	}
	detailResp := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	var detailWithTransfers struct {
		Documents []app.PlanMaterialTransferPending `json:"material_transfer_documents"`
	}
	if detailResp.Code != 200 || json.Unmarshal(detailResp.Body.Bytes(), &detailWithTransfers) != nil || len(detailWithTransfers.Documents) != 1 || detailWithTransfers.Documents[0].ID != one.Documents[0].ID || detailWithTransfers.Documents[0].Status != "draft" {
		t.Fatalf("missing plan transfer summary: %d %s", detailResp.Code, detailResp.Body.String())
	}
	secondCommand := command
	secondCommand.IdempotencyKey = "cancel-draft-test"
	secondCommand.Items = []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "AUTO-GREEN", QtyG: 1000}}}}
	second, err := repo.CreatePlanMaterialTransfers(ctx, secondCommand)
	if err != nil {
		t.Fatal(err)
	}
	cancelResp := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/cancel", second.Documents[0].ID), map[string]any{})
	var cancelled stockapp.StockDocumentDetail
	if cancelResp.Code != 200 || json.Unmarshal(cancelResp.Body.Bytes(), &cancelled) != nil || cancelled.Status != "cancelled" {
		t.Fatalf("cancel HTTP %d %s", cancelResp.Code, cancelResp.Body.String())
	}
	command.Items[0].Batches[0].QtyG = 5000
	if _, err = repo.CreatePlanMaterialTransfers(ctx, command); err == nil {
		t.Fatal("idempotency key accepted changed amount")
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=6000", 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_material_transfer_links", "production_plan_id="+fmt.Sprint(plan.ID), 2)
	// Stock entering WIP independently after draft creation must block an excess transfer.
	thirdCommand := command
	thirdCommand.IdempotencyKey = "sequential-doc-test"
	thirdCommand.Items = []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "raw_materials", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "AUTO-GREEN", QtyG: 3000}}}}
	third, err := repo.CreatePlanMaterialTransfers(ctx, thirdCommand)
	if err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=20000 WHERE material_batch_id=1030 AND warehouse='wip'`, schema))
	postPath := fmt.Sprintf("/api/stock-documents/%d/submit", one.Documents[0].ID)
	blocked := serveMultilevelProductionJSON(t, e, http.MethodPost, postPath, map[string]any{})
	if blocked.Code == 200 {
		t.Fatal("posting exceeded latest WIP shortage")
	}

	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=20000", 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030 AND warehouse='wip'`, schema))
	postedResp := serveMultilevelProductionJSON(t, e, http.MethodPost, postPath, map[string]any{})
	var posted stockapp.StockDocumentDetail
	if postedResp.Code != 200 || json.Unmarshal(postedResp.Body.Bytes(), &posted) != nil || posted.Status != "submitted" {
		t.Fatalf("post HTTP %d %s", postedResp.Code, postedResp.Body.String())
	}
	replay := serveMultilevelProductionJSON(t, e, http.MethodPost, postPath, map[string]any{})
	if replay.Code != 200 {
		t.Fatalf("post replay HTTP %d %s", replay.Code, replay.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=10000", 1)
	thirdPath := fmt.Sprintf("/api/stock-documents/%d/submit", third.Documents[0].ID)
	thirdResp := serveMultilevelProductionJSON(t, e, http.MethodPost, thirdPath, map[string]any{})
	if thirdResp.Code != 200 {
		t.Fatalf("second valid transfer HTTP %d %s", thirdResp.Code, thirdResp.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=13000", 1)
}

func TestPlanTransferConcurrentHTTPRequestsCannotOverpromise(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "CONCURRENT-GREEN", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',15000);`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	if create.Code != 200 {
		t.Fatal(create.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(create.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.ComponentSources) != 1 {
		t.Fatalf("sources %+v", plan.ComponentSources)
	}
	endpoint := fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID)
	results := make(chan int, 2)
	start := make(chan struct{})
	for _, key := range []string{"parallel-1", "parallel-2"} {
		key := key
		go func() {
			<-start
			body := map[string]any{"draft_token": plan.DraftToken, "idempotency_key": key, "items": []map[string]any{{"source_id": plan.ComponentSources[0].ID, "warehouse": "raw_materials", "batches": []map[string]any{{"batch_code": "CONCURRENT-GREEN", "qty_g": 10000}}}}}
			rec := serveMultilevelProductionJSON(t, e, http.MethodPost, endpoint, body)
			results <- rec.Code
		}()
	}
	close(start)
	first, second := <-results, <-results
	if (first == 200) == (second == 200) {
		t.Fatalf("expected one success and one blocked, got %d and %d", first, second)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_material_transfer_links", "production_plan_id="+fmt.Sprint(plan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=6000", 1)
}

func TestPlanTransferPackagedProductUsesCountsAndNeverConvertsLoose(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	e := newProductionFlowTestEcho(pool, schema)
	registerStockEntryAPI(e, app.NewService(pgprod.NewRepository(pool, schema)), stockapp.NewService(pgstock.NewRepository(pool, schema)))
	create := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	if create.Code != 200 {
		t.Fatal(create.Body.String())
	}
	var plan app.ProductionPlanDetail
	if err := json.Unmarshal(create.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.products(id,name,sku_name,sku_code,product_kind,active) VALUES(2,'包装商品','包装商品','TRANSFER-TEST','roasted_bean',true);
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units,onhand_loose_g) VALUES(2,0,0,454,'finished_goods',0,10,454);
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status) VALUES
 ('PACKED-BATCH','finished_product',2,'包装商品',0,0,454,4540,10,4540,10,'passed'),
 ('LOOSE-BATCH','finished_product',2,'包装商品',0,0,454,454,0,454,0,'passed');
 UPDATE %[1]s.production_plan_component_sources SET component_type='product',component_id=2,component_name='包装商品',component_spec_g=454,unit='件',required_g=908,required_units=2 WHERE production_plan_id=%[2]d;`, schema, plan.ID))
	get := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if get.Code != 200 {
		t.Fatal(get.Body.String())
	}
	if err := json.Unmarshal(get.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.ComponentSources) != 1 {
		t.Fatalf("sources %+v", plan.ComponentSources)
	}
	cmd := app.PlanMaterialTransferCommand{ProductionPlanID: plan.ID, DraftToken: plan.DraftToken, IdempotencyKey: "packed-test", Items: []app.PlanMaterialTransferItem{{SourceID: plan.ComponentSources[0].ID, Warehouse: "finished_goods", Batches: []app.PlanMaterialTransferBatch{{BatchCode: "LOOSE-BATCH", QtyUnits: 1}}}}}
	repo := pgprod.NewRepository(pool, schema)
	if _, err := repo.CreatePlanMaterialTransfers(ctx, cmd); err == nil || !strings.Contains(err.Error(), "只有散装") {
		t.Fatalf("loose stock incorrectly converted into package: %v", err)
	}
	cmd.Items[0].Batches[0] = app.PlanMaterialTransferBatch{BatchCode: "PACKED-BATCH", QtyG: 908, QtyUnits: 2}
	rec := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/material-transfers", plan.ID), cmd)
	if rec.Code != 200 {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var result app.PlanMaterialTransferResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 1 || len(result.Documents[0].Items) != 1 || result.Documents[0].Items[0].QtyG != 0 || result.Documents[0].Items[0].QtyUnits != 2 {
		t.Fatalf("packed quantities were not normalized: %+v", result)
	}
	get = serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", plan.ID), nil)
	if err := json.Unmarshal(get.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.MaterialTransferDocuments) != 1 || plan.MaterialTransferDocuments[0].QtyG != 908 || plan.MaterialTransferDocuments[0].QtyUnits != 2 {
		t.Fatalf("equivalent pending summary %+v", plan.MaterialTransferDocuments)
	}
	posted := serveMultilevelProductionJSON(t, e, http.MethodPost, fmt.Sprintf("/api/stock-documents/%d/submit", result.Documents[0].ID), map[string]any{})
	if posted.Code != 200 {
		t.Fatalf("post packaged transfer %d %s", posted.Code, posted.Body.String())
	}
	var wipUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=2 AND spec_g=454 AND warehouse='wip'`, schema)).Scan(&wipUnits); err != nil || wipUnits != 2 {
		t.Fatalf("WIP units=%d err=%v", wipUnits, err)
	}
}
