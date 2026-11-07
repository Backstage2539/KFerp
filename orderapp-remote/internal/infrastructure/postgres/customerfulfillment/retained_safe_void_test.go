package customerfulfillment

import (
	"context"
	"fmt"
	financeapp "orderapp/internal/application/finance"
	salesapp "orderapp/internal/application/sales"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	salespg "orderapp/internal/infrastructure/postgres/sales"
	"testing"

	resolution "orderapp/internal/domain/orderresolution"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func TestRetainedCloseKeepsCompletedSharedProduction(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,delivery_mode,delivery_warehouse_code,pay_status_id)
VALUES (78001,'SAFE-VOID-SOURCE',501,720,'customer_stock','DEV-E2E-A1',(SELECT id FROM %[1]s.pay_statuses WHERE name IN('未付款','未收款') ORDER BY id LIMIT 1)),
       (78002,'SAFE-VOID-OTHER',501,360,'ship','', (SELECT id FROM %[1]s.pay_statuses WHERE name IN('未付款','未收款') ORDER BY id LIMIT 1));
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)
VALUES(78001,78001,1,911,'retained','1000g',24,'袋',30,720),(78002,78002,1,911,'other','1000g',12,'袋',30,360);
INSERT INTO %[1]s.production_plans(id,plan_no,status) VALUES(78001,'SAFE-VOID-PLAN','completed');
INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,order_nos)
VALUES(78001,78001,911,'shared',1000,36000,'SAFE-VOID-SOURCE,SAFE-VOID-OTHER');
INSERT INTO %[1]s.work_orders(id,work_order_no,production_plan_id,production_plan_item_id,product_id,product_name,spec_g,planned_g,order_nos,status)
VALUES(78001,'SAFE-VOID-WORK',78001,78001,911,'shared',1000,36000,'SAFE-VOID-SOURCE,SAFE-VOID-OTHER','completed');
UPDATE %[1]s.production_plan_items SET customer_id=501,target_warehouse='finished_goods' WHERE id=78001;
UPDATE %[1]s.work_orders SET customer_id=501,target_warehouse='finished_goods' WHERE id=78001;
INSERT INTO %[1]s.customer_stock_supply_reservations(entitlement_id,customer_id,request_id,request_item_id,order_id,order_item_id,reserved_qty,released_qty,status)
SELECT id,501,78001,78001,78002,78002,12,12,'released' FROM %[1]s.customer_stock_entitlements WHERE source_order_id=78001;
`, schema))
	if err != nil {
		t.Fatal(err)
	}
	var before string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('plan',(SELECT to_jsonb(p) FROM %[1]s.production_plans p WHERE id=78001),'item',(SELECT to_jsonb(i) FROM %[1]s.production_plan_items i WHERE id=78001),'work',(SELECT to_jsonb(w) FROM %[1]s.work_orders w WHERE id=78001),'other',(SELECT to_jsonb(o) FROM %[1]s.orders o WHERE id=78002),'stock',(SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM %[1]s.stock_batches b),'reservation',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM %[1]s.customer_stock_supply_reservations r))::text`, schema)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	if err := resolutionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Context(ctx, 78001)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-06", Version: current.Version, IdempotencyKey: "close-shared-1"}
	for _, d := range current.Dependencies {
		if d.Kind == "production" {
			cmd.Dispositions = append(cmd.Dispositions, resolution.Disposition{Kind: d.Kind, ID: d.ID, Choice: "keep_company_stock"})
		}
	}
	result, err := repo.Submit(ctx, 78001, cmd, "employee:703")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("result=%+v", result)
	}
	again, err := repo.Submit(ctx, 78001, cmd, "employee:703")
	if err != nil || again.ID != result.ID {
		t.Fatalf("idempotent replay: %+v %v", again, err)
	}

	var void bool
	var status string
	var auditCount int
	var after string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT o.is_void,o.resolution_status,(SELECT count(*) FROM %[1]s.audit_logs a WHERE a.entity_type='order' AND a.entity_id=o.id AND a.action='resolution_submit') FROM %[1]s.orders o JOIN %[1]s.customer_stock_entitlements e ON e.source_order_id=o.id WHERE o.id=78001`, schema)).Scan(&void, &status, &auditCount); err != nil {
		t.Fatal(err)
	}
	if void || status != "closed" || auditCount != 1 {
		t.Fatalf("void=%v entitlement=%s audits=%d", void, status, auditCount)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('plan',(SELECT to_jsonb(p) FROM %[1]s.production_plans p WHERE id=78001),'item',(SELECT to_jsonb(i) FROM %[1]s.production_plan_items i WHERE id=78001),'work',(SELECT to_jsonb(w) FROM %[1]s.work_orders w WHERE id=78001),'other',(SELECT to_jsonb(o) FROM %[1]s.orders o WHERE id=78002),'stock',(SELECT jsonb_agg(to_jsonb(b) ORDER BY id) FROM %[1]s.stock_batches b),'reservation',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM %[1]s.customer_stock_supply_reservations r))::text`, schema)).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("void altered shared production, other order, stock or reservation: before=%s after=%s", before, after)
	}
}

