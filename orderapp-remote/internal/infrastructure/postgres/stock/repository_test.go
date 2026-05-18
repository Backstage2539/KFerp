package stock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	stockapp "orderapp/internal/application/stock"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReceiveMaterialCreatesBatchAndLedger(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',300);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	res, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       1200,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	if !strings.HasPrefix(res.BatchCode, "MB-") {
		t.Fatalf("batch code = %q, want MB-*", res.BatchCode)
	}

	var onhandG, remainingG, ledgerChange int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=$1`, schema), res.BatchID).Scan(&remainingG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_change_g FROM %s.stock_ledger_entries WHERE source_doc_type='material_receipt' AND source_doc_id=$1`, schema), res.ReceiptID).Scan(&ledgerChange); err != nil {
		t.Fatal(err)
	}
	if onhandG != 1500 || remainingG != 1200 || ledgerChange != 1200 {
		t.Fatalf("onhand/remaining/ledger = %d/%d/%d, want 1500/1200/1200", onhandG, remainingG, ledgerChange)
	}
}

func TestMaterialCostAdjustmentRequiresAvailableBatchSourceGuard(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, "func (r Repository) createMaterialCostAdjustment")
	end := strings.Index(src, "func (r Repository) materialAdjustmentUnitCostTx")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("createMaterialCostAdjustment function not found")
	}
	body := src[start:end]
	for _, want := range []string{
		"b.remaining_g > 0",
		"b.status='active'",
		"COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("material cost adjustment must only target available batches; missing %q", want)
		}
	}
}

func TestStockAdjustmentsWriteAuditLogsSourceGuard(t *testing.T) {
	b, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	quantityBody := stockRepositoryFunctionBody(t, src, "func (r Repository) CreateAdjustment", "func (r Repository) createMaterialCostAdjustment")
	costBody := stockRepositoryFunctionBody(t, src, "func (r Repository) createMaterialCostAdjustment", "func (r Repository) materialAdjustmentUnitCostTx")

	for _, want := range []string{
		"AuditInsertTx",
		"stock_adjustment",
		`postgresinfra.StrPtr("qty_g")`,
		`"adjustment_type"`,
		`"quantity"`,
		`"change_g"`,
	} {
		if !strings.Contains(quantityBody, want) {
			t.Fatalf("quantity stock adjustment must write audit log; missing %q", want)
		}
	}
	for _, want := range []string{
		"AuditInsertTx",
		"stock_adjustment",
		`postgresinfra.StrPtr("unit_cost")`,
		`"adjustment_type"`,
		`"material_cost"`,
		`"material_batch_id"`,
		`"value_change"`,
	} {
		if !strings.Contains(costBody, want) {
			t.Fatalf("material cost adjustment must write audit log; missing %q", want)
		}
	}
}

func TestEnsureSchemaAddsQualityStatusColumns(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
);
`, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	for _, tc := range []struct {
		table string
	}{
		{table: "stock_batches"},
		{table: "material_batches"},
	} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema=$1 AND table_name=$2 AND column_name='quality_status'
			)
		`, schema, tc.table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("%s missing quality_status column", tc.table)
		}
	}
}

