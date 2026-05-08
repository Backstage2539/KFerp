# Customer Processing ERP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the ERP-only customer processing workspace so bound customer accounts can view only their own代加工 data and submit processing/direct-ship requests, while internal users can manually adjust custody inventory.

**Architecture:** Reuse the existing `customerfulfillment` module for代加工业务数据 and add a small ERP-customer identity boundary keyed by `employee_id -> customer_id`. Internal admin APIs continue using explicit `customer_id`; customer-facing APIs derive `customer_id` from the logged-in employee binding and never accept it from the request body.

**Tech Stack:** Go/Echo/Postgres for APIs and persistence, Vue shell for ERP UI, existing authz roles/permissions, Go unit tests and Vitest source tests.

---

### Task 1: Auth And Customer Binding

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/authz/schema.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/authz_middleware.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/employee_context.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/schema.go`

- [ ] Add permissions `customer_processing.read` and `customer_processing.submit`.
- [ ] Add role `customer_processing_customer` with only those permissions.
- [ ] Add view permission `customerProcessingPortal -> customer_processing.read`.
- [ ] Add route permissions for `/api/customer-processing/portal`: GET requires read, non-GET requires submit.
- [ ] Export current employee ID from support middleware for customer route binding.
- [ ] Create `customer_erp_user_bindings` table with unique `(employee_id, customer_id)` and active status.
- [ ] Run `go test ./internal/application/authz ./internal/interfaces/http/support ./internal/infrastructure/postgres/customerfulfillment`.

### Task 2: Customer Processing Service APIs

**Files:**
- Modify: `orderapp-remote/internal/application/customerfulfillment/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/module.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`
- Modify tests under the same packages.

- [ ] Add repository/service methods:
  - `CustomerPortalContext(employeeID)`
  - `CustomerPortalOverview(employeeID)`
  - `SubmitCustomerProcessingWorkOrder(employeeID, payload)`
  - `SubmitCustomerDirectShipOrder(employeeID, payload)`
  - `AdjustCustodyInventory(customerID, payload)` for internal users.
  - `UpsertCustomerERPBinding(customerID, employeeID, status)` for internal setup.
- [ ] Customer route handlers must reject unbound employees and must not read `customer_id` from the body.
- [ ] Processing submissions write `customer_processing_work_orders` as `submitted`.
- [ ] Direct-ship submissions write `customer_direct_ship_import_orders` and item rows as `submitted`, without creating stock adjustments.
- [ ] Inventory adjustment writes `customer_custody_ledger_entries` and updates `customer_custody_balances`.
- [ ] Run targeted Go tests for customerfulfillment.

### Task 3: ERP UI

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/views/CustomerProcessingPortalView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`

- [ ] Add menu entry `customerProcessingPortal` under a customer-facing group.
- [ ] Add a customer-only workbench that loads `/api/customer-processing/portal/overview`.
- [ ] Show raw custody inventory, submitted/active work orders, finished-goods inventory, direct-ship orders, fees and settlements.
- [ ] Add forms for customer processing request and direct-ship request.
- [ ] Add internal inventory adjustment and ERP binding forms to `CustomerFulfillmentView`.
- [ ] Run `npm test -- customer-fulfillment` and `npm run build` in `frontend-vue-shell`.

### Task 4: Verification And Deployment

**Files:**
- Code from previous tasks.

- [ ] Run targeted Go tests.
- [ ] Run frontend build.
- [ ] Run full `go test ./...` if time permits.
- [ ] Deploy to development stack with `./deploy_orderapp.sh` or manual `scp + docker compose build orderapp && docker compose up -d`.
- [ ] Smoke test:
  - `/app/api/customer-fulfillment/149/overview` still works for admin.
  - `/app/api/customer-processing/portal/overview` rejects unbound users.
  - Bound customer user can fetch only its own overview.
  - Customer submit endpoints create submitted records without changing custody balance.
