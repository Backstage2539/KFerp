package sales

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"
	pdfinfra "orderapp/internal/infrastructure/pdf"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSalesOrderSettingsRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)

	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存", PaymentText: "微信或对公转账",
		BankAccountName: "孟连口加农业科技有限公司", BankName: "中国农业银行孟连支行", BankAccountNo: "6222000000000000",
		PaymentTextXMM: 18, PaymentTextYMM: 142, PaymentTextWidthMM: 98, PaymentTextHeightMM: 54,
		PaymentCodeXMM: 126, PaymentCodeYMM: 104, PaymentCodeWidthMM: 76, PaymentCodeHeightMM: 126,
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
	if got.BankAccountName != "孟连口加农业科技有限公司" || got.BankName != "中国农业银行孟连支行" || got.BankAccountNo != "6222000000000000" {
		t.Fatalf("bank account settings = %+v", got)
	}
	if got.PaymentTextXMM != 18 || got.PaymentTextYMM != 142 || got.PaymentTextWidthMM != 98 || got.PaymentTextHeightMM != 54 {
		t.Fatalf("payment text layout settings = %+v", got)
	}
	if got.PaymentCodeXMM != 126 || got.PaymentCodeYMM != 104 || got.PaymentCodeWidthMM != 76 || got.PaymentCodeHeightMM != 126 {
		t.Fatalf("payment code layout settings = %+v", got)
	}
}

func TestSalesOrderPaymentCodeRoundTrip(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
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
	if len(settings.PaymentCodes) != 1 || settings.PaymentCodes[0].ID != code.ID || settings.PaymentCodes[0].Label != "微信" || settings.PaymentCodes[0].Asset.ObjectKey != "sales-order/payment/wx.png" {
		t.Fatalf("payment codes = %+v", settings.PaymentCodes)
	}
}

func TestDeactivateSalesOrderPaymentCodeHidesWithoutDeletingRecordOrAsset(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema)
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

	if err := repo.DeactivateSalesOrderPaymentCode(ctx, code.ID, "测试员"); err != nil {
		t.Fatalf("DeactivateSalesOrderPaymentCode: %v", err)
	}

	settings, err := repo.LoadSalesOrderSettings(ctx)
	if err != nil {
		t.Fatalf("LoadSalesOrderSettings: %v", err)
	}
	if len(settings.PaymentCodes) != 0 {
		t.Fatalf("inactive payment code should be hidden from settings, got %+v", settings.PaymentCodes)
	}
	var codeRows, assetRows int
	var active bool
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int, COALESCE(bool_or(active), false) FROM %s.sales_order_payment_codes WHERE id=$1`, schema), code.ID).Scan(&codeRows, &active); err != nil {
		t.Fatalf("query payment code row: %v", err)
	}
	if codeRows != 1 || active {
		t.Fatalf("payment code row count/active = %d/%v, want 1/false", codeRows, active)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*)::int FROM %s.sales_order_assets WHERE id=$1`, schema), asset.ID).Scan(&assetRows); err != nil {
		t.Fatalf("query payment code asset row: %v", err)
	}
	if assetRows != 1 {
		t.Fatalf("payment code asset rows = %d, want 1", assetRows)
	}
	var action string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT action FROM %s.audit_logs WHERE entity_type='sales_order_payment_code' AND entity_id=$1 AND field='active' ORDER BY id DESC LIMIT 1`, schema), code.ID).Scan(&action); err != nil {
		t.Fatalf("query payment code audit action: %v", err)
	}
	if action != "deactivate" {
		t.Fatalf("payment code audit action = %q, want deactivate", action)
	}
}

func TestGenerateSalesOrderDocumentCreatesVersions(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address, taxpayer_id, bank_account_name, bank_name, bank_account_no) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县', '91530827MACGJ29D6J', '孟连口加农业科技有限公司', '中国农业银行孟连支行', '6222000000000000')`, schema)); err != nil {
		t.Fatalf("insert company profile: %v", err)
	}

	first, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate first: %v", err)
	}
	if first.Snapshot.CompanyName != "棵凡咖啡" {
		t.Fatalf("generated snapshot company_name = %q, want global company profile name", first.Snapshot.CompanyName)
	}
	if first.Snapshot.BankAccountName != "孟连口加农业科技有限公司" || first.Snapshot.BankName != "中国农业银行孟连支行" || first.Snapshot.BankAccountNo != "6222000000000000" {
		t.Fatalf("generated snapshot bank account fields = %+v", first.Snapshot)
	}
	if first.Snapshot.CompanyAddress != "云南省普洱市孟连县" || first.Snapshot.TaxpayerID != "91530827MACGJ29D6J" {
		t.Fatalf("generated snapshot company account identity fields = %+v", first.Snapshot)
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
	if len(docs) != 2 || docs[0].VersionNo != 2 || !docs[0].IsLatest || docs[1].VersionNo != 1 || docs[1].IsLatest {
		t.Fatalf("docs = %+v", docs)
	}
	file, err := repo.LoadSalesOrderDocumentFile(ctx, 1, 0, true)
	if err != nil {
		t.Fatalf("LoadSalesOrderDocumentFile latest: %v", err)
	}
	b, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if string(b) != "%PDF-test" || file.Filename != "SO-20260430-0008-V2.pdf" {
		t.Fatalf("file=%+v bytes=%q", file, b)
	}
}

func TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(t.TempDir()), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET total_amount=2455, shipping_amount=169, discount_amount=261.65, grand_total=2362.35 WHERE id=1`, schema)); err != nil {
		t.Fatalf("update order amounts: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.order_items SET discount_type='unit_amount', discount_amount=100 WHERE order_id=1 AND line_no=1`, schema)); err != nil {
		t.Fatalf("update first item discount: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, spec, qty, unit, unit_price, discount_type, discount_amount, line_total)
		VALUES(1, 2, 1, '芬纳-曲奇定制（20%%乌干达，15%%云南厌氧日晒，65%%云南水洗）', '1000g', 2, '件', 117, 'percent', 61.65, 93.35)`, schema)); err != nil {
		t.Fatalf("insert discounted item: %v", err)
	}

	if err := repo.SaveSalesOrderNote(ctx, salesapp.SaveSalesOrderNoteCommand{Actor: "销售", OrderID: 1, Note: "  末行备注：随货附赠杯测样  "}); err != nil {
		t.Fatalf("SaveSalesOrderNote: %v", err)
	}
	preview, err := repo.PreviewSalesOrderDocument(ctx, 1)
	if err != nil {
		t.Fatalf("PreviewSalesOrderDocument: %v", err)
	}
	if preview.Snapshot.SalesOrderNote != "末行备注：随货附赠杯测样" || preview.Snapshot.Shipping != "169.00" || preview.Snapshot.Discount != "261.65" {
		t.Fatalf("snapshot financial fields = %+v", preview.Snapshot)
	}
	want := []salesdomain.SalesOrderDiscountBreakdown{
		{Type: "unit_amount", Amount: "100.00"},
		{Type: "percent", Amount: "61.65"},
		{Type: "order_amount", Amount: "100.00"},
	}
	if fmt.Sprint(preview.Snapshot.DiscountBreakdowns) != fmt.Sprint(want) {
		t.Fatalf("discount breakdowns = %+v, want %+v", preview.Snapshot.DiscountBreakdowns, want)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='order' AND entity_id=1 AND field='sales_order_note' AND old_value='' AND new_value='末行备注：随货附赠杯测样'`, schema)).Scan(&auditCount); err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("sales_order_note audit count = %d, want 1", auditCount)
	}
}

func TestGenerateSalesOrderImageCreatesIndependentImageVersions(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}

	pdf, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate PDF: %v", err)
	}
	first, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate first image: %v", err)
	}
	second, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("Generate second image: %v", err)
	}
	if first.Document.VersionNo != 1 || second.Document.VersionNo != 2 || !second.Document.IsLatest {
		t.Fatalf("image versions first=%+v second=%+v", first.Document, second.Document)
	}
	if pdfDocs, err := repo.ListSalesOrderDocuments(ctx, 1); err != nil || len(pdfDocs) != 1 || !pdfDocs[0].IsLatest || pdfDocs[0].ID != pdf.Document.ID {
		t.Fatalf("PDF latest should remain independent of image generation, docs=%+v err=%v", pdfDocs, err)
	}
	images, err := repo.ListSalesOrderImageDocuments(ctx, 1)
	if err != nil {
		t.Fatalf("ListSalesOrderImageDocuments: %v", err)
	}
	if len(images) != 2 || images[0].VersionNo != 2 || !images[0].IsLatest || images[0].DownloadURL == "" || images[1].IsLatest {
		t.Fatalf("images = %+v", images)
	}
	file, err := repo.LoadSalesOrderImageFile(ctx, 1, 0, true)
	if err != nil {
		t.Fatalf("LoadSalesOrderImageFile latest: %v", err)
	}
	b, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("read png: %v", err)
	}
	if string(b) != "\x89PNG\r\n\x1a\nimage-test" || file.Filename != "SO-20260430-0008-V2.png" {
		t.Fatalf("file=%+v bytes=%q", file, b)
	}
}

func TestGenerateSalesOrderDocumentCleansFileWhenDocumentInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.sales_order_documents ADD CONSTRAINT sales_order_document_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateSalesOrderDocument(ctx, salesapp.GenerateSalesOrderDocumentCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateSalesOrderDocument should fail when document insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
}

