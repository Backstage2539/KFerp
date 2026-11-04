package bom

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
	supporthttp "orderapp/internal/interfaces/http/support"
	"os"
	"strings"
	"testing"
	"time"
)

// Keep startup migration coverage independent of the historical full BOM workflow fixture.
func TestProductionMaterialBackfillReleasePostgres(t *testing.T) {
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
	schema := fmt.Sprintf("release_material_%d_%d", os.Getpid(), time.Now().UnixNano())
	mustPR598SQL(t, ctx, pool, "CREATE SCHEMA "+schema)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	mustPR598SQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			active BOOLEAN NOT NULL DEFAULT true,
			base_product_id BIGINT NOT NULL DEFAULT 0
		);
	`, schema))
	if err := supporthttp.EnsureAuditTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresmaterials.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var outputConstraintOID int64
	var outputConstraintValidated bool
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint, c.convalidated
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&outputConstraintOID, &outputConstraintValidated); err != nil {
		t.Fatal(err)
	}
	if !outputConstraintValidated {
		t.Fatal("production_boms_output_binding_check must be validated")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(code,name,output_type,output_material_id,legacy_product_id)
		VALUES('BOM-PR598-LEGACY-MATERIAL','legacy material output','material',9001,949)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("repeated schema setup must preserve a material BOM with a legacy product id: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_boms WHERE code='BOM-PR598-LEGACY-MATERIAL'`, schema)); err != nil {
		t.Fatal(err)
	}
	var outputConstraintOIDAfter int64
	if err := pool.QueryRow(ctx, `
		SELECT c.oid::bigint
		FROM pg_constraint c
		JOIN pg_class t ON t.oid=c.conrelid
		JOIN pg_namespace n ON n.oid=t.relnamespace
		WHERE n.nspname=$1 AND t.relname='production_boms' AND c.conname='production_boms_output_binding_check'
	`, schema).Scan(&outputConstraintOIDAfter); err != nil {
		t.Fatal(err)
	}
	if outputConstraintOIDAfter != outputConstraintOID {
		t.Fatalf("output binding constraint was recreated: oid %d -> %d", outputConstraintOID, outputConstraintOIDAfter)
	}
}
