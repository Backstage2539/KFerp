package production

import (
	"context"
	"fmt"
	"testing"

	productionapp "orderapp/internal/application/production"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateQualityInspectionAppliesBatchQualityStatus(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.quality_inspections (
	id BIGSERIAL PRIMARY KEY,
	scope TEXT NOT NULL DEFAULT '',
	reference_type TEXT NOT NULL DEFAULT '',
	reference_no TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT 'hold',
	metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	note TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.work_orders (
	id BIGSERIAL PRIMARY KEY,
	work_order_no TEXT NOT NULL UNIQUE,
	running_item_id BIGINT NOT NULL UNIQUE
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
	remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
INSERT INTO %s.work_orders(work_order_no,running_item_id) VALUES ('WO-0000000020',20);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units)
VALUES
	('FP-0000000020','finished_product',9,'橘皮乌龙',454,'production_run',20,'A20260427-071539-6b',9840,21,9840,21),
	('MB-0000000007','material',7,'孟连水洗5T批次',0,'material_receipt',7,'MB-0000000007',60000,0,60000,0);
INSERT INTO %s.material_batches(batch_code,material_id) VALUES ('MB-0000000007',7);
`, schema, schema, schema, schema, schema, schema, schema))

	repo := NewRepository(pool, schema)
	if _, err := repo.CreateQualityInspection(ctx, productionapp.QualityInspectionCommand{
		Scope:         "work_order",
		ReferenceType: "work_order",
		ReferenceNo:   "WO-0000000020",
		ItemName:      "橘皮乌龙",
		Result:        "reject",
		Operator:      "qa",
	}); err != nil {
		t.Fatalf("CreateQualityInspection work order: %v", err)
	}
	assertQualityStatus(t, ctx, pool, schema, "stock_batches", "FP-0000000020", "reject")

	if _, err := repo.CreateQualityInspection(ctx, productionapp.QualityInspectionCommand{
		Scope:         "finished_batch",
		ReferenceType: "finished_batch",
		ReferenceNo:   "FP-0000000020",
		ItemName:      "橘皮乌龙",
		Result:        "hold",
		Operator:      "qa",
	}); err != nil {
		t.Fatalf("CreateQualityInspection finished batch: %v", err)
	}
	assertQualityStatus(t, ctx, pool, schema, "stock_batches", "FP-0000000020", "hold")

	if _, err := repo.CreateQualityInspection(ctx, productionapp.QualityInspectionCommand{
		Scope:         "raw_material",
		ReferenceType: "raw_material",
		ReferenceNo:   "MB-0000000007",
		ItemName:      "孟连水洗5T批次",
		Result:        "reject",
		Operator:      "qa",
	}); err != nil {
		t.Fatalf("CreateQualityInspection raw material: %v", err)
	}
	assertQualityStatus(t, ctx, pool, schema, "material_batches", "MB-0000000007", "reject")
	assertQualityStatus(t, ctx, pool, schema, "stock_batches", "MB-0000000007", "reject")
}

func TestBackfillQualityStatusesUsesLatestInspection(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.quality_inspections (
	id BIGSERIAL PRIMARY KEY,
	scope TEXT NOT NULL DEFAULT '',
	reference_type TEXT NOT NULL DEFAULT '',
	reference_no TEXT NOT NULL DEFAULT '',
	item_name TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT 'hold',
	metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	note TEXT NOT NULL DEFAULT '',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.work_orders (
	id BIGSERIAL PRIMARY KEY,
	work_order_no TEXT NOT NULL UNIQUE,
	running_item_id BIGINT NOT NULL UNIQUE
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	item_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
INSERT INTO %s.work_orders(work_order_no,running_item_id) VALUES ('WO-0000000020',20);
INSERT INTO %s.stock_batches(batch_code,item_type,source_doc_id)
VALUES ('FP-0000000020','finished_product',20),('MB-0000000007','material',0);
INSERT INTO %s.material_batches(batch_code) VALUES ('MB-0000000007');
INSERT INTO %s.quality_inspections(scope,reference_type,reference_no,result,created_at)
VALUES
	('work_order','work_order','WO-0000000020','reject',now() - interval '2 minutes'),
	('work_order','work_order','WO-0000000020','hold',now() - interval '1 minute'),
	('raw_material','raw_material','MB-0000000007','reject',now());
`, schema, schema, schema, schema, schema, schema, schema, schema))

	if err := backfillQualityStatusesFromInspections(ctx, pool, schema); err != nil {
		t.Fatalf("backfillQualityStatusesFromInspections: %v", err)
	}
	assertQualityStatus(t, ctx, pool, schema, "stock_batches", "FP-0000000020", "hold")
	assertQualityStatus(t, ctx, pool, schema, "material_batches", "MB-0000000007", "reject")
	assertQualityStatus(t, ctx, pool, schema, "stock_batches", "MB-0000000007", "reject")
}

func assertQualityStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table, batchCode, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT quality_status FROM %s.%s WHERE batch_code=$1`, schema, table), batchCode).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s %s quality_status = %q, want %q", table, batchCode, got, want)
	}
}
