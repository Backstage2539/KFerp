package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"math"
	"net/http"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestStockProductionPreviewIsReadOnlyAndCreatesIndependentMaterialPlan(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	body := map[string]any{"source_type": "stock", "request_id": "stock-1", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 20, "target_warehouse": "wip"}}}
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", body)
	if preview.Code != 200 {
		t.Fatalf("preview %d %s", preview.Code, preview.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", "true", 0)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body)
	if create.Code != 200 {
		t.Fatalf("create %d %s", create.Code, create.Body.String())
	}
	var p struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND output_material_id=10 AND output_qty=20 AND planned_g=25000 AND order_nos=''", p.ID), 1)
	again := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body)
	if again.Code != 200 {
		t.Fatalf("retry %s", again.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 1)
	body["items"] = []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 21, "target_warehouse": "wip"}}
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body); rec.Code == 200 {
		t.Fatal("same idempotency key must not accept changed quantity")
	}
}

func selectPlanningTestSources(t *testing.T, app *echo.Echo, planID int64) {
	t.Helper()
	rec := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/production-plans/%d", planID), nil)
	var detail productionapp.ProductionPlanDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	byItem := map[int64][]productionapp.ProductionPlanComponentSource{}
	for _, s := range detail.ComponentSources {
		if len(s.Options) == 0 {
			continue
		}
		s.SourceWarehouse = s.Options[0].Warehouse
		s.SourceOwnerCustomerID = s.Options[0].OwnerCustomerID
		byItem[s.ProductionPlanItemID] = append(byItem[s.ProductionPlanItemID], s)
	}
	for id, sources := range byItem {
		rec := serveMultilevelProductionJSON(t, app, http.MethodPut, fmt.Sprintf("/api/production-plans/%d/items/%d/component-sources", planID, id), map[string]any{"sources": sources})
		if rec.Code != 200 {
			t.Fatalf("sources %d %s", rec.Code, rec.Body.String())
		}
	}
}
func preparePlanningTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string, app *echo.Echo, planID int64) {
	t.Helper()
	selectPlanningTestSources(t, app, planID)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes)
 SELECT production_plan_id,id,1,CASE WHEN output_type='material' THEN '烘焙' ELSE '包装' END,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,inventory_unit,15,1,CASE WHEN inventory_unit='kg' THEN planned_g/1000.0 ELSE planned_g END,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, planID))
}
func TestStockSupplyPartialReceiptsEnablePackagingAndKeepCostsIncremental(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=100000 WHERE id=30`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "MB-GREEN-STOCK", "生产生豆", 100000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "MB-BAG-STOCK", "227g包装袋", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=50 WHERE id=1030`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	createPlan := func(body map[string]any) int64 {
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body)
		if rec.Code != 200 {
			t.Fatalf("create %s", rec.Body.String())
		}
		var p productionapp.ProductionPlanDetail
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		preparePlanningTest(t, ctx, pool, schema, app, p.ID)
		return p.ID
	}
	submit := func(id int64) {
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", id), nil)
		if rec.Code != 200 {
			t.Fatalf("submit %s", rec.Body.String())
		}
	}
	stock := createPlan(map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 30, "target_warehouse": "wip"}}})
	submit(stock)
	assertProductionFlowCount(t, pool, schema, "orders", fmt.Sprintf("process_status_id IN (SELECT id FROM %s.order_process_statuses WHERE name='待处理')", schema), 1)
	orders := createPlan(map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}})
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", orders), 0)
	assertProductionFlowCount(t, pool, schema, "production_supply_allocations", fmt.Sprintf("production_plan_id=%d AND required_g=22700 AND status='proposed'", orders), 1)
	submit(orders)
	submit(orders)
	var supplier, consumer int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), stock).Scan(&supplier); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), orders).Scan(&consumer); err != nil {
		t.Fatal(err)
	}
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/cancel", supplier), nil); rec.Code == 200 {
		t.Fatal("allocated supplier must not cancel")
	}
	for _, id := range []int64{supplier} {
		if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", id), nil); rec.Code != 200 {
			t.Fatalf("start %s", rec.Body.String())
		}
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='running',actual_input_qty=28.375,actual_output_qty=22.7,actual_operation_cost=30 WHERE work_order_id=%d`, schema, supplier))
	receipt := map[string]any{"completion_mode": "partial", "request_id": "receipt-one", "finished_qty_g": 22700, "consumed_input_g": 28375, "warehouse": "wip"}
	for i := 0; i < 2; i++ {
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", supplier), receipt)
		if rec.Code != 200 {
			t.Fatalf("partial %s", rec.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_material_receipts", fmt.Sprintf("work_order_id=%d", supplier), 1)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='partially_completed'", supplier), 1)
	rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", consumer), nil)
	if rec.Code != 200 {
		t.Fatalf("early packaging %s", rec.Body.String())
	}
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", supplier), map[string]any{"action": "finish"})
	if preview.Code != 200 || !strings.Contains(preview.Body.String(), `"qty_g":7300`) {
		t.Fatalf("remaining receipt preview %s", preview.Body.String())
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='completed',actual_input_qty=37.5,actual_output_qty=30 WHERE work_order_id=%d`, schema, supplier))
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", supplier), map[string]any{"completion_mode": "final", "request_id": "receipt-two", "finished_qty_g": 7300, "consumed_input_g": 9125, "warehouse": "wip"})
	if rec.Code != 200 {
		var reserved string
		_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(r))::text FROM %s.work_order_material_reservations r WHERE work_order_id=$1`, schema), supplier).Scan(&reserved)
		t.Fatalf("final %s reservations=%s", rec.Body.String(), reserved)
	}
	assertProductionFlowCount(t, pool, schema, "production_material_receipts", fmt.Sprintf("work_order_id=%d", supplier), 2)
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("work_order_id=%d AND actual_input_qty=37.5 AND actual_output_qty=30", supplier), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies", fmt.Sprintf("depends_on_work_order_id=%d AND delivered_g=22700", supplier), 1)
	var input int64
	var operation float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(input_g),SUM(operation_cost)::float8 FROM %s.production_material_receipts WHERE work_order_id=$1`, schema), supplier).Scan(&input, &operation); err != nil {
		t.Fatal(err)
	}
	if input != 37500 || math.Abs(operation-30) > 0.001 {
		t.Fatalf("incremental input/cost = %d / %f", input, operation)
	}
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=10 AND unit_cost=63.5", 2)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND actual_cost=1905", supplier), 1)
	// Pre-upgrade dependencies have no delivered counters; existing full batch
	// reservations must keep them executable without rewriting historical records.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_order_dependencies SET delivered_g=0 WHERE depends_on_work_order_id=%d`, schema, supplier))
	detail := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", consumer), nil)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), `"supply_ready":true`) {
		t.Fatalf("historical fulfilled dependency %s", detail.Body.String())
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET status='completed',actual_input_qty=22.7,actual_output_qty=22.7 WHERE work_order_id=%d`, schema, consumer))
	finish := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", consumer), map[string]any{"finished_units": 100, "consumed_input_g": 22700, "warehouse": "finished_goods"})
	if finish.Code != 200 {
		t.Fatalf("package finish %s", finish.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id IN (SELECT running_item_id FROM %s.work_orders WHERE id=%d) AND material_cost=1541.45", schema, consumer), 1)
	assertProductionFlowCount(t, pool, schema, "material_batches", "material_id=10 AND remaining_g=7300", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND status='submitted' AND entry_type='finished_receipt'", consumer), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND order_nos='SO-ML-227' AND jsonb_array_length(demand_sources_json)=1", orders), 1)

}

func TestInflightSupplyConcurrentSubmissionAndCancellationRelease(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.materials SET onhand_g=100000 WHERE id=30;UPDATE %[1]s.materials SET onhand_units=1000 WHERE id=20;
 INSERT INTO %[1]s.orders(id,order_no,order_date,process_status_id) SELECT 2,'SO-ML-SECOND','2026-08-11',process_status_id FROM %[1]s.orders WHERE id=1;
 INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id) VALUES(2,1,'227g包装熟豆',100,'袋','227g',1);`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "MB-CONCURRENT-GREEN", "生产生豆", 100000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "MB-CONCURRENT-BAG", "227g包装袋", 1000)
	app := newProductionFlowTestEcho(pool, schema)
	create := func(body map[string]any) int64 {
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body)
		if rec.Code != 200 {
			t.Fatalf("create %s", rec.Body.String())
		}
		var p productionapp.ProductionPlanDetail
		_ = json.Unmarshal(rec.Body.Bytes(), &p)
		preparePlanningTest(t, ctx, pool, schema, app, p.ID)
		return p.ID
	}
	stock := create(map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 30, "target_warehouse": "wip"}}})
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", stock), nil); rec.Code != 200 {
		t.Fatalf("stock submit %s", rec.Body.String())
	}
	ids := []int64{}
	for _, day := range []string{"2026-08-10", "2026-08-11"} {
		ids = append(ids, create(map[string]any{"from": day, "to": day, "selected": []string{"1-227"}}))
	}
	type result struct {
		id   int64
		code int
		body string
	}
	results := make(chan result, 2)
	for _, id := range ids {
		go func(id int64) {
			rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", id), nil)
			results <- result{id, rec.Code, rec.Body.String()}
		}(id)
	}
	winner, loser := int64(0), int64(0)
	for i := 0; i < 2; i++ {
		r := <-results
		if r.code == 200 {
			if winner != 0 {
				t.Fatal("double allocation")
			}
			winner = r.id
		} else if strings.Contains(r.body, "在途供应余量已变化") {
			loser = r.id
		} else {
			t.Fatalf("unexpected submit %s", r.body)
		}
	}
	if winner == 0 || loser == 0 {
		t.Fatalf("winner=%d loser=%d", winner, loser)
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='draft'", loser), 1)
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", loser), nil); rec.Code != 200 {
		t.Fatalf("refresh while occupied %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material' AND planned_output_g=15400", loser), 1)
	var consumer, supplier int64
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), winner).Scan(&consumer)
	_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_orders WHERE production_plan_id=$1`, schema), stock).Scan(&supplier)
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/cancel", supplier), nil); rec.Code == 200 {
		t.Fatal("supplier cancelled while allocated")
	}
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/cancel", consumer), nil); rec.Code != 200 {
		t.Fatalf("consumer cancel %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_supply_allocations", fmt.Sprintf("work_order_id=%d AND status='released'", consumer), 1)
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", loser), nil); rec.Code != 200 {
		t.Fatalf("refresh released supply %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("production_plan_id=%d AND output_type='material'", loser), 0)
	if rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", loser), nil); rec.Code != 200 {
		t.Fatalf("released supply reusable %s", rec.Body.String())
	}
}

func TestDemandProductGroupsAvoidOrderSourceColumnCollision(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	// Live orders include a text source column; it must not shadow the JSON
	// element used to resolve a demand's exact plan/order-item association.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN source TEXT NOT NULL DEFAULT 'erp'`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("demand with order source: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var data productionapp.PlanSummaryData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.ProductGroups) != 1 || len(data.Rows) != 1 || len(data.Rows[0].OrderDetails) != 1 || !data.Rows[0].DemandSelectable {
		t.Fatalf("unexpected structured demand: %+v", data)
	}
}

