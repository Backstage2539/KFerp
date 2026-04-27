# Excel Costing DDD Adaptation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Excel costing migration on top of the current `develop` DDD architecture and deploy it for server testing.

**Architecture:** Pure formulas live in `internal/domain/costing`; use cases live in `internal/application/costing`; Postgres details live in `internal/infrastructure/postgres/costing`; Echo routes live in `internal/interfaces/http/costing`; `internal/appmain` only wires modules together. Vue UI is a normal `frontend-vue-shell/src/views` component.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, Vue 3 + Vite, existing KFerp Docker deployment.

---

### Task 1: Domain Costing Engine

**Files:**
- Create: `orderapp-remote/internal/domain/costing/engine.go`
- Create: `orderapp-remote/internal/domain/costing/engine_test.go`

- [ ] Write tests for Excel cached values: roasted bean cost `77.5`, wholesale kg `132.29283486842107`, wholesale drip bag `3.0803695000000006`, retail drip 10 bags `43.26055625000001`.
- [ ] Run `go test ./internal/domain/costing -count=1` and confirm it fails because the package/functions are missing.
- [ ] Implement domain structs, defaults, validation, and calculation.
- [ ] Run `go test ./internal/domain/costing -count=1` and confirm it passes.

### Task 2: Application Service

**Files:**
- Create: `orderapp-remote/internal/application/costing/service.go`
- Create: `orderapp-remote/internal/application/costing/service_test.go`

- [ ] Write tests for empty calculate requests, valid calculate requests, and saved-run orchestration.
- [ ] Run `go test ./internal/application/costing -count=1` and confirm it fails before service implementation.
- [ ] Implement repository interface and service methods for parameters, calculate, bean-list, save run, and publish.
- [ ] Run `go test ./internal/application/costing -count=1` and confirm it passes.

### Task 3: Postgres Adapter And Schema

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/appmain/schema_setup.go`

- [ ] Add schema setup for `cost_parameters`, `cost_calculation_runs`, and `cost_calculation_items`.
- [ ] Seed default cost parameters idempotently.
- [ ] Implement product/BOM/material input loading and run persistence.
- [ ] Implement publishing into catalog price fields and `product_price_tiers`, with audit logs.
- [ ] Wire costing schema into `internal/appmain/schema_setup.go`.

### Task 4: HTTP Costing Module

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/costing/module.go`
- Create: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Create: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`
- Modify: `orderapp-remote/internal/appmain/app_routes.go`

- [ ] Write API tests for `POST /api/costing/calculate` and route registration.
- [ ] Run `go test ./internal/interfaces/http/costing -count=1` and confirm it fails before route implementation.
- [ ] Implement routes for parameters, calculate, bean-list, runs, and publish.
- [ ] Wire costing routes into `internal/appmain/app_routes.go`.
- [ ] Run `go test ./internal/interfaces/http/costing -count=1`.

### Task 5: Vue View And Requirement Workflow

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_074_step1_test.go`

- [ ] Add the `成本核算` menu entry under `物料管理`.
- [ ] Implement a Vue costing view using `apiGet` and `apiSend`.
- [ ] Add PR-074 workflow rows through `seedReqRow`.
- [ ] Test the seed strings exist in the support package.
- [ ] Run `npm run build` in `orderapp-remote/frontend-vue-shell`.

### Task 6: Verification And Deployment

**Commands:**
- `cd orderapp-remote && go test ./...`
- `cd orderapp-remote/frontend-vue-shell && npm run build`
- push feature branch
- merge into `develop` only after tests pass
- push `develop`
- `./deploy_orderapp.sh`
- postdeploy smoke checks from `kferp-deploy`

**Acceptance Evidence:**
- Unit tests show the Excel cached formula values are preserved.
- API-level tests cover costing calculation without UI.
- Vue shell build includes the new costing view.
- Server deployment completes and `/app/` returns the expected authenticated app response.

