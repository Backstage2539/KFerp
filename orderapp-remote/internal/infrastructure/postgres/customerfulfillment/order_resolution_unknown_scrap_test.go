package customerfulfillment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	resolutionpg "orderapp/internal/infrastructure/postgres/orderresolution"
)

func TestCompanyScrapRejectsUnknownCostBeforeAndAfterApproval(t *testing.T) {
	t.Run("known_cost_posts", func(t *testing.T) {
		pool, schema := companyScrapFixture(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET created_at='2026-11-01' WHERE id=78400`, schema)); err != nil {
			t.Fatal(err)
		}
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
		if _, err = r.CompleteStep(ctx, 78400, result.ID, warehouse.ID, resolutionpg.StepCommand{Choice: "scrap", Reference: "LOSS", Note: "实物报损", IdempotencyKey: "warehouse"}, "warehouse"); err != nil {
			t.Fatal(err)
		}
		var batch, records, journals int
		if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78400),(SELECT count(*) FROM %[1]s.order_resolution_company_scraps),(SELECT count(*) FROM %[1]s.finance_adjustments)`, schema)).Scan(&batch, &records, &journals); err != nil {
			t.Fatal(err)
		}
		if batch != 2 || records != 1 || journals != 2 {
			t.Fatalf("known cost failed: %d %d %d", batch, records, journals)
		}
	})
	for _, table := range []string{"stock_batches", "production_batch_costs"} {
		t.Run(table+"_preview", func(t *testing.T) {
			pool, schema := companyScrapFixture(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET created_at='2026-11-01' WHERE id=78400`, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.%[2]s ADD COLUMN IF NOT EXISTS cost_unavailable boolean NOT NULL DEFAULT false; UPDATE %[1]s.%[2]s SET cost_unavailable=true WHERE id=78400`, schema, table)); err != nil {
				t.Fatal(err)
			}
			r := resolutionpg.NewRepository(pool, schema)
			c, err := r.Context(ctx, 78400)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Dependencies) != 1 || len(c.Dependencies[0].CompanyStock) != 1 || !strings.Contains(c.Dependencies[0].CompanyStock[0].ScrapIssue, "成本待核算") {
				t.Fatalf("unknown cost missing from preview: %+v", c.Dependencies)
			}
			if _, err = r.Submit(ctx, 78400, companyScrapCommand(c.Version, "reclassify_loss"), "sales"); err == nil {
				t.Fatal("unknown cost accepted")
			}
		})
		t.Run(table+"_after_approval", func(t *testing.T) {
			pool, schema := companyScrapFixture(t)
			ctx := context.Background()
			if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET created_at='2026-11-01' WHERE id=78400`, schema)); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %[1]s.%[2]s ADD COLUMN IF NOT EXISTS cost_unavailable boolean NOT NULL DEFAULT false`, schema, table)); err != nil {
				t.Fatal(err)
			}
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
			if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.%[2]s SET cost_unavailable=true WHERE id=78400`, schema, table)); err != nil {
				t.Fatal(err)
			}
			if _, err = r.CompleteStep(ctx, 78400, result.ID, warehouse.ID, resolutionpg.StepCommand{Choice: "scrap", Reference: "LOSS", Note: "实物报损", IdempotencyKey: "warehouse"}, "warehouse"); err == nil {
				t.Fatal("unknown cost posted after approval")
			}
			var batch, records, journals int
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT (SELECT remaining_units FROM %[1]s.stock_batches WHERE id=78400),(SELECT count(*) FROM %[1]s.order_resolution_company_scraps),(SELECT count(*) FROM %[1]s.finance_adjustments)`, schema)).Scan(&batch, &records, &journals); err != nil {
				t.Fatal(err)
			}
			if batch != 5 || records != 0 || journals != 0 {
				t.Fatalf("partial change: %d %d %d", batch, records, journals)
			}
		})
	}
}
