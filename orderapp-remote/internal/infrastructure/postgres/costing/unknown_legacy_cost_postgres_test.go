package costing

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/costing"
	"os"
	"strings"
	"testing"
	"time"
)

func TestUnknownLegacyCostOnlyMarksAffectedProductPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("legacy_cost_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	sql := fmt.Sprintf(`CREATE TABLE %[1]s.products(id bigint PRIMARY KEY);
 CREATE TABLE %[1]s.cost_calculation_items(id bigserial,run_id bigint,result_json jsonb);
 CREATE TABLE %[1]s.product_bom_sources(product_id bigint,source_type text,source_product_id bigint);
 CREATE TABLE %[1]s.product_bom_items(product_id bigint,material_id bigint);
 CREATE TABLE %[1]s.material_batches(id bigint,material_id bigint,status text,quality_status text,cost_unavailable boolean);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,qty_g bigint,qty_units bigint);
 INSERT INTO %[1]s.products VALUES(1),(2);
 INSERT INTO %[1]s.cost_calculation_items(run_id,result_json) VALUES(99,'{"product_id":1,"name":"未知成本商品"}');
 INSERT INTO %[1]s.product_bom_items VALUES(1,10),(2,20);
 INSERT INTO %[1]s.material_batches VALUES(1,10,'active','pass',true),(2,20,'active','pass',false);
 INSERT INTO %[1]s.material_batch_locations VALUES(1,1000,0),(2,1000,0);`, schema)
	if _, err = pool.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
	inputs := []domain.ProductInput{{ProductID: 1}, {ProductID: 2}}
	if err = NewRepository(pool, schema).markUnknownLegacyBomCosts(ctx, inputs); err != nil {
		t.Fatal(err)
	}
	if !inputs[0].CostUnavailable || inputs[1].CostUnavailable {
		t.Fatalf("wrong unknown flags: %+v", inputs)
	}
	// A client supplied BOM version cannot hide an unknown legacy material.
	flags, err := NewRepository(pool, schema).FindUnknownCostProductRefs(ctx, []domain.ProductInput{{ProductID: 1, BomVersionID: 9999}, {ProductID: 2, BomVersionID: 9999}})
	if err != nil || len(flags) != 2 || !flags[0] || flags[1] {
		t.Fatalf("forged BOM version bypassed legacy stock cost: %+v %v", flags, err)
	}
	found, err := NewRepository(pool, schema).FindUnknownCostProductIDs(ctx, []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if !found[1] || found[2] {
		t.Fatalf("incorrect server recheck: %+v", found)
	}
	content := map[string]any{"groups": []any{map[string]any{"items": []any{map[string]any{"product_id": 1}}}}}
	if err = NewRepository(pool, schema).ValidateBeanListCostAvailability(ctx, content); err == nil || !strings.Contains(err.Error(), "待核算") {
		t.Fatalf("draft export guard: %v", err)
	}
	if err = NewRepository(pool, schema).PublishRun(ctx, "test", 99); err == nil || !strings.Contains(err.Error(), "待核算") {
		t.Fatalf("old run should be blocked: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unknownErr := NewRepository(pool, schema).validateCurrentProductCosts(ctx, tx, []int64{1})
	knownErr := NewRepository(pool, schema).validateCurrentProductCosts(ctx, tx, []int64{2})
	_ = tx.Rollback(ctx)
	if unknownErr == nil || !strings.Contains(unknownErr.Error(), "待核算") || knownErr != nil {
		t.Fatalf("publication guard unknown=%v known=%v", unknownErr, knownErr)
	}
}

func TestMaterialTrialIgnoresOtherVariantSnapshotsPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("variant_filter_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`CREATE TABLE %[1]s.production_bom_version_operation_costs(id bigint,version_id bigint,variant_id bigint,operation_name text,workstation_name text,capacity_name text,hourly_rate_snapshot numeric,standard_minutes_snapshot numeric,batch_size_qty_snapshot numeric,batch_size_unit_snapshot text,cost_method text,piece_rate_snapshot numeric,rate_unit_snapshot text,operation_unit_cost numeric,operation_cost_unit text,cost_unavailable boolean,sort_order int);
 INSERT INTO %[1]s.production_bom_version_operation_costs VALUES(1,1,0,'base','','',0,0,0,'kg','time',0,'',1,'kg',false,1),(2,1,99,'other variant','','',0,0,0,'kg','time',0,'',999,'kg',true,1);`, schema)
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	rows, err := NewRepository(pool, schema).loadMaterialCostTrialOperationDetails(ctx, 1, 0, "kg")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UnitCost != 1 {
		t.Fatalf("other variant affected base cost: %+v", rows)
	}
}
