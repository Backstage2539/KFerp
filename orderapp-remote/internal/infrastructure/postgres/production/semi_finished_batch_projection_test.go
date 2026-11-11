package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestProjectSemiFinishedBatchesUsesOnlySelectedBatchAndSharesIt(t *testing.T) {
	sources := []app.ProductionPlanComponentSource{
		{ID: 10, ComponentType: "material", ComponentID: 5, RequiredG: 7000},
		{ID: 11, ComponentType: "material", ComponentID: 5, RequiredG: 8000},
	}
	pools := []semiFinishedBatchPool{{materialID: 5, batchID: 8, batchCode: "chosen", owner: 0, warehouse: "wip", selectedG: 10000, availableG: 10000}}
	projectSemiFinishedBatchSources(sources, pools)
	if sources[0].WIPCoveredG != 7000 || sources[1].WIPCoveredG != 3000 || sources[1].ShortageG != 5000 {
		t.Fatalf("allocation should share selected batch once: %+v", sources)
	}
	if len(sources[0].Allocations) != 1 || sources[0].Allocations[0].Batches[0].BatchID != 8 {
		t.Fatalf("selected batch lost: %+v", sources[0])
	}
}

func TestProjectSemiFinishedBatchesLeavesOtherMaterialsUntouched(t *testing.T) {
	sources := []app.ProductionPlanComponentSource{{ID: 10, ComponentType: "material", ComponentID: 6, RequiredG: 3, WIPCoveredG: 2}}
	projectSemiFinishedBatchSources(sources, []semiFinishedBatchPool{{materialID: 5, batchID: 1, selectedG: 1, availableG: 1}})
	if sources[0].WIPCoveredG != 2 {
		t.Fatal("unrelated material changed")
	}
}

func TestNormalizeSelectedSemiFinishedUsesActualWIPWithoutChangingBatch(t *testing.T) {
	selected := []semiFinishedNormalizedSelection{
		{materialID: 5, batchID: 17, owner: 0, warehouse: "raw_materials", qtyG: 10000},
		{materialID: 5, batchID: 17, owner: 0, warehouse: "wip", qtyG: 5000},
		{materialID: 5, batchID: 18, owner: 0, warehouse: "raw_materials", qtyG: 4000},
	}
	got := normalizeSelectedSemiFinishedLocations(selected, map[int64][2]int64{17: {10000, 0}, 18: {0, 0}})
	quantities := map[string]int64{}
	for _, s := range got {
		quantities[fmt.Sprintf("%d:%s", s.batchID, s.warehouse)] += s.qtyG
	}
	if quantities["17:wip"] != 10000 || quantities["17:raw_materials"] != 5000 || quantities["18:raw_materials"] != 4000 {
		t.Fatalf("unexpected allocation: %+v", quantities)
	}
}

func TestSelectedSemiFinishedTransferRejectsUnselectedBatch(t *testing.T) {
	source := app.ProductionPlanComponentSource{AllocationMode: "semi_finished_selected", Allocations: []app.ProductionPlanSourceAllocation{{Warehouse: "raw_materials", OwnerCustomerID: 4, Batches: []app.ProductionPlanSourceBatch{{BatchCode: "selected", QtyG: 5000}}}}}
	if err := validateSelectedSemiFinishedTransfer(source, "raw_materials", 4, "wrong", 1, 0, 0, 0); err == nil {
		t.Fatal("unselected batch must fail")
	}
	if err := validateSelectedSemiFinishedTransfer(source, "raw_materials", 4, "selected", 3000, 0, 1000, 0); err != nil {
		t.Fatal(err)
	}
	if err := validateSelectedSemiFinishedTransfer(source, "raw_materials", 4, "selected", 5000, 0, 1000, 0); err == nil {
		t.Fatal("pending quantity must count")
	}
}

func TestSelectedReservationRequiresExplicitCompleteBatches(t *testing.T) {
	a := app.ProductionPlanSourceAllocation{QtyG: 5000, Batches: []app.ProductionPlanSourceBatch{{BatchID: 17, QtyG: 4000}}}
	if err := validateSelectedReservationAllocation(a); err == nil {
		t.Fatal("missing batch quantity must fail")
	}
	a.Batches[0].QtyG = 5000
	if err := validateSelectedReservationAllocation(a); err != nil {
		t.Fatal(err)
	}
	a.Batches[0].BatchID = 0
	if err := validateSelectedReservationAllocation(a); err == nil {
		t.Fatal("batch id required")
	}
}

