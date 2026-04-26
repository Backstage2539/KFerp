# DDD Module Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the temporary `internal/appmain` package into DDD-oriented business modules so the root application package is only a composition root.

**Architecture:** `internal/appmain` owns process startup, schema-step composition, route composition, and dependency wiring only. Existing handlers and adapters are grouped into bounded HTTP modules under `internal/interfaces/http/<module>` while existing business use cases remain under `internal/application/<module>` and pure rules remain under `internal/domain/<module>`. Module-level `RegisterRoutes` and `EnsureSchema` entrypoints are the migration boundary for later finer-grained postgres adapter extraction.

**Tech Stack:** Go 1.22, Echo, pgx, PostgreSQL, Vue/Vite, existing requirement workflow tables.

---

### Task 1: Add Architecture Guardrails And DEV Requirement Seeds

**Files:**
- Modify: `orderapp-remote/internal/architecture/root_package_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`

- [x] Add architecture tests that fail while `internal/appmain` contains business modules.
- [x] Add tests that require HTTP module directories plus existing application/domain boundaries.
- [x] Add requirement seed rows `DEV-DDD-*`, `UT-DDD-*`, `API-DDD-*`, and `REV-DDD-*`.
- [x] Run `go test ./internal/architecture -run TestAppmain -count=1` and verify failure before migration.

### Task 2: Extract Shared HTTP And Support Modules

**Files:**
- Create: `internal/interfaces/http/support`
- Modify: `internal/appmain/app_routes.go`
- Modify: `internal/appmain/schema_setup.go`

- [x] Move request helpers, template rendering, auth context, operation logs, docs, static routes, and requirement workflow into the support module.
- [x] Move docs and static Vue shell routes out of `appmain`.
- [x] Move requirement table APIs/pages/store/migrations into support module.
- [x] Keep existing Vue requirement API contracts unchanged.

### Task 3: Extract Company, Catalog, Materials, BOM, And Customer Modules

**Files:**
- Create: `internal/interfaces/http/company`
- Create: `internal/interfaces/http/production`
- Create: `internal/interfaces/http/customer`
- Modify: `internal/appmain/app_routes.go`
- Modify: `internal/appmain/schema_setup.go`

- [x] Move company department/employee HTTP and postgres code behind company application service.
- [x] Move catalog/product/BOM/materials route and repository adapters into the production/catalog operations module.
- [x] Move customer HTTP and postgres adapters out of `appmain`.
- [x] Preserve existing JSON route paths.

### Task 4: Extract Production, Inventory, Sales, Shipping, Audit, And Settings Modules

**Files:**
- Create: `internal/interfaces/http/production`
- Create: `internal/interfaces/http/sales`
- Create: `internal/interfaces/http/support`
- Modify: `internal/appmain/app_routes.go`
- Modify: `internal/appmain/schema_setup.go`

- [x] Move production flow, batch, logs, allocation, material consumption, and stock ledger code out of `appmain`.
- [x] Move sales order routes, APIs, repositories, pricing adapters, and shipping export code out of `appmain`.
- [x] Move audit middleware/pages and sender/outsource settings out of `appmain`.
- [x] Preserve existing production and order API contracts.

### Task 5: Verify, Merge, And Deploy

**Files:**
- Modify only files required by verification failures.

- [x] Run `go test ./... -count=1`.
- [x] Run `npm run build` in `orderapp-remote/frontend-vue-shell`.
- [x] Push `codex/ddd-module-split-20260426`.
- [x] Fast-forward merge into `develop`.
- [x] Deploy from `develop`.
- [x] Smoke test Vue and API routes after deployment.
