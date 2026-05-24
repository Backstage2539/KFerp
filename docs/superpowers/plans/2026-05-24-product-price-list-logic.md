# Product Price List Logic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn KFerp's hard-coded bean-list/product-kind model into a configurable product type/subtype model while reusing the existing product bean-list publication machinery as the generalized product price list.

**Architecture:** Keep existing tables and routes compatible first, then add generalized columns and resolver helpers beside them. Product type is the level-1 `product_categories` node, product subtype is the level-2 node, and `product_kind` remains as a legacy compatibility field until all pricing, order-entry, and production paths can derive behavior from category configuration. Existing `bean_list_publications` stays the source of published price snapshots and gains product-type category semantics before any table rename.

**Tech Stack:** Go application/domain/infrastructure packages, PostgreSQL schema migrations embedded in Go, Vue/Vite shell, Node built-in test runner, existing operation manuals and support PR/DEV seed tests.

---

## Scope Guard

This is a multi-stage migration. Each stage must leave develop deployable and preserve current熟豆、生豆、挂耳、速溶咖啡 behavior. Do not remove `product_kind` or `bean_list_publications.list_type` until the compatibility tests prove all old orders, old publications, and current order-entry flows still work.

## File Map

- `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`: category configuration columns, seed/migration from legacy product kind.
- `orderapp-remote/internal/application/catalog/service.go`: product type/subtype DTOs and config commands.
- `orderapp-remote/internal/domain/catalog/product_kind.go`: legacy mapping helpers only; new code should use category resolver helpers.
- `orderapp-remote/internal/domain/catalog/product_category_config.go`: new pure helpers for resolving product type/subtype, unit rules, and config priority.
- `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`: API coverage for product type/subtype labels, config persistence, and legacy compatibility.
- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`: UI labels and payload helpers.
- `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`: frontend behavior tests.
- `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`: `bean_list_publications` generalized product type association.
- `orderapp-remote/internal/application/costing/service.go`: product price list DTO aliases beside legacy bean-list DTO names.
- `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`: rename page and controls from 产品豆单/豆单 to 产品价格表 while keeping existing routes.
- `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`: title/subtitle aliases for product price list.
- `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`: load published price lists by product type and keep legacy list types.
- `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`: version selector generalized by product type, with legacy selectors preserved.
- `orderapp-remote/internal/infrastructure/postgres/production/plan_queries.go`: no-BOM/default-material and process route resolution.
- `orderapp-remote/internal/interfaces/http/support/req_store.go`: PR/DEV seeds for each phase.
- Manuals: `OP_MANUAL_COSTING.md`, `OP_MANUAL_ORDER_SALES.md`, `OP_MANUAL_PRODUCTION.md`, `OP_MANUAL_INVENTORY_MATERIALS.md` and deployed copies under `orderapp-remote/docs/`.

## Phase 1: Model Compatibility And Naming Upgrade

**Target PR:** `PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT`

### Task 1.1: Support PR/DEV seed and docs evidence

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_354_product_type_subtype_compat_test.go`
- Modify: `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/docs/REQUIREMENTS.md`, `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-24-product-type-subtype-compat.md`

- [ ] Write failing support test requiring `PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT`, `DEV-354-PRODUCT-TYPE-SUBTYPE-COMPAT`, and operation manual references.
- [ ] Run: `go test ./internal/interfaces/http/support -run TestDev354 -count=1`. Expected: FAIL because seeds/docs are absent.
- [ ] Add seed rows and docs entries.
- [ ] Re-run the same command. Expected: PASS.

### Task 1.2: Rename UI concepts without changing storage

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`

- [ ] Write failing frontend test that source labels contain `产品类型` and `产品子类型`, and no user-facing SKU classification label says `一级分类` or `二级分类`.
- [ ] Run: `node --test src/lib/product-settings.test.js`. Expected: FAIL on missing labels.
- [ ] Replace user-facing category labels only; do not rename DB fields yet.
- [ ] Re-run: `node --test src/lib/product-settings.test.js`. Expected: PASS.

### Task 1.3: Add product type/subtype resolver helpers

**Files:**
- Create: `orderapp-remote/internal/domain/catalog/product_category_config.go`
- Test: `orderapp-remote/internal/domain/catalog/product_category_config_test.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`

- [ ] Write failing tests for `ProductTypeLabel`, `ProductSubtypeLabel`, and `LegacyKindDefaultTypeName` mapping: `roasted/roasted_bean -> 熟豆`, `green_bean -> 生豆`, `drip_bag -> 挂耳`, `instant_coffee -> 速溶咖啡`.
- [ ] Run: `go test ./internal/domain/catalog -run 'TestProductCategory|TestLegacyKind' -count=1`. Expected: FAIL because helpers do not exist.
- [ ] Implement pure helper functions without changing existing product kind behavior.
- [ ] Re-run targeted test. Expected: PASS.

## Phase 2: Product Subtype Config And Unit Rules

**Target PR:** `PR-355-PRODUCT-SUBTYPE-CONFIG-UNIT-RULES`

### Task 2.1: Schema and DTOs

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`