func TestOrderResolutionRejectsStalePreviewAndAuditFailure(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78003,'RESOLVE-CONCURRENCY',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78003,78003,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78003)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "selected", Lines: []resolution.Selection{{ItemID: 78003, Quantity: 1}}, Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-06", Version: c.Version, IdempotencyKey: "stale-1"}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET notes='edited' WHERE id=78003`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Submit(ctx, 78003, cmd, "employee:703"); err != resolutionpg.ErrStale {
		t.Fatalf("stale=%v", err)
	}
	c, err = repo.Context(ctx, 78003)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = c.Version
	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_resolution_audit()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.action='resolution_submit' THEN RAISE EXCEPTION 'audit unavailable';END IF;RETURN NEW;END$$;CREATE TRIGGER reject_resolution_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_resolution_audit()`, schema))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Submit(ctx, 78003, cmd, "employee:703"); err == nil {
		t.Fatal("audit failure must roll back")
	}
	var closed float64
	var count int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_closed_qty,(SELECT count(*) FROM %[1]s.order_resolution_requests WHERE order_id=78003) FROM %[1]s.order_items WHERE id=78003`, schema)).Scan(&closed, &count); err != nil {
		t.Fatal(err)
	}
	if closed != 0 || count != 0 {
		t.Fatalf("partially committed closed=%v requests=%d", closed, count)
	}
}

