package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
)

func TestInventoryOrderSourcesRespectWarehouseAndRefreshPayment(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
        UPDATE %[1]s.warehouses SET is_virtual=(code='DEV-E2E-A1');
        INSERT INTO %[1]s.orders(id,order_no,customer_id,pay_status_id,delivery_mode,delivery_warehouse_code)
        VALUES(71220,'SOURCE-PAID',501,(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1),'customer_stock','DEV-E2E-A1'),
              (71221,'SOURCE-UNPAID',501,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),'customer_stock','DEV-E2E-A1'),
              (71222,'SOURCE-OTHER-WAREHOUSE',501,(SELECT id FROM %[1]s.pay_statuses WHERE name='未付款' LIMIT 1),'customer_stock','DEV-E2E-A2');
        INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,qty,unit,unit_price,line_total,bom_spec_id,bom_variant_id)
        VALUES(71220,1,910,'咖啡',10,'袋',30,300,7031,7032),
              (71221,1,910,'咖啡',20,'袋',30,600,7031,7032),
              (71222,1,910,'咖啡',30,'袋',30,900,7031,7032);
        UPDATE %[1]s.customer_stock_entitlements SET received_qty=4 WHERE source_order_id=71220;
    `, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	load := func() app.CustomerAssetInventory {
		t.Helper()
		rows, err := repo.ListCustomerAssetInventory(ctx, app.CustomerAssetInventoryQuery{CustomerID: 501, InventoryType: "finished_product"})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("inventory rows=%+v", rows)
		}
		return rows[0]
	}
	row := load()
	if len(row.OrderSources) != 2 {
		t.Fatalf("authorized sources=%+v", row.OrderSources)
	}
	if row.OrderSources[0].OrderNo != "SOURCE-PAID" || row.OrderSources[0].PayStatus != "已付款" || row.OrderSources[0].PendingInboundQty != 6 || row.OrderSources[0].ReceivedQty != 4 {
		t.Fatalf("paid source=%+v", row.OrderSources[0])
	}
	if row.OrderSources[1].OrderNo != "SOURCE-UNPAID" || row.OrderSources[1].PayStatus != "未付款" || row.OrderSources[1].PendingInboundQty != 20 {
		t.Fatalf("unpaid source=%+v", row.OrderSources[1])
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.orders SET pay_status_id=(SELECT id FROM %[1]s.pay_statuses WHERE name='已付款' LIMIT 1) WHERE id=71221`, schema)); err != nil {
		t.Fatal(err)
	}
	refreshed := load()
	if refreshed.OrderSources[1].PayStatus != "已付款" || refreshed.TotalQty != row.TotalQty || refreshed.PendingInboundQty != row.PendingInboundQty {
		t.Fatalf("refresh changed stock or missed payment: before=%+v after=%+v", row, refreshed)
	}
}
