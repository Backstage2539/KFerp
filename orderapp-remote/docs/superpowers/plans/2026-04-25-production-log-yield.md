# Production Log Yield Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add editable input weight on the production planning page, compute actual yield from completed output, and introduce a production log page under the production workflow menu.

**Architecture:** Keep the production workflow inside the unified Vue/Vite frontend. Persist planned input and BOM yield snapshot on `produce_running_items`, write one immutable `production_logs` record per completed running item, and expose that data through a JSON API consumed by `frontend-vue-shell`. Legacy page URLs may redirect or remain read-only compatibility surfaces, but user-facing production changes target Vue components.

**Tech Stack:** Go, Echo, PostgreSQL, JSON API, Vue 3 + Vite, existing requirement tables, `go test`, handler/API verification, frontend build verification.

---

## File Structure

**Create:**

- `docs/superpowers/plans/2026-04-25-production-log-yield.md`
- `orderapp-remote/production_logs_page.go`
- `orderapp-remote/production_logs_page_test.go`
- `orderapp-remote/dev_070_step1_test.go`
- `orderapp-remote/dev_071_step1_test.go`
- `orderapp-remote/dev_072_step1_test.go`
- `orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue`

**Modify:**

- `orderapp-remote/production_flow.go`
- `orderapp-remote/unprod_summary_page.go`
- `orderapp-remote/frontend-vue-shell/src/views/ProducePlanView.vue`
- `orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue` if the running page is touched by this workflow
- `orderapp-remote/materials.go`
- `orderapp-remote/schema_setup.go`
- `orderapp-remote/material_consumption.go`
- `orderapp-remote/production_flow_test.go`
- `orderapp-remote/frontend-vue-shell/src/App.vue`
- `orderapp-remote/req_store.go` or the current request-table helper used for new rows

**Verify / Possibly Touch:**

- `orderapp-remote/produce_materials.go`
- `orderapp-remote/produce_batch_api.go`
- `orderapp-remote/frontend-vue-shell/src/App.vue` shared navigation

---

### Task 1: Lock The Data Model And Calculation Helpers

**Files:**

- Modify: `orderapp-remote/production_flow.go`
- Modify: `orderapp-remote/materials.go`
- Modify: `orderapp-remote/schema_setup.go`
- Modify: `orderapp-remote/production_flow_test.go`
- Test: `orderapp-remote/dev_070_step1_test.go`

- [ ] **Step 1: Write the failing tests for the new production math**

Add focused tests before touching production code:

```go
func TestDefaultProductionInputGUsesBomYield(t *testing.T) {
	got := defaultProductionInputG(2270, 0.82)
	if got != 2769 {
		t.Fatalf("defaultProductionInputG() = %d, want 2769", got)
	}
}

func TestDefaultProductionInputGFallsBackToPointEight(t *testing.T) {
	got := defaultProductionInputG(2270, 0)
	if got != 2838 {
		t.Fatalf("defaultProductionInputG() = %d, want 2838", got)
	}
}

func TestActualYieldRateFromFinishedOutput(t *testing.T) {
	got, err := actualYieldRate(227, 8, 91, 2500)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.7628 {
		t.Fatalf("actualYieldRate() = %.4f, want 0.7628", got)
	}
}
```

