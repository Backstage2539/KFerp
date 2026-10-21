package customerportal

import (
	"context"
	"fmt"
	stockpg "orderapp/internal/infrastructure/postgres/stock"
	"testing"
)

func TestMiniInventoryLegacyServicePageRequiresCurrentBoundVirtualWarehouse(t *testing.T) {
	pool, schema := newCustomerPortalTestDB(t)
	ctx := context.Background()
	if err := stockpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.customers(id,name)VALUES(77,'库存权限客户A'),(78,'库存权限客户B');
INSERT INTO %[1]s.warehouses(code,name,kind,customer_id,is_virtual,active)VALUES('own','授权客户仓','finished',77,false,true),('other','其他客户仓','finished',77,false,true),('unbound','未绑定虚拟仓','finished',77,false,true),('physical','非虚拟仓','finished',77,false,true),('disabled','停用仓','finished',77,false,true);
INSERT INTO %[1]s.customer_inventory_items(customer_id,item_type,item_id,item_name,warehouse,qty_units)VALUES(77,'finished_product',101,'授权商品','own',1),(77,'finished_product',102,'其他仓历史商品','other',1),(77,'finished_product',103,'未绑定仓历史商品','unbound',1),(77,'finished_product',104,'非虚拟仓历史商品','physical',1),(77,'finished_product',105,'停用仓历史商品','disabled',1);
UPDATE %[1]s.warehouses SET is_virtual=true WHERE code IN('own','other','unbound','disabled');
UPDATE %[1]s.warehouses SET customer_id=78 WHERE code='other';UPDATE %[1]s.warehouses SET customer_id=0 WHERE code='unbound';UPDATE %[1]s.warehouses SET active=false WHERE code='disabled';
`, schema)); err != nil {
		t.Fatal(err)
	}
	rows, err := NewRepository(pool, schema).listInventory(ctx, 77, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Warehouse != "own" {
		t.Fatalf("compatibility inventory API leaked ungranted warehouse: %+v", rows)
	}
}
