package bom

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"
	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublishProductBOMVersionGuardsOnlyRemovedSpecsPostgres(t *testing.T) {
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
	defer pool.Close()
	schema := fmt.Sprintf("pr600_same_bom_version_guard_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	ddl := []string{
		fmt.Sprintf(`CREATE TABLE %s.production_boms(
			id BIGINT PRIMARY KEY,output_type TEXT NOT NULL,output_product_id BIGINT NOT NULL DEFAULT 0,
			output_material_id BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL,process_route_id BIGINT NOT NULL DEFAULT 0,
			output_unit TEXT NOT NULL DEFAULT 'unit',published_at TIMESTAMPTZ,published_by TEXT NOT NULL DEFAULT '',
			source_spec_template_version_id BIGINT NOT NULL DEFAULT 0,main_input_material_id BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_spec_template_versions(
			id BIGINT PRIMARY KEY,status TEXT NOT NULL,published_at TIMESTAMPTZ
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.materials(
			id BIGINT PRIMARY KEY,deprecated_at TIMESTAMPTZ
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_operation_costs(version_id BIGINT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_output_bindings(
			output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_production_bom_bindings(
			bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,bound_at TIMESTAMPTZ,bound_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_production_configs(
			production_bom_id BIGINT NOT NULL,production_bom_version_id BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_bom_spec_migrations(product_id BIGINT PRIMARY KEY,state TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.finished_inventory(
			product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT 'finished_goods',
			onhand_units BIGINT NOT NULL DEFAULT 0,onhand_loose_g BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,
			old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
	}
	for _, statement := range ddl {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_boms VALUES(10,'product',42,0),(20,'product',43,0);
		INSERT INTO %[1]s.production_bom_spec_template_versions VALUES(900,'published',now());
		INSERT INTO %[1]s.materials VALUES(901,NULL);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status,source_spec_template_version_id,main_input_material_id) VALUES
			(11,10,'published',900,901),(12,10,'draft',900,901),(13,10,'draft',900,901),
			(14,10,'draft',900,901),(15,10,'draft',900,901),
			(21,20,'published',0,0),(22,20,'draft',0,0);
		INSERT INTO %[1]s.production_bom_specs VALUES
			(100,10,'227g'),(101,10,'454g'),(102,10,'100g');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,is_default) VALUES
			(200,11,100,true),(201,11,101,false),
			(210,12,100,true),(211,12,101,false),(212,12,102,false),
			(213,13,101,true),(214,13,102,false),
			(215,15,100,false),(216,15,101,false);
		INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',42,10,11,true,now(),'seed');
		INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',43,20,21,true,now(),'seed');
		INSERT INTO %[1]s.product_production_bom_bindings VALUES(10,11,now(),'seed');
		INSERT INTO %[1]s.product_production_bom_bindings VALUES(20,21,now(),'seed');
		INSERT INTO %[1]s.product_production_configs VALUES(10,11,now(),'seed');
		INSERT INTO %[1]s.product_production_configs VALUES(20,21,now(),'seed');
		INSERT INTO %[1]s.product_bom_spec_migrations VALUES(42,'cutover'),(43,'legacy');
		INSERT INTO %[1]s.finished_inventory VALUES(42,100,200,0,'finished_goods',1,0);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	publish := func(versionID int64) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := repo.publishProductionBomVersionTx(ctx, tx, bomapp.PublishProductionBomVersionCommand{VersionID: versionID, Actor: "tester"}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	if err := publish(14); err == nil || !strings.Contains(err.Error(), "at least one specification") {
		t.Fatalf("cutover same-BOM publish with no specifications error = %v", err)
	}
	if err := publish(15); err == nil || !strings.Contains(err.Error(), "exactly one default specification") {
		t.Fatalf("cutover same-BOM publish with no default specification error = %v", err)
	}
	if err := publish(22); err != nil {
		t.Fatalf("legacy same-BOM publish must keep empty specification-group compatibility: %v", err)
	}

	if err := publish(12); err != nil {
		t.Fatalf("publish retaining all existing specs: %v", err)
	}
	err = publish(13)
	var blocked *productspecmigrationapp.DefaultBOMSwitchBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("publish removing stocked spec err=%v, want default switch blocker", err)
	}
	var bindingVersion int64
	var candidateStatus, currentStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT binding.bom_version_id,candidate.status,current.status
		FROM %s.production_bom_output_bindings binding
		JOIN %s.production_bom_versions candidate ON candidate.id=13
		JOIN %s.production_bom_versions current ON current.id=12
		WHERE binding.output_type='product' AND binding.output_id=42 AND binding.is_default=true
	`, schema, schema, schema)).Scan(&bindingVersion, &candidateStatus, &currentStatus); err != nil {
		t.Fatal(err)
	}
	if bindingVersion != 12 || candidateStatus != "draft" || currentStatus != "published" {
		t.Fatalf("blocked publish mutated binding/version state: binding=%d candidate=%s current=%s", bindingVersion, candidateStatus, currentStatus)
	}
	var blockedAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='production_bom_version' AND entity_id=13 AND action='publish_production_bom_version'
	`, schema)).Scan(&blockedAuditCount); err != nil {
		t.Fatal(err)
	}
	if blockedAuditCount != 0 {
		t.Fatalf("blocked publish wrote %d audit rows", blockedAuditCount)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=0,onhand_loose_g=0`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := publish(13); err != nil {
		t.Fatalf("publish after removed-spec activity cleared: %v", err)
	}
}

