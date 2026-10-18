package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	productionapp "orderapp/internal/application/production"
	salesapp "orderapp/internal/application/sales"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
	manufacturing "orderapp/internal/infrastructure/postgres/manufacturing"
	postproduction "orderapp/internal/infrastructure/postgres/production"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"strings"
	"testing"
	"time"
)

func TestPR703AcceptedRetainedSaleCreatesOnlyPendingInventory(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES(70301,'PR703-SOURCE',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total) VALUES(70301,1,911,'寄存24包','1000g',24,'袋',30,720);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	var promised, received int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT promised_qty,received_qty FROM %s.customer_stock_entitlements WHERE source_order_id=70301`, schema)).Scan(&promised, &received); err != nil {
		t.Fatal(err)
	}
	if promised != 24 || received != 0 {
		t.Fatalf("promised=%d received=%d, want 24/0", promised, received)
	}
	var actual int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(onhand_units),0) FROM %s.finished_inventory WHERE product_id=911 AND warehouse='DEV-E2E-A1'`, schema)).Scan(&actual); err != nil {
		t.Fatal(err)
	}
	if actual != 1 {
		t.Fatalf("virtual inventory changed physical inventory: %d", actual)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending' WHERE id=70301`, schema)); err != nil {
		t.Fatal(err)
	}
	var active int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.customer_stock_entitlements WHERE source_order_id=70301 AND status='active'`, schema)).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("pending confirmation exposed %d entitlements", active)
	}
}

func seedPR703StockProduct(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	if _, e := pool.Exec(ctx, fmt.Sprintf(`
 UPDATE %[1]s.finished_inventory SET onhand_units=0,onhand_loose_g=0;
 UPDATE %[1]s.stock_batches SET remaining_units=0,remaining_g=0;
 INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'direct_ship',true);
 INSERT INTO %[1]s.customer_fulfillment_settings(customer_id,modes_json)VALUES(501,'[{"mode":"stock","enabled":true,"warehouse_code":"DEV-E2E-A1","fee_configured":true}]');
 INSERT INTO %[1]s.company_employees(id,name,phone,department_id,active)VALUES(703,'销售员','13900000703',(SELECT id FROM %[1]s.company_departments ORDER BY id LIMIT 1),true);
 INSERT INTO %[1]s.sources(name) VALUES('PR703客户');
 INSERT INTO %[1]s.order_types(name)VALUES('PR703销售');
 UPDATE %[1]s.customers SET responsible_employee_id=703,default_source_id=(SELECT id FROM %[1]s.sources ORDER BY id LIMIT 1),default_order_type_id=(SELECT id FROM %[1]s.order_types ORDER BY id LIMIT 1) WHERE id=501;
 INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status)VALUES(7030,'PR703-BOM','寄存咖啡BOM','product',910,'active');
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7030,7030,'V1','published',1,'袋');
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(7031,7030,'PR703-454','454g','454g袋','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(7032,7030,7031,'454g袋','袋',true);
 INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)VALUES('product',910,7030,7030,true);
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units)VALUES(910,7031,7032,0,'DEV-E2E-A1',501,0);
 UPDATE %[1]s.products SET unit_rule_override_json='{"default_sales_unit":"bag","inventory_unit":"bag","unit_conversion_json":{"bag":{"bag":1}}}' WHERE id=910;
 `, schema)); e != nil {
		t.Fatal(e)
	}
	if e := manufacturing.EnsureSchema(ctx, pool, schema); e != nil {
		t.Fatal(e)
	}
	if e := migration.EnsureAuthorityView(ctx, pool, schema); e != nil {
		t.Fatal(e)
	}
	return pool, schema
}

func TestPR703RetainedSaleReservesConvertsAndBlocksShipment(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	price := 30.0
	productID := int64(910)
	badUnit := salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "箱不能静默变袋", Units: 2, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "box", ManualPrice: &price}}}
	if _, err := postgressales.NewRepository(pool, schema).SaveOrder(ctx, badUnit); err == nil || !strings.Contains(err.Error(), "库存单位") {
		t.Fatalf("retained box quantity silently became bags: %v", err)
	}

	sale, e := postgressales.NewRepository(pool, schema).SaveOrder(ctx, salesapp.SaveOrderCommand{CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-A1", Actor: "employee:703", Items: []salesapp.OrderItemCommand{{ProductID: &productID, Name: "寄存咖啡", Units: 24, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}})
	if e != nil {
		t.Fatal(e)
	}
	assertDemand := func(expected float64) {
		t.Helper()
		data, e := postproduction.NewRepository(pool, schema).PlanSummary(ctx, productionapp.PlanSummaryQuery{CustomerID: 501})
		if e != nil {
			t.Fatal(e)
		}
		got := float64(0)
		for _, row := range data.Rows {
			if row.ProductID == 910 {
				got += row.SalesSpecCount
			}
		}
		if got != expected {
			t.Fatalf("production demand=%v want %v rows=%+v", got, expected, data.Rows)
		}
	}
	assertDemand(24)
	repo := NewRepository(pool, schema)
	assets, e := repo.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if e != nil {
		t.Fatal(e)
	}
	if len(assets) != 1 || assets[0].TotalQty != 24 || assets[0].PendingInboundQty != 24 || assets[0].PhysicalAvailableQty != 0 || len(assets[0].Batches) != 0 {
		t.Fatalf("virtual assets=%+v", assets)
	}
	configurePR703StockFees(t, pool, schema, 7031)
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	cmd := app.MiniDirectShipCommand{CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "PR703-RESERVE-12", RecipientName: "测试收件人", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 12}}}
	preview, e := svc.PreviewMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	if !preview.CanSubmit || preview.StockReady || preview.TotalAmount != 0 || preview.Fees == nil || preview.Fees.NetWeightG != 5448 || preview.Fees.ServiceFeeCents != 900 || preview.Fees.ShippingFeeCents != 1800 || preview.PriceQuoteToken == "" || preview.FeeQuoteToken == "" {
		t.Fatalf("real service pending preview=%+v", preview)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, "tampered-fee-token"
	if _, e = svc.SubmitMiniDirectShip(ctx, cmd); e == nil {
		t.Fatal("tampered fee quote token submitted")
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_direct_ship_requests", "customer_id=501", 0)
	cmd.FeeQuoteToken = preview.FeeQuoteToken
	request, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	replay, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil || replay.ID != request.ID {
		t.Fatalf("real service replay=%+v err=%v", replay, e)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_fee_items", "customer_id=501 AND fee_type='direct_ship_service'", 1)
	order := app.DirectShipOrderSummary{OrderID: request.OrderID, OrderNo: request.OrderNo}
	assertDemand(24)
	if _, e = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), order.OrderID); e == nil {
		t.Fatal("virtual stock was allowed to ship")
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET is_void=true WHERE id=$1`, schema), sale.OrderID); e == nil {
		t.Fatal("source sale with promised output was voided")
	}
	receive := func(key string, units int64) {
		t.Helper()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		var batchID int64
		if err = tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)VALUES($1,'finished_product',910,'寄存咖啡',0,7031,7032,501,$2::bigint,$2::bigint,'passed')RETURNING id`, schema), key, units).Scan(&batchID); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,quality_status)VALUES($1,'finished_product',910,'同码其他版本',0,7031,999999,501,'passed')`, schema), key+"-other-variant"); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=onhand_units+$1,owner_customer_id=501 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'`, schema), units); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)VALUES('finished_product',910,'寄存咖啡',501,0,7031,7032,'DEV-E2E-A1','production_run',703,$1,$2::bigint,(SELECT onhand_units FROM %s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'))`, schema, schema), key, units); err != nil {
			t.Fatal(err)
		}
		// A production run of another frozen variant cannot satisfy this source sale.
		if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 703, 910, 7031, 999999, 0, units, key+"-wrong-variant", key, "DEV-E2E-A1", sale.OrderNo, "employee:703"); err == nil {
			t.Fatal("another production variant fulfilled the frozen source sale")
		}
		if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 703, 910, 7031, 7032, 0, units, key+"-bad-batch", key+"-other-variant", "DEV-E2E-A1", sale.OrderNo, "employee:703"); err == nil {
			t.Fatal("batch of another production variant fulfilled frozen source")
		}
		for i := 0; i < 2; i++ {
			if err = customerstock.ReceiveProductionTx(ctx, tx, schema, 703, 910, 7031, 7032, 0, units, key, key, "DEV-E2E-A1", sale.OrderNo, "employee:703"); err != nil {
				t.Fatal(err)
			}
		}
		var wrongReceipts int
		if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.customer_stock_entitlement_receipts WHERE receipt_key=$1 AND stock_batch_id<>$2`, schema), key, batchID).Scan(&wrongReceipts); err != nil || wrongReceipts != 0 {
			t.Fatalf("receipt picked another variant batch: %d %v", wrongReceipts, err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	receive("RECEIPT-6", 6)
	if _, e = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), order.OrderID); e == nil {
		t.Fatal("6/12 receipt was allowed to ship")
	}
	receive("RECEIPT-12", 6)
	if _, e = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), order.OrderID); e != nil {
		t.Fatal(e)
	}
	var received, converted int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT received_qty FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), sale.OrderID).Scan(&received); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(converted_qty) FROM %s.customer_stock_supply_reservations WHERE request_id=$1`, schema), request.ID).Scan(&converted); e != nil {
		t.Fatal(e)
	}
	if received != 12 || converted != 12 {
		t.Fatalf("received/converted=%d/%d, want 12/12", received, converted)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`SELECT %s.customer_stock_require_shippable($1)`, schema), sale.OrderID); e == nil {
		t.Fatal("retained source sale could ship")
	}
	changed := cmd
	changed.Items = []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 13}}
	preview, e = repo.PreviewMiniDirectShip(ctx, changed)
	if e != nil {
		t.Fatal(e)
	}
	if preview.CanSubmit || !preview.NeedsTopup || preview.PurchaseGapQty != 1 {
		t.Fatalf("over-purchase preview=%+v", preview)
	}
	assertDemand(12)
	assets, e = repo.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if e != nil {
		t.Fatal(e)
	}
	if len(assets) != 1 || assets[0].TotalQty != 24 || assets[0].PendingInboundQty != 12 || assets[0].PhysicalAvailableQty != 0 || assets[0].OccupiedQty != 12 {
		t.Fatalf("converted assets doubled=%+v", assets)
	}
	if _, e = postgressales.NewRepository(pool, schema).FillOrderTracking(ctx, salesapp.FillOrderTrackingCommand{OrderID: order.OrderID, TrackingNo: "SF-PR703", Actor: "employee:703"}); e != nil {
		t.Fatal(e)
	}
	assertDemand(12)
	var physical int64
	if e = pool.QueryRow(ctx, fmt.Sprintf("SELECT onhand_units FROM %s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1'", schema)).Scan(&physical); e != nil {
		t.Fatal(e)
	}
	if physical != 0 {
		t.Fatalf("shipment did not deduct12 physical units: remaining %d", physical)
	}
	// Reserve the second half, then cancel after its receipt: stock stays real and can be reused.
	cmd.IdempotencyKey = "PR703-SECOND-12"
	preview, e = svc.PreviewMiniDirectShip(ctx, cmd)
	if e != nil || !preview.CanSubmit || preview.StockReady {
		t.Fatalf("second pending preview=%+v err=%v", preview, e)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	second, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	assertDemand(12)
	receive("RECEIPT-24", 12)
	assertDemand(0)
	var deliveryStatus string
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT delivery_status FROM %s.orders WHERE id=$1`, schema), sale.OrderID).Scan(&deliveryStatus); e != nil || deliveryStatus != "received" {
		t.Fatalf("source delivery=%s err=%v", deliveryStatus, e)
	}
	if _, e = svc.CancelMiniDirectShipRequest(ctx, 501, second.ID, "employee:703"); e != nil {
		t.Fatal(e)
	}
	var secondFee float64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(amount),0) FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1`, schema), second.OrderID).Scan(&secondFee); e != nil || secondFee != 0 {
		t.Fatalf("cancelled service fee=%v err=%v", secondFee, e)
	}
	assets, e = repo.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if e != nil || len(assets) != 1 || assets[0].TotalQty != 12 || assets[0].PhysicalAvailableQty != 12 || assets[0].PendingInboundQty != 0 {
		t.Fatalf("cancel after receipt assets=%+v err=%v", assets, e)
	}
	cmd.IdempotencyKey = "PR703-FINAL-12"
	preview, e = svc.PreviewMiniDirectShip(ctx, cmd)
	if e != nil || !preview.StockReady || !preview.CanSubmit {
		t.Fatalf("final physical preview=%+v err=%v", preview, e)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	final, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = postgressales.NewRepository(pool, schema).FillOrderTracking(ctx, salesapp.FillOrderTrackingCommand{OrderID: final.OrderID, TrackingNo: "SF-PR703-FINAL", Actor: "employee:703"}); e != nil {
		t.Fatal(e)
	}
	assertDemand(0)
	assets, e = repo.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if e != nil || len(assets) != 0 {
		t.Fatalf("24 fully shipped left assets=%+v err=%v", assets, e)
	}
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT received_qty FROM %s.customer_stock_entitlements WHERE source_order_id=$1`, schema), sale.OrderID).Scan(&received); e != nil || received != 24 {
		t.Fatalf("cumulative source receipt=%d err=%v", received, e)
	}
	t.Logf("real app.Service+sales.Service: source %s buys24; token tamper rejected; first12 reserved future -> receipt6 blocks shipping -> receipt12 ships; remaining12 future -> receipt24 cancels to physical12 -> resubmits and ships; production demand 24 -> 12 -> 0, final inventory0; source received24", sale.OrderNo)
}

