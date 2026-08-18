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

func TestResolvedBomCostsExposeDefaultSpecificationAsProductFallbackPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr602_default_spec_%d_%d", os.Getpid(), time.Now().UnixNano())
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
			version_id BIGINT PRIMARY KEY, operation_unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0,
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

		-- 227g 规格默认：0.227kg 熟豆半成品 + 1 个袋子
		-- 454g 规格非默认：0.454kg 熟豆半成品 + 1 个袋子
		INSERT INTO %[1]s.materials(id,name,unit,is_semi_finished,purchase_price) VALUES
			(2,'袋子','个',false,0.5),(30,'熟豆','kg',true,0);
		INSERT INTO %[1]s.production_boms(id,output_type,output_material_id,output_product_id) VALUES
			(30,'material',30,0),(60,'product',0,600);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,output_qty,output_unit) VALUES
			(300,30,'published',1,'kg'),(600,60,'published',1,'袋');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,is_default,bom_id,bom_version_id) VALUES
			('material',30,true,30,300),('product',600,true,60,600);
		-- 半成品制造配方：1kg 熟豆 = 1kg 生豆价格 100 元（用非半成品生豆物料演算）
		INSERT INTO %[1]s.materials(id,name,unit,is_semi_finished,purchase_price) VALUES (1,'生豆','kg',false,100);
		INSERT INTO %[1]s.production_bom_version_items(id,version_id,component_type,material_id,consume_unit,qty_per_unit)
		VALUES(3001,300,'material',1,'kg',1);

		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES
			(701,60,'BOM-SPEC-701','spec-1','227g','袋'),(702,60,'BOM-SPEC-702','spec-2','454g','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES
			(1701,600,701,'227g','袋',true,10),(1702,600,702,'454g','袋',false,20);
		INSERT INTO %[1]s.production_bom_version_items(
			id,version_id,variant_id,component_type,material_id,consume_unit,qty_per_unit
		) VALUES
			(6001,600,1701,'material',30,'kg',0.227),(6002,600,1701,'material',2,'个',1),
			(6003,600,1702,'material',30,'kg',0.454),(6004,600,1702,'material',2,'个',1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	costs, err := NewRepository(pool, schema).loadResolvedProductionBomCosts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	spec227, ok := costs[productionBomSpecCostMapKey(701)]
	if !ok || !spec227.Resolved {
		t.Fatalf("default 227g specification must resolve, got %+v", spec227)
	}
	wantDefault := 0.227*100 + 0.5
	if diff := spec227.TotalCostPerOutputUnit - wantDefault; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("default spec cost = %.6f, want %.6f", spec227.TotalCostPerOutputUnit, wantDefault)
	}

	productCost, ok := costs[600]
	if !ok {
		t.Fatalf("variant-only version must expose the product key as the default specification fallback: %+v", costs)
	}
	if !productCost.Resolved || productCost.ProductID != 600 {
		t.Fatalf("product fallback cost must stay resolved and carry product id, got %+v", productCost)
	}
	if diff := productCost.TotalCostPerOutputUnit - wantDefault; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("product fallback cost = %.6f, want default spec cost %.6f", productCost.TotalCostPerOutputUnit, wantDefault)
	}
	if productCost.VersionID != 600 {
		t.Fatalf("product fallback must keep the variant version id, got %d", productCost.VersionID)
	}

	spec454, ok := costs[productionBomSpecCostMapKey(702)]
	if !ok || !spec454.Resolved {
		t.Fatalf("non-default 454g specification must still resolve independently, got %+v", spec454)
	}
}

func TestResolveProductionBomTrialItemCostReportsSpecificReasons(t *testing.T) {
	// 零单价：半成品或未维护采购价的物料
	_, ok, reason := resolveProductionBomTrialItemCost(productionBomCostItem{
		ID: 1, ComponentType: "material", ComponentMaterialID: 71, ComponentIsSemi: true,
		ConsumeUnit: "kg", QtyPerUnit: 0.454, UnitCost: 0, UnitCostUnit: "kg",
	}, 0, "kg", 0, 1, "袋", map[int64]productionBomResolvedCost{})
	if ok {
		t.Fatal("zero unit cost must fail")
	}
	if !strings.Contains(reason, "单价为 0") || !strings.Contains(reason, "半成品") {
		t.Fatalf("zero-cost reason must mention zero price and semi-finished guidance, got %q", reason)
	}

	// 单位不匹配：consume_unit=kg 但成本单位是“个”
	_, ok, reason = resolveProductionBomTrialItemCost(productionBomCostItem{
		ID: 2, ComponentType: "material", ComponentMaterialID: 5,
		ConsumeUnit: "kg", QtyPerUnit: 1, UnitCost: 10, UnitCostUnit: "个",
	}, 10, "个", 0, 1, "袋", map[int64]productionBomResolvedCost{})
	if ok {
		t.Fatal("mass consume unit against piece cost unit must fail")
	}
	if !strings.Contains(reason, "无法换算") || !strings.Contains(reason, "kg") || !strings.Contains(reason, "个") {
		t.Fatalf("unit mismatch reason must name both units, got %q", reason)
	}
}
