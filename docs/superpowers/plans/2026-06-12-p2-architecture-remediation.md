# P2 Architecture Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete a behavior-preserving P2 architecture remediation pass for ERP Vue mega components, the miniapp service page, and Postgres migration/versioning structure.

**Architecture:** Move API access and pure UI decision logic out of oversized page components into testable modules, then add guard tests so the logic does not drift back into single-file pages. Introduce a migration ledger skeleton and tests without changing existing `EnsureSchema` bootstrap behavior or applying destructive data migrations.

**Tech Stack:** Vue 3 + Vite, Node test runner, UniApp/Vitest miniapp tests, Go architecture tests, pgx/postgres schema bootstrap.

---

### Task 1: ERP Vue API and Page-Boundary Split

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/api/view-context.js`
- Create: `orderapp-remote/frontend-vue-shell/src/api/view-context.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/frontend-architecture.test.js`

- [ ] **Step 1: Write failing frontend tests**

Add tests proving workspace customer options use `/api/view-context/options` first, then fall back to fulfillment customers and customer list endpoints. Add a static architecture test requiring `App.vue` to import the view-context API module rather than owning hardcoded view-context API endpoint strings.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/api/view-context.test.js src/lib/frontend-architecture.test.js
```

Expected: fail because `src/api/view-context.js` is missing and `App.vue` still owns hardcoded view-context endpoint strings.

- [ ] **Step 3: Extract view-context API helpers**

Implement `fetchWorkspaceCustomerOptions`, `fetchWorkspaceOrderOptions`, `fetchViewContextPresets`, `saveViewContextPreset`, and `disableViewContextPreset`, then wire `App.vue` to those helpers. Keep all endpoint paths and UI state unchanged.

- [ ] **Step 4: Run GREEN**

Run the same Node tests and confirm they pass.

### Task 2: Costing Price-List Workflow Helper Boundary

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/lib/costing-price-list-workflow.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/costing-price-list-workflow.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/frontend-architecture.test.js`

- [ ] **Step 1: Write failing tests**

Add a pure helper test for price-list pricing-rule trial request selection and a static guard requiring `CostingView.vue` to import that helper.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/costing-price-list-workflow.test.js src/lib/frontend-architecture.test.js
```

Expected: fail because the helper file is missing and the view still owns the request selection loop.

- [ ] **Step 3: Extract helper**

Move `priceListPricingRuleTrialRequestsForRows` into the helper module, with dependency injection for `priceTablePricingRuleTrialPayload` and `priceTablePricingRuleTrialCacheKey`, then call it from `CostingView.vue`.

- [ ] **Step 4: Run GREEN**

Run the same tests plus existing costing/product-setting helper tests.

### Task 3: Miniapp Service Page Helper Boundary

**Files:**
- Create: `miniapp/src/utils/serviceForms.ts`
- Create: `miniapp/src/utils/serviceForms.test.ts`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/utils/servicePage.test.ts`

- [ ] **Step 1: Write failing miniapp tests**

Add tests for default direct-ship, processing, fulfillment, and order-search forms. Add a static service-page test requiring service.vue to import form factories rather than inline all defaults.

- [ ] **Step 2: Run RED**

Run:

```bash
cd miniapp
npm test -- --run src/utils/serviceForms.test.ts src/utils/servicePage.test.ts
```

Expected: fail because `serviceForms.ts` is missing and service.vue still owns inline form defaults.

- [ ] **Step 3: Extract form factories**

Move form type definitions and default form factories into `serviceForms.ts`. Replace inline default objects in service.vue with factory calls.

- [ ] **Step 4: Run GREEN**

Run the same miniapp tests and then `npm test`.

### Task 4: Postgres Migration Ledger Skeleton

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/migrations.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/migrations_test.go`
- Modify: `orderapp-remote/internal/architecture/ddd_module_test.go`
- Create: `orderapp-remote/docs/migrations/README.md`

- [ ] **Step 1: Write failing Go tests**

Add tests for `schema_migrations` ledger DDL, migration ordering, and architecture guard that requires a migrations README and migration ledger API.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/infrastructure/postgres -run 'TestMigration' -count=1
go test ./internal/architecture -run TestPostgresMigrationLedgerExists -count=1
```

Expected: fail because the migration ledger API and README are missing.

- [ ] **Step 3: Implement migration skeleton**

Implement `Migration`, `MigrationLedgerDDL`, `ValidateMigrations`, and `EnsureMigrationLedger`. Do not call it from app startup yet, and do not move existing `EnsureSchema` steps.

- [ ] **Step 4: Run GREEN**

Run the same Go tests.

### Task 5: Final Verification and Evidence

- [ ] **Step 1: Run targeted frontend and miniapp tests**

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/api/view-context.test.js src/lib/frontend-architecture.test.js src/lib/costing-price-list-workflow.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/product-settings.test.js
cd ../../../miniapp
npm test
```

- [ ] **Step 2: Run backend and broad checks**

```bash
cd orderapp-remote
go test ./internal/architecture ./internal/infrastructure/postgres -count=1
go test ./...
npm run build --prefix frontend-vue-shell
cd ../miniapp && npm run typecheck && npm run build:mp-weixin
cd ..
scripts/verify_kferp.sh changed
git diff --check
```

- [ ] **Step 3: Report evidence**

Final report must include PR-477/DEV entries, changed files, RED/GREEN evidence, build outputs, and remaining debt. No deployment unless Van explicitly requests it.
