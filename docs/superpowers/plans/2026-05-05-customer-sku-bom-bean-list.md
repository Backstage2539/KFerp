# Customer SKU BOM And Bean List Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make customer SKUs visible and maintainable, add whole-BOM delete, and enforce customer-scoped bean list product isolation.

**Architecture:** Keep existing Vue/Vite pages and Go application services. Add small API extensions and backend validation at persistence boundaries, then wire focused UI changes into Product Settings, BOM, and Costing.

**Tech Stack:** Go, Echo, pgx/Postgres, Vue 3, Vite, node:test.

---

### Task 1: Requirement Seeds And Source Guards

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_150_customer_custom_products_test.go`

- [ ] Add `PR-152`, `DEV-152-01..03`, `UT-152-01`, `API-152-01`, and `REV-152-01` covering BOM maintenance, customer SKU visibility, and customer bean list isolation.
- [ ] Add source guards requiring `bom_item_count`, customer SKU list UI, `DELETE /api/bom/:product_id`, `scope:"customer"`, and `customer_id` in bean list publishing.
- [ ] Run `go test ./internal/interfaces/http/support -run 'TestCustomerCustomProductsRequirementSeeds|TestCustomerSkuBomBeanListWiring' -count=1` and verify the new guard fails before implementation.

### Task 2: BOM Delete And BOM Row Creation

**Files:**
- Modify: `orderapp-remote/internal/application/bom/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/bom/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/bom/bom_api.go`
- Test: `orderapp-remote/internal/interfaces/http/bom/bom_customer_sku_test.go`

- [ ] Add failing tests for `DELETE /api/bom/:product_id` route registration and frontend delete wiring.
- [ ] Add `DeleteBom(ctx, productID)` through service and repository.
- [ ] Make `SaveItem` upsert `product_bom` before saving the item so "new BOM" works from the first material row.
- [ ] Add the API route and return `200 {"ok":true}`.
- [ ] Run `go test ./internal/interfaces/http/bom -count=1`.

### Task 3: Customer SKU Visibility In Product Settings

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/options.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/catalog_application_mapping.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`

- [ ] Add failing tests requiring `/api/product-settings` rows to include `bom_item_count`.
- [ ] Add `BomItemCount` through product query, domain, mapping, and JSON rows.
- [ ] Add computed customer SKU rows and a customer filter in Product Settings.
- [ ] Render customer SKU table with BOM count and "维护 BOM" action.
- [ ] Run catalog/support tests and `npm run build`.

### Task 4: Customer Bean List Scope

**Files:**
- Modify: `orderapp-remote/internal/domain/costing/engine.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/application/costing/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/costing/costing_api.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CostingView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js`
- Test: `orderapp-remote/internal/interfaces/http/costing/costing_api_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`

- [ ] Add failing tests for customer scope owner resolution and frontend product filtering.
- [ ] Carry product ownership metadata through costing results.
- [ ] Add customer selector and candidate filtering in Costing drawer.
- [ ] Store customer publications with `owner_type='customer'`, `owner_key='<customer_id>'`.
- [ ] Validate publication content product IDs against official/customer scope before saving.
- [ ] Run costing API/source tests, node tests, and Vue build.

### Task 5: Final Verification

- [ ] Run targeted unit/API tests:
  - `go test ./internal/interfaces/http/support -count=1`
  - `go test ./internal/interfaces/http/bom -count=1`
  - `go test ./internal/interfaces/http/catalog -count=1`
  - `go test ./internal/interfaces/http/costing -count=1`
  - `go test ./internal/infrastructure/postgres/costing -count=1`
  - `node --test src/lib/bean-list-pdf.test.js`
  - `npm run build`
- [ ] Run `git diff --check`.
- [ ] Start local Vue dev server if needed and provide the URL for browser review.
