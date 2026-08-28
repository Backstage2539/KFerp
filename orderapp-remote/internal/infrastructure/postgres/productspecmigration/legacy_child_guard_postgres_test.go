package productspecmigration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestManualLegacyChildrenAreMappedAndTombstonedPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES(1,'parent');
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,base_product_id,custom_type,spec_label,sku_code,sku_name,
			net_content_qty,net_content_unit
		) VALUES
			(2,'manual parent child',1,0,'','manual-parent','MANUAL-PARENT','手工父子规格',1,'袋'),
			(3,'manual base child',0,1,'','manual-base','MANUAL-BASE','手工基础规格',1,'盒'),
			(4,'family alias',0,1,'public_sku_alias','alias','ALIAS','客户别名',0,'');
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(10,'product',1,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
		VALUES(11,10,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',1,10,11,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
		VALUES(100,10,'manual-parent','手工父子规格','袋'),
		      (101,10,'manual-base','手工基础规格','盒');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES
			(200,11,100,'手工父子规格','袋',true,1),
			(201,11,101,'手工基础规格','盒',false,2);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	prepared, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 1, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Mappings) != 2 {
		t.Fatalf("manual legacy mappings=%+v, want parent/base children only", prepared.Mappings)
	}
	mappedChildren := map[int64]bool{}
	for _, mapping := range prepared.Mappings {
		mappedChildren[mapping.LegacyChildProductID] = true
	}
	if !mappedChildren[2] || !mappedChildren[3] || mappedChildren[4] {
		t.Fatalf("manual legacy mapping children=%v", mappedChildren)
	}

	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 1, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if !assessed.Readiness.Ready || assessed.Readiness.ActiveSpecCount != 2 || assessed.Readiness.PublishedSpecCount != 2 {
		t.Fatalf("manual legacy readiness=%+v", assessed.Readiness)
	}
	cutover, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 1, Actor: "guard-test"})
	if err != nil || cutover.State != productspecmigrationapp.StateCutover {
		t.Fatalf("manual legacy cutover=%+v err=%v", cutover, err)
	}

	for _, childID := range []int64{2, 3} {
		var active bool
		var status string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,derived_spec_status FROM %s.products WHERE id=$1`, schema), childID).Scan(&active, &status); err != nil {
			t.Fatal(err)
		}
		if active || status != "bom_spec_cutover" {
			t.Fatalf("manual child %d active=%v status=%q", childID, active, status)
		}
	}
	var aliasActive bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.products WHERE id=4`, schema)).Scan(&aliasActive); err != nil {
		t.Fatal(err)
	}
	if !aliasActive {
		t.Fatal("family alias was incorrectly tombstoned as a legacy child SKU")
	}

	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,parent_product_id,active) VALUES(5,'late child',1,true)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "legacy_child_sku_write_rejected") {
		t.Fatalf("active child insert after cutover error=%v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,base_product_id,custom_type,active)
		VALUES(6,'late family alias',1,'public_sku_alias',true)
	`, schema)); err != nil {
		t.Fatalf("family-level alias after cutover must remain allowed: %v", err)
	}
}

func TestDuplicateLegacySpecKeyWithDifferentUnitsBlocksCutoverPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES(10,'parent');
		INSERT INTO %[1]s.products(id,name,parent_product_id,spec_label,sku_code,sku_name,net_content_qty,net_content_unit)
		VALUES(11,'same bag',10,'same-spec','SAME-BAG','同规格袋',1,'袋'),
		      (12,'same box',10,'same-spec','SAME-BOX','同规格盒',1,'盒');
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(20,'product',10,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
		VALUES(21,20,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',10,20,21,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
		VALUES(110,20,'same-spec','同规格','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(210,21,110,'同规格','袋',true,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	prepared, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 10, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Mappings) != 2 {
		t.Fatalf("duplicate legacy mappings=%+v, want both source children preserved", prepared.Mappings)
	}
	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 10, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if assessed.Readiness.Ready {
		t.Fatalf("unit-ambiguous duplicate legacy key unexpectedly ready: %+v", assessed.Readiness)
	}
	foundAmbiguous := false
	for _, blocker := range assessed.Readiness.Blockers {
		if blocker.Code == "ambiguous_legacy_specs" && blocker.Count > 0 {
			foundAmbiguous = true
		}
	}
	if !foundAmbiguous {
		t.Fatalf("duplicate unit blockers=%+v, want ambiguous_legacy_specs", assessed.Readiness.Blockers)
	}
	_, err = repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 10, Actor: "guard-test"})
	var blocked *productspecmigrationapp.CutoverBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("unit-ambiguous duplicate cutover error=%v", err)
	}
}

func TestSimpleProductWithoutLegacyChildCanCutOverWithPublishedBOMSpecPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES(20,'simple parent');
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(30,'product',20,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
		VALUES(31,30,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',20,30,31,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
		VALUES(120,30,'default','默认规格','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(220,31,120,'默认规格','袋',true,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	prepared, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 20, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Mappings) != 0 {
		t.Fatalf("simple product mappings=%+v, want no legacy child mapping", prepared.Mappings)
	}
	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 20, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if !assessed.Readiness.Ready || assessed.Readiness.ActiveSpecCount != 1 || assessed.Readiness.PublishedSpecCount != 1 {
		t.Fatalf("simple product readiness=%+v, want one published BOM spec and ready", assessed.Readiness)
	}
	cutover, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 20, Actor: "guard-test"})
	if err != nil || cutover.State != productspecmigrationapp.StateCutover {
		t.Fatalf("simple product cutover=%+v err=%v", cutover, err)
	}
	var active bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.products WHERE id=20`, schema)).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("simple parent product was incorrectly tombstoned")
	}
	var tombstoned int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE((meta->>'legacy_children_tombstoned')::bigint,-1)
		FROM %s.audit_logs
		WHERE entity_type='product_bom_spec_migration' AND entity_id=20 AND action='cutover'
		ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&tombstoned); err != nil {
		t.Fatal(err)
	}
	if tombstoned != 0 {
		t.Fatalf("simple product cutover audit tombstoned=%d, want 0 legacy children", tombstoned)
	}
}

