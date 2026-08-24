package catalog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	catalogapp "orderapp/internal/application/catalog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductUnitDefinitionRejectsBOMReferencedDeactivateAndReclassificationPostgres(t *testing.T) {
	ctx, pool, schema := newProductUnitBOMGuardTestDB(t)
	repository := NewRepository(pool, schema)
	active := true
	inactive := false

	for _, tc := range []struct {
		name string
		code string
	}{
		{name: "spec template", code: "袋"},
		{name: "published BOM spec", code: "盒"},
		{name: "archived BOM spec", code: "罐"},
	} {
		t.Run(tc.name+" reclassification", func(t *testing.T) {
			_, err := repository.SaveProductUnitDefinition(ctx, catalogapp.SaveProductUnitDefinitionCommand{
				Actor: "pr600-unit-guard", Code: tc.code, Name: tc.code, UnitType: "count", AllowDecimal: false, Active: &active,
			})
			assertRejectedProductUnitBOMMutation(t, ctx, pool, schema, tc.code, err)
		})
		t.Run(tc.name+" save inactive", func(t *testing.T) {
			_, err := repository.SaveProductUnitDefinition(ctx, catalogapp.SaveProductUnitDefinitionCommand{
				Actor: "pr600-unit-guard", Code: tc.code, Name: tc.code, UnitType: "package", AllowDecimal: false, Active: &inactive,
			})
			assertRejectedProductUnitBOMMutation(t, ctx, pool, schema, tc.code, err)
		})
		t.Run(tc.name+" delete", func(t *testing.T) {
			err := repository.DeleteProductUnitDefinition(ctx, catalogapp.DeleteProductUnitDefinitionCommand{
				Actor: "pr600-unit-guard", Code: tc.code,
			})
			assertRejectedProductUnitBOMMutation(t, ctx, pool, schema, tc.code, err)
		})
	}

	if _, err := repository.SaveProductUnitDefinition(ctx, catalogapp.SaveProductUnitDefinitionCommand{
		Actor: "pr600-unit-guard", Code: "袋", Name: "包装袋", UnitType: "package", AllowDecimal: true, Active: &active,
	}); err != nil {
		t.Fatalf("non-semantic edit of referenced unit: %v", err)
	}
	var name string
	var allowDecimal bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT name,allow_decimal FROM %s.product_unit_definitions WHERE code='袋'`, schema)).Scan(&name, &allowDecimal); err != nil {
		t.Fatal(err)
	}
	if name != "包装袋" || !allowDecimal {
		t.Fatalf("allowed referenced-unit metadata edit = %q/%v", name, allowDecimal)
	}

	if err := repository.DeleteProductUnitDefinition(ctx, catalogapp.DeleteProductUnitDefinitionCommand{
		Actor: "pr600-unit-guard", Code: "提",
	}); err != nil {
		t.Fatalf("delete unreferenced unit: %v", err)
	}
	var unreferencedActive bool
	var unreferencedDeleted bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,deleted_at IS NOT NULL FROM %s.product_unit_definitions WHERE code='提'`, schema)).Scan(&unreferencedActive, &unreferencedDeleted); err != nil {
		t.Fatal(err)
	}
	if unreferencedActive || !unreferencedDeleted {
		t.Fatalf("unreferenced unit delete state = active:%v deleted:%v", unreferencedActive, unreferencedDeleted)
	}
}

func assertRejectedProductUnitBOMMutation(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, code string, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "BOM specification") {
		t.Fatalf("BOM-referenced unit %q mutation error = %v", code, err)
	}
	var unitType string
	var active bool
	var deleted bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_type,active,deleted_at IS NOT NULL FROM %s.product_unit_definitions WHERE code=$1`, schema), code).Scan(&unitType, &active, &deleted); err != nil {
		t.Fatal(err)
	}
	if unitType != "package" || !active || deleted {
		t.Fatalf("rejected mutation changed %q to type=%q active=%v deleted=%v", code, unitType, active, deleted)
	}
	var rejectedAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='product_unit_definition' AND actor='pr600-unit-guard'
		  AND COALESCE(meta->>'code',new_value,'')=$1
	`, schema), code).Scan(&rejectedAuditCount); err != nil {
		t.Fatal(err)
	}
	if rejectedAuditCount != 0 {
		t.Fatalf("rejected mutation of %q wrote %d audit rows", code, rejectedAuditCount)
	}
}

func newProductUnitBOMGuardTestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
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
	schema := fmt.Sprintf("test_pr600_unit_bom_guard_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.product_unit_definitions(
			code TEXT PRIMARY KEY,name TEXT NOT NULL,unit_type TEXT NOT NULL DEFAULT 'other',
			allow_decimal BOOLEAN NOT NULL DEFAULT true,active BOOLEAN NOT NULL DEFAULT true,
			deleted_at TIMESTAMPTZ,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE %[1]s.materials(id BIGSERIAL PRIMARY KEY,code TEXT NOT NULL UNIQUE,unit TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_spec_template_versions(id BIGINT PRIMARY KEY,status TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_spec_template_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,inventory_unit TEXT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_versions(id BIGINT PRIMARY KEY,status TEXT NOT NULL);
		CREATE TABLE %[1]s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,inventory_unit TEXT NOT NULL
		);
		CREATE TABLE %[1]s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,
			field TEXT,old_value TEXT,new_value TEXT,meta JSONB
		);
		INSERT INTO %[1]s.product_unit_definitions(code,name,unit_type,allow_decimal,active) VALUES
			('袋','袋','package',false,true),('盒','盒','package',false,true),
			('罐','罐','package',false,true),('提','提','package',false,true);
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,status) VALUES(1,'draft');
		INSERT INTO %[1]s.production_bom_spec_template_variants(id,version_id,inventory_unit) VALUES(11,1,'袋');
		INSERT INTO %[1]s.production_bom_versions(id,status) VALUES(2,'published'),(3,'archived'),(4,'draft');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,inventory_unit) VALUES
			(21,2,201,'盒'),(31,3,301,'罐'),(41,4,401,'提');
	`, schema)); err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	return ctx, pool, schema
}