func configurePR703StockFees(t *testing.T, pool *pgxpool.Pool, schema string, bomSpecID int64) {
	t.Helper()
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	tm, e := repo.SaveShippingFeeTemplate(ctx, app.SaveShippingFeeTemplateCommand{Actor: "employee:703", Template: app.ShippingFeeTemplate{Name: "测试运费", Active: true, PackagingWeightG: 200, Rules: []app.ShippingFeeRegionRule{{Province: "云南省", FirstWeightG: 1000, FirstFeeCents: 800, AdditionalWeightG: 1000, AdditionalFeeCents: 200}}}})
	if e != nil {
		t.Fatal(e)
	}
	settings, e := repo.LoadFulfillmentSettings(ctx, 501)
	if e != nil {
		t.Fatal(e)
	}
	_, e = repo.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Actor: "employee:703", ExpectedRevision: settings.Revision, Settings: app.FulfillmentSettings{CustomerID: 501, Modes: []app.FulfillmentModeSettings{{Mode: "stock", Enabled: true, WarehouseCode: "DEV-E2E-A1", FeeConfigured: true, FixedFeeCents: 300, PerPackageFeeCents: 50, ShippingTemplateID: tm.ID}}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.spec_conversion_groups(name,base_unit,members_json)VALUES('留厂测试净重','kg',$1::jsonb)`, schema), fmt.Sprintf(`[{"bom_spec_id":%d,"inventory_unit":"袋","base_quantity":"0.454"}]`, bomSpecID)); e != nil {
		t.Fatal(e)
	}
}

// The historical FIFO test used child-SKU/gram writes retired by PR-622. Keep
// its FIFO, concurrency, cancellation and shipment assertions on current BOM units.
func seedMiniDirectShipCanonicalStock(t *testing.T, pool *pgxpool.Pool, schema string) {
	t.Helper()
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, e := pool.Exec(ctx, fmt.Sprintf(`
 UPDATE %[1]s.products SET parent_product_id=0,base_product_id=0,sku_code='FIFO-PARENT-911',is_processing_product=true,name='萨其姆',sku_name='萨其姆' WHERE id=911;
 INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status)VALUES(9110,'FIFO-BOM','FIFO BOM','product',911,'active');
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(9110,9110,'V1','published',1,'袋');
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)VALUES(9111,9110,'DEV-E2E-911','1kg','1kg','袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(9112,9110,9111,'1kg','袋',true);
 INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)VALUES('product',911,9110,9110,true);
 UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+onhand_loose_g/1000,onhand_loose_g=0,spec_g=0,bom_spec_id=9111,bom_variant_id=9112 WHERE product_id=911;
 UPDATE %[1]s.stock_batches SET spec_g=0,bom_spec_id=9111,bom_variant_id=9112,qty_g=0,remaining_g=0 WHERE item_id=911 AND item_type='finished_product';
 UPDATE %[1]s.stock_ledger_entries SET spec_g=0,bom_spec_id=9111,bom_variant_id=9112,qty_change_g=0,qty_after_g=0 WHERE item_id=911 AND item_type='finished_product';
 UPDATE %[1]s.produce_running_items SET spec_g=0,bom_spec_id=9111,bom_variant_id=9112 WHERE product_id=911;
 `, schema))
	if e != nil {
		t.Fatal(e)
	}
	if e = migration.EnsureAuthorityView(ctx, pool, schema); e != nil {
		t.Fatal(e)
	}
}

func TestPR703ConfiguredModeCancellationAndSourceDependencyGuards(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipCanonicalStock(t, pool, schema)
	_, e := pool.Exec(ctx, fmt.Sprintf(`
 UPDATE %[1]s.finished_inventory SET onhand_units=0,onhand_loose_g=0;
 UPDATE %[1]s.stock_batches SET remaining_units=0,remaining_g=0;
 INSERT INTO %[1]s.customer_fulfillment_settings(customer_id,modes_json)VALUES(501,'[{"mode":"stock","enabled":true,"warehouse_code":"DEV-E2E-A1","fee_configured":true},{"mode":"on_demand","enabled":true}]');
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(70301,'SOURCE-CANCEL',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,spec,qty,unit,unit_price,line_total)VALUES(70301,911,9111,9112,'1kg',24,'袋',30,720);
 `, schema))
	if e != nil {
		t.Fatal(e)
	}
	repo := NewRepository(pool, schema)
	cmd := app.MiniDirectShipCommand{CustomerID: 501, IdempotencyKey: "CANCEL-PROMISE", RecipientName: "收件人", RecipientPhone: "13800000000", DetailAddress: "大理市", Items: []app.MiniDirectShipItemCommand{{ProductID: 911, BomSpecID: 9111, BomVariantID: 9112, Qty: 12}}}
	if _, e = repo.PreviewMiniDirectShip(ctx, cmd); e == nil {
		t.Fatal("configured customer bypassed explicit mode")
	}
	cmd.FulfillmentMode = "stock"
	fakePurchase := cmd
	fakePurchase.UsageCode = "product_order"
	if _, e = repo.PrepareMiniDirectShipOrder(ctx, fakePurchase); e == nil {
		t.Fatal("product order accepted stock free-goods mode")
	}
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = repo.submitExplicitDirectShipOrderTx(ctx, tx, app.SubmitCustomerDirectShipOrderCommand{CustomerID: 501, PortalServiceCode: "product_order", DeliveryMode: "customer_stock", DeliveryWarehouseCode: "DEV-E2E-B1"}); e == nil {
		t.Fatal("retained purchase accepted another customer warehouse")
	}
	order, e := repo.submitExplicitDirectShipOrderTx(ctx, tx, app.SubmitCustomerDirectShipOrderCommand{CustomerID: 501, FulfillmentMode: "stock", ReceiverName: cmd.RecipientName, ReceiverPhone: cmd.RecipientPhone, ReceiverAddress: cmd.DetailAddress, Items: []app.SubmitCustomerDirectShipOrderItem{{ProductID: 911, BomSpecID: 9111, BomVariantID: 9112, ProductName: "库存咖啡", QuantityUnits: 12}}})
	if e != nil {
		t.Fatal(e)
	}
	request, e := repo.recordMiniDirectShipOrderTx(ctx, tx, app.PreparedMiniDirectShipOrder{Command: cmd, RequestHash: "CANCEL-HASH"}, order)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = postgressales.NewRepository(pool, schema).Void(ctx, order.OrderID, "employee", "普通作废"); e == nil {
		t.Fatal("ordinary void bypassed reservation cancellation")
	}
	productID := int64(911)
	if _, e = postgressales.NewRepository(pool, schema).SaveOrder(ctx, salesapp.SaveOrderCommand{EditID: order.OrderID, CustomerID: 501, Items: []salesapp.OrderItemCommand{{ProductID: &productID, BomSpecID: 9111, BomVariantID: 9112, Units: 12}}}); e == nil {
		t.Fatal("ordinary save erased fulfillment mode")
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET qty=23 WHERE order_id=70301`, schema)); e == nil {
		t.Fatal("promised source quantity was edited")
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET unit_price=31 WHERE order_id=70301`, schema)); e != nil {
		t.Fatalf("non-identity source price update blocked: %v", e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes='正常状态更新' WHERE id=70301`, schema)); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.CancelMiniDirectShipRequest(ctx, 501, request.ID, "employee"); e != nil {
		t.Fatal(e)
	}
	if e = postgressales.NewRepository(pool, schema).Void(ctx, 70301, "employee", "取消未生产购货"); e != nil {
		t.Fatal(e)
	}
	var active int
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.customer_stock_entitlements WHERE source_order_id=70301 AND status='active'`, schema)).Scan(&active); e != nil {
		t.Fatal(e)
	}
	if active != 0 {
		t.Fatal("cancelled sale retained active virtual stock")
	}
}

func TestPR703LegacyEntitlementUsesFrozenOrderSpec(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	_, e := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(70351,'LEGACY-454',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total,price_source_json)VALUES
 (70351,1,911,'冻结454g','454g',12,'袋',30,360,'{}'),
 (70351,2,911,'冻结227g','454g',12,'袋',30,360,'{"effective_sales_spec":{"net_content_qty":227,"net_content_unit":"g"}}');
 UPDATE %[1]s.products SET net_content_qty=2000 WHERE id=911;
 UPDATE %[1]s.orders SET notes='更新其他信息' WHERE id=70351;
 `, schema))
	if e != nil {
		t.Fatal(e)
	}
	rows, e := pool.Query(ctx, fmt.Sprintf(`SELECT e.spec_g,e.promised_qty FROM %s.customer_stock_entitlements e JOIN %s.order_items i ON i.id=e.source_order_item_id WHERE e.source_order_id=70351 ORDER BY i.line_no`, schema, schema))
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for _, want := range []int64{454, 227} {
		if !rows.Next() {
			t.Fatal("missing source identity")
		}
		var g, qty int64
		if e = rows.Scan(&g, &qty); e != nil {
			t.Fatal(e)
		}
		if g != want || qty != 12 {
			t.Fatalf("legacy rights %dg qty%d want%dg qty12", g, qty, want)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("unexpected frozen rights", rows.Err())
	}
	t.Log("454g order stays454g despite product1000g→2000g; structured frozen227g snapshot wins over display454g")
}