- [ ] Add failing repository/API tests proving a product subtype can persist: `gradient_template_id`, `operation_template_id`, `price_list_rule_json`, `inventory_unit`, `quote_unit`, `order_unit`, `unit_conversion_json`, `integer_unit`.
- [ ] Run catalog package tests. Expected: FAIL on missing columns/fields.
- [ ] Add nullable/defaulted columns to `product_categories`; expose them in DTO/API payloads.
- [ ] Re-run catalog tests. Expected: PASS.

### Task 2.2: SKU override fields

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`
- Tests: catalog API and frontend product-settings tests.

- [ ] Add failing tests for SKU override of subtype config: price template, operation template, and unit rule.
- [ ] Implement columns and payload handling.
- [ ] Verify targeted tests pass.

## Phase 3: Customer Product Rule Templates And Overrides

**Target PR:** `PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES`

### Task 3.1: Rule template schema and resolver

**Files:**
- Create: `orderapp-remote/internal/domain/catalog/product_rule_resolution.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/application/customer/service.go`
- Tests: domain resolver tests, customer/catalog API tests.

- [ ] Write failing resolver tests for priority: customer override > customer product rule template > product subtype default > product type default > system fallback.
- [ ] Add tables `customer_product_rule_templates`, `customer_product_rule_template_items`, `customer_product_rule_overrides`, and customer binding column.
- [ ] Implement resolver and API read/write endpoints.
- [ ] Verify tests pass.

## Phase 4: Product Price List Generalization

**Target PR:** `PR-357-PRODUCT-PRICE-LIST-GENERALIZATION`

### Task 4.1: Generalize publications while preserving bean-list compatibility

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
- Modify: `orderapp-remote/internal/application/costing/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Tests: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`

- [ ] Add failing tests proving publications can be queried by `product_type_category_id` and legacy `list_type` still returns the same rows.
- [ ] Add `product_type_category_id`, `product_type_name`, and compatibility mapping from `commercial/retail -> 熟豆`, `green -> 生豆`, `drip -> 挂耳`.
- [ ] Keep old public URLs and download routes working.
- [ ] Verify costing tests pass.

### Task 4.2: Rename page semantics

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`
- Modify: `OP_MANUAL_COSTING.md` and `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- Tests: Vue source tests and manual support tests.

- [ ] Add failing tests requiring user-facing page label `产品价格表` and no primary action label `发布豆单`.
- [ ] Replace labels while preserving route/API names.
- [ ] Verify frontend tests pass.

## Phase 5: Order Entry And Production Integration

**Target PR:** `PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION`

### Task 5.1: Order form payload

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Tests: sales API and frontend order-entry tests.

- [ ] Add failing tests proving order form returns product-price-list options by product type, while old commercial/green/drip selectors remain populated.
- [ ] Implement generalized payload fields beside legacy fields.
- [ ] Verify targeted tests pass.

### Task 5.2: Production resolver

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/plan_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/production/unprod_summary.go`
- Tests: `orderapp-remote/internal/infrastructure/postgres/production/plan_queries_test.go`

- [ ] Add failing tests proving no-BOM products use product subtype default material and operation template, including 速溶咖啡.
- [ ] Implement resolver calls with legacy instant-coffee fallback preserved.
- [ ] Verify production tests pass.

## Phase 6: Migration And Product Kind Cleanup

**Target PR:** `PR-359-PRODUCT-KIND-MIGRATION-CLEANUP`

### Task 6.1: Migration guards

**Files:**
- Modify: catalog/costing schema files and repository tests.
- Modify: order-entry and production compatibility tests.

- [ ] Add failing tests proving old product_kind rows are mapped to product type/subtype records without changing historical order items or publication content.
- [ ] Implement idempotent migration statements.
- [ ] Verify full targeted backend/frontend suites.

### Task 6.2: Final docs and acceptance

**Files:**
- Modify all manuals listed in objective.
- Create final acceptance doc.

- [ ] Document how to add 速溶咖啡 product type, configure customer-specific tiers, publish product price list, place order, and enter production.
- [ ] Run support tests proving manuals and acceptance docs are wired.

## Final Verification And Integration

- [ ] Run targeted Go tests: `go test ./internal/domain/catalog ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`.
- [ ] Run frontend tests: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/product-settings.test.js src/lib/order-entry.test.js src/lib/bean-list-pdf.test.js`.
- [ ] Run Vite build: `cd orderapp-remote/frontend-vue-shell && npm run build`.
- [ ] Run full Go test if targeted suites pass: `cd orderapp-remote && go test ./... -count=1`.
- [ ] Fetch and merge latest `origin/develop` into feature branch.
- [ ] Re-run verification after merge.
- [ ] Push feature branch.
- [ ] Merge into develop, push develop, deploy development stack with `./deploy_orderapp.sh`.
- [ ] Smoke: Docker compose ps, unauth `/app/` 401, authenticated `/app/` 200/303-to-200, PR visible, product price list page reachable, order form returns generalized price-list data.
