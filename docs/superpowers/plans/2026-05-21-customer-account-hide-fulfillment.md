# Customer Account Hide Fulfillment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persisted admin setting that hides the internal fulfillment operator console in customer-account mode while leaving the page and APIs intact.

**Architecture:** Support HTTP exposes `/api/ui-settings` backed by `app_config`. Vue shell loads the setting and applies it through menu filtering. A Settings menu page lets administrators save the toggle.

**Tech Stack:** Go/Echo/pgx, Vue 3/Vite, Node test runner.

---

### Task 1: Tests First

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-permissions.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/api/auth.test.js`
- Create: `orderapp-remote/internal/interfaces/http/support/ui_settings_test.go`

- [ ] Write failing frontend tests for filtering `customerFulfillment` when `hideCustomerAccountFulfillment` is true and actor is a customer account.
- [ ] Write failing frontend API wrapper tests for `/api/ui-settings`.
- [ ] Write failing Go API tests for default setting, save behavior, permission rejection, and audit metadata.

### Task 2: Backend Setting API

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/support/ui_settings.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/module.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/authz_middleware.go`
- Modify: `orderapp-remote/internal/application/authz/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/authz/repository.go`

- [ ] Add `app_config` schema creation in support setup.
- [ ] Add `/api/ui-settings` GET/PUT routes.
- [ ] Require authentication for GET and `settings.write` for PUT.
- [ ] Add `account_type` to `/api/auth/me` so frontend can detect customer-account mode.
- [ ] Insert an audit record when the setting changes.

### Task 3: Frontend Integration

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/api/ui-settings.js`
- Create: `orderapp-remote/frontend-vue-shell/src/views/UISettingsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-permissions.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`

- [ ] Load UI settings in the shell with a default-hidden fallback.
- [ ] Add menu filtering options for customer-account mode.
- [ ] Add a Settings menu item and view for the toggle.

### Task 4: Documentation And Evidence

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_SETTINGS_AUDIT.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-21-customer-account-hide-fulfillment.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`

- [ ] Add PR/DEV/REV seeds.
- [ ] Update settings manual and acceptance checklist.
- [ ] Record verification evidence.
