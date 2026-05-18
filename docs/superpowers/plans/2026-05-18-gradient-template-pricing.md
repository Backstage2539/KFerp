# Gradient Template Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add reusable gradient pricing templates bound to product secondary categories, with explainable bean-list price sources.

**Architecture:** Store gradient templates and tiers in Postgres, expose them through catalog/product-settings APIs, and apply them inside the existing costing domain before bean-list preview and price publish. The Vue product settings page owns template management and category binding; the costing page owns price explanation and temporary what-if calculation.

**Tech Stack:** Go, Echo, pgx/Postgres, Vue 3 + Vite, existing support requirement table seeds, Markdown operation manuals.

---

### Task 1: Backend Domain Tests

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine_test.go`
- Modify: `orderapp-remote/internal/domain/costing/engine.go`

- [ ] Write failing tests for template-driven commercial tiers:
  - a product with a template tier named `大客户量单` matches by `min_weight_g/max_weight_g`, not by label.
  - template display unit `kg` produces kg prices and `spec_g=1000`.
  - template display unit `lb` produces lb prices and `spec_g=454`.
  - explanation output contains product cost, fast cost parameters, template margin, conversion, and final price.
- [ ] Run `cd orderapp-remote && go test ./internal/domain/costing -run 'TestGradient|TestPriceExplanation'`.
- [ ] Implement minimal costing domain types and functions:
  - `GradientTemplate`
  - `GradientTemplateTier`
  - `PriceExplanation`
  - template-aware tier building.
- [ ] Re-run the domain tests and keep existing costing domain tests green.

### Task 2: Backend Repository And API Tests

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`
- Modify: `orderapp-remote/internal/application/costing/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`

- [ ] Write failing API tests for:
  - `GET /api/product-settings` returns `gradient_templates` and category `gradient_template_id`.
  - creating/updating/deactivating a gradient template.
  - binding a secondary category to a template.
  - `POST /api/costing/price-explanation` returns formula steps and temporary recalculation without saving settings.
- [ ] Run targeted API tests and confirm RED.
- [ ] Add schema tables and category column.
- [ ] Add catalog repository/service methods for template CRUD and category binding.
- [ ] Add costing explanation service and API route.
- [ ] Run targeted API tests and confirm GREEN.

### Task 3: Costing Integration Tests And Implementation

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/application/costing/service_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`

- [ ] Write failing tests proving `BeanList` applies category-bound templates for product inputs and leaves unbound categories on existing defaults.
- [ ] Run targeted tests and confirm RED.
- [ ] Extend product input loading with category template metadata.
- [ ] Apply template rules during `ValidateProductInput` and `CalculateProduct`.
- [ ] Preserve publish semantics: transaction prices update only through existing publish run flow.
- [ ] Run targeted tests and confirm GREEN.

### Task 4: Frontend Tests And UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Add: `orderapp-remote/frontend-vue-shell/src/lib/gradient-templates.js`
- Add: `orderapp-remote/frontend-vue-shell/src/lib/gradient-templates.test.js`

- [ ] Write failing JS tests for template normalization, weight interval validation, and price explanation temporary update helpers.
- [ ] Run `cd orderapp-remote/frontend-vue-shell && npm test -- gradient-templates`.
- [ ] Add gradient template management to product settings.
- [ ] Add template selector to secondary categories.
- [ ] Add source buttons and explanation drawer to costing/bean-list prices.
- [ ] Run frontend tests and build.

### Task 5: Documentation And Requirement Seeds

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify: `OP_MANUAL_INVENTORY_MATERIALS.md`
- Modify: `OPERATION_MANUALS.md`
- Modify matching files under `orderapp-remote/docs/`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Add: `docs/acceptance/2026-05-18-gradient-template-pricing.md`

- [ ] Add PR/DEV rows for the feature.
- [ ] Update operation manuals with entry path, roles, steps, result checks, and common failures.
- [ ] Add acceptance checklist with concrete verification commands and evidence fields.
- [ ] Run support/manual tests.

### Task 6: Full Verification, Merge, Deploy

**Files:**
- No new production code files.

- [ ] Run Go targeted tests.
- [ ] Run `cd orderapp-remote && go test ./...`.
- [ ] Run `cd orderapp-remote/frontend-vue-shell && npm test`.
- [ ] Run `cd orderapp-remote/frontend-vue-shell && npm run build`.
- [ ] Push `codex/gradient-template-pricing-20260518`.
- [ ] Fetch latest `origin/develop`, merge/rebase into feature branch, rerun relevant checks.
- [ ] Merge feature branch into local `develop`, push `develop`.
- [ ] Verify `origin/develop` hash and deploy development with `./deploy_orderapp.sh development`.
- [ ] Smoke test development stack and record evidence.
