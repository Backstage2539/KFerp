# Green Bean List Manual Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make green bean order pricing come only from selected green bean-list publication snapshots, with green bean-list prices generated from BOM cost snapshots and editable per template tier.

**Architecture:** Keep green bean products in the existing product/BOM/bean-list/order tables. Add BOM material cost snapshots, publish green bean-list content with tradeable tier data, and teach order save to resolve publication prices by product kind and selected publication version. Remove the bound-roasted-price fallback from form and save paths.

**Tech Stack:** Go, PostgreSQL/pgx, Vue 3, Vite, Node test runner.

---

### Task 1: BOM Cost Snapshots

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`

- [ ] Add failing static tests requiring `product_bom_items.unit_cost_snapshot` for green bean costing and forbidding `material_valuation` as the green bean cost source.
- [ ] Add `unit_cost_snapshot NUMERIC(12,4) NOT NULL DEFAULT 0` to `product_bom_items`, backfill existing zero snapshots from `materials.purchase_price`.
- [ ] When saving material BOM items, store the current material purchase price into `unit_cost_snapshot`.
- [ ] When copying/activating BOM rows, copy `unit_cost_snapshot`.
- [ ] Update costing input SQL to calculate green bean cost from BOM item snapshots through the bound roasted BOM.

### Task 2: Green Bean Template Prices Are Cost Defaults

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Test: `orderapp-remote/internal/application/costing/service_test.go`

- [ ] Add a failing service/domain test proving green bean template tiers default to BOM cost converted to the template unit and ignore template margin.
- [ ] Change `buildGreenBeanTemplateSaleTiers` to use cost-only default price.
- [ ] Keep template tier labels, ranges, display unit, and min/max quantities unchanged.

### Task 3: Bean-List Publication Content Carries Tradeable Tiers

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js`

- [ ] Add failing frontend tests proving green PDF items include `green_bean_sale_tiers` snapshots and manual green tier overrides are reflected in both `prices` and `green_bean_sale_tiers`.
- [ ] Extend PDF group building to include raw tier snapshots for trade pricing.
- [ ] Add green bean-list tier price inputs in the product picker when `listType === 'green'`.
- [ ] Store manual green tier prices in existing per-product customizers and apply them when building groups.

### Task 4: Order Pricing Uses Selected Publication Per Product Kind

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/orderbeans/usage.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Tests: `orderapp-remote/internal/infrastructure/postgres/orderbeans/usage_test.go`
- Tests: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Tests: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] Add failing tests proving order form no longer returns bound roasted tiers for green products.
- [ ] Add failing tests proving saving a green bean order with only bound roasted tiers fails with a missing green bean-list price error.
- [ ] Add failing tests proving a selected green bean-list publication supplies the green order price and is recorded on the order item.
- [ ] Add `list_type` to bean-list version options and expose per-customer latest public/customer versions for commercial, green, and drip.
- [ ] Add separate request fields for commercial, green, and drip bean-list publication IDs, while preserving the legacy `bean_list_publication_id`.
- [ ] Resolve published prices from the selected publication for green products; do not fall back to product or bound roasted tiers.
- [ ] Remove `greenBeanOrderPriceProductIDTx` and `greenBeanBoundRoastedTierPriceSourceJSON`.

### Task 5: Documentation, Requirement Tables, and Verification

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_GREEN_BEAN_SALES.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify mirrored docs under `orderapp-remote/docs/`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create acceptance evidence under both `docs/acceptance/` and `orderapp-remote/docs/acceptance/`

- [ ] Replace all “green bean falls back to bound roasted tier” text with “green bean must use green bean-list publication price”.
- [ ] Document BOM cost snapshot, manual green tier pricing, and separate order bean-list version selectors.
- [ ] Update PR/DEV seed evidence for PR-289.
- [ ] Run `node --test src/lib/*.test.js src/api/*.test.js`, `go test ./...`, `npm run build`, `git diff --check`.
- [ ] Merge into latest `origin/develop`, rerun checks, push, deploy development stack, and smoke test order form and green order pricing.