func TestPR703FutureEntitlementConcurrentSubmitCannotOversubscribe(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipCanonicalStock(t, pool, schema)
	if _, e := pool.Exec(ctx, fmt.Sprintf(`
 UPDATE %[1]s.finished_inventory SET onhand_units=0,onhand_loose_g=0;
 UPDATE %[1]s.stock_batches SET remaining_units=0,remaining_g=0;
 INSERT INTO %[1]s.customer_service_capabilities(customer_id,capability_code,enabled)VALUES(501,'direct_ship',true);
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(70361,'CONCURRENT-12',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(70361,911,9111,9112,'未来库存','1kg',12,'袋',30,360);
 `, schema)); e != nil {
		t.Fatal(e)
	}
	configurePR703StockFees(t, pool, schema, 9111)
	repo := NewRepository(pool, schema)
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	cmds := make([]app.MiniDirectShipCommand, 2)
	for i := range cmds {
		cmds[i] = app.MiniDirectShipCommand{CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: fmt.Sprintf("CONCURRENT-FUTURE-%d", i), RecipientName: "测试收件人", RecipientPhone: "13800000000", Province: "云南省", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 911, BomSpecID: 9111, BomVariantID: 9112, Qty: 8}}}
		preview, e := svc.PreviewMiniDirectShip(ctx, cmds[i])
		if e != nil {
			t.Fatal(e)
		}
		if !preview.CanSubmit {
			t.Fatalf("pre-race preview=%+v", preview)
		}
		cmds[i].PriceQuoteToken, cmds[i].FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	}
	type outcome struct {
		request app.MiniDirectShipRequest
		err     error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	for i := range cmds {
		go func(cmd app.MiniDirectShipCommand) {
			<-start
			r, e := svc.SubmitMiniDirectShip(ctx, cmd)
			results <- outcome{r, e}
		}(cmds[i])
	}
	close(start)
	successes := 0
	var winner app.MiniDirectShipRequest
	for range cmds {
		r := <-results
		if r.err == nil {
			successes++
			winner = r.request
		} else {
			t.Logf("losing concurrent submit rejected: %v", r.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent submits=%d want1", successes)
	}
	var reserved int64
	if e := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_qty-converted_qty-released_qty),0) FROM %s.customer_stock_supply_reservations`, schema)).Scan(&reserved); e != nil || reserved != 8 {
		t.Fatalf("reserved=%d err=%v", reserved, e)
	}
	if _, e := svc.CancelMiniDirectShipRequest(ctx, 501, winner.ID, "employee:703"); e != nil {
		t.Fatal(e)
	}
	if e := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_qty-converted_qty-released_qty),0) FROM %s.customer_stock_supply_reservations`, schema)).Scan(&reserved); e != nil || reserved != 0 {
		t.Fatalf("cancelled reservation=%d err=%v", reserved, e)
	}
	t.Log("12 future rights; simultaneous8+8 yields one success and reserved8, cancel releases back to12; no free order or duplicate fee from losing transaction")
}