func TestLegacyUnitMustMatchPublishedBOMSpecUnitPostgres(t *testing.T) {
	for _, tc := range []struct {
		name       string
		legacyUnit string
	}{
		{name: "different unit", legacyUnit: "g"},
		{name: "missing unit", legacyUnit: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, schema := newLegacyChildGuardTestDB(t)
			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %[1]s.products(id,name) VALUES(40,'parent');
				INSERT INTO %[1]s.products(
					id,name,parent_product_id,derived_spec_key,derived_spec_name,derived_sales_unit,
					spec_label,sku_code,sku_name,net_content_qty,net_content_unit
				) VALUES(41,'legacy child',40,'one-kg','一千克',%[2]s,'一千克','LEGACY-1KG','一千克',0,'');
				INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
				VALUES(50,'product',40,'active');
				INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
				VALUES(51,50,'V001','published',now());
				INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
				VALUES('product',40,50,51,true);
				INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
				VALUES(140,50,'one-kg','一千克','kg');
				INSERT INTO %[1]s.production_bom_version_variants(
					id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
				) VALUES(240,51,140,'一千克','kg',true,1);
			`, schema, quoteLiteral(tc.legacyUnit))); err != nil {
				t.Fatal(err)
			}

			repo := NewRepository(pool, schema)
			if _, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 40, Actor: "guard-test"}); err != nil {
				t.Fatal(err)
			}
			assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 40, Actor: "guard-test"})
			if err != nil {
				t.Fatal(err)
			}
			if assessed.Readiness.Ready {
				t.Fatalf("legacy unit %q unexpectedly ready: %+v", tc.legacyUnit, assessed.Readiness)
			}
			found := false
			for _, blocker := range assessed.Readiness.Blockers {
				if blocker.Code == "legacy_unit_mismatch" && blocker.Count == 1 {
					found = true
				}
			}
			if !found {
				t.Fatalf("legacy unit %q blockers=%+v, want legacy_unit_mismatch", tc.legacyUnit, assessed.Readiness.Blockers)
			}
			_, err = repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 40, Actor: "guard-test"})
			var blocked *productspecmigrationapp.CutoverBlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("legacy unit %q cutover error=%v, want blocked", tc.legacyUnit, err)
			}
		})
	}
}

func TestLegacyUnitUsesCurrentInventoryOverrideInsteadOfDisplaySalesLabelPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		ALTER TABLE %[1]s.products ADD COLUMN unit_rule_override_json TEXT NOT NULL DEFAULT '{}';
		INSERT INTO %[1]s.products(id,name) VALUES(60,'parent');
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,derived_spec_key,derived_spec_name,derived_sales_unit,
			spec_label,sku_code,sku_name,net_content_qty,net_content_unit,unit_rule_override_json
		) VALUES(
			61,'legacy box',60,'box-10','盒（10袋）','盒（10袋）',
			'盒（10袋）','LEGACY-BOX-10','盒（10袋）',10,'袋',
			'{"inventory_unit":"盒","default_sales_unit":"盒","unit_conversion_json":{"盒":{"盒":1}}}'
		);
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(70,'product',60,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
		VALUES(71,70,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',60,70,71,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
		VALUES(160,70,'box-10','盒（10袋）','盒');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(260,71,160,'盒（10袋）','盒',true,1);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	prepared, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 60, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Mappings) != 1 || prepared.Mappings[0].LegacySalesUnit != "盒" {
		t.Fatalf("legacy mapping=%+v, want current inventory unit 盒 instead of display label", prepared.Mappings)
	}
	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 60, Actor: "guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	if !assessed.Readiness.Ready || assessed.Readiness.LegacyUnitMismatchCount != 0 {
		t.Fatalf("inventory-unit override readiness=%+v, want ready", assessed.Readiness)
	}
}

func TestReadinessCountsLegacyCustomerInventoryWithoutBOMColumnsPostgres(t *testing.T) {
	ctx, pool, schema := newLegacyChildGuardTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		DROP TABLE %[1]s.customer_inventory_items;
		CREATE TABLE %[1]s.customer_inventory_items(
			id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL DEFAULT 0,
			item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,
			item_name TEXT NOT NULL DEFAULT '',spec_g BIGINT NOT NULL DEFAULT 0,
			warehouse TEXT NOT NULL DEFAULT '',qty_g BIGINT NOT NULL DEFAULT 0,
			qty_units BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'available'
		);
		INSERT INTO %[1]s.products(id,name) VALUES(60,'parent');
		INSERT INTO %[1]s.products(
			id,name,parent_product_id,derived_spec_key,derived_spec_name,derived_sales_unit,spec_label,sku_code,sku_name
		) VALUES(61,'legacy child',60,'bag','袋装','袋','袋装','LEGACY-BAG','袋装');
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status) VALUES(70,'product',60,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at) VALUES(71,70,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',60,70,71,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(160,70,'bag','袋装','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(260,71,160,'袋装','袋',true,1);
		INSERT INTO %[1]s.customer_inventory_items(customer_id,item_type,item_id,item_name,spec_g,warehouse,qty_units)
		VALUES(1,'product',61,'袋装',0,'customer',2);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	if _, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 60, Actor: "guard-test"}); err != nil {
		t.Fatal(err)
	}
	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 60, Actor: "guard-test"})
	if err != nil {
		t.Fatalf("assess legacy customer inventory schema: %v", err)
	}
	if assessed.Readiness.Ready || assessed.Readiness.LegacyStockCount != 1 {
		t.Fatalf("legacy customer inventory readiness=%+v, want one stock blocker", assessed.Readiness)
	}
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func newLegacyChildGuardTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
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
	schema := fmt.Sprintf("pr600_legacy_child_guard_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '',active BOOLEAN NOT NULL DEFAULT true,
			parent_product_id BIGINT NOT NULL DEFAULT 0,base_product_id BIGINT NOT NULL DEFAULT 0,
			custom_type TEXT NOT NULL DEFAULT '',auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
			derived_spec_key TEXT NOT NULL DEFAULT '',derived_spec_name TEXT NOT NULL DEFAULT '',
			derived_sales_unit TEXT NOT NULL DEFAULT '',derived_unit_template_id BIGINT NOT NULL DEFAULT 0,
			derived_spec_status TEXT NOT NULL DEFAULT '',spec_label TEXT NOT NULL DEFAULT '',sku_code TEXT NOT NULL DEFAULT '',
			sku_name TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '',net_content_qty NUMERIC(14,6) NOT NULL DEFAULT 0,
			net_content_unit TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,
			old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.production_boms(id BIGINT PRIMARY KEY,output_type TEXT,output_product_id BIGINT,status TEXT);
		CREATE TABLE %[1]s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT,version_no TEXT,status TEXT,published_at TIMESTAMPTZ,
			source_spec_template_version_id BIGINT NOT NULL DEFAULT 900,
			main_input_material_id BIGINT NOT NULL DEFAULT 901
		);
		CREATE TABLE %[1]s.production_bom_spec_template_versions(
			id BIGINT PRIMARY KEY,status TEXT NOT NULL,published_at TIMESTAMPTZ
		);
		CREATE TABLE %[1]s.materials(
			id BIGINT PRIMARY KEY,deprecated_at TIMESTAMPTZ
		);
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,status,published_at)
		VALUES(900,'archived',now());
		INSERT INTO %[1]s.materials(id,deprecated_at) VALUES(901,NULL);
		CREATE TABLE %[1]s.production_bom_output_bindings(
			output_type TEXT,output_id BIGINT,bom_id BIGINT,bom_version_id BIGINT,is_default BOOLEAN,updated_at TIMESTAMPTZ DEFAULT now()
		);
		CREATE TABLE %[1]s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT,code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '',
			spec_key TEXT,name TEXT,inventory_unit TEXT
		);
			CREATE TABLE %[1]s.production_bom_version_variants(
				id BIGINT PRIMARY KEY,version_id BIGINT,bom_spec_id BIGINT,spec_name_snapshot TEXT,inventory_unit TEXT,
				is_default BOOLEAN,sort_order INT,material_loss_rate NUMERIC,process_route_id BIGINT
			);
			-- customer portal/fulfillment schema runs before the PR-600 migration
			-- schema and therefore starts without the additive BOM identity columns.
			CREATE TABLE %[1]s.customer_inventory_items(
				id BIGSERIAL PRIMARY KEY,customer_id BIGINT NOT NULL DEFAULT 0,
				item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,
				item_name TEXT NOT NULL DEFAULT '',spec_g BIGINT NOT NULL DEFAULT 0,
				warehouse TEXT NOT NULL DEFAULT '',qty_g BIGINT NOT NULL DEFAULT 0,
				qty_units BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'available'
			);
		`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("repeat EnsureSchema: %v", err)
	}
	return ctx, pool, schema
}
