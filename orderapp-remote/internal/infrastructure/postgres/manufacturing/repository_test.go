package manufacturing

import (
	"context"
	"fmt"
	manufacturingapp "orderapp/internal/application/manufacturing"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPR563PieceCapacityPersistsAndAuditsInPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for manufacturing postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_pr563_piece_capacity_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema first run: %v", err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema second run must be idempotent: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT NULL,
			action TEXT NOT NULL DEFAULT '',
			field TEXT NULL,
			old_value TEXT NULL,
			new_value TEXT NULL,
			meta JSONB NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`, schema)); err != nil {
		t.Fatalf("create audit logs: %v", err)
	}

	var workstationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.manufacturing_workstations(code,name,status,hourly_rate)
		VALUES('PACK-PR563','包装工位','active',90)
		RETURNING id
	`, schema)).Scan(&workstationID); err != nil {
		t.Fatalf("insert workstation: %v", err)
	}

	repo := NewRepository(pool, schema)
	saved, err := repo.SaveManufacturingWorkstationCapacity(ctx, manufacturingapp.SaveWorkstationCapacityCommand{
		WorkstationID:      workstationID,
		Code:               "PACK-100",
		Name:               "包装100件",
		Status:             "active",
		BatchSizeQty:       100,
		BatchSizeUnit:      "件",
		StandardMinutes:    20,
		CostMethod:         "piece",
		PieceRate:          0.5,
		ProductionCapacity: 1,
		Actor:              "pr563-test",
	})
	if err != nil {
		t.Fatalf("save piece capacity: %v", err)
	}
	if saved.CostMethod != "piece" || saved.PieceRate != 0.5 || saved.BatchSizeUnit != "件" {
		t.Fatalf("saved piece capacity = %+v", saved)
	}

	if err := repo.DeactivateManufacturingWorkstationCapacity(ctx, manufacturingapp.TemplateStatusCommand{
		ID:    saved.ID,
		Actor: "pr563-test",
	}); err != nil {
		t.Fatalf("deactivate piece capacity: %v", err)
	}
	var status, meta string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT c.status, COALESCE(a.meta::text,'')
		FROM %s.manufacturing_workstation_capacities c
		JOIN %s.audit_logs a ON a.entity_id=c.id
		WHERE c.id=$1
		  AND a.entity_type='manufacturing_workstation_capacity'
		  AND a.action='deactivate'
		ORDER BY a.id DESC
		LIMIT 1
	`, schema, schema), saved.ID).Scan(&status, &meta); err != nil {
		t.Fatalf("load deactivation audit: %v", err)
	}
	if status != "inactive" {
		t.Fatalf("capacity status = %q, want inactive", status)
	}
	for _, want := range []string{`"cost_method": "piece"`, `"piece_rate": 0.5`, `"rate_unit": "sales_spec_count"`, `"batch_size_unit": "件"`} {
		if !strings.Contains(meta, want) {
			t.Fatalf("deactivation audit meta %s missing %q", meta, want)
		}
	}
}

func TestManufacturingRepositoryKeepsOperationCostAndWorkstationApplicability(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"standard_operation_cost",
		"manufacturing_workstation_operations",
		"attachWorkstationOperations",
		"applicable_operation_ids",
		"sc.id=pro.standard_cost_capacity_id",
		"StandardCostSummary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("manufacturing repository missing operation-cost/workstation-applicability marker %q", want)
		}
	}
}

func TestSaveIndustryTemplateCleansReferencedProductFieldsBeforeAuditAndCommit(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	save := manufacturingRepositoryFunctionForTest(t, string(src), "func (r Repository) SaveIndustryTemplate", "func (r Repository) industryTemplateByID")

	definitionInsert := strings.Index(save, "INSERT INTO %s.industry_field_definitions")
	cleanupCall := strings.Index(save, "cleanupProductProductionConfigFieldsForIndustryTemplateTx(ctx, tx, r.schema, id)")
	auditCall := strings.Index(save, "postgresinfra.AuditInsertTx")
	commitCall := strings.Index(save, "tx.Commit(ctx)")
	if cleanupCall < 0 {
		t.Fatal("SaveIndustryTemplate must clean product fields after rebuilding industry definitions")
	}
	if definitionInsert < 0 || cleanupCall <= definitionInsert {
		t.Fatal("SaveIndustryTemplate must clean product fields only after rebuilding industry definitions")
	}
	if auditCall < 0 || cleanupCall >= auditCall {
		t.Fatal("SaveIndustryTemplate must clean product fields before writing the template-save audit")
	}
	if commitCall < 0 || cleanupCall >= commitCall {
		t.Fatal("SaveIndustryTemplate must clean product fields before committing the template transaction")
	}
	if !strings.Contains(save, `"removed_product_field_count": removedProductFieldCount`) {
		t.Fatal("SaveIndustryTemplate audit must record the number of product fields removed by template cleanup")
	}

	cleanup := manufacturingRepositoryFunctionForTest(t, string(src), "func cleanupProductProductionConfigFieldsForIndustryTemplateTx", "func (r Repository) industryTemplateByID")
	for _, want := range []string{
		"ranked_definitions AS",
		"WHERE key_rank=1",
		"SET field_key=winner.field_key",
		"label=winner.label",
		"options_json=winner.options_json",
		"DELETE FROM %[1]s.product_production_config_fields",
		"product_production_config_industry_templates edited",
		"edited.template_id=$1",
		"product_production_config_industry_templates selected",
		"d.template_id=selected.template_id",
		"lower(btrim(d.field_key))=lower(COALESCE(NULLIF(btrim(f.template_field_key),''),btrim(f.field_key)))",
		"RowsAffected()",
	} {
		if !strings.Contains(cleanup, want) {
			t.Fatalf("template-change product field cleanup missing %q", want)
		}
	}
}

func TestCleanupProductProductionConfigFieldsForIndustryTemplateTx(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for manufacturing postgres tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_manufacturing_template_cleanup_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	mustExec := func(query string) {
		t.Helper()
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatalf("exec manufacturing template cleanup SQL: %v", err)
		}
	}
	mustExec(fmt.Sprintf(`