func TestPR703AllocationsKeepActualBatchAndSourceVariants(t *testing.T) {
	items := []app.MiniDirectShipItemCommand{{ProductID: 1, BomSpecID: 2, BomVariantID: 200, Qty: 2, IsProcessingProduct: true}}
	physical, future, gaps := planMiniDirectShipFulfillment(items, []miniStockCandidate{{ProductID: 1, BomSpecID: 2, BomVariantID: 100, AvailableQty: 1}}, []miniProcessingOutputCandidate{{EntitlementID: 7, ProductID: 1, BomSpecID: 2, BomVariantID: 99, AvailableQty: 1}})
	if len(gaps) != 0 || len(physical) != 1 || len(future) != 1 {
		t.Fatalf("plan %v %v %v", physical, future, gaps)
	}
	if physical[0].BomVariantID != 100 || future[0].BomVariantID != 99 {
		t.Fatalf("traceability lost actual frozen variants: physical=%+v future=%+v", physical, future)
	}
}

func TestPR703ArchivedFutureRightsRemainOrderableByStableSpec(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	if _, e := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(70371,'ARCHIVED-PROMISE',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(70371,910,7031,7032,'原版未来库存','454g',12,'袋',30,360);
 UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;
 INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)VALUES(7034,7033,7031,'454g袋','袋',true);
 UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_id=910;
 `, schema)); e != nil {
		t.Fatal(e)
	}
	configurePR703StockFees(t, pool, schema, 7031)
	repo := NewRepository(pool, schema)
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	catalog, e := svc.MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 501, FulfillmentMode: "stock"})
	if e != nil || len(catalog.ProductFamilies) != 1 {
		t.Fatalf("archived promise catalog=%+v err=%v", catalog, e)
	}
	cmd := app.MiniDirectShipCommand{CustomerID: 501, FulfillmentMode: "stock", IdempotencyKey: "ARCHIVED-PROMISE-12", RecipientName: "测试收件人", RecipientPhone: "13800000000", Province: "云南省", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 12}}}
	preview, e := svc.PreviewMiniDirectShip(ctx, cmd)
	if e != nil || !preview.CanSubmit {
		t.Fatalf("old promise cannot use stable spec after publication: preview=%+v err=%v", preview, e)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	request, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatal(e)
	}
	var variant int64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT e.bom_variant_id FROM %s.customer_stock_supply_reservations r JOIN %s.customer_stock_entitlements e ON e.id=r.entitlement_id WHERE r.request_id=$1`, schema, schema), request.ID).Scan(&variant); e != nil || variant != 7032 {
		t.Fatalf("source frozen version changed: %d %v", variant, e)
	}
	t.Log("V1 future rights survive V2 publication; current stable spec can reserve, source manufacturing remains frozen to V1")
}