func TestDemandProductGroupsBlocksIncompleteBOMBeforeSelection(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`DELETE FROM %s.production_bom_version_items WHERE version_id=100`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced", nil)
	var data productionapp.PlanSummaryData
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &data) != nil || len(data.Rows) != 1 {
		t.Fatalf("demand listing: %d %s", rec.Code, rec.Body.String())
	}
	if data.Rows[0].DemandSelectable || !strings.Contains(data.Rows[0].BlockingReason, "物料明细") {
		t.Fatalf("incomplete BOM remained selectable: %+v", data.Rows[0])
	}
	preview := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?plan=1&selected="+data.Rows[0].SelectionID, nil)
	var selected productionapp.PlanSummaryData
	if preview.Code != http.StatusOK || json.Unmarshal(preview.Body.Bytes(), &selected) != nil || selected.PlanReady || !strings.Contains(selected.Error, "物料明细") {
		t.Fatalf("selected refresh must retain a useful error: %d %s", preview.Code, preview.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 0)
}

func TestDemandProductGroupsPreserveFrozenUnitsAndExactOrderItems(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(1,'订单客户甲'),(2,'订单客户乙') ON CONFLICT(id) DO UPDATE SET name=excluded.name;
 UPDATE %[1]s.orders SET customer_id=1 WHERE id=1;
 INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id) VALUES(2,'SO-SPECS','2026-08-12',2);
 INSERT INTO %[1]s.order_items(order_id,line_no,item_name,qty,unit,spec,product_id,price_source_json) VALUES
 (2,1,'227g包装熟豆',2,'盒','10袋装',1,'{"production_quantity_snapshot":{"sku_id":1,"parent_product_id":1,"spec_label":"10袋装","sales_unit":"盒","inventory_unit":"kg","inventory_qty_per_sales_unit":2.27,"conversion_source":"published"}}'),
 (2,2,'227g包装熟豆',3,'袋','227g',1,'{"production_quantity_snapshot":{"sku_id":1,"parent_product_id":1,"spec_label":"227g","sales_unit":"袋","inventory_unit":"kg","inventory_qty_per_sales_unit":0.227,"conversion_source":"published"}}');`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if rec.Code != 200 {
		t.Fatalf("demand %s", rec.Body.String())
	}
	var data productionapp.PlanSummaryData
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.ProductGroups) != 1 || len(data.ProductGroups[0].Specs) != 2 {
		t.Fatalf("groups %+v", data.ProductGroups)
	}
	keys := map[string]bool{}
	total := map[string]float64{}
	var chosen string
	for _, row := range data.Rows {
		if row.SelectionID == "" || keys[row.SelectionID] {
			t.Fatalf("selection identity %+v", row)
		}
		keys[row.SelectionID] = true
		for _, d := range row.OrderDetails {
			total[d.SalesUnit] += d.Quantity
			if d.CustomerName == "" || d.OrderItemID == 0 {
				t.Fatalf("order detail %+v", d)
			}
		}
		if row.SalesUnit == "盒" {
			chosen = row.SelectionID
			if row.NeedG != 4540 {
				t.Fatalf("frozen weight=%d", row.NeedG)
			}
		}
	}
	if total["袋"] != 103 || total["盒"] != 2 {
		t.Fatalf("counts %+v", total)
	}
	// The read-only POST and existing order preview use the same expansion.
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{chosen}})
	if rec.Code != 200 {
		t.Fatalf("preview selected snapshot %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 0)
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{chosen}})
	if rec.Code != 200 {
		t.Fatalf("create selected order scope %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", "output_type='product' AND jsonb_array_length(demand_sources_json)=1 AND demand_sources_json->0->>'sales_unit'='盒'", 1)
	remaining := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	var after productionapp.PlanSummaryData
	_ = json.Unmarshal(remaining.Body.Bytes(), &after)
	for _, row := range after.Rows {
		if row.SalesUnit == "袋" && !row.DemandSelectable {
			t.Fatalf("unselected order item incorrectly planned %+v", row)
		}
	}

}

func TestStockPlanningRejectsCustomerMaterialForOtherCustomer(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(901,'货主甲'),(902,'订单乙');UPDATE %[1]s.materials SET owner_customer_id=901,onhand_g=0,purchase_price=0,is_semi_finished=true WHERE id=10;UPDATE %[1]s.orders SET customer_id=902 WHERE id=1`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	stock := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 10, "target_warehouse": "wip"}}})
	if stock.Code != 200 {
		t.Fatalf("own material stock preview %s", stock.Body.String())
	}
	demands := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	var data productionapp.PlanSummaryData
	_ = json.Unmarshal(demands.Body.Bytes(), &data)
	if len(data.Rows) != 1 {
		t.Fatalf("demands %s", demands.Body.String())
	}
	rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "selected": []string{data.Rows[0].SelectionID}})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "其他货主") {
		t.Fatalf("cross-owner preview %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 0)
}

func TestStockSupplyCoverageCombinesExistingStockWithEligibleInflightWarehouse(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "MIX-GREEN", "生豆", 100000)
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 30, "target_warehouse": "wip"}}})
	var stock productionapp.ProductionPlanDetail
	if rec.Code != 200 {
		t.Fatalf("stock %s", rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &stock)
	preparePlanningTest(t, ctx, pool, schema, app, stock.ID)
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", stock.ID), nil)
	if rec.Code != 200 {
		t.Fatalf("submit %s", rec.Body.String())
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "MIX-ROASTED", "熟豆", 5000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='raw_materials' WHERE material_id=10`, schema))
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", map[string]any{"selected": []string{"1-227"}, "from": "2026-08-01", "to": "2026-08-31"})
	var preview productionapp.ProductionPlanPreview
	if rec.Code != 200 {
		t.Fatalf("preview %s", rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &preview)
	found := false
	for _, node := range preview.ManufacturingPlan.Nodes {
		if node.Item.Type == "material" && node.Item.ID == 10 {
			found = true
			if node.StockCoveredQty != 5 || math.Abs(node.InflightCoveredQty-17.7) > 0.00001 || node.ShortageQty != 0 {
				t.Fatalf("mixed coverage %+v", node)
			}
		}
	}
	if !found {
		t.Fatal("roasted demand missing")
	}
}
