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
	if err := repo.SaveSalesOrderSettings(ctx, salesapp.SaveSalesOrderSettingsCommand{Actor: "测试员", CompanyName: "浅焙作坊咖啡", Note: "请密封保存"}); err != nil {
		t.Fatalf("SaveSalesOrderSettings: %v", err)
	}

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
		fmt.Sprintf(`CREATE TABLE %s.products (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders (
			id BIGSERIAL PRIMARY KEY,
			order_no TEXT NOT NULL DEFAULT '',
			order_date DATE,
			customer_id BIGINT,
			total_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			shipping_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
			grand_total NUMERIC(12,2) NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			line_no INTEGER NOT NULL DEFAULT 0,
			product_id BIGINT,
			item_name TEXT NOT NULL DEFAULT '',
			spec TEXT NOT NULL DEFAULT '',
			qty NUMERIC(12,2) NOT NULL DEFAULT 0,
			unit TEXT NOT NULL DEFAULT '',
			unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
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
