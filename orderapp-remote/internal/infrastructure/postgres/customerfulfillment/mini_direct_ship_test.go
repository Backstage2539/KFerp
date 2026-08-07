package customerfulfillment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
	inventoryapp "orderapp/internal/application/inventory"
	salesapp "orderapp/internal/application/sales"
	stockapp "orderapp/internal/application/stock"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMiniDirectShipClosedLoopFIFOIsolationIdempotencyAndCancellation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	repo := NewRepository(pool, schema)

	base := app.MiniDirectShipCommand{
		CustomerID: 501, EmployeeID: 701, MiniUserID: 801, IdempotencyKey: "DEV-E2E-DS-1",
		RecipientName: "张三", RecipientPhone: "13800138000", Province: "云南省", City: "普洱市",
		District: "思茅区", DetailAddress: "咖啡路 8 号",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 911, SpecG: 1000, Qty: 3}},
		Actor: "mini_user:801",
	}
	preview, err := repo.PreviewMiniDirectShip(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanSubmit || len(preview.Warehouses) != 2 || len(preview.Shortages) != 0 {
		t.Fatalf("preview = %#v, want FIFO split across two customer warehouses", preview)
	}
	shortageCmd := base
	shortageCmd.Items = []app.MiniDirectShipItemCommand{{ProductID: 911, SpecG: 1000, Qty: 5}}
	shortage, err := repo.PreviewMiniDirectShip(ctx, shortageCmd)
	if err != nil {
		t.Fatal(err)
	}
	if shortage.CanSubmit || len(shortage.Shortages) != 1 || shortage.Shortages[0].AvailableQty != 4 {
		t.Fatalf("shortage = %#v, other customer's 100 units must stay isolated", shortage)
	}

	created, err := repo.SubmitMiniDirectShip(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID <= 0 || created.Status != "reserved" || len(created.Packages) != 2 || len(created.Items) != 1 || created.Items[0].ProductName != "萨其姆 1kg" || created.Items[0].SKUCode != "DEV-E2E-911" || created.Items[0].SpecLabel != "1kg" {
		t.Fatalf("created = %#v", created)
	}
	var firstBatch, secondBatch, firstQty, secondQty int64
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT batch_id,allocated_qty FROM %s.customer_direct_ship_request_allocations
		WHERE request_id=$1 ORDER BY id
	`, schema), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Scan(&firstBatch, &firstQty) != nil || !rows.Next() || rows.Scan(&secondBatch, &secondQty) != nil {
		rows.Close()
		t.Fatal("expected two FIFO allocation rows")
	}
	rows.Close()
	if firstBatch != 1001 || firstQty != 2 || secondBatch != 1002 || secondQty != 1 {
		t.Fatalf("FIFO allocations = batch %d/%d then %d/%d", firstBatch, firstQty, secondBatch, secondQty)
	}

	again, err := repo.SubmitMiniDirectShip(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatalf("idempotent request ID = %d, want %d", again.ID, created.ID)
	}
	changed := base
	changed.Note = "different payload"
	if _, err := repo.SubmitMiniDirectShip(ctx, changed); !errors.Is(err, app.ErrMiniDirectShipIdempotency) {
		t.Fatalf("changed payload idempotency error = %v", err)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_direct_ship_requests", "customer_id=501", 1)
	assertMiniDirectShipCount(t, pool, schema, "orders", "customer_id=501 AND portal_service_code='direct_ship'", 2)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "entity_type='customer_direct_ship_request' AND action='submit'", 1)

	inventory, err := repo.ListCustomerCentralInventory(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 1 || inventory[0].TotalQty != 4 || inventory[0].ReservedQty != 3 || inventory[0].AvailableQty != 1 || len(inventory[0].Warehouses) != 2 {
		t.Fatalf("reserved central inventory = %#v", inventory)
	}
	batches, err := repo.ListCustomerCentralInventoryBatches(ctx, 501, 911, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || batches[0].ProductionDate != "2026-08-01" || batches[0].AvailableQty != 0 || batches[0].ReservedQty != 2 || batches[1].AvailableQty != 1 || batches[1].ReservedQty != 1 {
		t.Fatalf("batch inventory = %#v", batches)
	}

	cancelled, err := repo.CancelMiniDirectShipRequest(ctx, 501, created.ID, "mini_user:801")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled = %#v", cancelled)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_stock_batch_allocations", fmt.Sprintf("request_id=%d", created.ID), 0)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='customer_direct_ship_request' AND entity_id=%d", created.ID), 2)
	assertMiniDirectShipCount(t, pool, schema, "orders", fmt.Sprintf("id IN (SELECT order_id FROM %s.customer_direct_ship_request_orders WHERE request_id=%d) AND is_void=true", schema, created.ID), 2)

	availableAgain, err := repo.ListCustomerCentralInventory(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if len(availableAgain) != 1 || availableAgain[0].AvailableQty != 4 || availableAgain[0].ReservedQty != 0 {
		t.Fatalf("released inventory = %#v", availableAgain)
	}
	var cancelledBatchG, cancelledAggregateG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_g),0) FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=911 AND batch_code LIKE 'DEV-E2E-FP-A%%'
	`, schema)).Scan(&cancelledBatchG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(onhand_units*spec_g+onhand_loose_g),0) FROM %s.finished_inventory
		WHERE product_id=911 AND spec_g=1000 AND warehouse IN ('DEV-E2E-A1','DEV-E2E-A2')
	`, schema)).Scan(&cancelledAggregateG); err != nil {
		t.Fatal(err)
	}
	if cancelledBatchG != 4000 || cancelledAggregateG != 4000 {
		t.Fatalf("cancelled reservation changed physical stock: batches=%dg aggregate=%dg", cancelledBatchG, cancelledAggregateG)
	}

	// The customer advisory lock plus transactional reservations must let only
	// one of two concurrent three-unit requests consume the four available units.
	cmds := []app.MiniDirectShipCommand{base, base}
	cmds[0].IdempotencyKey = "DEV-E2E-CONCURRENT-A"
	cmds[1].IdempotencyKey = "DEV-E2E-CONCURRENT-B"
	var wg sync.WaitGroup
	errs := make([]error, 2)
	results := make([]app.MiniDirectShipRequest, 2)
	for i := range cmds {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = repo.SubmitMiniDirectShip(ctx, cmds[index])
		}(i)
	}
	wg.Wait()
	successes, shortages := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, app.ErrMiniDirectShipStockInsufficient) {
			shortages++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || shortages != 1 {
		t.Fatalf("concurrent outcomes successes=%d shortages=%d errors=%v", successes, shortages, errs)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_stock_batch_allocations", "request_id>0", 2)
	for _, result := range results {
		if result.ID > 0 {
			if _, err := repo.CancelMiniDirectShipRequest(ctx, 501, result.ID, "mini_user:801"); err != nil {
				t.Fatal(err)
			}
		}
	}

	shipCmd := base
	shipCmd.IdempotencyKey = "DEV-E2E-SHIP"
	shipCmd.Items = []app.MiniDirectShipItemCommand{{ProductID: 911, SpecG: 1000, Qty: 4}}
	shippedRequest, err := repo.SubmitMiniDirectShip(ctx, shipCmd)
	if err != nil {
		t.Fatal(err)
	}
	var shipmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipments(shipment_no,created_by,status)
		VALUES('DEV-E2E-SHIPMENT','erp-test','excel_generated') RETURNING id
	`, schema)).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	trackingItems := make([]salesapp.ShipmentTrackingItemCommand, 0, len(shippedRequest.Packages))
	for index, pkg := range shippedRequest.Packages {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_shipment_orders(shipment_id,order_id,tracking_no)
			VALUES($1,$2,'')
		`, schema), shipmentID, pkg.OrderID); err != nil {
			t.Fatal(err)
		}
		trackingItems = append(trackingItems, salesapp.ShipmentTrackingItemCommand{OrderID: pkg.OrderID, TrackingNo: fmt.Sprintf("SF-DEV-%02d", index+1)})
	}
	salesRepo := postgressales.NewRepository(pool, schema)
	filled, err := salesRepo.FillShipmentTracking(ctx, salesapp.FillShipmentTrackingCommand{Actor: "erp-test", ShipmentID: shipmentID, Items: trackingItems})
	if err != nil {
		t.Fatal(err)
	}
	if filled.Updated != 2 {
		t.Fatalf("shipment updated=%d want 2", filled.Updated)
	}
	var batchRemainingUnits, aggregateUnits, aggregateLooseG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0) FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=911 AND batch_code LIKE 'DEV-E2E-FP-A%%'
	`, schema)).Scan(&batchRemainingUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(onhand_units),0),COALESCE(SUM(onhand_loose_g),0)
		FROM %s.finished_inventory WHERE product_id=911 AND spec_g=1000 AND warehouse IN ('DEV-E2E-A1','DEV-E2E-A2')
	`, schema)).Scan(&aggregateUnits, &aggregateLooseG); err != nil {
		t.Fatal(err)
	}
	if batchRemainingUnits != 0 || aggregateUnits != 0 || aggregateLooseG != 0 {
		t.Fatalf("shipment stock mismatch: batches=%d aggregate=%d+%dg", batchRemainingUnits, aggregateUnits, aggregateLooseG)
	}
	erpInventory, err := postgresinventory.NewRepository(pool, schema).ListFinished(ctx, inventoryapp.FinishedInventoryQuery{Q: "DEV-E2E 萨其姆", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range erpInventory.Rows {
		if (row.Warehouse == "DEV-E2E-A1" || row.Warehouse == "DEV-E2E-A2") && (row.Units != 0 || row.LooseG != 0 || row.TotalG != 0) {
			t.Fatalf("ERP finished inventory retained shipped stock: %#v", row)
		}
	}
	stockRepo := postgresstock.NewRepository(pool, schema)
	for _, warehouse := range []string{"DEV-E2E-A1", "DEV-E2E-A2"} {
		warehouseInventory, err := stockRepo.ListWarehouseInventory(ctx, stockapp.WarehouseInventoryQuery{Warehouse: warehouse, ItemType: "finished_product", CustomerID: 501, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(warehouseInventory.Rows) != 0 {
			t.Fatalf("ERP stock inventory for %s refilled shipped aggregate: %#v", warehouse, warehouseInventory.Rows)
		}
	}
	for index, pkg := range shippedRequest.Packages {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_shipping_tracking_events(order_id,tracking_no,event_time,status,description,location,source)
			VALUES($1,$2,'2026-08-07 12:00+08','signed','客户已签收','昆明市','DEV-E2E')
		`, schema), pkg.OrderID, trackingItems[index].TrackingNo); err != nil {
			t.Fatal(err)
		}
	}
	shippedDetail, err := repo.GetMiniDirectShipRequest(ctx, 501, shippedRequest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if shippedDetail.Status != "delivered" || len(shippedDetail.Packages) != 2 || shippedDetail.Packages[0].Status != "delivered" || shippedDetail.Packages[0].DeliveredAt == "" || len(shippedDetail.Packages[0].Events) != 1 {
		t.Fatalf("delivered request = %#v", shippedDetail)
	}
	if _, err := repo.CancelMiniDirectShipRequest(ctx, 501, shippedRequest.ID, "mini_user:801"); !errors.Is(err, app.ErrMiniDirectShipCannotCancel) {
		t.Fatalf("cancel shipped error=%v", err)
	}
	postShipmentInventory, err := repo.ListCustomerCentralInventory(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if len(postShipmentInventory) != 0 {
		t.Fatalf("post-shipment inventory = %#v; shipped traceable batch must not reappear as legacy stock", postShipmentInventory)
	}
	postShipmentCatalog, err := repo.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	if len(postShipmentCatalog.ProductFamilies) != 0 {
		t.Fatalf("post-shipment catalog = %#v; depleted SKU must disappear", postShipmentCatalog)
	}
	postShipmentPreview := shipCmd
	postShipmentPreview.IdempotencyKey = ""
	postShipmentPreview.Items = []app.MiniDirectShipItemCommand{{ProductID: 911, SpecG: 1000, Qty: 2}}
	previewAfterShipment, err := repo.PreviewMiniDirectShip(ctx, postShipmentPreview)
	if err != nil {
		t.Fatal(err)
	}
	if previewAfterShipment.CanSubmit || len(previewAfterShipment.Shortages) != 1 || previewAfterShipment.Shortages[0].AvailableQty != 0 {
		t.Fatalf("post-shipment preview = %#v; shipped stock must not return", previewAfterShipment)
	}
	postShipmentBatches, err := repo.ListCustomerCentralInventoryBatches(ctx, 501, 911, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(postShipmentBatches) != 0 {
		t.Fatalf("post-shipment batches = %#v", postShipmentBatches)
	}
}

func TestMiniDirectShipLegacyShipmentNormalizesLooseInventoryAndDeductsOnce(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(id,name,active) VALUES (601,'DEV-E2E 旧库存客户',true);
		INSERT INTO %[1]s.products(id,name,sku_name,sku_code,parent_product_id,spec_label,net_content_qty,net_content_unit,product_kind,active)
		VALUES
			(920,'DEV-E2E 旧库存商品','','',0,'',0,'','roasted_bean',true),
			(921,'DEV-E2E 旧库存商品 1kg','旧库存商品 1kg','DEV-E2E-921',920,'1kg',1000,'g','roasted_bean',true);
		INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active,customer_id)
		VALUES('DEV-E2E-L1','旧库存客户成品仓','finished',1,true,601)
		ON CONFLICT (code) DO UPDATE SET customer_id=excluded.customer_id,kind=excluded.kind,active=true;
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(921,1000,'DEV-E2E-L1',0,2000)
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	created, err := repo.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 601, EmployeeID: 702, MiniUserID: 802, IdempotencyKey: "DEV-E2E-LEGACY-SHIP",
		RecipientName: "李四", RecipientPhone: "13900139000", Province: "云南省", City: "昆明市",
		District: "盘龙区", DetailAddress: "咖啡路 9 号",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 921, SpecG: 1000, Qty: 1}},
		Actor: "mini_user:802",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Packages) != 1 {
		t.Fatalf("legacy direct ship packages = %#v", created.Packages)
	}
	orderID := created.Packages[0].OrderID
	var allocationBatchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT batch_id FROM %s.order_stock_batch_allocations WHERE order_id=$1
	`, schema), orderID).Scan(&allocationBatchID); err != nil {
		t.Fatal(err)
	}
	if allocationBatchID != 0 {
		t.Fatalf("legacy allocation batch_id=%d want 0", allocationBatchID)
	}

	var shipmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipments(shipment_no,created_by,status)
		VALUES('DEV-E2E-LEGACY-SHIPMENT','erp-test','excel_generated') RETURNING id
	`, schema)).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipment_orders(shipment_id,order_id,tracking_no) VALUES($1,$2,'')
	`, schema), shipmentID, orderID); err != nil {
		t.Fatal(err)
	}
	fillCmd := salesapp.FillShipmentTrackingCommand{
		Actor: "erp-test", ShipmentID: shipmentID,
		Items: []salesapp.ShipmentTrackingItemCommand{{OrderID: orderID, TrackingNo: "SF-DEV-LEGACY"}},
	}
	salesRepo := postgressales.NewRepository(pool, schema)
	if _, err := salesRepo.FillShipmentTracking(ctx, fillCmd); err != nil {
		t.Fatal(err)
	}
	if _, err := salesRepo.FillShipmentTracking(ctx, fillCmd); err != nil {
		t.Fatal(err)
	}

	var units, looseG, deductionCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=921 AND spec_g=1000 AND warehouse='DEV-E2E-L1'
	`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.order_stock_deductions WHERE order_id=$1
	`, schema), orderID).Scan(&deductionCount); err != nil {
		t.Fatal(err)
	}
	if units != 1 || looseG != 0 || deductionCount != 1 {
		t.Fatalf("legacy shipment inventory=%d units + %dg deductions=%d; want normalized 1+0g and one deduction", units, looseG, deductionCount)
	}
	legacyInventory, err := repo.ListCustomerCentralInventory(ctx, 601)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyInventory) != 1 || legacyInventory[0].TotalQty != 1 || legacyInventory[0].AvailableQty != 1 || legacyInventory[0].ReservedQty != 0 {
		t.Fatalf("legacy inventory after shipment = %#v", legacyInventory)
	}
	legacyCatalog, err := repo.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 601})
	if err != nil {
		t.Fatal(err)
	}
	assertMiniCatalogAvailableQty(t, legacyCatalog, 921, 1000, 1)
	legacyPreview, err := repo.PreviewMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 601, RecipientName: "李四", RecipientPhone: "13900139000", DetailAddress: "咖啡路 9 号",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 921, SpecG: 1000, Qty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !legacyPreview.CanSubmit || len(legacyPreview.Shortages) != 0 {
		t.Fatalf("legacy preview after shipment = %#v", legacyPreview)
	}
	legacyBatches, err := repo.ListCustomerCentralInventoryBatches(ctx, 601, 921, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyBatches) != 1 || legacyBatches[0].BatchID != 0 || legacyBatches[0].AvailableQty != 1 {
		t.Fatalf("legacy batches after shipment = %#v", legacyBatches)
	}
}

func TestMiniDirectShipMixedTraceableAndLegacyStockSurvivesSyncedShipment(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(id,name,active) VALUES (602,'DEV-E2E 混合库存客户',true);
		INSERT INTO %[1]s.products(id,name,sku_name,sku_code,parent_product_id,spec_label,net_content_qty,net_content_unit,product_kind,active)
		VALUES
			(930,'DEV-E2E 混合库存商品','','',0,'',0,'','roasted_bean',true),
			(931,'DEV-E2E 混合库存商品 1kg','混合库存商品 1kg','DEV-E2E-931',930,'1kg',1000,'g','roasted_bean',true);
		INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active,customer_id)
		VALUES('DEV-E2E-M1','混合库存客户成品仓','finished',1,true,602)
		ON CONFLICT (code) DO UPDATE SET customer_id=excluded.customer_id,kind=excluded.kind,active=true;
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(931,1000,'DEV-E2E-M1',1,2000)
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,status,started_at,finished_at)
		VALUES(2101,'DEV-E2E-MIX-P1',931,'DEV-E2E 混合库存商品 1kg',1000,2000,'done','2026-08-04 08:00+08','2026-08-04 10:00+08');
		INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,created_at)
		VALUES(1101,'DEV-E2E-FP-M1','finished_product',931,'DEV-E2E 混合库存商品 1kg',1000,'production_run',2101,2000,2,2000,2,'passed','2026-08-04 10:00+08');
		INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units,created_at)
		VALUES('finished_product',931,'DEV-E2E 混合库存商品 1kg',1000,'DEV-E2E-M1','production_run',2101,'DEV-E2E-FP-M1',2000,2000,2,2,'2026-08-04 10:00+08');
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	before, err := repo.ListCustomerCentralInventory(ctx, 602)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].TotalQty != 3 || before[0].AvailableQty != 3 {
		t.Fatalf("mixed inventory before shipment = %#v, want traceable 2 + legacy 1", before)
	}
	beforeBatches, err := repo.ListCustomerCentralInventoryBatches(ctx, 602, 931, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeBatches) != 2 || beforeBatches[0].BatchID != 1101 || beforeBatches[0].AvailableQty != 2 || beforeBatches[1].BatchID != 0 || beforeBatches[1].AvailableQty != 1 {
		t.Fatalf("mixed batches before shipment = %#v", beforeBatches)
	}

	created, err := repo.SubmitMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 602, EmployeeID: 703, MiniUserID: 803, IdempotencyKey: "DEV-E2E-MIXED-SHIP",
		RecipientName: "王五", RecipientPhone: "13700137000", Province: "云南省", City: "昆明市",
		District: "五华区", DetailAddress: "咖啡路 10 号",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 931, SpecG: 1000, Qty: 1}},
		Actor: "mini_user:803",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Packages) != 1 {
		t.Fatalf("mixed shipment packages = %#v", created.Packages)
	}
	orderID := created.Packages[0].OrderID
	var shipmentID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipments(shipment_no,created_by,status)
		VALUES('DEV-E2E-MIXED-SHIPMENT','erp-test','excel_generated') RETURNING id
	`, schema)).Scan(&shipmentID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_shipment_orders(shipment_id,order_id,tracking_no) VALUES($1,$2,'')
	`, schema), shipmentID, orderID); err != nil {
		t.Fatal(err)
	}
	if _, err := postgressales.NewRepository(pool, schema).FillShipmentTracking(ctx, salesapp.FillShipmentTrackingCommand{
		Actor: "erp-test", ShipmentID: shipmentID,
		Items: []salesapp.ShipmentTrackingItemCommand{{OrderID: orderID, TrackingNo: "SF-DEV-MIXED"}},
	}); err != nil {
		t.Fatal(err)
	}

	after, err := repo.ListCustomerCentralInventory(ctx, 602)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].TotalQty != 2 || after[0].AvailableQty != 2 || after[0].ReservedQty != 0 {
		t.Fatalf("mixed inventory after synced shipment = %#v, want traceable 1 + legacy 1", after)
	}
	catalog, err := repo.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 602})
	if err != nil {
		t.Fatal(err)
	}
	assertMiniCatalogAvailableQty(t, catalog, 931, 1000, 2)
	preview, err := repo.PreviewMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 602, RecipientName: "王五", RecipientPhone: "13700137000", DetailAddress: "咖啡路 10 号",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 931, SpecG: 1000, Qty: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanSubmit || len(preview.Shortages) != 0 {
		t.Fatalf("mixed preview after shipment = %#v, remaining traceable+legacy must both be available", preview)
	}
	afterBatches, err := repo.ListCustomerCentralInventoryBatches(ctx, 602, 931, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterBatches) != 2 || afterBatches[0].BatchID != 1101 || afterBatches[0].AvailableQty != 1 || afterBatches[1].BatchID != 0 || afterBatches[1].AvailableQty != 1 {
		t.Fatalf("mixed batches after synced shipment = %#v", afterBatches)
	}
}

func TestMiniDirectShipHistoricalUnsyncedMixedStockAndCompatibleWarehouseKinds(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customers(id,name,active) VALUES (603,'DEV-E2E 历史混合库存客户',true);
		INSERT INTO %[1]s.products(id,name,sku_name,sku_code,parent_product_id,spec_label,net_content_qty,net_content_unit,product_kind,active)
		VALUES
			(940,'DEV-E2E 历史混合库存商品','','',0,'',0,'','roasted_bean',true),
			(941,'DEV-E2E 历史混合库存商品 1kg','历史混合库存商品 1kg','DEV-E2E-941',940,'1kg',1000,'g','roasted_bean',true),
			(942,'DEV-E2E 历史纯批次商品 1kg','历史纯批次商品 1kg','DEV-E2E-942',940,'1kg',1000,'g','roasted_bean',true);
		INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active,customer_id) VALUES
			('DEV-E2E-HIST-CUSTOMER','历史客户代加工成品仓','customer_processing',1,true,603),
			('DEV-E2E-HIST-RAW','历史客户原料仓','raw',2,true,603),
			('DEV-E2E-HIST-WIP','历史客户WIP仓','wip',3,true,603),
			('DEV-E2E-HIST-PACK','历史客户包材仓','packaging',4,true,603),
			('DEV-E2E-HIST-LOSS','历史客户损耗仓','loss',5,true,603)
		ON CONFLICT (code) DO UPDATE SET customer_id=excluded.customer_id,kind=excluded.kind,active=true;
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES
			(941,1000,'DEV-E2E-HIST-CUSTOMER',4,0),
			(942,1000,'DEV-E2E-HIST-CUSTOMER',2,0),
			(941,1000,'DEV-E2E-HIST-RAW',10,0),
			(941,1000,'DEV-E2E-HIST-WIP',10,0),
			(941,1000,'DEV-E2E-HIST-PACK',10,0),
			(941,1000,'DEV-E2E-HIST-LOSS',10,0)
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,created_at)
		VALUES
			(1201,'DEV-E2E-FP-HIST-D','finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'production_run',2201,1000,1,0,0,'passed','2026-08-01 10:00+08'),
			(1202,'DEV-E2E-FP-HIST-L','finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'production_run',2202,1000,1,0,0,'passed','2026-08-02 10:00+08'),
			(1203,'DEV-E2E-FP-HIST-R','finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'production_run',2203,1000,1,1000,1,'passed','2026-08-03 10:00+08'),
			(1210,'DEV-E2E-FP-HIST-ALL','finished_product',942,'DEV-E2E 历史纯批次商品 1kg',1000,'production_run',2210,2000,2,0,0,'passed','2026-08-01 10:00+08');
		INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units,created_at)
		VALUES
			('finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER','production_run',2201,'DEV-E2E-FP-HIST-D',1000,1000,1,1,'2026-08-01 10:00+08'),
			('finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER','production_run',2202,'DEV-E2E-FP-HIST-L',1000,1000,1,1,'2026-08-02 10:00+08'),
			('finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER','production_run',2203,'DEV-E2E-FP-HIST-R',1000,1000,1,1,'2026-08-03 10:00+08'),
			('finished_product',942,'DEV-E2E 历史纯批次商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER','production_run',2210,'DEV-E2E-FP-HIST-ALL',2000,2000,2,2,'2026-08-01 10:00+08');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	var historicalOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(order_no,order_date,customer_id,portal_service_code,source_warehouse,receiver_name,receiver_phone,receiver_address)
		VALUES('DEV-E2E-HIST-ORDER','2026-08-04',603,'direct_ship','DEV-E2E-HIST-CUSTOMER','历史收件人','13600136000','昆明')
		RETURNING id
	`, schema)).Scan(&historicalOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_stock_deductions(
			order_id,product_id,spec_g,batch_id,batch_code,deducted_g,source_doc_type,source_doc_id,operator,created_at
		) VALUES
			($1,941,1000,1201,'DEV-E2E-FP-HIST-D',1000,'sales_order_shipment',$1,'legacy-test','2026-08-04 12:00+08'),
			($1,942,1000,1210,'DEV-E2E-FP-HIST-ALL',2000,'sales_order_shipment',$1,'legacy-test','2026-08-04 12:00+08')
	`, schema), historicalOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator,created_at
		) VALUES
			('finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER',
			 'sales_order_shipment',$1,'DEV-E2E-FP-HIST-D',1000,-1000,0,1,-1,0,'legacy-test','2026-08-04 12:00+08'),
			('finished_product',941,'DEV-E2E 历史混合库存商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER',
			 'sales_order_shipment',$1,'DEV-E2E-FP-HIST-L',1000,-1000,0,1,-1,0,'legacy-test','2026-08-04 12:01+08')
			,('finished_product',942,'DEV-E2E 历史纯批次商品 1kg',1000,'DEV-E2E-HIST-CUSTOMER',
			 'sales_order_shipment',$1,'DEV-E2E-FP-HIST-ALL',2000,-2000,0,2,-2,0,'legacy-test','2026-08-04 12:02+08')
	`, schema), historicalOrderID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	inventory, err := repo.ListCustomerCentralInventory(ctx, 603)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory) != 1 || inventory[0].TotalQty != 2 || inventory[0].AvailableQty != 2 || len(inventory[0].Warehouses) != 1 || inventory[0].Warehouses[0] != "历史客户代加工成品仓" {
		t.Fatalf("historical mixed compatible-warehouse inventory = %#v, want traceable 1 + legacy 1 only from customer_processing", inventory)
	}
	catalog, err := repo.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 603})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.ProductFamilies) != 1 {
		t.Fatalf("historical mixed catalog = %#v", catalog)
	}
	assertMiniCatalogAvailableQty(t, catalog, 941, 1000, 2)
	preview, err := repo.PreviewMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 603, RecipientName: "历史收件人", RecipientPhone: "13600136000", DetailAddress: "昆明",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 941, SpecG: 1000, Qty: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.CanSubmit || len(preview.Warehouses) != 1 || preview.Warehouses[0].Warehouse != "DEV-E2E-HIST-CUSTOMER" {
		t.Fatalf("historical mixed preview = %#v", preview)
	}
	shortageCmd := app.MiniDirectShipCommand{
		CustomerID: 603, RecipientName: "历史收件人", RecipientPhone: "13600136000", DetailAddress: "昆明",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 941, SpecG: 1000, Qty: 3}},
	}
	shortage, err := repo.PreviewMiniDirectShip(ctx, shortageCmd)
	if err != nil {
		t.Fatal(err)
	}
	if shortage.CanSubmit || len(shortage.Shortages) != 1 || shortage.Shortages[0].AvailableQty != 2 {
		t.Fatalf("historical mixed shortage = %#v", shortage)
	}
	batches, err := repo.ListCustomerCentralInventoryBatches(ctx, 603, 941, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || batches[0].BatchID != 1203 || batches[0].AvailableQty != 1 || batches[1].BatchID != 0 || batches[1].AvailableQty != 1 {
		t.Fatalf("historical mixed batches = %#v", batches)
	}
	pureTracePreview, err := repo.PreviewMiniDirectShip(ctx, app.MiniDirectShipCommand{
		CustomerID: 603, RecipientName: "历史收件人", RecipientPhone: "13600136000", DetailAddress: "昆明",
		Items: []app.MiniDirectShipItemCommand{{ProductID: 942, SpecG: 1000, Qty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pureTracePreview.CanSubmit || len(pureTracePreview.Shortages) != 1 || pureTracePreview.Shortages[0].AvailableQty != 0 {
		t.Fatalf("historical depleted traceable preview = %#v", pureTracePreview)
	}
	pureTraceBatches, err := repo.ListCustomerCentralInventoryBatches(ctx, 603, 942, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(pureTraceBatches) != 0 {
		t.Fatalf("historical depleted traceable batches = %#v", pureTraceBatches)
	}
}

func TestMiniDirectShipPlannerMergesBatchAllocationsIntoWarehousePackages(t *testing.T) {
	items := []app.MiniDirectShipItemCommand{{ProductID: 9, SpecG: 454, Qty: 4}}
	candidates := []miniStockCandidate{
		{ProductID: 9, SpecG: 454, Warehouse: "customer-a", BatchID: 1, BatchCode: "A", AvailableQty: 2},
		{ProductID: 9, SpecG: 454, Warehouse: "customer-a", BatchID: 2, BatchCode: "B", AvailableQty: 1},
		{ProductID: 9, SpecG: 454, Warehouse: "customer-b", BatchID: 3, BatchCode: "C", AvailableQty: 1},
	}
	allocations, shortages := planMiniDirectShipAllocations(items, candidates)
	preview := miniDirectShipPreview(allocations, shortages)
	if !preview.CanSubmit || len(preview.Warehouses) != 2 || preview.Warehouses[0].Items[0].Qty != 3 || preview.Warehouses[1].Items[0].Qty != 1 {
		t.Fatalf("preview = %#v", preview)
	}
}

func newMiniDirectShipTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := newCustomerFulfillmentTestDB(t)
	ctx := context.Background()
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock.EnsureSchema: %v", err)
	}
	if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("production.EnsureSchema: %v", err)
	}
	if err := postgressales.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("sales.EnsureSchema: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.order_audit_logs(
			id BIGSERIAL PRIMARY KEY,order_id BIGINT NOT NULL,actor TEXT NOT NULL DEFAULT '',
			field TEXT NOT NULL DEFAULT '',old_value TEXT NULL,new_value TEXT NULL,
			changed_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("order audit test schema: %v", err)
	}
	return pool, schema
}

func seedMiniDirectShipStock(t *testing.T, pool *pgxpool.Pool, schema string) {
	t.Helper()
	ctx := context.Background()
	q := fmt.Sprintf(`
		INSERT INTO %[1]s.customers(id,name,active) VALUES
			(501,'DEV-E2E 客户A',true),(502,'DEV-E2E 客户B',true);
		INSERT INTO %[1]s.products(id,name,sku_name,sku_code,parent_product_id,spec_label,net_content_qty,net_content_unit,product_kind,active)
		VALUES
			(910,'DEV-E2E 萨其姆','','',0,'',0,'','roasted_bean',true),
			(911,'DEV-E2E 萨其姆 1kg','萨其姆 1kg','DEV-E2E-911',910,'1kg',1000,'g','roasted_bean',true);
		INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,active,customer_id) VALUES
			('DEV-E2E-A1','客户A成品仓1','finished',1,true,501),
			('DEV-E2E-A2','客户A成品仓2','finished',2,true,501),
			('DEV-E2E-B1','客户B成品仓','finished',3,true,502)
		ON CONFLICT (code) DO UPDATE SET customer_id=excluded.customer_id,kind=excluded.kind,active=true;
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES
			(911,1000,'DEV-E2E-A1',1,1000),(911,1000,'DEV-E2E-A2',2,0),(911,1000,'DEV-E2E-B1',100,0)
		ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %[1]s.produce_running_items(id,batch_id,product_id,product_name,spec_g,need_g,status,started_at,finished_at) VALUES
			(2001,'DEV-E2E-P1',911,'DEV-E2E 萨其姆 1kg',1000,2000,'done','2026-08-01 08:00+08','2026-08-01 10:00+08'),
			(2002,'DEV-E2E-P2',911,'DEV-E2E 萨其姆 1kg',1000,2000,'done','2026-08-02 08:00+08','2026-08-02 10:00+08'),
			(2003,'DEV-E2E-P3',911,'DEV-E2E 萨其姆 1kg',1000,100000,'done','2026-08-03 08:00+08','2026-08-03 10:00+08');
		INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,created_at) VALUES
			(1001,'DEV-E2E-FP-A1','finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'production_run',2001,2000,2,2000,2,'passed','2026-08-01 10:00+08'),
			(1002,'DEV-E2E-FP-A2','finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'production_run',2002,2000,2,2000,2,'passed','2026-08-02 10:00+08'),
			(1003,'DEV-E2E-FP-B1','finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'production_run',2003,100000,100,100000,100,'passed','2026-08-03 10:00+08');
		INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units,created_at) VALUES
			('finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'DEV-E2E-A1','production_run',2001,'DEV-E2E-FP-A1',2000,2000,2,2,'2026-08-01 10:00+08'),
			('finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'DEV-E2E-A2','production_run',2002,'DEV-E2E-FP-A2',2000,2000,2,2,'2026-08-02 10:00+08'),
			('finished_product',911,'DEV-E2E 萨其姆 1kg',1000,'DEV-E2E-B1','production_run',2003,'DEV-E2E-FP-B1',100000,100000,100,100,'2026-08-03 10:00+08');
	`, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		t.Fatalf("seed direct ship stock: %v", err)
	}
}

func assertMiniDirectShipCount(t *testing.T, pool *pgxpool.Pool, schema, table, where string, want int64) {
	t.Helper()
	var got int64
	if err := pool.QueryRow(context.Background(), fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE %s", schema, table, where)).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s WHERE %s count=%d want=%d", table, where, got, want)
	}
}

func assertMiniCatalogAvailableQty(t *testing.T, catalog app.MiniDirectShipCatalog, productID, specG, want int64) {
	t.Helper()
	for _, family := range catalog.ProductFamilies {
		specs, ok := family["specs"].([]map[string]any)
		if !ok {
			continue
		}
		for _, spec := range specs {
			if spec["product_id"] == productID && spec["net_content_qty"] == float64(specG) {
				if spec["available_qty"] != want {
					t.Fatalf("catalog available_qty=%#v want %d in %#v", spec["available_qty"], want, catalog)
				}
				return
			}
		}
	}
	t.Fatalf("catalog product/spec %d/%d not found in %#v", productID, specG, catalog)
}