func TestPR703OnDemandRealServiceUsesPublicFixedPriceAndCreatesSalesOrder(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	configurePR703StockFees(t, pool, schema, 7031)
	repo := NewRepository(pool, schema)
	settings, e := repo.LoadFulfillmentSettings(ctx, 501)
	if e != nil {
		t.Fatal(e)
	}
	mode := settings.Modes[0]
	mode.Mode = "on_demand"
	mode.WarehouseCode = ""
	settings.Modes = append(settings.Modes, mode)
	if _, e = repo.SaveFulfillmentSettings(ctx, app.SaveFulfillmentSettingsCommand{Settings: settings, ExpectedRevision: settings.Revision, Actor: "employee:703"}); e != nil {
		t.Fatal(e)
	}
	var publicationID int64
	content := `{"price_rows":[{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"source_price_record_id":703454,"min_qty":1,"final_unit_price":30,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count","effective_sales_spec":{"product_id":910,"parent_product_id":910,"bom_spec_id":7031,"bom_variant_id":7032,"net_content_qty":454,"net_content_unit":"g","sales_unit":"袋","inventory_unit":"袋"}}]}`
	if e = pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.bean_list_publications(list_type,version_no,status,owner_type,owner_key,publication_purpose,config_json,content_json,changelog,actor)VALUES('commercial','PR703-ON-DEMAND','published','official','','factory_supply','{}',$1::jsonb,'','test')RETURNING id`, schema), content).Scan(&publicationID); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_order_price_table_bindings(customer_id,usage_code,product_type_key,publication_id)VALUES(501,'direct_ship','commercial',$1)`, schema), publicationID); e != nil {
		t.Fatal(e)
	}
	svc := app.NewService(repo)
	svc.UseSalesOrderService(salesapp.NewService(postgressales.NewRepository(pool, schema)))
	cmd := app.MiniDirectShipCommand{CustomerID: 501, FulfillmentMode: "on_demand", IdempotencyKey: "REAL-ON-DEMAND", RecipientName: "测试收件人", RecipientPhone: "13800000000", Province: "云南省", City: "大理市", DetailAddress: "测试路1号", Actor: "employee:703", Items: []app.MiniDirectShipItemCommand{{ProductID: 910, BomSpecID: 7031, BomVariantID: 7032, Qty: 12}}}
	preview, e := svc.PreviewMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatalf("real on-demand preview: %v", e)
	}
	if !preview.CanSubmit || preview.TotalAmount != 360 || len(preview.Items) != 1 || preview.Items[0].UnitPrice != 30 || preview.Fees == nil || preview.Fees.ServiceFeeCents != 900 || preview.Fees.ShippingFeeCents != 1800 {
		t.Fatalf("real on-demand preview=%+v", preview)
	}
	cmd.PriceQuoteToken, cmd.FeeQuoteToken = preview.PriceQuoteToken, preview.FeeQuoteToken
	request, e := svc.SubmitMiniDirectShip(ctx, cmd)
	if e != nil {
		t.Fatalf("real on-demand submit: %v", e)
	}
	var goods, shipping, fee, grand float64
	var modeName string
	var qty int64
	var linePrice float64
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT total_amount,shipping_amount,grand_total,fulfillment_mode FROM %s.orders WHERE id=$1`, schema), request.OrderID).Scan(&goods, &shipping, &grand, &modeName); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty,unit_price FROM %s.order_items WHERE order_id=$1`, schema), request.OrderID).Scan(&qty, &linePrice); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(amount) FROM %s.customer_fee_items WHERE source_type='order' AND source_id=$1 AND fee_type='direct_ship_service'`, schema), request.OrderID).Scan(&fee); e != nil {
		t.Fatal(e)
	}
	if goods != 360 || shipping != 18 || grand != 378 || fee != 9 || modeName != "on_demand" || qty != 12 || linePrice != 30 {
		t.Fatalf("on-demand persisted goods=%v freight=%v grand=%v service=%v mode=%s qty%d price%v", goods, shipping, grand, fee, modeName, qty, linePrice)
	}
	assertMiniDirectShipCount(t, pool, schema, "customer_stock_supply_reservations", "customer_id=501", 0)
	assertMiniDirectShipCount(t, pool, schema, "customer_direct_ship_request_allocations", "request_id="+fmt.Sprint(request.ID), 0)
	t.Logf("real app.Service->sales.SaveOrder %s: public fixed30 x12 =360, service9 + freight18, goods/pricing snapshots frozen, no stock entitlement consumption", request.OrderNo)
}

func TestPR703InternalOrderListIncludesRetainedSaleAndDropshipWithoutCustomerLogin(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.customer_portal_profiles(customer_id,enabled,capability_template_key)VALUES(501,true,'public_sku_direct_ship') ON CONFLICT(customer_id)DO UPDATE SET enabled=true;
 INSERT INTO %[1]s.orders(id,customer_id,order_no,delivery_mode,delivery_warehouse_code,grand_total,total_amount)VALUES(703501,501,'PR703-RETAINED-LIST','customer_stock','DEV-E2E-A1',720,720);
 INSERT INTO %[1]s.orders(id,customer_id,order_no,portal_service_code,fulfillment_mode,grand_total,shipping_amount)VALUES(703502,501,'PR703-DROPSHIP-LIST','direct_ship','stock',18,18);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	result, err := postgressales.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{Scope: "fulfillment", CustomerID: 501, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := map[int64]bool{}
	for _, row := range result.Rows {
		found[row.ID] = true
	}
	if !found[703501] || !found[703502] {
		t.Fatalf("internal fulfillment hides retained sale/dropship without external login: %+v", found)
	}
}
