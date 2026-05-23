# Order Backfill Dates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add editable ERP document date and customer order date fields for order backfill, then show both dates on sales orders and delivery notes.

**Architecture:** Persist `orders.document_date` beside existing `orders.order_date`. The API accepts and returns both dates; UI payloads send both. Document snapshots carry both dates so renderers are stable even after later edits.

**Tech Stack:** Go, PostgreSQL, Echo HTTP handlers, Vue/Vite, Node test runner, gofpdf.

---

### Task 1: Red Tests

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/sales_order_pdf_test.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/delivery_note_pdf_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] Add an API test that saves `document_date=2026-05-23` and `order_date=2026-05-20`, then fetches edit data and expects both values.
- [ ] Add PDF tests that set snapshot `DocumentDate` and `OrderDate`, then verify rendered metadata rows include both Chinese labels.
- [ ] Add a frontend payload test expecting `buildOrderPayload` to include `document_date`.
- [ ] Run targeted tests and confirm they fail because the feature is missing.

### Task 2: Backend Date Model

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/core/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/schema.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_dto.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_sales_mapping.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_queries.go`

- [ ] Add `document_date` schema creation and migration with `UPDATE orders SET document_date=order_date WHERE document_date IS NULL`.
- [ ] Parse `document_date` in create/update commands, defaulting it to `order_date` when omitted.
- [ ] Save and update `document_date` in `orders`.
- [ ] Return `document_date` in order edit and list data.
- [ ] Generate new order numbers from `document_date`.

### Task 3: Document Snapshots and Renderers

**Files:**
- Modify: `orderapp-remote/internal/domain/sales/sales_order.go`
- Modify: `orderapp-remote/internal/domain/sales/delivery_note.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/sales_order_pdf.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/delivery_note_pdf.go`

- [ ] Add `DocumentDate` to sales order snapshots and delivery note snapshots.
- [ ] Load both dates from orders, falling back `document_date` to `order_date`.
- [ ] Render `单据日期` and `订单日期` on sales orders.
- [ ] Render `单据日期`, `订单日期`, and existing `出库日期` on delivery notes.

### Task 4: Vue Entry and List UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrdersView.vue`

- [ ] Add `document_date` to the order entry form and payload.
- [ ] Show `订单补录` helper text and both editable date inputs.
- [ ] Populate both dates from create defaults and edit API.
- [ ] Show both dates in order list rows and order detail drawer header.

### Task 5: Manuals, Requirements, Seeds, and Acceptance

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `docs/acceptance/2026-05-23-order-backfill-dates.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-23-order-backfill-dates.md`

- [ ] Add PR/DEV seed rows for the feature.
- [ ] Update operation manuals and acceptance checklist.
- [ ] Record local verification evidence.

### Task 6: Verification and Integration

- [ ] Run `go test ./...` in `orderapp-remote`.
- [ ] Run `node --test src/lib/*.test.js src/api/*.test.js` in `frontend-vue-shell`.
- [ ] Run `npm run build` in `frontend-vue-shell`.
- [ ] Run `git diff --check`.
- [ ] Commit, push feature branch, merge latest `origin/develop`, rerun checks, merge to `develop`, push, deploy development stack, and smoke test.
