package customerfulfillment

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	"sync"
	"testing"
)

func productionStopFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := companyAllocationFixture(t)
	_, err := pool.Exec(context.Background(), fmt.Sprintf(`
 UPDATE %[1]s.production_plans SET status='running' WHERE id=78400;
 UPDATE %[1]s.production_plan_items SET order_nos='ALLOC-SOURCE' WHERE id=78400;
 UPDATE %[1]s.work_orders SET order_nos='ALLOC-SOURCE',status='running',material_consumption_policy='bom_start_v1' WHERE id=78400;
 UPDATE %[1]s.produce_running_items SET status='running',finished_at=NULL WHERE id=78400;
 INSERT INTO %[1]s.job_cards(id,work_order_id,operation,status,started_at,actual_input_qty) VALUES(78400,78400,'烘焙','running',now(),5);
 INSERT INTO %[1]s.production_operation_events(id,work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_unit,output_unit,output_qty,warehouse,payload) VALUES(78400,78400,78400,'complete','posted','stop-test','v','operator','kg','kg',2,'wip','{}');
 INSERT INTO %[1]s.production_operation_balances(id,event_id,work_order_id,job_card_id,sequence_no,owner_customer_id,warehouse,unit,output_qty,remaining_qty,total_cost,remaining_cost) VALUES(78400,78400,78400,78400,1,0,'wip','kg',2,2,25,25);
 INSERT INTO %[1]s.production_started_inputs(id,work_order_id,job_card_id,request_id,input_qty,remaining_qty,input_unit,material_cost,remaining_cost) VALUES(78400,78400,78400,'stop-input',5,3,'kg',50,30);
 INSERT INTO %[1]s.production_started_input_components(id,input_id,component_key,component_type,component_id,material_batch_id,warehouse,qty,remaining_qty,unit,cost,remaining_cost) VALUES(78400,78400,'material:1','material',1,1,'wip',5,3,'kg',50,30);
 INSERT INTO %[1]s.work_order_material_reservations(id,work_order_id,running_item_id,material_id,material_name,unit,reserved_g,consumed_g,source_warehouse) VALUES(78400,78400,78400,1,'原料','g',8000,5000,'raw_materials');
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	return pool, schema
}

func TestOrderResolutionProductionStopRevalidatesAndRollsBack(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"input changed", `UPDATE %[1]s.production_started_inputs SET remaining_qty=2 WHERE id=78400`},
		{"WIP changed", `UPDATE %[1]s.production_operation_balances SET remaining_qty=1,remaining_cost=12.5 WHERE id=78400`},
		{"ownership changed", `UPDATE %[1]s.production_started_input_components SET owner_customer_id=501 WHERE id=78400`},
		{"reservation changed", `UPDATE %[1]s.work_order_material_reservations SET consumed_g=6000 WHERE id=78400`},
		{"audit rollback", `CREATE FUNCTION %[1]s.fail_stop_audit()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.action='resolution_production_stop' THEN RAISE EXCEPTION 'forced stop audit failure';END IF;RETURN NEW;END$$;CREATE TRIGGER fail_stop_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_stop_audit()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := productionStopFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78400, productionStopCommand(c.Version), "sales")
			if err != nil {
				t.Fatal(err)
			}
			first, second := result.Steps[0], result.Steps[1]
			if _, err = r.CompleteStep(ctx, 78400, result.ID, first.ID, resolutionpg.StepCommand{Choice: "stop", Note: "确认原方案", IdempotencyKey: "first"}, "warehouse"); err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.change, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, second.ID, resolutionpg.StepCommand{Choice: "stop", Note: "确认原方案", IdempotencyKey: "second"}, "production"); err == nil {
				t.Fatal("changed facts or missing audit accepted")
			}
			var stops int
			var status string
			var returned int64
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_resolution_production_stops),(SELECT status FROM %[1]s.work_orders WHERE id=78400),(SELECT returned_g FROM %[1]s.work_order_material_reservations WHERE id=78400)`, schema)).Scan(&stops, &status, &returned); err != nil {
				t.Fatal(err)
			}
			if stops != 0 || status != "running" || returned != 0 {
				t.Fatalf("partial stop escaped rollback: %d %s %d", stops, status, returned)
			}
		})
	}
}

func TestOrderResolutionProductionStopConcurrentConfirmationAndReopen(t *testing.T) {
	pool, schema := productionStopFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := productionStopCommand(c.Version)
	cmd.Dispositions[0].Stops[0].Reservations = "keep"
	result, err := r.Submit(ctx, 78400, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	first, last := result.Steps[0], result.Steps[1]
	if _, err = r.CompleteStep(ctx, 78400, result.ID, first.ID, resolutionpg.StepCommand{Choice: "stop", Note: "保留预留", IdempotencyKey: "first"}, "warehouse"); err != nil {
		t.Fatal(err)
	}
	confirm := resolutionpg.StepCommand{Choice: "stop", Note: "保留原位及预留", IdempotencyKey: "last"}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.CompleteStep(ctx, 78400, result.ID, last.ID, confirm, "production")
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		}
	}
	if success == 0 {
		t.Fatal("no concurrent confirmation succeeded")
	}
	if _, err = r.CompleteStep(ctx, 78400, result.ID, last.ID, confirm, "production"); err != nil {
		t.Fatal("retry must return original result", err)
	}
	var records, logs int
	var status string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_resolution_production_stops),(SELECT count(*) FROM %[1]s.audit_logs WHERE action='resolution_production_stop'),(SELECT status FROM %[1]s.work_order_material_reservations WHERE id=78400)`, schema)).Scan(&records, &logs, &status); err != nil {
		t.Fatal(err)
	}
	if records != 1 || logs != 1 || status != "reserved" {
		t.Fatalf("duplicate stop or changed explicit retention: %d %d %s", records, logs, status)
	}
	c, err = r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	reopen := productionStopCommand(c.Version)
	reopen.Action = "reopen"
	reopen.Dispositions = nil
	reopen.IdempotencyKey = "reopen-after-stop"
	if _, err = r.Submit(ctx, 78400, reopen, "sales"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=78400`, schema)).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "stopped" {
		t.Fatal("reopening order resumed stopped work")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.order_resolution_production_stops`, schema)); err == nil {
		t.Fatal("deleted stop history")
	}
	if err = productionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal("production reinitialization with stopped work", err)
	}
	if err = resolutionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal("resolution reinitialization with stopped work", err)
	}
}
func productionStopCommand(version string) domain.Command {
	cmd := companyAllocationCommand(version)
	cmd.IdempotencyKey = "production-stop"
	cmd.Dispositions = []domain.Disposition{{Kind: "production", ID: 78400, Choice: "stop", Stops: []domain.StopSelection{{WorkOrderID: 78400, Materials: "retain_original", WIP: "retain_original", Reservations: "release_unconsumed", Outputs: "keep_original"}}}}
	return cmd
}
func TestOrderResolutionProductionStopPreservesPostedFacts(t *testing.T) {
	pool, schema := productionStopFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Dependencies) != 1 || len(c.Dependencies[0].StopWorkOrders) != 1 {
		t.Fatalf("missing physical remainder preview: %+v", c.Dependencies)
	}
	w := c.Dependencies[0].StopWorkOrders[0]
	if !w.HasMaterials || !w.HasWIP || !w.HasReservations || !w.HasOutputs || len(w.Remainders) < 4 {
		t.Fatalf("incomplete stop facts: %+v", w)
	}
	snapshot := func() string {
		var raw string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('inputs',(SELECT jsonb_agg(to_jsonb(i)) FROM %[1]s.production_started_inputs i),'components',(SELECT jsonb_agg(to_jsonb(i)) FROM %[1]s.production_started_input_components i),'wip',(SELECT jsonb_agg(to_jsonb(i)) FROM %[1]s.production_operation_balances i),'stock',(SELECT jsonb_agg(to_jsonb(i)) FROM %[1]s.stock_batches i),'ledger',(SELECT jsonb_agg(to_jsonb(i)) FROM %[1]s.stock_ledger_entries i),'other',(SELECT to_jsonb(i) FROM %[1]s.orders i WHERE id=78402))::text`, schema)).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	before := snapshot()
	result, err := r.Submit(ctx, 78400, productionStopCommand(c.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("requires production and warehouse: %+v", result.Steps)
	}
	for _, step := range result.Steps {
		result, err = r.CompleteStep(ctx, 78400, result.ID, step.ID, resolutionpg.StepCommand{Choice: "stop", Note: "核对原位保管，释放未耗用预留，保留已完工产出", IdempotencyKey: fmt.Sprint("stop-", step.ID)}, step.Permission)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Applied || result.Status != "completed" || snapshot() != before {
		t.Fatalf("stop changed posted business facts: %+v", result)
	}
	var status string
	var consumed, returned int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,consumed_g,returned_g FROM %[1]s.work_order_material_reservations WHERE id=78400`, schema)).Scan(&status, &consumed, &returned); err != nil {
		t.Fatal(err)
	}
	if status != "released" || consumed != 5000 || returned != 3000 {
		t.Fatalf("reservation history lost: %s %d %d", status, consumed, returned)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=78400`, schema)).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "stopped" {
		t.Fatal("work order still executable", status)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_operation_events(id,work_order_id,job_card_id,kind,status,request_id,expected_version,operator,input_unit,output_unit,output_qty,warehouse,payload) VALUES(78410,78401,78401,'complete','posted','other-work','v','operator','kg','kg',1,'wip','{}')`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO %[1]s.production_operation_movements(event_id,source_balance_id,qty,unit,warehouse,cost) VALUES(78410,78400,1,'kg','wip',10)`,
		`INSERT INTO %[1]s.production_started_handoffs(event_id,job_card_id,source_balance_id,qty,remaining_qty) VALUES(78410,78401,78400,1,1)`,
		`INSERT INTO %[1]s.production_started_input_component_allocations(component_id,event_id,qty,cost) VALUES(78400,78410,1,10)`,
		`INSERT INTO %[1]s.production_started_input_returns(component_id,event_id,qty,cost) VALUES(78400,78410,1,10)`,
	} {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, fmt.Sprintf(sql, schema))
		_ = tx.Rollback(ctx)
		if e == nil {
			t.Errorf("other work could consume stopped material/WIP: %s", sql)
		}
	}
	for _, sql := range []string{`INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,order_nos) VALUES(78409,'STOP-BYPASS',78400,'ALLOC-TARGET')`, `UPDATE %[1]s.job_cards SET work_order_id=0 WHERE id=78400`} {
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		_, e = tx.Exec(ctx, fmt.Sprintf(sql, schema))
		_ = tx.Rollback(ctx)
		if e == nil {
			t.Fatal("stopped production identity could be replaced", sql)
		}
	}
	for _, sql := range []string{`UPDATE %[1]s.work_orders SET status='running' WHERE id=78400`, `UPDATE %[1]s.job_cards SET status='completed' WHERE id=78400`, `UPDATE %[1]s.production_started_inputs SET remaining_qty=2 WHERE id=78400`, `UPDATE %[1]s.production_operation_balances SET remaining_qty=1 WHERE id=78400`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(sql, schema)); err == nil {
			t.Fatal("stopped work bypassed its preserved disposition", sql)
		}
	}
}

func TestOrderResolutionProductionStopWeightOutputCost(t *testing.T) {
	pool, schema := productionStopFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.stock_batches SET remaining_units=0,remaining_g=2000,qty_g=2000,unit_cost=15 WHERE id=78400`, schema))
	if err != nil {
		t.Fatal(err)
	}
	c, err := resolutionpg.NewRepository(pool, schema).Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range c.Dependencies {
		for _, w := range d.StopWorkOrders {
			for _, row := range w.Remainders {
				if row.Kind == "outputs" {
					if row.Quantity != 2000 || row.Unit != "g" || row.Cost != 30 {
						t.Fatalf("weight batch cost/quantity: %+v", row)
					}
					return
				}
			}
		}
	}
	t.Fatal("missing weight output")
}