func TestOrderResolutionClosedOrderCannotAcquireNewObligations(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,resolution_status)VALUES(78004,'CLOSED-GUARD',501,'closed')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,allocated_units)VALUES(78004,911,1)`,
		`INSERT INTO %s.order_stock_deductions(order_id,product_id,deducted_units,batch_code)VALUES(78004,911,1,'closed-batch')`,
		`UPDATE %s.orders SET ship_tracking_no='SF-CLOSED' WHERE id=78004`,
	} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(query, schema)); err == nil {
			t.Fatalf("closed order acquired obligation: %s", query)
		}
	}
}

func TestOrderResolutionFinancialAdjustmentUsesSelectedPeriod(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount)VALUES(78005,'PERIOD-CLOSE',501,'2026-09-14',90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78005,78005,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78005)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-06", Version: c.Version, IdempotencyKey: "period-1"}
	if _, err = repo.Submit(ctx, 78005, cmd, "employee:703"); err != nil {
		t.Fatal(err)
	}
	fin := financepg.NewRepository(pool, schema)
	for month, want := range map[string]float64{"2026-09": 90, "2026-11": -90} {
		v, _, err := fin.MonthlySourceTotals(ctx, financeapp.ReportFilter{Month: month})
		if err != nil {
			t.Fatal(err)
		}
		if float64(v.RevenueTaxInclusive) != want {
			t.Fatalf("%s revenue=%v want=%v", month, v.RevenueTaxInclusive, want)
		}
		rows, err := fin.FinanceSourceDetails(ctx, financeapp.ReportFilter{Month: month})
		if err != nil {
			t.Fatal(err)
		}
		sum := 0.0
		for _, row := range rows {
			if row.Section == "revenue" {
				sum += float64(row.Amount)
			}
		}
		if sum != want {
			t.Fatalf("%s detail sum=%v", month, sum)
		}
	}
}

func TestOrderResolutionRefundIsExplicitBoundedAndIdempotent(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,prepayment_amount)VALUES(78006,'REFUND-TEST',501,90,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78006,78006,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78006)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-06", Version: c.Version, IdempotencyKey: "refund-1"}
	out, err := repo.Submit(ctx, 78006, cmd, "employee:703")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "pending" || len(out.Steps) != 1 {
		t.Fatalf("out=%+v", out)
	}
	c, err = repo.Context(ctx, 78006)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != "closed" || c.CreditUsedCents != 0 {
		t.Fatalf("close should precede refund: %+v", c)
	}
	step := resolutionpg.StepCommand{IdempotencyKey: "refund-confirm-1", PostingDate: "2026-11-06", Choice: "refund", AmountCents: 9001, Reference: "bank-proof", Account: "bank", Note: "财务核对实际退款"}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_refund_audit()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.action IN('resolution_step','resolution_step_failed')THEN RAISE EXCEPTION 'audit unavailable';END IF;RETURN NEW;END$$;CREATE TRIGGER reject_refund_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_refund_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	validAttempt := step
	validAttempt.AmountCents = 9000
	if _, err = repo.CompleteStep(ctx, 78006, out.ID, out.Steps[0].ID, validAttempt, "finance"); err == nil {
		t.Fatal("refund posted without an audit")
	}
	var failedMoney, failedEvidence int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT(SELECT count(*)FROM %[1]s.order_resolution_financial_entries WHERE order_id=78006 AND kind='refund'),(SELECT count(*)FROM %[1]s.order_resolution_step_failures WHERE step_id=$1)`, schema), out.Steps[0].ID).Scan(&failedMoney, &failedEvidence); err != nil || failedMoney != 0 || failedEvidence != 0 {
		t.Fatalf("audit rollback left money/evidence %d/%d err=%v", failedMoney, failedEvidence, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_refund_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}

	if _, err = repo.CompleteStep(ctx, 78006, out.ID, out.Steps[0].ID, step, "finance"); err == nil {
		t.Fatal("refunded beyond available credit")
	}
	failed, historyErr := repo.History(ctx, 78006)
	if historyErr != nil || len(failed) != 1 || failed[0].Steps[0].Status != "failed" {
		t.Fatalf("failed attempt not tracked: %+v %v", failed, historyErr)
	}
	// The same failed request can be retried, without recording duplicate failure
	// events or moving funds. Correcting its payload still requires full validation.
	if _, err = repo.CompleteStep(ctx, 78006, out.ID, out.Steps[0].ID, step, "finance"); err == nil {
		t.Fatal("invalid retry succeeded")
	}
	var attempts, effects int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_resolution_step_failures WHERE step_id=$1),(SELECT count(*) FROM %[1]s.order_resolution_financial_entries WHERE order_id=78006 AND kind='refund')`, schema), out.Steps[0].ID).Scan(&attempts, &effects); err != nil || attempts != 1 || effects != 0 {
		t.Fatalf("failure retries/effects=%d/%d err=%v", attempts, effects, err)
	}
	step.AmountCents = 9000
	done, err := repo.CompleteStep(ctx, 78006, out.ID, out.Steps[0].ID, step, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "completed" || done.Steps[0].LastError != "" || len(done.Steps[0].Failures) != 1 {
		t.Fatalf("done=%+v", done)
	}
	if _, err = repo.CompleteStep(ctx, 78006, out.ID, out.Steps[0].ID, step, "finance"); err != nil {
		t.Fatal(err)
	}
	var n int
	var received float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT count(*) FROM %[1]s.order_resolution_financial_entries WHERE order_id=78006 AND kind='refund'),prepayment_amount::float8 FROM %[1]s.orders WHERE id=78006`, schema)).Scan(&n, &received); err != nil {
		t.Fatal(err)
	}
	if n != 1 || received != 90 {
		t.Fatalf("refund n=%d original received=%v", n, received)
	}
}

func TestOrderResolutionDiscountAllocationRetainsFreight(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,discount_amount,grand_total,shipping_amount)VALUES(78007,'DISCOUNT-CLOSE',501,90,10,85,5);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78007,78007,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78007)
	if err != nil {
		t.Fatal(err)
	}
	if c.TotalCents != 8500 || len(c.Lines) != 1 || c.Lines[0].AmountCents != 8000 {
		t.Fatalf("net goods not allocated: %+v", c)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "discount-close", Dispositions: []resolution.Disposition{{Kind: "fee", ID: -1, Choice: "keep"}}}
	out, err := repo.Submit(ctx, 78007, cmd, "staff")
	if err != nil {
		t.Fatal(err)
	}
	if out.Plan.CreditCents != 8000 {
		t.Fatalf("freight implicitly waived: %+v", out)
	}
}

func TestOrderResolutionCancelsUnstartedPlanAtomically(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78008,'CANCEL-UNSTARTED',501,90);
 INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78008,78008,1,911,'coffee','1000g',3,'袋',30,90);
 INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78008,'PLAN-UNSTARTED','draft');
 INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,order_nos)VALUES(78008,78008,911,'coffee',1000,3000,'CANCEL-UNSTARTED');`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78008)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "close-unstarted", Dispositions: []resolution.Disposition{{Kind: "production", ID: 78008, Choice: "cancel_unstarted"}}}
	out, err := repo.Submit(ctx, 78008, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "pending" || len(out.Steps) != 1 {
		t.Fatalf("unexpected %+v", out)
	}
	done, err := repo.CompleteStep(ctx, 78008, out.ID, out.Steps[0].ID, resolutionpg.StepCommand{Choice: "cancel_unstarted", Note: "生产负责人确认未开工", IdempotencyKey: "cancel-plan"}, "production")
	if err != nil {
		t.Fatal(err)
	}
	var state string
	var n int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,(SELECT count(*)FROM %[1]s.production_plan_items WHERE production_plan_id=78008)FROM %[1]s.production_plans WHERE id=78008`, schema)).Scan(&state, &n); err != nil {
		t.Fatal(err)
	}
	if state != "cancelled" || n != 1 || done.Status != "completed" {
		t.Fatalf("state=%s items=%d request=%+v", state, n, done)
	}
}

