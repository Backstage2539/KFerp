# WIP Shared Pool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a WIP shared-pool material flow where production can use material issued over multiple days and across multiple products.

**Architecture:** Keep warehouse movement in the stock module and production consumption in the production Postgres adapter. Material receipt creates raw-warehouse batch location; transfer moves batch location between raw and WIP without changing total material stock; production finish consumes only WIP batch locations.

**Tech Stack:** Go DDD modules, Postgres schema migrations, Echo JSON APIs, Vue/Vite shell.

---

### Task 1: Stock WIP Data Model

**Files:**
- Modify: `internal/application/stock/service.go`
- Modify: `internal/infrastructure/postgres/stock/schema.go`
- Modify: `internal/infrastructure/postgres/stock/repository.go`

- [x] Add warehouse presets for `raw_materials`, `packaging`, `wip`, `finished_goods`, and `loss`.
- [x] Add `material_batch_locations` to split one batch across raw and WIP warehouses.
- [x] Add `material_transfers` and `material_transfer_items` for issue/return documents.
- [x] Initialize raw warehouse location when material receipt creates a material batch.

### Task 2: Transfer APIs

**Files:**
- Modify: `internal/interfaces/http/stock/stock_api.go`
- Modify: `frontend-vue-shell/src/App.vue`
- Create: `frontend-vue-shell/src/views/WipMaterialsView.vue`

- [x] Add `GET /api/stock/warehouses`.
- [x] Add `GET /api/stock/material-batch-locations`.
- [x] Add `POST /api/stock/material-transfers`.
- [x] Add Vue WIP view for issue to WIP and return to raw warehouse.

### Task 3: Production Consumption

**Files:**
- Modify: `internal/infrastructure/postgres/production/material_consumption.go`

- [x] Change material batch allocation to read only WIP locations.
- [x] Deduct WIP location quantity, material batch remaining quantity, and stock batch remaining quantity on production finish.
- [x] Record production material ledger entries under `wip`.

### Task 4: Workflow Evidence

**Files:**
- Modify: `internal/interfaces/http/support/req_store.go`
- Add: `internal/interfaces/http/support/dev_081_step1_test.go`

- [x] Seed PR/DEV/UT/API/REV rows for PR-081.
- [x] Run unit/API/build checks before push.
