package costing

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResolvedBomCostsKeepPublishedProductSpecificationsIsolatedPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_cost_variants_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(
			id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', unit TEXT NOT NULL DEFAULT 'kg',
			is_semi_finished BOOLEAN NOT NULL DEFAULT false, purchase_price NUMERIC(12,4) NOT NULL DEFAULT 0
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
		CREATE TABLE %[1]s.production_bom_specs(
			id BIGINT PRIMARY KEY, bom_id BIGINT NOT NULL, code TEXT NOT NULL DEFAULT '', barcode TEXT NOT NULL DEFAULT '',
			spec_key TEXT NOT NULL, name TEXT NOT NULL, inventory_unit TEXT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_version_variants(
			id BIGINT PRIMARY KEY, version_id BIGINT NOT NULL, bom_spec_id BIGINT NOT NULL,
			spec_name_snapshot TEXT NOT NULL, inventory_unit TEXT NOT NULL, is_default BOOLEAN NOT NULL DEFAULT false,
			sort_order INT NOT NULL DEFAULT 100, material_loss_rate NUMERIC(12,6) NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.process_route_operations(
			id BIGINT PRIMARY KEY, route_id BIGINT NOT NULL, planned_operation_cost NUMERIC(14,4) NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_bom_version_items(
			id BIGINT PRIMARY KEY, version_id BIGINT NOT NULL, variant_id BIGINT NOT NULL DEFAULT 0,
			component_type TEXT NOT NULL DEFAULT 'material', material_id BIGINT NOT NULL DEFAULT 0,
			component_product_id BIGINT NOT NULL DEFAULT 0, component_bom_spec_id BIGINT NOT NULL DEFAULT 0,
			component_spec_g BIGINT NOT NULL DEFAULT 0, consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
			qty_per_unit NUMERIC(18,6) NOT NULL DEFAULT 0, ratio_pct NUMERIC(12,6) NOT NULL DEFAULT 0,
			material_loss_rate NUMERIC(12,6) NOT NULL DEFAULT 0, unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0
		);

		INSERT INTO %[1]s.materials(id,name,unit,is_semi_finished,purchase_price) VALUES
			(1,'生豆','kg',false,100),(2,'袋子','个',false,0.5),(3,'无价格组件','个',false,0),(30,'熟豆','kg',true,0);

		INSERT INTO %[1]s.production_boms(id,output_type,output_material_id,output_product_id) VALUES
			(30,'material',30,0),(60,'product',0,600),(70,'product',0,700);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,output_qty,output_unit) VALUES
			(300,30,'published',1,'kg'),(600,60,'published',1,'unit'),(700,70,'published',1,'盒');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,is_default,bom_id,bom_version_id) VALUES
			('material',30,true,30,300),('product',600,true,60,600),('product',700,true,70,700);
		INSERT INTO %[1]s.production_bom_version_operation_costs(version_id,operation_unit_cost,cost_method,operation_cost_unit)
		VALUES(700,0.1,'time','盒');
		INSERT INTO %[1]s.production_bom_version_items(id,version_id,component_type,material_id,consume_unit,qty_per_unit)
		VALUES(3001,300,'material',1,'kg',1),(7001,700,'material',2,'unit',2);

		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		SELECT 700+n,60,'BOM-SPEC-'||(700+n)::text,'bag-'||n::text,'袋规格'||n::text,'袋'
		FROM generate_series(1,10) AS n;
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
		) SELECT 1700+n,600,700+n,'袋规格'||n::text,'袋',n=1,n*10,0,900+n
		FROM generate_series(1,10) AS n;
		INSERT INTO %[1]s.process_route_operations(id,route_id,planned_operation_cost) VALUES
			(1,901,0.4),(2,902,0.3);

		-- 规格2：每袋 227g 熟豆 + 1 个袋子；规格1明确递归消耗规格2。
		INSERT INTO %[1]s.production_bom_version_items(
			id,version_id,variant_id,component_type,material_id,component_product_id,component_bom_spec_id,consume_unit,qty_per_unit
		) VALUES
			(6001,600,1701,'product',0,600,702,'unit',1),
			(6002,600,1702,'material',30,0,0,'g',227),
			(6003,600,1702,'material',2,0,0,'unit',1),
			(6004,600,1703,'material',30,0,0,'g',454),
			(6005,600,1703,'material',2,0,0,'unit',1);
		INSERT INTO %[1]s.production_bom_version_items(
			id,version_id,variant_id,component_type,material_id,consume_unit,qty_per_unit
		) SELECT 6100+n,600,1700+n,'material',2,'unit',1 FROM generate_series(4,9) AS n;
		-- 第10个规格有实际用量但没有可用成本，必须只让自身 fail closed。
		INSERT INTO %[1]s.production_bom_version_items(
			id,version_id,variant_id,component_type,material_id,consume_unit,qty_per_unit
		) VALUES(6010,600,1710,'material',3,'unit',1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	costs, err := NewRepository(pool, schema).loadResolvedProductionBomCosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bag227 := costs[-702]
	if !bag227.Resolved {
		t.Fatalf("227g specification cost unresolved: %+v", bag227)
	}
	if diff := math.Abs(bag227.InputCostPerOutputUnit - 23.2); diff > 1e-9 {
		t.Fatalf("227g specification input cost = %.6f, want 23.2", bag227.InputCostPerOutputUnit)
	}
	if diff := math.Abs(bag227.OperationCostPerOutputUnit - 0.3); diff > 1e-9 {
		t.Fatalf("227g specification route cost = %.6f, want 0.3", bag227.OperationCostPerOutputUnit)
	}
	if diff := math.Abs(100*bag227.InputCostPerOutputUnit - 2320); diff > 1e-9 {
		t.Fatalf("100 bags input cost = %.6f, want 22.7kg beans + 100 bags = 2320", 100*bag227.InputCostPerOutputUnit)
	}
	if !bag227.HasManufacturedMaterialComponent {
		t.Fatal("227g specification must recursively cost its manufactured semi-finished material")
	}

	bag454 := costs[-703]
	if !bag454.Resolved || math.Abs(bag454.InputCostPerOutputUnit-45.9) > 1e-9 {
		t.Fatalf("454g same-unit specification cost = %+v, want isolated 45.9", bag454)
	}
	bagFromBag := costs[-701]
	if !bagFromBag.Resolved || !bagFromBag.HasProductComponent || math.Abs(bagFromBag.TotalCostPerOutputUnit-23.9) > 1e-9 {
		t.Fatalf("specification-to-specification recursive cost = %+v, want 23.9", bagFromBag)
	}
	if invalid := costs[-710]; invalid.Resolved {
		t.Fatalf("invalid tenth specification must fail closed, got %+v", invalid)
	}
	if !costs[-702].Resolved || !costs[-703].Resolved {
		t.Fatal("one invalid specification must not contaminate sibling specification costs")
	}
	legacy := costs[700]
	if !legacy.Resolved || math.Abs(legacy.TotalCostPerOutputUnit-1.1) > 1e-9 {
		t.Fatalf("legacy single-recipe product cost = %+v, want 1.1", legacy)
	}
}
