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

func TestMaterialNeedToDeduct(t *testing.T) {
	cases := []struct {
		unit      string
		qty       int64
		wantG     int64
		wantUnits int64
	}{
		{unit: "g", qty: 250, wantG: 250},
		{unit: "kg", qty: 2, wantG: 2000},
		{unit: "克", qty: 300, wantG: 300},
		{unit: "个", qty: 7, wantUnits: 7},
		{unit: "张", qty: 5, wantUnits: 5},
	}

	for _, tc := range cases {
		gotG, gotUnits := materialNeedToDeduct(tc.unit, tc.qty)
		if gotG != tc.wantG || gotUnits != tc.wantUnits {
			t.Fatalf("materialNeedToDeduct(%q,%d) = %d/%d, want %d/%d", tc.unit, tc.qty, gotG, gotUnits, tc.wantG, tc.wantUnits)
		}
	}
}

func TestIsWeightMaterialUnit(t *testing.T) {
	for _, unit := range []string{"g", "kg", "克", "千克"} {
		if !isWeightMaterialUnit(unit) {
			t.Fatalf("expected %q to be weight unit", unit)
		}
	}
	if isWeightMaterialUnit("个") {
		t.Fatalf("expected 个 not to be weight unit")
	}
}

func TestMarshalMaterialConsumptionSummary(t *testing.T) {
	got, err := marshalMaterialConsumptionSummary([]materialConsumptionSummaryItem{
		{MaterialID: 1, MaterialName: "卡蒂姆水洗", Unit: "g", DeductG: 1200, BatchCode: "MB-0000000001"},
		{MaterialID: 9, MaterialName: "豆袋", Unit: "个", DeductUnits: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, needle := range []string{`"material_id":1`, `"material_name":"卡蒂姆水洗"`, `"deduct_g":1200`, `"batch_code":"MB-0000000001"`, `"material_name":"豆袋"`, `"deduct_units":8`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("material summary json missing %q in %s", needle, text)
		}
	}
}

func TestMaterialBatchAllocationsConsumeOnlyWIPLocation(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	receipt_id BIGINT NOT NULL DEFAULT 0,
	qty_g BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	note TEXT NOT NULL DEFAULT '',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,
	batch_code TEXT NOT NULL DEFAULT '',
	material_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(material_batch_id, warehouse)
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.material_batches(id,batch_code,material_id,qty_g,remaining_g,received_at)
VALUES (1,'MB-OLD',7,1000,1000,now() - interval '1 day');
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES (1,'MB-OLD',7,'raw_materials',1000);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,qty_g,remaining_g)
VALUES ('MB-OLD','material',7,'水洗豆',1000,1000);
`, schema, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = materialBatchAllocationsTx(ctx, tx, schema, 7, 600)
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("expected WIP stock insufficient error when only raw warehouse has the batch")
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %s.material_batch_locations SET qty_g=300 WHERE material_batch_id=1 AND warehouse='raw_materials';
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES (1,'MB-OLD',7,'wip',700);
`, schema, schema))

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	alloc, err := materialBatchAllocationsTx(ctx, tx, schema, 7, 600)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("materialBatchAllocationsTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if len(alloc) != 1 || alloc[0].BatchCode != "MB-OLD" || alloc[0].QtyG != 600 {
		t.Fatalf("alloc = %+v, want MB-OLD 600g", alloc)
	}

	var rawG, wipG, remainingG, stockBatchRemainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=1 AND warehouse='raw_materials'`, schema)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=1 AND warehouse='wip'`, schema)).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=1`, schema)).Scan(&remainingG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.stock_batches WHERE batch_code='MB-OLD'`, schema)).Scan(&stockBatchRemainingG); err != nil {
		t.Fatal(err)
	}
	if rawG != 300 || wipG != 100 || remainingG != 400 || stockBatchRemainingG != 400 {
		t.Fatalf("raw/wip/material/stock batch = %d/%d/%d/%d, want 300/100/400/400", rawG, wipG, remainingG, stockBatchRemainingG)
	}
}

func newProductionTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for production tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_production_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func mustExecProductionSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