func TestTransferMaterialMovesBatchLocationWithoutChangingTotalOnhand(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',0);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       60000,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	transfer, err := repo.TransferMaterial(ctx, stockapp.MaterialTransferCommand{
		MaterialID:     1,
		FromWarehouse:  "raw_materials",
		ToWarehouse:    "wip",
		QtyG:           20000,
		Note:           "三天生产领料",
		Operator:       "jj",
		IdempotencyKey: "领料单-1",
	})
	if err != nil {
		t.Fatalf("TransferMaterial: %v", err)
	}
	if transfer.TransferNo == "" {
		t.Fatalf("transfer no is empty")
	}

	var onhandG, rawG, wipG, batchRemainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='raw_materials'`, schema), receipt.BatchID).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='wip'`, schema), receipt.BatchID).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=$1`, schema), receipt.BatchID).Scan(&batchRemainingG); err != nil {
		t.Fatal(err)
	}
	if onhandG != 60000 || rawG != 40000 || wipG != 20000 || batchRemainingG != 60000 {
		t.Fatalf("onhand/raw/wip/batch remaining = %d/%d/%d/%d, want 60000/40000/20000/60000", onhandG, rawG, wipG, batchRemainingG)
	}

	var outCount, inCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='material_transfer' AND source_doc_id=$1 AND warehouse='raw_materials' AND qty_change_g=-20000`, schema), transfer.TransferID).Scan(&outCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='material_transfer' AND source_doc_id=$1 AND warehouse='wip' AND qty_change_g=20000`, schema), transfer.TransferID).Scan(&inCount); err != nil {
		t.Fatal(err)
	}
	if outCount != 1 || inCount != 1 {
		t.Fatalf("ledger transfer entries = raw %d / wip %d, want 1/1", outCount, inCount)
	}
}

func TestTransferMaterialSkipsFrozenQualityBatches(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',0);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       1200,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.material_batches SET quality_status='reject' WHERE id=%d;
		UPDATE %s.stock_batches SET quality_status='reject' WHERE batch_code='%s';
	`, schema, receipt.BatchID, schema, receipt.BatchCode))

	_, err = repo.TransferMaterial(ctx, stockapp.MaterialTransferCommand{
		MaterialID:    1,
		FromWarehouse: "raw_materials",
		ToWarehouse:   "wip",
		QtyG:          500,
		Operator:      "jj",
	})
	if err == nil || !strings.Contains(err.Error(), "quality") {
		t.Fatalf("TransferMaterial with frozen batch error = %v, want quality block", err)
	}
}

func TestFinishedInventorySupportsWarehousesAndFinishedTransfers(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.products(id,name) VALUES (9,'橘皮乌龙');
INSERT INTO %s.finished_inventory(product_id,spec_g,onhand_units,onhand_loose_g) VALUES (9,454,1,20);
`, schema, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,sort_order,is_default,active,description)
		VALUES('finished_shop','门店成品仓','finished',45,false,true,'门店/临时销售成品仓')
		ON CONFLICT (code) DO UPDATE SET active=true;
	`, schema))

	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (9,454,'finished_shop',0,0)
		ON CONFLICT (product_id,spec_g,warehouse) DO NOTHING;
	`, schema))

	repo := NewRepository(pool, schema)
	adjustment, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType:    "finished_product",
		ItemID:      9,
		SpecG:       454,
		Warehouse:   "finished_goods",
		TargetUnits: 3,
		TargetG:     20,
		Reason:      "期初成品盘点",
		Operator:    "jj",
	})
	if err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
	if adjustment.AdjustmentID <= 0 {
		t.Fatalf("adjustment = %+v", adjustment)
	}
	transfer, err := repo.TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID:      9,
		SpecG:          454,
		FromWarehouse:  "finished_goods",
		ToWarehouse:    "finished_shop",
		QtyUnits:       1,
		QtyLooseG:      20,
		Note:           "门店备货",
		Operator:       "jj",
		IdempotencyKey: "finished-transfer-1",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}
	if transfer.TransferNo == "" {
		t.Fatalf("transfer no is empty")
	}

	var finishedGoodsUnits, finishedGoodsLoose, shopUnits, shopLoose int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=9 AND spec_g=454 AND warehouse='finished_goods'`, schema)).Scan(&finishedGoodsUnits, &finishedGoodsLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=9 AND spec_g=454 AND warehouse='finished_shop'`, schema)).Scan(&shopUnits, &shopLoose); err != nil {
		t.Fatal(err)
	}
	if finishedGoodsUnits != 2 || finishedGoodsLoose != 0 || shopUnits != 1 || shopLoose != 20 {
		t.Fatalf("finished warehouses = finished_goods %d/%d, shop %d/%d; want 2/0 and 1/20", finishedGoodsUnits, finishedGoodsLoose, shopUnits, shopLoose)
	}

	var outCount, inCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='finished_product_transfer' AND source_doc_id=$1 AND warehouse='finished_goods' AND qty_change_g=-474`, schema), transfer.TransferID).Scan(&outCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='finished_product_transfer' AND source_doc_id=$1 AND warehouse='finished_shop' AND qty_change_g=474`, schema), transfer.TransferID).Scan(&inCount); err != nil {
		t.Fatal(err)
	}
	if outCount != 1 || inCount != 1 {
		t.Fatalf("finished transfer ledger = out %d / in %d, want 1/1", outCount, inCount)
	}
}

func TestTransferFinishedProductRejectsFrozenBatch(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.products(id,name) VALUES (9,'橘皮乌龙');
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
VALUES (9,454,'finished_goods',2,0)
ON CONFLICT (product_id,spec_g,warehouse) DO UPDATE SET onhand_units=2,onhand_loose_g=0;
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,operator,created_at)
VALUES ('FP-FROZEN','finished_product',9,'橘皮乌龙',454,'production_run',20,'WO-20',908,2,908,2,'reject','qa',now());
INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,source_batch_id,qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator,created_at)
VALUES ('finished_product',9,'橘皮乌龙',454,'finished_goods','production_run',20,'FP-FROZEN','WO-20',0,908,908,0,2,2,'qa',now());
`, schema, schema, schema))

	repo := NewRepository(pool, schema)
	_, err := repo.TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID:      9,
		SpecG:          454,
		FromWarehouse:  "finished_goods",
		ToWarehouse:    "finished_shop",
		QtyUnits:       1,
		Operator:       "jj",
		IdempotencyKey: "frozen-finished-transfer",
	})
	if err == nil || !strings.Contains(err.Error(), "quality") {
		t.Fatalf("TransferFinishedProduct with frozen batch error = %v, want quality block", err)
	}
}

