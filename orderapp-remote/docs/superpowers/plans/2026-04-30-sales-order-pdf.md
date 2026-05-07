# Sales Order PDF Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build versioned sales-order PDF generation from orders, with configurable company text, payment codes, payment notes, and seal image.

**Architecture:** Extend the existing sales DDD module. Application layer owns sales-order commands and validation, PostgreSQL infrastructure owns settings/assets/document version persistence, a small PDF renderer package owns PDF bytes, and Vue/Vite owns settings and document list pages.

**Tech Stack:** Go 1.22, Echo, pgx, PostgreSQL JSONB, Vue 3 + Vite, Node test runner. PDF implementation uses a pure-Go PDF dependency added to `go.mod` after a smoke test confirms Chinese font rendering in Docker.

---

## File Structure

- Create `internal/domain/sales/sales_order.go`: pure sales-order value types, version calculation, money formatting, snapshot validation.
- Create `internal/domain/sales/sales_order_test.go`: unit coverage for version/money/snapshot behavior.
- Modify `internal/application/sales/service.go`: add sales-order setting/document structs, repository interface methods, service methods.
- Create `internal/application/sales/sales_order_service_test.go`: application-level validation and repository orchestration tests.
- Modify `internal/infrastructure/postgres/sales/schema.go`: create sales-order settings/assets/payment-codes/documents tables and indexes.
- Create `internal/infrastructure/postgres/sales/sales_order_repository.go`: setting persistence, asset metadata, document version persistence, order snapshot query.
- Create `internal/infrastructure/postgres/sales/sales_order_repository_test.go`: repository tests using the existing Postgres test pattern.
- Create `internal/infrastructure/pdf/sales_order_pdf.go`: render snapshot into PDF bytes.
- Create `internal/infrastructure/pdf/sales_order_pdf_test.go`: smoke tests for non-empty PDF bytes and Chinese text path.
- Modify `internal/appmain/app_routes.go`: construct sales repository with `assetDir` and PDF renderer dependency.
- Modify `internal/infrastructure/postgres/sales/repository.go`: add `assetDir` and optional PDF renderer fields to repository.
- Modify `internal/infrastructure/postgres/sales/schema.go`: call sales-order schema creation in `EnsureSchema`.
- Create `internal/interfaces/http/sales/sales_order_settings.go`: settings JSON API and image upload handlers.
- Create `internal/interfaces/http/sales/sales_order_documents.go`: document list/generate/download handlers.
- Modify `internal/interfaces/http/sales/module.go`: register new routes.
- Create `internal/interfaces/http/sales/sales_order_api_test.go`: handler/API tests for settings, generation, versioning, download.
- Modify `frontend-vue-shell/src/App.vue`: add menu entry and internal views.
- Modify `frontend-vue-shell/src/views/OrdersView.vue`: add `销售单` action.
- Create `frontend-vue-shell/src/views/SalesOrderSettingsView.vue`: setting form and image upload UI.
- Create `frontend-vue-shell/src/views/SalesOrderView.vue`: order summary, versions, generate/download UI.
- Create `frontend-vue-shell/src/lib/sales-order.js`: small URL/payload helpers.
- Create `frontend-vue-shell/src/lib/sales-order.test.js`: frontend helper tests.
- Modify `internal/interfaces/http/support/req_store.go`: seed PR/DEV/UT/API/REV rows.
- Modify `Dockerfile`: copy packaged Chinese font files if the chosen PDF library needs local TTF assets.
- Create `assets/fonts/README.md` and add the selected open-source Chinese font file only after license verification.

## Task 1: Requirement Tracking Rows

**Files:**
- Modify: `internal/interfaces/http/support/req_store.go`
- Test: existing `go test ./internal/interfaces/http/support`

- [ ] **Step 1: Write the failing test**

Add a focused test in `internal/interfaces/http/support/req_store_test.go`:

```go
func TestSeedReqWorkflowIncludesSalesOrderPDFRows(t *testing.T) {
	items := reqWorkflowSeedRows()
	want := []string{
		"PR-SALES-ORDER-001",
		"DEV-SALES-ORDER-001",
		"UT-SALES-ORDER-001",
		"API-SALES-ORDER-001",
		"REV-SALES-ORDER-001",
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.code] = true
	}
	for _, code := range want {
		if !seen[code] {
			t.Fatalf("seed rows missing %s", code)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/interfaces/http/support -run TestSeedReqWorkflowIncludesSalesOrderPDFRows -count=1
```

Expected: FAIL because `reqWorkflowSeedRows` does not exist or the sales-order rows are missing.

- [ ] **Step 3: Extract seed rows and add sales-order records**

In `internal/interfaces/http/support/req_store.go`, extract the anonymous seed slice into:

