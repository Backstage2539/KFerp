# Document PDF Preview And Contract Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make sales-order and delivery-note previews render as real preview-marked PDFs with draggable seals, and turn contract stamping into a saved, deletable contract workspace.

**Architecture:** Backend adds preview-PDF service/repository methods that render but do not persist document versions. Frontend introduces one shared PDF stamp preview component used by sales orders, delivery notes, and contracts. Contract metadata save/delete is handled through application service methods and PostgreSQL soft-delete columns.

**Tech Stack:** Go, Echo, pgx/PostgreSQL, gofpdf, Vue 3, Vite, PDF.js, pdf-lib, Node test runner.

---

## File Map

- Modify `orderapp-remote/internal/infrastructure/pdf/sales_order_pdf.go`: add preview render path and preview marker.
- Modify `orderapp-remote/internal/infrastructure/pdf/delivery_note_pdf.go`: add preview render path and preview marker.
- Modify `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`: extend renderer interfaces with preview methods.
- Modify `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository.go`: add `PreviewSalesOrderPDF`.
- Modify `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository.go`: add `PreviewDeliveryNotePDF`.
- Modify `orderapp-remote/internal/application/sales/service.go`: add preview PDF structs, repository interface methods, and service methods.
- Modify `orderapp-remote/internal/interfaces/http/sales/sales_order_documents.go`: add `/api/orders/:id/sales-order-preview.pdf`.
- Modify `orderapp-remote/internal/interfaces/http/sales/delivery_note_documents.go`: add `/api/orders/:id/delivery-note-preview.pdf`.
- Modify sales API/repository tests under `orderapp-remote/internal/interfaces/http/sales` and `orderapp-remote/internal/infrastructure/postgres/sales`.
- Modify `orderapp-remote/internal/application/contracts/service.go`: add update/delete commands and repository methods.
- Modify `orderapp-remote/internal/infrastructure/postgres/contracts/schema.go`: add `note`, `deleted_at`, `deleted_by` columns.
- Modify `orderapp-remote/internal/infrastructure/postgres/contracts/repository.go`: add update and soft delete; list/download only active documents.
- Modify `orderapp-remote/internal/interfaces/http/contracts/module.go` and `contracts_api.go`: add `PUT /api/contracts/:id` and `DELETE /api/contracts/:id`.
- Create `orderapp-remote/frontend-vue-shell/src/components/PDFStampPreview.vue`: shared PDF renderer/overlay.
- Create `orderapp-remote/frontend-vue-shell/src/lib/document-pdf-stamp.js`: PDF point/mm conversion and drag helpers.
- Modify `orderapp-remote/frontend-vue-shell/src/views/SalesOrderView.vue`: replace HTML document preview with shared PDF preview.
- Modify `orderapp-remote/frontend-vue-shell/src/views/DeliveryNoteView.vue`: replace HTML document preview with shared PDF preview.
- Modify `orderapp-remote/frontend-vue-shell/src/views/ContractsView.vue`: use shared preview component, improve layout, add metadata save/delete.
- Modify tests in `orderapp-remote/frontend-vue-shell/src/lib/*.test.js` and support guard tests.
- Modify docs: `OP_MANUAL_ORDER_SALES.md`, `REQUIREMENTS.md`, `ACCEPTANCE_TESTS.md`, `orderapp-remote/internal/interfaces/http/support/req_store.go`, and an acceptance evidence file.

## Task 1: Backend Sales/Delivery Preview PDFs

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/pdf/sales_order_pdf.go`
- Modify: `orderapp-remote/internal/infrastructure/pdf/delivery_note_pdf.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository.go`
- Modify: `orderapp-remote/internal/application/sales/service.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/sales_order_documents.go`
- Modify: `orderapp-remote/internal/interfaces/http/sales/delivery_note_documents.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/sales_order_api_test.go`
- Test: `orderapp-remote/internal/interfaces/http/sales/delivery_note_api_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/sales/sales_order_repository_test.go`
- Test: `orderapp-remote/internal/infrastructure/postgres/sales/delivery_note_repository_test.go`

- [ ] **Step 1: Write failing API tests for preview PDF endpoints**

Add sales-order assertions:

```go
previewPDFReq := httptest.NewRequest(http.MethodGet, "/api/orders/1/sales-order-preview.pdf", nil)
previewPDFRec := httptest.NewRecorder()
e.ServeHTTP(previewPDFRec, previewPDFReq)
if previewPDFRec.Code != http.StatusOK || previewPDFRec.Header().Get(echo.HeaderContentType) != "application/pdf" {
    t.Fatalf("preview pdf status=%d content-type=%q body=%s", previewPDFRec.Code, previewPDFRec.Header().Get(echo.HeaderContentType), previewPDFRec.Body.String())
}
if !bytes.HasPrefix(previewPDFRec.Body.Bytes(), []byte("%PDF-")) {
    t.Fatalf("preview pdf prefix=%q", previewPDFRec.Body.Bytes()[:min(len(previewPDFRec.Body.Bytes()), 8)])
}
```

Add delivery-note assertions with `/api/orders/1/delivery-note-preview.pdf`.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/sales -run 'TestSalesOrderPreviewAPIDoesNotCreateDocumentVersion|TestDeliveryNoteDocumentAPI' -count=1 -v
```

