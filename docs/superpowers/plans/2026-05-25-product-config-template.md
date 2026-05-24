# Product Config Template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the visible customer rule-template/customer override workflow with one reusable 商品配置模板 model that product subtypes bind to and customer subtypes can copy.

**Architecture:** Add `product_config_templates` as the canonical template table. A product subtype stores `product_config_template_id`; binding or editing a template materializes its fields back to existing subtype columns (`gradient_template_id`, `operation_template_id`, unit fields, price rule JSON) so current product price list, order entry, and production SQL keep working. Legacy customer product rule tables remain for backward compatibility but are hidden from the main UI.

**Tech Stack:** Go 1.22 backend, PostgreSQL schema migrations in Go strings, Vue 3 + Vite frontend, Node `node:test`, existing PR/DEV/UT/API/REV seed store.

---

### Task 1: Product Config Template Backend

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Test: `orderapp-remote/internal/application/catalog/service_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/catalog/repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`

- [ ] Write failing tests proving `/api/product-settings` exposes `product_config_templates`, saving a template calls the catalog service, and deriving a customer template carries `source_template_id`.
- [ ] Write failing repository/schema tests looking for `product_config_templates`, `product_config_template_id`, category selects, save/upsert methods, derive helper, and materialization back into product category fields.
- [ ] Implement `ProductConfigTemplate`, save/derive commands, repository interface methods, service validation, schema table/columns/indexes, list/save/derive repository methods, and HTTP routes.
- [ ] Run targeted Go tests and confirm they pass:
  `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -run 'ProductConfig|ProductSettingsAPI' -count=1`

### Task 2: Subtype Binding And Customer Copy Behavior

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`

- [ ] Write failing frontend tests proving customer category tree does not show public SKU when only public category is enabled, and public subtype copy keeps customer context while cloning/binding a customer 商品配置.
- [ ] Write failing backend tests proving deriving a public category copies the bound product config template for that customer and assigns the derived template ID to the derived category.
- [ ] Implement `product_config_template_id` in category payload/decorator/builders and derive category materialization.
- [ ] Fix `usePublicSkuInCategoryTree` to follow the public SKU switch, not the public category switch.
- [ ] Run targeted tests:
  `node --test src/lib/product-settings.test.js`
  `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog -run 'ProductConfig|DeriveProductCategory|AssignProductCategory' -count=1`

### Task 3: SKU Settings UI Simplification

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`

- [ ] Write failing source-level tests requiring the UI to show “商品配置”, “复制为客户配置”, “更换商品配置”, and no longer show “客户产品规则”, “客户规则模板”, “客户专属覆盖”, or “纳入产品价格表”.
- [ ] Replace the visible customer rule panel with a 商品配置 panel. Keep structured controls for階梯价模板、工序模板、价格表生成规则、单位换算、整数规则; remove the “纳入产品价格表” checkbox.
- [ ] Change product subtype inline config to bind/select 商品配置模板 and show a short summary; do not expose raw subtype unit/price fields there.
- [ ] Run Vue lib tests and Vite build:
  `node --test src/lib/product-settings.test.js`
  `npm run build`

### Task 4: Docs, Requirement Seeds, And Acceptance

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/docs/acceptance/2026-05-25-product-config-template.md`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_362_product_config_template_test.go`

- [ ] Add PR/DEV/UT/API/REV rows for `PR-362-PRODUCT-CONFIG-TEMPLATE`.
- [ ] Update user manuals to describe 商品配置模板, subtype binding, copy behavior, and price-list inclusion being controlled by the 产品价格表 page.
- [ ] Add acceptance evidence for the customer copy scenario and the hidden legacy rule UI.
- [ ] Run support tests:
  `go test ./internal/interfaces/http/support -run 'TestDev36(2|1|0)' -count=1`

### Task 5: Full Verification And Deploy

**Files:**
- No code changes unless verification exposes a defect.

- [ ] Run full local verification:
  `cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js src/api/*.test.js && npm run build`
  `cd orderapp-remote && go test ./... -count=1`
  `git diff --check`
- [ ] Commit and push branch `codex/product-config-template-20260525`.
- [ ] Merge latest `origin/develop`, rerun targeted frontend/backend checks, then fast-forward merge into `develop` and push.
- [ ] Deploy development with the KFerp deployment workflow and verify `/app/vue-shell/?view=productSettings`, `/app/api/product-settings`, REQ API, and deployed JS markers.

---

Self-review:
- Covers the three confirmed requirements: 商品配置模板 unification, removing price-list inclusion checkbox, and fixing public SKU leakage after customer category copy.
- No destructive migration: legacy customer rule tables stay available for old data; UI no longer exposes them as the primary model.
- Existing price list/order/production behavior is protected by materializing template fields to current subtype columns.