func TestMaterialAdjustmentBackfillCreatesTransferableRawBatch(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,purchase_price,onhand_g) VALUES (1,'BEAN-1','水洗豆',38.25,0);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	res, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType:  "material",
		ItemID:    1,
		Warehouse: "raw_materials",
		TargetG:   60000,
		Reason:    "期初库存补录",
		Operator:  "jj",
	})
	if err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}

	var batchRemainingG, rawG int64
	var batchUnitCost, stockBatchUnitCost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g, unit_cost::float8 FROM %s.material_batches WHERE batch_code=$1`, schema), fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&batchRemainingG, &batchUnitCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8 FROM %s.stock_batches WHERE batch_code=$1`, schema), fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&stockBatchUnitCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE batch_code=$1 AND warehouse='raw_materials'`, schema), fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if batchRemainingG != 60000 || rawG != 60000 {
		t.Fatalf("backfill batch remaining/raw = %d/%d, want 60000/60000", batchRemainingG, rawG)
	}
	if batchUnitCost != 38.25 || stockBatchUnitCost != 38.25 {
		t.Fatalf("backfill unit costs = %.2f/%.2f, want 38.25/38.25", batchUnitCost, stockBatchUnitCost)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.audit_logs
		WHERE entity_type='stock_adjustment'
		  AND entity_id=$1
		  AND action='submit'
		  AND field='qty_g'
		  AND old_value='0'
		  AND new_value='60000'
		  AND meta->>'adjustment_type'='quantity'
		  AND meta->>'batch_code'=$2
	`, schema), res.AdjustmentID, fmt.Sprintf("ADJ-%010d", res.AdjustmentID)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("stock adjustment audit rows = %d, want 1", auditCount)
	}
}

