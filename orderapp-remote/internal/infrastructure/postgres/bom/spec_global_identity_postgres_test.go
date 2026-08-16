package bom

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionBOMSpecCodeAndBarcodeDoNotCollideWithLegacySKUPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_spec_global_identity_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY, sku_code TEXT NOT NULL DEFAULT '', barcode TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.production_bom_specs(
			id BIGINT PRIMARY KEY, code TEXT NOT NULL DEFAULT '', barcode TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO %[1]s.products(id,sku_code,barcode) VALUES(1,'SKU-LEGACY-001','690000000001');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := ensureProductionBomSpecGlobalIdentityGuards(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{"new spec code cannot reuse legacy child SKU code", fmt.Sprintf(`INSERT INTO %s.production_bom_specs(id,code) VALUES(1,'sku-legacy-001')`, schema), "bom_spec_code_conflicts_legacy_sku"},
		{"new spec barcode cannot reuse legacy child barcode", fmt.Sprintf(`INSERT INTO %s.production_bom_specs(id,code,barcode) VALUES(2,'BOM-SPEC-000002','690000000001')`, schema), "bom_spec_barcode_conflicts_legacy_sku"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.query)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_specs(id,code,barcode) VALUES(3,'BOM-SPEC-000003','690000000003')`, schema)); err != nil {
		t.Fatalf("insert new BOM spec: %v", err)
	}
	for _, tc := range []struct {
		name  string
		query string
		want  string
	}{
		{"legacy SKU code cannot claim new BOM spec code", fmt.Sprintf(`INSERT INTO %s.products(id,sku_code) VALUES(2,'bom-spec-000003')`, schema), "legacy_sku_code_conflicts_bom_spec"},
		{"legacy SKU barcode cannot claim new BOM spec barcode", fmt.Sprintf(`INSERT INTO %s.products(id,sku_code,barcode) VALUES(3,'SKU-LEGACY-003','690000000003')`, schema), "legacy_sku_barcode_conflicts_bom_spec"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.query)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %[1]s.test_delay_global_identifier_insert()
		RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_sleep(0.4);
			RETURN NEW;
		END $$;
		CREATE TRIGGER zz_test_delay_global_identifier_insert
			BEFORE INSERT ON %[1]s.production_bom_specs
			FOR EACH ROW EXECUTE FUNCTION %[1]s.test_delay_global_identifier_insert();
		CREATE TRIGGER zz_test_delay_global_identifier_insert
			BEFORE INSERT ON %[1]s.products
			FOR EACH ROW EXECUTE FUNCTION %[1]s.test_delay_global_identifier_insert();
	`, schema)); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name         string
		specInsert   string
		legacyInsert string
		identifier   string
	}{
		{
			name:         "concurrent code claim",
			specInsert:   fmt.Sprintf(`INSERT INTO %s.production_bom_specs(id,code) VALUES(101,'SHARED-CONCURRENT-CODE')`, schema),
			legacyInsert: fmt.Sprintf(`INSERT INTO %s.products(id,sku_code) VALUES(101,'shared-concurrent-code')`, schema),
			identifier:   "code",
		},
		{
			name:         "concurrent barcode claim",
			specInsert:   fmt.Sprintf(`INSERT INTO %s.production_bom_specs(id,code,barcode) VALUES(102,'BOM-SPEC-CONCURRENT-102','699999999999')`, schema),
			legacyInsert: fmt.Sprintf(`INSERT INTO %s.products(id,sku_code,barcode) VALUES(102,'SKU-CONCURRENT-102','699999999999')`, schema),
			identifier:   "barcode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := make(chan struct{})
			errs := make([]error, 2)
			queries := []string{tc.specInsert, tc.legacyInsert}
			var wg sync.WaitGroup
			for index, query := range queries {
				wg.Add(1)
				go func(index int, query string) {
					defer wg.Done()
					<-start
					_, errs[index] = pool.Exec(ctx, query)
				}(index, query)
			}
			close(start)
			wg.Wait()

			successes := 0
			conflicts := 0
			for _, err := range errs {
				switch {
				case err == nil:
					successes++
				case strings.Contains(err.Error(), "conflicts_legacy_sku") || strings.Contains(err.Error(), "conflicts_bom_spec"):
					conflicts++
				default:
					t.Fatalf("unexpected concurrent %s error: %v", tc.identifier, err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("concurrent %s successes/conflicts=%d/%d errors=%v, want 1/1", tc.identifier, successes, conflicts, errs)
			}
		})
	}
}

func TestProductionBOMSpecGlobalIdentityPreflightFailsClosedAndAllowsMissingLegacyProductsPostgres(t *testing.T) {
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

	t.Run("missing legacy products table remains compatible", func(t *testing.T) {
		schema := fmt.Sprintf("pr600_spec_identity_no_products_%d_%d", os.Getpid(), time.Now().UnixNano())
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			CREATE SCHEMA %[1]s;
			CREATE TABLE %[1]s.production_bom_specs(id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '');
		`, schema)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
		if err := ensureProductionBomSpecGlobalIdentityGuards(ctx, pool, schema); err != nil {
			t.Fatalf("missing products compatibility: %v", err)
		}
	})

	t.Run("existing cross-table collision blocks guard installation", func(t *testing.T) {
		schema := fmt.Sprintf("pr600_spec_identity_preflight_%d_%d", os.Getpid(), time.Now().UnixNano())
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			CREATE SCHEMA %[1]s;
			CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,sku_code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '');
			CREATE TABLE %[1]s.production_bom_specs(id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '');
			INSERT INTO %[1]s.products(id,sku_code,barcode) VALUES(1,'DUPLICATE-CODE','');
			INSERT INTO %[1]s.production_bom_specs(id,code,barcode) VALUES(1,'duplicate-code','');
		`, schema)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
		err := ensureProductionBomSpecGlobalIdentityGuards(ctx, pool, schema)
		if err == nil || !strings.Contains(err.Error(), "existing legacy SKU code/barcode conflicts") {
			t.Fatalf("preflight error=%v, want existing-collision failure", err)
		}
	})
}
