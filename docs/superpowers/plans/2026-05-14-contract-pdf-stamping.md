# Contract PDF Stamping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build contract upload, DOCX-to-PDF conversion, multi-page seal dragging, and stamped PDF saving.

**Architecture:** Add a focused `contracts` backend module for file storage and metadata, reuse the existing sales-order seal asset settings for seal selection, and implement PDF page rendering/stamping in the Vue shell. Keep generated PDF editing in the browser with `pdf-lib`, while backend handles durable storage and DOCX conversion.

**Tech Stack:** Go 1.22, Echo, pgx/Postgres, LibreOffice `soffice`, Vue 3 + Vite, PDF.js, pdf-lib, Node test runner.

---

### Task 1: Requirement Seeds And Navigation

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_273_contract_pdf_stamping_test.go`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`

- [ ] Write a support test that fails until PR-275/DEV-275 rows, menu key `contracts`, and hidden/download labels exist.
- [ ] Run `go test ./internal/interfaces/http/support -run TestDev273ContractPDFStamping -count=1 -v` and confirm RED.
- [ ] Add PR/DEV seed rows and the `contracts` menu route.
- [ ] Re-run the support test and confirm GREEN.

### Task 2: Backend Contract Service

**Files:**
- Create: `orderapp-remote/internal/application/contracts/service.go`
- Create: `orderapp-remote/internal/application/contracts/service_test.go`
- Create: `orderapp-remote/internal/infrastructure/docconvert/soffice.go`
- Create: `orderapp-remote/internal/infrastructure/docconvert/soffice_test.go`

- [ ] Write service tests for PDF upload, DOCX conversion through a fake converter, unsupported type rejection, and stamped PDF validation.
- [ ] Run `go test ./internal/application/contracts ./internal/infrastructure/docconvert -count=1 -v` and confirm RED.
- [ ] Implement service validation, file writing, cleanup, and LibreOffice converter.
- [ ] Re-run the tests and confirm GREEN.

### Task 3: PostgreSQL Contract Repository

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/contracts/schema.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/contracts/repository.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/contracts/repository_test.go`
- Modify: `orderapp-remote/internal/appmain/schema_setup.go`

- [ ] Write repository tests for schema creation, contract insert/list, stamped version latest handling, and file lookup.
- [ ] Run `go test ./internal/infrastructure/postgres/contracts -count=1 -v` and confirm RED.
- [ ] Implement schema and repository methods.
- [ ] Register schema setup after the sales schema.
- [ ] Re-run repository tests and app schema bootstrap tests.

### Task 4: HTTP API And Runtime Wiring

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/contracts/module.go`
- Create: `orderapp-remote/internal/interfaces/http/contracts/contracts_api.go`
- Create: `orderapp-remote/internal/interfaces/http/contracts/contracts_api_test.go`
- Modify: `orderapp-remote/internal/appmain/app_routes.go`
- Modify: `orderapp-remote/internal/config/runtime.go`
- Modify: `orderapp-remote/internal/config/runtime_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/sales_order_settings.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository.go`

- [ ] Write HTTP tests for PDF upload, DOCX upload with fake converter, stamped PDF save/download, invalid type rejection, and sales-order seal asset listing.
- [ ] Run `go test ./internal/interfaces/http/contracts ./internal/interfaces/http/sales -run 'TestContract|TestSalesOrderSeal' -count=1 -v` and confirm RED.
- [ ] Implement routes, multipart parsing, downloads, runtime converter config, and seal listing.
- [ ] Re-run HTTP tests and confirm GREEN.

### Task 5: Vue Contract Workspace

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/package.json`
- Modify: `orderapp-remote/frontend-vue-shell/package-lock.json`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/contract-stamp.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/contract-stamp.test.js`
- Create: `orderapp-remote/frontend-vue-shell/src/views/ContractsView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`

- [ ] Add `pdf-lib` and `pdfjs-dist`.
- [ ] Write frontend helper tests for file-type labels, PDF coordinate conversion, drag updates, and stamped PDF placement payloads.
- [ ] Run `node --test src/lib/contract-stamp.test.js` and confirm RED.
- [ ] Implement helper functions.
- [ ] Build `ContractsView.vue` with upload, contract list, seal selector, multi-page canvases, draggable seal overlays, save, and download actions.
- [ ] Re-run helper tests and `npm run build`.

### Task 6: Manuals And Acceptance

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `OPERATION_MANUALS.md`
- Modify: `orderapp-remote/docs/REQUIREMENTS.md`
- Modify: `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- Modify: `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/docs/OPERATION_MANUALS.md`
- Create: `docs/acceptance/2026-05-14-contract-pdf-stamping.md`

- [ ] Update source and frontend manual docs with the contract stamping workflow.
- [ ] Add acceptance checklist evidence markers for upload, DOCX conversion, multi-page drag, seal choice, and stamped PDF saving.
- [ ] Re-run support/manual guard tests.

### Task 7: Final Verification

- [ ] Run `go test ./internal/application/contracts ./internal/infrastructure/docconvert ./internal/infrastructure/postgres/contracts ./internal/interfaces/http/contracts ./internal/interfaces/http/sales ./internal/interfaces/http/support -count=1`.
- [ ] Run `go test ./...`.
- [ ] Run `cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js`.
- [ ] Run `cd orderapp-remote/frontend-vue-shell && npm run build`.
- [ ] Run `git diff --check`.
- [ ] Audit objective coverage against concrete files, routes, tests, manuals, and acceptance evidence.
