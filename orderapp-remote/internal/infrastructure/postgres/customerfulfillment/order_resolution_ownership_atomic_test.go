package customerfulfillment

import (
	"context"
	"fmt"
	domain "orderapp/internal/domain/orderresolution"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	"testing"
	"time"
)

func TestOrderResolutionOwnershipRejectsStaleOrUnapprovedStockAtomically(t *testing.T) {
	for _, tc := range []struct{ name, mutation string }{
		{"foreign destination inventory", `UPDATE %s.warehouses SET customer_id=502 WHERE code='finished_goods'`},
		{"changed source cost", `UPDATE %s.stock_batches SET unit_cost=13 WHERE id=78510`},
		{"missing original shipment", `DELETE FROM %s.stock_ledger_entries WHERE source_doc_type='production_run'`},
		{"disabled destination", `UPDATE %s.warehouses SET active=false WHERE code='finished_goods'`},
		{"missing outgoing proof", `CREATE FUNCTION %[1]s.erase_outgoing_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.source_doc_type='order_resolution_return' THEN DELETE FROM %[1]s.stock_ledger_entries WHERE source_doc_type='order_resolution_ownership_return_out'; END IF;RETURN NEW;END $$;CREATE TRIGGER erase_outgoing_test AFTER INSERT ON %[1]s.stock_ledger_entries FOR EACH ROW EXECUTE FUNCTION %[1]s.erase_outgoing_test()`},
		{"reserved shared batch", `INSERT INTO %[1]s.orders(id,order_no,customer_id,fulfillment_mode)VALUES(78512,'RESERVED-OTHER',501,'stock'); INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,bom_spec_id,bom_variant_id,qty,unit)VALUES(78512,78512,1,910,'咖啡',7031,7032,4,'袋'); INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,spec_g,batch_id,batch_code,allocated_units,warehouse)VALUES(78512,78512,910,7031,7032,0,78510,'OWNERSHIP-SHARED-BATCH',4,'DEV-E2E-A1')`},
		{"audit failure", `CREATE FUNCTION %[1]s.reject_return_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='resolution_return_stock' THEN RAISE EXCEPTION 'forced return audit failure';END IF;RETURN NEW;END $$;CREATE TRIGGER reject_return_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_return_audit()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := ownershipReturnFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			current, err := r.Context(ctx, 78510)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78510, ownershipReturnCommand(current), "sales")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.mutation, schema)); err != nil {
				t.Fatal(err)
			}
			snapshot := func() string {
				t.Helper()
				var raw string
				if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('batches',(SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM %[1]s.stock_batches b),'inventory',(SELECT jsonb_agg(to_jsonb(i) ORDER BY product_id,bom_spec_id,warehouse) FROM %[1]s.finished_inventory i),'ledger',(SELECT jsonb_agg(to_jsonb(l) ORDER BY id) FROM %[1]s.stock_ledger_entries l),'returns',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM %[1]s.order_resolution_stock_returns r),'entitlements',(SELECT jsonb_agg(to_jsonb(e)ORDER BY id)FROM %[1]s.customer_stock_entitlements e),'credit',(SELECT resolution_credit_cents FROM %[1]s.orders WHERE id=78510))::text`, schema)).Scan(&raw); err != nil {
					t.Fatal(err)
				}
				return raw
			}
			before := snapshot()
			for _, step := range result.Steps {
				if step.Kind == "return_stock" {
					_, err = r.CompleteStep(ctx, 78510, result.ID, step.ID, resolutionpg.StepCommand{Choice: "receive", Reference: "R-negative", Note: "退回核对", PostingDate: "2026-11-08", IdempotencyKey: "invalid-return"}, "warehouse")
					if err == nil {
						t.Fatal("invalid return accepted")
					}
				}
			}
			if after := snapshot(); after != before {
				t.Fatal("failed return retained partial stock or receivable changes")
			}
		})
	}
}

func TestOrderResolutionOwnershipConcurrentWarehouseConfirmation(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78510)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78510, ownershipReturnCommand(current), "sales")
	if err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func(n int) {
			<-start
			_, err := r.CompleteStep(ctx, 78510, result.ID, stockID, resolutionpg.StepCommand{Choice: "receive", Note: "并发核对同一次退货", Reference: "SAME-RECEIPT", PostingDate: "2026-11-08", IdempotencyKey: fmt.Sprintf("parallel-%d", n)}, "warehouse")
			results <- err
		}(n)
	}
	close(start)
	success := 0
	for n := 0; n < 2; n++ {
		if err := <-results; err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent confirmations succeeded %d times", success)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "status='posted'", 1)
	assertMiniDirectShipCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='order_resolution_return'", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_return_stock'", 1)
}

func TestOrderResolutionOwnershipRejectsClosedReceiptPeriodAndCanRetry(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	ctx := context.Background()
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	current, err := r.Context(ctx, 78510)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78510, ownershipReturnCommand(current), "sales")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finance_monthly_reports(month,status)VALUES('2026-10','closed')`, schema)); err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "核对收货日期", Reference: "DATE-RECEIPT", PostingDate: "2026-10-31", IdempotencyKey: "old-period"}
	if _, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse"); err == nil {
		t.Fatal("closed month receipt allowed")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "true", 0)
	confirm.PostingDate = "2026-11-08"
	confirm.IdempotencyKey = "new-date"
	if _, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "posting_date='2026-11-08'", 1)
	if err = resolutionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("schema re-init destroyed posted return: %v", err)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "status='posted'", 1)
}

func TestOrderResolutionOwnershipCannotReceiveBeforeRequiredApproval(t *testing.T) {
	pool, schema := ownershipReturnFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	if _, err := r.SavePolicy(ctx, 1, domain.Policy{Approvals: map[string]string{"return": "finance.resolve"}}, "admin"); err != nil {
		t.Fatal(err)
	}
	current, err := r.Context(ctx, 78510)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78510, ownershipReturnCommand(current), "sales")
	if err != nil {
		t.Fatal(err)
	}
	stockID := returnStepID(t, result, "return_stock")
	confirm := resolutionpg.StepCommand{Choice: "receive", Note: "实际退回核对", Reference: "APPROVAL-RETURN", PostingDate: "2026-11-08", IdempotencyKey: "approval-first"}
	if _, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse"); err == nil {
		t.Fatal("irreversible stock receipt bypassed required approval")
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_stock_returns", "true", 0)
	if _, err = r.CompleteStep(ctx, 78510, result.ID, returnStepID(t, result, "approval"), resolutionpg.StepCommand{Choice: "approve", Note: "同意本次原批次退回方案", IdempotencyKey: "approve"}, "finance"); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CompleteStep(ctx, 78510, result.ID, stockID, confirm, "warehouse"); err != nil {
		t.Fatal(err)
	}
}
