package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestPlanPickingFrozenVariantsAndLegacyDatabase(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer pool.Close()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`
 CREATE TABLE %[1]s.warehouses(code TEXT,name TEXT,kind TEXT,customer_id BIGINT,sort_order INT,active BOOLEAN);
 INSERT INTO %[1]s.warehouses VALUES('wip','WIP','wip',0,1,true);
 CREATE TABLE %[1]s.stock_batches(id BIGINT PRIMARY KEY,batch_code TEXT,item_type TEXT,item_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,spec_g BIGINT,owner_customer_id BIGINT,remaining_g BIGINT,remaining_units BIGINT,quality_status TEXT,created_at TIMESTAMPTZ DEFAULT now());
 CREATE TABLE %[1]s.stock_ledger_entries(id BIGSERIAL PRIMARY KEY,item_type TEXT,item_id BIGINT,bom_spec_id BIGINT,spec_g BIGINT,warehouse TEXT,source_batch_code TEXT,source_batch_id TEXT);
 CREATE TABLE %[1]s.work_order_material_reservation_batches(stock_batch_id BIGINT,status TEXT,reserved_g BIGINT,consumed_g BIGINT,returned_g BIGINT,reserved_units BIGINT,consumed_units BIGINT,returned_units BIGINT);
 INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,bom_spec_id,bom_variant_id,spec_g,owner_customer_id,remaining_g,remaining_units,quality_status) VALUES
 (11,'FROZEN-A','finished_product',9,4,11,1000,0,10000,10,'pass'),
 (12,'FROZEN-B','finished_product',9,4,12,1000,0,10000,10,'pass'),
 (13,'OTHER-VARIANT','finished_product',9,4,13,1000,0,50000,50,'pass');
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,bom_spec_id,spec_g,warehouse,source_batch_code,source_batch_id) VALUES
 ('finished_product',9,4,1000,'wip','FROZEN-A',''),('finished_product',9,4,1000,'wip','FROZEN-B',''),('finished_product',9,4,1000,'wip','OTHER-VARIANT','');`, schema)
	mustExecProductionSQL(t, ctx, pool, ddl)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	mk := func(id, variant, units int64) app.ProductionPlanComponentSource {
		s := app.ProductionPlanComponentSource{ID: id, ComponentType: "product", ComponentID: 9, ComponentBOMSpecID: 4, ComponentBOMVariantID: variant, ComponentSpecG: 1000, RequiredG: units * 1000, RequiredUnits: units, AllocationMode: "auto"}
		var e error
		s.Options, e = componentSourceOptionsTx(ctx, tx, schema, s, 0)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	rows := []app.ProductionPlanComponentSource{mk(1, 11, 8), mk(2, 11, 5), mk(3, 12, 5)}
	calculatePickingSources(rows)
	if rows[0].WIPCoveredUnits != 8 || rows[1].WIPCoveredUnits != 2 || rows[2].WIPCoveredUnits != 5 {
		t.Fatalf("variant/shared budget wrong: %+v", rows)
	}
	if err := fillPickingBatchSuggestionsTx(ctx, tx, schema, rows); err != nil {
		t.Fatal(err)
	}
	for _, s := range rows {
		for _, a := range s.Allocations {
			for _, b := range a.Batches {
				if (s.ComponentBOMVariantID == 11 && b.BatchCode != "FROZEN-A") || (s.ComponentBOMVariantID == 12 && b.BatchCode != "FROZEN-B") {
					t.Fatalf("wrong frozen batch source=%d batch=%s", s.ID, b.BatchCode)
				}
			}
		}
	}
	legacy := []app.ProductionPlanComponentSource{mk(4, 0, 70)}
	calculatePickingSources(legacy)
	if legacy[0].WIPCoveredUnits != 70 {
		t.Fatalf("legacy unversioned inventory changed: %+v", legacy[0])
	}
	mixed := []app.ProductionPlanComponentSource{mk(5, 0, 70), mk(6, 11, 5)}
	calculatePickingSources(mixed)
	if mixed[0].WIPCoveredUnits != 0 || !strings.Contains(mixed[0].AdjustmentMessage, "版本") {
		t.Fatalf("ambiguous mixed historical variant must be blocked: %+v", mixed)
	}
}