- [ ] **Step 2: Run the focused test file and verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestDefaultProductionInputG|TestActualYieldRateFromFinishedOutput'
```

Expected:

- FAIL because helper functions do not exist yet.

- [ ] **Step 3: Add the minimal helper functions and schema additions**

Implement:

- `normalizeYieldRate(rate float64) float64`
- `defaultProductionInputG(needG int64, yieldRate float64) int64`
- `finishedTotalG(specG, units, looseG int64) int64`
- `actualYieldRate(specG, units, looseG, inputG int64) (float64, error)`

Extend `ensureProductionRunTable()`:

```go
ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS input_g BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000;
ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_units BIGINT NOT NULL DEFAULT 0;
ALTER TABLE %s.produce_running_items ADD COLUMN IF NOT EXISTS planned_loose_g BIGINT NOT NULL DEFAULT 0;
```

Add `ensureProductionLogTable()` in `materials.go` or a nearby schema file:

```go
CREATE TABLE IF NOT EXISTS %s.production_logs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL UNIQUE,
	batch_id TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	order_nos TEXT NOT NULL DEFAULT '',
	planned_need_g BIGINT NOT NULL DEFAULT 0,
	input_g BIGINT NOT NULL DEFAULT 0,
	bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000,
	finished_units BIGINT NOT NULL DEFAULT 0,
	finished_loose_g BIGINT NOT NULL DEFAULT 0,
	finished_total_g BIGINT NOT NULL DEFAULT 0,
	actual_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	started_by TEXT NOT NULL DEFAULT '',
	started_at TIMESTAMPTZ,
	finished_by TEXT NOT NULL DEFAULT '',
	finished_at TIMESTAMPTZ,
	inventory_units_before BIGINT NOT NULL DEFAULT 0,
	inventory_loose_g_before BIGINT NOT NULL DEFAULT 0,
	inventory_units_after BIGINT NOT NULL DEFAULT 0,
	inventory_loose_g_after BIGINT NOT NULL DEFAULT 0,
	material_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Wire `ensureProductionLogTable()` into `ensureAppSchema()`.

- [ ] **Step 4: Re-run the focused tests and verify GREEN**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestDefaultProductionInputG|TestActualYieldRateFromFinishedOutput'
```

Expected:

- PASS

- [ ] **Step 5: Commit the data-model/calculation slice**

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
git add production_flow.go materials.go schema_setup.go production_flow_test.go dev_070_step1_test.go
git commit -m "feat: add production input and yield helpers"
```

### Task 2: Capture Input Weight On The Production Planning Page

**Files:**

- Modify: `orderapp-remote/unprod_summary_page.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProducePlanView.vue`
- Modify: `orderapp-remote/production_flow.go`
- Test: `orderapp-remote/dev_071_step1_test.go`
- Test: `orderapp-remote/production_flow_test.go`

- [ ] **Step 1: Write the failing tests for planning-page input fields**

Add regression tests for:

- production plan rows expose a default `input_g`
- Vue start payload contains one `input_by_key` value per selected row
- Vue page contains the `投料数(g)` column and submits through the JSON API

Example Vue source check:

```go
func TestProducePlanVueContainsInputGFields(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/views/ProducePlanView.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"投料数(g)", "input_by_key", "startProduction"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("ProducePlanView.vue missing %q", needle)
		}
	}
}
```

- [ ] **Step 2: Run the focused planning tests and verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProducePlanVueContainsInputGFields'
```

Expected:

- FAIL because the new field is not present yet.

- [ ] **Step 3: Implement plan-row input defaults and form submission**

Extend the page model in `unprod_summary_page.go` so each plan row carries:

```go
type ProducePlanDisplayRow struct {
	UnprodNeedRow
	BomYieldRate float64
	InputG       int64
}
```

When `plan=1`:

- load the BOM yield map once
- derive `InputG = defaultProductionInputG(r.GapG, bomYield)`
- expose `BomYieldRate` and `InputG` to the template

Update `frontend-vue-shell/src/views/ProducePlanView.vue`:

- add a `投料数(g)` column to the plan table
- render editable number controls for selected plan rows
- make `startProduction()` submit `input_by_key` to `/api/produce/start`

Update `/produce/start` in `production_flow.go`:

- parse posted `input_g_*` values into a `map[string]int64`
- validate each selected row has `input_g > 0`
- pass that map into `saveRunningItems()`

Change `saveRunningItems()` signature so it persists:

- `input_g`
- `bom_yield_rate`
- `planned_units`
- `planned_loose_g`

- [ ] **Step 4: Re-run the planning tests and verify GREEN**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestUnproducedTemplateContainsInputGFields|TestDefaultProductionInputG'
```

Expected:

- PASS

