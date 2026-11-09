package bom

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

func TestQuantityBomOperationSnapshotFreezesUnknownAndSpeedPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("bom_quantity_%d", time.Now().UnixNano())
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	ddl := fmt.Sprintf(`
 CREATE TABLE %[1]s.production_bom_versions(id bigint,process_route_id bigint,output_unit text);
 CREATE TABLE %[1]s.process_routes(id bigint,name text);
 CREATE TABLE %[1]s.production_bom_version_variants(id bigint,version_id bigint,process_route_id bigint,inventory_unit text);
 CREATE TABLE %[1]s.process_route_operations(id bigint,route_id bigint,seq int,operation_id bigint,operation text,standard_cost_capacity_id bigint);
 CREATE TABLE %[1]s.manufacturing_operations(id bigint,name text);
 CREATE TABLE %[1]s.manufacturing_workstations(id bigint,name text,status text,hourly_rate numeric);
 CREATE TABLE %[1]s.manufacturing_workstation_operations(workstation_id bigint,operation_id bigint);
 CREATE TABLE %[1]s.manufacturing_workstation_capacities(id bigint,workstation_id bigint,name text,status text,hourly_rate numeric,standard_minutes numeric,batch_size_qty numeric,batch_size_unit text,cost_method text,piece_rate numeric,scheduling_mode text,throughput_per_hour numeric);
 CREATE TABLE %[1]s.production_bom_version_operation_costs(id bigserial,version_id bigint,operation_id bigint,operation_name text,workstation_id bigint,workstation_name text,workstation_capacity_id bigint,capacity_name text,hourly_rate_snapshot numeric,standard_minutes_snapshot numeric,batch_size_qty_snapshot numeric,batch_size_unit_snapshot text,cost_method text,piece_rate_snapshot numeric,rate_unit_snapshot text,operation_unit_cost numeric,operation_cost_unit text,sort_order int,created_at timestamptz,cost_unavailable boolean,scheduling_mode_snapshot text,throughput_per_hour_snapshot numeric,variant_id bigint);
 INSERT INTO %[1]s.production_bom_versions VALUES(1,1,'kg'),(2,1,'kg'); INSERT INTO %[1]s.production_bom_version_variants VALUES(101,1,1,'kg'),(102,2,1,'kg'); INSERT INTO %[1]s.process_routes VALUES(1,'route');
 INSERT INTO %[1]s.process_route_operations VALUES(1,1,1,1,'process',1); INSERT INTO %[1]s.manufacturing_operations VALUES(1,'process');
 INSERT INTO %[1]s.manufacturing_workstations VALUES(1,'station','active',120); INSERT INTO %[1]s.manufacturing_workstation_operations VALUES(1,1);
 INSERT INTO %[1]s.manufacturing_workstation_capacities VALUES(1,1,'continuous','active',120,0,0,'kg','time',0,'quantity',0);`, schema)
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = refreshProductionBomVersionOperationCostSnapshotsTx(ctx, tx, schema, 1); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET throughput_per_hour=600`, schema)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = refreshProductionBomVersionOperationCostSnapshotsTx(ctx, tx, schema, 2); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.manufacturing_workstation_capacities SET throughput_per_hour=100`, schema)); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var missing bool
	var savedSpeed, cost float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT cost_unavailable FROM %s.production_bom_version_operation_costs WHERE version_id=1 AND variant_id=101`, schema)).Scan(&missing); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT throughput_per_hour_snapshot::float8,operation_unit_cost::float8 FROM %s.production_bom_version_operation_costs WHERE version_id=2 AND variant_id=102`, schema)).Scan(&savedSpeed, &cost); err != nil {
		t.Fatal(err)
	}
	if !missing || savedSpeed != 600 || cost != 0.2 {
		t.Fatalf("snapshots missing=%v speed=%v cost=%v", missing, savedSpeed, cost)
	}
}
