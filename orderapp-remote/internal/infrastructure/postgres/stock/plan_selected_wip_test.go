package stock

import (
	"context"
	"fmt"
	"testing"
)

func TestSelectedPlanWIPCoverageUsesExternallyMovedSelectedBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_plans(id bigint primary key,semi_finished_stock_policy text);
 CREATE TABLE %[1]s.production_plan_semi_finished_selections(production_plan_id bigint,material_id bigint,batch_id bigint,warehouse text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.material_batches(id bigint primary key,owner_customer_id bigint,material_id bigint,status text,quality_status text);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,warehouse text,qty_g bigint,qty_units bigint);
 CREATE TABLE %[1]s.work_order_material_reservation_batches(material_batch_id bigint,warehouse text,status text,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint);
 INSERT INTO %[1]s.production_plans VALUES(1,'manual');
 INSERT INTO %[1]s.production_plan_semi_finished_selections VALUES(1,5,17,'raw_materials',15000,0);
 INSERT INTO %[1]s.material_batches VALUES(17,0,5,'active','pass'),(18,0,5,'active','pass');
 INSERT INTO %[1]s.material_batch_locations VALUES(17,'wip',10000,0),(18,'wip',40000,0);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	sources := []planWIPSource{{ID: 10, Kind: "material", ItemID: 5, RequiredG: 7000}, {ID: 11, Kind: "material", ItemID: 5, RequiredG: 8000}}
	coverage, selected, err := NewRepository(pool, schema).selectedPlanWIPCoverageTx(ctx, tx, 1, sources)
	if err != nil {
		t.Fatal(err)
	}
	if !selected[10] || !selected[11] || coverage[10].g != 7000 || coverage[11].g != 3000 {
		t.Fatalf("unexpected WIP coverage %+v selected %+v", coverage, selected)
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET quality_status='hold' WHERE id=17`, schema)); err != nil {
		t.Fatal(err)
	}
	coverage, _, err = NewRepository(pool, schema).selectedPlanWIPCoverageTx(ctx, tx, 1, sources)
	if err != nil {
		t.Fatal(err)
	}
	if coverage[10].g != 0 || coverage[11].g != 0 {
		t.Fatalf("held or unselected batch covered demand: %+v", coverage)
	}
}
