# Architecture Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move KFerp from a mixed legacy template/main-package architecture to a Vue/Vite + JSON API + application/domain/repository layered architecture before feature growth makes the migration expensive.

**Architecture:** Upgrade in small vertical slices. Each slice keeps the app deployable, adds guardrail tests, migrates one user-facing surface to Vue/Vite, moves business orchestration into application services, and leaves SQL behind repository interfaces. Legacy routes remain only as redirects or compatibility read-only surfaces.

**Tech Stack:** Go, Echo, pgx/PostgreSQL, Vue 3 + Vite, existing requirement tables, `go test`, handler/API tests, frontend build verification.

---

## File Structure

**Create / Expand:**

- `orderapp-remote/review_architecture_guard_test.go`
  - Static guardrails for workflow, Vue/Vite migration, and route/schema separation.
- `orderapp-remote/internal/ports/unit_of_work.go`
  - Transaction boundary contract used by application services.
- `orderapp-remote/internal/application/production/service.go`
  - Production use cases: plan, start, finish, cancel, logs.
- `orderapp-remote/internal/domain/production/*.go`
  - Pure production math and validation.
- `orderapp-remote/internal/infrastructure/postgres/production_repository.go`
  - SQL implementation for production use cases.
- `orderapp-remote/internal/interfaces/http/production_handlers.go`
  - Echo handlers that expose JSON API and call application services.
- `orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue`
  - Vue replacement for the production running page.
- `orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue`
  - Vue replacement for the production log page.
- `orderapp-remote/frontend-vue-shell/src/api/production.js`
  - Frontend API client for production views.

**Modify:**

- `HOW_TO_WORKFLOW.md`
  - Align canonical workflow with test-first + Vue/Vite rules.
- `orderapp-remote/docs/superpowers/plans/2026-04-25-production-log-yield.md`
  - Mark as corrected legacy plan or rewrite to Vue/API direction.
- `orderapp-remote/frontend-vue-shell/src/App.vue`
  - Use internal Vue views for migrated pages.
- `orderapp-remote/static_frontend_routes.go`
  - Redirect legacy page URLs to Vue shell views.
- `orderapp-remote/app_routes.go`
  - Register migrated JSON API handlers from a smaller composition root.
- `orderapp-remote/production_flow.go`
  - Shrink to domain-compatible logic or remove after migration.
- `orderapp-remote/production_flow_routes.go`
  - Temporary compatibility routes only; no new business logic.
- `orderapp-remote/production_flow_schema.go`
  - Temporary schema bootstrap until moved to infra migrations.
- `orderapp-remote/audit_unified.go`
  - Central `AuditService`.
- `orderapp-remote/operation_log.go`
  - Request-only `OperationLogger`.
- `orderapp-remote/internal/application/sales/service.go`
  - Later phase: convert from repository wrapper into real order use cases.
- `orderapp-remote/order_routes.go`
- `orderapp-remote/sales_order_repository.go`
  - Later phase: migrate order workflows.
- `orderapp-remote/materials.go`
- `orderapp-remote/bom_api.go`
  - Later phase: migrate materials/BOM workflows.

---

### Task 1: Baseline Branch And Guardrails

**Files:**

- Create/Modify: `orderapp-remote/review_architecture_guard_test.go`
- Modify: `HOW_TO_WORKFLOW.md`
- Modify: `orderapp-remote/docs/superpowers/plans/2026-04-25-production-log-yield.md`

- [ ] **Step 1: Start from latest integration branch**

```bash
cd /Users/yiiiple-work/Documents/KFerp
git fetch origin
git switch -c codex/architecture-upgrade-20260426 origin/develop
```

Expected:

- branch is not `develop`
- branch starts from current `origin/develop`

- [ ] **Step 2: Write failing guardrail tests**

Add tests to `orderapp-remote/review_architecture_guard_test.go`:

```go
func TestWorkflowRequiresTestsBeforeImplementation(t *testing.T) {
	body, err := os.ReadFile("../HOW_TO_WORKFLOW.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	testFirst := strings.Index(content, "先编写单元测试代码")
	implementation := strings.Index(content, "实现代码")
	if testFirst < 0 || implementation < 0 || testFirst > implementation {
		t.Fatal("workflow must require unit/API tests before implementation")
	}
}

func TestProductionLogPlanDoesNotExtendLegacyTemplates(t *testing.T) {
	body, err := os.ReadFile("docs/superpowers/plans/2026-04-25-production-log-yield.md")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"Extend the existing server-rendered production workflow",
		"server-rendered HTML templates",
		"templates/production_logs.html",
		"templates/unprod_summary.html",
		"templates/produce_running.html",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("plan still directs legacy template work: %q", forbidden)
		}
	}
}
```

