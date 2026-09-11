package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"net/http"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"strings"
	"testing"
)

func TestAutoPickingCombinesWarehousesWithoutMovingOrReservingDraftStock(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "AUTO-GREEN", "生产生豆", 21000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active) VALUES('raw_backup','备用原料仓','raw_materials',90,true);
 UPDATE %[1]s.material_batch_locations SET qty_g=6000 WHERE material_batch_id=1030;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(1030,30,'raw_materials',10000),(1030,30,'raw_backup',5000);
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	body := map[string]any{"source_type": "stock", "items": []map[string]any{{"output_type": "material", "output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}}
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", body)
	if preview.Code != 200 {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", body)
	if create.Code != 200 {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	var plan struct {
		ID               int64 `json:"id"`
		ComponentSources []struct {
			ComponentID int64 `json:"component_id"`
			WIPCoveredG int64 `json:"wip_covered_g"`
			TransferG   int64 `json:"transfer_g"`
			ShortageG   int64 `json:"shortage_g"`
			Allocations []struct {
				Warehouse string `json:"warehouse"`
				QtyG      int64  `json:"qty_g"`
			} `json:"allocations"`
		} `json:"component_sources"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.ComponentSources) != 1 {
		t.Fatalf("sources: %+v", plan.ComponentSources)
	}
	source := plan.ComponentSources[0]
	if source.ComponentID != 30 || source.WIPCoveredG != 6000 || source.TransferG != 15000 || source.ShortageG != 0 || len(source.Allocations) != 3 {
		t.Fatalf("expected WIP 6000 + transfer 15000 across 3 locations, got %+v", source)
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", "true", 0)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=6000", 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_operation_splits(production_plan_id,production_plan_item_id,operation_seq,operation,batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,planned_qty,planned_qty_g,planned_minutes) SELECT production_plan_id,id,1,'烘焙',planned_g/1000.0,'kg',15,1,planned_g/1000.0,planned_g,15 FROM %[1]s.production_plan_items WHERE production_plan_id=%[2]d`, schema, plan.ID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if submit.Code != 200 {
		t.Fatalf("auto submit: %d %s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND status='reserved'", 3)
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=6000", 1)
	var woID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s.work_orders WHERE production_plan_id=$1", schema), plan.ID).Scan(&woID); err != nil {
		t.Fatal(err)
	}
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", woID), map[string]any{})
	if start.Code != 400 || !strings.Contains(start.Body.String(), "WIP") {
		t.Fatalf("source reservation outside WIP incorrectly allowed start: %s", start.Body.String())
	}
	stockService := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	issue := func(warehouse string, qty int64, key string) stockapp.StockDocumentDetail {
		t.Helper()
		detail, err := stockService.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: woID, Operator: "仓库员", IdempotencyKey: key, Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 30, InventoryUnit: "kg", QtyG: qty, FromWarehouse: warehouse, ToWarehouse: "wip", BatchCode: "AUTO-GREEN"}}})
		if err != nil {
			t.Fatalf("partial picking %s %d: %v", warehouse, qty, err)
		}
		return detail
	}
	first := issue("raw_materials", 4000, "pick-first")
	again := issue("raw_materials", 4000, "pick-first")
	if first.ID != again.ID {
		t.Fatal("repeated picking created another document")
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='raw_materials' AND reserved_g=6000", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='wip' AND reserved_g=10000", 1)
	start = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", woID), nil)
	if start.Code == 200 {
		t.Fatal("partial picking incorrectly allowed start")
	}
	if _, err := stockService.CancelStockDocument(ctx, first.ID, "仓库员"); err != nil {
		t.Fatalf("cancel partial picking: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=6000", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='raw_materials' AND reserved_g=10000", 1)
	issue("raw_materials", 4000, "pick-first-again")
	issue("raw_materials", 6000, "pick-rest-a")
	issue("raw_backup", 5000, "pick-b")
	start = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", woID), nil)
	if start.Code != 200 {
		t.Fatalf("fully picked start %d %s", start.Code, start.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "material_batch_locations", "material_batch_id=1030 AND warehouse='wip' AND qty_g=21000", 1)
	_, err := stockService.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: woID, Operator: "操作员", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 30, InventoryUnit: "kg", QtyG: 1000, FromWarehouse: "raw_materials", BatchCode: "AUTO-GREEN"}}})
	if err == nil || !strings.Contains(err.Error(), "WIP") {
		t.Fatalf("non-WIP consumption must be rejected: %v", err)
	}
	_, err = stockService.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialConsumption, WorkOrderID: woID, Operator: "操作员", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 30, InventoryUnit: "kg", QtyG: 1000, FromWarehouse: "wip", BatchCode: "AUTO-GREEN"}}})
	if err != nil {
		t.Fatalf("frozen WIP consumption: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "material_batch_id=1030 AND warehouse='wip' AND consumed_g=1000", 1)

}

func pickAllProductionComponents(t *testing.T, pool *pgxpool.Pool, schema string, app *echo.Echo, workOrderID int64) {
	t.Helper()
	preview := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", workOrderID), map[string]any{"action": "issue"})
	var data productionapp.StockDocumentPreview
	if preview.Code != 200 {
		t.Fatalf("picking preview %s", preview.Body.String())
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	stockService := stockapp.NewService(postgresstock.NewRepository(pool, schema))
	for _, i := range data.Document.Items {
		draft, err := stockService.CreateStockDocumentDraft(context.Background(), stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: workOrderID, Operator: "仓库员", Items: []stockapp.StockDocumentItemCommand{{ItemType: i.ItemType, MaterialID: i.MaterialID, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, InventoryUnit: i.InventoryUnit, OwnerCustomerID: i.OwnerCustomerID, QtyG: i.QtyG, QtyUnits: i.QtyUnits, BatchCode: i.BatchCode, FromWarehouse: i.FromWarehouse, ToWarehouse: i.ToWarehouse}}})
		if err != nil {
			t.Fatalf("picking: %v", err)
		}
		restored := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", workOrderID), map[string]any{"action": "issue", "stock_document_id": draft.ID})
		var restoredData productionapp.StockDocumentPreview
		if restored.Code != 200 {
			t.Fatalf("restore picking draft: %s", restored.Body.String())
		}
		if err = json.Unmarshal(restored.Body.Bytes(), &restoredData); err != nil {
			t.Fatal(err)
		}
		if len(restoredData.Document.Items) != 1 {
			t.Fatal("saved picking row lost")
		}
		got := restoredData.Document.Items[0]
		if got.OwnerCustomerID != i.OwnerCustomerID || got.BomSpecID != i.BomSpecID || got.BomVariantID != i.BomVariantID || got.FrozenPicking != i.FrozenPicking || got.BatchCode != i.BatchCode {
			t.Fatalf("saved picking identity changed: %+v want %+v", got, i)
		}
		if len(draft.Items) != 1 || draft.Items[0].FrozenPicking != i.FrozenPicking {
			t.Fatal("saved picking lost edit lock")
		}
		if _, err = stockService.SubmitStockDocument(context.Background(), draft.ID, "仓库员"); err != nil {
			t.Fatalf("submit picking: %v", err)
		}
	}
}

func TestAutoPickingPreservesManualIntentOnRefreshAndRejectsShortSupply(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 3030, 30, "MANUAL-WIP", "生产生豆", 30000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;UPDATE %[1]s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=3030;INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(3030,30,'raw_materials',20000)`, schema))
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	var plan productionapp.ProductionPlanDetail
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	source := plan.ComponentSources[0]
	source.AllocationMode = "manual"
	source.ManualAllocations = []productionapp.ProductionPlanSourceAllocation{{Warehouse: "raw_materials", QtyG: 15000}}
	rec = serveMultilevelProductionJSON(t, app, http.MethodPatch, fmt.Sprintf("/api/production-plans/%d/draft", plan.ID), map[string]any{"draft_token": plan.DraftToken, "component_sources": []productionapp.ProductionPlanComponentSource{source}})
	if rec.Code != 200 {
		t.Fatalf("save manual %s", rec.Body.String())
	}
	for i := 0; i < 2; i++ {
		rec = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/refresh-supply", plan.ID), nil)
		if rec.Code != 200 {
			t.Fatalf("refresh manual %s", rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
			t.Fatal(err)
		}
		s := plan.ComponentSources[0]
		if s.WIPCoveredG != 6000 || s.TransferG != 15000 || s.AllocationMode != "manual" || len(s.ManualAllocations) != 1 {
			t.Fatalf("manual lost on refresh %+v", s)
		}
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "true", 0)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=0 WHERE material_batch_id=3030 AND warehouse='raw_materials'`, schema))
	rec = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", plan.ID), nil)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "生产生豆") || !strings.Contains(rec.Body.String(), "草稿已保留") {
		t.Fatalf("short submit %s", rec.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='draft'", plan.ID), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", "true", 0)
}