Expected: FAIL because preview PDF routes are missing.

- [ ] **Step 3: Add service/repository preview PDF contracts**

Add structs:

```go
type SalesOrderPreviewPDF struct {
    Preview SalesOrderPreview
    Data    []byte
    Filename string
}

type DeliveryNotePreviewPDF struct {
    Preview DeliveryNotePreview
    Data    []byte
    Filename string
}
```

Add repository methods:

```go
PreviewSalesOrderPDF(ctx context.Context, orderID int64) (SalesOrderPreviewPDF, error)
PreviewDeliveryNotePDF(ctx context.Context, orderID int64) (DeliveryNotePreviewPDF, error)
```

Add service validation methods mirroring the JSON preview validation.

- [ ] **Step 4: Add preview render methods and marker**

Refactor sales renderer:

```go
func (r SalesOrderRenderer) Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
    return r.render(snapshot, false)
}

func (r SalesOrderRenderer) RenderPreview(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
    return r.render(snapshot, true)
}
```

At the end of `render`, call `renderPreviewLabel(pdf, "PREVIEW 预览版")` when preview is true. Implement the delivery-note renderer the same way.

- [ ] **Step 5: Add HTTP routes**

Register:

```go
e.GET("/api/orders/:id/sales-order-preview.pdf", h.previewPDF)
e.GET("/api/orders/:id/delivery-note-preview.pdf", h.previewPDF)
```

Handlers return `application/pdf` and `inline; filename="<order>-preview.pdf"`.

- [ ] **Step 6: Run focused backend tests**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/infrastructure/pdf -run 'Preview|SalesOrderDocumentAPI|DeliveryNoteDocumentAPI|Render' -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add orderapp-remote/internal/application/sales orderapp-remote/internal/infrastructure/pdf orderapp-remote/internal/infrastructure/postgres/sales orderapp-remote/internal/interfaces/http/sales
git commit -m "Add preview PDF endpoints for sales documents"
```

## Task 2: Contract Metadata Save And Soft Delete

**Files:**
- Modify: `orderapp-remote/internal/application/contracts/service.go`
- Modify: `orderapp-remote/internal/application/contracts/service_test.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/contracts/schema.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/contracts/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/contracts/repository_test.go`
- Modify: `orderapp-remote/internal/interfaces/http/contracts/module.go`
- Modify: `orderapp-remote/internal/interfaces/http/contracts/contracts_api.go`
- Modify: `orderapp-remote/internal/interfaces/http/contracts/contracts_api_test.go`

- [ ] **Step 1: Write failing service and API tests**

Service test:

```go
doc, err := svc.UpdateContract(context.Background(), UpdateContractCommand{Actor: "测试员", ContractID: 77, Title: "新版合同", Note: "客户已确认"})
if err != nil { t.Fatalf("UpdateContract: %v", err) }
if doc.Title != "新版合同" || doc.Note != "客户已确认" { t.Fatalf("doc=%+v", doc) }
```

API test:

```go
req := httptest.NewRequest(http.MethodPut, "/api/contracts/7", strings.NewReader(`{"title":"新版合同","note":"客户已确认"}`))
req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
rec := httptest.NewRecorder()
e.ServeHTTP(rec, req)
if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"note":"客户已确认"`) {
    t.Fatalf("update status=%d body=%s", rec.Code, rec.Body.String())
}
```

Delete test checks `DELETE /api/contracts/7` returns 200 and repository receives actor/id.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd orderapp-remote
go test ./internal/application/contracts ./internal/interfaces/http/contracts ./internal/infrastructure/postgres/contracts -run 'UpdateContract|DeleteContract|ContractAPI' -count=1 -v
```

Expected: FAIL because methods/routes/columns do not exist.

- [ ] **Step 3: Add application contract methods**

Add commands:

```go
type UpdateContractCommand struct {
    Actor string
    ContractID int64
    Title string
    Note string
}