- [ ] **Step 3: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestWorkflowRequiresTestsBeforeImplementation|TestProductionLogPlanDoesNotExtendLegacyTemplates'
```

Expected:

- FAIL before the workflow and plan are corrected.

- [ ] **Step 4: Correct workflow and plan**

Update:

- `HOW_TO_WORKFLOW.md`: PR -> DEV -> UT/API test design -> write failing tests -> implementation -> UT evidence -> API evidence -> REV -> deploy.
- `2026-04-25-production-log-yield.md`: Vue/Vite + JSON API only; legacy URLs may redirect; no new template work.

- [ ] **Step 5: Verify GREEN**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestWorkflowRequiresTestsBeforeImplementation|TestProductionLogPlanDoesNotExtendLegacyTemplates'
```

Expected:

- PASS.

- [ ] **Step 6: Commit**

```bash
git add HOW_TO_WORKFLOW.md orderapp-remote/docs/superpowers/plans/2026-04-25-production-log-yield.md orderapp-remote/review_architecture_guard_test.go
git commit -m "chore: add architecture workflow guardrails"
```

---

### Task 2: Logging And Audit Service Separation

**Files:**

- Create: `orderapp-remote/audit_schema.go`
- Create: `orderapp-remote/audit_service_test.go`
- Modify: `orderapp-remote/audit_unified.go`
- Modify: `orderapp-remote/operation_log.go`
- Modify: `orderapp-remote/operation_log_test.go`
- Modify: `orderapp-remote/audit.go`

- [ ] **Step 1: Write failing logger separation tests**

Add to `operation_log_test.go`:

```go
func TestOperationLogDoesNotMirrorRequestsIntoAuditLogs(t *testing.T) {
	body, err := os.ReadFile("operation_log.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if strings.Contains(content, "auditInsert(") {
		t.Fatal("operation log should not mirror request rows into audit_logs")
	}
	if strings.Contains(content, "entity_type") {
		t.Fatal("operation_log.go should not know audit_logs schema details")
	}
}
```

Add to `audit_service_test.go`:

```go
func TestAuditUnifiedOwnsAuditServiceAndTxHelper(t *testing.T) {
	body, err := os.ReadFile("audit_unified.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{"type AuditService struct", "type AuditEntry struct", "func auditInsertTx"} {
		if !strings.Contains(content, want) {
			t.Fatalf("audit_unified.go missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestOperationLogDoesNotMirrorRequestsIntoAuditLogs|TestAuditUnifiedOwnsAuditServiceAndTxHelper'
```

Expected:

- FAIL until request logging and audit service are separated.

- [ ] **Step 3: Implement logging split**

Implement:

- `AuditService` in `audit_unified.go`
- `auditInsertTx` for transaction-bound audit rows
- `OperationLogger` in `operation_log.go`
- `audit_schema.go` for all audit/log DDL
- Remove request mirroring from `operation_log.go`
- Change inline order audit to use `auditInsertTx(ctx, tx, ...)`

- [ ] **Step 4: Verify GREEN**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestOperationLogDoesNotMirrorRequestsIntoAuditLogs|TestAuditUnifiedOwnsAuditServiceAndTxHelper|TestInlineOrderAuditUsesTransactionHelper'
go test ./...
```

Expected:

- focused tests PASS
- full Go suite PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/audit_schema.go orderapp-remote/audit_service_test.go orderapp-remote/audit_unified.go orderapp-remote/operation_log.go orderapp-remote/operation_log_test.go orderapp-remote/audit.go
git commit -m "refactor: separate operation logging and audit service"
```

---

### Task 3: Production Logs To Vue + JSON API

**Files:**

- Create: `orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/production_logs_page.go`
- Modify: `orderapp-remote/production_logs_page_test.go`

- [ ] **Step 1: Write failing Vue/API tests**

Update `production_logs_page_test.go`:

```go
func TestProductionLogsVueContainsKeyColumns(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/views/ProductionLogsView.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, needle := range []string{"生产日志", "真实出品率", "投料数(g)", "完成时间", "/api/produce/logs"} {
		if !strings.Contains(content, needle) {
			t.Fatalf("ProductionLogsView.vue missing %q", needle)
		}
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionLogsVueContainsKeyColumns'
```

Expected:

- FAIL before `ProductionLogsView.vue` exists.

- [ ] **Step 3: Add JSON API and Vue page**

Implement:

- `GET /api/produce/logs`
- `GET /produce/logs` redirects to `/vue-shell?view=produceLogs`
- `ProductionLogsView.vue` fetches `/api/produce/logs`
- `App.vue` wires `produceLogs` as internal view

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionLogsVueContainsKeyColumns|TestProductionLogPagesExposeJSONAPIAndVueRedirect|TestProductionMenusContainLogsEntry'
cd frontend-vue-shell && npm run build
```

Expected:

- focused tests PASS
- Vue shell build PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/production_logs_page.go orderapp-remote/production_logs_page_test.go orderapp-remote/frontend-vue-shell/src/App.vue orderapp-remote/frontend-vue-shell/src/views/ProductionLogsView.vue
git commit -m "refactor: move production logs to vue api view"
```

---

### Task 4: Production Flow File Split

**Files:**

- Create: `orderapp-remote/production_flow_routes.go`
- Create: `orderapp-remote/production_flow_schema.go`
- Modify: `orderapp-remote/production_flow.go`
- Modify: `orderapp-remote/review_architecture_guard_test.go`

- [ ] **Step 1: Write failing split test**

Add:

```go
func TestProductionFlowRoutesAndSchemaAreSplitOut(t *testing.T) {
	body, err := os.ReadFile("production_flow.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, forbidden := range []string{
		"func registerProductionFlowPages",
		"func ensureProductionRunTable",
		"e.POST(\"/produce/start\"",
	} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("production_flow.go still owns split-out concern %q", forbidden)
		}
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionFlowRoutesAndSchemaAreSplitOut'
```

Expected:

- FAIL while `production_flow.go` still owns routes/schema.

- [ ] **Step 3: Move code without behavior change**

Move:

- `registerProductionFlowPages`, request structs, request parsing helpers -> `production_flow_routes.go`
- `ensureProductionRunTable` -> `production_flow_schema.go`

Keep existing function names so callers do not change.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProductionFlowRoutesAndSchemaAreSplitOut|TestProduceStartAPIUsesSubmittedInputG|TestProduceFinishHandlerWritesProductionLog'
go test ./...
```

Expected:

- focused tests PASS
- full Go suite PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/production_flow.go orderapp-remote/production_flow_routes.go orderapp-remote/production_flow_schema.go orderapp-remote/review_architecture_guard_test.go
git commit -m "refactor: split production flow routes and schema"
```

---

### Task 5: Production Application Service Extraction

**Files:**

- Create/Modify: `orderapp-remote/internal/application/production/service.go`
- Create: `orderapp-remote/internal/application/production/service_flow_test.go`
- Create: `orderapp-remote/internal/domain/production/yield.go`
- Create: `orderapp-remote/internal/domain/production/yield_test.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/production_repository.go`
- Modify: `orderapp-remote/production_flow.go`
- Modify: `orderapp-remote/production_flow_routes.go`

- [ ] **Step 1: Move pure production math into domain with tests**

Create `internal/domain/production/yield_test.go`:

```go
func TestDefaultInputGramsUsesYieldRate(t *testing.T) {
	got := DefaultInputGrams(800, 0.8)
	if got != 1000 {
		t.Fatalf("DefaultInputGrams() = %d, want 1000", got)
	}
}

func TestActualYieldRateRoundsToFourDecimals(t *testing.T) {
	got, err := ActualYieldRate(227, 3, 19, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0.7000 {
		t.Fatalf("ActualYieldRate() = %.4f, want 0.7000", got)
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./internal/domain/production
```

Expected:

- FAIL before package/functions exist.

- [ ] **Step 3: Implement domain production math**

Create `internal/domain/production/yield.go` with:

```go
package production

import (
	"fmt"
	"math"
)

func NormalizeYieldRate(rate float64) float64 {
	if rate <= 0 || rate > 1 {
		return 0.8
	}
	return rate
}

func DefaultInputGrams(needG int64, yieldRate float64) int64 {
	if needG <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(needG) / NormalizeYieldRate(yieldRate)))
}

func FinishedTotalGrams(specG, units, looseG int64) int64 {
	if specG <= 0 || units < 0 || looseG < 0 {
		return 0
	}
	return units*specG + looseG
}

func ActualYieldRate(specG, units, looseG, inputG int64) (float64, error) {
	if inputG <= 0 {
		return 0, fmt.Errorf("input_g must be greater than 0")
	}
	total := FinishedTotalGrams(specG, units, looseG)
	rate := float64(total) / float64(inputG)
	return math.Round(rate*10000) / 10000, nil
}
```

