package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	app "orderapp/internal/application/customerfulfillment"
	"reflect"
	"testing"
)

func inventoryScopeFixture(t *testing.T) (*pgxpool.Pool, string, *Repository) {
	t.Helper()
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`UPDATE %s.warehouses SET is_virtual=false`, schema)); err != nil {
		t.Fatal(err)
	}
	return pool, schema, NewRepository(pool, schema)
}

func TestMiniInventoryScopeEmptyBoundWarehouseKeepsFinishedCategory(t *testing.T) {
	pool, schema, r := inventoryScopeFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,is_virtual,customer_id,active)VALUES('own-empty','客户空成品仓','finished',true,501,true),('other','其他客户仓','raw',true,502,true),('unbound','未绑定仓','packaging',true,0,true),('inactive','停用仓','raw',true,501,false)`, schema)); err != nil {
		t.Fatal(err)
	}
	scope, err := r.CustomerAssetInventoryScope(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope.InventoryTypes, []string{"finished_product"}) || len(scope.Warehouses) != 1 || scope.Warehouses[0].WarehouseCode != "own-empty" {
		t.Fatalf("empty warehouse scope=%+v", scope)
	}
	rows, err := r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501})
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty authorized warehouse exposed other stock: %+v err=%v", rows, err)
	}
	for _, key := range []string{"green_bean", "packaging", "semi_finished"} {
		rows, err := r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: key})
		if err != nil || len(rows) != 0 {
			t.Fatalf("unauthorized category %s returned %+v err=%v", key, rows, err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET active=false WHERE code='own-empty'`, schema)); err != nil {
		t.Fatal(err)
	}
	scope, err = r.CustomerAssetInventoryScope(ctx, 501)
	if err != nil || len(scope.InventoryTypes) != 0 || len(scope.Warehouses) != 0 {
		t.Fatalf("no-grant scope=%+v err=%v", scope, err)
	}
	rows, err = r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501})
	if err != nil || len(rows) != 0 {
		t.Fatalf("no-grant inventory=%+v err=%v", rows, err)
	}
}

