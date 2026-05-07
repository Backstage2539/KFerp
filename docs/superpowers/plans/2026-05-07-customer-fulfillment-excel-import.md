# Customer Fulfillment Excel Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first reusable ERP customer fulfillment account workflow: parse exported Excel workbooks for processing, direct shipping, and settlement; persist import batches and rows; apply them into customer custody inventory, customer SKUs, work orders, direct-ship orders, fees, and settlement batches; expose the workflow in the Vue/Vite ERP shell.

**Architecture:** Add a new `customerfulfillment` vertical slice with application parser/service, Postgres repository/schema, HTTP module, and Vue view. The parser is pure Go and testable without the database; parsing stores import batches and row payloads first, while applying a batch performs idempotent business writes. Existing customer portal, customer-only SKU, `orders`, `order_items`, `customer_fee_items`, and `customer_settlement_batches` stay the integration points.

**Tech Stack:** Go, Echo, PostgreSQL/pgx, excelize, Vue 3 + Vite, node test, Vitest for miniapp, existing PR/DEV/UT/API/REV requirement tables.

---

## File Structure

- Create `orderapp-remote/internal/application/customerfulfillment/service.go`: DTOs, parser-facing commands, service validation, import parsing and apply orchestration.
- Create `orderapp-remote/internal/application/customerfulfillment/parser.go`: Excel parsing for processing, direct-ship, and settlement workbooks.
- Create `orderapp-remote/internal/application/customerfulfillment/parser_test.go`: pure parser tests with generated xlsx workbooks.
- Create `orderapp-remote/internal/application/customerfulfillment/service_test.go`: service validation and repository delegation tests.
- Create `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/schema.go`: tables for import batches, rows, custody items, custody ledger, balances, processing work orders, packaging jobs, conversion jobs, direct-ship import orders/items, and billing rules.
- Create `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`: Postgres implementation for parse storage, overview queries, and idempotent apply.
- Create `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/schema_test.go`: source guard for required schema objects.
- Create `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository_test.go`: DB-backed tests for parse persistence and apply idempotency.
- Create `orderapp-remote/internal/interfaces/http/customerfulfillment/module.go`: route registration.
- Create `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`: ERP JSON and multipart import API handlers.
- Create `orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go`: API handler tests for parse/apply/overview.
- Modify `orderapp-remote/internal/appmain/schema_setup.go`: include the new schema step.
- Modify `orderapp-remote/internal/appmain/app_routes.go`: construct and register the new service.
- Modify `orderapp-remote/internal/interfaces/http/support/authz_middleware.go`: give the new ERP endpoints a permission boundary.
- Modify `orderapp-remote/internal/interfaces/http/support/req_store.go`: seed PR/DEV/UT/API/REV rows.
- Create `orderapp-remote/internal/interfaces/http/support/dev_157_customer_fulfillment_excel_import_test.go`: source guard for requirements, routes, menu, Vue page, and no template debt.
- Modify `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`: add "客户履约账户" to the customer/settings area.
- Modify `orderapp-remote/frontend-vue-shell/src/App.vue`: import and wire `CustomerFulfillmentView`.
- Create `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js`: API wrappers.
- Create `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.test.js`: wrapper URL and form-data tests.
- Create `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.js`: UI helpers for import type labels, summaries, and row status labels.
- Create `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.test.js`: helper tests.
- Create `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`: Vue workbench.

## Task 1: Requirement Rows And Architecture Guards

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Create: `orderapp-remote/internal/interfaces/http/support/dev_157_customer_fulfillment_excel_import_test.go`

- [ ] **Step 1: Write failing requirement/source guard test**

Create `dev_157_customer_fulfillment_excel_import_test.go` with checks for these exact requirement codes and source wiring:

```go
func TestCustomerFulfillmentExcelImportRequirementSeeds(t *testing.T) {
	body := string(readSupportFileForTest(t, "req_store.go"))
	for _, code := range []string{
		"PR-157",
		"DEV-157-01",
		"DEV-157-02",
		"DEV-157-03",
		"DEV-157-04",
		"UT-157-01",
		"API-157-01",
		"REV-157-01",
	} {
		if !strings.Contains(body, code) {
			t.Fatalf("req_store.go missing %s", code)
		}
	}
}
```

Add a second test that reads:

