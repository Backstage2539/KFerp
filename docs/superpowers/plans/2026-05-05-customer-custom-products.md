# Customer Custom Products Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add customer-specific coffee SKUs that can reuse shared raw materials while having independent BOM, roast level, finished inventory, and cost identity.

**Architecture:** Extend `products` with ownership metadata and keep the existing `product_id`-centered production/inventory/costing model. Add a product settings API for creating customer SKUs from base products, and filter order-entry products by selected customer.

**Tech Stack:** Go/Echo/Postgres schema bootstrap, Vue 3 + Vite shell, existing `api/client` helpers, existing Go and Node test suites.

---

### Task 1: Product Ownership Data Model

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/core/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/schema.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/options.go`

- [ ] Write API/unit tests proving product settings exposes `customer_id`, `base_product_id`, `visibility`, and `custom_type`.
- [ ] Run the targeted test and confirm it fails because fields are missing.
- [ ] Add product columns and indexes through schema bootstrap.
- [ ] Thread ownership fields through catalog application and HTTP DTOs.
- [ ] Re-run targeted tests.

### Task 2: Create Customer SKU API

**Files:**
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/catalog/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`

- [ ] Write an API test for `POST /api/product-settings/custom-products`.
- [ ] Confirm the test fails because the route does not exist.
- [ ] Add repository command to create a customer SKU from a base product.
- [ ] Copy BOM, price tiers, category assignment, and selected roast/yield data when requested.
- [ ] Add HTTP handler validation.
- [ ] Re-run targeted tests.

### Task 3: Sales Order Product Filtering

**Files:**
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] Write an API test that `/api/order/form?customer_id=3` returns public products and customer 3 SKUs, excluding other customer SKUs.
- [ ] Confirm the test fails because order form does not filter by customer ownership.
- [ ] Thread customer ownership fields through sales product options.
- [ ] Filter API query when `customer_id` is provided.
- [ ] Add Vue dropdown filtering by selected customer for interactive order entry.
- [ ] Re-run targeted Go and Node tests.

### Task 4: Product Settings UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`
- Add/Modify test: `orderapp-remote/frontend-vue-shell/src/lib/costing-settings.test.js` is not used; add focused source guards under Go support tests if Vue behavior is source-guarded.

- [ ] Add a compact “客户专属 SKU” form to Product Settings.
- [ ] Load `customers` and base `products` from `/api/product-settings`.
- [ ] Submit to `/api/product-settings/custom-products`.
- [ ] Show customer/type markers in the product table.
- [ ] Add source guard/API tests for wiring if no component test harness exists.

### Task 5: Workflow Records and Verification

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Modify: `memory/2026-05-05.md`

- [ ] Add PR/DEV/UT/API/REV rows for the customer custom product requirement.
- [ ] Run focused Go tests for catalog and sales APIs.
- [ ] Run focused Vue shell tests for order-entry logic.
- [ ] Run broader `go test ./... -count=1` if focused tests pass.
- [ ] Run Vue shell test/build commands.
- [ ] Review acceptance criteria against this spec and record evidence.
