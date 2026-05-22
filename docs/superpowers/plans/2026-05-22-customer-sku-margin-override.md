# Customer SKU Margin Override Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let customer-owned/customized SKU rows use the existing product-level `margin_rate_override` field.

**Architecture:** Reuse the existing product master field, PUT `/api/products/:id`, costing repository load, and costing engine behavior. The change is mainly frontend visibility plus requirement/manual updates: editable customer-owned rows can save the field; public reference rows in a customer context remain read-only through `canEditSkuRow`.

**Tech Stack:** Go/Echo API tests, Vue 3/Vite frontend, Node `node:test`, Markdown operation manuals.

---

### Task 1: RED Tests

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_settings_api_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_292_product_margin_override_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js`

- [ ] Add an API test proving a customer-owned SKU returns and saves `margin_rate_override`.
- [ ] Add a support test proving the margin column is not gated by `!selectedCustomerSkuCustomerID`.
- [ ] Add a frontend helper test proving `buildProductBasicsPayload` sends customer SKU margin override.
- [ ] Run targeted tests and confirm the new support test fails before implementation.

### Task 2: Implementation

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ProductSettingsView.vue`

- [ ] Always render the “利润率覆盖” header and cell.
- [ ] Disable the input for non-editable rows using existing `canEditSkuRow(row)`.
- [ ] Update table colspan values from customer/public conditional counts to the unified column count.
- [ ] Run targeted tests and confirm they pass.

### Task 3: Docs And Requirement Seeds

**Files:**
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`

- [ ] Update requirements and acceptance wording from public-only margin override to public plus customer-owned/customized SKU.
- [ ] Update the costing operation manual with the customer SKU workflow.
- [ ] Add PR/DEV/UT/API/REV rows for this requirement.
- [ ] Run support tests for requirement/manual evidence.

### Task 4: Verification

- [ ] Run `go test ./...`.
- [ ] Run `node --test src/lib/*.test.js src/api/*.test.js`.
- [ ] Run `npm run build`.
- [ ] Run `git diff --check`.
- [ ] Scan for conflict markers.
