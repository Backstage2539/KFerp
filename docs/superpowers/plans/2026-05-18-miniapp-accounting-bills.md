# Miniapp Accounting Bills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the miniapp settlement tab into an accounting-focused bill page with period filters, receivable/payment summary, and lightweight order-bill rows.

**Architecture:** Keep `/api/mini/services/settlement` as the single bill endpoint. The backend computes settlement summary from the current customer's filtered orders and exposes `payment_method`; the miniapp uses a settlement-specific UI that shows accounting rows and jumps to the order tab for detail.

**Tech Stack:** Go customer portal service/repository/tests; Vue 3 + uni-app miniapp; Markdown operation docs and req seed evidence.

---

### Task 1: Backend Billing Summary

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Test: `orderapp-remote/internal/application/customerportal/service_test.go`

- [ ] Write a failing service test that a settlement page with paid and unpaid orders summarizes `应收总额`, `未付款金额`, `未付款订单`, and `已付款金额`.
- [ ] Run `go test ./internal/application/customerportal -run TestGetSettlementServicePageSummaryShowsReceivableLedger -count=1 -v` and confirm it fails.
- [ ] Implement settlement summary by parsing order totals and classifying paid statuses containing `已付`, `已收`, or `已支付`.
- [ ] Run the same test and confirm it passes.

### Task 2: Repository Order Bill Fields

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`

- [ ] Write a failing repository test proving settlement order rows include `payment_method`, respect date and pay-status filters, and remain current-customer scoped.
- [ ] Run `go test ./internal/infrastructure/postgres/customerportal -run TestLoadSettlementServicePageFiltersOrderBillsByPeriodAndPayment -count=1 -v`.
- [ ] Add `PaymentMethod` to `CustomerOrderSummary` and query `o.payment_method` from `listCustomerOrders`.
- [ ] Run the targeted repository test. If no local database exists, record the skip and rely on Docker build/server verification later.

### Task 3: Miniapp Billing UI

**Files:**
- Modify: `miniapp/src/utils/orderFilters.ts`
- Modify: `miniapp/src/utils/orderFilters.test.ts`
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/pages/service/service.vue`

- [ ] Write failing tests for `week`, `month`, and `year` period presets.
- [ ] Run `npm test -- --run src/utils/orderFilters.test.ts` and confirm the new test fails before implementation.
- [ ] Extend preset helpers and settlement filters; default settlement page to current month.
- [ ] Replace settlement order rendering with lightweight bill rows and an order-number tap handler that opens `/pages/service/service?key=orders&q=<order_no>`.
- [ ] Run the targeted miniapp test and confirm it passes.

### Task 4: Docs And Evidence

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Add: `orderapp-remote/internal/interfaces/http/support/dev_288_miniapp_accounting_bills_test.go`

- [ ] Add PR/DEV markers for accounting-focused miniapp bills.
- [ ] Update the operation manual with period filters, summary meaning, and order-number navigation.
- [ ] Add support evidence tests for docs and source wiring.
- [ ] Run `go test ./internal/interfaces/http/support -run TestMiniappAccountingBills -count=1 -v`.

### Task 5: Verification And Deployment

**Files:**
- No source edits unless verification finds a defect.

- [ ] Run `go test ./... -count=1` in `orderapp-remote`.
- [ ] Run `npm test`, `npm run typecheck`, and `VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin` in `miniapp`.
- [ ] Run `npm run build` in `orderapp-remote/frontend-vue-shell`.
- [ ] Commit, push feature branch, merge latest `origin/develop`, rerun relevant checks.
- [ ] Merge to `develop`, push, deploy with `./deploy_orderapp.sh`.
- [ ] Verify 岩师傅 settlement API returns accounting summary and lightweight order bill fields; verify public app smoke checks.
