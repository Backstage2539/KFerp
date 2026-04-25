# Roast Plan Material Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate `生产计划/开始生产` to the Vue/Vite frontend and make roast batch edits drive material summary totals and submitted production input.

**Architecture:** Add JSON endpoints for the production-plan page, render the page inside `frontend-vue-shell` instead of the legacy HTML template, and keep `POST /produce/start` behavior mirrored by a new JSON start endpoint. The Vue page owns roast-batch edit state and recomputes final input grams and material summaries from backend-provided BOM ratios.

**Tech Stack:** Go + Echo + pgx, Vue 3 + Vite, existing production-flow/domain helpers, Go tests + frontend build verification.

---

## File Structure

- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/unprod_summary_page.go`
  - Split data loading from HTML rendering into reusable functions.
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/unprod_summary_api.go`
  - JSON contract for loading the production-plan page and starting production from Vue.
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/production_flow.go`
  - Reuse start-production parsing/validation for form and JSON callers.
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_materials_bom.go`
  - Add material-summary calculation based on final roast input grams.
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/roast_split.go`
  - Expose structured roast-plan rows with per-batch grams and batch counts.
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/static_frontend_routes.go`
  - Route legacy `/produce/unproduced` entry to the Vue shell view.
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/App.vue`
  - Support an internal Vue page for `producePlan` instead of iframe.
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/views/ProducePlanView.vue`
  - Full page UI for filters, plan rows, material summary, roast suggestions, and start action.
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/lib/produce-plan.js`
  - Frontend pure helpers for recomputing roast/material rows.
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_plan_api_test.go`
  - API contract coverage.
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_materials_test.go`
  - Final-input-driven BOM material summary tests.
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/production_flow_api_test.go`
  - JSON start-production path.

### Task 1: Back-End Data Contract

**Files:**
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/unprod_summary_page.go`
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/roast_split.go`
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_materials_bom.go`
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_materials_test.go`

- [ ] **Step 1: Write the failing material-summary tests**

```go
func TestCalcProducePlanMaterialsFromFinalInputsUsesRoastInputForBomBeans(t *testing.T) {
	rows := []UnprodNeedRow{{ProductID: 1, Product: "曲奇拼配", SpecG: 1000, GapG: 1000}}
	finalInputs := map[int64]int64{1: 2000}
	bomMap := map[int64][]bomNeedItem{
		1: {
			{ProductID: 1, MaterialName: "豆子A", MaterialUnit: "g", RatioPct: 70},
			{ProductID: 1, MaterialName: "豆子B", MaterialUnit: "g", RatioPct: 30},
		},
	}

	got := calcProducePlanMaterialsFromFinalInputs(rows, finalInputs, bomMap, defaultProducePlanParams())

	assertMaterialQty(t, got, "豆子A", "g", 1400)
	assertMaterialQty(t, got, "豆子B", "g", 600)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestCalcProducePlanMaterialsFromFinalInputsUsesRoastInputForBomBeans'`
Expected: FAIL with `undefined: calcProducePlanMaterialsFromFinalInputs`

- [ ] **Step 3: Implement minimal structured roast-plan/material helpers**

```go
type RoastPlanRow struct {
	ProductID     int64   `json:"product_id"`
	ProductName   string  `json:"product_name"`
	Machine       string  `json:"machine"`
	BatchCount    int64   `json:"batch_count"`
	DefaultBatchG int64   `json:"default_batch_g"`
	FinalInputG   int64   `json:"final_input_g"`
	YieldRate     float64 `json:"yield_rate"`
}

func calcProducePlanMaterialsFromFinalInputs(rows []UnprodNeedRow, finalInputByProductID map[int64]int64, bomMap map[int64][]bomNeedItem, p ProducePlanParams) []MaterialNeed {
	// bean BOM items use final_input_g; bag/unit materials still use gap/spec piece logic
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -run 'TestCalcProducePlanMaterialsFromFinalInputsUsesRoastInputForBomBeans|TestCalcProducePlanMaterialsWithBOMUsesConfiguredBeanNames'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add produce_materials_bom.go roast_split.go produce_materials_test.go unprod_summary_page.go
git commit -m "feat: add roast-plan material calculation helpers"
```

### Task 2: JSON API For Vue Production Plan

**Files:**
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/unprod_summary_api.go`
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/unprod_summary_page.go`
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/production_flow.go`
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/produce_plan_api_test.go`
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/production_flow_api_test.go`

- [ ] **Step 1: Write the failing API tests**

```go
func TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/produce/unproduced?selected=1-1000&plan=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"roast_plans"`) {
		t.Fatalf("body missing roast_plans: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials|TestProduceStartAPIUsesSubmittedInputG'`
