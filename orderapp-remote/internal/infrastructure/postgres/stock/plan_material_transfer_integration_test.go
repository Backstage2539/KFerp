package stock

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
	"os"
	"testing"
	"time"
)

func TestPlanMaterialTransferPostingGuardDatabase(t *testing.T) {
	url := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("isolated database not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("plan_transfer_guard_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`CREATE TABLE %[1]s.production_plans(id BIGINT PRIMARY KEY,status TEXT,revision BIGINT);
 CREATE TABLE %[1]s.production_plan_items(id BIGINT PRIMARY KEY,production_plan_id BIGINT);
 CREATE TABLE %[1]s.production_plan_component_sources(id BIGINT PRIMARY KEY,production_plan_id BIGINT,production_plan_item_id BIGINT,component_type TEXT,component_id BIGINT,component_bom_spec_id BIGINT,component_bom_variant_id BIGINT,component_spec_g BIGINT,required_g BIGINT,required_units BIGINT,unit TEXT,bom_version_id BIGINT,allocation_mode TEXT,allocations_json JSONB,source_owner_customer_id BIGINT);
 CREATE TABLE %[1]s.stock_entry_items(id BIGINT PRIMARY KEY,stock_entry_id BIGINT,item_type TEXT,material_id BIGINT,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,spec_g BIGINT,qty_g BIGINT,qty_units BIGINT);
 INSERT INTO %[1]s.production_plans VALUES(1,'draft',2);
 INSERT INTO %[1]s.production_plan_items VALUES(10,1);
 INSERT INTO %[1]s.production_plan_component_sources VALUES(20,1,10,'material',30,0,0,0,100,0,'g',3,'auto','[]'::jsonb,0);
 INSERT INTO %[1]s.stock_entry_items VALUES(40,50,'material',30,0,0,0,0,100,0);`, schema)
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	if err = ensurePlanMaterialTransferLinks(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_plan_material_transfer_links(production_plan_id,production_plan_item_id,component_source_id,stock_entry_id,stock_entry_item_id,plan_revision,qty_g,source_snapshot) VALUES(1,10,20,50,40,2,100,jsonb_build_object('required_g',100,'required_units',0,'unit','g','bom_version_id',3,'allocation_mode','auto','allocations_json','[]'::jsonb,'source_owner_customer_id',0))`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	doc := stockapp.StockDocumentDetail{StockDocumentRow: stockapp.StockDocumentRow{ID: 50, SourceType: planMaterialTransferSource, SourceID: 1}, Items: []stockapp.StockDocumentItemRow{{ID: 40}}}
	verify := func(wantErr bool) {
		t.Helper()
		tx, e := pool.BeginTx(ctx, pgx.TxOptions{})
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback(ctx)
		e = repo.validatePlanMaterialTransferDocumentTx(ctx, tx, doc)
		if (e != nil) != wantErr {
			t.Fatalf("wantErr %v, error %v", wantErr, e)
		}
	}
	verify(false)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET qty_g=101 WHERE id=40`, schema)); err != nil {
		t.Fatal(err)
	}
	verify(true)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_entry_items SET qty_g=100 WHERE id=40; UPDATE %s.production_plans SET revision=3 WHERE id=1`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	verify(false) // capacity or other unrelated revision must not invalidate material transfer
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_component_sources SET required_g=99 WHERE id=20`, schema)); err != nil {
		t.Fatal(err)
	}
	verify(true)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_component_sources SET required_g=100 WHERE id=20`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_component_sources WHERE id=20`, schema)); err != nil {
		t.Fatal(err)
	}
	verify(true)
}

func TestPlanWIPCoverageSeparatesFrozenVariantsDatabase(t *testing.T) {
	pool, schema, _, _ := setupPickingSnapshotTest(t)
	ctx := context.Background()
	ddl := fmt.Sprintf(`
 CREATE TABLE %[1]s.production_plan_component_sources(
 id BIGINT PRIMARY KEY,production_plan_id BIGINT,production_plan_item_id BIGINT,component_type TEXT,component_id BIGINT,
 component_bom_spec_id BIGINT,component_bom_variant_id BIGINT,component_spec_g BIGINT,required_g BIGINT,required_units BIGINT,
 allocation_mode TEXT,allocations_json JSONB);
 INSERT INTO %[1]s.production_plan_component_sources VALUES
 (11,1,1,'product',9,0,11,1000,8000,8,'auto','[]'),
 (12,1,2,'product',9,0,12,1000,5000,5,'auto','[]');
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status)
 VALUES('FROZEN-VARIANT-A','finished_product',9,'测试熟豆',0,11,1000,10000,10,10000,10,'pass'),
 ('FROZEN-VARIANT-B','finished_product',9,'测试熟豆',0,12,1000,10000,10,10000,10,'pass');
 INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,owner_customer_id,onhand_units) VALUES(9,0,11,1000,'wip',0,20);
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,bom_spec_id,bom_variant_id,spec_g,warehouse,source_batch_code) VALUES('finished_product',9,0,11,1000,'wip','FROZEN-VARIANT-A'),('finished_product',9,0,12,1000,'wip','FROZEN-VARIANT-B');`, schema)
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	coverage, err := NewRepository(pool, schema).planCurrentWIPCoverageTx(ctx, tx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if coverage[11].n != 8 || coverage[11].g != 8000 || coverage[12].n != 5 || coverage[12].g != 5000 {
		t.Fatalf("frozen variants mixed: %+v", coverage)
	}
}
