package production

import (
	"context"
	"fmt"
	"testing"
)

func TestStartedInputSchemaKeepsLegacyOrdersAndRejectsOverallocation(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	var defaultPolicy string
	err := pool.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_schema=$1 AND table_name='work_orders' AND column_name='material_consumption_policy'`, schema).Scan(&defaultPolicy)
	if err != nil || defaultPolicy != "'legacy'::text" {
		t.Fatalf("historical orders must retain legacy policy: %q %v", defaultPolicy, err)
	}
	query := fmt.Sprintf(`INSERT INTO %s.production_started_inputs(work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,material_cost,remaining_cost) VALUES(1,2,$1,10,$2,'kg',20,$3)`, schema)
	if _, err := pool.Exec(ctx, query, "valid", 10, 20); err != nil {
		t.Fatalf("valid start balance: %v", err)
	}
	if _, err := pool.Exec(ctx, query, "valid", 10, 20); err == nil {
		t.Fatal("duplicate start request accepted")
	}
	if _, err := pool.Exec(ctx, query, "bad-quantity", 11, 20); err == nil {
		t.Fatal("remaining quantity may not exceed started quantity")
	}
	if _, err := pool.Exec(ctx, query, "bad-cost", 10, 21); err == nil {
		t.Fatal("remaining cost may not exceed started cost")
	}
}

func TestStartedInputComponentBalancesRetainBatchIdentity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	var inputID int64
	err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.production_started_inputs(work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,material_cost,remaining_cost) VALUES(1,2,'start',10,10,'kg',20,20) RETURNING id`, schema)).Scan(&inputID)
	if err != nil {
		t.Fatal(err)
	}
	query := fmt.Sprintf(`INSERT INTO %s.production_started_input_components(input_id,component_key,component_type,component_id,material_batch_id,stock_batch_id,warehouse,owner_customer_id,qty,remaining_qty,unit,cost,remaining_cost) VALUES($1,'material:34:0:0','material',34,$2,$3,'WIP',7,10,$4,'kg',20,$5)`, schema)
	if _, err = pool.Exec(ctx, query, inputID, 41, 0, 10, 20); err != nil {
		t.Fatalf("valid component: %v", err)
	}
	for _, tc := range []struct {
		materialBatch, stockBatch int
		qty, cost                 int
	}{{0, 0, 10, 20}, {41, 52, 10, 20}, {41, 0, 11, 20}, {41, 0, 10, 21}} {
		if _, err = pool.Exec(ctx, query, inputID, tc.materialBatch, tc.stockBatch, tc.qty, tc.cost); err == nil {
			t.Fatalf("invalid component accepted: %+v", tc)
		}
	}
}
