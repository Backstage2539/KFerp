package manufacturing

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
		"DELETE FROM %[1]s.product_production_config_fields",
		"product_production_configs",
		"c.industry_field_template_id=$1",
		"d.template_id=$1",
		"btrim(d.field_key)=COALESCE(NULLIF(btrim(f.template_field_key),''),btrim(f.field_key))",
		"RowsAffected()",
	} {
		if !strings.Contains(cleanup, want) {
			t.Fatalf("template-change product field cleanup missing %q", want)
		}
	}
	if strings.Contains(cleanup, "lower(btrim") {
		t.Fatal("template-change product field cleanup must keep exact case-sensitive key semantics")
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
	template_field_key TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.industry_field_definitions (
	id BIGINT PRIMARY KEY,
	template_id BIGINT NOT NULL,
	field_key TEXT NOT NULL DEFAULT ''
);

INSERT INTO %[1]s.product_production_configs(product_id, industry_field_template_id) VALUES
	(101,10),
	(102,20),
	(103,0);
INSERT INTO %[1]s.industry_field_definitions(id, template_id, field_key) VALUES
	(1,10,'exact-key'),
	(2,10,'fallback-key'),
	(3,10,'CaseKey'),
	(4,20,'other-key');
INSERT INTO %[1]s.product_production_config_fields(id, product_id, field_key, template_field_key) VALUES
	(1,101,'renamed-old-key','renamed-old-key'),
	(2,101,'ignored-exact-key',' exact-key '),
	(3,101,' fallback-key ','   '),
	(4,101,'ignored-case-key','casekey'),
	(5,102,'template-external-but-unrelated','template-external-but-unrelated'),
	(6,103,'untemplated-but-unrelated','untemplated-but-unrelated');
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
	if removed != 2 {
		_ = tx.Rollback(ctx)
		t.Fatalf("removed product field count = %d, want 2", removed)
	}
	assertManufacturingProductFieldIDs(t, ctx, tx, schema, []int64{2, 3, 5, 6})
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback cleanup transaction: %v", err)
	}
	assertManufacturingProductFieldIDs(t, ctx, pool, schema, []int64{1, 2, 3, 4, 5, 6})
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