func TestSemiFinishedProjectionAndReservationPreserveSelectedBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_plans(id bigint primary key,semi_finished_stock_policy text);
 CREATE TABLE %[1]s.production_plan_semi_finished_selections(production_plan_id bigint,item_key text,material_id bigint,warehouse text,batch_id bigint,qty_g bigint,qty_units bigint,transfer_baseline_g bigint not null default 0,transfer_baseline_units bigint not null default 0);
 CREATE TABLE %[1]s.material_batches(id bigint primary key,batch_code text,owner_customer_id bigint,material_id bigint,status text,quality_status text);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,material_id bigint,warehouse text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.warehouses(code text,name text); CREATE TABLE %[1]s.customers(id bigint,name text);
 CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigserial primary key,reservation_id bigint,work_order_id bigint,material_id bigint,component_type text,component_id bigint,component_bom_spec_id bigint not null default 0,component_spec_g bigint not null default 0,material_batch_id bigint,stock_batch_id bigint,batch_code text,warehouse text,owner_customer_id bigint,reserved_g bigint default 0,consumed_g bigint default 0,returned_g bigint default 0,reserved_units bigint default 0,consumed_units bigint default 0,returned_units bigint default 0,status text,created_at timestamptz,updated_at timestamptz, UNIQUE(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse));
 INSERT INTO %[1]s.production_plans VALUES(1,'manual');
 INSERT INTO %[1]s.production_plan_semi_finished_selections(production_plan_id,item_key,material_id,warehouse,batch_id,qty_g,qty_units) VALUES(1,'material:5',5,'raw_materials',17,10000,0),(1,'material:5',5,'wip',17,5000,0);
 CREATE TABLE %[1]s.stock_entries(id bigint,status text);
 CREATE TABLE %[1]s.stock_entry_items(id bigint,stock_entry_id bigint,from_warehouse text,item_type text,material_id bigint,owner_customer_id bigint,batch_code text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.production_plan_material_transfer_links(production_plan_id bigint,stock_entry_id bigint,stock_entry_item_id bigint);
 INSERT INTO %[1]s.material_batches VALUES(17,'SELECTED',0,5,'active','pass'),(18,'FIFO-OTHER',0,5,'active','pass');
 INSERT INTO %[1]s.material_batch_locations VALUES(17,5,'wip',5000,0),(17,5,'raw_materials',10000,0),(18,5,'wip',40000,0);
 `, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	sources := []app.ProductionPlanComponentSource{{ComponentType: "material", ComponentID: 5, RequiredG: 7000}, {ComponentType: "material", ComponentID: 5, RequiredG: 8000}}
	if err = applySelectedSemiFinishedBatchSourcesTx(ctx, tx, schema, 1, sources); err != nil {
		t.Fatal(err)
	}
	if sources[0].WIPCoveredG != 5000 || sources[0].TransferG != 2000 || sources[1].TransferG != 8000 {
		t.Fatalf("unexpected allocation %+v", sources)
	}
	// The same chosen batch can be moved by a separate stock document. Its
	// actual WIP location must be recognized without a link to this plan.
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET qty_g=10000 WHERE material_batch_id=17 AND warehouse='wip'; UPDATE %[1]s.material_batch_locations SET qty_g=5000 WHERE material_batch_id=17 AND warehouse='raw_materials'`, schema)); err != nil {
		t.Fatal(err)
	}
	sources = []app.ProductionPlanComponentSource{{ComponentType: "material", ComponentID: 5, RequiredG: 7000}, {ComponentType: "material", ComponentID: 5, RequiredG: 8000}}
	if err = applySelectedSemiFinishedBatchSourcesTx(ctx, tx, schema, 1, sources); err != nil {
		t.Fatal(err)
	}
	if sources[0].WIPCoveredG != 7000 || sources[1].WIPCoveredG != 3000 || sources[1].TransferG != 5000 {
		t.Fatalf("external move should be counted once for selected batch: %+v", sources)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.material_batch_locations SET qty_g=15000 WHERE material_batch_id=17 AND warehouse='wip'; UPDATE %[1]s.material_batch_locations SET qty_g=0 WHERE material_batch_id=17 AND warehouse='raw_materials'; INSERT INTO %[1]s.stock_entries VALUES(55,'submitted'); INSERT INTO %[1]s.stock_entry_items VALUES(56,55,'raw_materials','material',5,0,'SELECTED',10000,0); INSERT INTO %[1]s.production_plan_material_transfer_links VALUES(1,55,56)`, schema)); err != nil {
		t.Fatal(err)
	}
	sources = []app.ProductionPlanComponentSource{{ComponentType: "material", ComponentID: 5, RequiredG: 15000}}
	if err = applySelectedSemiFinishedBatchSourcesTx(ctx, tx, schema, 1, sources); err != nil {
		t.Fatal(err)
	}
	if sources[0].WIPCoveredG != 15000 || sources[0].ShortageG != 0 {
		t.Fatalf("posted transfer not mapped into WIP: %+v", sources)
	}
	if err = bindSelectedSemiFinishedReservationTx(ctx, tx, schema, 99, 101, 5, sources[0].Allocations[0]); err != nil {
		t.Fatal(err)
	}
	var batchID, qty int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT material_batch_id,reserved_g FROM %s.work_order_material_reservation_batches WHERE reservation_id=99`, schema)).Scan(&batchID, &qty); err != nil {
		t.Fatal(err)
	}
	if batchID != 17 || qty != 15000 {
		t.Fatalf("bound batch %d qty %d", batchID, qty)
	}
	// Re-selecting from the original warehouse records the old posted move as
	// baseline; it must not be credited a second time to the new selection.
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_semi_finished_selections SET qty_g=7000,transfer_baseline_g=10000 WHERE warehouse='raw_materials'`, schema)); err != nil {
		t.Fatal(err)
	}
	normalized, err := normalizedSelectedSemiFinishedTx(ctx, tx, schema, 1)
	if err != nil {
		t.Fatal(err)
	}
	gotWIP, gotRaw := int64(0), int64(0)
	for _, row := range normalized {
		if row.warehouse == "wip" {
			gotWIP += row.qtyG
		} else if row.warehouse == "raw_materials" {
			gotRaw += row.qtyG
		}
	}
	if gotWIP != 5000 || gotRaw != 7000 {
		t.Fatalf("old transfer counted again: wip=%d raw=%d", gotWIP, gotRaw)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plans SET semi_finished_stock_policy='' WHERE id=1`, schema)); err != nil {
		t.Fatal(err)
	}
	legacy := []app.ProductionPlanComponentSource{{ComponentType: "material", ComponentID: 5, RequiredG: 7000, WIPCoveredG: 1234}}
	if err = applySelectedSemiFinishedBatchSourcesTx(ctx, tx, schema, 1, legacy); err != nil {
		t.Fatal(err)
	}
	if legacy[0].WIPCoveredG != 1234 {
		t.Fatalf("legacy projection changed: %+v", legacy[0])
	}
}