- `internal/appmain/app_routes.go`
- `internal/appmain/schema_setup.go`
- `frontend-vue-shell/src/lib/menu-ia.js`
- `frontend-vue-shell/src/App.vue`

and requires these strings:

```go
[]string{
	"postgrescustomerfulfillment",
	"customerfulfillmenthttp.RegisterRoutes",
	"customerFulfillment",
	"CustomerFulfillmentView",
	"客户履约账户",
}
```

Run:

```bash
go test ./internal/interfaces/http/support -run TestCustomerFulfillmentExcelImport -count=1
```

Expected: FAIL because no rows or wiring exist yet.

- [ ] **Step 2: Seed requirement rows**

Add rows to `req_store.go`:

- `PR-157`: 客户履约账户与 Excel 导入闭环。
- `DEV-157-01`: Excel 解析与导入批次持久化。
- `DEV-157-02`: 客户托管库存、客户 SKU、加工工单和代发订单应用导入。
- `DEV-157-03`: 费用明细与月结结算生成。
- `DEV-157-04`: Vue/Vite 客户履约账户工作台。
- `UT-157-01`: parser/service/repository/frontend helper unit tests.
- `API-157-01`: parse/apply/overview API tests.
- `REV-157-01`: 按誉观山三份样本文件验收。

Run the focused support test again. Expected: requirement seed assertion passes, architecture wiring assertion still fails until later tasks add routes and UI.

## Task 2: Pure Excel Parser

**Files:**
- Create: `orderapp-remote/internal/application/customerfulfillment/service.go`
- Create: `orderapp-remote/internal/application/customerfulfillment/parser.go`
- Create: `orderapp-remote/internal/application/customerfulfillment/parser_test.go`

- [ ] **Step 1: Write failing parser tests**

Add tests that generate small workbooks using excelize:

```go
func TestParseProcessingWorkbookExtractsCustodyWorkOrdersAndSKU(t *testing.T)
func TestParseDirectShipWorkbookCarriesForwardOrderHeaderRows(t *testing.T)
func TestParseSettlementWorkbookExtractsFeeLines(t *testing.T)
func TestParseQuantityAndExcelDateHelpers(t *testing.T)
```

The processing workbook test must include sheets `生豆入库表`, `生豆库存表`, `生产工单`, `生产子工单-包装`, `SKU`, and `耗材库存（预估）`; it must expect row types `raw_bean_receipt`, `raw_bean_balance`, `processing_work_order`, `packaging_job`, `customer_sku`, and `packaging_balance`.

The direct-ship workbook test must create a second product row with empty date/order/address and assert the parser carries the previous order header.

The settlement workbook test must create rows matching `烘焙`, `代发、仓储费用`, and `物流费用` sections and assert fee types `roasting`, `direct_ship_service`, `storage`, and `shipping`.

Run:

```bash
go test ./internal/application/customerfulfillment -run TestParse -count=1
```

Expected: FAIL because the package does not exist.

- [ ] **Step 2: Implement parser DTOs and helpers**

Create DTOs in `service.go`:

```go
type ImportType string

const (
	ImportTypeProcessingWorkbook ImportType = "processing_workbook"
	ImportTypeDirectShipWorkbook ImportType = "direct_ship_workbook"
	ImportTypeSettlementWorkbook ImportType = "settlement_workbook"
)

type ParsedWorkbook struct {
	ImportType ImportType
	Rows       []ParsedRow
	Summary    ImportSummary
}

type ParsedRow struct {
	SheetName   string         `json:"sheet_name"`
	RowNo       int            `json:"row_no"`
	RowType     string         `json:"row_type"`
	ExternalKey string         `json:"external_key"`
	Payload     map[string]any `json:"payload"`
	Error       string         `json:"error,omitempty"`
}

type ImportSummary struct {
	TotalRows          int `json:"total_rows"`
	ValidRows          int `json:"valid_rows"`
	InvalidRows        int `json:"invalid_rows"`
	RawBeanReceipts    int `json:"raw_bean_receipts"`
	RawBeanIssues      int `json:"raw_bean_issues"`
	RawBeanBalances    int `json:"raw_bean_balances"`
	CustomerSKUs       int `json:"customer_skus"`
	PackagingBalances  int `json:"packaging_balances"`
	ProcessingOrders   int `json:"processing_orders"`
	PackagingJobs      int `json:"packaging_jobs"`
	ConversionJobs     int `json:"conversion_jobs"`
	DirectShipOrders   int `json:"direct_ship_orders"`
	DirectShipItems    int `json:"direct_ship_items"`
	FeeItems           int `json:"fee_items"`
	SettlementBatches  int `json:"settlement_batches"`
}
```