func TestOrderResolutionAmendCreatesLinkedDraftWithoutNewObligation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78009,'AMEND-ORIGINAL',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78009,78009,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78009)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "amend", Reason: "entry_error", Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "amend-original"}
	out, err := repo.Submit(ctx, 78009, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var source, request int64
	var state string
	err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT source_order_id,request_id,status FROM %s.order_resolution_amendments WHERE request_id=$1`, schema), out.ID).Scan(&source, &request, &state)
	if err != nil {
		t.Fatal(err)
	}
	if source != 78009 || request != out.ID || state != "draft" {
		t.Fatalf("source=%d request=%d state=%s", source, request, state)
	}
	var n int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)FROM %s.orders WHERE amended_from_order_id=78009`, schema)).Scan(&n); err != nil || n != 0 {
		t.Fatalf("draft created live order: %d %v", n, err)
	}
}

func TestOrderResolutionAmendmentRequiresMatchingCustomerAndReview(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78010,'AMEND-SOURCE',501,90),(78011,'AMEND-NEW',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78010,78010,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78010)
	if err != nil {
		t.Fatal(err)
	}
	out, err := repo.Submit(ctx, 78010, resolution.Command{Action: "amend", Reason: "entry_error", Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "amend-bind"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = resolutionpg.AttachAmendmentTx(ctx, tx, schema, out.AmendmentID, 78011, "sales"); err == nil {
		t.Fatal("amendment bypassed review")
	}
	tx.Rollback(ctx)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET confirmation_required=true,confirmation_status='pending' WHERE id=78011`, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = resolutionpg.AttachAmendmentTx(ctx, tx, schema, out.AmendmentID, 78011, "sales"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var from int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT amended_from_order_id FROM %s.orders WHERE id=78011`, schema)).Scan(&from); err != nil || from != 78010 {
		t.Fatalf("source=%d err=%v", from, err)
	}
}

func TestOrderResolutionWithdrawRestoresPauseOnlyBeforeEffects(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78012,'WITHDRAW-TEST',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78012,78012,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78012)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "unknown", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "withdraw-test"}
	out, err := repo.Submit(ctx, 78012, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Withdraw(ctx, 78012, out.ID, "录入信息需重新核对", "sales"); err != nil {
		t.Fatal(err)
	}
	c, err = repo.Context(ctx, 78012)
	if err != nil || c.Paused || c.CreditCents != 0 || c.Status != "open" {
		t.Fatalf("withdraw context=%+v err=%v", c, err)
	}
}

func TestOrderResolutionLegacyVoidCannotBypassExplicitChoices(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(id,order_no,customer_id,total_amount)VALUES(78013,'LEGACY-VOID',501,90)`, schema)); err != nil {
		t.Fatal(err)
	}
	if n, err := salespg.NewRepository(pool, schema).VoidMany(ctx, []int64{78013}, "sales", "取消"); err == nil || n != 0 {
		t.Fatalf("legacy bypass: n=%d err=%v", n, err)
	}
	var invalid bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_void FROM %s.orders WHERE id=78013`, schema)).Scan(&invalid); err != nil || invalid {
		t.Fatalf("legacy changed real state: %v %v", invalid, err)
	}
}

func TestOrderResolutionPendingPreventsChangedQuantitiesAndNewProduction(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78014,'PENDING-GUARD',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78014,78014,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78014,'PENDING-NEW-PLAN','draft')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78014)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Submit(ctx, 78014, resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "unknown", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "pending-guard"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE %s.order_items SET qty=4 WHERE id=78014`, `UPDATE %s.orders SET total_amount=120 WHERE id=78014`, `INSERT INTO %s.production_plan_items(production_plan_id,product_id,product_name,spec_g,planned_g,order_nos)VALUES(78014,911,'coffee',1000,3000,'PENDING-GUARD')`} {
		if _, err = pool.Exec(ctx, fmt.Sprintf(q, schema)); err == nil {
			t.Fatalf("pending mutation accepted: %s", q)
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET order_date='2026-11-06' WHERE id=78014`, schema)); err != nil {
		t.Fatalf("ordinary date correction blocked: %v", err)
	}
}

