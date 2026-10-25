package customerfulfillment

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	postgressales "orderapp/internal/infrastructure/postgres/sales"
	"testing"
	"time"
)

func TestPR729OrderSaveShipmentDeductsActualFinishedStock(t *testing.T) {
	pool, schema := seedPR703StockProduct(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,pay_status_id,ship_status_id,ship_method) VALUES(72901,'PR729-SHIP',501,current_date,1,1,'pickup');
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units) VALUES(910,7031,7032,0,'finished_goods',0,24);
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,qty_units,remaining_units,qty_g,remaining_g,unit_cost,quality_status) VALUES('PR729-FINISHED','finished_product',910,'验收咖啡',7031,7032,0,24,24,0,0,30,'passed');
 INSERT INTO %[1]s.ship_statuses(name) SELECT '已发货' WHERE NOT EXISTS(SELECT 1 FROM %[1]s.ship_statuses WHERE name='已发货');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	var shipped int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.ship_statuses WHERE name='已发货' LIMIT 1`, schema)).Scan(&shipped); err != nil {
		t.Fatal(err)
	}
	price, product := 30.0, int64(910)
	cmd := salesapp.SaveOrderCommand{EditID: 72901, CustomerID: 501, PayStatusID: 1, ShipStatusID: 1, ShipMethod: "pickup", OrderDate: time.Now(), DocumentDate: time.Now(), DeliveryMode: "ship", Actor: "PR729验收", StockBatchDecision: "use_batch", Items: []salesapp.OrderItemCommand{{ProductID: &product, Name: "验收咖啡", Units: 24, BomSpecID: 7031, BomVariantID: 7032, SalesUnit: "bag", ManualPrice: &price}}}
	repo := postgressales.NewRepository(pool, schema)
	order, err := repo.SaveOrder(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	cmd.EditID = order.OrderID
	cmd.ShipStatusID = shipped
	// A stale availability preview must not save a shipped state without stock.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=23 WHERE product_id=910; UPDATE %s.stock_batches SET remaining_units=23 WHERE batch_code='PR729-FINISHED'`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.SaveOrder(ctx, cmd); err == nil {
		t.Fatal("shipment with insufficient stock must fail")
	}
	var status int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT ship_status_id FROM %s.orders WHERE id=$1`, schema), order.OrderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != 1 {
		t.Fatalf("failed shipment changed shipping status: %d", status)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=24 WHERE product_id=910; UPDATE %s.stock_batches SET remaining_units=24 WHERE batch_code='PR729-FINISHED'`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.SaveOrder(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	var stock, remaining, deducted int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='finished_goods'`, schema)).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='PR729-FINISHED'`, schema)).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(deducted_units),0) FROM %s.order_stock_deductions WHERE order_id=$1`, schema), order.OrderID).Scan(&deducted); err != nil {
		t.Fatal(err)
	}
	if stock != 0 || remaining != 0 || deducted != 24 {
		t.Fatalf("shipped order must deduct inventory/batch and audit actual units: stock=%d batch=%d deducted=%d", stock, remaining, deducted)
	}
	// Editing the already shipped order must not deduct the same stock again.
	cmd.StockBatchDecision = ""
	cmd.Notes = "repeat save after response loss"
	if _, err = repo.SaveOrder(ctx, cmd); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(deducted_units),0) FROM %s.order_stock_deductions WHERE order_id=$1`, schema), order.OrderID).Scan(&deducted); err != nil {
		t.Fatal(err)
	}
	if deducted != 24 {
		t.Fatalf("repeat save deducted %d units", deducted)
	}

}