Expected: FAIL with `404` or missing route/body assertions

- [ ] **Step 3: Implement the JSON routes and shared start logic**

```go
e.GET("/api/produce/unproduced", func(c echo.Context) error {
	data, err := loadUnprodSummaryData(c.Request().Context(), pool, schema, parseSummaryQuery(c))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, data)
})

e.POST("/api/produce/start", func(c echo.Context) error {
	var req produceStartRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request"})
	}
	res, err := startProduction(ctx, pool, schema, req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
	return c.JSON(http.StatusOK, res)
})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -run 'TestProducePlanSummaryAPIIncludesRoastRowsAndMaterials|TestProduceStartAPIUsesSubmittedInputG|TestProduceStartHandlerPersistsInputG'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add unprod_summary_api.go unprod_summary_page.go production_flow.go produce_plan_api_test.go production_flow_api_test.go
git commit -m "feat: add production plan json api"
```

### Task 3: Vue/Vite Production Plan Page

**Files:**
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/App.vue`
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/views/ProducePlanView.vue`
- Create: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell/src/lib/produce-plan.js`
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/static_frontend_routes.go`
- Test: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/production_flow_test.go`

- [ ] **Step 1: Write the failing page/route tests**

```go
func TestVueShellProducePlanIsNoLongerTemplateDriven(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"ProducePlanView", "view=producePlan"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("App.vue missing %q", needle)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestVueShellProducePlanIsNoLongerTemplateDriven'`
Expected: FAIL because `ProducePlanView` is missing

- [ ] **Step 3: Implement the Vue page and route wiring**

```vue
<!-- ProducePlanView.vue -->
<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { buildMaterialSummary, buildStartPayload } from '../lib/produce-plan'
</script>
```

```js
// App.vue
import ProducePlanView from './views/ProducePlanView.vue'
const internalViews = { producePlan: ProducePlanView }
```

```go
// static_frontend_routes.go
e.GET("/produce/unproduced", func(c echo.Context) error {
	return c.Redirect(http.StatusFound, "/vue-shell?view=producePlan")
})
```

- [ ] **Step 4: Run tests and frontend build**

Run:
```bash
go test ./... -run 'TestVueShellProducePlanIsNoLongerTemplateDriven|TestProductionPlanTemplateContainsInputG'
cd frontend-vue-shell && npm run build
```

Expected:
- Go tests PASS
- Vite build succeeds without type/runtime import errors

- [ ] **Step 5: Commit**

```bash
git add frontend-vue-shell/src/App.vue frontend-vue-shell/src/views/ProducePlanView.vue frontend-vue-shell/src/lib/produce-plan.js static_frontend_routes.go production_flow_test.go
git commit -m "feat: migrate produce plan page to vue shell"
```

### Task 4: Full Verification, Requirement Tables, Deploy

**Files:**
- Modify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/REQUIREMENTS.md` (only if needed by current workflow)
- Modify: requirement tables through existing app/db workflow
- Verify: `/Users/yiiiple-work/Documents/KFerp/orderapp-remote/docs/superpowers/specs/2026-04-25-roast-plan-material-sync-design.md`

- [ ] **Step 1: Run full backend tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 2: Run frontend builds**

Run:
```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend && npm run build
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote/frontend-vue-shell && npm run build
```

Expected: both builds PASS

- [ ] **Step 3: Update 5 tables for this requirement**

Codes to add/update:
- `PR-PROD-002`
- `DEV-PROD-004`
- `UT-PROD-003`
- `API-PROD-004`
- `REV-PROD-002`

- [ ] **Step 4: Deploy and smoke test**

Run:
```bash
ssh root@1.12.242.58 "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d orderapp"
ssh root@1.12.242.58 'set -a; . /opt/stacks/erp/.env; curl -ks -u "order:${ORDERAPP_PASS}" "https://erp.qacoohee.com/vue-shell?view=producePlan" | sed -n "1,120p"'
```

Expected:
- build succeeds
- app container healthy
- Vue shell page loads

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "feat: sync roast plan edits with material summary"
```

## Self-Review

- Spec coverage:
  - Vue/Vite migration: Task 3
  - roast batch editable quantity: Task 3
  - material summary driven by final input grams: Task 1 + Task 3
  - start production uses edited final inputs: Task 2 + Task 3
  - deploy and evidence: Task 4
- Placeholder scan: no `TBD` / `TODO` placeholders remain in task steps.
- Type consistency:
  - `RoastPlanRow`, `calcProducePlanMaterialsFromFinalInputs`, JSON routes, and `ProducePlanView` names are used consistently across tasks.
