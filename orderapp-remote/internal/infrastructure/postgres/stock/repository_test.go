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
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema second pass: %v", err)
	}

	repo := NewRepository(pool, schema)
	res, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID:                1,
		Supplier:                  "云南供应商",
		QtyG:                      1200,
		UnitCost:                  42.5,
		CropSeason:                "2025/26",
		Origin:                    "云南保山",
		ProducerFlavorDescription: "李子、红糖",
		Operator:                  "jj",
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
	var receiptCropSeason, batchOrigin, batchProducerFlavor string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT crop_season FROM %s.material_receipts WHERE id=$1`, schema), res.ReceiptID).Scan(&receiptCropSeason); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT origin,producer_flavor_description FROM %s.material_batches WHERE id=$1`, schema), res.BatchID).Scan(&batchOrigin, &batchProducerFlavor); err != nil {
		t.Fatal(err)
	}
	if receiptCropSeason != "2025/26" || batchOrigin != "云南保山" || batchProducerFlavor != "李子、红糖" {
		t.Fatalf("receipt/batch metadata = %q/%q/%q", receiptCropSeason, batchOrigin, batchProducerFlavor)
	}

	listed, err := repo.ListMaterialBatches(ctx, stockapp.MaterialBatchQuery{Limit: 20})
	if err != nil {
		t.Fatalf("ListMaterialBatches: %v", err)
	}
	var receiptBatchFound, legacyBatchFound bool
	for _, row := range listed.Rows {
		if row.ID == res.BatchID {
			receiptBatchFound = row.CropSeason == "2025/26" &&
				row.Origin == "云南保山" &&
				row.ProducerFlavorDescription == "李子、红糖"
		}
		if row.BatchCode == "LEGACY-MAT-0000000001" {
			legacyBatchFound = row.Supplier == "legacy_onhand" && row.RemainingG == 300
		}
	}
	if len(listed.Rows) != 2 || !receiptBatchFound || !legacyBatchFound {
		t.Fatalf("listed material batch metadata = %+v", listed.Rows)
	}
}

