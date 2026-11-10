package production

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	productionpg "orderapp/internal/infrastructure/postgres/production"
)

func TestPlanMaterialOptionsDatabaseIdentityAndPending(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_plans(id,plan_no,status,picking_version) VALUES(71,'PP-OPT','draft',1),(72,'PP-OTHER','draft',1);
		INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_name) VALUES(81,71,'任务一'),(82,72,'任务二');
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES(11,'M-A','物料甲','bean','kg','kg'),(12,'M-B','物料乙','bean','kg','kg');
		INSERT INTO %[1]s.production_plan_component_sources(id,production_plan_id,production_plan_item_id,component_type,component_id,component_name,unit,required_g) VALUES(91,71,81,'material',11,'物料甲','kg',3000),(92,72,82,'material',12,'物料乙','kg',1000);
		INSERT INTO %[1]s.material_batches(id,batch_code,material_id,material_name,received_g,qty_g,remaining_g,status,quality_status,owner_customer_id) VALUES(101,'GOOD',11,'物料甲',5000,5000,5000,'active','pass',0),(102,'HELD',11,'物料甲',2000,2000,2000,'active','hold',0),(103,'OTHER',12,'物料乙',1000,1000,1000,'active','pass',0),(104,'WIP',11,'物料甲',500,500,500,'active','pass',0);
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(101,'GOOD',11,'raw_materials',5000),(102,'HELD',11,'raw_materials',2000),(103,'OTHER',12,'raw_materials',1000),(104,'WIP',11,'wip',500);
		INSERT INTO %[1]s.work_order_material_reservations(id,work_order_id,material_id,component_type,component_id,reserved_g,source_warehouse,status) VALUES(401,701,11,'material',11,1000,'raw_materials','reserved');
		INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,reserved_g,status) VALUES(401,701,11,'material',11,101,'GOOD','raw_materials',1000,'reserved');
		INSERT INTO %[1]s.stock_entries(id,entry_no,status) VALUES(201,'SE-DRAFT','draft'),(202,'SE-DONE','submitted');
		INSERT INTO %[1]s.production_plan_material_transfer_links(production_plan_id,production_plan_item_id,component_source_id,stock_entry_id,stock_entry_item_id,plan_revision,qty_g) VALUES(71,81,91,201,301,1,1200),(71,81,91,201,302,1,300),(71,81,91,202,303,1,500);
	`, schema))
	repo := productionpg.NewRepository(pool, schema)
	got, err := repo.PlanMaterialTransferOptions(ctx, 71, 91, "raw_materials")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.ID != 91 || len(got.Options) != 2 {
		t.Fatalf("unexpected rows %+v", got)
	}
	for _, row := range got.Options {
		if row.ItemID != 11 || row.OwnerCustomerID != 0 {
			t.Fatalf("foreign batch leaked %+v", row)
		}
		if row.BatchCode == "GOOD" && (row.AvailableG != 4000 || row.ReservedG == nil || *row.ReservedG != 1000 || row.QualityStatus != "pass") {
			t.Fatalf("valid stock incorrect: %+v", row)
		}
		if row.BatchCode == "HELD" && (row.AvailableG != 0 || row.QualityStatus != "hold" || row.UnavailableReason == "") {
			t.Fatalf("held stock incorrect: %+v", row)
		}
	}
	if len(got.PendingDocuments) != 2 || got.PendingDocuments[0].QtyG != 1500 {
		t.Fatalf("wrong linked documents: %+v", got.PendingDocuments)
	}
	if _, err := repo.PlanMaterialTransferOptions(ctx, 71, 92, "raw_materials"); err == nil {
		t.Fatal("other plan source was accepted")
	}
	if _, err := repo.PlanMaterialTransferOptions(ctx, 71, 91, "unknown"); err == nil {
		t.Fatal("unknown warehouse was accepted")
	}
	all, err := repo.PlanMaterialTransferOptions(ctx, 71, 91, "")
	if err != nil || len(all.Options) != 3 {
		t.Fatalf("all warehouse query failed: %+v, %v", all, err)
	}
	wip, err := repo.PlanMaterialTransferOptions(ctx, 71, 91, "wip")
	if err != nil || len(wip.Options) != 1 || wip.Options[0].BatchCode != "WIP" {
		t.Fatalf("wip query failed: %+v, %v", wip, err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservations(id,work_order_id,material_id,component_type,component_id,reserved_g,source_warehouse,status) VALUES(402,702,11,'material',11,300,'raw_materials','reserved')`, schema))
	unbound, err := repo.PlanMaterialTransferOptions(ctx, 71, 91, "raw_materials")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range unbound.Options {
		encoded, _ := json.Marshal(row)
		var decoded map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		if row.AvailableG != 0 || row.UnavailableReason == "" || decoded["reserved_g"] != nil {
			t.Fatalf("unbound reservation must not be reported as known batch reservation: %+v %s", row, encoded)
		}
	}
	// Legacy links created before equivalent-weight storage retain grams and
	// packaged quantities must be reconstructed from the frozen specification.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.products(id,name,sku_name,sku_code,product_kind,active) VALUES(991,'旧版商品','旧版商品','OLD-PACK','roasted_bean',true);
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_name) VALUES(83,71,'旧版商品任务');
 INSERT INTO %[1]s.production_plan_component_sources(id,production_plan_id,production_plan_item_id,component_type,component_id,component_spec_g,component_name,unit,required_g,required_units) VALUES(93,71,83,'product',991,454,'旧版商品','件',908,2);
 INSERT INTO %[1]s.stock_entries(id,entry_no,status) VALUES(203,'SE-OLD-PACK','draft');
 INSERT INTO %[1]s.production_plan_material_transfer_links(production_plan_id,production_plan_item_id,component_source_id,stock_entry_id,stock_entry_item_id,plan_revision,qty_units) VALUES(71,83,93,203,304,1,2);`, schema))
	old, err := repo.PlanMaterialTransferOptions(ctx, 71, 93, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(old.PendingDocuments) != 1 || old.PendingDocuments[0].QtyG != 908 || old.PendingDocuments[0].QtyUnits != 2 {
		t.Fatalf("legacy packed pending not reconstructed: %+v", old.PendingDocuments)
	}

}