func TestOrderResolutionSplitRefundBalanceAndOffsetReconciles(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,order_date,customer_id,total_amount,grand_total,prepayment_amount,payment_goods_amount)VALUES(78015,'SPLIT-SOURCE','2026-11-07',501,90,90,90,90),(78016,'SPLIT-TARGET','2026-11-07',501,90,90,0,50);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78015,78015,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78015)
	if err != nil {
		t.Fatal(err)
	}
	out, err := repo.Submit(ctx, 78015, resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "split-funds"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	targets, e := repo.Targets(ctx, 78015)
	if e != nil {
		t.Fatal(e)
	}
	var found bool
	for _, target := range targets {
		if target.ID == 78016 {
			found = true
			if target.AvailableCents != 4000 {
				t.Fatalf("target balance=%+v", target)
			}
		}
		if target.ID == 78015 {
			t.Fatal("source offered as own offset target")
		}
	}
	if !found {
		t.Fatal("same-customer receivable not selectable")
	}
	for i, v := range []struct {
		choice string
		amount int64
	}{{"refund", 3000}, {"customer_credit", 2000}, {"offset", 4000}} {
		done, e := repo.CompleteStep(ctx, 78015, out.ID, out.Steps[0].ID, resolutionpg.StepCommand{Choice: v.choice, AmountCents: v.amount, Account: "银行", Reference: fmt.Sprintf("split-%d", i), Note: "核实处置", PostingDate: "2026-11-07", TargetID: 78016, IdempotencyKey: fmt.Sprintf("split-%d", i)}, "finance")
		if e != nil {
			t.Fatal(e)
		}
		if (i < 2 && done.Status != "pending") || (i == 2 && done.Status != "completed") {
			t.Fatalf("step=%d request=%+v", i, done)
		}
	}
	lines, err := customerSettlementOrderChargesTx(ctx, pool, schema, 501, "2026-11-01", "2026-11-30", true, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if line.SourceID == 78016 && line.DueCents != 0 {
			t.Fatalf("offset still collectible: %+v", line)
		}
	}
	var received float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT payment_goods_amount::float8 FROM %s.orders WHERE id=78016`, schema)).Scan(&received); err != nil || received != 50 {
		t.Fatalf("offset rewrote cash payment: %v %v", received, err)
	}
	c, err = repo.Context(ctx, 78015)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.Submit(ctx, 78015, resolution.Command{Action: "reopen", Reason: "other", Note: "重新履约一个", Scope: "selected", Lines: []resolution.Selection{{ItemID: 78015, Quantity: 1}}, Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "reopen-after-funds"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	lines, err = customerSettlementOrderChargesTx(ctx, pool, schema, 501, "2026-11-01", "2026-11-30", true, 100)
	if err != nil {
		t.Fatal(err)
	}
	var due int64
	for _, line := range lines {
		if line.SourceID == 78015 {
			due += line.DueCents
		}
	}
	if due != 3000 {
		t.Fatalf("reopened obligation must be unpaid after settled money: %d", due)
	}
	list, e := salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{OrderID: 78015, Limit: 10})
	if e != nil {
		t.Fatal(e)
	}
	if list.Summary.PendingSettlementAmount != "30.00" {
		t.Fatalf("sales summary omitted restored debt: %+v", list.Summary)
	}
	balances, err := repo.Balances(ctx, 78015)
	if err != nil || len(balances) != 1 || balances[0].AvailableCents != 2000 {
		t.Fatalf("retained balance missing: %+v %v", balances, err)
	}
	use := resolutionpg.BalanceCommand{SourceEntryID: balances[0].ID, ExpectedAvailableCents: 2000, ExpectedTargetDueCents: 3000, StepCommand: resolutionpg.StepCommand{Choice: "offset", AmountCents: 1000, TargetID: 78015, PostingDate: "2026-11-07", Note: "客户指定抵扣重新开启应收", IdempotencyKey: "balance-offset"}}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_balance_audit()RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='customer_balance_use' THEN RAISE EXCEPTION 'test audit failure';END IF;RETURN NEW;END $$;CREATE TRIGGER fail_balance_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_balance_audit()`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err == nil {
		t.Fatal("balance succeeded without audit")
	}
	failedUses, err := repo.BalanceHistory(ctx, 78015)
	if err != nil || len(failedUses) != 0 {
		t.Fatalf("balance write survived audit failure: %+v %v", failedUses, err)
	}
	list, e = salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{OrderID: 78015, Limit: 10})
	if e != nil || list.Summary.PendingSettlementAmount != "30.00" {
		t.Fatalf("offset survived audit rollback: %+v %v", list.Summary, e)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER fail_balance_audit ON %s.audit_logs`, schema)); err != nil {
		t.Fatal(err)
	}
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err != nil {
		t.Fatal(err)
	}
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err != nil {
		t.Fatalf("balance replay: %v", err)
	}
	use.Choice = "refund"
	use.ExpectedAvailableCents = 1000
	use.IdempotencyKey = "balance-refund"
	use.Account = "银行"
	use.Reference = "balance-payment"
	use.AmountCents = 1001
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err == nil {
		t.Fatal("balance overdraw allowed")
	}
	use.AmountCents = 500
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err != nil {
		t.Fatal(err)
	}
	use.ExpectedAvailableCents = 500
	use.IdempotencyKey = "different-key-same-voucher"
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err == nil {
		t.Fatal("same bank voucher refunded same balance twice")
	}
	use.Reference = "second-balance-payment"
	use.IdempotencyKey = "second-balance-refund"
	if err = repo.UseBalance(ctx, 78015, use, "finance"); err != nil {
		t.Fatal(err)
	}
	balances, err = repo.Balances(ctx, 78015)
	if err != nil || len(balances) != 0 {
		t.Fatalf("balance spent twice: %+v %v", balances, err)
	}
	list, e = salespg.NewRepository(pool, schema).ListOrders(ctx, salesapp.OrderListQuery{OrderID: 78015, Limit: 10})
	if e != nil {
		t.Fatal(e)
	}
	uses, err := repo.BalanceHistory(ctx, 78015)
	if err != nil || len(uses) != 3 {
		t.Fatalf("balance history lost: %+v %v", uses, err)
	}
	if list.Summary.PendingSettlementAmount != "20.00" {
		t.Fatalf("balance use changed original cash or double deducted: %+v", list.Summary)
	}

}

func TestOrderResolutionPolicyRevisionAndPendingEnforcement(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	repo := resolutionpg.NewRepository(pool, schema)
	initial, err := repo.Policy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	policy := resolution.Policy{Approvals: map[string]string{"close": "finance.resolve"}}
	saved, err := repo.SavePolicy(ctx, initial.Revision, policy, "employee:703")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != initial.Revision+1 {
		t.Fatal(saved)
	}
	if _, err = repo.SavePolicy(ctx, initial.Revision, policy, "employee:703"); err == nil {
		t.Fatal("stale policy overwrite allowed")
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78100,'POLICY-ORDER',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78100,78100,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	c, err := repo.Context(ctx, 78100)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "stop_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "policy"}
	result, err := repo.Submit(ctx, 78100, cmd, "employee:703")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || len(result.Steps) != 1 || result.Steps[0].Permission != "finance.resolve" {
		t.Fatalf("approval bypass: %+v", result)
	}
	policy.Disabled = map[string][]string{"action": {"close"}}
	if _, err = repo.SavePolicy(ctx, saved.Revision, policy, "employee:703"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CompleteStep(ctx, 78100, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "approve", Note: "reviewed", IdempotencyKey: "approve"}, "employee:703"); err == nil {
		t.Fatal("pending action ignored current policy")
	}
	var audits int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='resolution_policy'`, schema)).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}

