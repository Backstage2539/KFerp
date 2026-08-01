package sales

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	salesapp "orderapp/internal/application/sales"
	salesdomain "orderapp/internal/domain/sales"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDeliveryMethodDisplayNameHidesInternalCodes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "sf_small", want: "顺丰发货"},
		{in: "sf_large", want: "顺丰大件"},
		{in: "sf_express", want: "顺丰标快"},
		{in: "顺丰冷运", want: "顺丰冷运"},
	} {
		if got := deliveryMethodDisplayName(tc.in); got != tc.want {
			t.Fatalf("deliveryMethodDisplayName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGenerateDeliveryNoteDocumentCleansFileWhenDocumentInsertFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir))
	repo.deliveryNoteRenderer = fakeDeliveryNoteRenderer{}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.delivery_note_documents ADD CONSTRAINT delivery_note_document_test_reject_latest CHECK (is_latest = false)`, schema)); err != nil {
		t.Fatalf("add reject latest constraint: %v", err)
	}

	if _, err := repo.GenerateDeliveryNoteDocument(ctx, salesapp.GenerateDeliveryNoteDocumentCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatalf("GenerateDeliveryNoteDocument should fail when document insert is rejected")
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
	var assetRows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.delivery_note_assets`, schema)).Scan(&assetRows); err != nil {
		t.Fatalf("count rolled back delivery note assets: %v", err)
	}
	if assetRows != 0 {
		t.Fatalf("delivery note asset rows after rollback = %d, want 0", assetRows)
	}
}

func TestGenerateDeliveryNoteDocumentCleansPartialFilesWhenImageWriteFails(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir))
	repo.deliveryNoteRenderer = fakeDeliveryNoteRenderer{}
	writeCalls := 0
	repo.deliveryNoteAssetWriter = func(path string, data []byte) error {
		writeCalls++
		if writeCalls == 1 {
			return os.WriteFile(path, data, 0644)
		}
		partial := data
		if len(partial) > 8 {
			partial = partial[:8]
		}
		if err := os.WriteFile(path, partial, 0644); err != nil {
			return err
		}
		return errors.New("injected image write failure after partial file creation")
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)

	if _, err := repo.GenerateDeliveryNoteDocument(ctx, salesapp.GenerateDeliveryNoteDocumentCommand{Actor: "测试员", OrderID: 1}); err == nil {
		t.Fatal("GenerateDeliveryNoteDocument should fail after partial image write")
	}
	if writeCalls != 2 {
		t.Fatalf("asset write calls = %d, want 2", writeCalls)
	}
	assertSalesPostgresAssetDirEmpty(t, assetDir)
	var assetRows, documentRows, auditRows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.delivery_note_assets`, schema)).Scan(&assetRows); err != nil {
		t.Fatalf("count rolled back assets: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.delivery_note_documents`, schema)).Scan(&documentRows); err != nil {
		t.Fatalf("count rolled back documents: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='delivery_note_document'`, schema)).Scan(&auditRows); err != nil {
		t.Fatalf("count rolled back audits: %v", err)
	}
	if assetRows != 0 || documentRows != 0 || auditRows != 0 {
		t.Fatalf("rows after partial write failure assets=%d documents=%d audits=%d", assetRows, documentRows, auditRows)
	}
}