Implement helpers in `parser.go`:

```go
func ParseWorkbook(importType ImportType, r io.Reader) (ParsedWorkbook, error)
func parseQtyG(value string) (int64, bool)
func parseQtyUnits(value string) (int64, bool)
func parseExcelDateText(value string) string
func normalizedCell(value string) string
```

Use `excelize.OpenReader(r)` and `GetRows(sheet)`. Treat missing known sheets as empty, not fatal. Unknown import type returns `import type invalid`.

Run parser tests. Expected: PASS.

## Task 3: Application Service Validation

**Files:**
- Modify: `orderapp-remote/internal/application/customerfulfillment/service.go`
- Create: `orderapp-remote/internal/application/customerfulfillment/service_test.go`

- [ ] **Step 1: Write failing service tests**

Add tests:

```go
func TestServiceParseImportRequiresCustomerAndFile(t *testing.T)
func TestServiceParseImportStoresParsedRowsWithSHA(t *testing.T)
func TestServiceApplyImportDelegatesToRepository(t *testing.T)
func TestServiceCreateSettlementRequiresPeriod(t *testing.T)
```

Use a fake repository that records commands. The parse test must assert `SourceSHA256` is a 64-character lowercase hex string and `SourceFilename` is trimmed.

Run:

```bash
go test ./internal/application/customerfulfillment -run TestService -count=1
```

Expected: FAIL because service methods are missing.

- [ ] **Step 2: Implement service**

Add commands and repository interface:

```go
type ParseImportCommand struct {
	CustomerID      int64
	ImportType      ImportType
	SourceFilename  string
	Reader          io.Reader
	CreatedBy       string
}

type StoreParsedImportCommand struct {
	CustomerID      int64
	ImportType      ImportType
	SourceFilename  string
	SourceSHA256    string
	Parsed          ParsedWorkbook
	CreatedBy       string
}

type ApplyImportCommand struct {
	BatchID int64
	Actor   string
}

type CreateSettlementCommand struct {
	CustomerID  int64
	PeriodFrom  string
	PeriodTo    string
	CreatedBy   string
}

type Repository interface {
	StoreParsedImport(context.Context, StoreParsedImportCommand) (ImportBatch, error)
	ApplyImport(context.Context, ApplyImportCommand) (ApplyResult, error)
	CreateSettlement(context.Context, CreateSettlementCommand) (SettlementResult, error)
	Overview(context.Context, OverviewQuery) (Overview, error)
	ListImports(context.Context, ListImportsQuery) ([]ImportBatch, error)
}
```

`ParseImport` must read at most 20 MB, compute SHA-256, call `ParseWorkbook`, then store parsed rows. `ApplyImport` requires positive `BatchID`. `CreateSettlement` validates both dates as `YYYY-MM-DD`.

Run service tests. Expected: PASS.

## Task 4: Postgres Schema And Parse Persistence

**Files:**
- Create: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/schema.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/schema_test.go`
- Create: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository_test.go`
- Modify: `orderapp-remote/internal/appmain/schema_setup.go`

- [ ] **Step 1: Write failing schema and repository tests**

Schema test requires table names from the design:

```go
customer_fulfillment_import_batches
customer_fulfillment_import_rows
customer_custody_items
customer_custody_ledger_entries
customer_custody_balances
customer_processing_work_orders
customer_processing_work_order_inputs
customer_processing_packaging_jobs
customer_inventory_conversion_jobs
customer_direct_ship_import_orders
customer_direct_ship_import_order_items
customer_billing_rules
```

Repository test must:

- Create test schema with `customers`, `products`, statuses, and order tables.
- Run `EnsureSchema`.
- Call `StoreParsedImport` with two rows.
- Assert duplicate same customer/type/SHA returns the same batch instead of inserting a second batch.
- Assert import rows are persisted as JSON payloads.

Run:

```bash
go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestCustomerFulfillmentSchema|TestStoreParsedImport' -count=1
```

Expected: FAIL because schema/repository do not exist.

- [ ] **Step 2: Implement schema and parse persistence**