```go
type reqSeedRow struct {
	table, code, title, status, assignee, evidence string
}

func reqWorkflowSeedRows() []reqSeedRow {
	return []reqSeedRow{
		{table: "req_product", code: "PR-SALES-ORDER-001", title: "订单支持生成正式销售单 PDF，并支持销售单设置、快照和版本留痕", status: "todo", assignee: "VA"},
		{table: "req_dev", code: "DEV-SALES-ORDER-001", title: "新增销售单设置、版本化 PDF 生成、下载 API 和 Vue 页面", status: "todo", assignee: "Codex"},
		{table: "req_unit", code: "UT-SALES-ORDER-001", title: "覆盖销售单版本号、快照、金额格式化和 PDF 渲染单元测试", status: "todo", assignee: "Codex"},
		{table: "req_api", code: "API-SALES-ORDER-001", title: "覆盖销售单设置、生成 V1/V2、下载 PDF 的 API 测试", status: "todo", assignee: "Codex"},
		{table: "req_review", code: "REV-SALES-ORDER-001", title: "验收销售单 PDF 内容、设置快照、历史版本保留和下载", status: "todo", assignee: "VA"},
	}
}
```

Then append existing rows into the same function and have `seedReqWorkflowA` iterate over `reqWorkflowSeedRows()`.

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/interfaces/http/support -run TestSeedReqWorkflowIncludesSalesOrderPDFRows -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/interfaces/http/support/req_store.go internal/interfaces/http/support/req_store_test.go
git commit -m "chore: seed sales order pdf requirements"
```

## Task 2: Domain Model and Snapshot Rules

**Files:**
- Create: `internal/domain/sales/sales_order.go`
- Create: `internal/domain/sales/sales_order_test.go`

- [ ] **Step 1: Write failing domain tests**

Create `internal/domain/sales/sales_order_test.go`:

```go
package sales

import "testing"

func TestNextSalesOrderVersion(t *testing.T) {
	if got := NextSalesOrderVersion(nil); got != 1 {
		t.Fatalf("NextSalesOrderVersion(nil) = %d, want 1", got)
	}
	if got := NextSalesOrderVersion([]int{1, 2, 4}); got != 5 {
		t.Fatalf("NextSalesOrderVersion([1,2,4]) = %d, want 5", got)
	}
}

func TestFormatSalesOrderMoney(t *testing.T) {
	if got := FormatSalesOrderMoney(322); got != "322.00" {
		t.Fatalf("FormatSalesOrderMoney(322) = %q, want 322.00", got)
	}
	if got := FormatSalesOrderMoney(67.125); got != "67.13" {
		t.Fatalf("FormatSalesOrderMoney(67.125) = %q, want 67.13", got)
	}
}

