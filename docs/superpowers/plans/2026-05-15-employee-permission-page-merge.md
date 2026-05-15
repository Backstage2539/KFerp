# Employee Permission Page Merge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Merge employee maintenance and user permission management into one internal-user page.

**Architecture:** Keep the existing backend APIs and permission checks. Move the user-permission UI controls into `CompanyStaffView.vue`, remove the standalone menu/config entry, and keep `view=userPermissions` as a legacy alias to `employees`.

**Tech Stack:** Go HTTP/authz tests, Vue 3 + Vite shell, Node test runner, Markdown manuals.

---

### Task 1: Add Guard Tests

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/support/dev_284_employee_permission_page_merge_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.test.js`
- Modify: `orderapp-remote/internal/infrastructure/postgres/authz/schema_test.go`

- [ ] Add a Go support test that reads `CompanyStaffView.vue`, `UserPermissionsView.vue`, `App.vue`, and `menu-ia.js`.
- [ ] Assert employee maintenance contains `fetchInternalAuthAccounts`, `resetEmployeePassword`, `saveEmployeeRoles`, `内部权限`, and no external-account strings.
- [ ] Assert App no longer imports `UserPermissionsView.vue` and menu does not expose a `用户权限` item.
- [ ] Run the new test and confirm it fails before implementation.

### Task 2: Merge Vue UI

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/CompanyStaffView.vue`
- Delete or stop using: `orderapp-remote/frontend-vue-shell/src/views/UserPermissionsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`

- [ ] Import auth helpers into `CompanyStaffView.vue`.
- [ ] In employee mode, load employees, roles, assignments, and internal account state together.
- [ ] Add account controls and role checkboxes to the employee table.
- [ ] Remove the visible `userPermissions` menu item.
- [ ] Normalize `view=userPermissions` to `employees`.
- [ ] Run the guard tests and targeted Node tests.

### Task 3: Merge View Config

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/authz/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/authz/schema_test.go`

- [ ] Remove `userPermissions` from `defaultViewPermissions()`.
- [ ] Update schema tests so `employees` remains covered and `userPermissions` is absent.
- [ ] Run `go test ./internal/infrastructure/postgres/authz -count=1`.

### Task 4: Docs And Requirement Records

**Files:**
- Modify: `docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `docs/OP_MANUAL_SETTINGS_AUDIT.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_SETTINGS_AUDIT.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `docs/acceptance/2026-05-15-employee-permission-page-merge.md`
- Create: `orderapp-remote/docs/acceptance/2026-05-15-employee-permission-page-merge.md`

- [ ] Update manuals to direct operators to “系统 / 员工维护” for employee, account, password, and role work.
- [ ] Add PR/DEV records for the merged page.
- [ ] Add acceptance evidence after verification.

### Task 5: Verify, Merge, Deploy

**Commands:**
- `go test ./internal/interfaces/http/support -run TestEmployeePermissionPageMerge -count=1`
- `go test ./internal/infrastructure/postgres/authz -count=1`
- `node --test src/lib/menu-ia.test.js src/api/auth.test.js`
- `npm run build`
- `go test ./... -count=1`
- Push feature branch, merge into `develop`, rerun checks, deploy development stack.