- [ ] **Step 5: Commit the planning-page slice**

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
git add unprod_summary_page.go frontend-vue-shell/src/views/ProducePlanView.vue production_flow.go production_flow_test.go dev_071_step1_test.go
git commit -m "feat: add production input weight planning"
```

### Task 3: Finish Production With Actual Yield And Immutable Production Logs

**Files:**

- Modify: `orderapp-remote/production_flow.go`
- Modify: `orderapp-remote/material_consumption.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue` if this workflow migrates the running page in the same slice
- Modify: `orderapp-remote/production_flow_test.go`
- Test: `orderapp-remote/dev_072_step1_test.go`

- [ ] **Step 1: Write the failing tests for production completion logging**

Add tests that lock the new behavior:

- the Vue running page shows `计划投料数` and `BOM 出品率`
- a helper builds material-summary JSON from material consumption rows
- `finishRunningItem()` rejects zero `input_g`
- `finishRunningItem()` creates one `production_logs` row with expected totals

The database test can be integration-style if the repo already supports it; otherwise create a focused repository/helper test around SQL generation and calculation helpers.

- [ ] **Step 2: Run the focused completion tests and verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProduceRunningTemplateContains|TestActualYieldRate'
```

Expected:

- FAIL because the page and log write path are still old.

- [ ] **Step 3: Implement the completion transaction changes**

In `listRunningItems()` load:

- `input_g`
- `bom_yield_rate`
- `planned_units`
- `planned_loose_g`

Add these fields to `ProduceRunRow`.

Update the Vue running page to render:

- `计划投料数(g)`
- `BOM 出品率`
- `预计成品`

Inside `finishRunningItem()`:

1. lock the running item
2. read current finished inventory before values
3. compute:
   - `finishedTotalG`
   - `actualYieldRate`
4. normalize and upsert finished inventory
5. deduct materials
6. fetch the just-written material-consumption rows for this `running_item_id`
7. build a JSON summary like:

```json
[
  {"material_id":1,"material_name":"卡蒂姆水洗","unit":"g","deduct_g":1200,"deduct_units":0},
  {"material_id":9,"material_name":"豆袋","unit":"个","deduct_g":0,"deduct_units":8}
]
```

8. insert one `production_logs` row
9. mark the running item done

Keep the entire sequence inside one transaction.

- [ ] **Step 4: Re-run the completion tests and the broader Go suite**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProduceRunningTemplateContains|TestDefaultProductionInputG|TestActualYieldRate'
go test ./...
```

Expected:

- all targeted tests PASS
- full suite PASS

- [ ] **Step 5: Commit the completion/logging slice**

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
git add production_flow.go material_consumption.go frontend-vue-shell/src/views/ProduceRunningView.vue production_flow_test.go dev_072_step1_test.go
git commit -m "feat: log completed production yield"
```

### Task 4: Add The Production Log Page And Menu Entry

**Files:**

- Create: `orderapp-remote/production_logs_page.go`
- Create: `orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue`
- Create: `orderapp-remote/production_logs_page_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: shared Vue shell navigation that lists production-flow links

- [ ] **Step 1: Write the failing tests for the new page and menu**

Add tests for:

- Vue page contains `生产日志`
- JSON API registration responds with production log data
- Vue shell menu contains `produceLogs` as an internal view

Example source assertions:

```go
func TestProductionLogsVueContainsKeyColumns(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/views/ProductionLogsView.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"生产日志", "真实出品率", "投料数(g)", "完成时间"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("ProductionLogsView.vue missing %q", needle)
		}
	}
}
```

- [ ] **Step 2: Run the focused page/menu tests and verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionLogsVueContainsKeyColumns'
```

Expected:

- FAIL because the Vue page/API does not exist yet.

- [ ] **Step 3: Implement the page, query, and navigation**

Create `production_logs_page.go` with:

- `registerProductionLogPages(e, pool, schema)`
- `GET /produce/logs`
- `GET /api/produce/logs`
- optional filters: `from`, `to`, `product_id`, `batch_id`, `operator`

Query shape:

```sql
SELECT id, batch_id, product_id, product_name, spec_g, order_nos,
       planned_need_g, input_g, bom_yield_rate,
       finished_units, finished_loose_g, finished_total_g, actual_yield_rate,
       started_by, to_char(started_at,'YYYY-MM-DD HH24:MI'),
       finished_by, to_char(finished_at,'YYYY-MM-DD HH24:MI'),
       inventory_units_before, inventory_loose_g_before,
       inventory_units_after, inventory_loose_g_after,
       material_summary
FROM %s.production_logs
...
ORDER BY finished_at DESC, id DESC
LIMIT 200
```

