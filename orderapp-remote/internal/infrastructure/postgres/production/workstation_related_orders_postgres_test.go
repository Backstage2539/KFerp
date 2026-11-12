package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"testing"
)

func TestWorkstationRelatedOrdersUsesExactDemandSources(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() }()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.work_orders(id bigint primary key,production_plan_item_id bigint);
 CREATE TABLE %[1]s.work_order_dependencies(work_order_id bigint,depends_on_work_order_id bigint);
 CREATE TABLE %[1]s.production_plan_items(id bigint primary key,demand_sources_json jsonb);
 CREATE TABLE %[1]s.production_supply_allocations(supplier_work_order_id bigint,production_plan_item_id bigint,status text,work_order_id bigint);
 CREATE TABLE %[1]s.orders(id bigint primary key,notes text);
 CREATE TABLE %[1]s.order_items(id bigint primary key,order_id bigint,item_note text);
 INSERT INTO %[1]s.orders VALUES(83,'原订单要求'),(86,'另一个订单');
 INSERT INTO %[1]s.order_items VALUES(173,83,'原商品要求'),(185,86,'');
 INSERT INTO %[1]s.work_orders VALUES (1,10),(2,20),(3,30);
 INSERT INTO %[1]s.work_order_dependencies VALUES(2,1);
 INSERT INTO %[1]s.production_plan_items VALUES
 (10,'[]'),(20,'[{"order_id":83,"order_item_id":173,"customer_name":"A","quantity":50,"sales_unit":"袋"}]'),
 (30,'[{"order_id":86,"order_item_id":185,"customer_name":"B","quantity":10,"sales_unit":"袋"}]');
 INSERT INTO %[1]s.production_supply_allocations VALUES (1,30,'reserved',3),(1,20,'reserved',2);`, schema))
	repo := NewRepository(pool, schema)
	orders, err := repo.workstationRelatedOrders(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Fatalf("want two distinct actual order lines, got %+v", orders)
	}
	seen := map[int64]bool{}
	for _, o := range orders {
		seen[o.OrderItemID] = true
		if o.OrderItemID == 173 && (o.OriginalOrderNote != "原订单要求" || o.OriginalItemNote != "原商品要求") {
			t.Fatalf("missing original notes: %+v", o)
		}
	}
	if !seen[173] || !seen[185] {
		t.Fatalf("wrong orders %+v", orders)
	}
	late := mergeLateSupplyOrders([]productionapp.ProductionDemandOrder{}, orders)
	if len(late) != 2 || !late[0].LateAssociation || !late[1].LateAssociation || late[0].OriginalOrderNote != "" || late[1].OriginalItemNote != "" {
		t.Fatalf("empty frozen stock snapshot must show only later association metadata: %+v", late)
	}
}

func TestFreezeWorkOrderNotesKeepsExactOrderOperationSnapshot(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() }()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.work_orders(id bigint primary key,production_plan_item_id bigint,related_orders_snapshot_json jsonb);
 CREATE TABLE %[1]s.work_order_dependencies(work_order_id bigint,depends_on_work_order_id bigint);
 CREATE TABLE %[1]s.production_plan_items(id bigint primary key,demand_sources_json jsonb);
 CREATE TABLE %[1]s.production_supply_allocations(supplier_work_order_id bigint,production_plan_item_id bigint);
 CREATE TABLE %[1]s.orders(id bigint primary key,notes text);
 CREATE TABLE %[1]s.order_items(id bigint primary key,order_id bigint,item_note text,operation_notes_json jsonb);
 CREATE TABLE %[1]s.process_route_operations(id bigint,route_id bigint,operation_id bigint,seq int,operation text);
 CREATE TABLE %[1]s.job_cards(work_order_id bigint,sequence_no int,operation_id bigint);
 INSERT INTO %[1]s.work_orders VALUES (1,10,NULL);
 INSERT INTO %[1]s.production_plan_items VALUES (10,'[{"order_id":83,"order_item_id":173,"customer_name":"A","quantity":50}]');
 INSERT INTO %[1]s.orders VALUES(83,'原备注');
 INSERT INTO %[1]s.order_items VALUES(173,83,'','[{"output_type":"product","output_id":9,"route_id":3,"step_id":7,"operation_id":8,"operation_name":"包装","note":"贴标签"},{"output_type":"product","output_id":9,"bom_spec_id":2,"bom_variant_id":20,"route_id":99,"step_id":97,"operation_id":98,"operation_name":"另一规格包装","note":"另一规格"}]');
 INSERT INTO %[1]s.process_route_operations VALUES (7,3,8,2,'包装');
 INSERT INTO %[1]s.job_cards VALUES (1,2,8);`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	orders, err := freezeWorkOrderRelatedOrdersTx(ctx, tx, schema, 1, "product", 9, 0, 0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 1 || len(orders[0].OperationNotes) != 1 || orders[0].OperationNotes[0].SequenceNo != 2 {
		t.Fatalf("wrong notes: %+v", orders)
	}
	if _, err = freezeWorkOrderRelatedOrdersTx(ctx, tx, schema, 1, "product", 9, 0, 0, 99); err == nil {
		t.Fatal("changed route must not silently drop a note")
	}
	var stored string
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT related_orders_snapshot_json::text FROM %s.work_orders WHERE id=1`, schema)).Scan(&stored); err != nil || stored == "" {
		t.Fatalf("snapshot missing %q %v", stored, err)
	}
}

func TestLateSupplyOrdersSupplementButNeverRewriteFrozenNotes(t *testing.T) {
	frozen := []productionapp.ProductionDemandOrder{{OrderID: 1, OrderItemID: 10, OperationNotes: []productionapp.ProductionOrderOperationNote{{Note: "冻结要求"}}}}
	live := []productionapp.ProductionDemandOrder{{OrderID: 1, OrderItemID: 10, OperationNotes: []productionapp.ProductionOrderOperationNote{{Note: "不得覆盖"}}}, {OrderID: 2, OrderItemID: 20, CustomerName: "后关联客户", OriginalItemNote: "不得猜测", OperationNotes: []productionapp.ProductionOrderOperationNote{{Note: "不得显示"}}}}
	got := mergeLateSupplyOrders(frozen, live)
	if len(got) != 2 || got[0].OperationNotes[0].Note != "冻结要求" || !got[1].LateAssociation || len(got[1].OperationNotes) != 0 || got[1].OriginalItemNote != "" {
		t.Fatalf("incorrect merge %+v", got)
	}
	got = mergeLateSupplyOrders([]productionapp.ProductionDemandOrder{}, live[1:])
	if len(got) != 1 || !got[0].LateAssociation {
		t.Fatalf("empty stock snapshot should show late order: %+v", got)
	}
}
