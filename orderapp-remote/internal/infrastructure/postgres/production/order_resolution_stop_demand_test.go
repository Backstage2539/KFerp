package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestStoppedPlanDoesNotHideReopenedRemainingDemand(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); pool.Close() })
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.orders(id BIGINT,order_no TEXT,customer_id BIGINT);
 CREATE TABLE %[1]s.order_items(id BIGINT,order_id BIGINT,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT);
 CREATE TABLE %[1]s.production_plans(id BIGINT,plan_no TEXT,status TEXT);
 CREATE TABLE %[1]s.production_plan_items(id BIGINT,production_plan_id BIGINT,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,spec_g BIGINT,customer_id BIGINT,order_nos TEXT,replan_status TEXT,demand_sources_json JSONB);
 CREATE TABLE %[1]s.work_orders(id BIGINT,production_plan_item_id BIGINT,work_order_no TEXT,status TEXT);
 INSERT INTO %[1]s.orders VALUES(1,'REOPEN',1);INSERT INTO %[1]s.order_items VALUES(1,1,10,11,12);
 INSERT INTO %[1]s.production_plans VALUES(1,'ORIGINAL-STOP','stopped');
 INSERT INTO %[1]s.production_plan_items VALUES(1,1,10,11,12,0,1,'REOPEN','active','[]');
 INSERT INTO %[1]s.work_orders VALUES(1,1,'STOPPED-REMAINDER','stopped'),(2,1,'PRESERVED-COMPLETED','completed');
 `, schema))
	rows, err := splitStructuredProductionDemandRow(ctx, pool, schema, UnprodNeedRow{ProductID: 10, BomSpecID: 11, BomVariantID: 12, InventoryQtyPerSalesUnit: 1, InventoryUnit: "袋", OrderDetails: []app.ProductionDemandOrder{{OrderItemID: 1, OrderID: 1, OrderNo: "REOPEN", Quantity: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DemandStatus != "unplanned" || !rows[0].DemandSelectable || rows[0].ProductionPlanID != 0 {
		t.Fatalf("historical completed part hid reopened remainder: %+v", rows)
	}
}
