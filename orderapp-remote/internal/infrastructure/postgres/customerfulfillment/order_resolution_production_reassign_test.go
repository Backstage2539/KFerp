package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	productionpg "orderapp/internal/infrastructure/postgres/production"
	"sync"
	"testing"
)

func productionReassignFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := productionStopFixture(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %[1]s.stock_ledger_entries WHERE source_batch_code='ALLOC-COMPANY-BATCH';DELETE FROM %[1]s.stock_batches WHERE id=78400;UPDATE %[1]s.finished_inventory SET onhand_units=0 WHERE product_id=910;UPDATE %[1]s.order_items SET qty=5,line_total=150 WHERE id IN(78400,78401);UPDATE %[1]s.orders SET total_amount=150,grand_total=150 WHERE id IN(78400,78401);UPDATE %[1]s.production_plan_items SET inventory_unit='袋',customer_id=501,target_warehouse='finished_goods' WHERE id=78400;UPDATE %[1]s.work_orders SET inventory_unit='袋',customer_id=501,target_warehouse='finished_goods' WHERE id=78400;UPDATE %[1]s.produce_running_items SET order_nos='ALLOC-SOURCE',planned_units=5 WHERE id=78400`, schema))
	if err != nil {
		t.Fatal(err)
	}
	// Use the canonical output backfill before freezing the transfer snapshot.
	if err = productionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	return pool, schema
}
func productionReassignCommand(version string) domain.Command {
	cmd := companyAllocationCommand(version)
	cmd.IdempotencyKey = "production-reassign"
	cmd.Dispositions = nil
	if err := json.Unmarshal([]byte(`[{"kind":"production","id":78400,"choice":"reassign","reassignments":[{"work_order_id":78400,"target_item_id":78401,"quantity":5}]}]`), &cmd.Dispositions); err != nil {
		panic(err)
	}
	return cmd
}
func TestOrderResolutionProductionReassignPreservesPostedInputsAndSourceHistory(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := productionReassignCommand(c.Version)
	result, err := r.Submit(ctx, 78400, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Steps) != 1 || result.Steps[0].Permission != "production.resolve" {
		t.Fatalf("missing production confirmation: %+v", result.Steps)
	}
	confirm := resolutionpg.StepCommand{Choice: "reassign", Note: "原五袋生产转供明确目标需求，余料及在制品保留原工单", Reference: "PRODUCTION-ASSIGN", IdempotencyKey: "confirm"}
	for n := 0; n < 2; n++ {
		result, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, confirm, "production")
		if err != nil {
			t.Fatal(err)
		}
	}
	if !result.Applied {
		t.Fatal("reassignment not applied")
	}
	var work, plan, run string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT order_nos FROM %[1]s.work_orders WHERE id=78400),(SELECT order_nos FROM %[1]s.production_plan_items WHERE id=78400),(SELECT order_nos FROM %[1]s.produce_running_items WHERE id=78400)`, schema)).Scan(&work, &plan, &run); err != nil {
		t.Fatal(err)
	}
	if work != "ALLOC-TARGET" || plan != work || run != work {
		t.Fatalf("effective demand not switched: %s %s %s", work, plan, run)
	}
	assertMiniDirectShipCount(t, pool, schema, "production_started_input_components", "id=78400 AND remaining_qty=3 AND remaining_cost=30", 1)
	assertMiniDirectShipCount(t, pool, schema, "production_operation_balances", "id=78400 AND remaining_qty=2 AND remaining_cost=25", 1)
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_production_reassignments", "source_order_id=78400 AND target_order_id=78401 AND quantity=5 AND snapshot_json->'work'->>'order_nos'='ALLOC-SOURCE'", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_production_reassign'", 1)
	next, err := r.Context(ctx, 78401)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Dependencies) == 0 {
		t.Fatal("target production not shown")
	}
}