func TestLegacyReceiveMaterialValidatesInventoryUnitAndQuantityDimension(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,code TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'bean',
	unit TEXT NOT NULL DEFAULT 'kg',purchase_price NUMERIC(12,2) NOT NULL DEFAULT 0,
	sale_price NUMERIC(12,2) NOT NULL DEFAULT 0,onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,min_level_g BIGINT NOT NULL DEFAULT 0,
	min_level_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.products (id BIGINT PRIMARY KEY,name TEXT NOT NULL);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id,spec_g)
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',
	entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,
	meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.materials(id,code,name,unit) VALUES
	(30,'RAW-KG','千克原料','kg'),(31,'BAG-UNIT','计数包材','袋');
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	repo := NewRepository(pool, schema)

	if _, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 30, QtyG: 1000, UnitCost: 288, Operator: "legacy-purchase",
	}); err != nil {
		t.Fatalf("legacy empty unit_code compatibility: %v", err)
	}
	for _, tc := range []struct {
		name string
		cmd  stockapp.MaterialReceiptCommand
		want string
	}{
		{"kg plus g rejected", stockapp.MaterialReceiptCommand{MaterialID: 30, UnitCode: "g", QtyG: 1000, UnitCost: 288}, "库存单位必须与物料档案一致"},
		{"bag plus box rejected", stockapp.MaterialReceiptCommand{MaterialID: 31, UnitCode: "盒", QtyUnits: 1, UnitCost: 2}, "库存单位必须与物料档案一致"},
		{"kg with count rejected", stockapp.MaterialReceiptCommand{MaterialID: 30, UnitCode: "kg", QtyUnits: 1, UnitCost: 288}, "重量数量"},
		{"bag with weight rejected", stockapp.MaterialReceiptCommand{MaterialID: 31, UnitCode: "袋", QtyG: 1000, UnitCost: 2}, "计数数量"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.ReceiveMaterial(ctx, tc.cmd); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ReceiveMaterial error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestMaterialQuantityAdjustmentUsesLockedMaterialInventoryUnit(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,unit) VALUES
			(32,'ADJ-KG','盘点千克原料','kg'),
			(33,'ADJ-BAG','盘点计数包材','袋'),
			(34,'ADJ-KG-LEGACY','旧接口千克原料','kg'),
			(35,'ADJ-BAG-LEGACY','旧接口计数包材','袋');
	`, schema))

	for _, tc := range []struct {
		name       string
		cmd        stockapp.StockAdjustmentCommand
		materialID int64
	}{
		{
			name: "kg plus g rejected",
			cmd: stockapp.StockAdjustmentCommand{
				ItemType: "material", ItemID: 32, Warehouse: "raw_materials",
				HasTargetQty: true, TargetQty: 1, UnitCode: "g", Reason: "盘点", Operator: "jj",
			},
			materialID: 32,
		},
		{
			name: "bag plus box rejected",
			cmd: stockapp.StockAdjustmentCommand{
				ItemType: "material", ItemID: 33, Warehouse: "raw_materials",
				HasTargetQty: true, TargetQty: 2, UnitCode: "盒", Reason: "盘点", Operator: "jj",
			},
			materialID: 33,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.CreateAdjustment(ctx, tc.cmd); err == nil || !strings.Contains(err.Error(), "库存单位必须与物料档案一致") {
				t.Fatalf("CreateAdjustment error = %v, want master unit mismatch", err)
			}
			var onhandG, onhandUnits int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,onhand_units FROM %s.materials WHERE id=$1`, schema), tc.materialID).Scan(&onhandG, &onhandUnits); err != nil {
				t.Fatal(err)
			}
			if onhandG != 0 || onhandUnits != 0 {
				t.Fatalf("failed adjustment changed stock to %dg/%d units", onhandG, onhandUnits)
			}
		})
	}

	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "material", ItemID: 32, Warehouse: "raw_materials",
		HasTargetQty: true, TargetQty: 2.5, Reason: "盘点", Operator: "jj",
	}); err != nil {
		t.Fatalf("empty unit_code falls back to kg master: %v", err)
	}
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "material", ItemID: 33, Warehouse: "raw_materials",
		HasTargetQty: true, TargetQty: 4, Reason: "盘点", Operator: "jj",
	}); err != nil {
		t.Fatalf("empty unit_code falls back to count master: %v", err)
	}
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "material", ItemID: 34, Warehouse: "raw_materials",
		TargetG: 1500, Reason: "旧接口盘点", Operator: "legacy",
	}); err != nil {
		t.Fatalf("legacy target_g compatibility: %v", err)
	}
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "material", ItemID: 35, Warehouse: "raw_materials",
		TargetUnits: 6, Reason: "旧接口盘点", Operator: "legacy",
	}); err != nil {
		t.Fatalf("legacy target_units compatibility: %v", err)
	}
	for _, want := range []struct {
		id       int64
		qtyG     int64
		qtyUnits int64
	}{{32, 2500, 0}, {33, 0, 4}, {34, 1500, 0}, {35, 0, 6}} {
		var qtyG, qtyUnits int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,onhand_units FROM %s.materials WHERE id=$1`, schema), want.id).Scan(&qtyG, &qtyUnits); err != nil {
			t.Fatal(err)
		}
		if qtyG != want.qtyG || qtyUnits != want.qtyUnits {
			t.Fatalf("material %d stock = %dg/%d units, want %dg/%d units", want.id, qtyG, qtyUnits, want.qtyG, want.qtyUnits)
		}
	}
}

func TestReceiveMaterialStoresNonWeightInventoryUnits(t *testing.T) {
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
	kind TEXT NOT NULL DEFAULT 'packaging',
	unit TEXT NOT NULL DEFAULT 'box',
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
	name TEXT NOT NULL,
	customer_id BIGINT NOT NULL DEFAULT 0
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
INSERT INTO %s.materials(id,code,name,onhand_units) VALUES (2,'BOX-1','挂耳盒',5);
`, schema, schema, schema, schema, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %s.business_groups (id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.business_group_items (id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %s.business_group_assignments (
			id BIGSERIAL PRIMARY KEY,group_id BIGINT NOT NULL DEFAULT 0,group_item_id BIGINT NOT NULL DEFAULT 0,
			usage_key TEXT NOT NULL DEFAULT '',object_key TEXT NOT NULL DEFAULT '',object_id BIGINT NOT NULL DEFAULT 0,object_ref TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.production_bom_specs (
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL DEFAULT 0,name TEXT NOT NULL DEFAULT '',inventory_unit TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %s.production_bom_version_variants (
			id BIGINT PRIMARY KEY,bom_spec_id BIGINT NOT NULL DEFAULT 0,spec_name_snapshot TEXT NOT NULL DEFAULT '',inventory_unit TEXT NOT NULL DEFAULT ''
		);
	`, schema, schema, schema, schema, schema))

	repo := NewRepository(pool, schema)
	res, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{
		MaterialID: 2,
		Supplier:   "包材供应商",
		QtyUnits:   12,
		UnitCost:   1.5,
		Operator:   "jj",
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial non-weight units: %v", err)
	}

	var onhandG, onhandUnits, batchRemainingG, batchRemainingUnits, locationG, locationUnits, ledgerUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g,onhand_units FROM %s.materials WHERE id=2`, schema)).Scan(&onhandG, &onhandUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g,remaining_units FROM %s.material_batches WHERE id=$1`, schema), res.BatchID).Scan(&batchRemainingG, &batchRemainingUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g,qty_units FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='raw_materials'`, schema), res.BatchID).Scan(&locationG, &locationUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_change_units FROM %s.stock_ledger_entries WHERE source_doc_type='material_receipt' AND source_doc_id=$1`, schema), res.ReceiptID).Scan(&ledgerUnits); err != nil {
		t.Fatal(err)
	}
	if onhandG != 0 || onhandUnits != 17 || batchRemainingG != 0 || batchRemainingUnits != 12 || locationG != 0 || locationUnits != 12 || ledgerUnits != 12 {
		t.Fatalf("unit receipt state = onhand %dg/%d units batch %dg/%d units location %dg/%d units ledger %d units, want 0g/17 units and 12-unit batch/location/ledger", onhandG, onhandUnits, batchRemainingG, batchRemainingUnits, locationG, locationUnits, ledgerUnits)
	}

	inventory, err := repo.ListWarehouseInventory(ctx, stockapp.WarehouseInventoryQuery{Warehouse: "raw_materials", ItemType: "material", Limit: 20})
	if err != nil {
		t.Fatalf("ListWarehouseInventory: %v", err)
	}
	if len(inventory.Rows) != 1 || inventory.Rows[0].QtyG != 0 || inventory.Rows[0].QtyUnits != 12 {
		t.Fatalf("warehouse inventory rows = %+v, want one 12-unit material row", inventory.Rows)
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
	for _, tc := range []struct {
		table  string
		column string
	}{
		{table: "material_receipts", column: "crop_season"},
		{table: "material_receipts", column: "origin"},
		{table: "material_receipts", column: "producer_flavor_description"},
		{table: "material_batches", column: "crop_season"},
		{table: "material_batches", column: "origin"},
		{table: "material_batches", column: "producer_flavor_description"},
	} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema=$1 AND table_name=$2 AND column_name=$3
			)
		`, schema, tc.table, tc.column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("%s missing %s column", tc.table, tc.column)
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
		ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO NOTHING;
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

func TestBOMSpecFinishedInventoryAdjustmentAndTransferKeepCanonicalIdentity(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
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
INSERT INTO %[1]s.products(id,name) VALUES(7,'规格组商品');
INSERT INTO %[1]s.production_bom_versions(id,bom_id,status) VALUES(41,31,'published');
INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
VALUES('product',7,31,41,true);
INSERT INTO %[1]s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit)
VALUES(91,31,'bag-227','227g袋','袋'),(92,31,'bag-454','454g袋','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
VALUES(191,41,91,'227g袋','袋',true,10),(192,41,92,'454g袋','袋',false,20);
`, schema))
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,sort_order,is_default,active,description)
		VALUES('finished_shop','门店成品仓','finished',45,false,true,'门店成品仓')
		ON CONFLICT (code) DO UPDATE SET active=true;
	`, schema))

	repo := NewRepository(pool, schema)
	for _, cmd := range []stockapp.StockAdjustmentCommand{
		{ItemType: itemTypeFinishedProduct, ItemID: 7, BomSpecID: 91, BomVariantID: 191, Warehouse: "finished_goods", TargetUnits: 12, Reason: "规格期初", Operator: "qa"},
		{ItemType: itemTypeFinishedProduct, ItemID: 7, BomSpecID: 92, BomVariantID: 192, Warehouse: "finished_goods", TargetUnits: 5, Reason: "规格期初", Operator: "qa"},
	} {
		if _, err := repo.CreateAdjustment(ctx, cmd); err != nil {
			t.Fatalf("CreateAdjustment(%d): %v", cmd.BomSpecID, err)
		}
	}
	service := stockapp.NewService(repo)
	serviceTransfer, err := service.TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 7, BomSpecID: 91, BomVariantID: 191, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyUnits: 4, Operator: "qa", IdempotencyKey: "bom-spec-unified-transfer-91",
	})
	if err != nil {
		t.Fatalf("unified TransferFinishedProduct: %v", err)
	}
	if serviceTransfer.EntryID <= 0 || serviceTransfer.BomSpecID != 91 || serviceTransfer.BomVariantID != 191 || serviceTransfer.SpecG != 0 {
		t.Fatalf("unified transfer identity = %+v", serviceTransfer)
	}
	var unifiedSource91, unifiedSource92, unifiedTarget91, entrySpecID, entryVariantID, entryQtyG, entryQtyUnits, entryAuditCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=91 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&unifiedSource91); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=92 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&unifiedSource92); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=91 AND spec_g=0 AND warehouse='finished_shop'
	`, schema)).Scan(&unifiedTarget91); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_spec_id,bom_variant_id,qty_g,qty_units FROM %s.stock_entry_items WHERE stock_entry_id=$1
	`, schema), serviceTransfer.EntryID).Scan(&entrySpecID, &entryVariantID, &entryQtyG, &entryQtyUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.audit_logs
		WHERE entity_type='stock_entry' AND entity_id=$1 AND action='submit'
	`, schema), serviceTransfer.EntryID).Scan(&entryAuditCount); err != nil {
		t.Fatal(err)
	}
	var sourceBatchUnits, targetBatchUnits, moveCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches b
		WHERE item_type='finished_product' AND item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_adjustment'
	`, schema)).Scan(&sourceBatchUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches b
		WHERE item_type='finished_product' AND item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_entry_transfer'
	`, schema)).Scan(&targetBatchUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_id=$1 AND bom_spec_id=91 AND qty_units=4
	`, schema), serviceTransfer.EntryID).Scan(&moveCount); err != nil {
		t.Fatal(err)
	}
	if unifiedSource91 != 8 || unifiedSource92 != 5 || unifiedTarget91 != 4 || entrySpecID != 91 || entryVariantID != 191 || entryQtyG != 0 || entryQtyUnits != 4 || entryAuditCount != 1 || sourceBatchUnits != 8 || targetBatchUnits != 4 || moveCount != 1 {
		t.Fatalf("unified inventory=%d/%d/%d entry=%d/%d g=%d units=%d audit=%d batches=%d/%d moves=%d", unifiedSource91, unifiedSource92, unifiedTarget91, entrySpecID, entryVariantID, entryQtyG, entryQtyUnits, entryAuditCount, sourceBatchUnits, targetBatchUnits, moveCount)
	}
	inventory, err := repo.ListWarehouseInventory(ctx, stockapp.WarehouseInventoryQuery{
		ItemType: "finished_product",
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("ListWarehouseInventory: %v", err)
	}
	seenSpecs := map[int64]bool{}
	for _, row := range inventory.Rows {
		if row.ItemID != 7 || row.BomSpecID <= 0 {
			continue
		}
		if row.BomSpecName != map[int64]string{91: "227g袋", 92: "454g袋"}[row.BomSpecID] {
			t.Fatalf("warehouse inventory BOM spec name row = %+v", row)
		}
		if row.InventoryUnit != "袋" {
			t.Fatalf("warehouse inventory unit row = %+v", row)
		}
		seenSpecs[row.BomSpecID] = true
	}
	if !seenSpecs[91] || !seenSpecs[92] {
		t.Fatalf("warehouse inventory BOM specs = %+v; want 91 and 92", seenSpecs)
	}
	if _, err := service.CancelStockDocument(ctx, serviceTransfer.EntryID, "qa"); err != nil {
		t.Fatalf("CancelStockDocument: %v", err)
	}
	var restoredSource, restoredTarget, restoredSourceBatch, restoredTargetBatch int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=91 AND warehouse='finished_goods'`, schema)).Scan(&restoredSource); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND bom_spec_id=91 AND warehouse='finished_shop'`, schema)).Scan(&restoredTarget); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches WHERE item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_adjustment'`, schema)).Scan(&restoredSourceBatch); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches WHERE item_id=7 AND bom_spec_id=91 AND source_doc_type='stock_entry_transfer'`, schema)).Scan(&restoredTargetBatch); err != nil {
		t.Fatal(err)
	}
	if restoredSource != 12 || restoredTarget != 0 || restoredSourceBatch != 12 || restoredTargetBatch != 0 {
		t.Fatalf("restored inventory=%d/%d batches=%d/%d", restoredSource, restoredTarget, restoredSourceBatch, restoredTargetBatch)
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
ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO UPDATE SET onhand_units=2,onhand_loose_g=0;
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

func TestGetStockTraceBackfillsBlankMaterialBatchFromLedger(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL DEFAULT '',
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
CREATE TABLE %s.production_logs (
	running_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	order_nos TEXT NOT NULL DEFAULT '',
	input_g BIGINT NOT NULL DEFAULT 0,
	finished_total_g BIGINT NOT NULL DEFAULT 0,
	actual_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	started_by TEXT NOT NULL DEFAULT '',
	finished_by TEXT NOT NULL DEFAULT '',
	finished_at TIMESTAMPTZ
);
CREATE TABLE %s.work_orders (
	work_order_no TEXT NOT NULL DEFAULT '',
	running_item_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.material_consumption_logs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	material_name TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '',
	deduct_g BIGINT NOT NULL DEFAULT 0,
	deduct_units BIGINT NOT NULL DEFAULT 0,
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	material_batch_code TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.material_batches (
	id BIGINT PRIMARY KEY,
	batch_code TEXT NOT NULL DEFAULT ''
);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,quality_status,operator,created_at)
VALUES ('FP-TRACE','finished_product',9,'红岩拼配',454,'production_run',99,'PB-99',908,2,908,2,'unchecked','qa',now());
INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_doc_id,source_batch_code,source_batch_id,qty_before_g,qty_change_g,qty_after_g,operator,created_at)
VALUES
	('finished_product',9,'红岩拼配',454,'finished_goods','production_run',99,'FP-TRACE','PB-99',0,908,908,'qa',now()),
	('material',1,'卡蒂姆水洗',0,'wip','production_run',99,'MB-TRACE','PB-99',2000,-1000,1000,'qa',now());
INSERT INTO %s.production_logs(running_item_id,batch_id,order_nos,input_g,finished_total_g,actual_yield_rate,started_by,finished_by,finished_at)
VALUES (99,'PB-99','SO-99',1000,908,0.908,'start','finish',now());
INSERT INTO %s.work_orders(work_order_no,running_item_id) VALUES ('WO-99',99);
INSERT INTO %s.material_consumption_logs(running_item_id,material_id,material_name,unit,deduct_g,deduct_units,material_batch_id,material_batch_code)
VALUES (99,1,'卡蒂姆水洗','g',1000,0,0,'');
INSERT INTO %s.material_batches(id,batch_code) VALUES (7,'MB-TRACE');
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	trace, err := NewRepository(pool, schema).GetStockTrace(ctx, stockapp.StockTraceQuery{BatchCode: "FP-TRACE"})
	if err != nil {
		t.Fatalf("GetStockTrace: %v", err)
	}
	if len(trace.Materials) != 1 {
		t.Fatalf("trace materials = %+v, want one material", trace.Materials)
	}
	if trace.Materials[0].MaterialBatchCode != "MB-TRACE" || trace.Materials[0].MaterialBatchID != 7 {
		t.Fatalf("trace material batch = %+v, want MB-TRACE id 7", trace.Materials[0])
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