func TestAutoPickingOwnerAndQualityIsolation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4030, 30, "OWNED-WIP", "生产生豆", 6000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4031, 30, "OTHER-OWNER", "生产生豆", 50000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 4032, 30, "HELD-WIP", "生产生豆", 50000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.customers(id,name) VALUES(7001,'测试货主甲'),(7002,'测试货主乙');
 UPDATE %[1]s.materials SET owner_customer_id=7001 WHERE id IN(10,30);
 UPDATE %[1]s.materials SET is_semi_finished=true,purchase_price=0 WHERE id=10;
 UPDATE %[1]s.material_batches SET owner_customer_id=CASE WHEN id=4031 THEN 7002 ELSE 7001 END WHERE id IN(4030,4031,4032);
 UPDATE %[1]s.material_batches SET quality_status='hold' WHERE id=4032;
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,warehouse,qty_g) VALUES(4030,30,'raw_materials',15000);
 UPDATE %[1]s.material_batches SET qty_g=21000,remaining_g=21000 WHERE id=4030;
 `, schema))
	app := newProductionFlowTestEcho(pool, schema)
	rec := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/preview", map[string]any{"source_type": "stock", "items": []map[string]any{{"output_material_id": 10, "output_qty": 16.8, "target_warehouse": "wip"}}})
	var preview productionapp.ProductionPlanPreview
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.ComponentSources) != 1 {
		t.Fatalf("preview sources %+v", preview.ComponentSources)
	}
	source := preview.ComponentSources[0]
	if source.WIPCoveredG != 6000 || source.TransferG != 15000 || source.ShortageG != 0 {
		t.Fatalf("cross owner or held stock leaked %+v", source)
	}
	for _, a := range source.Allocations {
		if a.OwnerCustomerID != 7001 {
			t.Fatalf("wrong owner %+v", a)
		}
		for _, b := range a.Batches {
			if b.BatchID != 4030 {
				t.Fatalf("wrong batch %+v", b)
			}
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_plans", "true", 0)
}
