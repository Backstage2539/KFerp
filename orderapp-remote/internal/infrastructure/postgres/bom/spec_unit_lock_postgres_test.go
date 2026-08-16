package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionBomSpecInventoryUnitLocksHistoricalAndBusinessReferencesPostgres(t *testing.T) {
	tests := []struct {
		name       string
		prepareSQL string
	}{
		{
			name: "archived BOM version",
			prepareSQL: `
				INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,output_unit) VALUES(101,1,'archived','袋');
				INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)
				VALUES(1001,101,11,'227g 袋装','袋',true);
			`,
		},
		{
			name:       "finished inventory",
			prepareSQL: `INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,warehouse,onhand_units) VALUES(7,11,'finished_goods',0);`,
		},
		{
			name:       "stock batch history",
			prepareSQL: `INSERT INTO %[1]s.stock_batches(item_type,item_id,bom_spec_id) VALUES('finished_product',7,11);`,
		},
		{
			name:       "price record",
			prepareSQL: `INSERT INTO %[1]s.product_price_records(product_id,bom_spec_id) VALUES(7,11);`,
		},
		{
			name:       "price tier",
			prepareSQL: `INSERT INTO %[1]s.product_price_tiers(product_id,bom_spec_id) VALUES(7,11);`,
		},
		{
			name:       "tier price scheme",
			prepareSQL: `INSERT INTO %[1]s.product_tier_price_scheme_tiers(scheme_id,bom_spec_id) VALUES(3,11);`,
		},
		{
			name:       "order history",
			prepareSQL: `INSERT INTO %[1]s.order_items(order_id,product_id,bom_spec_id) VALUES(8,7,11);`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, schema := newProductionBomSpecUnitLockTestDB(t)
			if _, err := pool.Exec(ctx, fmt.Sprintf(tc.prepareSQL, schema)); err != nil {
				t.Fatalf("prepare %s reference: %v", tc.name, err)
			}

			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_versions SET output_unit='盒' WHERE id=201`, schema)); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			err = saveProductionBomDraftVariantsTx(ctx, tx, schema, 1, 201, []bomapp.ProductionBomDraftVariant{{
				BomSpecID:     11,
				SpecKey:       "227g-bag",
				Name:          "227g 袋装",
				InventoryUnit: "盒",
				IsDefault:     true,
				Items: []bomapp.ProductionBomDraftItem{{
					MaterialID:    21,
					ComponentType: "material",
					ConsumeUnit:   "kg",
					QtyPerUnit:    0.227,
				}},
			}}, "pr600-unit-lock")
			if err == nil || !strings.Contains(err.Error(), "inventory_unit cannot be changed") {
				_ = tx.Rollback(ctx)
				t.Fatalf("unit change with %s reference error = %v", tc.name, err)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}

			var specUnit, versionUnit, variantUnit string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT inventory_unit FROM %s.production_bom_specs WHERE id=11`, schema)).Scan(&specUnit); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT output_unit FROM %s.production_bom_versions WHERE id=201`, schema)).Scan(&versionUnit); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT inventory_unit FROM %s.production_bom_version_variants WHERE id=2001`, schema)).Scan(&variantUnit); err != nil {
				t.Fatalf("draft variant was not rolled back: %v", err)
			}
			if specUnit != "袋" || versionUnit != "袋" || variantUnit != "袋" {
				t.Fatalf("rejected unit change mutated snapshots: spec=%q version=%q variant=%q", specUnit, versionUnit, variantUnit)
			}
			var auditCount int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs`, schema)).Scan(&auditCount); err != nil {
				t.Fatal(err)
			}
			if auditCount != 0 {
				t.Fatalf("rejected unit change wrote %d audit rows", auditCount)
			}
		})
	}
}

func TestProductionBomSpecChangesWriteFieldAuditsInSameTransactionPostgres(t *testing.T) {
	ctx, pool, schema := newProductionBomSpecUnitLockTestDB(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	variants := []bomapp.ProductionBomDraftVariant{
		{
			BomSpecID: 11, SpecKey: "227g-bag", Name: "227g 新袋装", InventoryUnit: "盒", Barcode: "BAR-227", IsDefault: true,
			Items: []bomapp.ProductionBomDraftItem{{MaterialID: 21, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.227}},
		},
		{
			SpecKey: "454g-bag", Name: "454g 袋装", InventoryUnit: "袋", Barcode: "BAR-454",
			Items: []bomapp.ProductionBomDraftItem{{MaterialID: 21, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: 0.454}},
		},
	}
	if err := saveProductionBomDraftVariantsTx(ctx, tx, schema, 1, 201, variants, "spec-auditor"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	assertAudit := func(action, field, oldValue, newValue string) {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*) FROM %s.audit_logs
			WHERE actor='spec-auditor' AND entity_type='production_bom_spec'
			  AND action=$1 AND field=$2 AND COALESCE(old_value,'')=$3 AND COALESCE(new_value,'')=$4
		`, schema), action, field, oldValue, newValue).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("audit %s/%s %q -> %q count = %d, want 1", action, field, oldValue, newValue, count)
		}
	}
	assertAudit("update", "name", "227g 袋装", "227g 新袋装")
	assertAudit("update", "inventory_unit", "袋", "盒")
	assertAudit("update", "barcode", "", "BAR-227")
	assertAudit("create", "spec_key", "", "454g-bag")
	assertAudit("create", "name", "", "454g 袋装")
	assertAudit("create", "inventory_unit", "", "袋")
	assertAudit("create", "barcode", "", "BAR-454")

	var newSpecID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_bom_specs WHERE spec_key='454g-bag'`, schema)).Scan(&newSpecID); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveProductionBomDraftVariantsTx(ctx, tx, schema, 1, 201, variants[1:], "spec-auditor"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var removedCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE actor='spec-auditor' AND entity_type='production_bom_spec' AND entity_id=11
		  AND action='remove_from_draft' AND field='bom_version_id'
		  AND old_value='201' AND COALESCE(new_value,'')=''
	`, schema)).Scan(&removedCount); err != nil {
		t.Fatal(err)
	}
	if removedCount != 1 {
		t.Fatalf("removed specification audit count = %d, want 1", removedCount)
	}
	var stableSpecCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_specs WHERE id IN (11,$1)`, schema), newSpecID).Scan(&stableSpecCount); err != nil {
		t.Fatal(err)
	}
	if stableSpecCount != 2 {
		t.Fatalf("draft removal must retain stable specification tombstone/history rows, count=%d", stableSpecCount)
	}
}

func newProductionBomSpecUnitLockTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
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
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("test_pr600_spec_unit_lock_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL,
			yield_rate NUMERIC NOT NULL DEFAULT 1,output_qty NUMERIC NOT NULL DEFAULT 1,
			output_unit TEXT NOT NULL DEFAULT '袋',material_loss_rate NUMERIC NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0,
			special_attrs_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
			special_attrs_json JSONB NOT NULL DEFAULT '{}'::jsonb
		);
		CREATE TABLE %[1]s.production_bom_specs(
			id BIGSERIAL PRIMARY KEY,bom_id BIGINT NOT NULL,code TEXT NOT NULL,spec_key TEXT NOT NULL,
			name TEXT NOT NULL,inventory_unit TEXT NOT NULL,barcode TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',updated_by TEXT NOT NULL DEFAULT ''
		);
		CREATE UNIQUE INDEX production_bom_specs_bom_key_uq ON %[1]s.production_bom_specs(bom_id,lower(spec_key));
		CREATE TABLE %[1]s.production_bom_version_variants(
			id BIGSERIAL PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,
			sort_order INT NOT NULL DEFAULT 0,material_loss_rate NUMERIC NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_bom_version_items(
			id BIGSERIAL PRIMARY KEY,version_id BIGINT NOT NULL,variant_id BIGINT NOT NULL DEFAULT 0,
			material_id BIGINT NOT NULL DEFAULT 0,component_type TEXT NOT NULL DEFAULT 'material',
			component_product_id BIGINT NOT NULL DEFAULT 0,component_bom_spec_id BIGINT NOT NULL DEFAULT 0,
			component_spec_g BIGINT NOT NULL DEFAULT 0,consume_unit TEXT NOT NULL DEFAULT '',
			qty_per_unit NUMERIC NOT NULL DEFAULT 0,ratio_pct NUMERIC NOT NULL DEFAULT 0,
			material_loss_rate NUMERIC NOT NULL DEFAULT 0,unit_cost_snapshot NUMERIC NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,purchase_price NUMERIC NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.product_unit_definitions(code TEXT PRIMARY KEY,active BOOLEAN NOT NULL,deleted_at TIMESTAMPTZ);
		CREATE TABLE %[1]s.finished_inventory(product_id BIGINT,bom_spec_id BIGINT,warehouse TEXT,onhand_units BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.stock_batches(id BIGSERIAL PRIMARY KEY,item_type TEXT,item_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.stock_ledger_entries(id BIGSERIAL PRIMARY KEY,item_type TEXT,item_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.stock_entry_items(id BIGSERIAL PRIMARY KEY,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.stock_adjustment_items(id BIGSERIAL PRIMARY KEY,item_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.finished_product_transfer_items(id BIGSERIAL PRIMARY KEY,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.product_price_records(id BIGSERIAL PRIMARY KEY,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.product_price_tiers(id BIGSERIAL PRIMARY KEY,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.product_tier_price_scheme_tiers(id BIGSERIAL PRIMARY KEY,scheme_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.order_items(id BIGSERIAL PRIMARY KEY,order_id BIGINT,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.order_stock_batch_allocations(id BIGSERIAL PRIMARY KEY,order_id BIGINT,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.order_stock_deductions(id BIGSERIAL PRIMARY KEY,order_id BIGINT,product_id BIGINT,bom_spec_id BIGINT);
		CREATE TABLE %[1]s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,
			field TEXT,old_value TEXT,new_value TEXT,meta JSONB
		);
		INSERT INTO %[1]s.product_unit_definitions(code,active) VALUES('袋',true),('盒',true);
		INSERT INTO %[1]s.materials(id,purchase_price) VALUES(21,40);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		VALUES(11,1,'BOM-SPEC-000011','227g-bag','227g 袋装','袋');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,output_unit) VALUES(201,1,'draft','袋');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)
		VALUES(2001,201,11,'227g 袋装','袋',true);
		INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,consume_unit,qty_per_unit)
		VALUES(201,2001,21,'kg',0.227);
	`, schema)); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return ctx, pool, schema
}
