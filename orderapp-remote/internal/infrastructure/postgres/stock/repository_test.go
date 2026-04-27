package stock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReceiveMaterialCreatesBatchAndLedger(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',
	field TEXT,
	old_value TEXT,
	new_value TEXT,
	meta JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',300);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	res, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       1200,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	if !strings.HasPrefix(res.BatchCode, "MB-") {
		t.Fatalf("batch code = %q, want MB-*", res.BatchCode)
	}

	var onhandG, remainingG, ledgerChange int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=$1`, schema), res.BatchID).Scan(&remainingG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_change_g FROM %s.stock_ledger_entries WHERE source_doc_type='material_receipt' AND source_doc_id=$1`, schema), res.ReceiptID).Scan(&ledgerChange); err != nil {
		t.Fatal(err)
	}
	if onhandG != 1500 || remainingG != 1200 || ledgerChange != 1200 {
		t.Fatalf("onhand/remaining/ledger = %d/%d/%d, want 1500/1200/1200", onhandG, remainingG, ledgerChange)
	}
}

func TestTransferMaterialMovesBatchLocationWithoutChangingTotalOnhand(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',
	field TEXT,
	old_value TEXT,
	new_value TEXT,
	meta JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',0);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       60000,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	transfer, err := repo.TransferMaterial(ctx, stockapp.MaterialTransferCommand{
		MaterialID:     1,
		FromWarehouse:  "raw_materials",
		ToWarehouse:    "wip",
		QtyG:           20000,
		Note:           "三天生产领料",
		Operator:       "jj",
		IdempotencyKey: "领料单-1",
	})
	if err != nil {
		t.Fatalf("TransferMaterial: %v", err)
	}
	if transfer.TransferNo == "" {
		t.Fatalf("transfer no is empty")
	}

	var onhandG, rawG, wipG, batchRemainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='raw_materials'`, schema), receipt.BatchID).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='wip'`, schema), receipt.BatchID).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=$1`, schema), receipt.BatchID).Scan(&batchRemainingG); err != nil {
		t.Fatal(err)
	}
	if onhandG != 60000 || rawG != 40000 || wipG != 20000 || batchRemainingG != 60000 {
		t.Fatalf("onhand/raw/wip/batch remaining = %d/%d/%d/%d, want 60000/40000/20000/60000", onhandG, rawG, wipG, batchRemainingG)
	}

	var outCount, inCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='material_transfer' AND source_doc_id=$1 AND warehouse='raw_materials' AND qty_change_g=-20000`, schema), transfer.TransferID).Scan(&outCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='material_transfer' AND source_doc_id=$1 AND warehouse='wip' AND qty_change_g=20000`, schema), transfer.TransferID).Scan(&inCount); err != nil {
		t.Fatal(err)
	}
	if outCount != 1 || inCount != 1 {
		t.Fatalf("ledger transfer entries = raw %d / wip %d, want 1/1", outCount, inCount)
	}
}

func TestMaterialAdjustmentBackfillCreatesTransferableRawBatch(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
);
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',0);
`, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	res, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType:  "material",
		ItemID:    1,
		Warehouse: "raw_materials",
		TargetG:   60000,
		Reason:    "期初库存补录",
		Operator:  "jj",
	})
	if err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}

	var batchRemainingG, rawG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE batch_code=$1`, schema), fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&batchRemainingG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE batch_code=$1 AND warehouse='raw_materials'`, schema), fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if batchRemainingG != 60000 || rawG != 60000 {
		t.Fatalf("backfill batch remaining/raw = %d/%d, want 60000/60000", batchRemainingG, rawG)
	}
}

func newStockTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for stock tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_stock_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func mustExecStockSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