func TestBindProductProductionBomUsesExplicitStructureAndSetsDirectIdentityPostgres(t *testing.T) {
	ctx, pool, schema := newPR598BomBindingTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name,active) VALUES(501,'规格商品',true),(502,'兼容商品',true);
		INSERT INTO %[1]s.production_boms(id,code,name,output_type,specification_mode,output_product_id,status) VALUES
			(601,'BOM-601','当前规格组','product','spec_group',501,'active'),
			(602,'BOM-602','候选空规格组','product','spec_group',501,'active'),
			(603,'BOM-603','兼容单一产出','product','single',502,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_unit,published_at,created_at) VALUES
			(701,601,'V001','published','袋',now()-interval '1 minute',now()-interval '1 minute'),
			(702,602,'V001','published','袋',now(),now()),
			(703,603,'V001','published','袋',now(),now());
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES
			(801,601,'BOM-SPEC-000801','227g','227g','袋');
		INSERT INTO %[1]s.production_bom_version_variants(version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)
			VALUES(701,801,'227g','袋',true);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
			VALUES('product',501,601,701,true,'seed');
		INSERT INTO %[1]s.product_production_bom_bindings(product_id,bom_id,bom_version_id,bound_by)
			VALUES(501,601,701,'seed');
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES(501,'cutover'),(502,'legacy');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{
		ProductID: 501, BomID: 602, Actor: "cutover-switch",
	}); err == nil || !strings.Contains(err.Error(), "at least one specification") {
		t.Fatalf("cutover switch to empty specification group error = %v", err)
	}
	var retainedBOMID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_id FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=501 AND is_default=true`, schema)).Scan(&retainedBOMID); err != nil {
		t.Fatal(err)
	}
	if retainedBOMID != 601 {
		t.Fatalf("rejected cutover switch changed default BOM to %d", retainedBOMID)
	}
	if _, err := repo.BindProductProductionBom(ctx, bomapp.BindProductProductionBomCommand{
		ProductID: 502, BomID: 603, Actor: "legacy-switch",
	}); err != nil {
		t.Fatalf("single-output product must bind without a specification group: %v", err)
	}
	var identityMode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT spec_identity_mode FROM %s.product_bom_spec_migrations WHERE product_id=502`, schema)).Scan(&identityMode); err != nil {
		t.Fatal(err)
	}
	if identityMode != "product" {
		t.Fatalf("single-output default identity mode=%q want product", identityMode)
	}
}
