# P1 Architecture Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the first behavior-preserving P1 architecture remediation pass for audit logging, HTTP support boundaries, and catalog/pricing/product boundaries.

**Architecture:** Add architecture tests first, then move shared contracts behind smaller files and adapters. Keep runtime route paths, JSON contracts, database tables, and user workflows unchanged.

**Tech Stack:** Go 1.x, Echo HTTP handlers, pgx/postgres adapters, existing KFerp architecture tests.

---

### Task 1: Audit Logging Single Source

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/audit_unified.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/audit_service_test.go`
- Modify: `orderapp-remote/internal/architecture/ddd_module_test.go`

- [ ] **Step 1: Write the failing architecture test**

Add a test that fails when `support/audit_unified.go` owns its own `AuditService.Insert` SQL implementation instead of delegating to `internal/infrastructure/postgres/audit.go`.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestAuditLogImplementationHasSingleSource -count=1
```

Expected: fail with a message naming the duplicate support audit implementation.

- [ ] **Step 3: Implement single source**

Keep support package call sites stable, but change `support.AuditMeta`, `support.AuditEntry`, `support.AuditService`, `support.NewAuditService`, `support.AuditInsert`, and `support.AuditInsertTx` to aliases/delegates of `internal/infrastructure/postgres`.

- [ ] **Step 4: Run GREEN**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestAuditLogImplementationHasSingleSource -count=1
go test ./internal/interfaces/http/support -run 'TestAudit|TestOperationLog' -count=1
```

Expected: pass.

### Task 2: HTTP Support Boundary Split

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/module.go`
- Create: `orderapp-remote/internal/interfaces/http/support/auth_module.go`
- Create: `orderapp-remote/internal/interfaces/http/support/req_module.go`
- Create: `orderapp-remote/internal/interfaces/http/support/view_context_module.go`
- Modify: `orderapp-remote/internal/architecture/ddd_module_test.go`

- [ ] **Step 1: Write the failing architecture test**

Add a test that fails while `support/module.go` directly calls all auth, REQ, view-context, docs, and UI-settings registration/schema functions.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestSupportModuleUsesFocusedSubmodules -count=1
```

Expected: fail because focused module functions do not exist yet.

- [ ] **Step 3: Extract focused submodule functions**

Move grouped calls into small support-package files:

- `registerAuthRoutes` / `ensureAuthSchema`
- `registerReqRoutes` / `ensureReqSchema`
- `registerViewContextRoutes` / `ensureViewContextSchema`

Keep exported `RegisterRoutes` and `EnsureSchema` signatures unchanged.

- [ ] **Step 4: Run GREEN**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestSupportModuleUsesFocusedSubmodules -count=1
go test ./internal/interfaces/http/support -count=1
```

Expected: pass.

### Task 3: Catalog/Price/Product Boundary Guard

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/catalog/product_routes.go`
- Create: `orderapp-remote/internal/interfaces/http/catalog/pricing_routes.go`
- Create: `orderapp-remote/internal/interfaces/http/catalog/classification_routes.go`
- Create: `orderapp-remote/internal/interfaces/http/catalog/business_group_routes.go`
- Create: `orderapp-remote/internal/application/catalog/repository_ports.go`
- Modify: `orderapp-remote/internal/application/catalog/service.go`
- Modify: `orderapp-remote/internal/architecture/ddd_module_test.go`

- [ ] **Step 1: Write the failing architecture test**

Add a test that fails while the catalog application repository port remains embedded inside the large service file and the catalog route registration has no pricing/classification/business-group subroute files.

- [ ] **Step 2: Run RED**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestCatalogBoundaryHasFocusedPortsAndRouteFiles -count=1
```

Expected: fail because focused files are missing.

- [ ] **Step 3: Extract repository port and route groups**

Move the `Repository` interface into `repository_ports.go`. Split route registration into helper functions by concern while keeping `RegisterProductRoutes` and every public endpoint path unchanged.

- [ ] **Step 4: Run GREEN**

Run:

```bash
cd orderapp-remote
go test ./internal/architecture -run TestCatalogBoundaryHasFocusedPortsAndRouteFiles -count=1
go test ./internal/interfaces/http/catalog -count=1
go test ./internal/application/catalog -count=1
```

Expected: pass.

### Task 4: Final Verification

- [ ] **Step 1: Run targeted architecture and touched package tests**

```bash
cd orderapp-remote
go test ./internal/architecture -count=1
go test ./internal/interfaces/http/support -count=1
go test ./internal/interfaces/http/catalog -count=1
go test ./internal/application/catalog -count=1
```

- [ ] **Step 2: Run broad verification**

```bash
cd orderapp-remote
go test ./...
npm run build --prefix frontend-vue-shell
scripts/verify_kferp.sh changed
git diff --check
```

- [ ] **Step 3: Report evidence**

Final report must include changed files, RED/GREEN commands, broad verification results, and any remaining architecture debt that was intentionally left for follow-up.