CREATE SCHEMA %[1]s;
CREATE TABLE %[1]s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	industry_field_template_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.product_production_config_fields (
	id BIGINT PRIMARY KEY,
	product_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	template_field_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	field_type TEXT NOT NULL DEFAULT 'text',
	unit TEXT NOT NULL DEFAULT '',
	required BOOLEAN NOT NULL DEFAULT false,
	options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	sort_order INT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.product_production_config_industry_templates (
	product_id BIGINT NOT NULL,
	template_id BIGINT NOT NULL,
	sort_order INT NOT NULL DEFAULT 1,
	PRIMARY KEY(product_id, template_id)
);
CREATE TABLE %[1]s.industry_field_definitions (
	id BIGINT PRIMARY KEY,
	template_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	field_type TEXT NOT NULL DEFAULT 'text',
	unit TEXT NOT NULL DEFAULT '',
	required BOOLEAN NOT NULL DEFAULT false,
	options_json JSONB NOT NULL DEFAULT '[]'::jsonb,
	sort_order INT NOT NULL DEFAULT 0
);

INSERT INTO %[1]s.product_production_configs(product_id, industry_field_template_id) VALUES
	(101,10),
	(102,20),
	(103,0);
INSERT INTO %[1]s.product_production_config_industry_templates(product_id, template_id, sort_order) VALUES
	(101,10,1),
	(101,20,2),
	(102,20,1);
INSERT INTO %[1]s.industry_field_definitions(id, template_id, field_key, label, sort_order) VALUES
	(1,10,'exact-key','Exact A',1),
	(2,10,'fallback-key','Fallback A',2),
	(3,10,'CaseKey','Case A',3),
	(4,20,'other-key','Other B',1),
	(5,20,'shared-key','Shared B',2);
INSERT INTO %[1]s.product_production_config_fields(id, product_id, field_key, template_field_key, label) VALUES
	(1,101,'renamed-old-key','renamed-old-key',''),
	(2,101,'ignored-exact-key',' exact-key ',''),
	(3,101,' fallback-key ','   ',''),
	(4,101,'ignored-case-key','casekey',''),
	(5,102,'template-external-but-unrelated','template-external-but-unrelated',''),
	(6,103,'untemplated-but-unrelated','untemplated-but-unrelated',''),
	(7,101,'shared-key','shared-key','Old A');
`, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin cleanup transaction: %v", err)
	}
	removed, err := cleanupProductProductionConfigFieldsForIndustryTemplateTx(ctx, tx, schema, 10)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("cleanupProductProductionConfigFieldsForIndustryTemplateTx: %v", err)
	}
	if removed != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("removed product field count = %d, want 1", removed)
	}
	assertManufacturingProductFieldIDs(t, ctx, tx, schema, []int64{2, 3, 4, 5, 6, 7})
	var sharedLabel, canonicalCaseKey string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT label FROM %s.product_production_config_fields WHERE id=7`, schema)).Scan(&sharedLabel); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("load shared field metadata: %v", err)
	}
	if sharedLabel != "Shared B" {
		_ = tx.Rollback(ctx)
		t.Fatalf("shared field label=%q, want remaining template B metadata", sharedLabel)
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT field_key FROM %s.product_production_config_fields WHERE id=4`, schema)).Scan(&canonicalCaseKey); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("load case-insensitive field key: %v", err)
	}
	if canonicalCaseKey != "CaseKey" {
		_ = tx.Rollback(ctx)
		t.Fatalf("canonical field key=%q, want first selected definition key CaseKey", canonicalCaseKey)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback cleanup transaction: %v", err)
	}
	assertManufacturingProductFieldIDs(t, ctx, pool, schema, []int64{1, 2, 3, 4, 5, 6, 7})
}

type manufacturingProductFieldIDQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func assertManufacturingProductFieldIDs(t *testing.T, ctx context.Context, db manufacturingProductFieldIDQuerier, schema string, want []int64) {
	t.Helper()
	rows, err := db.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.product_production_config_fields ORDER BY id`, schema))
	if err != nil {
		t.Fatalf("query manufacturing product field ids: %v", err)
	}
	defer rows.Close()
	got := make([]int64, 0, len(want))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan manufacturing product field id: %v", err)
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate manufacturing product field ids: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("manufacturing product field ids = %v, want %v", got, want)
	}
}

func manufacturingRepositoryFunctionForTest(t *testing.T, src, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(src, startMarker)
	if start < 0 {
		t.Fatalf("repository source missing function marker %q", startMarker)
	}
	end := strings.Index(src[start+len(startMarker):], endMarker)
	if end < 0 {
		t.Fatalf("repository source missing function end marker %q", endMarker)
	}
	return src[start : start+len(startMarker)+end]
}