func TestGenerateSalesOrderImageCleansFileWhenImageInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir), WithSalesOrderRenderer(fakeSalesOrderRenderer{}))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedSalesOrderDocumentOrder(t, ctx, pool, schema)
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{
		Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存",
	}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.sales_order_images ADD CONSTRAINT sales_order_image_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateSalesOrderImage(ctx, salesapp.GenerateSalesOrderImageCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateSalesOrderImage should fail when image insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
}

func TestNewRepositoryPassesAssetDirToDefaultSalesOrderRenderer(t *testing.T) {
	dir := t.TempDir()
	repo := NewRepository(nil, "public", WithSalesOrderAssetDir(dir))
	renderer, ok := repo.renderer.(pdfinfra.SalesOrderRenderer)
	if !ok {
		t.Fatalf("renderer type = %T, want pdf.SalesOrderRenderer", repo.renderer)
	}
	if renderer.AssetBaseDir != dir {
		t.Fatalf("renderer AssetBaseDir = %q, want %q", renderer.AssetBaseDir, dir)
	}
}

type fakeSalesOrderRenderer struct{}

func (fakeSalesOrderRenderer) Render(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-test"), nil
}

func (fakeSalesOrderRenderer) RenderPreview(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-preview-test"), nil
}

func (fakeSalesOrderRenderer) RenderPNG(snapshot salesdomain.SalesOrderSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("\x89PNG\r\n\x1a\nimage-test"), nil
}

func newSalesPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for sales postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_sales_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func prepareSalesSchemaPrerequisites(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE %s.order_process_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, sort INTEGER NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT true)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.ship_statuses (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customers (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			company_name TEXT NOT NULL DEFAULT '',
			company_address TEXT NOT NULL DEFAULT '',
			company_phone TEXT NOT NULL DEFAULT '',
			contact TEXT NOT NULL DEFAULT '',
			phone TEXT NOT NULL DEFAULT '',
			address TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.company_profile (
			id INTEGER PRIMARY KEY DEFAULT 1,
			company_name TEXT NOT NULL DEFAULT '',
			company_address TEXT NOT NULL DEFAULT '',
			company_phone TEXT NOT NULL DEFAULT '',
			taxpayer_id TEXT NOT NULL DEFAULT '',
			bank_account_name TEXT NOT NULL DEFAULT '',
			bank_name TEXT NOT NULL DEFAULT '',
			bank_account_no TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_by TEXT NOT NULL DEFAULT '',
			CONSTRAINT company_profile_singleton CHECK (id = 1)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.products (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT NOT NULL DEFAULT '',
			order_date DATE,
			customer_id BIGINT,
			total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			sales_order_note TEXT NOT NULL DEFAULT '',
			grand_total NUMERIC(12,2) NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			line_no INTEGER NOT NULL DEFAULT 0,
			product_id BIGINT,
			item_name TEXT NOT NULL DEFAULT '',
			item_note TEXT NOT NULL DEFAULT '',
			spec TEXT NOT NULL DEFAULT '',
			qty NUMERIC(12,2) NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '',
			unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
			discount_type TEXT NOT NULL DEFAULT '',
			discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			line_total NUMERIC(12,2) NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.audit_logs (
			id BIGSERIAL PRIMARY KEY,
			ts TIMESTAMPTZ NOT NULL DEFAULT now(),
			actor TEXT NOT NULL DEFAULT '',
			entity_type TEXT NOT NULL DEFAULT '',
			entity_id BIGINT,
			action TEXT NOT NULL DEFAULT '',
			field TEXT,
			old_value TEXT,
			new_value TEXT,
			meta JSONB
		)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare schema: %v", err)
		}
	}
}

func assertSalesPostgresAssetDirEmpty(t *testing.T, assetDir string) {
	t.Helper()
	entries, err := os.ReadDir(assetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("asset dir should be empty after failed sales order generation, entries=%+v", entries)
	}
}

func seedSalesOrderDocumentOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	stmts := []string{
		fmt.Sprintf(`INSERT INTO %s.customers(id, name, company_name, company_address, company_phone) VALUES(1, '某某咖啡馆', '某某咖啡贸易公司', '上海市徐汇区', '021-12345678')`, schema),
		fmt.Sprintf(`INSERT INTO %s.products(id, name) VALUES(1, '橘皮乌龙')`, schema),
		fmt.Sprintf(`INSERT INTO %s.orders(id, order_no, order_date, customer_id, total_amount, shipping_amount, discount_amount, grand_total)
			VALUES(1, 'SO-20260430-0008', '2026-04-30', 1, 134, 0, 0, 134)`, schema),
		fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, spec, qty, unit, unit_price, line_total)
			VALUES(1, 1, 1, '橘皮乌龙', '300g', 2, '件', 67, 134)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed order: %v", err)
		}
	}
}
