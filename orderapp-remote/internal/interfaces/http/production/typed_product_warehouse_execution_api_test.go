package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
	"testing"
)

func TestTypedProductDependencyConsumesBatchesFromFrozenWarehouses(t *testing.T) {
	testTypedProductWarehouseFlow(t, true)
}
func TestAutoPickingTypedProductMustTransferBeforeConsumption(t *testing.T) {
	testTypedProductWarehouseFlow(t, false)
}
func testTypedProductWarehouseFlow(t *testing.T, legacy bool) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedTypedProductComponentFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 12_030, 30, "MB-TYPED-WAREHOUSE-GREEN", "生产生豆", 20_000)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":         "2026-08-01",
		"to":           "2026-08-31",
		"selected":     []string{"1-227"},
		"input_by_key": map[string]int64{"1-227": 22_700},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create warehouse-aware typed product plan status=%d body=%s", create.Code, create.Body.String())
	}

	if !legacy {
		var preview productionapp.ProductionPlanDetail
		if err := json.Unmarshal(create.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, source := range preview.ComponentSources {
			if source.ComponentType != "product" {
				continue
			}
			for _, allocation := range source.Allocations {
				found = true
				if len(allocation.Batches) == 0 {
					t.Fatal("product picking suggestion must include frozen-spec FIFO batches")
				}
			}
		}
		if !found {
			t.Fatal("fixture must exercise stocked product component")
		}
	}

	var planID, upstreamPlanItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT production_plan_id,id
		FROM %s.production_plan_items
		WHERE output_type='product' AND output_product_id=2
		ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&planID, &upstreamPlanItemID); err != nil {
		t.Fatalf("load typed upstream plan item: %v", err)
	}
	if legacy {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.production_plans SET picking_version=0 WHERE id=%d`, schema, planID))
	}
	update := serveMultilevelProductionJSON(t, app, http.MethodPatch,
		fmt.Sprintf("/api/production-plans/%d/items/%d/target-warehouse", planID, upstreamPlanItemID),
		map[string]any{"target_warehouse": "finished_shop"})
	if update.Code != http.StatusOK {
		t.Fatalf("freeze typed upstream target warehouse status=%d body=%s", update.Code, update.Body.String())
	}

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_product_id=2 THEN '烘焙' ELSE '包装' END,
		       planned_output_g,'g',15,1,planned_output_g,planned_output_g,15
		FROM %s.production_plan_items
		WHERE production_plan_id=%d;
	`, schema, schema, planID))

	selectPlanningTestSources(t, app, planID)
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit warehouse-aware typed product plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	var rootWorkOrderID, upstreamWorkOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='product' AND output_product_id=1
	`, schema), planID).Scan(&rootWorkOrderID); err != nil {
		t.Fatalf("load typed root work order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_type='product' AND output_product_id=2
	`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
		t.Fatalf("load typed upstream work order: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf(
		"id=%d AND target_warehouse='finished_shop'", upstreamWorkOrderID,
	), 1)

	upstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
	if upstreamStart.Code != http.StatusOK {
		t.Fatalf("start typed upstream status=%d body=%s", upstreamStart.Code, upstreamStart.Body.String())
	}
	var upstreamRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), upstreamWorkOrderID).Scan(&upstreamRunningItemID); err != nil {
		t.Fatalf("load typed upstream running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=12700,actual_output_qty=12700
		WHERE work_order_id=%d;
	`, schema, upstreamWorkOrderID))
	wrongWarehouse := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
		"finished_units":   12_700,
		"finished_loose_g": 0,
		"consumed_input_g": 12_700,
		"warehouse":        "finished_goods",
		"note":             "must reject completion outside frozen target warehouse",
	})
	if wrongWarehouse.Code != http.StatusBadRequest {
		t.Fatalf("complete typed upstream outside frozen warehouse status=%d body=%s, want rejection", wrongWarehouse.Code, wrongWarehouse.Body.String())
	}
	upstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
		"finished_units":   12_700,
		"finished_loose_g": 0,
		"consumed_input_g": 12_700,
		"warehouse":        "finished_shop",
		"note":             "produce typed dependency into frozen warehouse",
	})
	if upstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete typed upstream into frozen warehouse status=%d body=%s", upstreamComplete.Code, upstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='finished_goods' AND onhand_units=10000", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='finished_shop' AND onhand_units=12700", 1)

	if !legacy {
		blocked := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
		if blocked.Code == 200 {
			t.Fatal("product in source warehouses cannot start")
		}
		preview := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/stock-document-preview", rootWorkOrderID), map[string]any{"action": "issue"})
		var data productionapp.StockDocumentPreview
		if err := json.Unmarshal(preview.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if preview.Code != 200 || len(data.Document.Items) != 2 {
			t.Fatalf("typed picking preview %s", preview.Body.String())
		}
		stockService := stockapp.NewService(postgresstock.NewRepository(pool, schema))
		for _, i := range data.Document.Items {
			if _, err := stockService.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialTransferForManufacture, WorkOrderID: rootWorkOrderID, Operator: "仓库员", Items: []stockapp.StockDocumentItemCommand{{ItemType: i.ItemType, MaterialID: i.MaterialID, ProductID: i.ProductID, BomSpecID: i.BomSpecID, BomVariantID: i.BomVariantID, SpecG: i.SpecG, InventoryUnit: i.InventoryUnit, OwnerCustomerID: i.OwnerCustomerID, QtyG: i.QtyG, QtyUnits: i.QtyUnits, BatchCode: i.BatchCode, FromWarehouse: i.FromWarehouse, ToWarehouse: i.ToWarehouse}}}); err != nil {
				var batches string
				_ = pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_agg(to_jsonb(b))::text FROM %s.stock_batches b WHERE item_id=2`, schema)).Scan(&batches)
				t.Fatalf("typed picking: %v item=%+v batches=%s", err, i, batches)
			}
		}
	}
	downstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if downstreamStart.Code != http.StatusOK {
		t.Fatalf("start typed downstream after mixed-warehouse supply status=%d body=%s", downstreamStart.Code, downstreamStart.Body.String())
	}
	var rootRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), rootWorkOrderID).Scan(&rootRunningItemID); err != nil {
		t.Fatalf("load typed root running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=22700,actual_output_qty=22700
		WHERE work_order_id=%d;
	`, schema, rootWorkOrderID))
	downstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", rootWorkOrderID), map[string]any{
		"finished_units":   100,
		"finished_loose_g": 0,
		"consumed_input_g": 22_700,
		"warehouse":        "finished_goods",
		"note":             "consume typed dependency from frozen warehouses",
	})
	if downstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete typed downstream from mixed warehouses status=%d body=%s", downstreamComplete.Code, downstreamComplete.Body.String())
	}

	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='finished_goods' AND onhand_units=0 AND onhand_loose_g=0", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=2 AND spec_g=1 AND warehouse='finished_shop' AND onhand_units=0 AND onhand_loose_g=0", 1)
	if !legacy {
		assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf("work_order_id=%d AND component_type='product' AND warehouse='wip' AND status='consumed'", rootWorkOrderID), 2)
		return
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND warehouse='finished_goods' AND reserved_g=10000 AND consumed_g=10000 AND status='consumed'",
		rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND component_type='product' AND component_id=2 AND warehouse='finished_shop' AND reserved_g=12700 AND consumed_g=12700 AND status='consumed'",
		rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf(
		"source_doc_type='production_run' AND source_doc_id=%d AND item_type='finished_product' AND item_id=2 AND warehouse='finished_goods' AND qty_change_g=-10000",
		rootRunningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf(
		"source_doc_type='production_run' AND source_doc_id=%d AND item_type='finished_product' AND item_id=2 AND warehouse='finished_shop' AND qty_change_g=-12700",
		rootRunningItemID,
	), 1)
}