Add page registration from the same bootstrap path that registers other production pages.

Update Vue frontend:

- `frontend-vue-shell/src/App.vue`: add `produceLogs` as an internal Vue view
- `frontend-vue-shell/src/views/ProductionLogsView.vue`: fetch `/api/produce/logs`

- [ ] **Step 4: Run the page tests and frontend build verification**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionLogsVueContainsKeyColumns|TestProduceRunningTemplateContainsFinishedInventoryFields'
cd frontend-vue-shell && npm run build
```

Expected:

- tests PASS
- Vue frontend build PASS

- [ ] **Step 5: Commit the page/menu slice**

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
git add production_logs_page.go production_logs_page_test.go frontend-vue-shell/src/views/ProductionLogsView.vue frontend-vue-shell/src/App.vue
git commit -m "feat: add production log page"
```

### Task 5: Update Requirement Tables And Finish Verification

**Files:**

- Modify: `orderapp-remote/req_store.go` or the live request-table insertion path used in this codebase
- Verify: production routes and database state

- [ ] **Step 1: Add requirement-table rows for this feature**

Insert or upsert:

- `PR-PROD-001` — 生产计划投料、真实出品率、生产日志
- `DEV-PROD-001` — 生产计划页投料录入
- `DEV-PROD-002` — 生产中真实出品率计算与日志落库
- `DEV-PROD-003` — 生产日志页面与菜单
- `UT-PROD-001` — 投料与出品率计算测试
- `UT-PROD-002` — 生产日志落库测试
- `API-PROD-001` — 开始生产保存投料数
- `API-PROD-002` — 完成生产写生产日志
- `API-PROD-003` — 生产日志页面返回 200
- `REV-PROD-001` — Van 验收生产日志和真实出品率

- [ ] **Step 2: Run API-level verification locally or against a controlled environment**

Run commands shaped like:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./...
curl -k -u "order:${ORDERAPP_PASS}" -s "https://erp.qacoohee.com/app/produce/logs" | grep -q "生产日志"
curl -k -u "order:${ORDERAPP_PASS}" -s "https://erp.qacoohee.com/app/produce/running" | grep -q "计划投料数"
```

And verify one completed-production row has:

- non-zero `input_g`
- non-zero `finished_total_g`
- expected `actual_yield_rate`

- [ ] **Step 3: Re-read the spec and confirm coverage**

Checklist:

- input weight editable on planning page
- actual yield derived from output only
- production log under production workflow
- log contains batch/product/order/input/output/yield/operator/inventory/material summary
- 5 requirement tables updated

- [ ] **Step 4: Push branch and merge only after clean verification**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
git push -u origin <feature-branch>
git fetch origin develop
git switch develop
git merge --ff-only <feature-branch>
git push origin develop
```

- [ ] **Step 5: Deploy and run production smoke checks**

Use the existing deployment workflow:

```bash
ssh root@1.12.242.58 "cd /opt/stacks/erp && docker compose ps"
# upload tested source tree
ssh root@1.12.242.58 "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d orderapp"
ssh root@1.12.242.58 'set -a; . /opt/stacks/erp/.env; curl -k -u "order:${ORDERAPP_PASS}" -s https://erp.qacoohee.com/app/produce/logs | grep -q "生产日志"'
ssh root@1.12.242.58 'set -a; . /opt/stacks/erp/.env; curl -k -u "order:${ORDERAPP_PASS}" -s https://erp.qacoohee.com/app/produce/running | grep -q "计划投料数"'
```

Expected:

- container healthy
- both pages return 200 with expected text
- completed-production data writes exactly one log row

---

## Self-Review

**Spec coverage:** All requirements from the approved spec map to one of the five tasks above: data model, planning input, running completion and real yield, log page/menu, requirement tables plus verification.

**Placeholder scan:** No `TODO`/`TBD` placeholders remain. Commands, files, and target behaviors are explicit.

**Type consistency:** The plan consistently uses `input_g`, `bom_yield_rate`, `planned_units`, `planned_loose_g`, `finished_total_g`, and `actual_yield_rate` across schema, code, templates, and logs.
