# Finance Monthly Closing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build KFerp finance v1: monthly finance dashboard, expense management, medium-depth tax estimates, strong-lock monthly closing, amount adjustments, boss brief reports, and PDF/Excel exports.

**Architecture:** Add a dedicated `finance` DDD module. Pure calculation rules live in `internal/domain/finance`; use cases live in `internal/application/finance`; Postgres schema and queries live in `internal/infrastructure/postgres/finance`; Echo handlers live in `internal/interfaces/http/finance`. Vue/Vite receives finance views and menu entries only; no legacy templates are added or changed.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, gofpdf, excelize, Vue 3 + Vite, Node test runner.

---

## File Structure

- Create `orderapp-remote/internal/domain/finance/model.go`: money, taxpayer settings, company type, monthly metrics, tax estimate, closing lock and adjustment domain rules.
- Create `orderapp-remote/internal/domain/finance/model_test.go`: red/green tests for net profit, taxpayer estimates, strong lock, and adjustments.
- Create `orderapp-remote/internal/application/finance/service.go`: finance use cases and repository interfaces.
- Create `orderapp-remote/internal/application/finance/service_test.go`: service tests using a fake repository.
- Create `orderapp-remote/internal/infrastructure/postgres/finance/schema.go`: finance settings, expenses, closing snapshots, adjustments, and audit-friendly tables.
- Create `orderapp-remote/internal/infrastructure/postgres/finance/repository.go`: settings, expenses, monthly aggregation, closing, adjustments, and export data loading.
- Create `orderapp-remote/internal/interfaces/http/finance/module.go`: route registration.
- Create `orderapp-remote/internal/interfaces/http/finance/finance_api.go`: JSON and export endpoints.
- Create `orderapp-remote/internal/interfaces/http/finance/finance_api_test.go`: handler-level API tests.
- Create `orderapp-remote/internal/infrastructure/pdf/finance_report_pdf.go`: monthly report PDF rendering.
- Create `orderapp-remote/internal/infrastructure/excel/finance_report_excel.go`: monthly report Excel rendering.
- Modify `orderapp-remote/internal/appmain/schema_setup.go`: include finance schema setup.
- Modify `orderapp-remote/internal/appmain/app_routes.go`: wire finance repository, service, and routes.
- Modify `orderapp-remote/internal/infrastructure/postgres/authz/schema.go`: seed finance permissions and view permissions.
- Modify `orderapp-remote/internal/interfaces/http/support/authz_middleware.go`: protect `/api/finance`.
- Modify `orderapp-remote/internal/interfaces/http/support/req_store.go`: keep PR/DEV/UT/API/REV finance rows and update evidence as tasks complete.
- Create `orderapp-remote/frontend-vue-shell/src/lib/finance.js`: formatters and report helpers.
- Create `orderapp-remote/frontend-vue-shell/src/lib/finance.test.js`: front-end helper/source tests.
- Create `orderapp-remote/frontend-vue-shell/src/views/FinanceDashboardView.vue`: owner dashboard and exception cards.
- Create `orderapp-remote/frontend-vue-shell/src/views/FinanceExpensesView.vue`: expense entry and list.
- Create `orderapp-remote/frontend-vue-shell/src/views/FinanceClosingView.vue`: draft generation, strong-lock closing, adjustments, and hidden whitelist mode switch.
- Create `orderapp-remote/frontend-vue-shell/src/views/FinanceReportView.vue`: boss brief, detail tabs, and export links.
- Create `orderapp-remote/frontend-vue-shell/src/views/FinanceSettingsView.vue`: company/taxpayer settings.
- Modify `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js` and `src/App.vue`: add finance menu group and views.
- Modify root and `orderapp-remote/docs` requirement/acceptance/manual docs with finance user workflow.

## Task 1: Domain Finance Rules

**Files:** `internal/domain/finance/model.go`, `internal/domain/finance/model_test.go`

- [ ] Write failing tests for operating net profit, small-scale VAT estimate, general VAT estimate, small low-profit CIT preference, strong-lock adjustment requirement, and amount adjustment totals.
- [ ] Run `go test ./internal/domain/finance -count=1` and confirm it fails because the package does not exist.
- [ ] Implement normalized settings, monthly metric calculation, tax estimate calculation, closing mode validation, and adjustment application.
- [ ] Run `go test ./internal/domain/finance -count=1` and confirm it passes.

