# Order Backfill Continuous Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real `订单补录` continuous entry mode so operators can save and immediately enter the next backfilled order with shared header context retained.

**Architecture:** Keep backend order save unchanged. Add Vue state and post-save branching in `OrderEntryView.vue`, with source-level tests in `order-entry.test.js` and support seed tests in Go. Update workflow docs and acceptance evidence alongside the UI change.

**Tech Stack:** Vue 3 + Vite frontend, Node test runner, Go support tests, Markdown manuals and acceptance docs.

---

### Task 1: Red Tests

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_348_order_backfill_continuous_mode_test.go`

- [x] Add a failing frontend test named `order entry exposes continuous backfill mode and save flow`.
- [x] Assert the source contains `backfillMode`, `保存并继续补录`, `save({ continueBackfill: true })`, and `resetForBackfillContinuation`.
- [x] Assert the reset function clears tracking number, payment amount fields, payment voucher id, notes, discount, outsourced fee fields, and replaces rows with `[newRow()]`.
- [x] Add a failing support test that expects `PR-348-ORDER-BACKFILL-CONTINUOUS-MODE`, `DEV-348-ORDER-BACKFILL-CONTINUOUS-MODE`, `UT-348-ORDER-BACKFILL-CONTINUOUS-MODE`, `API-348-ORDER-BACKFILL-CONTINUOUS-MODE`, and `REV-348-ORDER-BACKFILL-CONTINUOUS-MODE` in `req_store.go`.
- [x] Add support assertions that docs mention `保存并继续补录` and the acceptance evidence file exists.
- [x] Run `node --test src/lib/order-entry.test.js` and `go test ./internal/interfaces/http/support -run TestDev348 -count=1`; both should fail because the feature and seeds are not yet present.

### Task 2: Vue Implementation

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`

- [x] Add `backfillMode = ref(false)` and `canUseBackfillMode = computed(() => !form.edit_id && !copyMode.value)`.
- [x] Persist `backfillMode` in the existing order entry draft save/restore.
- [x] Add a checkbox-style control in the `订单补录` area. It must make the mode clearly clickable and keep both date inputs visible and editable.
- [x] Add a save action group. Normal mode keeps `保存订单`; backfill mode shows `保存并查看订单` and `保存并继续补录`.
- [x] Change `save()` to accept `{ continueBackfill = false }`.
- [x] After successful save, if `continueBackfill` is true, call `resetForBackfillContinuation()`, skip redirect, keep draft saving enabled, and store a fresh draft with the preserved context.
- [x] Implement `resetForBackfillContinuation()` according to the design reset/preserve lists.

### Task 3: Docs And Seeds

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `docs/acceptance/2026-05-23-order-backfill-continuous-mode.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-23-order-backfill-continuous-mode.md`

- [x] Add PR/DEV/UT/API/REV rows for PR-348.
- [x] Add acceptance criteria for one-off backfill and continuous backfill.
- [x] Update the order sales manual with exact operator steps.
- [x] Add acceptance evidence listing the test commands to run.

### Task 4: Verification And Delivery

**Files:**
- No planned source changes beyond Tasks 1-3.

- [x] Run targeted tests: `node --test src/lib/order-entry.test.js`; `go test ./internal/interfaces/http/support -run TestDev348 -count=1`.
- [x] Run full local verification: `go test ./...`; `node --test src/lib/*.test.js src/api/*.test.js`; `npm run build`; `git diff --check`.
- [x] Start the Vue dev server.
- [ ] Inspect the order entry page with a live backend in the development stack because local Vite does not proxy auth APIs.
- [ ] Commit and push the feature branch.
- [ ] Fetch latest `origin/develop`, merge/rebase into the feature branch if needed, rerun verification, merge to `develop`, push, deploy development stack, and smoke-test `/app/`, `/app/vue-shell?view=order`, and `/app/api/order/form`.
