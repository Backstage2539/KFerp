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

func TestStandardFinishedBatchAdjustmentAPIReconcilesWarehouseInventory(t *testing.T) {
	pool, schema := newStockHTTPPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',name TEXT NOT NULL DEFAULT '',unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,4) NOT NULL DEFAULT 0,onhand_g BIGINT NOT NULL DEFAULT 0,onhand_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.products(id,name) VALUES(7,'初晓'),(58,'曜石2.0');
`, schema, schema, schema, schema, schema))
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock EnsureSchema: %v", err)
	}
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,active) VALUES('customer_wh','吴国宏仓库','finished',true)
		ON CONFLICT(code) DO UPDATE SET active=true;
	`, schema))
	for _, product := range []struct {
		id   int64
		name string
	}{{id: 7, name: "初晓"}, {id: 58, name: "曜石2.0"}} {
		for index, batchCode := range []string{fmt.Sprintf("FP-LEGACY-%d-A", product.id), fmt.Sprintf("FP-LEGACY-%d-B", product.id)} {
			createdAt := "2026-01-01 00:00:00+00"
			if index == 1 {
				createdAt = "2026-01-02 00:00:00+00"
			}
			var batchID int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`
				INSERT INTO %s.stock_batches(
					batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,source_doc_type,
					source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator,created_at
				) VALUES($1,'finished_product',$2,$3,0,0,0,'production_operation_event',
					(SELECT COALESCE(MAX(id),0)+1 FROM %s.stock_batches),$1,0,14,0,14,'qa',$4::timestamptz)
				RETURNING id
			`, schema, schema), batchCode, product.id, product.name, createdAt).Scan(&batchID); err != nil {
				t.Fatalf("insert legacy batch %s: %v", batchCode, err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.stock_ledger_entries(
					item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,
					source_doc_id,source_batch_code,source_batch_id,qty_before_g,qty_change_g,qty_after_g,
					qty_before_units,qty_change_units,qty_after_units,operator,created_at
				) VALUES('finished_product',$1,$2,0,0,0,'customer_wh','production_operation_event',$3,$4,$4,0,0,0,0,14,14,'qa',$5::timestamptz)
			`, schema), product.id, product.name, batchID, batchCode, createdAt); err != nil {
				t.Fatalf("insert legacy batch ledger %s: %v", batchCode, err)
			}
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
			VALUES($1,0,0,0,'customer_wh',28,0)
		`, schema), product.id); err != nil {
			t.Fatalf("insert legacy inventory summary for %s: %v", product.name, err)
		}
	}

	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(postgresstock.NewRepository(pool, schema))})
	postAdjustment := func(productID, specG, targetUnits int64) {
		t.Helper()
		body := fmt.Sprintf(`{"item_type":"finished_product","item_id":%d,"spec_g":%d,"warehouse":"customer_wh","target_units":%d,"target_g":0,"reason":"吴国宏仓库库存更正"}`, productID, specG, targetUnits)
		req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /api/stock/adjustments product=%d spec=%d target=%d status=%d body=%s", productID, specG, targetUnits, rec.Code, rec.Body.String())
		}
	}
	for _, productID := range []int64{7, 58} {
		postAdjustment(productID, 0, 0)
		postAdjustment(productID, 454, 1)
		postAdjustment(productID, 227, 4)
	}

	for _, product := range []struct {
		id    int64
		specG int64
		units int64
	}{{7, 454, 1}, {7, 227, 4}, {58, 454, 1}, {58, 227, 4}} {
		var summaryUnits, summaryLooseG, batchUnits, batchGrams int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
			WHERE product_id=$1 AND bom_spec_id=0 AND spec_g=$2 AND warehouse='customer_wh'
		`, schema), product.id, product.specG).Scan(&summaryUnits, &summaryLooseG); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(b.remaining_units),0)::bigint,COALESCE(SUM(b.remaining_g),0)::bigint
			FROM %s.stock_batches b
			LEFT JOIN LATERAL (
				SELECT warehouse FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
			) last_ledger ON true
			WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=0 AND b.spec_g=$2
			  AND COALESCE(last_ledger.warehouse,'finished_goods')='customer_wh'
		`, schema, schema), product.id, product.specG).Scan(&batchUnits, &batchGrams); err != nil {
			t.Fatal(err)
		}
		if summaryUnits != product.units || summaryLooseG != 0 || batchUnits != product.units || batchGrams != product.specG*product.units {
			t.Fatalf("product=%d spec=%d summary=%d/%d batches=%d/%d, want %d/0 and %d/%d", product.id, product.specG, summaryUnits, summaryLooseG, batchUnits, batchGrams, product.units, product.units, product.specG*product.units)
		}
	}
	var nonzeroRows, legacyUnits, originalBatchCount, adjustmentCount, ledgerCount, auditCount, allocationCount int64
	for query, dest := range map[string]*int64{
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.finished_inventory WHERE warehouse='customer_wh' AND (onhand_units<>0 OR onhand_loose_g<>0)`, schema):                     &nonzeroRows,
		fmt.Sprintf(`SELECT COALESCE(SUM(onhand_units),0)::bigint FROM %s.finished_inventory WHERE warehouse='customer_wh' AND spec_g=0`, schema):                      &legacyUnits,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE source_doc_type='production_operation_event' AND item_id IN (7,58)`, schema):                          &originalBatchCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id IN (7,58) AND warehouse='customer_wh' AND reason='吴国宏仓库库存更正'`, schema):                    &adjustmentCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id IN (7,58) AND warehouse='customer_wh' AND source_doc_type='stock_adjustment'`, schema): &ledgerCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='stock_adjustment' AND action='submit' AND meta->>'reason'='吴国宏仓库库存更正'`, schema):             &auditCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustment_batch_allocations WHERE item_type='finished_product' AND warehouse='customer_wh'`, schema):               &allocationCount,
	} {
		if err := pool.QueryRow(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if nonzeroRows != 4 || legacyUnits != 0 || originalBatchCount != 4 || adjustmentCount != 6 || ledgerCount != 6 || auditCount != 6 || allocationCount != 4 {
		t.Fatalf("final rows/legacy/original batches/adjustments/ledgers/audits/allocations = %d/%d/%d/%d/%d/%d/%d, want 4/0/4/6/6/6/4", nonzeroRows, legacyUnits, originalBatchCount, adjustmentCount, ledgerCount, auditCount, allocationCount)
	}
}

func TestCanonicalBOMBatchAdjustmentAPIUsesActualProductSpecifications(t *testing.T) {
	pool, schema := newStockHTTPPostgresTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,code TEXT NOT NULL DEFAULT '',name TEXT NOT NULL DEFAULT '',unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,4) NOT NULL DEFAULT 0,onhand_g BIGINT NOT NULL DEFAULT 0,onhand_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,
	action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
CREATE TABLE %s.production_bom_output_bindings(
	output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.production_bom_specs(
	id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL
);
CREATE TABLE %s.production_bom_version_variants(
	id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,
	inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL
);
INSERT INTO %s.products(id,name) VALUES(7,'初晓'),(58,'曜石2.0');
INSERT INTO %s.production_bom_versions(id,bom_id,status) VALUES(553,883,'published'),(554,890,'published');
INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
VALUES('product',7,883,553,true),('product',58,890,554,true);
INSERT INTO %s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
VALUES(19,883,'spec-5','227g袋装','袋'),(20,883,'spec-6','454g袋装','袋'),
	  (68,890,'spec-5','227g袋装','袋'),(69,890,'spec-6','454g袋装','袋');
INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(1185,553,19,'227g袋装','袋',false,5),(1186,553,20,'454g袋装','袋',true,6),
	  (1205,554,68,'227g袋装','袋',false,5),(1206,554,69,'454g袋装','袋',true,6);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("stock EnsureSchema: %v", err)
	}
	mustExecStockHTTPPostgres(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,active) VALUES('customer_wh','吴国宏仓库','finished',true)
		ON CONFLICT(code) DO UPDATE SET active=true;
	`, schema))
	seed := func(productID, bomSpecID, variantID, units int64, code, createdAt string) {
		t.Helper()
		var batchID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_batches(
				batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,source_doc_type,
				source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator,created_at
			) VALUES($1,'finished_product',$2,CASE WHEN $2::bigint=7 THEN '初晓' ELSE '曜石2.0' END,0,$3,$4,
				'production_operation_event',(SELECT COALESCE(MAX(id),0)+1 FROM %s.stock_batches),$1,0,$5,0,$5,'qa',$6::timestamptz)
			RETURNING id
		`, schema, schema), code, productID, bomSpecID, variantID, units, createdAt).Scan(&batchID); err != nil {
			t.Fatalf("insert source batch %s: %v", code, err)
		}
		name := "初晓"
		if productID == 58 {
			name = "曜石2.0"
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.stock_ledger_entries(
				item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,
				source_doc_id,source_batch_code,source_batch_id,qty_before_g,qty_change_g,qty_after_g,
				qty_before_units,qty_change_units,qty_after_units,operator,created_at
			) VALUES('finished_product',$1,$2,0,$3,$4,'customer_wh','production_operation_event',$5,$6,$6,0,0,0,0,$7,$7,'qa',$8::timestamptz)
		`, schema), productID, name, bomSpecID, variantID, batchID, code, units, createdAt); err != nil {
			t.Fatalf("insert source ledger %s: %v", code, err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
			VALUES($1,$2,$3,0,'customer_wh',$4,0)
			ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
			SET onhand_units=%s.finished_inventory.onhand_units+excluded.onhand_units
		`, schema, schema), productID, bomSpecID, variantID, units); err != nil {
			t.Fatalf("insert source summary for %s: %v", code, err)
		}
	}
	seed(7, 20, 1186, 8, "FP-P7-454-A", "2026-01-01 00:00:00+00")
	seed(7, 20, 1186, 6, "FP-P7-454-B", "2026-01-02 00:00:00+00")
	seed(7, 19, 1185, 5, "FP-P7-227-A", "2026-01-01 00:00:00+00")
	seed(7, 19, 1185, 9, "FP-P7-227-B", "2026-01-02 00:00:00+00")
	seed(58, 69, 1206, 1, "FP-P58-454", "2026-01-01 00:00:00+00")
	seed(58, 68, 1205, 3, "FP-P58-227", "2026-01-01 00:00:00+00")

	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(postgresstock.NewRepository(pool, schema))})
	postAdjustment := func(productID, bomSpecID, units int64) {
		t.Helper()
		body := fmt.Sprintf(`{"item_type":"finished_product","item_id":%d,"bom_spec_id":%d,"unit_code":"袋","warehouse":"customer_wh","target_units":%d,"reason":"吴国宏仓库库存更正"}`, productID, bomSpecID, units)
		req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST adjustment product=%d bom_spec=%d target=%d status=%d body=%s", productID, bomSpecID, units, rec.Code, rec.Body.String())
		}
	}
	for _, target := range []struct{ productID, bomSpecID, units int64 }{
		{7, 20, 1}, {7, 19, 4}, {58, 69, 1}, {58, 68, 4},
	} {
		postAdjustment(target.productID, target.bomSpecID, target.units)
	}
	for _, target := range []struct{ productID, bomSpecID, variantID, units, grams int64 }{
		{7, 20, 1186, 1, 0}, {7, 19, 1185, 4, 0}, {58, 69, 1206, 1, 0}, {58, 68, 1205, 4, 0},
	} {
		var summaryUnits, batchUnits int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units FROM %s.finished_inventory WHERE product_id=$1 AND bom_spec_id=$2 AND warehouse='customer_wh'
		`, schema), target.productID, target.bomSpecID).Scan(&summaryUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(b.remaining_units),0)::bigint FROM %s.stock_batches b
			LEFT JOIN LATERAL (
				SELECT warehouse FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
			) last_ledger ON true
			WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2
			  AND COALESCE(last_ledger.warehouse,'finished_goods')='customer_wh'
		`, schema, schema), target.productID, target.bomSpecID).Scan(&batchUnits); err != nil {
			t.Fatal(err)
		}
		if summaryUnits != target.units || batchUnits != target.units {
			t.Fatalf("product=%d bom_spec=%d summary/batches=%d/%d, want %d", target.productID, target.bomSpecID, summaryUnits, batchUnits, target.units)
		}
	}
	var adjustmentCount, ledgerCount, auditCount, allocationCount, originalBatchCount int64
	for query, dest := range map[string]*int64{
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id IN (7,58) AND warehouse='customer_wh' AND reason='吴国宏仓库库存更正'`, schema):                    &adjustmentCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id IN (7,58) AND warehouse='customer_wh' AND source_doc_type='stock_adjustment'`, schema): &ledgerCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='stock_adjustment' AND action='submit' AND meta->>'reason'='吴国宏仓库库存更正'`, schema):             &auditCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustment_batch_allocations WHERE item_type='finished_product' AND warehouse='customer_wh'`, schema):               &allocationCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE source_doc_type='production_operation_event' AND item_id IN (7,58)`, schema):                          &originalBatchCount,
	} {
		if err := pool.QueryRow(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if adjustmentCount != 4 || ledgerCount != 4 || auditCount != 4 || allocationCount != 4 || originalBatchCount != 6 {
		t.Fatalf("adjustments/ledgers/audits/allocations/production batches = %d/%d/%d/%d/%d, want 4/4/4/4/6", adjustmentCount, ledgerCount, auditCount, allocationCount, originalBatchCount)
	}
	var first454, second454, first227, second227, added227 int64
	for query, dest := range map[string]*int64{
		fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-P7-454-A'`, schema):                                             &first454,
		fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-P7-454-B'`, schema):                                             &second454,
		fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-P7-227-A'`, schema):                                             &first227,
		fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-P7-227-B'`, schema):                                             &second227,
		fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE item_id=58 AND bom_spec_id=68 AND source_doc_type='stock_adjustment'`, schema): &added227,
	} {
		if err := pool.QueryRow(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if first454 != 0 || second454 != 1 || first227 != 0 || second227 != 4 || added227 != 1 {
		t.Fatalf("FIFO/additional batch remaining = %d/%d/%d/%d/%d, want 0/1/0/4/1", first454, second454, first227, second227, added227)
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
