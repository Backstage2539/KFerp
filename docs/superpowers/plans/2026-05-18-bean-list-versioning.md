# Bean List Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bean list version management across publishing, customer portal display, miniapp cache, and order entry.

**Architecture:** Keep `bean_list_publications` as the immutable publication/version source. Add small tables for customer display pointers, generated assets, and customer acknowledgements. Resolve an effective publication through customer fixed version, customer latest, then official latest; order entry and miniapp both consume the same resolver.

**Tech Stack:** Go, PostgreSQL/pgx, Echo API tests, Vue/Vite frontend, uni-app miniapp TypeScript tests.

---

### Task 1: Backend Schema And Domain Helpers

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/schema.go`
- Modify: `orderapp-remote/internal/application/customerportal/service.go`
- Test: `orderapp-remote/internal/application/customerportal/service_test.go`

- [ ] Write failing tests for bean list diff and acknowledgement metadata.
- [ ] Add customer profile display mode columns, publication asset table, acknowledgement table, and order publication columns.
- [ ] Add `BeanListDiff`, `BeanListAck`, and effective publication metadata types.
- [ ] Run targeted Go tests and confirm the new tests pass.

### Task 2: Customer Portal Effective Version API

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/business_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerportal/admin_repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/customerportal/repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] Write failing repository/API tests for fixed customer version, latest customer fallback, official fallback, and acknowledgement.
- [ ] Implement effective bean list resolver and version option loading.
- [ ] Add miniapp acknowledgement endpoint.
- [ ] Add audit logs for customer display mode save and acknowledgement.

### Task 3: Order Entry Version Binding

**Files:**
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/order_form_queries.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/order_api.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/order_api_test.go`

- [ ] Write failing API tests that order form returns the customer's effective bean list and order save stores the selected publication/version.
- [ ] Add request/response fields for `bean_list_publication_id`.
- [ ] Persist order publication fields and include them in edit data.
- [ ] Keep existing product price tier behavior as fallback while binding orders to the selected publication snapshot.

### Task 4: Server-Side Asset Cache

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/customerportal/mini_api.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/costing/repository_test.go`
- Test: `orderapp-remote/internal/interfaces/http/customerportal/mini_api_test.go`

- [ ] Write failing tests proving a bean list PDF payload is generated once per `publication_id + asset_type`.
- [ ] Store PDF bytes in `bean_list_publication_assets.payload`.
- [ ] Serve cached PDF bytes on subsequent downloads.
- [ ] Add audit log when the cache row is first created.

### Task 5: ERP Vue UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CustomerPortalSettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/OrderEntryView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.js`
- Test: `orderapp-remote/frontend-vue-shell/src/lib/order-entry.test.js`

- [ ] Write failing unit tests for default version selection and unacknowledged update prompt state.
- [ ] Add customer bean list version selector in customer portal settings.
- [ ] Add order-entry version selector and update-confirmation dialog.
- [ ] Expose the bean list publication version list directly in 产品豆单, with filters for official/mine/customer and commercial/retail/green versions.
- [ ] Ensure customer without owned bean list sees no fixed-version selector.

### Task 6: Miniapp Cache And Prompt

**Files:**
- Modify: `miniapp/src/api/customerPortal.ts`
- Modify: `miniapp/src/pages/service/service.vue`
- Modify: `miniapp/src/utils/beanListPageCache.ts`
- Test: `miniapp/src/utils/beanListPageCache.test.ts`
- Test: `miniapp/src/api/customerPortal.test.ts`

- [ ] Write failing tests for per-version local cache and acknowledgement endpoint path.
- [ ] Keep local content when `cache_key` is unchanged.
- [ ] Show version update prompt before first order submission for an unacknowledged version.
- [ ] Call acknowledgement API after the customer confirms.

### Task 7: Requirements, Manuals, Acceptance

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_COSTING.md`
- Modify: `OP_MANUAL_CUSTOMER_PORTAL.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: mirrored docs under `orderapp-remote/docs/`
- Create: `docs/acceptance/2026-05-18-bean-list-versioning.md`

- [ ] Add PR/DEV rows for this requirement.
- [ ] Update manuals with version selection, cache, prompt, and troubleshooting.
- [ ] Add acceptance checklist and evidence placeholders.
- [ ] Run full targeted frontend/backend verification.
