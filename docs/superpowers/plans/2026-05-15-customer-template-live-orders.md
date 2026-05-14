# Customer Template Live Orders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make customer fulfillment accounts use ERP-style order lists and live capability templates with manual copy, rename, inactivation, indentation, and folding.

**Architecture:** Backend template records become active/inactive tree nodes with optional parent keys. Runtime capability resolution reads the referenced active template instead of stale copied customer capabilities. The customer-side ERP fulfillment portal reuses the existing order list API and sales/delivery document drawers.

**Tech Stack:** Go, Echo, pgx/Postgres, Vue 3/Vite, Node test runner.

---

### Task 1: Backend Live Template Model

**Files:**
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/admin_api.go`
- Test: `orderapp-remote/internal/application/customerportal/service_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] Write failing tests for `active=false`, parent template metadata, manual template copy, and live capability override.
- [ ] Verify the new tests fail because fields/endpoints/resolution do not exist or still read `customer_service_capabilities`.
- [ ] Add `ParentTemplateKey`, `Active`, `SortOrder`, and invalid-template error handling to the application model.
- [ ] Add schema columns and repository scan/save/list/copy/inactivate support.
- [ ] Change runtime customer capability/theme/entry-mode resolution to read the active referenced template first.
- [ ] Keep legacy capability rows as fallback only when no template is referenced.
- [ ] Re-run targeted Go tests and keep them green.

### Task 2: Customer-Side ERP Portal Order List Parity

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerProcessingPortalView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.js`
- Test: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.test.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.test.js`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_277_customer_template_live_orders_test.go`

- [ ] Write failing frontend/API/source tests for customer-side `fetchCustomerFulfillmentOrders`, `履约客户订单`, `订单费用`, `SalesOrderView`, and `DeliveryNoteView` markers in the customer portal.
- [ ] Verify the tests fail because the customer portal still renders the old `overview.direct_ship_orders` table.
- [ ] Add current customer scoped order fetch and detail loading to `CustomerProcessingPortalView.vue`.
- [ ] Reuse order fee helper, sales order drawer, and delivery note drawer.
- [ ] Put the customer-side order table at the bottom and align it with the ERP workbench fields.
- [ ] Re-run targeted frontend tests and source guard.

### Task 3: Template Tree UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerCapabilityTemplatesView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/customer-portal-theme.test.js`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_277_customer_template_live_orders_test.go`

- [ ] Write failing tests/source guards for `复制模板`, `模板失效`, `parent_template_key`, `collapsedTemplateKeys`, and active-template selection.
- [ ] Verify tests fail against the current flat template editor.
- [ ] Add tree rendering with root templates and indented child templates.
- [ ] Add collapse toggles per root template.
- [ ] Add copy, rename via label edit, save, and inactive toggle actions.
- [ ] Filter customer configuration dropdown to active templates while preserving invalid references for correction.
- [ ] Re-run targeted frontend/source tests.

### Task 4: Documentation, Seeds, Verification, Deploy

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`

- [ ] Add PR/DEV/UT/API/REV rows for live templates and customer-side order list parity.
- [ ] Update manuals for live template references, manual copy, invalid-template correction, tree folding, and bottom order list.
- [ ] Run targeted Go and Node tests.
- [ ] Run broader related Go suites, full frontend `node --test`, and `npm run build`.
- [ ] Merge latest `origin/develop` into the feature branch, rerun checks, push feature branch, merge into `develop`, and deploy the development stack.
- [ ] Smoke test `/app/`, current 岩师傅 capabilities, and customer portal order/static markers.