func TestOrderResolutionFinalApplyRechecksNewPayments(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78101,'PENDING-PAYMENT',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78101,78101,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78101)
	if err != nil {
		t.Fatal(err)
	}
	result, err := repo.Submit(ctx, 78101, resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "unknown", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "before-payment"}, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET prepayment_amount=40 WHERE id=78101`, schema)); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteStep(ctx, 78101, result.ID, result.Steps[0].ID, resolutionpg.StepCommand{Choice: "none", Note: "核实未开票", IdempotencyKey: "external-done"}, "finance")
	if err == nil {
		t.Fatal("stale plan finalized without allocating newly received money")
	}
	history, err := repo.History(ctx, 78101)
	if err != nil {
		t.Fatal(err)
	}
	if history[0].Applied || history[0].Steps[0].Status != "failed" {
		t.Fatalf("failed finalization left partial effects: %+v", history[0])
	}

	current, err := repo.Context(ctx, 78101)
	if err != nil {
		t.Fatal(err)
	}
	refreshedCommand := result.Command
	refreshedCommand.Version = current.Version
	refreshedCommand.IdempotencyKey = "refresh-after-payment"
	refreshed, err := repo.Refresh(ctx, 78101, result.ID, refreshedCommand, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed.Steps) != 2 || refreshed.Plan.CreditCents != 9000 {
		t.Fatalf("refresh missing new funds task: %+v", refreshed)
	}
	again, err := repo.Refresh(ctx, 78101, result.ID, refreshedCommand, "sales")
	if err != nil || len(again.Steps) != 2 {
		t.Fatalf("refresh replay: %+v %v", again, err)
	}
	done, err := repo.CompleteStep(ctx, 78101, result.ID, refreshed.Steps[0].ID, resolutionpg.StepCommand{Choice: "none", Note: "核实未开票", IdempotencyKey: "external-done"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if done.Applied {
		t.Fatal("settle-first applied before funds disposition")
	}
	done, err = repo.CompleteStep(ctx, 78101, result.ID, refreshed.Steps[1].ID, resolutionpg.StepCommand{Choice: "refund", AmountCents: 4000, Account: "bank", Reference: "return-40", PostingDate: "2026-11-07", Note: "已退", IdempotencyKey: "funds-done"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if !done.Applied || done.Status != "completed" {
		t.Fatalf("refresh did not recover: %+v", done)
	}
}

func TestOrderResolutionFreightWaiverIsExplicitAuditedAndKeepsOriginalAmount(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount,shipping_amount,grand_total)VALUES(78102,'FREIGHT-DISPOSITION',501,90,10,100);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78102,78102,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78102)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "all", Timing: "settle_first", ExternalStatus: "none", PostingMode: "open_period", PostingDate: "2026-11-07", Version: c.Version, IdempotencyKey: "fee-waive"}
	if _, err = repo.Submit(ctx, 78102, cmd, "sales"); err == nil {
		t.Fatal("freight disposition was implicit")
	}
	cmd.Dispositions = []resolution.Disposition{{Kind: "fee", ID: -1, Choice: "waive"}}
	out, err := repo.Submit(ctx, 78102, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied || len(out.Steps) != 1 || out.Plan.CreditCents != 10000 {
		t.Fatalf("fee approval missing: %+v", out)
	}
	out, err = repo.CompleteStep(ctx, 78102, out.ID, out.Steps[0].ID, resolutionpg.StepCommand{Choice: "waive", Note: "客户取消未发生运费，批准全免", IdempotencyKey: "fee-confirm"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Applied {
		t.Fatal("fee approval did not finish")
	}
	lines, err := customerSettlementOrderChargesTx(ctx, pool, schema, 501, "1900-01-01", "2999-12-31", true, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		if line.SourceID == 78102 && line.ReceivableCents != 0 {
			t.Fatalf("waived fee remains payable: %+v", line)
		}
	}

	var original float64
	var credit int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT shipping_amount,resolution_credit_cents FROM %s.orders WHERE id=78102`, schema)).Scan(&original, &credit); err != nil || original != 10 || credit != 10000 {
		t.Fatalf("original=%v credit=%d err=%v", original, credit, err)
	}
}

