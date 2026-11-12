package production

import (
	"context"
	"fmt"
	"math"
	"testing"
)

func TestWorkOrderStartedMaterialCostPostgres(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	sql := fmt.Sprintf(`
 CREATE TABLE %[1]s.production_started_inputs(id bigint primary key,work_order_id bigint,status text,material_cost numeric(20,6));
 CREATE TABLE %[1]s.production_started_input_components(id bigint primary key,input_id bigint,material_batch_id bigint,stock_batch_id bigint,cost numeric(20,6));
 CREATE TABLE %[1]s.production_started_input_returns(id bigint primary key,component_id bigint,cost numeric(20,6));
 CREATE TABLE %[1]s.production_started_input_component_allocations(id bigint primary key,component_id bigint,cost numeric(20,6));
 CREATE TABLE %[1]s.material_batches(id bigint primary key,cost_unavailable boolean not null default false);
 CREATE TABLE %[1]s.stock_batches(id bigint primary key,cost_unavailable boolean not null default false);
 INSERT INTO %[1]s.material_batches VALUES(1,false),(2,true);
 INSERT INTO %[1]s.stock_batches VALUES(1,false),(2,true);
 INSERT INTO %[1]s.production_started_inputs VALUES(1,148,'posted',2992.565),(2,148,'posted',50),(3,148,'reversed',999),(4,149,'posted',88),(5,150,'posted',99),(6,151,'posted',20),(7,152,'posted',80);
 INSERT INTO %[1]s.production_started_input_components VALUES(1,1,1,0,2992.565),(2,2,0,1,50),(3,3,1,0,999),(4,4,2,0,88),(5,5,0,2,99),(6,7,99,0,80);
 INSERT INTO %[1]s.production_started_input_returns VALUES(1,2,12.5);
 INSERT INTO %[1]s.production_started_input_component_allocations VALUES(1,1,2000);`, schema)
	mustExecProductionSQL(t, ctx, pool, sql)
	repo := NewRepository(pool, schema)
	check := func(id int64, want *float64) {
		t.Helper()
		got, err := repo.GetWorkOrderStartedMaterialCost(ctx, id)
		if err != nil {
			t.Fatalf("order %d query: %v", id, err)
		}
		if want == nil {
			if got != nil {
				t.Fatalf("order %d expected unrecorded, got %v", id, *got)
			}
			return
		}
		if got == nil || math.Abs(*got-*want) > 0.000001 {
			t.Fatalf("order %d cost=%v want=%v", id, got, *want)
		}
	}
	expected := 3030.065
	check(148, &expected)
	for _, id := range []int64{149, 150, 151, 152, 153} {
		check(id, nil)
	}
	// Completed batch allocation remains part of the posted input cost.
	// Returns exceeding the original component value indicate corrupt history.
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf("INSERT INTO %s.production_started_input_returns VALUES(2,2,100)", schema))
	check(148, nil)
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf("DELETE FROM %s.production_started_input_returns WHERE id=2", schema))
	check(148, &expected)
	// An incomplete component breakdown cannot claim a reliable recorded cost.
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf("UPDATE %s.production_started_input_components SET cost=49 WHERE id=2", schema))
	check(148, nil)
}
