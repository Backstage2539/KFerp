package productspecmigration

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOperationSnapshotCopyPreservesVariantAndUnknownPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("database required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("snapshot_copy_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`CREATE TABLE %[1]s.production_bom_version_variants(id bigint,version_id bigint,bom_spec_id bigint);
 CREATE TABLE %[1]s.production_bom_version_operation_costs(id bigserial,version_id bigint,variant_id bigint,operation_id bigint,operation_name text,workstation_id bigint,workstation_name text,workstation_capacity_id bigint,capacity_name text,hourly_rate_snapshot numeric,standard_minutes_snapshot numeric,batch_size_qty_snapshot numeric,batch_size_unit_snapshot text,cost_method text,piece_rate_snapshot numeric,rate_unit_snapshot text,operation_unit_cost numeric,operation_cost_unit text,sort_order int,cost_unavailable boolean,scheduling_mode_snapshot text,throughput_per_hour_snapshot numeric);
 INSERT INTO %[1]s.production_bom_version_variants VALUES(11,1,101),(21,2,101),(31,3,102);
 INSERT INTO %[1]s.production_bom_version_operation_costs(version_id,variant_id,operation_id,cost_unavailable,scheduling_mode_snapshot,throughput_per_hour_snapshot) VALUES(1,11,1,true,'quantity',0),(1,0,2,false,'batch',0);`, schema)
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = copyFrozenOperationCostsTx(ctx, tx, schema, 2, 1, 0); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	var unknown bool
	var mode string
	var speed float64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable,scheduling_mode_snapshot,throughput_per_hour_snapshot::float8 FROM %s.production_bom_version_operation_costs WHERE version_id=2 AND variant_id=21`, schema)).Scan(&unknown, &mode, &speed); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !unknown || mode != "quantity" || speed != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("snapshot changed: %v %s %v", unknown, mode, speed)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_bom_version_operation_costs(version_id,variant_id,operation_id,cost_unavailable,scheduling_mode_snapshot,throughput_per_hour_snapshot) VALUES(4,0,4,true,'quantity',123)`, schema)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = copyFrozenOperationCostsTx(ctx, tx, schema, 5, 4, 51); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable,scheduling_mode_snapshot,throughput_per_hour_snapshot::float8 FROM %s.production_bom_version_operation_costs WHERE version_id=5 AND variant_id=51`, schema)).Scan(&unknown, &mode, &speed); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !unknown || mode != "quantity" || speed != 123 {
		_ = tx.Rollback(ctx)
		t.Fatalf("single-output snapshot changed: %v %s %v", unknown, mode, speed)
	}
	if err = copyFrozenOperationCostsTx(ctx, tx, schema, 6, 1, 61); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("multi-variant snapshot was flattened")
	}
	if err = copyFrozenOperationCostsTx(ctx, tx, schema, 3, 1, 0); err == nil {
		_ = tx.Rollback(ctx)
		t.Fatal("unmapped variant was accepted")
	}
	_ = tx.Rollback(ctx)
}
