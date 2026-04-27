# Warehouse Inventory Menu Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace overlapping stock menu entries with warehouse-based inventory management and record old feature improvements in the requirements tables.

**Architecture:** Add a stock application read model for `warehouse + item + batch` inventory, keep legacy pages addressable but remove them from the left menu, and make the Vue shell menu group-collapsible. Low-frequency tracing remains reachable through warehouse inventory tabs rather than separate primary menu items.

**Tech Stack:** Go application/infrastructure/http layers, Vue 3 + Vite, Node test runner, PostgreSQL read models.

---

### Task 1: Stock Read Model

**Files:**
- Modify: `internal/application/stock/service.go`
- Modify: `internal/infrastructure/postgres/stock/repository.go`
- Modify: `internal/interfaces/http/stock/stock_api.go`
- Test: `internal/interfaces/http/stock/stock_api_test.go`

- [ ] Add `WarehouseInventoryQuery`, `WarehouseInventoryRow`, and `WarehouseInventoryResult`.
- [ ] Add `ListWarehouseInventory` to the stock repository interface and service.
- [ ] Add `GET /api/stock/warehouse-inventory`.
- [ ] Include material batch locations by warehouse and finished inventory under `finished_goods`.

### Task 2: Vue Inventory Workspace

**Files:**
- Create: `frontend-vue-shell/src/views/WarehouseInventoryView.vue`
- Create: `frontend-vue-shell/src/views/StockOperationsView.vue`
- Modify: `frontend-vue-shell/src/App.vue`
- Test: `frontend-vue-shell/src/lib/menu-ia.test.js`

- [ ] Create a warehouse tree + inventory table view.
- [ ] Create a stock operations tab page that hosts existing receipt, WIP transfer, and adjustment workflows.
- [ ] Replace separate left-menu stock pages with `仓库库存`, `库存作业`, and `物料档案`.
- [ ] Keep old view keys routed internally for URL compatibility, but remove them from the primary menu.

### Task 3: Collapsible Menu Groups

**Files:**
- Create: `frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `frontend-vue-shell/src/App.vue`
- Test: `frontend-vue-shell/src/lib/menu-ia.test.js`

- [ ] Extract menu metadata and helpers.
- [ ] Persist expanded group ids in `localStorage`.
- [ ] Auto-expand the current page's group.
- [ ] Keep existing whole-sidebar hide/show behavior.

### Task 4: Requirements Seeds

**Files:**
- Modify: `internal/interfaces/http/support/req_store.go`

- [ ] Add PR/DEV/UT/API/REV rows for the menu and warehouse inventory rework.
- [ ] Add old feature improvement DEV rows directly into the development requirement table, with `todo` status for follow-up work.

### Verification

- [ ] `go test ./... -count=1`
- [ ] `npm run build`
- [ ] `node --test src/lib/order-entry.test.js src/lib/costing-settings.test.js src/lib/produce-plan.test.js src/lib/menu-ia.test.js`
- [ ] `git diff --check`