## Task 2: Application Service

**Files:** `internal/application/finance/service.go`, `internal/application/finance/service_test.go`

- [ ] Write failing tests for default settings, dashboard summary, expense validation, draft generation, close-month strong lock, whitelist mode switching, and adjustment creation.
- [ ] Run `go test ./internal/application/finance -count=1` and confirm it fails because the package does not exist.
- [ ] Implement the finance service with repository interface methods for settings, expenses, month aggregation, reports, exports, close, and adjustments.
- [ ] Run `go test ./internal/application/finance -count=1` and confirm it passes.

## Task 3: Postgres Persistence

**Files:** `internal/infrastructure/postgres/finance/schema.go`, `internal/infrastructure/postgres/finance/repository.go`

- [ ] Write failing repository/schema tests for finance tables, seed settings, expenses, snapshot closing, adjustment persistence, and cross-source monthly aggregation.
- [ ] Run focused postgres finance tests and confirm they fail because the package does not exist.
- [ ] Implement finance schema, setting seed rows, expense CRUD, aggregation queries, snapshot persistence, amount adjustments, and audit inserts.
- [ ] Run focused postgres finance tests and confirm they pass.

## Task 4: HTTP API And Exports

**Files:** `internal/interfaces/http/finance/*.go`, `internal/infrastructure/pdf/finance_report_pdf.go`, `internal/infrastructure/excel/finance_report_excel.go`

- [ ] Write failing handler tests for settings, dashboard, expenses, draft report, close, adjust, mode switch authorization, PDF export, and Excel export.
- [ ] Run `go test ./internal/interfaces/http/finance -count=1` and confirm it fails.
- [ ] Implement finance routes, JSON DTOs, authorization checks, PDF rendering, and Excel rendering.
- [ ] Run `go test ./internal/interfaces/http/finance -count=1` and confirm it passes.

## Task 5: App Wiring And Permissions

**Files:** `internal/appmain/*.go`, `internal/infrastructure/postgres/authz/schema.go`, `internal/interfaces/http/support/authz_middleware.go`

- [ ] Write failing source/permission guard tests proving finance routes, schema, permissions, and views are wired.
- [ ] Run focused authz/support/appmain tests and confirm they fail.
- [ ] Wire finance schema/routes and seed `finance.read`, `finance.write`, `finance.close`, `finance.close_mode.manage`, plus finance view permissions.
- [ ] Run focused authz/support/appmain tests and confirm they pass.

## Task 6: Vue Finance UI

**Files:** `frontend-vue-shell/src/views/Finance*.vue`, `frontend-vue-shell/src/lib/finance.js`, `frontend-vue-shell/src/lib/menu-ia.js`, `frontend-vue-shell/src/App.vue`

- [ ] Write failing Node/source tests for finance menu entries, boss brief default, detail tabs, hidden whitelist switch, and export links.
- [ ] Run `node --test src/lib/*.test.js src/api/*.test.js` and confirm the finance tests fail.
- [ ] Implement finance views using JSON APIs, readable management labels, compact report layout, strong-lock adjustment UI, and settings UI.
- [ ] Run Node tests and `npm run build` and confirm they pass.

## Task 7: Requirements, Manuals, And Review Evidence

**Files:** `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/docs/*`, `orderapp-remote/internal/interfaces/http/support/req_store.go`

- [ ] Update user-facing manuals and source docs for the finance workflow.
- [ ] Update PR-FIN-001 rows with implementation evidence.
- [ ] Add focused tests guarding the requirement seeds and manual links.
- [ ] Run support tests and confirm PR/DEV/UT/API/REV finance rows are present.

## Task 8: Full Verification

**Commands:**

- [ ] `cd orderapp-remote && go test ./... -count=1`
- [ ] `cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js src/api/*.test.js`
- [ ] `cd orderapp-remote/frontend-vue-shell && npm run build`
- [ ] `git diff --check`

**Acceptance Review:**

- [ ] Verify each G1-G4 acceptance item in `ACCEPTANCE_TESTS.md`.
- [ ] Record evidence in `REV-FIN-001` and related UT/API rows.