type DeleteContractCommand struct {
    Actor string
    ContractID int64
}
```

Validate positive id and non-empty title. Extend `ContractDocument` with `Note`, `DeletedAt`, and `DeletedBy`.

- [ ] **Step 4: Add PostgreSQL columns and queries**

Schema statements:

```sql
ALTER TABLE <schema>.contract_documents ADD COLUMN IF NOT EXISTS note TEXT NOT NULL DEFAULT '';
ALTER TABLE <schema>.contract_documents ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE <schema>.contract_documents ADD COLUMN IF NOT EXISTS deleted_by TEXT NOT NULL DEFAULT '';
```

`ListContracts` adds `WHERE deleted_at IS NULL`. Download queries also require `deleted_at IS NULL`. `DeleteContract` sets `deleted_at=now(), deleted_by=$actor`.

- [ ] **Step 5: Add HTTP handlers**

Add request body:

```go
type contractUpdateRequest struct {
    Title string `json:"title"`
    Note  string `json:"note"`
}
```

Register:

```go
e.PUT("/api/contracts/:id", h.update)
e.DELETE("/api/contracts/:id", h.delete)
```

- [ ] **Step 6: Run focused contract tests**

Run:

```bash
cd orderapp-remote
go test ./internal/application/contracts ./internal/interfaces/http/contracts ./internal/infrastructure/postgres/contracts -count=1 -v
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add orderapp-remote/internal/application/contracts orderapp-remote/internal/infrastructure/postgres/contracts orderapp-remote/internal/interfaces/http/contracts
git commit -m "Add contract metadata save and soft delete"
```

## Task 3: Shared PDF Stamp Preview Frontend

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/components/PDFStampPreview.vue`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/document-pdf-stamp.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/document-pdf-stamp.test.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/contract-stamp.js`

- [ ] **Step 1: Write helper tests**

Test mm-to-PDF placement and drag:

```js
test('converts A4 millimeter seal position to PDF point placement', () => {
  const got = salesSealMMToPDFPlacement({ x_mm: 32, y_mm: 5, width_mm: 36 }, { pageWidth: 595.28 })
  assert.equal(Math.round(got.x), 91)
  assert.equal(Math.round(got.width), 102)
  assert.equal(Math.round(got.height), 63)
})
```

- [ ] **Step 2: Run helper tests and verify RED**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/document-pdf-stamp.test.js
```

Expected: FAIL because helper file does not exist.

- [ ] **Step 3: Implement `document-pdf-stamp.js`**

Export:

```js
export function salesSealMMToPDFPlacement(seal, page) { /* x/y/width/height in PDF points */ }
export function pdfPlacementToSalesSealMM(placement, page) { /* x_mm/y_mm/width_mm */ }
export function movePDFStampPlacement(placement, delta, displayScale) { /* same math as contract */ }
```

- [ ] **Step 4: Create `PDFStampPreview.vue`**

The component loads `pdfUrl` through `apiFetch`, renders pages through PDF.js, displays `previewLabel`, shows seal overlays, and emits:

```js
emit('loaded', pages)
emit('placement-change', nextPlacement)
emit('placement-commit', nextPlacement)
```

- [ ] **Step 5: Run helper tests**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/document-pdf-stamp.test.js src/lib/contract-stamp.test.js
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add orderapp-remote/frontend-vue-shell/src/components/PDFStampPreview.vue orderapp-remote/frontend-vue-shell/src/lib/document-pdf-stamp.js orderapp-remote/frontend-vue-shell/src/lib/document-pdf-stamp.test.js orderapp-remote/frontend-vue-shell/src/lib/contract-stamp.js
git commit -m "Add shared PDF stamp preview component"
```

## Task 4: Sales Order And Delivery Note Vue Integration

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/SalesOrderView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/views/DeliveryNoteView.vue`
- Modify: support guard tests for sales/delivery document views.

- [ ] **Step 1: Write source guard tests**

Update support tests to require:

```go
"PDFStampPreview"
"/api/orders/${orderID.value}/sales-order-preview.pdf"
"PREVIEW 预览版"
"placement-commit"
```

Delivery-note guard uses `/api/orders/${orderID.value}/delivery-note-preview.pdf`.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run 'SalesOrder|DeliveryNote|PDF' -count=1 -v
```

Expected: FAIL until Vue files use the shared component.

- [ ] **Step 3: Replace HTML previews with `PDFStampPreview`**

Sales order computes:

```js
const previewPDFUrl = computed(() => orderID.value ? `/api/orders/${orderID.value}/sales-order-preview.pdf` : '')
```

On `placement-commit`, convert placement to mm and call `/api/settings/sales-order/seal-position`.

Delivery note mirrors the same flow with `delivery-note-preview.pdf`.

- [ ] **Step 4: Run frontend and guard tests**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js
npm run build
cd ..
go test ./internal/interfaces/http/support -run 'SalesOrder|DeliveryNote|PDF' -count=1 -v
```

