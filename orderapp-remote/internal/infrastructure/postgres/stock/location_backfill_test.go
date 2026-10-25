package stock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLocationBackfillOnlyCreatesLocationForBatchesWithoutAnyLocation(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g,qty_units,updated_at)")
	if start < 0 {
		t.Fatal("material batch location backfill missing")
	}
	end := strings.Index(text[start:], "ON CONFLICT (material_batch_id, warehouse) DO NOTHING")
	if end < 0 {
		t.Fatal("material batch location backfill conflict clause missing")
	}
	backfill := text[start : start+end]
	if !strings.Contains(backfill, "NOT EXISTS") || !strings.Contains(backfill, "FROM %s.material_batch_locations") {
		t.Fatal("startup backfill must not recreate a batch location when the batch already has any warehouse location")
	}
}

func TestEnsureMaterialBatchTablesDoesNotDuplicateExistingWarehouseBalance(t *testing.T) {
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		pool.Close()
	})
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL,onhand_g BIGINT NOT NULL DEFAULT 0,onhand_units BIGINT NOT NULL DEFAULT 0,purchase_price NUMERIC NOT NULL DEFAULT 0);
		INSERT INTO %[1]s.materials(id,name,onhand_g) VALUES(1,'已有WIP批次',100),(2,'尚无库位批次',50);
		CREATE TABLE %[1]s.material_batches(id BIGINT PRIMARY KEY,batch_code TEXT UNIQUE NOT NULL,material_id BIGINT NOT NULL,supplier TEXT NOT NULL DEFAULT '',receipt_id BIGINT NOT NULL DEFAULT 0,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,unit_cost NUMERIC NOT NULL DEFAULT 0,crop_season TEXT NOT NULL DEFAULT '',origin TEXT NOT NULL DEFAULT '',producer_flavor_description TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'active',quality_status TEXT NOT NULL DEFAULT 'pass',owner_customer_id BIGINT NOT NULL DEFAULT 0,material_name TEXT NOT NULL DEFAULT '',received_g BIGINT NOT NULL DEFAULT 0,note TEXT NOT NULL DEFAULT '',received_at TIMESTAMPTZ NOT NULL DEFAULT now(),created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		INSERT INTO %[1]s.material_batches(id,batch_code,material_id,qty_g,remaining_g) VALUES(1,'MP-EXISTING-WIP',1,100,100),(2,'MP-MISSING-LOCATION',2,50,50);
		CREATE TABLE %[1]s.material_batch_locations(material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL DEFAULT '',material_id BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT '',qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse));
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(1,'MP-EXISTING-WIP',1,'wip',100);
	`, schema))
	if err := ensureMaterialBatchTables(ctx, pool, schema); err != nil {
		t.Fatalf("first EnsureSchema material batch pass: %v", err)
	}
	if err := ensureMaterialBatchTables(ctx, pool, schema); err != nil {
		t.Fatalf("repeat EnsureSchema material batch pass: %v", err)
	}
	var total int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.material_batch_locations`, schema)).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("location rows=%d, want exactly existing WIP plus one missing-location backfill", total)
	}
	var wip, raw int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(sum(qty_g) FILTER(WHERE warehouse='wip'),0),COALESCE(sum(qty_g) FILTER(WHERE warehouse='raw_materials'),0) FROM %s.material_batch_locations WHERE material_batch_id=1`, schema)).Scan(&wip, &raw); err != nil {
		t.Fatal(err)
	}
	if wip != 100 || raw != 0 {
		t.Fatalf("existing batch locations wip=%d raw_materials=%d; repeat startup must not manufacture a duplicate", wip, raw)
	}
}
