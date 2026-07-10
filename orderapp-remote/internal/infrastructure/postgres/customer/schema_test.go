package customer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	postgrescore "orderapp/internal/infrastructure/postgres/core"
)

func TestCustomerSchemaDefinesCustomerAssets(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{
		"func EnsureSchema(",
		"CREATE TABLE IF NOT EXISTS %[1]s.customer_assets",
		"REFERENCES %[1]s.customers(id) ON DELETE CASCADE",
		"UNIQUE(customer_id, kind)",
		"CREATE INDEX IF NOT EXISTS customer_assets_customer_id_idx",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer schema missing %q", want)
		}
	}
}

func TestCustomerEnsureSchemaCreatesCustomerAssets(t *testing.T) {
	ctx := context.Background()
	pool := newCustomerRepositoryTestPool(t)
	schema := fmt.Sprintf("customer_schema_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schema))
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customer.EnsureSchema: %v", err)
	}

	var tableName string
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1)::text`, schema+".customer_assets").Scan(&tableName); err != nil {
		t.Fatalf("load customer_assets table: %v", err)
	}
	if tableName != schema+".customer_assets" {
		t.Fatalf("customer_assets table=%q", tableName)
	}
}