func TestMiniInventoryFinishedRowsBatchesAndLedgerUseOnlyBoundVirtualWarehouses(t *testing.T) {
	pool, schema, r := inventoryScopeFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET is_virtual=true WHERE code='DEV-E2E-A1'`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct {
		code            string
		owner           int64
		virtual, active bool
	}{{"public-stock", 0, false, true}, {"unbound-stock", 0, true, true}, {"other-stock", 502, true, true}, {"inactive-stock", 501, true, false}} {
		// Seed dirty historical ownership while valid, then emulate an old rebind.
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,customer_id,is_virtual,active)VALUES($1,$1,'finished',501,false,true)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,owner_customer_id,onhand_units)VALUES(911,1000,$1,501,10)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,spec_g,warehouse,owner_customer_id,qty_change_units,qty_after_units)VALUES('finished_product',911,1000,$1,501,10,10)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET customer_id=$2,is_virtual=$3,active=$4 WHERE code=$1`, schema), w.code, w.owner, w.virtual, w.active); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TotalQty != 2 || len(rows[0].Warehouses) != 1 || rows[0].Warehouses[0].WarehouseCode != "DEV-E2E-A1" {
		t.Fatalf("finished scope rows=%+v", rows)
	}
	central, err := r.ListCustomerBoundVirtualInventory(ctx, 501)
	if err != nil || len(central) != 1 || central[0].TotalQty != 2 || len(central[0].Warehouses) != 1 {
		t.Fatalf("legacy summary=%+v err=%v", central, err)
	}
	batches, err := r.ListCustomerCentralInventoryBatches(ctx, app.CustomerInventoryBatchQuery{CustomerID: 501, ProductID: 911, SpecG: 1000, BoundVirtualOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range batches {
		if row.Warehouse != "客户A成品仓1" {
			t.Fatalf("detail exposed other warehouse: %+v", row)
		}
	}
	if len(batches) != 1 {
		t.Fatalf("authorized batch detail=%+v", batches)
	}
	ledger, err := r.ListCustomerAssetInventoryLedger(ctx, app.CustomerAssetInventoryLedgerQuery{CustomerID: 501, InventoryType: "finished_product", ItemID: 911, SpecG: 1000, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 1 || ledger[0].WarehouseCode != "DEV-E2E-A1" {
		t.Fatalf("ledger leaked ungranted warehouse: %+v", ledger)
	}
}

func TestMiniInventoryMaterialsRequireBoundWarehouseAndPreserveRealMaterialTypes(t *testing.T) {
	pool, schema, r := inventoryScopeFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.warehouses(code,name,kind,customer_id,is_virtual)VALUES('own-raw','客户原料虚拟仓','raw',501,true),('own-pack','客户包材虚拟仓','packaging',501,true),('public-raw','工厂原料仓','raw',0,false);
INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,owner_customer_id,is_semi_finished)VALUES(71201,'G','生豆','bean','kg','kg',501,false),(71202,'P','包材','packaging','件','件',501,false),(71203,'S','半成品','other','kg','kg',501,true);
INSERT INTO %[1]s.material_batches(id,batch_code,material_id,owner_customer_id,qty_g,remaining_g,quality_status)VALUES(71201,'G1',71201,501,10000,10000,'passed'),(71202,'P1',71202,501,0,0,'passed'),(71203,'S1',71203,501,2000,2000,'passed');
INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units)VALUES(71201,'G1',71201,'own-raw',3000,0),(71201,'G1',71201,'public-raw',7000,0),(71202,'P1',71202,'own-pack',0,4),(71203,'S1',71203,'own-raw',2000,0);
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,warehouse,owner_customer_id,qty_change_g)VALUES('material',71201,'own-raw',501,3000),('material',71201,'public-raw',501,7000);
`, schema)); err != nil {
		t.Fatal(err)
	}
	scope, err := r.CustomerAssetInventoryScope(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope.InventoryTypes, []string{"green_bean", "packaging", "semi_finished"}) {
		t.Fatalf("material type grants=%+v", scope)
	}
	rows, err := r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501})
	if err != nil {
		t.Fatal(err)
	}
	byType := map[string]app.CustomerAssetInventory{}
	for _, row := range rows {
		byType[row.InventoryType] = row
	}
	if len(rows) != 3 || byType["green_bean"].TotalQty != 3 || byType["packaging"].TotalQty != 4 || byType["semi_finished"].TotalQty != 2 {
		t.Fatalf("material warehouse totals=%+v", rows)
	}
	ledger, err := r.ListCustomerAssetInventoryLedger(ctx, app.CustomerAssetInventoryLedgerQuery{CustomerID: 501, InventoryType: "green_bean", ItemID: 71201, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger) != 1 || ledger[0].WarehouseCode != "own-raw" || ledger[0].QuantityDelta != 3 {
		t.Fatalf("material ledger exposed public warehouse=%+v", ledger)
	}
}

func TestMiniInventoryPendingRightsStayWithinAuthorizedWarehouseAndSeparateFromPhysical(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	r := NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET is_virtual=(code='DEV-E2E-A1')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code)VALUES(71210,'SCOPE-PENDING-OWN',501,'customer_stock','DEV-E2E-A1'),(71211,'SCOPE-PENDING-NOT-GRANTED',501,'customer_stock','DEV-E2E-A2');
INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,qty,unit,unit_price,line_total,bom_spec_id,bom_variant_id)VALUES(71210,1,910,'客户在途库存',24,'袋',30,720,7031,7032),(71211,1,910,'未授权仓在途库存',100,'袋',30,3000,7031,7032);
`, schema)); err != nil {
		t.Fatal(err)
	}
	assets, err := r.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].PendingInboundQty != 24 || assets[0].PhysicalAvailableQty != 0 || len(assets[0].Warehouses) != 1 || assets[0].Warehouses[0].WarehouseCode != "DEV-E2E-A1" {
		t.Fatalf("pending asset scope/physical separation=%+v", assets)
	}
	central, err := r.ListCustomerBoundVirtualInventory(ctx, 501)
	if err != nil {
		t.Fatal(err)
	}
	if len(central) != 1 || central[0].PendingInboundQty != 24 || central[0].PhysicalAvailableQty != 0 || central[0].TotalQty != 24 {
		t.Fatalf("legacy pending summary mixed warehouse rights=%+v", central)
	}
}
