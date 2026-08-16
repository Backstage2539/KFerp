package productspecmigration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCustomerProductAliasRemainsParentLevelAfterBOMSpecCutoverPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_family_alias_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY);
		CREATE TABLE %[1]s.customer_product_aliases(id BIGINT PRIMARY KEY,product_id BIGINT NOT NULL);
		INSERT INTO %[1]s.products(id) VALUES(41);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(41,'cutover')`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_product_aliases(id,product_id) VALUES(1,41)`, schema)); err != nil {
		t.Fatalf("family-level alias must not require bom_spec_id after cutover: %v", err)
	}
	var triggerCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM pg_trigger trigger
		JOIN pg_class table_ref ON table_ref.oid=trigger.tgrelid
		JOIN pg_namespace namespace_ref ON namespace_ref.oid=table_ref.relnamespace
		WHERE namespace_ref.nspname=$1 AND table_ref.relname='customer_product_aliases'
		  AND trigger.tgname='bom_spec_identity_guard' AND NOT trigger.tgisinternal
	`, schema).Scan(&triggerCount); err != nil {
		t.Fatal(err)
	}
	if triggerCount != 0 {
		t.Fatalf("customer_product_aliases must not install bom_spec_identity_guard, got %d", triggerCount)
	}
}
