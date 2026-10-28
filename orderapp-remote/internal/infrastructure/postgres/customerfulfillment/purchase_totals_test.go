package customerfulfillment

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/customerfulfillment"
	customerstock "orderapp/internal/infrastructure/postgres/customerstock"
)

func TestPurchaseTotalsUseAcceptedOrdersAndReceiptsInBoundWarehouse(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES
(74401,'PR744-A',501,'customer_stock','DEV-E2E-A1'),(74402,'PR744-B',501,'customer_stock','DEV-E2E-A2'),(74403,'PR744-OTHER',502,'customer_stock','DEV-E2E-B1');
INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total) VALUES
(74401,1,911,'测试','1000g',20,'袋',30,600),(74402,1,911,'测试','1000g',10,'袋',30,300),(74403,1,911,'测试','1000g',99,'袋',30,2970);
UPDATE %[1]s.customer_stock_entitlements SET received_qty=8 WHERE source_order_id=74401;
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	totals, err := customerstock.ListPurchaseTotals(ctx, pool, schema, 501, "DEV-E2E-A1")
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].PurchasedQty != 20 || totals[0].ReceivedQty != 8 || totals[0].ProductID != 911 || !totals[0].HistoryComplete {
		t.Fatalf("totals=%+v", totals)
	}
	// A legacy retained order without a linked entitlement means a numeric
	// cumulative total would be incomplete and must not be presented as exact.
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES (74404,'PR744-LEGACY',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total) VALUES (74404,1,911,'历史','1000g',2,'袋',30,60);
 DELETE FROM %[1]s.customer_stock_entitlements WHERE source_order_id=74404;`, schema))
	if err != nil {
		t.Fatal(err)
	}
	totals, err = customerstock.ListPurchaseTotals(ctx, pool, schema, 501, "DEV-E2E-A1")
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].HistoryComplete {
		t.Fatalf("missing historical source must not appear complete: %+v", totals)
	}

}

func TestStockCatalogKeepsCumulativePurchaseAfterStockIsShipped(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,delivery_mode,delivery_warehouse_code) VALUES(74410,'PR744-CATALOG',501,'customer_stock','DEV-E2E-A1');
 INSERT INTO %[1]s.order_items(order_id,line_no,product_id,bom_spec_id,bom_variant_id,item_name,spec,qty,unit,unit_price,line_total) VALUES(74410,1,910,7031,7032,'454g','454g',20,'袋',30,600);
 UPDATE %[1]s.customer_stock_entitlements SET received_qty=20 WHERE source_order_id=74410;`, schema))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := NewRepository(pool, schema).MiniDirectShipCatalog(ctx, app.MiniDirectShipCatalogQuery{CustomerID: 501, FulfillmentMode: "stock"})
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range catalog.ProductFamilies {
		for _, spec := range family["specs"].([]map[string]any) {
			if spec["product_id"] == int64(910) && spec["bom_spec_id"] == int64(7031) {
				p, ok := spec["purchased_qty"].(*int64)
				r, ok2 := spec["received_qty"].(*int64)
				if !ok || !ok2 || *p != 20 || *r != 20 {
					t.Fatalf("cumulative data=%+v", spec)
				}
				return
			}
		}
	}
	t.Fatalf("shipped product missing from stock catalog: %+v", catalog)
}