func TestOrderResolutionProductionReassignRevalidationAndHistory(t *testing.T) {
	for _, tc := range []struct{ name, change string }{{"target quantity", `UPDATE %[1]s.order_items SET qty=4,line_total=120 WHERE id=78401`}, {"target warehouse", `UPDATE %[1]s.orders SET source_warehouse='finished_shop' WHERE id=78401`}, {"target paused", `UPDATE %[1]s.orders SET fulfillment_paused=true WHERE id=78401`}, {"WIP changed", `UPDATE %[1]s.production_operation_balances SET remaining_qty=1 WHERE id=78400`}, {"other production", `INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78401,'OTHER-TARGET-PLAN','draft');INSERT INTO %[1]s.production_plan_items(production_plan_id,product_id,bom_spec_id,bom_variant_id,order_nos)VALUES(78401,910,7031,7032,'ALLOC-TARGET')`}, {"audit rollback", `CREATE FUNCTION %[1]s.fail_reassign_audit()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.action='resolution_production_reassign' THEN RAISE EXCEPTION 'forced transfer audit';END IF;RETURN NEW;END$$;CREATE TRIGGER fail_reassign_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_reassign_audit()`}} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := productionReassignFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.change, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "reassign", Note: "原方案转供", Reference: "APPROVE", IdempotencyKey: "confirm"}, "production"); err == nil {
				t.Fatal("stale or unaudited transfer accepted")
			}
			assertMiniDirectShipCount(t, pool, schema, "work_orders", "id=78400 AND order_nos='ALLOC-SOURCE'", 1)
			assertMiniDirectShipCount(t, pool, schema, "order_resolution_production_reassignments", "true", 0)
			assertMiniDirectShipCount(t, pool, schema, "orders", "id=78400 AND resolution_credit_cents=0", 1)
		})
	}
}
func TestOrderResolutionProductionReassignCannotSilentlyChangeTargetOrSource(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	result, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "reassign", Note: "明确目标转供", Reference: "APPROVE", IdempotencyKey: "confirm"}, "production")
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`DELETE FROM %[1]s.order_resolution_production_reassignments`, `UPDATE %[1]s.work_orders SET order_nos='ALLOC-SOURCE' WHERE id=78400`, `UPDATE %[1]s.work_orders SET material_snapshot='[{"tamper":true}]' WHERE id=78400`, `UPDATE %[1]s.work_orders SET process_snapshot_json='{"tamper":true}' WHERE id=78400`, `UPDATE %[1]s.production_plan_items SET customer_id=501 WHERE id=78400`, `UPDATE %[1]s.order_items SET qty=1 WHERE id=78401`, `UPDATE %[1]s.orders SET customer_id=501 WHERE id=78401`, `UPDATE %[1]s.orders SET order_no='LOST-LINK' WHERE id=78401`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(query, schema)); err == nil {
			t.Errorf("reassignment history can be rewritten: %s", query)
		}
	}
	c, err = r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := productionReassignCommand(c.Version)
	cmd.Action = "reopen"
	cmd.Dispositions = nil
	cmd.IdempotencyKey = "reopen"
	if _, err = r.Submit(ctx, 78400, cmd, "sales"); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "work_orders", "id=78400 AND order_nos='ALLOC-TARGET'", 1)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.job_cards SET actual_input_qty=actual_input_qty+1 WHERE id=78400`, schema)); err != nil {
		t.Fatal("target-owned work cannot continue", err)
	}
}

func TestOrderResolutionProductionReassignDoesNotHideUnplannedTargetRemainder(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.order_items SET qty=8,line_total=240 WHERE id=78401;UPDATE %[1]s.orders SET total_amount=240,grand_total=240 WHERE id=78401`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales"); err == nil {
		t.Fatal("partial production hid remaining target demand; split target first")
	}
}

