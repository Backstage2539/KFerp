# Manufacturing Planning And Quality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fill the ERPNext-inspired manufacturing gaps that matter next: material demand planning, work-order WIP reservations with partial completion, and production quality inspections.

**Architecture:** Keep orchestration in `internal/application/production`, production persistence in `internal/infrastructure/postgres/production`, and HTTP handlers in `internal/interfaces/http/production`. Reuse existing stock and purchase concepts instead of creating a second inventory system. Vue/Vite pages stay under `frontend-vue-shell/src/views`.

**Tech Stack:** Go, Echo, PostgreSQL/pgx, Vue 3/Vite, Node test runner.

---

### Task 1: Requirement Seeds

**Files:**
- Modify: `internal/interfaces/http/support/req_store.go`
- Test: `internal/interfaces/http/support/dev_110_step1_test.go`

- [x] Add PR/DEV/UT/API/REV rows for `PR-110`.
- [x] Verify the seed test fails before implementation.
- [x] Mark DEV/UT/API rows done with concrete evidence after implementation.

### Task 2: Material Demand Plan

**Files:**
- Modify: `internal/application/production/service.go`
- Modify: `internal/infrastructure/postgres/production/repository.go`
- Modify: `internal/interfaces/http/production/production_flow_routes.go`
- Modify: `frontend-vue-shell/src/views/ProducePlanView.vue`
- Test: `internal/application/production/service_flow_test.go`
- Test: `internal/interfaces/http/production/production_flow_api_test.go`

- [x] Add `MaterialPlanQuery`, `MaterialPlanRow`, and `MaterialPlanResult`.
- [x] Add `Service.MaterialPlan(ctx, query)`.
- [x] Implement repository material planning from current unproduced demand, BOM material needs, WIP stock, raw stock, and open WIP reservations.
- [x] Add `GET /api/produce/material-plan`.
- [x] Add a material planning table to the production plan page showing required, WIP, raw, reserved, shortage, and purchase suggestion quantities.

### Task 3: Work Order WIP Reservations And Partial Completion

**Files:**
- Modify: `internal/application/production/service.go`
- Modify: `internal/infrastructure/postgres/production/schema.go`
- Modify: `internal/infrastructure/postgres/production/repository.go`
- Modify: `internal/infrastructure/postgres/production/running_repository.go`
- Modify: `internal/infrastructure/postgres/production/material_consumption.go`
- Modify: `internal/infrastructure/postgres/production/work_order.go`
- Modify: `internal/interfaces/http/production/production_flow_routes.go`
- Modify: `frontend-vue-shell/src/views/ProduceRunningView.vue`
- Test: `internal/infrastructure/postgres/production/material_consumption_test.go`
- Test: `internal/interfaces/http/production/production_flow_api_test.go`

- [x] Add `work_order_material_reservations` table.
- [x] On production start, create one reservation row per material need.
- [x] WIP availability checks subtract other open reserved quantities.
- [x] Completing production increments reservation consumed quantities.
- [x] Partial completion consumes a declared or inferred input quantity, adds finished stock, writes production logs, and leaves the running item open with reduced remaining demand.
- [x] Full completion keeps existing completion behavior.
- [x] Cancellation releases open reservation rows.

### Task 4: Quality Inspection

**Files:**
- Modify: `internal/application/production/service.go`
- Modify: `internal/infrastructure/postgres/production/schema.go`
- Create: `internal/infrastructure/postgres/production/quality.go`
- Create: `internal/interfaces/http/production/manufacturing_gap_api.go`
- Modify: `internal/interfaces/http/production/module.go`
- Create: `frontend-vue-shell/src/views/QualityInspectionsView.vue`
- Modify: `frontend-vue-shell/src/App.vue`
- Modify: `frontend-vue-shell/src/lib/menu-ia.js`
- Test: `internal/interfaces/http/production/manufacturing_gap_api_test.go`
- Test: `internal/interfaces/http/support/dev_110_step1_test.go`

- [x] Add `quality_inspections` table.
- [x] Add create/list quality inspection service methods.
- [x] Add `GET/POST /api/produce/quality-inspections`.
- [x] Add `生产质检` page under production management.
- [x] Support scopes: raw material, work order/job card, finished batch.

### Task 5: Verification

- [x] Run `go test ./... -count=1`.
- [x] Run `node --test src/lib/*.test.js`.
- [x] Run `npm run build`.
- [x] Run `git diff --check`.
- [ ] Push `codex/manufacturing-planning-quality-20260428`.