func TestCreateAdjustmentUpdatesMaterialBatchUnitCost(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,onhand_g) VALUES (1,'BEAN-1','水洗豆',0);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 1,
		Supplier:   "云南供应商",
		QtyG:       60000,
		UnitCost:   42.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}

	res, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		AdjustmentType:  "material_cost",
		ItemType:        "material",
		ItemID:          1,
		MaterialBatchID: receipt.BatchID,
		TargetUnitCost:  52.75,
		Reason:          "入库单价录错更正",
		Operator:        "jj",
	})
	if err != nil {
		t.Fatalf("CreateAdjustment material cost: %v", err)
	}

	var batchCost, stockBatchCost, beforeCost, afterCost, valueChange float64
	var quantityChange int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8 FROM %s.material_batches WHERE id=$1`, schema), receipt.BatchID).Scan(&batchCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8 FROM %s.stock_batches WHERE batch_code=$1`, schema), receipt.BatchCode).Scan(&stockBatchCost); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost_before::float8, unit_cost_after::float8, value_change::float8 FROM %s.stock_adjustments WHERE id=$1`, schema), res.AdjustmentID).Scan(&beforeCost, &afterCost, &valueChange); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_change_g FROM %s.stock_adjustment_items WHERE adjustment_id=$1`, schema), res.AdjustmentID).Scan(&quantityChange); err != nil {
		t.Fatal(err)
	}
	if batchCost != 52.75 || stockBatchCost != 52.75 {
		t.Fatalf("unit costs = material %.2f stock %.2f, want 52.75", batchCost, stockBatchCost)
	}
	if beforeCost != 42.5 || afterCost != 52.75 || valueChange != 615 || quantityChange != 0 {
		t.Fatalf("adjustment cost before/after/value/change = %.2f/%.2f/%.2f/%d, want 42.50/52.75/615.00/0", beforeCost, afterCost, valueChange, quantityChange)
	}

	var auditOld, auditNew, auditBatchCode string
	var auditValueChange float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT old_value,new_value,meta->>'batch_code',(meta->>'value_change')::float8
		FROM %s.audit_logs
		WHERE entity_type='stock_adjustment'
		  AND entity_id=$1
		  AND action='submit'
		  AND field='unit_cost'
		  AND meta->>'adjustment_type'='material_cost'
		  AND (meta->>'material_batch_id')::bigint=$2
	`, schema), res.AdjustmentID, receipt.BatchID).Scan(&auditOld, &auditNew, &auditBatchCode, &auditValueChange); err != nil {
		t.Fatal(err)
	}
	if auditOld != "42.5000" || auditNew != "52.7500" || auditBatchCode != receipt.BatchCode || auditValueChange != 615 {
		t.Fatalf("material cost audit = old %q new %q batch %q value %.2f, want 42.5000/52.7500/%s/615", auditOld, auditNew, auditBatchCode, auditValueChange, receipt.BatchCode)
	}
}

func TestEnsureSchemaBackfillsLegacyMaterialOnhandIntoRawBatchLocation(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'g',
	purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id, spec_g)
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
INSERT INTO %s.materials(id,code,name,onhand_g,purchase_price) VALUES (1,'Menglian-W-5T','孟连水洗5T批次',60000,45.50);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	repo := NewRepository(pool, schema)
	transfer, err := repo.TransferMaterial(ctx, stockapp.MaterialTransferCommand{
		MaterialID:     1,
		FromWarehouse:  "raw_materials",
		ToWarehouse:    "wip",
		QtyG:           60000,
		Note:           "WO-0000000020 领料",
		Operator:       "jj",
		IdempotencyKey: "wo-20-wip-transfer",
	})
	if err != nil {
		t.Fatalf("TransferMaterial with legacy onhand: %v", err)
	}
	if transfer.TransferNo == "" || len(transfer.Allocations) != 1 {
		t.Fatalf("transfer = %+v, want one legacy allocation", transfer)
	}

	var onhandG, rawG, wipG, batchRemainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&onhandG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=1 AND warehouse='raw_materials'`, schema)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=1 AND warehouse='wip'`, schema)).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE batch_code='LEGACY-MAT-0000000001'`, schema)).Scan(&batchRemainingG); err != nil {
		t.Fatal(err)
	}
	if onhandG != 60000 || rawG != 0 || wipG != 60000 || batchRemainingG != 60000 {
		t.Fatalf("onhand/raw/wip/batch remaining = %d/%d/%d/%d, want 60000/0/60000/60000", onhandG, rawG, wipG, batchRemainingG)
	}

	trace, err := repo.GetStockTrace(ctx, stockapp.StockTraceQuery{BatchCode: "LEGACY-MAT-0000000001"})
	if err != nil {
		t.Fatalf("GetStockTrace legacy material batch: %v", err)
	}
	if trace.TraceType != "material_batch" || trace.MaterialBatch.BatchCode != "LEGACY-MAT-0000000001" || trace.MaterialBatch.MaterialName != "孟连水洗5T批次" {
		t.Fatalf("legacy material trace = %+v", trace)
	}
	if len(trace.MaterialLocations) != 1 || trace.MaterialLocations[0].Warehouse != "wip" || trace.MaterialLocations[0].QtyG != 60000 {
		t.Fatalf("legacy material trace locations = %+v", trace.MaterialLocations)
	}
}

