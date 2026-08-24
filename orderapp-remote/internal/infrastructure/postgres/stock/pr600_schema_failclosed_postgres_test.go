package stock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnsureStockLedgerSchemaFailsClosedWhenBOMSpecIdentityIndexCannotBeCreated(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_stock_schema_failclosed_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	// Simulate a legacy table carrying duplicate source identity rows. The old
	// source-index name means CREATE INDEX IF NOT EXISTS in the first DDL pass
	// is skipped; replacing it with the BOM-spec identity index must fail rather
	// than leave a running application without that idempotency guarantee.
	for _, statement := range []string{
		fmt.Sprintf(`CREATE TABLE %s.stock_batches(
			id BIGSERIAL PRIMARY KEY,
			batch_code TEXT NOT NULL UNIQUE,
			item_type TEXT NOT NULL DEFAULT '',
			item_id BIGINT NOT NULL DEFAULT 0,
			spec_g BIGINT NOT NULL DEFAULT 0,
			source_doc_type TEXT NOT NULL DEFAULT '',
			source_doc_id BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE INDEX stock_batches_source_uq
			ON %s.stock_batches(source_doc_type,source_doc_id,item_type,item_id,spec_g)
			WHERE source_doc_type <> ''`, schema),
		fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,spec_g,source_doc_type,source_doc_id)
			VALUES ('A','product',1,0,'work_order',9),('B','product',1,0,'work_order',9)`, schema),
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	if err := ensureStockLedgerTables(ctx, pool, schema); err == nil {
		t.Fatal("EnsureSchema accepted an uninstalled BOM-spec stock source identity index")
	}
}

func TestEnsureStockLedgerSchemaKeepsBOMSpecIdentityIndexOnRepeat(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_stock_schema_repeat_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		t.Fatalf("first schema pass: %v", err)
	}
	if err := ensureStockLedgerTables(ctx, pool, schema); err != nil {
		t.Fatalf("second schema pass: %v", err)
	}
	var unique bool
	if err := pool.QueryRow(ctx, `
		SELECT i.indisunique
		FROM pg_index i
		JOIN pg_class c ON c.oid=i.indexrelid
		JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname=$1 AND c.relname='stock_batches_source_uq'
	`, schema).Scan(&unique); err != nil {
		t.Fatalf("read stock source identity index: %v", err)
	}
	if !unique {
		t.Fatal("stock source identity index must be unique")
	}
}