Implement `EnsureSchema(ctx, pool, schema)` with `CREATE TABLE IF NOT EXISTS` plus indexes:

- unique index on `(customer_id, import_type, source_sha256)`
- index on `customer_fulfillment_import_rows(batch_id, row_type, status)`
- unique custody item index on `(customer_id, item_type, external_code)` where external code is not empty
- unique direct ship import order index on `(customer_id, external_order_no, external_seq)` where external order is not empty

Implement:

```go
func NewRepository(pool *pgxpool.Pool, schema string) Repository
func (r Repository) StoreParsedImport(ctx context.Context, cmd app.StoreParsedImportCommand) (app.ImportBatch, error)
```

Store batches with status `parsed`, rows with status `valid` or `invalid` based on `ParsedRow.Error`.

Register the schema step in `appmain/schema_setup.go` as `customerfulfillment`.

Run repository tests. Expected: PASS.

## Task 5: Apply Processing Workbook Rows

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository_test.go`

- [ ] **Step 1: Write failing processing apply test**

Add `TestApplyProcessingImportCreatesCustodyAndWorkOrdersIdempotently`.

Seed a parsed batch with row types:

- `customer_sku`
- `raw_bean_receipt`
- `raw_bean_issue`
- `raw_bean_balance`
- `processing_work_order`
- `packaging_job`
- `packaging_balance`
- `conversion_job`

Expected after first apply:

- one customer-only product exists for the SKU.
- custody items exist for raw bean and packaging.
- custody ledger has receipt/issue/adjustment rows.
- custody balance equals imported balance.
- processing work order and packaging job exist.
- applying again keeps counts unchanged.

Run:

```bash
go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyProcessingImport -count=1
```

Expected: FAIL because `ApplyImport` is missing.

- [ ] **Step 2: Implement processing apply**

Implement:

```go
func (r Repository) ApplyImport(ctx context.Context, cmd app.ApplyImportCommand) (app.ApplyResult, error)
```

Inside a transaction:

- lock the batch row with `FOR UPDATE`
- load valid rows ordered by `id`
- dispatch by `row_type`
- mark each row `applied` with `target_type` and `target_id`
- mark batch `applied`

Processing handlers:

- Upsert `products` for customer SKU using customer ID and external code/name.
- Upsert `customer_custody_items` for raw beans and packaging.
- Insert idempotent `customer_custody_ledger_entries` by `(customer_id, source_type, external_doc_no, custody_item_id, movement_type)`.
- Upsert balances after every ledger movement.
- Upsert work orders by `(customer_id, external_work_order_no)`.
- Upsert packaging jobs by `(work_order_id, external_row_no)`.
- Upsert conversion jobs by `(customer_id, external_job_no, source_sku_name, target_sku_name)`.

Run the processing apply test. Expected: PASS.

## Task 6: Apply Direct-Ship Workbook Rows

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository_test.go`

- [ ] **Step 1: Write failing direct-ship apply test**

Add `TestApplyDirectShipImportCreatesOrdersAndSnapshotsIdempotently`.

Seed one direct-ship order with two item rows. Expected:

- `customer_direct_ship_import_orders` has one row.
- `customer_direct_ship_import_order_items` has two rows.
- `orders` has one row with `customer_id`, `portal_service_code='direct_ship'`, `receiver_name`, `receiver_phone`, `receiver_address`, `ship_tracking_no`, and `source_warehouse`.
- `order_items` has two rows.
- applying again keeps counts unchanged.

Run:

```bash
go test ./internal/infrastructure/postgres/customerfulfillment -run TestApplyDirectShipImport -count=1
```

Expected: FAIL until direct-ship apply exists.

- [ ] **Step 2: Implement direct-ship apply**

For each direct-ship order payload:

- parse receiver from payload fields already produced by parser.
- find or create `customer_direct_ship_import_orders`.
- create or reuse `orders` by checking `customer_direct_ship_import_orders.order_id`.
- use `portal_service_code='direct_ship'`.
- use customer processing warehouse from `customer_portal_profiles.processing_warehouse_code`; fallback to `cust_<customer_id>_processing`.
- create `order_items` for item rows, matching customer-only or public products by product name; if no product exists, keep `product_id=0` and preserve `item_name`.
- if tracking number exists, set `ship_tracking_no`.

Run direct-ship apply test. Expected: PASS.

## Task 7: Apply Settlement Rows And Generate Settlement Batch

