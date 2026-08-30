package catalog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	catalogapp "orderapp/internal/application/catalog"
	productspecmigrationapp "orderapp/internal/application/productspecmigration"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateSKURejectsLegacyChildAfterBOMSpecCutoverPostgres(t *testing.T) {
	ctx, pool, schema := newProductSpecCutoverCatalogTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.products(id,name,active,parent_product_id) VALUES(100,'cutover parent',true,0);
		INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(100,'cutover');
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	_, err := repo.CreateSKU(ctx, catalogapp.CreateSKUCommand{
		Actor:                "guard-test",
		ParentProductID:      100,
		Name:                 "late legacy child",
		SKUName:              "晚到规格",
		SpecLabel:            "late-spec",
		NetContentQty:        1,
		NetContentUnit:       "袋",
		Active:               true,
		SpecialAttrsJSON:     "{}",
		UnitRuleOverrideJSON: "{}",
	})
	var children int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products WHERE parent_product_id=100`, schema)).Scan(&children); err != nil {
		t.Fatal(err)
	}
	if err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "BOM 规格") {
		t.Fatalf("CreateSKU after cutover error=%v children=%d, want BOM-spec cutover validation before insert", err, children)
	}
	if children != 0 {
		t.Fatalf("CreateSKU persisted %d legacy children after cutover", children)
	}
}

func TestSyncDerivedSKUsDoesNotReactivateCutoverTombstonePostgres(t *testing.T) {
	ctx, pool, schema := newProductSpecCutoverCatalogTestDB(t)
	var templateID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_templates(
			name,inventory_unit,quote_unit,order_unit,unit_conversion_json,sales_specs_json,integer_unit,active
		) VALUES(
			'cutover template','袋','袋','袋','{}'::jsonb,
			'[{
				"spec_key":"bag-227","spec_name":"227g袋","sales_unit":"袋",
				"net_content_qty":227,"net_content_unit":"g","active":true,"default":true
			}]'::jsonb,false,true
		) RETURNING id
	`, schema)).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,parent_product_id,unit_template_id)
		VALUES(200,'cutover parent',true,0,$1)
	`, schema), templateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,active,parent_product_id,unit_template_id,auto_derived_sku,derived_unit_template_id,
			derived_spec_key,derived_spec_name,derived_sales_unit,derived_spec_status,spec_label,
			net_content_qty,net_content_unit
		) VALUES(201,'old 227g',false,200,$1,true,$1,'bag-227','227g袋','袋','bom_spec_cutover','227g袋',227,'g')
	`, schema), templateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(200,'cutover')
	`, schema)); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := syncDerivedSKUsForParentTx(ctx, tx, schema, "guard-test", 200); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("sync cutover parent: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var active bool
	var status string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,derived_spec_status FROM %s.products WHERE id=201`, schema)).Scan(&active, &status); err != nil {
		t.Fatal(err)
	}
	if active || status != "bom_spec_cutover" {
		t.Fatalf("sync reactivated cutover tombstone: active=%v status=%q", active, status)
	}
}