func TestOrderResolutionPauseBlocksIncreasingExistingReservation(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78103,'PAUSE-RESERVATION',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78103,78103,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.order_stock_batch_allocations(id,order_id,product_id,spec_g,allocated_units,allocated_g)VALUES(78103,78103,911,1000,1,1000)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78103)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Submit(ctx, 78103, resolution.Command{Action: "hold", Reason: "customer_cancel", Version: c.Version, IdempotencyKey: "pause"}, "sales"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_stock_batch_allocations SET allocated_units=2,allocated_g=2000 WHERE id=78103`, schema)); err == nil {
		t.Fatal("paused reservation increased through UPDATE")
	}
}

func TestOrderResolutionDraftSharedPlanIsVisibleAndVersioned(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,total_amount)VALUES(78104,'DRAFT-SHARED',501,90),(78105,'DRAFT-OTHER',501,90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78104,78104,1,911,'coffee','1000g',3,'袋',30,90);INSERT INTO %[1]s.production_plans(id,plan_no,status)VALUES(78104,'PP-DRAFT-SHARED','draft');INSERT INTO %[1]s.production_plan_items(id,production_plan_id,product_id,product_name,spec_g,planned_g,order_nos)VALUES(78104,78104,911,'coffee',1000,6000,'DRAFT-SHARED,DRAFT-OTHER')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78104)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Dependencies) != 1 || !c.Dependencies[0].Shared {
		t.Fatalf("shared draft not identified: %+v", c.Dependencies)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_plan_items SET planned_g=7000 WHERE id=78104`, schema)); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Context(ctx, 78104)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version == c.Version {
		t.Fatal("plan item changes did not invalidate preview")
	}
}