**Files:**
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository.go`
- Modify: `orderapp-remote/internal/infrastructure/postgres/customerfulfillment/repository_test.go`

- [ ] **Step 1: Write failing settlement tests**

Add tests:

```go
func TestApplySettlementImportCreatesFeeItemsIdempotently(t *testing.T)
func TestCreateSettlementAggregatesUnsettledFees(t *testing.T)
```

Expected:

- fee rows insert into `customer_fee_items`.
- repeated apply does not duplicate.
- creating settlement for `2026-03-01` to `2026-03-31` inserts one `customer_settlement_batches` row.
- unsettled fee rows in the period get `settlement_batch_id`.
- total amount equals sum of fee rows.

Run:

```bash
go test ./internal/infrastructure/postgres/customerfulfillment -run 'TestApplySettlement|TestCreateSettlement' -count=1
```

Expected: FAIL.

- [ ] **Step 2: Implement settlement apply and generation**

Map fee row types:

- `roasting` -> `processing`
- `grinding` -> `processing`
- `bagging` -> `packaging`
- `drip_bag` -> `processing`
- `boxing` -> `packaging`
- `direct_ship_service` -> `direct_ship_service`
- `storage` -> `storage`
- `shipping` -> `shipping`
- `adjustment` -> `adjustment`

Use `source_type='customer_fulfillment_import'` and `source_id=<row_id>`. Use `ON CONFLICT` or pre-check by source to avoid duplicates.

`CreateSettlement` inserts `settlement_no='CS-' || customer_id || '-' || period_from compact date || '-' || period_to compact date`, sums unsettled fees in period, updates those fee rows, and returns total.

Run settlement tests. Expected: PASS.

## Task 8: ERP HTTP API

**Files:**
- Create: `orderapp-remote/internal/interfaces/http/customerfulfillment/module.go`
- Create: `orderapp-remote/internal/interfaces/http/customerfulfillment/api.go`
- Create: `orderapp-remote/internal/interfaces/http/customerfulfillment/api_test.go`
- Modify: `orderapp-remote/internal/appmain/app_routes.go`
- Modify: `orderapp-remote/internal/interfaces/http/support/authz_middleware.go`

- [ ] **Step 1: Write failing API tests**

Create fake service tests for:

```go
func TestParseImportAPIAcceptsMultipartFile(t *testing.T)
func TestApplyImportAPIReturnsApplySummary(t *testing.T)
func TestOverviewAPIReturnsCustomerFulfillmentData(t *testing.T)
func TestCreateSettlementAPIRequiresPeriod(t *testing.T)
```

The multipart test posts to:

```text
POST /api/customer-fulfillment/147/imports/parse
```

with fields:

- `import_type=direct_ship_workbook`
- `file=@direct.xlsx`

Expected response contains `batch_id`, `valid_rows`, and `direct_ship_orders`.

Run:

```bash
go test ./internal/interfaces/http/customerfulfillment -count=1
```

Expected: FAIL.

- [ ] **Step 2: Implement HTTP module and route registration**

Define interface:

```go
type Service interface {
	ParseImport(context.Context, app.ParseImportCommand) (app.ImportBatch, error)
	ApplyImport(context.Context, app.ApplyImportCommand) (app.ApplyResult, error)
	CreateSettlement(context.Context, app.CreateSettlementCommand) (app.SettlementResult, error)
	Overview(context.Context, app.OverviewQuery) (app.Overview, error)
	ListImports(context.Context, app.ListImportsQuery) ([]app.ImportBatch, error)
}
```

Register routes:

```go
e.GET("/api/customer-fulfillment/:customer_id/overview", ...)
e.POST("/api/customer-fulfillment/:customer_id/imports/parse", ...)
e.POST("/api/customer-fulfillment/imports/:batch_id/apply", ...)
e.GET("/api/customer-fulfillment/:customer_id/imports", ...)
e.POST("/api/customer-fulfillment/:customer_id/settlements", ...)
```

In `app_routes.go`, construct:

```go
customerFulfillmentSvc := customerfulfillmentapp.NewService(postgrescustomerfulfillment.NewRepository(pool, schema))
customerfulfillmenthttp.RegisterRoutes(e, customerfulfillmenthttp.Dependencies{CustomerFulfillment: customerFulfillmentSvc})
```

In auth middleware, map `/api/customer-fulfillment` to the existing inventory/customer operations permission boundary, and add a support source guard that requires the `/api/customer-fulfillment` prefix to be present in `authz_middleware.go`.

Run API tests and support architecture guard. Expected: PASS.

## Task 9: Vue API Wrappers And Helpers

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.js`
- Create: `orderapp-remote/frontend-vue-shell/src/api/customer-fulfillment.test.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.js`
- Create: `orderapp-remote/frontend-vue-shell/src/lib/customer-fulfillment.test.js`