func TestGenerateDeliveryNoteDocumentCreatesPairedPDFAndPNGAssets(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir))
	repo.deliveryNoteRenderer = fakeDeliveryNoteRenderer{}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)

	result, err := repo.GenerateDeliveryNoteDocument(ctx, salesapp.GenerateDeliveryNoteDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("GenerateDeliveryNoteDocument: %v", err)
	}
	doc := result.Document
	if doc.PDFAssetID <= 0 || doc.ImageAssetID <= 0 || doc.PDFAssetID == doc.ImageAssetID {
		t.Fatalf("paired asset ids = pdf:%d image:%d", doc.PDFAssetID, doc.ImageAssetID)
	}
	if doc.DownloadURL == "" || doc.ImageDownloadURL == "" {
		t.Fatalf("paired download URLs missing: %+v", doc)
	}
	documents, err := repo.ListDeliveryNoteDocuments(ctx, 1)
	if err != nil {
		t.Fatalf("ListDeliveryNoteDocuments: %v", err)
	}
	if len(documents) != 1 || documents[0].ImageAssetID != doc.ImageAssetID || documents[0].ImageDownloadURL != doc.ImageDownloadURL {
		t.Fatalf("listed paired document = %+v, generated = %+v", documents, doc)
	}

	pdfFile, err := repo.LoadDeliveryNoteDocumentFile(ctx, 1, doc.ID, false)
	if err != nil {
		t.Fatalf("LoadDeliveryNoteDocumentFile: %v", err)
	}
	imageFile, err := repo.LoadDeliveryNoteImageFile(ctx, 1, doc.ID, false)
	if err != nil {
		t.Fatalf("LoadDeliveryNoteImageFile: %v", err)
	}
	latestImage, err := repo.LoadDeliveryNoteImageFile(ctx, 1, 0, true)
	if err != nil {
		t.Fatalf("LoadDeliveryNoteImageFile latest: %v", err)
	}
	if imageFile.Document.ID != doc.ID || latestImage.Document.ID != doc.ID || imageFile.Filename != "SO-20260513-DN01-DN-V1.png" {
		t.Fatalf("image files = explicit:%+v latest:%+v", imageFile, latestImage)
	}
	pdfBytes, err := os.ReadFile(pdfFile.Path)
	if err != nil {
		t.Fatalf("read generated PDF: %v", err)
	}
	imageBytes, err := os.ReadFile(imageFile.Path)
	if err != nil {
		t.Fatalf("read generated PNG: %v", err)
	}
	if string(pdfBytes) != "%PDF-delivery-test" || string(imageBytes) != "\x89PNG\r\n\x1a\ndelivery-image-test" {
		t.Fatalf("generated bytes pdf=%q image=%q", pdfBytes, imageBytes)
	}

	var pdfAssets, imageAssets, auditRows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER (WHERE kind='delivery_note_pdf'), count(*) FILTER (WHERE kind='delivery_note_image') FROM %s.delivery_note_assets`, schema)).Scan(&pdfAssets, &imageAssets); err != nil {
		t.Fatalf("count paired assets: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='delivery_note_document' AND entity_id=$1 AND action='create'`, schema), doc.ID).Scan(&auditRows); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if pdfAssets != 1 || imageAssets != 1 || auditRows != 1 {
		t.Fatalf("asset/audit counts pdf=%d image=%d audit=%d", pdfAssets, imageAssets, auditRows)
	}
}

func TestLoadDeliveryNoteImageFileReturnsNotFoundForLegacyVersionWithoutImage(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	prepareDeliveryNoteGenerationOrderColumns(t, ctx, pool, schema)
	assetDir := t.TempDir()
	repo := NewRepository(pool, schema, WithSalesOrderAssetDir(assetDir))
	repo.deliveryNoteRenderer = fakeDeliveryNoteRenderer{}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	seedDeliveryNoteGenerationOrder(t, ctx, pool, schema)
	result, err := repo.GenerateDeliveryNoteDocument(ctx, salesapp.GenerateDeliveryNoteDocumentCommand{Actor: "测试员", OrderID: 1})
	if err != nil {
		t.Fatalf("GenerateDeliveryNoteDocument: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.delivery_note_documents SET image_asset_id=NULL WHERE id=$1`, schema), result.Document.ID); err != nil {
		t.Fatalf("make legacy document: %v", err)
	}

	_, err = repo.LoadDeliveryNoteImageFile(ctx, 1, result.Document.ID, false)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("LoadDeliveryNoteImageFile legacy error = %v, want pgx.ErrNoRows", err)
	}
	_, err = repo.LoadDeliveryNoteImageFile(ctx, 1, 0, true)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("LoadDeliveryNoteImageFile latest legacy error = %v, want pgx.ErrNoRows", err)
	}
}