func TestOrderResolutionProductionReassignConcurrentAndStartup(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolutionpg.StepCommand{Choice: "reassign", Note: "明确目标及原投入", Reference: "PROOF", IdempotencyKey: "same"}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, cmd, "production")
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	success := false
	for err := range errs {
		success = success || err == nil
	}
	if !success {
		t.Fatal("no concurrent transfer succeeded")
	}
	if _, err = r.CompleteStep(ctx, 78400, result.ID, result.Steps[0].ID, cmd, "production"); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_production_reassignments", "true", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_production_reassign'", 1)
	for _, ensure := range []func(context.Context, *pgxpool.Pool, string) error{productionpg.EnsureSchema, resolutionpg.EnsureSchema} {
		if err = ensure(ctx, pool, schema); err != nil {
			t.Fatalf("restart: %v", err)
		}
	}
	for _, q := range []string{`INSERT INTO %[1]s.work_orders(work_order_no,production_plan_id,production_plan_item_id,running_item_id,product_id,bom_spec_id,bom_variant_id,order_nos)VALUES('DUP-TRANSFER',78400,78400,78400,910,7031,7032,'ALLOC-TARGET')`, `INSERT INTO %[1]s.production_plan_items(production_plan_id,product_id,bom_spec_id,bom_variant_id,order_nos)VALUES(78400,910,7031,7032,'ALLOC-SOURCE')`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(q, schema)); err == nil {
			t.Errorf("reassigned whole plan accepts new work or demand: %s", q)
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78501,'DUP-TARGET-PLAN','draft');INSERT INTO %[1]s.production_plan_items(production_plan_id,product_id,bom_spec_id,bom_variant_id,order_nos)VALUES(78501,910,7031,7032,'ALLOC-TARGET')`, schema)); err == nil {
		t.Fatal("target receives duplicate production after transfer")
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET order_date='2026-11-09' WHERE id=78401`, schema)); err != nil {
		t.Fatal("date edit must stay possible", err)
	}
}
func TestOrderResolutionProductionReassignRefreshSupersedesOldApproval(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_resolution_policy SET policy_json='{"approvals":{"close":"orders.resolve"},"disabled":{}}'`, schema)); err != nil {
		t.Fatal(err)
	}
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales")
	if err != nil {
		t.Fatal(err)
	}
	var prior int64
	for _, step := range result.Steps {
		if step.Kind == "production" {
			prior = step.ID
			_, err = r.CompleteStep(ctx, 78400, result.ID, step.ID, resolutionpg.StepCommand{Choice: "reassign", Note: "原目标确认", Reference: "APPROVE", IdempotencyKey: "before"}, "production")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET order_date='2026-11-09' WHERE id=78401`, schema)); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		c, err = r.Context(ctx, 78400)
		if err != nil {
			t.Fatal(err)
		}
		cmd := productionReassignCommand(c.Version)
		cmd.IdempotencyKey = fmt.Sprint("refresh-", n)
		result, err = r.Refresh(ctx, 78400, result.ID, cmd, "sales")
		if err != nil {
			t.Fatal(err)
		}
		pending := 0
		for _, step := range result.Steps {
			if step.ID == prior && step.Status != "superseded" {
				t.Fatal("stale production approval reused")
			}
			if step.Kind == "production" && step.Status == "pending" {
				pending++
			}
		}
		if pending != 1 {
			t.Fatalf("missing new confirmation: %+v", result.Steps)
		}
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_production_reassignments", "true", 0)
}

func TestOrderResolutionProductionReassignPreservesGenericCustomerSnapshot(t *testing.T) {
	pool, schema := productionReassignFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.work_orders SET customer_product_snapshot_json='[{"order_no":"ALLOC-SOURCE","customer_id":501,"customer_product_alias_id":0,"customer_product_display_name_snapshot":"","customer_item_code_snapshot":"","product_name_snapshot":"咖啡"}]' WHERE id=78400;UPDATE %[1]s.production_plan_items SET customer_product_snapshot_json=(SELECT customer_product_snapshot_json FROM %[1]s.work_orders WHERE id=78400) WHERE id=78400`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales"); err != nil {
		t.Fatal("generic customer labels wrongly treated as customer-specific production", err)
	}
}
func TestOrderResolutionProductionReassignRejectsUnacceptedOrCustomerSpecificTarget(t *testing.T) {
	for _, change := range []string{`UPDATE %[1]s.orders SET confirmation_status='pending' WHERE id=78401`, `UPDATE %[1]s.order_items SET customer_product_display_name_snapshot='专属品牌' WHERE id=78401`, `UPDATE %[1]s.order_items SET item_note='特殊加工要求' WHERE id=78401`} {
		t.Run(change, func(t *testing.T) {
			pool, schema := productionReassignFixture(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf(change, schema)); err != nil {
				t.Fatal(err)
			}
			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.Submit(ctx, 78400, productionReassignCommand(c.Version), "sales"); err == nil {
				t.Fatal("unconfirmed or customer-specific target accepted")
			}
		})
	}
}