- [ ] **Step 4: Extract production use case interfaces**

In `internal/application/production/service.go`, introduce:

```go
type RunningItem struct {
	ID           int64
	BatchID      string
	ProductID    int64
	ProductName  string
	SpecG        int64
	NeedG        int64
	InputG       int64
	BomYieldRate float64
	OrderNos     string
}

type Repository interface {
	ListRunning(ctx context.Context) ([]RunningItem, error)
	Start(ctx context.Context, cmd StartCommand) (string, error)
	Finish(ctx context.Context, cmd FinishCommand) error
	Cancel(ctx context.Context, cmd CancelCommand) error
}
```

- [ ] **Step 5: Route handlers call application service**

`production_flow_routes.go` should parse HTTP and call service methods. It should not contain SQL or production math.

- [ ] **Step 6: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./internal/domain/production ./internal/application/production
go test ./...
```

Expected:

- domain/application tests PASS
- full Go suite PASS

- [ ] **Step 7: Commit**

```bash
git add orderapp-remote/internal/domain/production orderapp-remote/internal/application/production orderapp-remote/internal/infrastructure/postgres/production_repository.go orderapp-remote/production_flow.go orderapp-remote/production_flow_routes.go
git commit -m "refactor: extract production application service"
```

---

### Task 6: Migrate Produce Running To Vue

**Files:**

- Create: `orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue`
- Create/Modify: `orderapp-remote/frontend-vue-shell/src/api/production.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/production_flow_routes.go`
- Test: `orderapp-remote/production_flow_api_test.go`

- [ ] **Step 1: Add API contract tests**

Add handler tests for:

- `GET /api/produce/running`
- `POST /api/produce/running/finish`
- `POST /api/produce/running/cancel`

Expected contract:

```json
{
  "rows": [
    {
      "id": 1,
      "batch_id": "PB-1",
      "product_name": "测试产品",
      "input_g": 1000,
      "bom_yield_rate": 0.8
    }
  ]
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProduceRunningAPI'
```

Expected:

- FAIL until JSON endpoints exist.

- [ ] **Step 3: Implement JSON endpoints and Vue page**

Implement:

- `GET /api/produce/running`
- `POST /api/produce/running/finish`
- `POST /api/produce/running/cancel`
- `ProduceRunningView.vue`
- `/produce/running` redirects to `/vue-shell?view=produceRunning`

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestProduceRunningAPI|TestProduceFinishHandlerWritesProductionLog'
cd frontend-vue-shell && npm run build
```

Expected:

- focused tests PASS
- Vue shell build PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/production_flow_routes.go orderapp-remote/production_flow_api_test.go orderapp-remote/frontend-vue-shell/src/App.vue orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue orderapp-remote/frontend-vue-shell/src/api/production.js
git commit -m "refactor: migrate running production page to vue"
```

---

### Task 7: Sales Service Upgrade

**Files:**

- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/application/sales/service_test.go`
- Create: `orderapp-remote/internal/domain/sales/order.go`
- Create: `orderapp-remote/internal/domain/sales/order_test.go`
- Modify: `orderapp-remote/order_routes.go`
- Modify: `orderapp-remote/sales_order_repository.go`

- [ ] **Step 1: Define typed application commands**

Replace string-heavy command fields gradually:

```go
type SaveOrderCommand struct {
	Actor          string
	EditID         int64
	OrderDate      time.Time
	CustomerID     int64
	SourceID       int64
	OrderTypeID    int64
	PayStatusID    int64
	ShipStatusID   int64
	ShippingAmount float64
	DiscountAmount float64
	RoundToInt      bool
	Items           []OrderItemCommand
}

type OrderItemCommand struct {
	ProductID   *int64
	TierID      *int64
	ManualPrice *float64
	Name        string
	Units       int64
	SpecG       int64
}
```

- [ ] **Step 2: Move form parsing to HTTP layer**

`order_routes.go` parses strings into typed command values. `sales_order_repository.go` no longer parses HTTP form arrays.

- [ ] **Step 3: Move pricing decisions to domain/application**

Domain/application decides:

- retail vs wholesale
- exact retail spec price vs fallback
- tier match
- manual price override
- round-to-int

Repository only loads pricing data and persists order rows.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./internal/domain/sales ./internal/application/sales
go test ./... -run 'TestSaveOrder|TestRetail|TestOrder'
go test ./...
```

Expected:

- sales domain/application tests PASS
- order API/regression tests PASS
- full Go suite PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/internal/domain/sales orderapp-remote/internal/application/sales orderapp-remote/order_routes.go orderapp-remote/sales_order_repository.go
git commit -m "refactor: make sales service own order use cases"
```

---

### Task 8: Materials And BOM Migration

**Files:**

- Create: `orderapp-remote/internal/application/materials/service.go`
- Create: `orderapp-remote/internal/domain/materials/*.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/materials_repository.go`
- Create: `orderapp-remote/frontend-vue-shell/src/views/MaterialsView.vue`
- Modify: `orderapp-remote/materials.go`
- Modify: `orderapp-remote/materials_page.go`
- Modify: `orderapp-remote/bom_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`

- [ ] **Step 1: Add API tests for materials**

Cover:

- list materials
- upsert material
- stock quantities
- purchase/sale prices

- [ ] **Step 2: Implement application/repository split**

Move SQL out of handlers into repository.

- [ ] **Step 3: Add Vue materials view**

`MaterialsView.vue` consumes JSON APIs and replaces `/materials` template behavior.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./... -run 'TestMaterials'
cd frontend-vue-shell && npm run build
go test ./...
```

Expected:

- materials tests PASS
- Vue shell build PASS
- full Go suite PASS

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/internal/application/materials orderapp-remote/internal/domain/materials orderapp-remote/internal/infrastructure/postgres/materials_repository.go orderapp-remote/materials.go orderapp-remote/materials_page.go orderapp-remote/frontend-vue-shell/src/views/MaterialsView.vue orderapp-remote/frontend-vue-shell/src/App.vue
git commit -m "refactor: migrate materials to layered vue api"
```

---

### Task 9: Final Architecture Gate

**Files:**

- Modify: `orderapp-remote/review_architecture_guard_test.go`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md` if architecture requirement tracking is mirrored there
- Update: 5 requirement tables through the app/db workflow

- [ ] **Step 1: Add architecture gate tests**

Add checks that future changes do not add new template-driven user pages:

```go
func TestVueShellOwnsMigratedProductionViews(t *testing.T) {
	body, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{"ProducePlanView", "ProduceRunningView", "ProductionLogsView"} {
		if !strings.Contains(content, want) {
			t.Fatalf("App.vue missing migrated production view %q", want)
		}
	}
}
```

- [ ] **Step 2: Run full verification**

Run:

```bash
cd /Users/yiiiple-work/Documents/KFerp/orderapp-remote
go test ./...
cd frontend-vue-shell && npm run build
```

Expected:

- all Go tests PASS
- Vue shell build PASS
- no legacy BOM frontend build remains

- [ ] **Step 3: Update requirement tables**

Create/complete:

- `PR-ARCH-001` 架构升级：Vue/Vite + 分层后端
- `DEV-ARCH-001` 工作流与架构护栏
- `DEV-ARCH-002` 日志/审计服务解耦
- `DEV-ARCH-003` 生产流程分层
- `DEV-ARCH-004` 生产页面 Vue/API 迁移
- `DEV-ARCH-005` 销售服务 use case 化
- `DEV-ARCH-006` 物料/BOM 分层迁移
- matching `UT-ARCH-*`, `API-ARCH-*`, `REV-ARCH-*`

- [ ] **Step 4: Push branch and open draft PR**

```bash
git status --short
git push -u origin codex/architecture-upgrade-20260426
```

Expected:

- branch pushed
- PR body lists completed tasks and verification evidence

---

## Acceptance Checklist

- [ ] `HOW_TO_WORKFLOW.md` requires test-first implementation.
- [ ] No corrected plan directs new user-facing work into `templates/*.html`.
- [ ] Production plan/logs/running are Vue internal views or explicitly queued migration tasks.
- [ ] Request logging writes `operation_logs` only.
- [ ] Business audit writes through `AuditService`.
- [ ] Transaction-sensitive business audit uses `auditInsertTx`.
- [ ] `production_flow.go` no longer owns route registration or schema DDL.
- [ ] Production use cases live in `internal/application/production`.
- [ ] Production pure rules live in `internal/domain/production`.
- [ ] Production SQL lives in repository/infra code.
- [ ] Sales service no longer acts only as repository passthrough.
- [ ] `go test ./...` passes.
- [ ] `frontend-vue-shell npm run build` passes.
- [ ] `frontend npm run build` passes.
- [ ] 5 requirement tables contain PR/DEV/UT/API/REV evidence for this architecture upgrade.

## Execution Recommendation

Run this plan in a dedicated branch after all active feature workflows have landed in `origin/develop`. Implement one task per commit, with full verification after each task. Do not merge partial architecture changes into `develop` unless the application remains deployable and tests/builds pass.