func TestOrderResolutionHistoricalPeriodRebuildsOnlyAfterApprovedEffects(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	seedMiniDirectShipStock(t, pool, schema)
	ctx := context.Background()
	if err := financepg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.orders(id,order_no,customer_id,order_date,total_amount)VALUES(78230,'HISTORICAL-APPROVAL',501,'2026-10-01',90);INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,item_name,spec,qty,unit,unit_price,line_total)VALUES(78230,78230,1,911,'coffee','1000g',3,'袋',30,90)`, schema))
	if err != nil {
		t.Fatal(err)
	}
	fin := financepg.NewRepository(pool, schema)
	report, err := fin.CloseMonthAtomic(ctx, financeapp.CloseMonthCommand{Month: "2026-10", Actor: "finance"})
	if err != nil {
		t.Fatal(err)
	}
	if report.RevenueTaxInclusive != 90 {
		t.Fatalf("initial report=%+v", report)
	}
	var before string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json::text FROM %s.finance_monthly_reports WHERE month='2026-10'`, schema)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	repo := resolutionpg.NewRepository(pool, schema)
	c, err := repo.Context(ctx, 78230)
	if err != nil {
		t.Fatal(err)
	}
	cmd := resolution.Command{Action: "close", Reason: "customer_cancel", Scope: "selected", Lines: []resolution.Selection{{ItemID: 78230, Quantity: 1}}, Timing: "settle_first", ExternalStatus: "unknown", PostingMode: "historical_correction", PostingDate: "2026-10-31", Version: c.Version, IdempotencyKey: "historical-close"}
	result, err := repo.Submit(ctx, 78230, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var periodStep, externalStep int64
	for _, step := range result.Steps {
		if step.Kind == "period" {
			periodStep = step.ID
		}
		if step.Kind == "external" {
			externalStep = step.ID
		}
	}
	_, err = repo.CompleteStep(ctx, 78230, result.ID, periodStep, resolutionpg.StepCommand{Choice: "approve", Note: "核对历史更正范围", Reference: "approval-001", IdempotencyKey: "approve-history"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	var state, interim string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,snapshot_json::text FROM %s.finance_monthly_reports WHERE month='2026-10'`, schema)).Scan(&state, &interim); err != nil {
		t.Fatal(err)
	}
	if state != "closed" || interim != before {
		t.Fatal("approval changed the closed report before business effects were applied")
	}
	// Another approved rebuild changes the month after this request's approval.
	// It must invalidate this approval without applying any order effects.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finance_monthly_reports SET snapshot_json=snapshot_json||'{"concurrent_revision":"other-approved-correction"}'::jsonb WHERE month='2026-10'`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CompleteStep(ctx, 78230, result.ID, externalStep, resolutionpg.StepCommand{Choice: "none", Note: "已核实未开票", IdempotencyKey: "before-reapproval"}, "finance"); err == nil {
		t.Fatal("historical approval reused after report changed")
	}
	var closed float64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT resolution_closed_qty::float8 FROM %s.order_items WHERE id=78230`, schema)).Scan(&closed); err != nil || closed != 0 {
		t.Fatalf("stale approval partially applied: %v %v", closed, err)
	}
	c, err = repo.Context(ctx, 78230)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Version = c.Version
	cmd.IdempotencyKey = "refresh-historical"
	result, err = repo.Refresh(ctx, 78230, result.ID, cmd, "sales")
	if err != nil {
		t.Fatal(err)
	}
	var newPeriod int64
	for _, step := range result.Steps {
		if step.Kind == "period" && step.Status == "pending" {
			newPeriod = step.ID
		}
	}
	if newPeriod == 0 || newPeriod == periodStep {
		t.Fatal("refresh did not request a new financial approval")
	}
	_, err = repo.CompleteStep(ctx, 78230, result.ID, newPeriod, resolutionpg.StepCommand{Choice: "approve", Note: "重新核对最新快照", Reference: "approval-002", IdempotencyKey: "reapprove-history"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT snapshot_json::text FROM %s.finance_monthly_reports WHERE month='2026-10'`, schema)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CompleteStep(ctx, 78230, result.ID, externalStep, resolutionpg.StepCommand{Choice: "none", Note: "已核实未开票", IdempotencyKey: "external-clear"}, "finance")
	if err != nil {
		t.Fatal(err)
	}
	var revenue float64
	var revisions int
	var preserved bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,(snapshot_json->>'revenue_tax_inclusive')::float8,(SELECT count(*)FROM %[1]s.order_resolution_period_revisions WHERE request_id=$1),(SELECT before_json::text=$2 FROM %[1]s.order_resolution_period_revisions WHERE request_id=$1)FROM %[1]s.finance_monthly_reports WHERE month='2026-10'`, schema), result.ID, before).Scan(&state, &revenue, &revisions, &preserved); err != nil {
		t.Fatal(err)
	}
	if state != "adjusted" || revenue != 60 || revisions != 1 || !preserved {
		t.Fatalf("historical correction=%s/%v revisions=%d preserved=%v", state, revenue, revisions, preserved)
	}
}
