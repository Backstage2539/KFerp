# Production Acceptance And WIP Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add production-flow acceptance checks and practical WIP reservation visibility/control.

**Architecture:** Add pure reservation rules in the production domain, expose normalized production application use cases, implement Postgres read/write models in the production adapter, and wire focused Vue/Vite pages. Keep stock movement itself in the stock module; this slice only suggests WIP transfers and manages reservations.

**Tech Stack:** Go, Echo, pgx/Postgres, Vue 3 + Vite, existing PR/DEV/UT/API/REV seed tables.

---

### Task 1: Domain And Service Contracts

**Files:**
- Create: `orderapp-remote/internal/domain/production/reservation.go`
- Modify: `orderapp-remote/internal/domain/production/work_order_test.go`
- Modify: `orderapp-remote/internal/application/production/service.go`
- Modify: `orderapp-remote/internal/application/production/service_flow_test.go`

- [x] Add failing domain tests for remaining reservation and adjustment validation.
- [x] Add domain implementation.
- [x] Add application DTOs and repository interface methods.
- [x] Add service tests proving query/command normalization.

### Task 2: Postgres WIP Reservation And Acceptance Read Models

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/production/wip_reservation.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/production/acceptance_smoke.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/material_plan.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/work_order.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/manufacturing_gap_source_test.go`

- [x] Extend material plan rows with WIP available and transfer suggestion grams.
- [x] Add WIP reservation list, adjust, and release SQL.
- [x] Add acceptance checklist SQL.
- [x] Include work-order reservation summary fields.

### Task 3: HTTP API

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/production/manufacturing_gap_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/production/manufacturing_gap_api_test.go`

- [x] Add `GET /api/produce/acceptance-smoke`.
- [x] Add `GET /api/produce/wip-reservations`.
- [x] Add `POST /api/produce/wip-reservations/adjust`.
- [x] Add `POST /api/produce/wip-reservations/release`.

### Task 4: Vue/Vite UI And Manuals

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/views/ProductionAcceptanceView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProducePlanView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/WorkOrdersView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/WarehouseInventoryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductionManualView.vue`
- Modify: `orderapp-remote/docs/production-flow-user-manual.md`

- [x] Add production acceptance page.
- [x] Surface WIP transfer suggestions in material plan.
- [x] Surface reservation summary in work orders.
- [x] Add WIP reservation drawer in warehouse inventory.
- [x] Update production manual.

### Task 5: Workflow Tables And Verification

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_115_step1_test.go`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

- [x] Seed PR-115/DEV/UT/API/REV rows for this slice.
- [x] Update source requirements and acceptance tests.
- [x] Run `go test ./... -count=1`.
- [x] Run `node --test src/lib/*.test.js src/api/*.test.js`.
- [x] Run `npm run build`.
- [x] Run `git diff --check HEAD`.
