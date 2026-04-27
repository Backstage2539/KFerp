# Material Bean Profile Table Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move coffee bean metadata from generic materials into a coffee-bean child table while preserving costing and bean-list behavior.

**Architecture:** `materials` remains the shared material aggregate root. `material_bean_profiles` stores coffee-bean-only attributes in infrastructure and application DTOs as `bean_profile`. Costing reads `materials.purchase_price` for price and joins the profile table only for bean-list metadata.

**Tech Stack:** Go 1.22, pgx/Postgres, Echo, Vue 3 + Vite.

---

### Task 1: Schema And DTO Shape

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/schema.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/materials/schema_test.go`
- Modify: `orderapp-remote/internal/application/materials/service.go`

- [x] Add failing tests for `material_bean_profiles` and no bean columns in canonical `materials` DDL.
- [x] Create `material_bean_profiles(material_id PRIMARY KEY REFERENCES materials(id) ON DELETE CASCADE, ...)`.
- [x] Migrate existing deployed profile columns into the child table with `INSERT ... SELECT ... ON CONFLICT DO UPDATE`.
- [x] Change application material DTOs to expose `BeanProfile *BeanProfile`.

### Task 2: Material Repository And API

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/materials/repository_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/MaterialsView.vue`

- [x] Add failing tests for batch default and bean profile normalization.
- [x] Join profile rows when listing materials.
- [x] Upsert profile rows only for `kind=bean`; delete profile rows when a material changes away from `bean`.
- [x] Keep audit logging for profile field changes.
- [x] Update Vue material grid to show bean fields only for bean rows and send `bean_profile`.

### Task 3: Costing And Workflow Seeds

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/costing/repository.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Test: `orderapp-remote/internal/interfaces/http/support/dev_076_step1_test.go`

- [x] Read bean-list metadata from `material_bean_profiles`.
- [x] Seed PR-076/DEV/UT/API/REV rows for the child-table correction.
- [x] Run focused Go tests.
- [x] Run full `go test ./...` and Vue build.

### Task 4: Deploy

**Files:**
- Use: `deploy_orderapp.sh`

- [ ] Push feature branch.
- [ ] Merge cleanly into `develop` after tests.
- [ ] Push `develop`.
- [ ] Deploy and smoke test material API, costing bean list, and Vue entries.