Expected: PASS. Vite chunk warning is acceptable if unchanged.

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/frontend-vue-shell/src/views/SalesOrderView.vue orderapp-remote/frontend-vue-shell/src/views/DeliveryNoteView.vue orderapp-remote/internal/interfaces/http/support
git commit -m "Use PDF preview stamping for sales documents"
```

## Task 5: Contract Workspace UI Integration

**Files:**
- Modify: `orderapp-remote/frontend-vue-shell/src/views/ContractsView.vue`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_273_contract_pdf_stamping_test.go`

- [ ] **Step 1: Write source guard tests**

Require markers:

```go
"PDFStampPreview"
"saveContractMetadata"
"deleteContract"
"/api/contracts/${selectedContractID.value}"
"合同备注"
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -run TestDev273ContractPDFStamping -count=1 -v
```

Expected: FAIL until UI is updated.

- [ ] **Step 3: Rework `ContractsView.vue`**

Use a register/workspace layout with metadata fields:

```vue
<input v-model.trim="contractForm.title" />
<textarea v-model.trim="contractForm.note"></textarea>
<button @click="saveContractMetadata">保存合同</button>
<button @click="deleteContract">删除合同</button>
<PDFStampPreview ... />
```

Keep `createStampedContractPDF` for final stamped PDF creation.

- [ ] **Step 4: Run frontend build and guard tests**

Run:

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js
npm run build
cd ..
go test ./internal/interfaces/http/support -run TestDev273ContractPDFStamping -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add orderapp-remote/frontend-vue-shell/src/views/ContractsView.vue orderapp-remote/internal/interfaces/http/support/dev_273_contract_pdf_stamping_test.go
git commit -m "Improve contract stamping workspace"
```

## Task 6: Requirements, Manuals, Acceptance Evidence

**Files:**
- Modify: `REQUIREMENTS.md`
- Modify: `ACCEPTANCE_TESTS.md`
- Modify: `OP_MANUAL_ORDER_SALES.md`
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Add: `docs/acceptance/2026-05-15-document-pdf-preview-and-contract-workspace.md`
- Add/Modify: support guard tests.

- [ ] **Step 1: Update requirements and acceptance**

Add PR/DEV rows for:

```text
PR-277-DOCUMENT-PDF-PREVIEW-STAMPING
DEV-277-01 backend preview PDFs
DEV-277-02 shared Vue PDF stamp preview
DEV-277-03 sales/delivery integration
PR-278-CONTRACT-WORKSPACE-SAVE-DELETE
DEV-278-01 contract metadata save/delete API
DEV-278-02 contract workspace UI
```

- [ ] **Step 2: Update manual**

Document:

```text
销售单/出库单预览显示“PREVIEW 预览版”，预览不会新增版本；确认生成 PDF 才新增正式历史版本。
合同标题和备注可保存；删除合同会从列表隐藏，但历史审计和文件保留。
```

- [ ] **Step 3: Add acceptance evidence skeleton**

Record commands and expected markers:

```text
SALES_ORDER_PREVIEW_PDF_OK
DELIVERY_NOTE_PREVIEW_PDF_OK
CONTRACT_METADATA_SAVE_DELETE_OK
CONTRACT_WORKSPACE_UI_OK
```

- [ ] **Step 4: Run docs/support checks**

Run:

```bash
cd orderapp-remote
go test ./internal/interfaces/http/support -count=1
git diff --check
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add REQUIREMENTS.md ACCEPTANCE_TESTS.md OP_MANUAL_ORDER_SALES.md docs/acceptance/2026-05-15-document-pdf-preview-and-contract-workspace.md orderapp-remote/internal/interfaces/http/support
git commit -m "Document PDF preview and contract workspace workflow"
```

## Task 7: Full Verification And Integration

**Files:**
- All changed files.

- [ ] **Step 1: Run backend full tests**

```bash
cd orderapp-remote
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run frontend tests and build**

```bash
cd orderapp-remote/frontend-vue-shell
node --test src/lib/*.test.js
npm run build
```

Expected: PASS, with only existing chunk-size warning if present.

- [ ] **Step 3: Run whitespace check**

```bash
git diff --check
```

Expected: no output.

- [ ] **Step 4: Commit or amend final fixes**

If any small fixes were needed:

```bash
git add <fixed files>
git commit -m "Polish document PDF preview workflow"
```

- [ ] **Step 5: Prepare merge/deploy**

```bash
git fetch origin
git log --oneline -3 origin/develop
git rev-parse origin/develop
git status --short --branch
```

Expected: branch is ahead of latest `origin/develop`, clean worktree, ready to push and merge after verification.

