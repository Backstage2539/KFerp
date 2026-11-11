package production

import (
	"context"
	"fmt"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
)

// Stock existing in a source warehouse cannot be treated as already present
// in WIP, including through the shared bulk-submit path.
func TestPlanSubmitBlocksSourceStockNotYetInWIP(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedProductionPlanLifecycleData(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 10, 10, "MB-PLAN-RAW", "计划生豆", 1000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batch_locations SET warehouse='raw_materials' WHERE material_batch_id=10`, schema))
	repo := postgresproduction.NewRepository(pool, schema)
	plan, err := repo.CreateProductionPlan(ctx, productionapp.CreateProductionPlanCommand{
		From: "2026-06-01", To: "2026-06-30", Selected: map[string]bool{"1-227": true},
		InputByKey: map[string]int64{"1-227": 600}, Operator: "计划员",
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	seedProductionPlanLifecycleOperationSplits(t, ctx, pool, schema, plan)
	_, err = repo.SubmitProductionPlan(ctx, productionapp.SubmitProductionPlanCommand{ID: plan.ID, Operator: "计划员"})
	if err == nil || !strings.Contains(err.Error(), "WIP") {
		t.Fatalf("expected WIP gate, got %v", err)
	}
	result, err := productionapp.NewService(repo).SubmitProductionPlans(ctx, productionapp.SubmitProductionPlansCommand{IDs: []int64{plan.ID}, Operator: "计划员"})
	if err != nil || len(result.Failed) != 1 || !strings.Contains(result.Failed[0].Error, "WIP") {
		t.Fatalf("bulk must also be blocked: %+v, %v", result, err)
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", "1=1", 0)
}