- [ ] **Step 1: Write failing frontend helper/API tests**

Tests must cover:

- `importTypeOptions()` returns the three import types with Chinese labels.
- `importSummaryCards(summary)` includes valid rows, invalid rows, direct-ship orders, processing orders, and fees only when relevant.
- `rowStatusLabel("invalid")` returns `错误`.
- `buildCustomerFulfillmentImportForm(importType, file)` returns `FormData` with `import_type` and `file`.
- API wrappers call `/api/customer-fulfillment/...`.

Run:

```bash
node --test src/lib/customer-fulfillment.test.js src/api/customer-fulfillment.test.js
```

Expected: FAIL.

- [ ] **Step 2: Implement wrappers and helpers**

Use existing `apiGet`, `apiSend`, and `apiFetch` patterns. For upload, use raw `apiFetch` with `FormData` and method `POST`; do not manually set multipart content type.

Run the new node tests. Expected: PASS.

## Task 10: Vue Workbench Wiring

**Files:**
- Create: `orderapp-remote/frontend-vue-shell/src/views/CustomerFulfillmentView.vue`
- Modify: `orderapp-remote/frontend-vue-shell/src/lib/menu-ia.js`
- Modify: `orderapp-remote/frontend-vue-shell/src/App.vue`
- Modify: `orderapp-remote/internal/interfaces/http/support/dev_157_customer_fulfillment_excel_import_test.go`

- [ ] **Step 1: Write failing source guard tests**

Extend support test to require:

- menu key `customerFulfillment`
- label `客户履约账户`
- `CustomerFulfillmentView.vue`
- no `templates/customer_fulfillment`
- API path `/api/customer-fulfillment`

Run:

```bash
go test ./internal/interfaces/http/support -run TestCustomerFulfillmentExcelImport -count=1
```

Expected: FAIL until Vue page is wired.

- [ ] **Step 2: Implement Vue workbench**

Create a dense operational view:

- customer id input for first iteration, plus customer name display from overview.
- import type segmented buttons.
- file input.
- parse button.
- apply latest parsed batch button.
- settlement period inputs and generate settlement button.
- cards for import summary.
- tables for imports, custody balances, work orders, direct-ship orders, fees, and settlements.
- error panel for invalid rows.

Wire it in `App.vue` and `menu-ia.js`.

Run:

```bash
node --test src/lib/*.test.js src/api/*.test.js
go test ./internal/interfaces/http/support -run TestCustomerFulfillmentExcelImport -count=1
```

Expected: PASS.

## Task 11: End-To-End Verification And Requirement Evidence

**Files:**
- Modify: `orderapp-remote/internal/interfaces/http/support/req_store.go`
- Modify: `memory/2026-05-07.md` after completion.

- [ ] **Step 1: Update evidence rows**

Update `UT-157-01`, `API-157-01`, and `REV-157-01` evidence strings with exact commands and coverage:

- parser/service/repository tests
- API handler tests
- Vue helper tests
- build commands

- [ ] **Step 2: Run final local verification**

Run:

```bash
go test ./internal/application/customerfulfillment ./internal/infrastructure/postgres/customerfulfillment ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/support -count=1
go test ./... -count=1
node --test src/lib/*.test.js src/api/*.test.js
npm run build
npm test --prefix miniapp
npm run typecheck --prefix miniapp
git diff --check
```

Expected: all commands pass. If any command fails, fix before proceeding.

- [ ] **Step 3: Completion audit**

Audit against the design:

- Excel parse exists for three workbook types.
- Parse step does not mutate business data.
- Apply step writes custody inventory, processing records, direct-ship orders, fee items, and settlements idempotently.
- Vue page is under Vue/Vite, not templates.
- Requirement rows exist with evidence.
- Customer-only SKU isolation remains covered by existing tests.

Do not mark the goal complete until this audit passes.
