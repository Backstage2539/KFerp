package costing

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResolvedBomCostDoesNotFallbackToSemiFinishedPurchaseOrBatchCostPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr600_cost_semi_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(
			id BIGINT PRIMARY KEY, unit TEXT NOT NULL DEFAULT 'kg',
			is_semi_finished BOOLEAN NOT NULL DEFAULT false,
			purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.material_batches(
			id BIGINT PRIMARY KEY, unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active', quality_status TEXT NOT NULL DEFAULT 'unchecked'
		);
		CREATE TABLE %[1]s.material_batch_locations(
			material_batch_id BIGINT NOT NULL, material_id BIGINT NOT NULL,
			qty_g BIGINT NOT NULL DEFAULT 0, qty_units BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_boms(
			id BIGINT PRIMARY KEY, output_type TEXT NOT NULL,
			output_material_id BIGINT NOT NULL DEFAULT 0, output_product_id BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %[1]s.production_bom_versions(
			id BIGINT PRIMARY KEY, bom_id BIGINT NOT NULL, status TEXT NOT NULL,
			yield_rate NUMERIC(12,6) NOT NULL DEFAULT 1,
			output_qty NUMERIC(18,6) NOT NULL DEFAULT 1, output_unit TEXT NOT NULL DEFAULT 'unit'
		);
		CREATE TABLE %[1]s.production_bom_output_bindings(
			output_type TEXT NOT NULL, output_id BIGINT NOT NULL, is_default BOOLEAN NOT NULL,
			bom_id BIGINT NOT NULL, bom_version_id BIGINT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_version_operation_costs(
			version_id BIGINT NOT NULL, operation_unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
			cost_method TEXT NOT NULL DEFAULT 'time', operation_cost_unit TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.production_bom_version_items(
			id BIGINT PRIMARY KEY, version_id BIGINT NOT NULL,
			component_type TEXT NOT NULL DEFAULT 'material', material_id BIGINT NOT NULL DEFAULT 0,
			component_product_id BIGINT NOT NULL DEFAULT 0, component_spec_g BIGINT NOT NULL DEFAULT 0,
			consume_unit TEXT NOT NULL DEFAULT 'ratio_pct', qty_per_unit NUMERIC(18,6) NOT NULL DEFAULT 0,
			ratio_pct NUMERIC(12,6) NOT NULL DEFAULT 0, material_loss_rate NUMERIC(12,6) NOT NULL DEFAULT 0,
			unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.materials(id,unit,is_semi_finished,purchase_price) VALUES(20,'kg',true,999);
		INSERT INTO %[1]s.material_batches(id,unit_cost,status,quality_status) VALUES(200,888,'active','qualified');
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,material_id,qty_g,qty_units) VALUES(200,20,1000,0);
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id) VALUES(30,'product',30);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,output_qty,output_unit) VALUES(300,30,'published',1,'袋');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,is_default,bom_id,bom_version_id)
		VALUES('product',30,true,30,300);
		INSERT INTO %[1]s.production_bom_version_items(
			id,version_id,component_type,material_id,consume_unit,qty_per_unit,unit_cost_snapshot
		) VALUES(3001,300,'material',20,'g_per_bag',227,0);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	costs, err := NewRepository(pool, schema).loadResolvedProductionBomCosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got, exists := costs[30]
	if !exists {
		t.Fatalf("resolved cost map missing product 30: %+v", costs)
	}
	if got.Resolved || got.TotalCostPerOutputUnit != 0 {
		t.Fatalf("semi-finished fallback resolved cost = %+v, want unresolved until a default published manufacturing BOM exists", got)
	}
}
