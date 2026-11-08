package customerfulfillment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	financeapp "orderapp/internal/application/finance"
	domain "orderapp/internal/domain/orderresolution"
	financepg "orderapp/internal/infrastructure/postgres/finance"
	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
	migration "orderapp/internal/infrastructure/postgres/productspecmigration"
	"testing"
)

func companyScrapFixture(t *testing.T) (*pgxpool.Pool, string) {
	pool, schema := companyAllocationFixture(t)
	if err := financepg.EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := resolutionpg.EnsureSchema(context.Background(), pool, schema); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(context.Background(), fmt.Sprintf(`UPDATE %[1]s.stock_batches SET unit_cost=10 WHERE id=78400;INSERT INTO %[1]s.production_batch_costs(id,running_item_id,batch_id,total_cost,created_at)VALUES(78400,78400,'ALLOC-RUN',50,'2026-10-30')`, schema))
	if err != nil {
		t.Fatal(err)
	}
	return pool, schema
}
func companyScrapCommand(version, treatment string) domain.Command {
	cmd := companyAllocationCommand(version)
	cmd.IdempotencyKey = "company-scrap"
	cmd.Dispositions = nil
	_ = json.Unmarshal([]byte(`[{"kind":"production","id":78400,"choice":"scrap","reference":"验收破损，三袋按原批次报损","cost_treatment":"`+treatment+`","stock_selections":[{"batch_id":78400,"quantity":3}]}]`), &cmd.Dispositions)
	return cmd
}
func TestOrderResolutionCompanyScrapPreservesSharedDemandAndOriginalCost(t *testing.T) {
	for _, treatment := range []string{"retain_cost", "reclassify_loss", "historical"} {
		t.Run(treatment, func(t *testing.T) {
			pool, schema := companyScrapFixture(t)
			ctx := context.Background()
			if treatment == "historical" {
				treatment = "retain_cost"
				if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=7030;INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit)VALUES(7033,7030,'V2','published',1,'袋');INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,logistics_weight_g)VALUES(7034,7033,7031,'454g袋','袋',true,454);UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=7033 WHERE output_type='product' AND output_id=910`, schema)); err != nil {
					t.Fatal(err)
				}
				if err := migration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
					t.Fatal(err)
				}
			}

			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Dependencies) != 1 || len(c.Dependencies[0].CompanyStock) != 1 || c.Dependencies[0].CompanyStock[0].CostRecordID != 78400 {
				t.Fatalf("missing proven original cost: %+v", c.Dependencies)
			}
			result, err := r.Submit(ctx, 78400, companyScrapCommand(c.Version, treatment), "sales")
			if err != nil {
				t.Fatal(err)
			}
			var finance, warehouse resolutionpg.Step
			for _, step := range result.Steps {
				if step.Permission == "finance.resolve" {
					finance = step
				}
				if step.Permission == "stock.resolve" {
					warehouse = step
				}
			}
			confirm := resolutionpg.StepCommand{Choice: "scrap", Note: "原批次三袋报损，其他订单两袋保留", Reference: "LOSS-CHECK-1", IdempotencyKey: "warehouse"}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, warehouse.ID, confirm, "warehouse"); err == nil {
				t.Fatal("warehouse loss before finance authorization")
			}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, finance.ID, resolutionpg.StepCommand{Choice: treatment, Reference: "LOSS-APPROVE-1", Note: "核对原成本及账期", IdempotencyKey: "finance"}, "finance"); err != nil {
				t.Fatal(err)
			}
			for n := 0; n < 2; n++ {
				result, err = r.CompleteStep(ctx, 78400, result.ID, warehouse.ID, confirm, "warehouse")
				if err != nil {
					t.Fatal(err)
				}
			}
			if !result.Applied {
				t.Fatal("loss and close not applied")
			}
			var batch, inventory, quantity, costCents, adjustments int64
			var originalCost, adjustmentTotal float64
			var plan, other string
			err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78400),(SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND warehouse='finished_goods'),(SELECT sum(quantity) FROM %[1]s.order_resolution_company_scraps),(SELECT sum(cost_cents) FROM %[1]s.order_resolution_company_scraps),(SELECT count(*) FROM %[1]s.finance_adjustments),(SELECT total_cost FROM %[1]s.production_batch_costs WHERE id=78400),(SELECT COALESCE(sum(amount),0) FROM %[1]s.finance_adjustments),(SELECT status FROM %[1]s.production_plans WHERE id=78400),(SELECT resolution_status FROM %[1]s.orders WHERE id=78402)`, schema)).Scan(&batch, &inventory, &quantity, &costCents, &adjustments, &originalCost, &adjustmentTotal, &plan, &other)
			if err != nil {
				t.Fatal(err)
			}
			for _, statement := range []string{
				`DELETE FROM %[1]s.order_resolution_company_scraps`,
				`UPDATE %[1]s.stock_ledger_entries SET qty_change_units=-1 WHERE source_doc_type='order_resolution_company_scrap'`,
				`UPDATE %[1]s.production_batch_costs SET total_cost=60 WHERE id=78400`,
				`UPDATE %[1]s.stock_batches SET unit_cost=20 WHERE id=78400`,
			} {
				tx, e := pool.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				_, e = tx.Exec(ctx, fmt.Sprintf(statement, schema))
				_ = tx.Rollback(ctx)
				if e == nil {
					t.Errorf("loss history editable: %s", statement)
				}
			}
			expected := int64(0)
			if treatment == "reclassify_loss" {
				expected = 2
			}
			service := financeapp.NewService(financepg.NewRepository(pool, schema))
			original, err := service.DraftReport(ctx, financeapp.ReportFilter{Month: "2026-10"})
			if err != nil {
				t.Fatal(err)
			}
			posting, err := service.DraftReport(ctx, financeapp.ReportFilter{Month: "2026-11"})
			if err != nil {
				t.Fatal(err)
			}
			if original.MainBusinessCost != 50 {
				t.Fatalf("original cost month changed: %+v", original)
			}
			expectedCost, expectedExpense := 0.0, 0.0
			if treatment == "reclassify_loss" {
				expectedCost = -30
				expectedExpense = 30
			}
			if float64(posting.AdjustedMainBusinessCost) != expectedCost || float64(posting.AdjustedPeriodExpenses) != expectedExpense {
				t.Fatalf("loss report double count or missing reclassification: %+v", posting)
			}
			if batch != 2 || inventory != 2 || quantity != 3 || costCents != 3000 || originalCost != 50 || adjustmentTotal != 0 || adjustments != expected || plan != "completed" || other != "open" {
				t.Fatalf("loss reconciliation: batch=%d inventory=%d qty=%d cost=%d original=%g adjustments=%d/%g plan=%s other=%s", batch, inventory, quantity, costCents, originalCost, adjustments, adjustmentTotal, plan, other)
			}
		})
	}
}

