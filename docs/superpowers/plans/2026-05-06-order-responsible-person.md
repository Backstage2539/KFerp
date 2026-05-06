# Order Responsible Person Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an order responsible person field that identifies who owns pre-sales and after-sales service for each order, with enough structure for future commission settlement.

**Architecture:** Store `responsible_party_type`, `responsible_party_id`, and `responsible_party_name` on `orders`. The API accepts a type/id pair and resolves the display name from either `company_employees` or `customers`, so commission logic can later join by stable entity and still retain a name snapshot.

**Tech Stack:** Go, Echo, PostgreSQL, Vue 3 + Vite, Node test runner.

---

### Task 1: Tests And Requirement Seeds

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_155_order_responsible_person_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] Add RED API tests for form options, saving employee/customer responsible parties, list output, and edit output.
- [ ] Add RED frontend tests for payload fields and option grouping.
- [ ] Add RED requirement seed/source wiring test for PR-155/DEV-155/UT-155/API-155/REV-155.

### Task 2: Backend Schema And API

**Files:**
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_dto.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_sales_mapping.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/core/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_queries.go`

- [ ] Add columns with idempotent migrations.
- [ ] Add employee options to `GET /api/order/form`.
- [ ] Resolve responsible party in `SaveOrder` and persist snapshot fields.
- [ ] Return responsible fields from list and edit APIs.

### Task 3: Vue Order Entry And List

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue`

- [ ] Build grouped responsible options from employees and customers.
- [ ] Add searchable `订单负责人` field to create/edit form.
- [ ] Show responsible person in order list and detail drawer.

### Task 4: Verification

- [ ] Run `go test ./internal/interfaces/http/sales ./internal/interfaces/http/support -count=1`.
- [ ] Run `node --test src/lib/order-entry.test.js`.
- [ ] Run `node --test src/lib/*.test.js src/api/*.test.js`.
- [ ] Run `npm run build --prefix orderapp-remote/frontend-vue-shell`.
- [ ] Run `go test ./... -count=1`.