func TestLegacySpecWriteInterfacesRetireAfterAllActiveProductsCutoverPostgres(t *testing.T) {
	ctx, pool, schema := newProductSpecCutoverCatalogTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,parent_product_id,base_product_id) VALUES
			(300,'cutover product A',true,0,0),
			(301,'cutover product B',true,0,0);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES
			(300,'cutover'),(301,'legacy');
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	active := true
	template, err := repo.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor: "guard-test", Name: "legacy template before retirement", InventoryUnit: "袋",
		QuoteUnit: "袋", OrderUnit: "袋", UnitConversionJSON: "{}", Active: &active,
	})
	if err != nil {
		t.Fatalf("legacy template before full cutover: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.product_bom_spec_migrations SET state='cutover' WHERE product_id=301`, schema)); err != nil {
		t.Fatal(err)
	}

	_, err = repo.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor: "guard-test", Name: "late legacy template", InventoryUnit: "袋",
		QuoteUnit: "袋", OrderUnit: "袋", UnitConversionJSON: "{}", Active: &active,
	})
	if err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "旧商品规格写入已停用") {
		t.Fatalf("SaveProductUnitTemplate after full cutover error=%v, want retired validation", err)
	}
	if err := repo.DeleteProductUnitTemplate(ctx, catalogapp.DeleteProductUnitTemplateCommand{Actor: "guard-test", ID: template.ID}); err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "旧商品规格写入已停用") {
		t.Fatalf("DeleteProductUnitTemplate after full cutover error=%v, want retired validation", err)
	}
	if _, err := repo.CreateSKU(ctx, catalogapp.CreateSKUCommand{Actor: "guard-test", Name: "late legacy product", Active: true}); err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "旧商品规格写入已停用") {
		t.Fatalf("CreateSKU after full cutover error=%v, want retired validation", err)
	}
	if err := repo.UpdateProductBasics(ctx, catalogapp.UpdateProductBasicsCommand{
		Actor: "guard-test", ProductID: 300, Name: "cutover product A", UnitTemplateID: template.ID,
		SpecialAttrsJSON: "{}", UnitRuleOverrideJSON: "{}",
	}); err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "BOM 规格") {
		t.Fatalf("UpdateProductBasics changed a cutover unit template: %v", err)
	}

	var activeTemplate bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active FROM %s.product_unit_templates WHERE id=$1`, schema), template.ID).Scan(&activeTemplate); err != nil {
		t.Fatal(err)
	}
	if !activeTemplate {
		t.Fatal("retired delete changed the legacy template")
	}
}

func TestCreateProductStartsBOMSpecPreparationWithoutReopeningRetiredLegacyWritesPostgres(t *testing.T) {
	ctx, pool, schema := newProductSpecCutoverCatalogTestDB(t)
	var templateID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_unit_templates(
			name,inventory_unit,quote_unit,order_unit,unit_conversion_json,sales_specs_json,integer_unit,active
		) VALUES(
			'legacy bag template','袋','袋','袋','{}'::jsonb,
			'[{"spec_key":"bag","spec_name":"袋装","sales_unit":"袋","active":true,"default":true}]'::jsonb,
			false,true
		) RETURNING id
	`, schema)).Scan(&templateID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,parent_product_id,base_product_id) VALUES
			(400,'legacy product A',true,0,0),(401,'legacy product B',true,0,0);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES
			(400,'cutover'),(401,'cutover');
	`, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	created, err := repo.CreateProduct(ctx, catalogapp.CreateProductCommand{
		Actor:                "guard-test",
		Name:                 "new BOM product",
		ProductKind:          "roasted",
		UnitTemplateID:       templateID,
		SpecialAttrsJSON:     "{}",
		UnitRuleOverrideJSON: `{"inventory_unit":"袋"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var unitTemplateID int64
	var migrationState string
	var legacyCatalogProduct bool
	var unitRuleOverrideJSON string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(product.unit_template_id,0),migration.state,
		       product.unit_rule_override_json::text,
		       COALESCE((to_jsonb(migration)->>'legacy_catalog_product')::boolean,true)
		FROM %[1]s.products product
		JOIN %[1]s.product_bom_spec_migrations migration ON migration.product_id=product.id
		WHERE product.id=$1
	`, schema), created.ID).Scan(&unitTemplateID, &migrationState, &unitRuleOverrideJSON, &legacyCatalogProduct); err != nil {
		t.Fatalf("load new product migration: %v", err)
	}
	if unitTemplateID != 0 || migrationState != "preparing" || legacyCatalogProduct {
		t.Fatalf("new product authority template=%d state=%q legacy=%v, want 0/preparing/false", unitTemplateID, migrationState, legacyCatalogProduct)
	}
	if !strings.Contains(unitRuleOverrideJSON, `"inventory_unit": "袋"`) {
		t.Fatalf("new direct product lost inventory unit: %s", unitRuleOverrideJSON)
	}
	var derivedChildren int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.products WHERE parent_product_id=$1`, schema), created.ID).Scan(&derivedChildren); err != nil {
		t.Fatal(err)
	}
	if derivedChildren != 0 {
		t.Fatalf("new BOM product derived legacy children=%d, want 0", derivedChildren)
	}
	active := true
	_, err = repo.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
		Actor: "guard-test", Name: "must remain retired", InventoryUnit: "袋",
		QuoteUnit: "袋", OrderUnit: "袋", UnitConversionJSON: "{}", Active: &active,
	})
	if err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "旧商品规格写入已停用") {
		t.Fatalf("new BOM product reopened legacy writes: %v", err)
	}
	var prepareAudits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='product_bom_spec_migration' AND entity_id=$1 AND action='prepare_new_product'
	`, schema), created.ID).Scan(&prepareAudits); err != nil {
		t.Fatal(err)
	}
	if prepareAudits != 1 {
		t.Fatalf("new product preparation audits=%d, want 1", prepareAudits)
	}
}

func TestFinalCutoverCompletesBeforeQueuedLegacyTemplateWritePostgres(t *testing.T) {
	ctx, pool, schema := newProductSpecCutoverCatalogTestDB(t)
	const finalProductID int64 = 880001
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active,parent_product_id,base_product_id) VALUES
			(880000,'already cut over',true,0,0),(%[2]d,'last legacy product',true,0,0);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES
			(880000,'cutover'),(%[2]d,'preparing');
		INSERT INTO %[1]s.production_boms(id,code,name,status) VALUES(881000,'BOM-LAST','last BOM','active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
			VALUES(881001,881000,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
			VALUES('product',%[2]d,881000,881001,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
			VALUES(881002,881000,'default','默认规格','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order
		) VALUES(881003,881001,881002,'默认规格','袋',true,1);
	`, schema, finalProductID)); err != nil {
		t.Fatal(err)
	}

	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, finalProductID); err != nil {
		t.Fatal(err)
	}

	migrationRepo := postgresproductspecmigration.NewRepository(pool, schema)
	cutoverDone := make(chan error, 1)
	go func() {
		_, cutoverErr := migrationRepo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: finalProductID, Actor: "guard-test"})
		cutoverDone <- cutoverErr
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pg_locks
				WHERE locktype='advisory' AND classid=0 AND objid=$1 AND objsubid=1 AND granted=false
			)
		`, finalProductID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cutover did not acquire the retirement lock before waiting on the parent migration lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	catalogRepo := NewRepository(pool, schema)
	active := true
	legacyWriteDone := make(chan error, 1)
	go func() {
		_, writeErr := catalogRepo.SaveProductUnitTemplate(ctx, catalogapp.SaveProductUnitTemplateCommand{
			Actor: "guard-test", Name: "late legacy template", InventoryUnit: "袋",
			QuoteUnit: "袋", OrderUnit: "袋", UnitConversionJSON: "{}", Active: &active,
		})
		legacyWriteDone <- writeErr
	}()
	select {
	case err := <-legacyWriteDone:
		t.Fatalf("legacy template write bypassed in-flight final cutover: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-cutoverDone:
		if err != nil {
			t.Fatalf("final cutover: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("final cutover timed out")
	}
	select {
	case err := <-legacyWriteDone:
		if err == nil || !catalogapp.IsValidationError(err) || !strings.Contains(err.Error(), "旧商品规格写入已停用") {
			t.Fatalf("queued legacy write after final cutover error=%v, want retired validation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued legacy write timed out")
	}
}

func newProductSpecCutoverCatalogTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
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
	schema := fmt.Sprintf("pr600_catalog_cutover_guard_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,
			old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %s.product_bom(
			product_id BIGINT PRIMARY KEY,yield_rate NUMERIC NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active',updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.product_bom_items(id BIGSERIAL PRIMARY KEY,product_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %s.product_bom_sources(
			product_id BIGINT PRIMARY KEY,source_type TEXT NOT NULL DEFAULT 'owned',
			source_product_id BIGINT NOT NULL DEFAULT 0,source_product_code_snapshot TEXT NOT NULL DEFAULT '',
			source_product_name_snapshot TEXT NOT NULL DEFAULT '',source_bom_version_id BIGINT NOT NULL DEFAULT 0,
			source_bom_version_no_snapshot TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.product_production_bom_bindings(
			product_id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL DEFAULT 0,bom_version_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %s.production_bom_groups(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.production_boms(
			id BIGINT PRIMARY KEY,group_id BIGINT NOT NULL DEFAULT 0,code TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'active'
		);
		CREATE TABLE %s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL DEFAULT 0,version_no TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',published_at TIMESTAMPTZ
		);
		CREATE TABLE %s.production_bom_version_items(id BIGSERIAL PRIMARY KEY,version_id BIGINT NOT NULL DEFAULT 0)
	`, schema, schema, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.production_bom_output_bindings(
			output_type TEXT NOT NULL DEFAULT '',output_id BIGINT NOT NULL DEFAULT 0,
			bom_id BIGINT NOT NULL DEFAULT 0,bom_version_id BIGINT NOT NULL DEFAULT 0,
			is_default BOOLEAN NOT NULL DEFAULT false,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL DEFAULT 0,code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '',
			spec_key TEXT NOT NULL DEFAULT '',name TEXT NOT NULL DEFAULT '',inventory_unit TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL DEFAULT 0,bom_spec_id BIGINT NOT NULL DEFAULT 0,
			spec_name_snapshot TEXT NOT NULL DEFAULT '',inventory_unit TEXT NOT NULL DEFAULT '',
			is_default BOOLEAN NOT NULL DEFAULT false,sort_order INTEGER NOT NULL DEFAULT 100,
			material_loss_rate NUMERIC NOT NULL DEFAULT 0,process_route_id BIGINT NOT NULL DEFAULT 0
		)
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, schema
}
