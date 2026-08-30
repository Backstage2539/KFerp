package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	inventoryapp "orderapp/internal/application/inventory"
	postgresinventory "orderapp/internal/infrastructure/postgres/inventory"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestFinishedInventoryAdjustAPIWritesLedgerAndAudit(t *testing.T) {
	pool, schema := newInventoryAPITestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecInventoryAPISQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,
	item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',
	source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,
	qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,
	qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,
	actor TEXT NOT NULL DEFAULT '',
	entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',
	field TEXT,
	old_value TEXT,
	new_value TEXT,
	meta JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.products(id,name,active) VALUES (1,'橘皮乌龙',true);
`, schema, schema, schema, schema, schema, schema))
	if err := postgresinventory.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("inventory EnsureSchema: %v", err)
	}
	if err := postgresinventory.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("inventory EnsureSchema second pass: %v", err)
	}

	e := echo.New()
	registerFinishedInventoryPages(e, inventoryapp.NewService(postgresinventory.NewRepository(pool, schema)))
	req := httptest.NewRequest(http.MethodPost, "/api/products/inventory", bytes.NewBufferString(`{"product_id":1,"spec_g":454,"units":1,"loose_g":500}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/products/inventory status=%d body=%s", rec.Code, rec.Body.String())
	}

	var units, looseG, ledgerCount, batchCount, auditCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=1 AND spec_g=454`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatal(err)
	}
	if units != 2 || looseG != 46 {
		t.Fatalf("inventory = %d units + %dg, want 2 units + 46g", units, looseG)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id=1 AND spec_g=454 AND source_doc_type='manual_adjustment'`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE item_id=1 AND spec_g=454 AND item_type='finished_product'`, schema)).Scan(&batchCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='finished_inventory' AND action='adjust'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || batchCount != 1 || auditCount != 1 {
		t.Fatalf("ledger=%d batch=%d audit=%d, want 1/1/1", ledgerCount, batchCount, auditCount)
	}
}

func TestFinishedInventoryBOMSpecAdjustmentPersistsCanonicalIdentityAndWholeUnits(t *testing.T) {
	pool, schema := newInventoryAPITestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecInventoryAPISQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.products (
	id BIGINT PRIMARY KEY,name TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true,
	auto_derived_sku BOOLEAN NOT NULL DEFAULT false,derived_spec_status TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %[1]s.production_bom_versions (
	id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL
);
CREATE TABLE %[1]s.production_bom_output_bindings (
	output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,
	bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %[1]s.production_bom_specs (
	id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL
);
CREATE TABLE %[1]s.production_bom_version_variants (
	id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,
	spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL
);
CREATE TABLE %[1]s.product_bom_spec_migrations (
	product_id BIGINT PRIMARY KEY,state TEXT NOT NULL
);
CREATE TABLE %[1]s.stock_batches (
	id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,item_name TEXT NOT NULL DEFAULT '',bom_spec_id BIGINT NOT NULL DEFAULT 0,
	bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,source_batch_id TEXT NOT NULL DEFAULT '',qty_g BIGINT NOT NULL DEFAULT 0,
	qty_units BIGINT NOT NULL DEFAULT 0,operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
	spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT '',source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,source_batch_code TEXT NOT NULL DEFAULT '',source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,qty_change_g BIGINT NOT NULL DEFAULT 0,qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,qty_change_units BIGINT NOT NULL DEFAULT 0,qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %[1]s.products(id,name) VALUES(7,'规格组商品');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,status) VALUES(41,31,'published');
INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
VALUES('product',7,31,41,true);
INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(91,31,'bag-227','227g袋','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(191,41,91,'227g袋','袋',true,10);
INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES(7,'cutover');
`, schema))
	if err := postgresinventory.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("inventory EnsureSchema: %v", err)
	}

	e := echo.New()
	registerFinishedInventoryPages(e, inventoryapp.NewService(postgresinventory.NewRepository(pool, schema)))
	req := httptest.NewRequest(http.MethodPost, "/api/products/inventory", bytes.NewBufferString(`{"product_id":7,"bom_spec_id":91,"unit_code":"袋","units":12}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST canonical inventory status=%d body=%s", rec.Code, rec.Body.String())
	}

	var units, looseG, batchSpecID, ledgerSpecID, auditCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=91 AND bom_variant_id=191 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&units, &looseG); err != nil {
		t.Fatal(err)
	}
	if units != 12 || looseG != 0 {
		t.Fatalf("canonical inventory=%d units + %dg", units, looseG)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %s.stock_batches WHERE item_id=7`, schema)).Scan(&batchSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %s.stock_ledger_entries WHERE item_id=7`, schema)).Scan(&ledgerSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='finished_inventory' AND meta->>'bom_spec_id'='91'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if batchSpecID != 91 || ledgerSpecID != 91 || auditCount != 1 {
		t.Fatalf("batch spec=%d ledger spec=%d audit=%d", batchSpecID, ledgerSpecID, auditCount)
	}

	mustExecInventoryAPISQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=41;
INSERT INTO %[1]s.production_bom_versions(id,bom_id,status) VALUES(42,31,'published');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(291,42,91,'227g袋 V2','袋',true,10);
UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=42 WHERE output_type='product' AND output_id=7;
`, schema))
	req = httptest.NewRequest(http.MethodPost, "/api/products/inventory", bytes.NewBufferString(`{"product_id":7,"bom_spec_id":91,"unit_code":"袋","units":15}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST V2 canonical inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	var currentVariantID, latestLedgerVariantID, latestAuditVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=91 AND spec_g=0 AND warehouse='finished_goods'`, schema)).Scan(&currentVariantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.stock_ledger_entries WHERE item_id=7 AND bom_spec_id=91 ORDER BY id DESC LIMIT 1`, schema)).Scan(&latestLedgerVariantID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE((meta->>'bom_variant_id')::bigint,0) FROM %s.audit_logs WHERE entity_type='finished_inventory' ORDER BY id DESC LIMIT 1`, schema)).Scan(&latestAuditVariantID); err != nil {
		t.Fatal(err)
	}
	if currentVariantID != 291 || latestLedgerVariantID != 291 || latestAuditVariantID != 291 {
		t.Fatalf("V2 identity inventory/ledger/audit=%d/%d/%d, want 291", currentVariantID, latestLedgerVariantID, latestAuditVariantID)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/products/inventory", bytes.NewBufferString(`{"product_id":7,"bom_spec_id":91,"bom_variant_id":191,"unit_code":"袋","units":16}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "current default published BOM") {
		t.Fatalf("POST stale canonical inventory status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/products/inventory", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET canonical inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Rows     []inventoryapp.FinishedInventoryRow `json:"rows"`
		Products []inventoryapp.ProductOption        `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Rows) != 1 || payload.Rows[0].BomSpecID != 91 || payload.Rows[0].BomVariantID != 291 || payload.Rows[0].InventoryUnit != "袋" || payload.Rows[0].Units != 15 {
		t.Fatalf("rows=%+v", payload.Rows)
	}
	if len(payload.Products) != 1 || payload.Products[0].MigrationState != "cutover" || len(payload.Products[0].BOMSpecs) != 1 || payload.Products[0].BOMSpecs[0].BomSpecID != 91 {
		t.Fatalf("products=%+v", payload.Products)
	}

	mustExecInventoryAPISQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.products ADD COLUMN IF NOT EXISTS unit_rule_override_json JSONB NOT NULL DEFAULT '{}'::jsonb;
		ALTER TABLE %[1]s.product_bom_spec_migrations ADD COLUMN IF NOT EXISTS legacy_catalog_product BOOLEAN NOT NULL DEFAULT true;
		ALTER TABLE %[1]s.product_bom_spec_migrations ADD COLUMN IF NOT EXISTS spec_identity_mode TEXT NOT NULL DEFAULT '';
		INSERT INTO %[1]s.products(id,name,unit_rule_override_json)
		VALUES(8,'直接商品盒装挂耳','{"inventory_unit":"盒","default_sales_unit":"盒","unit_conversion_json":{"盒":{"盒":1}}}'::jsonb);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state,legacy_catalog_product,spec_identity_mode)
		VALUES(8,'preparing',false,'product');
	`, schema))
	req = httptest.NewRequest(http.MethodPost, "/api/products/inventory", bytes.NewBufferString(`{"product_id":8,"bom_spec_id":0,"bom_variant_id":0,"spec_g":0,"unit_code":"盒","units":3}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST direct-product inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	var directUnits, directBatchSpecID, directLedgerSpecID, directAuditCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=8 AND bom_spec_id=0 AND bom_variant_id=0 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&directUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %s.stock_batches WHERE item_id=8 ORDER BY id DESC LIMIT 1`, schema)).Scan(&directBatchSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_spec_id FROM %s.stock_ledger_entries WHERE item_id=8 ORDER BY id DESC LIMIT 1`, schema)).Scan(&directLedgerSpecID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='finished_inventory' AND action='adjust' AND meta->>'product_id'='8'
	`, schema)).Scan(&directAuditCount); err != nil {
		t.Fatal(err)
	}
	if directUnits != 3 || directBatchSpecID != 0 || directLedgerSpecID != 0 || directAuditCount != 1 {
		t.Fatalf("direct inventory units/batch spec/ledger spec/audit=%d/%d/%d/%d", directUnits, directBatchSpecID, directLedgerSpecID, directAuditCount)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/products/inventory?q=直接商品盒装挂耳", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET direct-product inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	var directPayload struct {
		Rows     []inventoryapp.FinishedInventoryRow `json:"rows"`
		Products []inventoryapp.ProductOption        `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &directPayload); err != nil {
		t.Fatal(err)
	}
	if len(directPayload.Rows) != 1 || directPayload.Rows[0].ProductID != 8 || directPayload.Rows[0].BomSpecID != 0 ||
		directPayload.Rows[0].BomVariantID != 0 || directPayload.Rows[0].SpecG != 0 || directPayload.Rows[0].InventoryUnit != "盒" ||
		directPayload.Rows[0].Units != 3 || directPayload.Rows[0].SpecIdentityMode != "product" || directPayload.Rows[0].BomSpecAuthoritative {
		t.Fatalf("direct rows=%+v", directPayload.Rows)
	}
	if len(directPayload.Products) != 1 || directPayload.Products[0].ID != 8 || directPayload.Products[0].SpecIdentityMode != "product" || directPayload.Products[0].BomSpecAuthoritative {
		t.Fatalf("direct products=%+v", directPayload.Products)
	}
}

func newInventoryAPITestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for inventory API tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_inventory_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func mustExecInventoryAPISQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
