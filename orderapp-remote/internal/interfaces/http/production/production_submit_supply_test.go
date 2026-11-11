package production

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestSubmitSupplyConflictReturns409AndPassesDraftToken(t *testing.T) {
	repo := &workOrderAPIRepo{submitPlanErrByID: map[int64]error{41: fmt.Errorf("供应已变化，请重新核对生产安排")}}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/production-plans/41/submit", strings.NewReader(`{"draft_token":"token-41"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(fmt.Sprintf("%+v", repo.submitPlan), "token-41") {
		t.Fatalf("missing draft token: %+v", repo.submitPlan)
	}
}

func TestBatchSubmitPassesEachDraftToken(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/production-plans/submit", strings.NewReader(`{"ids":[41,42],"draft_tokens":{"41":"token-41","42":"token-42"}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.submitPlans) != 2 || !strings.Contains(fmt.Sprintf("%+v", repo.submitPlans[0]), "token-41") || !strings.Contains(fmt.Sprintf("%+v", repo.submitPlans[1]), "token-42") {
		t.Fatalf("missing batch tokens: %+v", repo.submitPlans)
	}
}

func TestSubmitRejectsStaleDraftTokenWithoutWorkOrders(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	app := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22700},
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	result := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), map[string]any{"draft_token": "stale"})
	if result.Code != http.StatusConflict || !strings.Contains(result.Body.String(), "草稿版本已变化") {
		t.Fatalf("stale submit=%d %s", result.Code, result.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 0)
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='draft'", planID), 1)
}

func TestSubmitRejectsChangedSupplyBeforeCreatingWorkOrders(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1010, 10, "SUBMIT-ROAST-BEFORE", "在制熟豆", 10000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1030, 30, "SUBMIT-GREEN", "生产生豆", 20000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 1020, 20, "SUBMIT-BAG", "227g包装袋", 100)
	app := newProductionFlowTestEcho(pool, schema)
	created := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31", "selected": []string{"1-227"}, "input_by_key": map[string]int64{"1-227": 22700},
	})
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 1011, 10, "SUBMIT-ROAST-NEW", "在制熟豆", 5000)
	result := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if result.Code != http.StatusConflict || !strings.Contains(result.Body.String(), "供应已变化") {
		t.Fatalf("supply changed submit=%d %s", result.Code, result.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 0)
	assertProductionFlowCount(t, pool, schema, "production_plans", fmt.Sprintf("id=%d AND status='draft'", planID), 1)
	batch := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans/submit", map[string]any{"ids": []int64{planID}})
	if batch.Code != http.StatusOK || !strings.Contains(batch.Body.String(), "供应已变化") || !strings.Contains(batch.Body.String(), `"work_order_count":0`) {
		t.Fatalf("batch submit must report the changed supply without issuing work orders: %d %s", batch.Code, batch.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("production_plan_id=%d", planID), 0)
}
