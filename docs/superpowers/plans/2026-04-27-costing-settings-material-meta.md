# Costing Settings Material Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add ERP-managed costing settings, material batch/bean-card metadata, commercial pound tiers, and richer costing/bean-list previews.

**Architecture:** Keep formulas in `internal/domain/costing`, orchestration in `internal/application/costing`, persistence in Postgres adapters, JSON routes in HTTP modules, and UI in Vue shell views. Material purchase price remains the single cost source.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, Vue 3 + Vite.

---

### Task 1: Commercial Costing Tiers

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Modify: `orderapp-remote/internal/domain/costing/engine_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`

- [x] Write a failing test that expects four commercial tiers: `2-13磅`, `14-23磅`, `24-47磅`, `大于47磅`.
- [x] Implement `CommercialWholesaleTiers` output with kg/lb price values.
- [x] Publish commercial tiers as `spec_g=454` package tiers with ranges `2-13`, `14-23`, `24-47`, and `48+`.

### Task 2: Costing Settings API

**Files:**
- Modify: `orderapp-remote/internal/application/costing/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`

- [x] Add application DTOs for parameter rows and update commands.
- [x] Add repository methods to list and update `cost_parameters`.
- [x] Add `GET /api/costing/settings` and `POST /api/costing/settings/:key`.
- [x] Test settings route registration and update behavior with a fake service.

### Task 3: Material Batch And Bean-Card Fields

**Files:**
- Modify: `orderapp-remote/internal/application/materials/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/repository_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/materials/materials_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/MaterialsView.vue`

- [x] Add material fields: batch number, origin, processing station, variety, process method, grade, altitude, flavor, bean-list note.
- [x] Default blank batch number to today's `YYYYMMDD`.
- [x] Include new fields in list/update JSON and Vue material grid.
- [x] Extend material diff logging for changed metadata fields.

### Task 4: Vue Costing And Settings Views

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Create: `orderapp-remote/frontend-vue-shell/src/views/CostingSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`

- [x] Show commercial tier columns in price trial.
- [x] Split bean-list preview into commercial wholesale and retail sections.
- [x] Add editable costing settings table under `设置`.

### Task 5: Workflow Seeds And Verification

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_075_step1_test.go`

- [x] Seed PR/DEV/UT/API/REV rows for this follow-up.
- [x] Run `cd orderapp-remote && go test ./...`.
- [x] Run `cd orderapp-remote/frontend-vue-shell && npm run build`.
- [ ] Deploy with `./deploy_orderapp.sh` after merge to `develop`.
