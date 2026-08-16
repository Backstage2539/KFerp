package stock

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	stockapp "orderapp/internal/application/stock"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func TestBOMSpecFinishedTransferAndAdjustmentResolveCurrentVariantPostgresAPI(t *testing.T) {
	pool, schema := newStockHTTPPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()

	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.materials (
	id BIGINT PRIMARY KEY,code TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'bean',unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,min_level_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.products (id BIGINT PRIMARY KEY,name TEXT NOT NULL,customer_id BIGINT NOT NULL DEFAULT 0);
CREATE TABLE %[1]s.work_orders (id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL DEFAULT '');
CREATE TABLE %[1]s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %[1]s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.business_groups (id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
CREATE TABLE %[1]s.business_group_items (id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
CREATE TABLE %[1]s.business_group_assignments (
	id BIGSERIAL PRIMARY KEY,group_id BIGINT NOT NULL DEFAULT 0,group_item_id BIGINT NOT NULL DEFAULT 0,
	usage_key TEXT NOT NULL DEFAULT '',object_key TEXT NOT NULL DEFAULT '',object_id BIGINT NOT NULL DEFAULT 0,object_ref TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.production_bom_versions (id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
CREATE TABLE %[1]s.production_bom_output_bindings (
	output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %[1]s.production_bom_specs (
	id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL
);
CREATE TABLE %[1]s.production_bom_version_variants (
	id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,
	inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL
);
CREATE TABLE %[1]s.product_bom_spec_migrations (product_id BIGINT PRIMARY KEY,state TEXT NOT NULL);
INSERT INTO %[1]s.products(id,name) VALUES(7,'规格组商品');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,status) VALUES(41,31,'published');
INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
VALUES('product',7,31,41,true);
INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
VALUES(91,31,'bag-227','227g袋','袋'),(92,31,'bag-454','454g袋','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(191,41,91,'227g袋','袋',true,10),(192,41,92,'454g袋','袋',false,20);
INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state) VALUES(7,'cutover');
`, schema))
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock EnsureSchema: %v", err)
	}
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.warehouses(code,name,kind,sort_order,is_default,active,description)
VALUES('finished_shop','门店成品仓','finished',45,false,true,'门店成品仓')
ON CONFLICT(code) DO UPDATE SET active=true;
`, schema))

	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(postgresstock.NewRepository(pool, schema))})
	post := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	rec := post("/api/stock/adjustments", `{"item_type":"finished_product","item_id":7,"bom_spec_id":91,"unit_code":"袋","warehouse":"finished_goods","target_units":12,"reason":"V1规格期初"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"bom_variant_id":191`) {
		t.Fatalf("POST V1 adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = post("/api/stock/adjustments", `{"item_type":"finished_product","item_id":7,"bom_spec_id":92,"unit_code":"袋","warehouse":"finished_goods","target_units":5,"reason":"另一规格期初"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST second spec adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}

	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=41;
INSERT INTO %[1]s.production_bom_versions(id,bom_id,status) VALUES(42,31,'published');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(291,42,91,'227g袋 V2','袋',true,10),(292,42,92,'454g袋 V2','袋',false,20);
UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=42 WHERE output_type='product' AND output_id=7;
`, schema))

	rec = post("/api/stock/finished-transfers", `{"product_id":7,"bom_spec_id":91,"unit_code":"袋","from_warehouse":"finished_goods","to_warehouse":"finished_shop","qty_units":4,"note":"V2规格转仓"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"bom_variant_id":291`) {
		t.Fatalf("POST V2 transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	var source91, source92, target91, entryVariant int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=91 AND warehouse='finished_goods'`, schema)).Scan(&source91); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=92 AND warehouse='finished_goods'`, schema)).Scan(&source92); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=91 AND warehouse='finished_shop'`, schema)).Scan(&target91); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_variant_id FROM %s.stock_entry_items ORDER BY id DESC LIMIT 1`, schema)).Scan(&entryVariant); err != nil {
		t.Fatal(err)
	}
	if source91 != 8 || source92 != 5 || target91 != 4 || entryVariant != 291 {
		t.Fatalf("V2 transfer inventory source91/source92/target91=%d/%d/%d entry_variant=%d", source91, source92, target91, entryVariant)
	}
	var transferLedgerV2, transferAuditV2 int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_entry' AND bom_variant_id=291`, schema)).Scan(&transferLedgerV2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='stock_entry' AND action='submit' AND meta->>'bom_spec_id'='91' AND meta->>'bom_variant_id'='291'`, schema)).Scan(&transferAuditV2); err != nil {
		t.Fatal(err)
	}
	if transferLedgerV2 != 2 || transferAuditV2 != 1 {
		t.Fatalf("V2 transfer ledger/audit=%d/%d, want 2/1", transferLedgerV2, transferAuditV2)
	}

	rec = post("/api/stock/adjustments", `{"item_type":"finished_product","item_id":7,"bom_spec_id":91,"unit_code":"袋","warehouse":"finished_shop","target_units":7,"reason":"V2规格盘点"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"bom_variant_id":291`) {
		t.Fatalf("POST V2 adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	var adjustmentLedgerV2, adjustmentAuditV2 int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_adjustment' AND bom_variant_id=291`, schema)).Scan(&adjustmentLedgerV2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='stock_adjustment' AND meta->>'bom_spec_id'='91' AND meta->>'bom_variant_id'='291'`, schema)).Scan(&adjustmentAuditV2); err != nil {
		t.Fatal(err)
	}
	if adjustmentLedgerV2 != 1 || adjustmentAuditV2 != 1 {
		t.Fatalf("V2 adjustment ledger/audit=%d/%d, want 1/1", adjustmentLedgerV2, adjustmentAuditV2)
	}

	for _, stale := range []struct{ path, body string }{
		{"/api/stock/finished-transfers", `{"product_id":7,"bom_spec_id":91,"bom_variant_id":191,"unit_code":"袋","from_warehouse":"finished_goods","to_warehouse":"finished_shop","qty_units":1}`},
		{"/api/stock/adjustments", `{"item_type":"finished_product","item_id":7,"bom_spec_id":91,"bom_variant_id":191,"unit_code":"袋","warehouse":"finished_shop","target_units":8,"reason":"过期版本"}`},
	} {
		rec = post(stale.path, stale.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "current default published BOM") {
			t.Fatalf("stale write %s status=%d body=%s", stale.path, rec.Code, rec.Body.String())
		}
	}
}

func newStockHTTPPostgresTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_stock_http_pr600_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	return pool, schema
}

func mustExecStockHTTPPostgres(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatal(err)
	}
}