func TestSalesOrderSnapshotValidate(t *testing.T) {
	s := SalesOrderSnapshot{
		OrderID:     9,
		OrderNo:     "SO-20260430-0008",
		CompanyName: "浅焙作坊咖啡",
		Items: []SalesOrderSnapshotItem{{
			Name: "橘皮乌龙", Spec: "300g", Qty: "2", UnitPrice: "67.00", LineTotal: "134.00",
		}},
		GrandTotal: "134.00",
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/domain/sales -run 'TestNextSalesOrderVersion|TestFormatSalesOrderMoney|TestSalesOrderSnapshotValidate' -count=1
```

Expected: FAIL because the types and functions are missing.

- [ ] **Step 3: Implement domain model**

Create `internal/domain/sales/sales_order.go`:

```go
package sales

import (
	"fmt"
	"strings"
)

type SalesOrderSnapshot struct {
	OrderID       int64                    `json:"order_id"`
	OrderNo       string                   `json:"order_no"`
	OrderDate     string                   `json:"order_date"`
	CustomerName  string                   `json:"customer_name"`
	CompanyName   string                   `json:"company_name"`
	PaymentText   string                   `json:"payment_text"`
	Note          string                   `json:"note"`
	Items         []SalesOrderSnapshotItem `json:"items"`
	TotalAmount   string                   `json:"total_amount"`
	Shipping      string                   `json:"shipping"`
	Discount      string                   `json:"discount"`
	GrandTotal    string                   `json:"grand_total"`
	PaymentCodes  []SalesOrderAssetRef     `json:"payment_codes"`
	Seal          *SalesOrderAssetRef       `json:"seal,omitempty"`
}

type SalesOrderSnapshotItem struct {
	Name      string `json:"name"`
	Spec      string `json:"spec"`
	Qty       string `json:"qty"`
	Unit      string `json:"unit"`
	UnitPrice string `json:"unit_price"`
	LineTotal string `json:"line_total"`
}

type SalesOrderAssetRef struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
}

func NextSalesOrderVersion(existing []int) int {
	max := 0
	for _, n := range existing {
		if n > max {
			max = n
		}
	}
	return max + 1
}

func FormatSalesOrderMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func (s SalesOrderSnapshot) Validate() error {
	if s.OrderID <= 0 {
		return fmt.Errorf("order_id required")
	}
	if strings.TrimSpace(s.OrderNo) == "" {
		return fmt.Errorf("order_no required")
	}
	if strings.TrimSpace(s.CompanyName) == "" {
		return fmt.Errorf("company_name required")
	}
	if len(s.Items) == 0 {
		return fmt.Errorf("items required")
	}
	if strings.TrimSpace(s.GrandTotal) == "" {
		return fmt.Errorf("grand_total required")
	}
	return nil
}
```

- [ ] **Step 4: Run domain tests**

Run:

```bash
go test ./internal/domain/sales -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/sales/sales_order.go internal/domain/sales/sales_order_test.go
git commit -m "feat: add sales order snapshot domain model"
```

## Task 3: Application Service Contracts

**Files:**
- Modify: `internal/application/sales/service.go`
- Create: `internal/application/sales/sales_order_service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `internal/application/sales/sales_order_service_test.go` with a fake repository implementing the existing interface plus new methods:

```go
package sales

import (
	"context"
	"testing"
)

func TestSaveSalesOrderSettingsValidatesCompanyName(t *testing.T) {
	repo := &fakeSalesOrderRepo{}
	svc := NewService(repo)
	err := svc.SaveSalesOrderSettings(context.Background(), SaveSalesOrderSettingsCommand{
		Actor:       "测试员",
		CompanyName: "  ",
	})
	if err == nil || err.Error() != "company_name required" {
		t.Fatalf("SaveSalesOrderSettings error = %v, want company_name required", err)
	}
}

func TestGenerateSalesOrderDocumentRequiresOrderID(t *testing.T) {
	repo := &fakeSalesOrderRepo{}
	svc := NewService(repo)
	_, err := svc.GenerateSalesOrderDocument(context.Background(), GenerateSalesOrderDocumentCommand{
		Actor: "测试员",
	})
	if err == nil || err.Error() != "invalid order id" {
		t.Fatalf("GenerateSalesOrderDocument error = %v, want invalid order id", err)
	}
}
```

The fake can return zero values for unrelated repository methods. Keep it in this test file so it does not affect production code.

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/application/sales -run 'TestSaveSalesOrderSettingsValidatesCompanyName|TestGenerateSalesOrderDocumentRequiresOrderID' -count=1
```

Expected: FAIL because service methods/types are missing.

- [ ] **Step 3: Add service types and repository interface methods**

In `internal/application/sales/service.go`, add:

```go
type SalesOrderSettings struct {
	CompanyName  string             `json:"company_name"`
	Note         string             `json:"note"`
	PaymentText  string             `json:"payment_text"`
	Seal         *SalesOrderAsset    `json:"seal,omitempty"`
	PaymentCodes []SalesOrderPayment `json:"payment_codes"`
}

type SalesOrderAsset struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Description string `json:"description"`
	ObjectKey   string `json:"object_key"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
}

type SalesOrderPayment struct {
	ID          int64  `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	AssetID     int64  `json:"asset_id"`
	AssetURL    string `json:"asset_url"`
	Sort        int    `json:"sort"`
	Active      bool   `json:"active"`
}

type SaveSalesOrderSettingsCommand struct {
	Actor       string
	CompanyName string
	Note        string
	PaymentText string
}

type GenerateSalesOrderDocumentCommand struct {
	Actor   string
	OrderID int64
}