func TestListOutboundLogsReturnsDeliveryNoteDocumentsForInventory(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.customers (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	company_name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.pay_statuses (id BIGINT PRIMARY KEY, name TEXT NOT NULL);
CREATE TABLE %s.ship_statuses (id BIGINT PRIMARY KEY, name TEXT NOT NULL);
CREATE TABLE %s.order_process_statuses (id BIGINT PRIMARY KEY, name TEXT NOT NULL);
CREATE TABLE %s.orders (
	id BIGINT PRIMARY KEY,
	order_no TEXT NOT NULL DEFAULT '',
	customer_id BIGINT,
	pay_status_id BIGINT,
	ship_status_id BIGINT,
	process_status_id BIGINT,
	ship_method TEXT NOT NULL DEFAULT '',
	ship_tracking_no TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.warehouses (code TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '');
CREATE TABLE %s.delivery_note_forms (
	order_id BIGINT PRIMARY KEY,
	posting_date DATE,
	source_warehouse TEXT NOT NULL DEFAULT 'finished_goods',
	delivery_method TEXT NOT NULL DEFAULT '',
	tracking_no TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.delivery_note_documents (
	id BIGINT PRIMARY KEY,
	order_id BIGINT NOT NULL,
	order_no TEXT NOT NULL DEFAULT '',
	version_no INTEGER NOT NULL,
	snapshot_json JSONB NOT NULL,
	is_latest BOOLEAN NOT NULL DEFAULT true,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_by TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.order_invoices (
	order_id BIGINT PRIMARY KEY,
	status TEXT NOT NULL DEFAULT 'requested'
);
INSERT INTO %s.customers(id,name,company_name) VALUES (1,'上海门店','上海门店公司');
INSERT INTO %s.pay_statuses(id,name) VALUES (1,'已付款');
INSERT INTO %s.ship_statuses(id,name) VALUES (1,'已发货');
INSERT INTO %s.order_process_statuses(id,name) VALUES (1,'无需生产');
INSERT INTO %s.orders(id,order_no,customer_id,pay_status_id,ship_status_id,process_status_id,ship_method,ship_tracking_no)
	VALUES (22,'SO-20260503-0001',1,1,1,1,'sf_small','SF123456789');
INSERT INTO %s.warehouses(code,name) VALUES ('finished_goods','成品仓');
INSERT INTO %s.delivery_note_forms(order_id,posting_date,source_warehouse,delivery_method,tracking_no)
	VALUES (22,'2026-05-03','finished_goods','sf_small','SF123456789');
INSERT INTO %s.delivery_note_documents(id,order_id,order_no,version_no,snapshot_json,is_latest,created_at,created_by)
	VALUES (11,22,'SO-20260503-0001',2,$${
		"customer_name":"上海门店",
		"posting_date":"2026-05-03",
		"source_warehouse":"finished_goods",
		"source_warehouse_name":"成品仓",
		"delivery_method":"sf_small",
		"tracking_no":"SF123456789"
	}$$::jsonb,true,'2026-05-03 10:00:00+08','stock');
INSERT INTO %s.order_invoices(order_id,status) VALUES (22,'uploaded');
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	repo := NewRepository(pool, schema)
	result, err := repo.ListOutboundLogs(ctx, stockapp.OutboundLogQuery{Q: "SO-20260503", Limit: 20})
	if err != nil {
		t.Fatalf("ListOutboundLogs: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("rows=%d, want 1: %+v", len(result.Rows), result.Rows)
	}
	row := result.Rows[0]
	if row.DocumentID != 11 || row.OrderID != 22 || row.OrderNo != "SO-20260503-0001" || row.DeliveryMethod != "顺丰发货" {
		t.Fatalf("outbound log row = %+v", row)
	}
	if row.DownloadURL != "/orders/22/delivery-notes/11.pdf" || row.LatestURL != "/orders/22/delivery-note-latest.pdf" {
		t.Fatalf("download urls = %q / %q", row.DownloadURL, row.LatestURL)
	}
	if row.PayStatus != "已付款" || row.ShipStatus != "已发货" || row.ProcessStatus != "无需生产" || row.InvoiceStatus != "uploaded" {
		t.Fatalf("status fields = %+v", row)
	}
}

func newStockTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for stock tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_stock_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func mustExecStockSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}

func stockRepositoryFunctionBody(t *testing.T, src, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(src, startMarker)
	end := strings.Index(src, endMarker)
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("function bounds not found: %q -> %q", startMarker, endMarker)
	}
	return src[start:end]
}
