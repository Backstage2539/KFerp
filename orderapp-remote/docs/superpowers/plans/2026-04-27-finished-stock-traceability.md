# Finished Stock Traceability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete DEV-087-03 and DEV-087-04 by making finished goods stock warehouse-aware and document-driven, and by freezing production material snapshots so traceability follows the actual work order.

**Architecture:** Keep changes inside the existing DDD split. Inventory/stock application services own finished goods commands and read models; Postgres adapters own schema migration and transactional stock movement. Production stores a JSON material snapshot on `produce_running_items` / `work_orders` and uses it at finish time instead of rereading mutable BOM rows.

**Tech Stack:** Go 1.22, pgx/Postgres, Echo HTTP APIs, Vue/Vite frontend shell, Node test runner.

---

### Task 1: Warehouse-Aware Finished Inventory Schema

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/inventory/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/repository.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/stock/repository_test.go`

- [ ] Add failing Postgres test that calls `stock.EnsureSchema` on an old `finished_inventory(product_id,spec_g)` table and then inserts two warehouses for the same product/spec.
- [ ] Expected RED: duplicate warehouse insert fails or `warehouse` column is missing.
- [ ] Add `warehouse TEXT NOT NULL DEFAULT 'finished_goods'`, replace old PK with `(product_id,spec_g,warehouse)`, and update warehouse inventory query to read `fi.warehouse`.
- [ ] GREEN: the same product/spec can exist in `finished_goods` and another finished warehouse, and `GET /api/stock/warehouse-inventory` can filter by either warehouse.

### Task 2: Finished Goods Documents

**Files:**
- Modify: `orderapp-remote/internal/application/stock/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/stock/stock_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/StockAdjustmentsView.vue`
- Test: `orderapp-remote/internal/application/stock/service_test.go`
- Test: `orderapp-remote/internal/interfaces/http/stock/stock_api_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/stock/repository_test.go`

- [ ] Add failing service/API tests for finished goods transfer and finished goods adjustment with `warehouse`.
- [ ] Expected RED: missing command/API or warehouse is ignored.
- [ ] Add `finished_product_transfers` table and `TransferFinishedProduct` service/repository/API.
- [ ] Update `CreateAdjustment` for `finished_product` to use `warehouse`, defaulting to `finished_goods`, and write ledger batch rows against that warehouse.
- [ ] Add a transfer tab/control to the existing Stock Operations workspace, not a new main menu page.
- [ ] GREEN: finished goods can be adjusted into one finished warehouse and transferred to another with paired ledger entries.

### Task 3: Production Finish Writes Finished Batch Into Warehouse Ledger

**Files:**
- Modify: `orderapp-remote/internal/application/production/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/production/production_flow_routes.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/running_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/stock_ledger.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProduceRunningView.vue`
- Test: `orderapp-remote/internal/interfaces/http/production/production_flow_api_test.go`

- [ ] Add failing test that finishing a running item writes `finished_inventory` with `warehouse='finished_goods'`, creates `FP-*` stock batch with remaining quantity, and ledger row uses the same warehouse.
- [ ] Expected RED: query with warehouse column fails or no row exists.
- [ ] Add `Warehouse` to `FinishCommand`, normalize default to `finished_goods`, bind API JSON field `warehouse`, and include warehouse in production finish inserts.
- [ ] GREEN: production finish becomes a finished goods receipt document into the selected/default finished warehouse.

### Task 4: Work Order Material Snapshot Freeze

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/material_consumption.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/work_order.go`
- Test: `orderapp-remote/internal/interfaces/http/production/production_flow_api_test.go`
- Test: `orderapp-remote/internal/interfaces/http/production/work_order_api_test.go`

- [ ] Add failing test: start production with BOM material A, change BOM to material B, finish production, and assert material A is consumed and shown in work order material summary.
- [ ] Expected RED: material B is consumed because current BOM is reread.
- [ ] Add `material_snapshot JSONB NOT NULL DEFAULT '[]'` to `produce_running_items` and `work_orders`.
- [ ] Build snapshot at production start from BOM item rows and packaging mapping; pass it into work order creation.
- [ ] Make finish use the stored snapshot first, with current BOM only as legacy fallback.
- [ ] GREEN: later BOM edits do not alter already-started work orders.

### Task 5: Traceability Read Model

**Files:**
- Modify: `orderapp-remote/internal/application/stock/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/stock/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/stock/stock_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/WarehouseInventoryView.vue`
- Test: `orderapp-remote/internal/interfaces/http/stock/stock_api_test.go`

- [ ] Add failing API test for `GET /api/stock/trace?batch=FP-0000000042` returning finished batch, production log, and material batch consumption rows.
- [ ] Expected RED: route missing.
- [ ] Implement trace read model joined from `stock_batches`, `production_logs`, and `material_consumption_logs`.
- [ ] Add a compact trace drawer in Warehouse Inventory so trace lookup stays inside the stock setting/query page.
- [ ] GREEN: trace API returns finished-to-material batch chain without adding a new main menu page.

### Task 6: Requirements And Verification

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Add/Modify: `orderapp-remote/internal/interfaces/http/support/dev_087_step2_test.go`

- [ ] Add failing requirement seed guard for `DEV-087-03` and `DEV-087-04` status moving from `todo` to `done`, with UT/API evidence rows.
- [ ] Expected RED: seed status/evidence not updated.
- [ ] Update requirement rows and add UT/API/REV evidence.
- [ ] Run `go test ./... -count=1`, `npm run build`, targeted Node tests, and `git diff --check`.