type SalesOrderDocument struct {
	ID        int64  `json:"id"`
	OrderID   int64  `json:"order_id"`
	OrderNo   string `json:"order_no"`
	VersionNo int    `json:"version_no"`
	IsLatest  bool   `json:"is_latest"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	URL       string `json:"url"`
}

type SalesOrderDocumentResult struct {
	Document SalesOrderDocument `json:"document"`
}
```

Extend `Repository`:

```go
	LoadSalesOrderSettings(ctx context.Context) (SalesOrderSettings, error)
	SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error
	ListSalesOrderDocuments(ctx context.Context, orderID int64) ([]SalesOrderDocument, error)
	GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (SalesOrderDocumentResult, error)
```

Add service methods:

```go
func (s *Service) SaveSalesOrderSettings(ctx context.Context, cmd SaveSalesOrderSettingsCommand) error {
	cmd.CompanyName = strings.TrimSpace(cmd.CompanyName)
	if cmd.CompanyName == "" {
		return fmt.Errorf("company_name required")
	}
	return s.repo.SaveSalesOrderSettings(ctx, cmd)
}

func (s *Service) GenerateSalesOrderDocument(ctx context.Context, cmd GenerateSalesOrderDocumentCommand) (SalesOrderDocumentResult, error) {
	if cmd.OrderID <= 0 {
		return SalesOrderDocumentResult{}, fmt.Errorf("invalid order id")
	}
	return s.repo.GenerateSalesOrderDocument(ctx, cmd)
}
```

Also add simple pass-through methods for load settings and list documents with `orderID > 0` validation.

- [ ] **Step 4: Run application tests**

Run:

```bash
go test ./internal/application/sales -count=1
```

Expected: PASS after fake repository includes all interface methods.

- [ ] **Step 5: Commit**

```bash
git add internal/application/sales/service.go internal/application/sales/sales_order_service_test.go
git commit -m "feat: add sales order application contract"
```

## Task 4: PostgreSQL Schema and Settings Repository

**Files:**
- Modify: `internal/infrastructure/postgres/sales/schema.go`
- Modify: `internal/infrastructure/postgres/sales/repository.go`
- Create: `internal/infrastructure/postgres/sales/sales_order_repository.go`
- Create: `internal/infrastructure/postgres/sales/sales_order_repository_test.go`

- [ ] **Step 1: Write failing repository tests**

Create `internal/infrastructure/postgres/sales/sales_order_repository_test.go`:

```go
func TestSalesOrderSettingsRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	repo := NewRepository(pool, schema)
	ctx := context.Background()

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存", PaymentText: "微信或对公转账",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	got, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if got.CompanyName != "浅焙作坊咖啡" || got.Note != "请密封保存" || got.PaymentText != "微信或对公转账" {
		t.Fatalf("settings = %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/infrastructure/postgres/sales -run TestSalesOrderSettingsRoundTrip -count=1
```

Expected: FAIL because schema/repository methods are missing.

- [ ] **Step 3: Add schema**

In `schema.go`, add `ensureSalesOrderTables` and call it from `EnsureSchema`:

```go
func ensureSalesOrderTables(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_settings (
			id INTEGER PRIMARY KEY DEFAULT 1,
			company_name TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			payment_text TEXT NOT NULL DEFAULT '',
			seal_asset_id BIGINT,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT '',
			CONSTRAINT sales_order_settings_singleton CHECK (id = 1)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_assets (
			id BIGSERIAL PRIMARY KEY,
			kind TEXT NOT NULL,
			filename TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			bytes BIGINT NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			object_key TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_payment_codes (
			id BIGSERIAL PRIMARY KEY,
			label TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			asset_id BIGINT NOT NULL REFERENCES %s.sales_order_assets(id),
			sort INTEGER NOT NULL DEFAULT 0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema, schema),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.sales_order_documents (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES %s.orders(id),
			order_no TEXT NOT NULL DEFAULT '',
			version_no INTEGER NOT NULL,
			snapshot_json JSONB NOT NULL,
			pdf_asset_id BIGINT REFERENCES %s.sales_order_assets(id),
			is_latest BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_by TEXT NOT NULL DEFAULT '',
			UNIQUE(order_id, version_no)
		)`, schema, schema, schema),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_sales_order_latest ON %s.sales_order_documents(order_id) WHERE is_latest`, schema, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Implement settings repository**

In `sales_order_repository.go`, add:

```go
func (r Repository) LoadSalesOrderSettings(ctx context.Context) (salesapp.SalesOrderSettings, error) {
	var s salesapp.SalesOrderSettings
	q := fmt.Sprintf(`SELECT company_name, note, payment_text FROM %s.sales_order_settings WHERE id=1`, r.schema)
	err := r.pool.QueryRow(ctx, q).Scan(&s.CompanyName, &s.Note, &s.PaymentText)
	if errors.Is(err, pgx.ErrNoRows) {
		return salesapp.SalesOrderSettings{}, nil
	}
	if err != nil {
		return salesapp.SalesOrderSettings{}, err
	}
	return s, nil
}

