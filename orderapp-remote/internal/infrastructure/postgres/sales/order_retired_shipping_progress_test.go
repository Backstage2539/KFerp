package sales

import (
	"context"
	"fmt"
	"testing"
)

func TestRetiredLegacyItemDoesNotKeepLiveFulfillmentPartiallyShipped(t *testing.T) {
	ctx := context.Background()
	pool, schema := newSalesPostgresTestDB(t)
	t.Cleanup(func() { pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.orders(id bigint,confirmation_required boolean,confirmation_status text,ship_status_id bigint);
 CREATE TABLE %[1]s.ship_statuses(id bigint,name text);
 CREATE TABLE %[1]s.order_items(order_id bigint,product_id bigint,bom_spec_id bigint,qty numeric);
 CREATE TABLE %[1]s.order_stock_deductions(order_id bigint,product_id bigint,bom_spec_id bigint,deducted_units numeric);
 INSERT INTO %[1]s.orders VALUES(1,true,'accepted',1);
 INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'部分发货'),(3,'已发货');
 INSERT INTO %[1]s.order_items VALUES(1,10,20,2),(1,11,0,0);
 INSERT INTO %[1]s.order_stock_deductions VALUES(1,10,20,2);
 `, schema)); err != nil {
		t.Fatal(err)
	}
	if err := ensureOrderShipmentProgressSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var progress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s.fulfillment_shipment_progress(1)`, schema)).Scan(&progress); err != nil {
		t.Fatal(err)
	}
	if progress != "已发货" {
		t.Fatalf("zero-quantity legacy item kept live items partially shipped: %s", progress)
	}
}
