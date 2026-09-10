package production

import (
	"context"
	"fmt"
	"testing"
)

func TestOrderCompletionWaitsForEveryWorkOrderAndCanonicalQuantityPostgres(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.orders(id bigint,order_no text,is_void boolean,process_status_id bigint);
 CREATE TABLE %[1]s.order_process_statuses(id bigint,name text);
 CREATE TABLE %[1]s.order_items(order_id bigint,product_id bigint,bom_spec_id bigint,spec text,qty numeric);
 CREATE TABLE %[1]s.work_orders(order_nos text,status text);
 CREATE TABLE %[1]s.produce_running_items(order_nos text,status text);
 CREATE TABLE %[1]s.order_stock_decisions(order_id bigint,decision text);
 CREATE TABLE %[1]s.production_logs(order_nos text,product_id bigint,bom_spec_id bigint,spec_g bigint,finished_units bigint,finished_total_g bigint);
 CREATE TABLE %[1]s.order_stock_batch_allocations(order_id bigint,product_id bigint,bom_spec_id bigint,spec_g bigint,batch_code text,allocated_units bigint,allocated_g bigint);
 CREATE TABLE %[1]s.order_stock_deductions(order_id bigint,product_id bigint,spec_g bigint,batch_code text);
 CREATE TABLE %[1]s.finished_inventory(product_id bigint,bom_spec_id bigint,spec_g bigint,warehouse text,onhand_units bigint,onhand_loose_g bigint);
 INSERT INTO %[1]s.orders VALUES(1,'CONF-1',false,1);
 INSERT INTO %[1]s.order_process_statuses VALUES(1,'生产中'),(2,'生产完成');
 INSERT INTO %[1]s.order_items VALUES(1,7,9,'袋',2);
 INSERT INTO %[1]s.work_orders VALUES('CONF-1','completed'),('CONF-1','draft');
 INSERT INTO %[1]s.production_logs VALUES('CONF-1',7,9,0,1,0);
 `, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, changed, err := completeOrderIfAllRunningDone(ctx, tx, schema, "CONF-1"); err != nil || changed {
		t.Fatalf("unfinished sibling completed order: changed=%v err=%v", changed, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.work_orders SET status='completed'", schema)); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := completeOrderIfAllRunningDone(ctx, tx, schema, "CONF-1"); err != nil || changed {
		t.Fatalf("one of two bags completed order: changed=%v err=%v", changed, err)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("UPDATE %s.production_logs SET finished_units=2", schema)); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := completeOrderIfAllRunningDone(ctx, tx, schema, "CONF-1"); err != nil || !changed {
		t.Fatalf("complete order not reflected: changed=%v err=%v", changed, err)
	}
}