func (r Repository) SaveSalesOrderSettings(ctx context.Context, cmd salesapp.SaveSalesOrderSettingsCommand) error {
	q := fmt.Sprintf(`INSERT INTO %s.sales_order_settings(id, company_name, note, payment_text, updated_at, updated_by)
		VALUES(1,$1,$2,$3,now(),$4)
		ON CONFLICT(id) DO UPDATE SET company_name=excluded.company_name,note=excluded.note,payment_text=excluded.payment_text,updated_at=now(),updated_by=excluded.updated_by`, r.schema)
	_, err := r.pool.Exec(ctx, q, cmd.CompanyName, cmd.Note, cmd.PaymentText, cmd.Actor)
	if err == nil {
		postgresinfra.AuditInsert(ctx, r.pool, r.schema, cmd.Actor, "sales_order_settings", nil, "update", postgresinfra.StrPtr("settings"), nil, postgresinfra.StrPtr(cmd.CompanyName), nil)
	}
	return err
}
```

- [ ] **Step 5: Run repository tests**

Run:

```bash
go test ./internal/infrastructure/postgres/sales -run TestSalesOrderSettingsRoundTrip -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/postgres/sales/schema.go internal/infrastructure/postgres/sales/repository.go internal/infrastructure/postgres/sales/sales_order_repository.go internal/infrastructure/postgres/sales/sales_order_repository_test.go
git commit -m "feat: persist sales order settings"
```

## Task 5: Asset Metadata and Upload Persistence

**Files:**
- Modify: `internal/application/sales/service.go`
- Modify: `internal/infrastructure/postgres/sales/repository.go`
- Modify: `internal/infrastructure/postgres/sales/sales_order_repository.go`
- Test: `internal/infrastructure/postgres/sales/sales_order_repository_test.go`

- [ ] **Step 1: Write failing asset test**

Add:

```go
func TestSalesOrderPaymentCodeRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	repo := NewRepository(pool, schema)
	ctx := context.Background()
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	asset, err := repo.SaveSalesOrderAsset(ctx, salesapp.SaveSalesOrderAssetCommand{
		Actor: "测试员", Kind: "payment_code", Filename: "wx.png", ContentType: "image/png", Bytes: 12, SHA256: "abc", ObjectKey: "sales-order/payment/wx.png",
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderAsset: %v", err)
	}
	code, err := repo.SaveSalesOrderPaymentCode(ctx, salesapp.SaveSalesOrderPaymentCodeCommand{
		Actor: "测试员", Label: "微信", Description: "扫码付款", AssetID: asset.ID, Sort: 10, Active: true,
	})
	if err != nil {
		t.Fatalf("SaveSalesOrderPaymentCode: %v", err)
	}
	settings, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if len(settings.PaymentCodes) != 1 || settings.PaymentCodes[0].ID != code.ID || settings.PaymentCodes[0].Label != "微信" {
		t.Fatalf("payment codes = %+v", settings.PaymentCodes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/infrastructure/postgres/sales -run TestSalesOrderPaymentCodeRoundTrip -count=1
```

Expected: FAIL because asset/payment methods are missing.

- [ ] **Step 3: Add application commands**

In `service.go`, add:

```go
type SaveSalesOrderAssetCommand struct {
	Actor       string
	Kind        string
	Filename    string
	ContentType string
	Bytes       int64
	SHA256      string
	ObjectKey   string
}

type SaveSalesOrderPaymentCodeCommand struct {
	Actor       string
	ID          int64
	Label       string
	Description string
	AssetID     int64
	Sort        int
	Active      bool
}
```

Extend repository interface with `SaveSalesOrderAsset`, `SaveSalesOrderPaymentCode`, `SetSalesOrderSealAsset`.

- [ ] **Step 4: Implement repository methods**

Implement `SaveSalesOrderAsset`, `SaveSalesOrderPaymentCode`, and update `LoadSalesOrderSettings` to join active payment codes ordered by `sort,id`.

- [ ] **Step 5: Run test**

```bash
go test ./internal/infrastructure/postgres/sales -run 'TestSalesOrderSettingsRoundTrip|TestSalesOrderPaymentCodeRoundTrip' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/application/sales/service.go internal/infrastructure/postgres/sales/repository.go internal/infrastructure/postgres/sales/sales_order_repository.go internal/infrastructure/postgres/sales/sales_order_repository_test.go
git commit -m "feat: persist sales order payment assets"
```

## Task 6: PDF Renderer Spike and Stable Renderer

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/infrastructure/pdf/sales_order_pdf.go`
- Create: `internal/infrastructure/pdf/sales_order_pdf_test.go`
- Create: `assets/fonts/README.md`
- Modify: `Dockerfile`

- [ ] **Step 1: Write failing PDF smoke test**

Create:

```go
package pdf

import (
	"bytes"
	"testing"

	salesdomain "orderapp/internal/domain/sales"
)

func TestRenderSalesOrderPDF(t *testing.T) {
	renderer := SalesOrderRenderer{}
	b, err := renderer.Render(salesdomain.SalesOrderSnapshot{
		OrderID: 1, OrderNo: "SO-20260430-0008", OrderDate: "2026-04-30",
		CustomerName: "某某咖啡馆", CompanyName: "浅焙作坊咖啡",
		PaymentText: "微信或对公转账", Note: "请密封避光保存",
		Items: []salesdomain.SalesOrderSnapshotItem{{Name: "橘皮乌龙", Spec: "300g", Qty: "2", Unit: "件", UnitPrice: "67.00", LineTotal: "134.00"}},
		TotalAmount: "134.00", Shipping: "0.00", Discount: "0.00", GrandTotal: "134.00",
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		head := b
		if len(head) > 5 {
			head = head[:5]
		}
		t.Fatalf("PDF missing header: %q", head)
	}
	if len(b) < 1000 {
		t.Fatalf("PDF size = %d, want >= 1000", len(b))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/infrastructure/pdf -run TestRenderSalesOrderPDF -count=1
```

Expected: FAIL because renderer package is missing.

- [ ] **Step 3: Add PDF dependency and renderer**

Run:

```bash
go get github.com/jung-kurt/gofpdf@v1.16.2
```

Before selecting the exact dependency version, verify the module is still obtainable with `go list -m -versions github.com/jung-kurt/gofpdf`; if it is not, use the actively available fork with the same UTF-8 font support and record that choice in `assets/fonts/README.md`.

Create a renderer that:

- Creates A4 portrait PDF.
- Uses a packaged UTF-8 Chinese TTF font via `AddUTF8Font`.
- Writes the formal layout selected in the spec.
- Adds images only when asset paths exist.
- Returns bytes from `pdf.Output(&buf)`.

Use a small embedded font helper:

```go
type SalesOrderRenderer struct {
	FontPath string
}
```

Default `FontPath` should be `assets/fonts/NotoSansSC-Regular.ttf`.

- [ ] **Step 4: Add font asset**

Add `assets/fonts/NotoSansSC-Regular.ttf` after verifying the license allows bundling. Add `assets/fonts/README.md` with:

```markdown
# Fonts

`NotoSansSC-Regular.ttf` is bundled for sales-order PDF Chinese rendering.
Source: Google Noto Fonts.
License: SIL Open Font License 1.1.
```

Modify `Dockerfile`:

```dockerfile
COPY assets /app/assets
```

- [ ] **Step 5: Run PDF tests**

```bash
go test ./internal/infrastructure/pdf -count=1
```

Expected: PASS.

- [ ] **Step 6: Run Docker build smoke**

```bash
docker build -t orderapp-sales-order-pdf-test .
```

Expected: build passes and `go test ./...` inside Docker passes.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum Dockerfile assets/fonts internal/infrastructure/pdf
git commit -m "feat: render sales order pdf"
```

## Task 7: Document Snapshot and Version Persistence

**Files:**
- Modify: `internal/application/sales/service.go`
- Modify: `internal/infrastructure/postgres/sales/repository.go`
- Modify: `internal/infrastructure/postgres/sales/sales_order_repository.go`
- Test: `internal/infrastructure/postgres/sales/sales_order_repository_test.go`

- [ ] **Step 1: Write failing document version test**

Add:

```go
func TestGenerateSalesOrderDocumentCreatesVersions(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	repo := NewRepository(pool, schema)
	ctx := context.Background()
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	_ = repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{Actor: "测试员", CompanyName: "浅焙作坊咖啡"})

	first, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate first: %v", err)
	}
	second, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate second: %v", err)
	}
	if first.Document.VersionNo != 1 || second.Document.VersionNo != 2 || !second.Document.IsLatest {
		t.Fatalf("versions first=%+v second=%+v", first.Document, second.Document)
	}
	docs, err := repo.ListSalesOrderDocuments(ctx, 1)
	if err != nil {
		t.Fatalf("ListSalesOrderDocuments: %v", err)
	}
	if len(docs) != 2 || docs[0].VersionNo != 2 || docs[1].VersionNo != 1 {
		t.Fatalf("docs = %+v", docs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/infrastructure/postgres/sales -run TestGenerateSalesOrderDocumentCreatesVersions -count=1
```

Expected: FAIL because generation persistence is missing.

- [ ] **Step 3: Implement snapshot query**

In repository, query order header and items:

```sql
SELECT o.id, o.order_no, to_char(o.order_date,'YYYY-MM-DD'), COALESCE(c.name,''),
       COALESCE(o.total_amount,0), COALESCE(o.shipping_amount,0),
       COALESCE(o.discount_amount,0), COALESCE(o.grand_total,0)
FROM <schema>.orders o
LEFT JOIN <schema>.customers c ON c.id=o.customer_id
WHERE o.id=$1
```

Then query items ordered by `line_no,id` and map to `salesdomain.SalesOrderSnapshotItem`.

- [ ] **Step 4: Implement version transaction**

In `GenerateSalesOrderDocument`:

- Begin transaction.
- Lock documents for order with `FOR UPDATE`.
- Compute next version with `salesdomain.NextSalesOrderVersion`.
- Build snapshot from order plus current settings.
- Render PDF bytes using repository renderer.
- Save PDF as a `sales_order_assets` row and filesystem object key `sales-orders/<order_no>/V<version>.pdf`.
- Set previous documents `is_latest=false`.
- Insert new row with `is_latest=true`.
- Commit.
- Audit `entity_type=sales_order_document`, `action=create`.

- [ ] **Step 5: Run repository tests**

```bash
go test ./internal/infrastructure/postgres/sales -run 'TestGenerateSalesOrderDocumentCreatesVersions|TestSalesOrderSettingsRoundTrip|TestSalesOrderPaymentCodeRoundTrip' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/application/sales/service.go internal/infrastructure/postgres/sales/repository.go internal/infrastructure/postgres/sales/sales_order_repository.go internal/infrastructure/postgres/sales/sales_order_repository_test.go
git commit -m "feat: version sales order documents"
```

## Task 8: HTTP Settings and Document APIs

**Files:**
- Create: `internal/interfaces/http/sales/sales_order_settings.go`
- Create: `internal/interfaces/http/sales/sales_order_documents.go`
- Modify: `internal/interfaces/http/sales/module.go`
- Create: `internal/interfaces/http/sales/sales_order_api_test.go`

- [ ] **Step 1: Write failing API tests**

Create `sales_order_api_test.go`:

```go
func TestSalesOrderSettingsAPI(t *testing.T) {
	e, svc := newSalesOrderAPITestServer(t)
	registerSalesOrderSettingsRoutes(e, svc)
	body := strings.NewReader(`{"company_name":"浅焙作坊咖啡","note":"请密封保存","payment_text":"微信或对公转账"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/sales-order", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSalesOrderDocumentAPI(t *testing.T) {
	e, svc := newSalesOrderAPITestServer(t)
	registerSalesOrderDocumentRoutes(e, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/1/sales-orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/interfaces/http/sales -run 'TestSalesOrderSettingsAPI|TestSalesOrderDocumentAPI' -count=1
```

Expected: FAIL because routes/helpers are missing.

- [ ] **Step 3: Implement route registration**

In `module.go`:

```go
registerSalesOrderSettingsRoutes(e, deps.Sales)
registerSalesOrderDocumentRoutes(e, deps.Sales)
```

`sales_order_settings.go` handlers:

- `GET /settings/sales-order` redirects to Vue view.
- `GET /api/settings/sales-order`
- `POST /api/settings/sales-order`
- `POST /api/settings/sales-order/payment-codes`
- `PUT /api/settings/sales-order/payment-codes/:id`
- `DELETE /api/settings/sales-order/payment-codes/:id`
- `POST /api/settings/sales-order/seal`

`sales_order_documents.go` handlers:

- `GET /orders/:id/sales-order` redirects to Vue view.
- `GET /api/orders/:id/sales-orders`
- `POST /api/orders/:id/sales-orders`
- `GET /orders/:id/sales-orders/:doc_id.pdf`
- `GET /orders/:id/sales-order-latest.pdf`

- [ ] **Step 4: Implement JSON and download responses**

For download:

```go
c.Response().Header().Set(echo.HeaderContentType, "application/pdf")
c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
return c.File(pdfPath)
```

Use service/repository method to resolve document metadata and file path, not direct path params.

- [ ] **Step 5: Run API tests**

```bash
go test ./internal/interfaces/http/sales -run 'TestSalesOrderSettingsAPI|TestSalesOrderDocumentAPI' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/interfaces/http/sales/sales_order_settings.go internal/interfaces/http/sales/sales_order_documents.go internal/interfaces/http/sales/sales_order_api_test.go internal/interfaces/http/sales/module.go
git commit -m "feat: add sales order pdf APIs"
```

## Task 9: Vue Settings and Sales Order Pages

**Files:**
- Modify: `frontend-vue-shell/src/App.vue`
- Modify: `frontend-vue-shell/src/views/OrdersView.vue`
- Create: `frontend-vue-shell/src/views/SalesOrderSettingsView.vue`
- Create: `frontend-vue-shell/src/views/SalesOrderView.vue`
- Create: `frontend-vue-shell/src/lib/sales-order.js`
- Create: `frontend-vue-shell/src/lib/sales-order.test.js`

- [ ] **Step 1: Write failing frontend helper tests**

Create `frontend-vue-shell/src/lib/sales-order.test.js`:

```js
import test from 'node:test'
import assert from 'node:assert/strict'
import { salesOrderPageUrl, salesOrderDownloadUrl } from './sales-order.js'

test('salesOrderPageUrl keeps order id in vue shell', () => {
  assert.equal(salesOrderPageUrl(12), '/vue-shell?view=salesOrder&order_id=12')
})

test('salesOrderDownloadUrl points to latest pdf', () => {
  assert.equal(salesOrderDownloadUrl(12), '/orders/12/sales-order-latest.pdf')
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend-vue-shell
node --test src/lib/sales-order.test.js
```

Expected: FAIL because helper is missing.

- [ ] **Step 3: Implement helper**

Create:

```js
export function salesOrderPageUrl(orderID) {
  return `/vue-shell?view=salesOrder&order_id=${Number(orderID || 0)}`
}

export function salesOrderDownloadUrl(orderID) {
  return `/orders/${Number(orderID || 0)}/sales-order-latest.pdf`
}
```

- [ ] **Step 4: Add Vue pages and menu**

`SalesOrderSettingsView.vue`:

- Loads `/api/settings/sales-order`.
- Saves text settings with `POST /api/settings/sales-order`.
- Uses multipart upload for payment codes and seal.
- Shows active payment code list with enable/disable controls.

`SalesOrderView.vue`:

- Reads `order_id` from URL.
- Loads `/api/orders/:id/sales-orders`.
- Calls `POST /api/orders/:id/sales-orders` to generate.
- Lists versions with download links.

`OrdersView.vue` operation cell:

```vue
<a class="text-link" :href="salesOrderPageUrl(row.id)">销售单</a>
```

`App.vue`:

- Import the two views.
- Add `salesOrder` and `salesOrderSettings` to `menuMap/internalViews`.
- Add `销售单设置` under settings menu.

- [ ] **Step 5: Run frontend tests/build**

```bash
cd frontend-vue-shell
node --test src/lib/sales-order.test.js
npm run build
```

Expected: helper tests and build pass.

- [ ] **Step 6: Commit**

```bash
git add frontend-vue-shell/src/App.vue frontend-vue-shell/src/views/OrdersView.vue frontend-vue-shell/src/views/SalesOrderSettingsView.vue frontend-vue-shell/src/views/SalesOrderView.vue frontend-vue-shell/src/lib/sales-order.js frontend-vue-shell/src/lib/sales-order.test.js
git commit -m "feat: add sales order pdf vue pages"
```

## Task 10: Full Verification and Integration

**Files:**
- All changed files.

- [ ] **Step 1: Run full backend tests**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 2: Run frontend tests**

```bash
cd frontend-vue-shell
node --test src/lib/*.test.js
```

Expected: all frontend helper tests pass.

- [ ] **Step 3: Run frontend build**

```bash
cd frontend-vue-shell
npm run build
```

Expected: Vite build succeeds.

- [ ] **Step 4: Run Docker build**

```bash
docker build -t orderapp-sales-order-pdf-test .
```

Expected: Docker build succeeds and internal `go test ./...` succeeds.

- [ ] **Step 5: Review requirement table evidence**

Update seeded rows or runtime rows so the 5 requirement tables show:

- `PR-SALES-ORDER-001`: status `done`, evidence references spec, generated PDF, and commit.
- `DEV-SALES-ORDER-001`: status `done`, evidence references changed files.
- `UT-SALES-ORDER-001`: status `done`, evidence includes `go test ./...` and frontend node tests.
- `API-SALES-ORDER-001`: status `done`, evidence includes sales order API tests.
- `REV-SALES-ORDER-001`: status `done`, evidence includes acceptance checklist.

- [ ] **Step 6: Push feature branch**

```bash
git status --short
git push -u origin codex/sales-order-pdf-20260430
```

Expected: feature branch is pushed.

- [ ] **Step 7: Merge latest develop into feature branch**

```bash
git fetch origin develop
git merge origin/develop
go test ./...
cd frontend-vue-shell && npm run build
```

Expected: clean merge and verification passes.

- [ ] **Step 8: Merge into develop and deploy**

```bash
git switch -c codex/integrate-sales-order-pdf-20260430 origin/develop
git merge --no-ff codex/sales-order-pdf-20260430 -m "Merge sales order pdf generation"
git push origin HEAD:develop
```

Then use the existing KFerp deployment workflow:

```bash
cd orderapp-remote
npm --prefix frontend-vue-shell run build
npm --prefix frontend run build
tar --exclude='./frontend/node_modules' --exclude='./frontend-vue-shell/node_modules' --exclude='./.env*' -czf - . | ssh root@1.12.242.58 "mkdir -p /opt/stacks/erp/orderapp && tar -xzf - -C /opt/stacks/erp/orderapp"
ssh root@1.12.242.58 "cd /opt/stacks/erp && docker compose build orderapp && docker compose up -d orderapp"
```

Expected: server container starts, `/app/vue-shell?view=salesOrderSettings` returns 200 after authentication, and one test order can generate/download PDF.
