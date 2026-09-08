package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionBomMaterialOwnershipRulesPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr639_bom_owner_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,owner_customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.production_boms(id BIGINT PRIMARY KEY,main_input_material_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL);
		CREATE TABLE %[1]s.production_bom_version_items(id BIGSERIAL PRIMARY KEY,version_id BIGINT NOT NULL,component_type TEXT NOT NULL DEFAULT 'material',material_id BIGINT NOT NULL DEFAULT 0);
		INSERT INTO %[1]s.products(id,customer_id) VALUES(1,0),(2,74),(3,75);
		INSERT INTO %[1]s.materials(id,owner_customer_id) VALUES(10,0),(20,74),(30,75);
		INSERT INTO %[1]s.production_boms(id,main_input_material_id) VALUES(100,0),(200,10),(300,20);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id) VALUES(101,100),(201,200),(301,300);
	`, schema)); err != nil {
		t.Fatal(err)
	}

	assertRule := func(name string, versionID, outputProductID int64, materialIDs []int64, wantError string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_bom_version_items WHERE version_id=$1`, schema), versionID); err != nil {
				t.Fatal(err)
			}
			for _, materialID := range materialIDs {
				if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_items(version_id,material_id) VALUES($1,$2)`, schema), versionID, materialID); err != nil {
					t.Fatal(err)
				}
			}
			err := validateProductionBomMaterialOwnershipForPublish(ctx, pool, schema, versionID, "product", outputProductID, 0)
			if wantError == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wantError != "" && (err == nil || !strings.Contains(err.Error(), wantError)) {
				t.Fatalf("error=%v want %q", err, wantError)
			}
		})
	}
	assertRule("factory accepts factory", 101, 1, []int64{10}, "")
	assertRule("factory rejects customer A", 101, 1, []int64{20}, "本公司 BOM")
	assertRule("customer A accepts factory and A", 201, 2, []int64{10, 20}, "")
	assertRule("customer A rejects customer B", 201, 2, []int64{30}, "同一客户")
	assertRule("main input is also checked", 301, 1, nil, "本公司 BOM")
}