func TestOrderResolutionCompanyScrapStaleFactsAndAuditRollback(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"closed posting period", `INSERT INTO %[1]s.finance_monthly_reports(month,status)VALUES('2026-11','closed')`},
		{"physical shortage", `UPDATE %[1]s.stock_batches SET remaining_units=2 WHERE id=78400;UPDATE %[1]s.finished_inventory SET onhand_units=2 WHERE product_id=910 AND warehouse='finished_goods'`},
		{"batch cost", `UPDATE %[1]s.stock_batches SET unit_cost=11 WHERE id=78400`},
		{"source cost", `UPDATE %[1]s.production_batch_costs SET total_cost=55 WHERE id=78400`},
		{"shared other demand", `UPDATE %[1]s.order_items SET qty=3,line_total=90 WHERE id=78402`},
		{"other reservation", `INSERT INTO %[1]s.order_stock_batch_allocations(order_id,order_item_id,product_id,bom_spec_id,bom_variant_id,batch_id,batch_code,warehouse,allocated_units)VALUES(78401,78401,910,7031,7032,78400,'ALLOC-COMPANY-BATCH','finished_goods',3)`},
		{"audit rollback", `CREATE FUNCTION %[1]s.fail_scrap_audit()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.action='resolution_company_scrap' THEN RAISE EXCEPTION 'forced scrap audit';END IF;RETURN NEW;END$$;CREATE TRIGGER fail_scrap_audit BEFORE INSERT ON %[1]s.audit_logs FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_scrap_audit()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := companyScrapFixture(t)
			ctx := context.Background()
			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Submit(ctx, 78400, companyScrapCommand(c.Version, "reclassify_loss"), "sales")
			if err != nil {
				t.Fatal(err)
			}
			var warehouse resolutionpg.Step
			for _, s := range result.Steps {
				if s.Kind == "production_finance" {
					if _, err = r.CompleteStep(ctx, 78400, result.ID, s.ID, resolutionpg.StepCommand{Choice: s.Choice, Note: "确认成本", Reference: "FIN-LOSS", IdempotencyKey: "finance"}, "finance"); err != nil {
						t.Fatal(err)
					}
				} else {
					warehouse = s
				}
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.change, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, warehouse.ID, resolutionpg.StepCommand{Choice: "scrap", Reference: "LOSS", Note: "实物报损", IdempotencyKey: "warehouse"}, "warehouse"); err == nil {
				t.Fatal("stale or unaudited loss accepted")
			}
			var batch, inventory, records, journals, credit int
			err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78400),(SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=910 AND warehouse='finished_goods'),(SELECT count(*) FROM %[1]s.order_resolution_company_scraps),(SELECT count(*) FROM %[1]s.finance_adjustments),(SELECT resolution_credit_cents FROM %[1]s.orders WHERE id=78400)`, schema)).Scan(&batch, &inventory, &records, &journals, &credit)
			if err != nil {
				t.Fatal(err)
			}
			wantStock := 5
			if tc.name == "physical shortage" {
				wantStock = 2
			}
			if batch != wantStock || inventory != wantStock || records != 0 || journals != 0 || credit != 0 {
				t.Fatalf("partial loss escaped: %d %d %d %d %d", batch, inventory, records, journals, credit)
			}
		})
	}
}
func TestOrderResolutionCompanyScrapRefreshRequiresNewCostApproval(t *testing.T) {
	pool, schema := companyScrapFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, companyScrapCommand(c.Version, "retain_cost"), "sales")
	if err != nil {
		t.Fatal(err)
	}
	var oldFinance int64
	for _, step := range result.Steps {
		if step.Kind == "production_finance" {
			oldFinance = step.ID
			_, err = r.CompleteStep(ctx, 78400, result.ID, step.ID, resolutionpg.StepCommand{Choice: step.Choice, Reference: "FIN-1", Note: "原成本批准", IdempotencyKey: "finance"}, "finance")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET unit_cost=9 WHERE id=78400`, schema)); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		c, err = r.Context(ctx, 78400)
		if err != nil {
			t.Fatal(err)
		}
		cmd := companyScrapCommand(c.Version, "retain_cost")
		cmd.IdempotencyKey = fmt.Sprintf("refresh-%d", n)
		result, err = r.Refresh(ctx, 78400, result.ID, cmd, "sales")
		if err != nil {
			t.Fatal(err)
		}
		pending := 0
		for _, step := range result.Steps {
			if step.ID == oldFinance && step.Status != "superseded" {
				t.Fatalf("old finance approval still reusable: %+v", step)
			}
			if step.Status == "pending" {
				pending++
			}
		}
		if pending != 2 {
			t.Fatalf("new source requires both confirmations: %+v", result.Steps)
		}
	}
}

func TestOrderResolutionCompanyScrapHeldBatchNeedsExplicitApprovalAndCannotTransfer(t *testing.T) {
	pool, schema := companyScrapFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET quality_status='reject' WHERE id=78400`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Dependencies) != 1 || len(c.Dependencies[0].CompanyStock) != 1 {
		t.Fatalf("rejected company batch unavailable for approved loss: %+v", c.Dependencies)
	}
	if len(c.Dependencies[0].CompanyStock[0].Targets) > 0 {
		t.Fatal("rejected stock offered to another order")
	}
	result, err := r.Submit(ctx, 78400, companyScrapCommand(c.Version, "retain_cost"), "sales")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Steps {
		if _, err = r.CompleteStep(ctx, 78400, result.ID, s.ID, resolutionpg.StepCommand{Choice: s.Choice, Note: "拒收批次确认报损", Reference: "REJECT-LOSS", IdempotencyKey: fmt.Sprint(s.ID)}, "approved-role"); err != nil {
			t.Fatal(err)
		}
	}
	assertMiniDirectShipCount(t, pool, schema, "stock_batches", "id=78400 AND remaining_units=2 AND quality_status='reject'", 1)
}

