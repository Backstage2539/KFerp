package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	bomapp "orderapp/internal/application/bom"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	supporthttp "orderapp/internal/interfaces/http/support"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublishProductionBomSpecTemplateVersionSerializesConcurrentDraftsPostgres(t *testing.T) {
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.product_unit_definitions(
			id BIGSERIAL PRIMARY KEY,
			code TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			deleted_at TIMESTAMPTZ
		);
		INSERT INTO %s.product_unit_definitions(code) VALUES('kg');

		INSERT INTO %s.production_bom_spec_templates(id,code,name,created_by,updated_by)
		VALUES(1,'BOM-SPEC-TPL-000001','并发发布模板','tester','tester');
		INSERT INTO %s.production_bom_spec_template_versions(
			id,template_id,version_no,status,created_at,published_at,created_by,published_by
		) VALUES
			(10,1,'V001','published',now()-interval '3 minutes',now()-interval '2 minutes','tester','tester'),
			(11,1,'V002','draft',now()-interval '1 minute',NULL,'tester',''),
			(12,1,'V003','draft',now()-interval '1 minute',NULL,'tester','');
		INSERT INTO %s.production_bom_spec_template_variants(
			id,version_id,spec_key,name,inventory_unit,is_default,sort_order,material_loss_rate
		) VALUES
			(100,10,'default','历史规格','kg',true,1,0),
			(101,11,'default','候选规格 A','kg',true,1,0),
			(102,12,'default','候选规格 B','kg',true,1,0);
		INSERT INTO %s.production_bom_spec_template_variant_items(
			variant_id,is_main_input,material_id,component_type,consume_unit,qty_per_unit,sort_order
		) VALUES
			(100,true,900,'material','kg',1,1),
			(101,true,901,'material','kg',1,1),
			(102,true,902,'material','kg',1,1);

		CREATE OR REPLACE FUNCTION %s.test_delay_spec_template_publish()
		RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF OLD.status='draft' AND NEW.status='published' THEN
				PERFORM pg_sleep(0.4);
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER zz_test_delay_spec_template_publish
			BEFORE UPDATE OF status ON %s.production_bom_spec_template_versions
			FOR EACH ROW EXECUTE FUNCTION %s.test_delay_spec_template_publish();
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(pool, schema)
	start := make(chan struct{})
	errs := make([]error, 2)
	versionIDs := []int64{11, 12}
	var wg sync.WaitGroup
	for index, versionID := range versionIDs {
		wg.Add(1)
		go func(index int, versionID int64) {
			defer wg.Done()
			<-start
			errs[index] = repo.PublishProductionBomSpecTemplateVersion(ctx, bomapp.PublishProductionBomSpecTemplateVersionCommand{
				VersionID: versionID,
				Actor:     fmt.Sprintf("publisher-%d", versionID),
			})
		}(index, versionID)
	}
	close(start)
	wg.Wait()

	successes := 0
	explicitConflicts := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case strings.Contains(err.Error(), "newer published specification template version"):
			explicitConflicts++
		default:
			t.Fatalf("unexpected concurrent publish error: %v", err)
		}
	}
	if successes != 1 || explicitConflicts != 1 {
		t.Fatalf("concurrent publish successes/conflicts=%d/%d errors=%v, want 1/1", successes, explicitConflicts, errs)
	}

	var publishedCount, publishAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.production_bom_spec_template_versions
		WHERE template_id=1 AND status='published'
	`, schema)).Scan(&publishedCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='production_bom_spec_template_version' AND action='publish'
	`, schema)).Scan(&publishAuditCount); err != nil {
		t.Fatal(err)
	}
	if publishedCount != 1 || publishAuditCount != 1 {
		t.Fatalf("published/audit counts=%d/%d, want 1/1", publishedCount, publishAuditCount)
	}
}

func TestProductionBomSpecTemplatePublishedConstraintMigrationFailsClosedPostgres(t *testing.T) {
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		DROP INDEX IF EXISTS %s.production_bom_spec_template_versions_one_published_uq;
		INSERT INTO %s.production_bom_spec_templates(id,code,name) VALUES(1,'BOM-SPEC-TPL-000001','历史冲突模板');
		INSERT INTO %s.production_bom_spec_template_versions(id,template_id,version_no,status)
		VALUES(10,1,'V001','published'),(11,1,'V002','published');
	`, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}

	if err := EnsureSchema(ctx, pool, schema); err == nil || !strings.Contains(err.Error(), "multiple published specification template versions") {
		t.Fatalf("EnsureSchema error=%v, want fail-closed duplicate-published preflight", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_spec_template_versions SET status='archived' WHERE id=10`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema after conflict repair: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_spec_template_versions SET status='published' WHERE id=10`, schema)); err == nil {
		t.Fatal("database constraint allowed a second published version for one specification template")
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("repeated EnsureSchema: %v", err)
	}
}

func TestUpdateProductionBomSpecTemplateDraftAuditsWholeGroupDifferencePostgres(t *testing.T) {
	ctx, pool, schema := newPR600SpecTemplatePublishTestDB(t)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.product_unit_definitions(
			id BIGSERIAL PRIMARY KEY,code TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true,deleted_at TIMESTAMPTZ
		);
		INSERT INTO %s.product_unit_definitions(code) VALUES('袋');
	`, schema, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	template, err := repo.CreateProductionBomSpecTemplate(ctx, bomapp.CreateProductionBomSpecTemplateCommand{Name: "审计模板", Actor: "template-auditor"})
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Versions) != 1 {
		t.Fatalf("created template versions=%d, want 1", len(template.Versions))
	}
	versionID := template.Versions[0].ID
	first := []bomapp.ProductionBomSpecTemplateVariant{
		{SpecKey: "227g", Name: "227g 旧规格", InventoryUnit: "袋", IsDefault: true, SortOrder: 1},
		{SpecKey: "454g", Name: "454g 规格", InventoryUnit: "袋", SortOrder: 2},
	}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: versionID, Variants: first, Actor: "template-auditor"}); err != nil {
		t.Fatal(err)
	}
	second := []bomapp.ProductionBomSpecTemplateVariant{
		{SpecKey: "227g", Name: "227g 新规格", InventoryUnit: "袋", IsDefault: true, SortOrder: 1},
	}
	if _, err := repo.UpdateProductionBomSpecTemplateVersionDraft(ctx, bomapp.UpdateProductionBomSpecTemplateVersionDraftCommand{VersionID: versionID, Variants: second, Actor: "template-auditor"}); err != nil {
		t.Fatal(err)
	}
	var oldValue, newValue string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT old_value,new_value FROM %s.audit_logs
		WHERE entity_type='production_bom_spec_template_version' AND entity_id=$1
		  AND action='update_specifications' AND field='variants'
		ORDER BY id DESC LIMIT 1
	`, schema), versionID).Scan(&oldValue, &newValue); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"227g 旧规格", "454g 规格"} {
		if !strings.Contains(oldValue, expected) {
			t.Fatalf("old specification-group audit missing %q: %s", expected, oldValue)
		}
	}
	if !strings.Contains(newValue, "227g 新规格") || strings.Contains(newValue, "454g 规格") {
		t.Fatalf("new specification-group audit does not show rename/removal: %s", newValue)
	}
}

func newPR600SpecTemplatePublishTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
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
	schema := fmt.Sprintf("pr600_spec_template_publish_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.products(
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			base_product_id BIGINT NOT NULL DEFAULT 0
		)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := supporthttp.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	return ctx, pool, schema
}
