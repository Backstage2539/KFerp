# Root Package Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move root-level Go business and test code out of `orderapp-remote`, leaving the module root as an executable entrypoint only.

**Architecture:** Keep behavior stable by moving the current root `package main` implementation into `internal/appmain` as one application package. The module root keeps only `main.go`, which delegates to `appmain.Main()`, while architecture tests enforce that future business and test files do not return to the root.

**Tech Stack:** Go 1.22, Echo, PostgreSQL/pgx, Vue/Vite frontend.

---

### Task 1: Add Root Layout Guard

**Files:**
- Create: `orderapp-remote/internal/architecture/root_package_test.go`

- [x] Add tests that fail while root contains business `.go` files or `_test.go` files.
- [x] Run `go test ./internal/architecture -run TestOrderappRootContainsOnlyEntrypoint -count=1` and verify failure.

### Task 2: Move Root Implementation Into `internal/appmain`

**Files:**
- Move: all `orderapp-remote/*.go` except the new root `main.go`
- Create: `orderapp-remote/internal/appmain/testmain_test.go`
- Modify: `orderapp-remote/main.go`

- [x] Move all current root Go files into `orderapp-remote/internal/appmain`.
- [x] Change moved files from `package main` to `package appmain`.
- [x] Rename the moved executable function to `Main()`.
- [x] Replace root `main.go` with a minimal delegating entrypoint.
- [x] Add a package test `TestMain` that changes test cwd back to the module root so existing file-based tests still inspect project paths consistently.

### Task 3: Update Source-Inspection Tests

**Files:**
- Modify moved `*_test.go` files under `orderapp-remote/internal/appmain`

- [x] Point source-inspection tests at `internal/appmain/<file>.go` for moved Go files.
- [x] Keep frontend, docs, db, and template path checks rooted at the module root.
- [x] Run `go test ./...` and fix any stale root-path assumptions.

### Task 4: Verify, Merge, Deploy

**Files:**
- Modify as needed from verification failures only.

- [x] Run `go test ./...`.
- [x] Run `npm run build` in `orderapp-remote/frontend-vue-shell`.
- [ ] Commit and push `codex/root-package-split-20260426`.
- [ ] Fast-forward merge into `develop`, rerun verification on `develop`, push `develop`.
- [ ] Deploy from `develop` and smoke test core Vue/API routes.
