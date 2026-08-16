package production

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionRunSchemaFailsClosedWhenBOMSpecOutputIdentityIndexCannotBeCreated(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_production_run_schema_failclosed_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	for _, statement := range []string{
		fmt.Sprintf(`CREATE TABLE %s.produce_running_items(
			id BIGINT PRIMARY KEY,batch_id TEXT NOT NULL DEFAULT '',product_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,need_g BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.produce_running_outputs(
			id BIGSERIAL PRIMARY KEY,running_item_id BIGINT NOT NULL,product_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE INDEX produce_running_outputs_identity_uq
			ON %s.produce_running_outputs(running_item_id,product_id,spec_g)`, schema),
		fmt.Sprintf(`INSERT INTO %s.produce_running_outputs(running_item_id,product_id,spec_g)
			VALUES(1,2,0),(1,2,0)`, schema),
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if err := ensureProductionRunTable(ctx, pool, schema); err == nil {
		t.Fatal("production run schema accepted an uninstalled BOM-spec output identity index")
	}
}

func TestProductionRunSchemaKeepsBOMSpecIdentityOnRepeat(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_production_run_schema_repeat_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	if err := ensureProductionRunTable(ctx, pool, schema); err != nil {
		t.Fatalf("first schema pass: %v", err)
	}
	if err := ensureProductionRunTable(ctx, pool, schema); err != nil {
		t.Fatalf("second schema pass: %v", err)
	}
	var unique bool
	if err := pool.QueryRow(ctx, `
		SELECT i.indisunique
		FROM pg_index i
		JOIN pg_class c ON c.oid=i.indexrelid
		JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname=$1 AND c.relname='produce_running_outputs_identity_uq'
	`, schema).Scan(&unique); err != nil {
		t.Fatalf("read output identity index: %v", err)
	}
	if !unique {
		t.Fatal("production output identity index must be unique")
	}
}
