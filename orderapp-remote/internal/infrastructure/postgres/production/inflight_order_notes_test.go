package production

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestInflightSupplyRequiresMatchingFrozenOrderNotes(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() }()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_supply_allocations(id bigint,production_plan_id bigint,production_plan_item_id bigint,supplier_work_order_id bigint);
 CREATE TABLE %[1]s.production_plan_item_dependencies(production_plan_id bigint,production_plan_item_id bigint,depends_on_plan_item_id bigint);
 CREATE TABLE %[1]s.production_plan_items(id bigint,demand_sources_json jsonb);
 CREATE TABLE %[1]s.work_orders(id bigint,work_order_no text,output_type text,output_material_id bigint,output_product_id bigint,product_id bigint,bom_spec_id bigint,bom_variant_id bigint,related_orders_snapshot_json jsonb);
 CREATE TABLE %[1]s.work_order_dependencies(work_order_id bigint,depends_on_work_order_id bigint);
 CREATE TABLE %[1]s.order_items(id bigint,order_id bigint,operation_notes_json jsonb);
 INSERT INTO %[1]s.production_supply_allocations VALUES(1,11,100,20);
 INSERT INTO %[1]s.production_plan_item_dependencies VALUES(11,101,100);
 INSERT INTO %[1]s.production_plan_items VALUES(100,'[]'),(101,'[{"order_id":2,"order_item_id":3,"order_no":"SO-2"}]');
	INSERT INTO %[1]s.work_orders VALUES(20,'WO-20','material',10,0,0,0,0,NULL),(21,'WO-21','material',11,0,0,0,0,NULL);
	INSERT INTO %[1]s.work_order_dependencies VALUES(20,21),(21,20);
 INSERT INTO %[1]s.order_items VALUES(3,2,'[]');`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = validateInflightOrderNotesTx(ctx, tx, schema, 11); err != nil {
		t.Fatalf("no notes must allow reuse: %v", err)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET operation_notes_json='[{"output_type":"material","output_id":10,"route_id":32,"step_id":7,"operation_id":8,"operation_name":"烘焙","note":"浅烘"}]' WHERE id=3`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = validateInflightOrderNotesTx(ctx, tx, schema, 11); err == nil || !strings.Contains(err.Error(), "新生产") {
		t.Fatalf("uncovered note must block: %v", err)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET related_orders_snapshot_json='[{"order_id":2,"order_item_id":3,"operation_notes":[{"output_type":"material","output_id":10,"route_id":32,"step_id":7,"operation_id":8,"operation_name":"烘焙","note":"浅烘"}]}]' WHERE id=20`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = validateInflightOrderNotesTx(ctx, tx, schema, 11); err != nil {
		t.Fatalf("matching frozen note must permit reuse: %v", err)
	}
	_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET operation_notes_json='[{"output_type":"material","output_id":11,"route_id":33,"step_id":17,"operation_id":18,"operation_name":"分选","note":"人工分选"}]' WHERE id=3`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if err = validateInflightOrderNotesTx(ctx, tx, schema, 11); err == nil || !strings.Contains(err.Error(), "WO-21") {
		t.Fatalf("uncovered indirect supplier must block: %v", err)
	}
}