func TestEnsureDeliveryNoteTablesAddsImageAssetIDIdempotently(t *testing.T) {
	pool, schema := newSalesPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	prepareSalesSchemaPrerequisites(t, ctx, pool, schema)
	for _, stmt := range []string{
		fmt.Sprintf(`CREATE TABLE %s.delivery_note_assets (id BIGSERIAL PRIMARY KEY, kind TEXT NOT NULL, filename TEXT NOT NULL DEFAULT '', content_type TEXT NOT NULL DEFAULT '', bytes BIGINT NOT NULL DEFAULT 0, sha256 TEXT NOT NULL DEFAULT '', object_key TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), created_by TEXT NOT NULL DEFAULT '')`, schema),
		fmt.Sprintf(`CREATE TABLE %s.delivery_note_documents (id BIGSERIAL PRIMARY KEY, order_id BIGINT NOT NULL REFERENCES %s.orders(id), order_no TEXT NOT NULL DEFAULT '', version_no INTEGER NOT NULL, snapshot_json JSONB NOT NULL, pdf_asset_id BIGINT REFERENCES %s.delivery_note_assets(id), is_latest BOOLEAN NOT NULL DEFAULT true, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), created_by TEXT NOT NULL DEFAULT '', UNIQUE(order_id, version_no))`, schema, schema, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create legacy delivery note schema: %v", err)
		}
	}
	if err := ensureDeliveryNoteTables(ctx, pool, schema); err != nil {
		t.Fatalf("first ensureDeliveryNoteTables: %v", err)
	}
	if err := ensureDeliveryNoteTables(ctx, pool, schema); err != nil {
		t.Fatalf("second ensureDeliveryNoteTables: %v", err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='delivery_note_documents' AND column_name='image_asset_id')`, schema).Scan(&exists); err != nil {
		t.Fatalf("check image_asset_id: %v", err)
	}
	if !exists {
		t.Fatal("delivery_note_documents.image_asset_id was not added")
	}
}

type fakeDeliveryNoteRenderer struct{}

func (fakeDeliveryNoteRenderer) Render(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-delivery-test"), nil
}

func (fakeDeliveryNoteRenderer) RenderPreview(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("%PDF-delivery-preview-test"), nil
}

func (fakeDeliveryNoteRenderer) RenderPNG(snapshot salesdomain.DeliveryNoteSnapshot) ([]byte, error) {
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	return []byte("\x89PNG\r\n\x1a\ndelivery-image-test"), nil
}

func prepareDeliveryNoteGenerationOrderColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	for _, stmt := range []string{
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_status_id BIGINT NOT NULL DEFAULT 0`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS ship_method TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_name TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_phone TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS receiver_address TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS source_warehouse TEXT NOT NULL DEFAULT ''`, schema),
		fmt.Sprintf(`ALTER TABLE %s.orders ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`, schema),
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("prepare delivery note order columns: %v", err)
		}
	}
}

func seedDeliveryNoteGenerationOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema string) {
	t.Helper()
	var shipStatusID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.ship_statuses WHERE name='已发货' ORDER BY id LIMIT 1`, schema)).Scan(&shipStatusID); err != nil {
		t.Fatalf("query shipped status: %v", err)
	}
	stmts := []string{
		fmt.Sprintf(`INSERT INTO %s.company_profile(id, company_name, company_address) VALUES(1, '棵凡咖啡', '云南省普洱市孟连县')`, schema),
		fmt.Sprintf(`INSERT INTO %s.customers(id, name, company_name, company_address, company_phone, contact, phone, address) VALUES(1, '某某咖啡馆', '某某咖啡贸易公司', '上海市徐汇区', '021-12345678', '张三', '13800000000', '上海市徐汇区')`, schema),
		fmt.Sprintf(`INSERT INTO %s.products(id, name) VALUES(1, '橘皮乌龙')`, schema),
		fmt.Sprintf(`INSERT INTO %s.orders(id, order_no, order_date, customer_id, ship_status_id, ship_method, ship_tracking_no, receiver_name, receiver_phone, receiver_address, source_warehouse, notes, total_amount, shipping_amount, discount_amount, grand_total)
			VALUES(1, 'SO-20260513-DN01', '2026-05-13', 1, %d, '顺丰', 'SF123456789', '张三', '13800000000', '上海市徐汇区', 'finished_goods', '随货附出库单', 134, 0, 0, 134)`, schema, shipStatusID),
		fmt.Sprintf(`INSERT INTO %s.order_items(order_id, line_no, product_id, item_name, item_note, spec, qty, unit, unit_price, line_total)
			VALUES(1, 1, 1, '橘皮乌龙', '发货备注', '300g', 2, '件', 67, 134)`, schema),
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed delivery note order: %v", err)
		}
	}
}