func TestOrderResolutionCompanyScrapConcurrentPostingAndReopen(t *testing.T) {
	pool, schema := companyScrapFixture(t)
	ctx := context.Background()
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Submit(ctx, 78400, companyScrapCommand(c.Version, "reclassify_loss"), "sales")
	if err != nil {
		t.Fatal(err)
	}
	var warehouse int64
	for _, s := range result.Steps {
		if s.Kind == "production_finance" {
			_, err = r.CompleteStep(ctx, 78400, result.ID, s.ID, resolutionpg.StepCommand{Choice: s.Choice, Note: "明确原成本", Reference: "FIN", IdempotencyKey: "finance"}, "finance")
			if err != nil {
				t.Fatal(err)
			}
		} else {
			warehouse = s.ID
		}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func(n int) {
			<-start
			_, err := r.CompleteStep(ctx, 78400, result.ID, warehouse, resolutionpg.StepCommand{Choice: "scrap", Note: "核对批次报损", Reference: "LOSS", IdempotencyKey: fmt.Sprint("parallel", n)}, "warehouse")
			results <- err
		}(n)
	}
	close(start)
	success := 0
	for n := 0; n < 2; n++ {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("posting success=%d", success)
	}
	assertMiniDirectShipCount(t, pool, schema, "order_resolution_company_scraps", "status='posted'", 1)
	assertMiniDirectShipCount(t, pool, schema, "stock_ledger_entries", "source_doc_type='order_resolution_company_scrap'", 1)
	assertMiniDirectShipCount(t, pool, schema, "audit_logs", "action='resolution_company_scrap'", 1)
	assertMiniDirectShipCount(t, pool, schema, "finance_adjustments", "type IN ('main_cost','expense')", 2)
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finance_adjustments SET amount=0`, schema)); err == nil {
		t.Fatal("posted journal editable")
	}
	if err = resolutionpg.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	c, err = r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	cmd := companyScrapCommand(c.Version, "retain_cost")
	cmd.Action = "reopen"
	cmd.IdempotencyKey = "reopen"
	cmd.Dispositions = []domain.Disposition{{Kind: "production", ID: 78400, Choice: "keep_company_stock"}}
	if _, err = r.Submit(ctx, 78400, cmd, "sales"); err != nil {
		t.Fatal(err)
	}
	assertMiniDirectShipCount(t, pool, schema, "stock_batches", "id=78400 AND remaining_units=2", 1)
	assertMiniDirectShipCount(t, pool, schema, "finance_adjustments", "amount<>0", 2)
}

func TestOrderResolutionCompanyScrapCannotPrecedeOriginalCost(t *testing.T) {
	pool, schema := companyScrapFixture(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_batch_costs SET created_at='2026-11-09' WHERE id=78400`, schema)); err != nil {
		t.Fatal(err)
	}
	r := resolutionpg.NewRepository(pool, schema)
	c, err := r.Context(ctx, 78400)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Submit(ctx, 78400, companyScrapCommand(c.Version, "reclassify_loss"), "sales"); err == nil {
		t.Fatal("loss precedes source cost within same month")
	}
}
