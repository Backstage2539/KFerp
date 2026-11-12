package stock

import (
	"context"
	"fmt"
	"strings"
	"testing"

	stockapp "orderapp/internal/application/stock"
)

func TestPlanTransferWIPGuardsRelevantInventoryAndAllowsOverage(t *testing.T) {
	pool, schema, svc, _ := setupPickingSnapshotTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,unit) VALUES (2,'BAG2','无关袋','袋');
 INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,reserved_units) VALUES(88,2,1);
 CREATE TABLE %[1]s.production_plan_component_sources(id bigint PRIMARY KEY,production_plan_id bigint,production_plan_item_id bigint,component_type text,component_id bigint,component_bom_spec_id bigint,component_bom_variant_id bigint,component_spec_g bigint,required_g bigint,required_units bigint,allocation_mode text,allocations_json jsonb);
 INSERT INTO %[1]s.production_plan_component_sources VALUES (20,1,1,'material',1,0,0,0,65384,0,'auto','[]'),(21,1,2,'material',2,0,0,0,0,80,'auto','[]'),(22,1,3,'material',1,0,0,0,5000,0,'auto','[]');`, schema))
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialReceipt, Operator: "fixture", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 2, InventoryUnit: "袋", QtyUnits: 423, ToWarehouse: "wip"}}}); err != nil {
		t.Fatal(err)
	}
	if err := ensurePlanMaterialTransferLinks(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.production_plan_material_transfer_links(production_plan_id,production_plan_item_id,component_source_id,stock_entry_id,stock_entry_item_id,plan_revision,qty_g) VALUES (1,1,20,50,40,1,120000)`, schema))
	doc := stockapp.StockDocumentDetail{StockDocumentRow: stockapp.StockDocumentRow{ID: 50, SourceType: planMaterialTransferSource, SourceID: 1}}
	verify := func() error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		return NewRepository(pool, schema).validatePlanMaterialTransferWIPTx(ctx, tx, doc)
	}
	if err := verify(); err != nil {
		t.Fatalf("unrelated WIP reservation must not block overage: %v", err)
	}
	// A transfer attached to source 22 must still respect source 20's
	// earlier claim on the same physical WIP inventory.
	if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{Purpose: stockapp.PurposeMaterialReceipt, Operator: "fixture", Items: []stockapp.StockDocumentItemCommand{{ItemType: "material", MaterialID: 1, InventoryUnit: "g", QtyG: 30000, ToWarehouse: "wip"}}}); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	coverage, err := NewRepository(pool, schema).planCurrentWIPCoverageForSourcesTx(ctx, tx, 1, map[int64]bool{22: true})
	_ = tx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if coverage[20].g != 30000 || coverage[22].g != 0 {
		t.Fatalf("shared WIP budget duplicated: %+v", coverage)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.production_plan_material_transfer_links SET component_source_id=21,qty_g=0,qty_units=1 WHERE stock_entry_id=50`, schema))
	if err := verify(); err == nil || !strings.Contains(err.Error(), "WIP库存") {
		t.Fatalf("relevant anomaly must block: %v", err)
	}
}

func TestSelectedPlanWIPIgnoresUnrelatedInvalidSelection(t *testing.T) {
	ctx := context.Background()
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_plans(id bigint primary key,semi_finished_stock_policy text);
 CREATE TABLE %[1]s.production_plan_semi_finished_selections(production_plan_id bigint,material_id bigint,batch_id bigint,warehouse text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.material_batches(id bigint primary key,owner_customer_id bigint,material_id bigint,status text,quality_status text);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,warehouse text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.work_order_material_reservation_batches(material_batch_id bigint,warehouse text,status text,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint);
 INSERT INTO %[1]s.production_plans VALUES(1,'manual');
 INSERT INTO %[1]s.production_plan_semi_finished_selections VALUES(1,5,17,'raw_materials',15000,0),(1,6,18,'raw_materials',5000,0);
 INSERT INTO %[1]s.material_batches VALUES(17,0,5,'active','pass'),(18,0,6,NULL,'pass');
 INSERT INTO %[1]s.material_batch_locations VALUES(17,'wip',10000,0);`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	coverage, selected, err := NewRepository(pool, schema).selectedPlanWIPCoverageTx(ctx, tx, 1, []planWIPSource{{ID: 10, Kind: "material", ItemID: 5, RequiredG: 15000}})
	if err != nil || !selected[10] || coverage[10].g != 10000 {
		t.Fatalf("unrelated selection blocked valid source: coverage=%+v selected=%+v err=%v", coverage, selected, err)
	}
	_, _, err = NewRepository(pool, schema).selectedPlanWIPCoverageTx(ctx, tx, 1, []planWIPSource{{ID: 11, Kind: "material", ItemID: 6, RequiredG: 5000}})
	if err == nil {
		t.Fatal("invalid selected batch for the current material must not be silently accepted")
	}
	_, selected, err = NewRepository(pool, schema).selectedPlanWIPCoverageTx(ctx, tx, 1, []planWIPSource{{ID: 12, Kind: "finished_product", ItemID: 9, RequiredG: 5000}})
	if err != nil || len(selected) != 0 {
		t.Fatalf("no material sources: selected=%v err=%v", selected, err)
	}
}
