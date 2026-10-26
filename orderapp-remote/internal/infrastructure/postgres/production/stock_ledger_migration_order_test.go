package production_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	productionpg "orderapp/internal/infrastructure/postgres/production"
	stockpg "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStockThenProductionLedgerMigrationsKeepManualAdjustmentSourcesRepeatable(t *testing.T) {
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
	schema := fmt.Sprintf("stock_production_migration_order_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '',onhand_g BIGINT NOT NULL DEFAULT 0,purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0)
	`, schema)); err != nil {
		t.Fatal(err)
	}

	if err := stockpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock schema migration: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,bom_spec_id,spec_g,source_doc_type,source_doc_id)
		VALUES('MANUAL-MIGRATION-1','finished_product',7,20,0,'manual_adjustment',0)
	`, schema)); err != nil {
		t.Fatalf("first manual adjustment batch: %v", err)
	}
	if err := productionpg.EnsureStockLedgerTables(ctx, pool, schema); err != nil {
		t.Fatalf("production ledger migration after stock migration: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(batch_code,item_type,item_id,bom_spec_id,spec_g,source_doc_type,source_doc_id)
		VALUES('MANUAL-MIGRATION-2','finished_product',7,20,0,'manual_adjustment',0)
	`, schema)); err != nil {
		t.Fatalf("repeated manual adjustment batch after production migration: %v", err)
	}
}
