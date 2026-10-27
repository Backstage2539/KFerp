package stock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	inventoryapp "orderapp/internal/application/inventory"
	stockapp "orderapp/internal/application/stock"
	inventorypg "orderapp/internal/infrastructure/postgres/inventory"

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
		ItemType: "material", ItemID: 33, Warehouse: "raw_materials",
		HasTargetQty: true, TargetQty: 1.5, UnitCode: "袋", Reason: "盘点", Operator: "jj",
	}); err == nil || !strings.Contains(err.Error(), "必须为整数") {
		t.Fatalf("fractional discrete adjustment error=%v", err)
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

func TestPR605MaterialAdjustmentUsesSelectedWarehouseAndDiscreteBatchQuantity(t *testing.T) {
	t.Run("weight adjustment preserves other warehouses", func(t *testing.T) {
		pool, schema := setupUnifiedStockDocumentTest(t)
		ctx := context.Background()
		repo := NewRepository(pool, schema)
		svc := stockapp.NewService(repo)
		mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
			INSERT INTO %s.materials(id,code,name,kind,unit,purchase_price)
			VALUES(40,'PR605-KG','多仓原料','bean','kg',40)
		`, schema))

		if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeMaterialReceipt, Operator: "purchase",
			Items: []stockapp.StockDocumentItemCommand{{
				ItemType: "material", MaterialID: 40, InventoryUnit: "kg",
				ToWarehouse: "raw_materials", QtyG: 10000, UnitCost: 40,
			}},
		}); err != nil {
			t.Fatalf("receive material: %v", err)
		}
		if _, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeMaterialTransfer, Operator: "stock",
			Items: []stockapp.StockDocumentItemCommand{{
				ItemType: "material", MaterialID: 40, InventoryUnit: "kg",
				FromWarehouse: "raw_materials", ToWarehouse: "wip", QtyG: 4000,
			}},
		}); err != nil {
			t.Fatalf("transfer material: %v", err)
		}
		adjustment, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: "material", ItemID: 40, Warehouse: "raw_materials",
			HasTargetQty: true, TargetQty: 5, UnitCode: "kg", Reason: "原料仓盘点", Operator: "counter",
		})
		if err != nil {
			t.Fatalf("adjust raw warehouse: %v", err)
		}

		var rawG, wipG, globalG, allocatedG int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT
				COALESCE(SUM(qty_g) FILTER (WHERE warehouse='raw_materials'),0)::bigint,
				COALESCE(SUM(qty_g) FILTER (WHERE warehouse='wip'),0)::bigint
			FROM %s.material_batch_locations WHERE material_id=40
		`, schema)).Scan(&rawG, &wipG); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=40`, schema)).Scan(&globalG); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(qty_change_g),0)::bigint
			FROM %s.stock_adjustment_batch_allocations WHERE adjustment_id=$1 AND warehouse='raw_materials'
		`, schema), adjustment.AdjustmentID).Scan(&allocatedG); err != nil {
			t.Fatal(err)
		}
		if rawG != 5000 || wipG != 4000 || globalG != 9000 || allocatedG != -1000 {
			t.Fatalf("raw/wip/global/allocation=%d/%d/%d/%d, want 5000/4000/9000/-1000", rawG, wipG, globalG, allocatedG)
		}
		balances, err := svc.ListMaterialBalances(ctx, stockapp.MaterialBalanceQuery{Warehouse: "raw_materials", MaterialIDs: []int64{40}})
		if err != nil || len(balances) != 1 || balances[0].BookQty != 5 {
			t.Fatalf("raw material balance=%+v err=%v, want 5kg", balances, err)
		}
	})

	t.Run("discrete packaging adjustment and cost use remaining units", func(t *testing.T) {
		pool, schema := setupUnifiedStockDocumentTest(t)
		ctx := context.Background()
		repo := NewRepository(pool, schema)
		svc := stockapp.NewService(repo)
		mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
			INSERT INTO %s.materials(id,code,name,kind,unit,purchase_price)
			VALUES(41,'PR605-BAG','离散包材','pack','袋',2)
		`, schema))

		receipt, err := svc.CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeMaterialReceipt, Operator: "purchase",
			Items: []stockapp.StockDocumentItemCommand{{
				ItemType: "material", MaterialID: 41, InventoryUnit: "袋",
				ToWarehouse: "packaging", QtyUnits: 10, UnitCost: 2,
			}},
		})
		if err != nil {
			t.Fatalf("receive packaging material: %v", err)
		}
		if len(receipt.Items) != 1 || len(receipt.Items[0].Allocations) != 1 {
			t.Fatalf("receipt allocations=%+v", receipt.Items)
		}
		batchID := receipt.Items[0].Allocations[0].MaterialBatchID
		adjustment, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: "material", ItemID: 41, Warehouse: "packaging",
			HasTargetQty: true, TargetQty: 6, UnitCode: "袋", Reason: "包材仓盘点", Operator: "counter",
		})
		if err != nil {
			t.Fatalf("adjust packaging warehouse: %v", err)
		}
		if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			AdjustmentType: "material_cost", ItemType: "material", ItemID: 41, MaterialBatchID: batchID,
			TargetUnitCost: 3, Reason: "盘点确认单价", Operator: "counter",
		}); err != nil {
			t.Fatalf("adjust discrete batch cost: %v", err)
		}

		var locationUnits, remainingUnits, globalUnits, allocatedUnits int64
		var valueChange float64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_units),0)::bigint FROM %s.material_batch_locations WHERE material_id=41 AND warehouse='packaging'`, schema)).Scan(&locationUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.material_batches WHERE id=$1`, schema), batchID).Scan(&remainingUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.materials WHERE id=41`, schema)).Scan(&globalUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_change_units),0)::bigint FROM %s.stock_adjustment_batch_allocations WHERE adjustment_id=$1`, schema), adjustment.AdjustmentID).Scan(&allocatedUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT value_change::float8 FROM %s.stock_adjustments WHERE material_batch_id=$1 AND adjustment_type='material_cost' ORDER BY id DESC LIMIT 1`, schema), batchID).Scan(&valueChange); err != nil {
			t.Fatal(err)
		}
		if locationUnits != 6 || remainingUnits != 6 || globalUnits != 6 || allocatedUnits != -4 || valueChange != 6 {
			t.Fatalf("location/remaining/global/allocation/value=%d/%d/%d/%d/%.2f, want 6/6/6/-4/6", locationUnits, remainingUnits, globalUnits, allocatedUnits, valueChange)
		}
	})
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
		CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE %s.production_bom_output_bindings(output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL);
		CREATE TABLE %s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL);
		INSERT INTO %s.production_bom_versions(id,bom_id,status) VALUES(553,883,'published');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',9,883,553,true);
		INSERT INTO %s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(20,883,'spec-6','454g袋装','袋');
		INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES(1186,553,20,'454g袋装','袋',true,6);
		DELETE FROM %s.finished_inventory WHERE product_id=9 AND spec_g=454 AND warehouse='finished_goods';
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,20,1186,0,'finished_goods',1,0);
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,
			source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator
		) VALUES('LEGACY-OPENING-9-454','finished_product',9,'橘皮乌龙',0,20,1186,
			'legacy_finished_inventory_opening',9001,'LEGACY-OPENING-9-454',0,1,0,1,'migration');
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator
		) VALUES('finished_product',9,'橘皮乌龙',0,20,1186,'finished_goods',
			'legacy_finished_inventory_opening',9001,'LEGACY-OPENING-9-454','LEGACY-OPENING-9-454',0,0,0,0,1,1,'migration');
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,sort_order,is_default,active,description)
		VALUES('finished_shop','门店成品仓','finished',45,false,true,'门店/临时销售成品仓')
		ON CONFLICT (code) DO UPDATE SET active=true;
	`, schema))

	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (9,20,1186,0,'finished_shop',0,0)
		ON CONFLICT (product_id,bom_spec_id,spec_g,warehouse) DO NOTHING;
	`, schema))

	repo := NewRepository(pool, schema)
	adjustment, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType:    "finished_product",
		ItemID:      9,
		BomSpecID:   20,
		UnitCode:    "袋",
		Warehouse:   "finished_goods",
		TargetUnits: 3,
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
		BomSpecID:      20,
		BomVariantID:   1186,
		UnitCode:       "袋",
		FromWarehouse:  "finished_goods",
		ToWarehouse:    "finished_shop",
		QtyUnits:       1,
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
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_goods'`, schema)).Scan(&finishedGoodsUnits, &finishedGoodsLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_shop'`, schema)).Scan(&shopUnits, &shopLoose); err != nil {
		t.Fatal(err)
	}
	if finishedGoodsUnits != 2 || finishedGoodsLoose != 0 || shopUnits != 1 || shopLoose != 0 {
		t.Fatalf("finished warehouses = finished_goods %d/%d, shop %d/%d; want 2/0 and 1/0", finishedGoodsUnits, finishedGoodsLoose, shopUnits, shopLoose)
	}

	var outCount, inCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='finished_product_transfer' AND source_doc_id=$1 AND warehouse='finished_goods' AND qty_change_g=0 AND qty_change_units=-1`, schema), transfer.TransferID).Scan(&outCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='finished_product_transfer' AND source_doc_id=$1 AND warehouse='finished_shop' AND qty_change_g=0 AND qty_change_units=1`, schema), transfer.TransferID).Scan(&inCount); err != nil {
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
	unit TEXT NOT NULL DEFAULT 'kg',
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

func TestFinishedProductAdjustmentReconcilesBatchesSummaryAndAudit(t *testing.T) {
	ctx := context.Background()
	pool, schema, repo := newFinishedAdjustmentTestRepo(t)
	insertFinishedAdjustmentBatch(t, ctx, pool, schema, "FP-ZERO-A", 7, 0, 14, "2025-12-30 00:00:00+00")
	insertFinishedAdjustmentBatch(t, ctx, pool, schema, "FP-ZERO-B", 7, 0, 14, "2025-12-31 00:00:00+00")
	insertFinishedAdjustmentBatch(t, ctx, pool, schema, "FP-OLD-A", 7, 454, 14, "2026-01-01 00:00:00+00")
	insertFinishedAdjustmentBatch(t, ctx, pool, schema, "FP-OLD-B", 7, 454, 14, "2026-01-02 00:00:00+00")
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(7,0,0,0,'customer_wh',28,0),(7,0,0,454,'customer_wh',28,0);
	`, schema))

	closePart := func(specG, targetUnits int64) int64 {
		t.Helper()
		result, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 7, SpecG: specG, Warehouse: "customer_wh",
			TargetUnits: targetUnits, Reason: "吴国宏仓库库存更正", Operator: "qa",
		})
		if err != nil {
			t.Fatalf("CreateAdjustment spec %d target %d: %v", specG, targetUnits, err)
		}
		return result.AdjustmentID
	}

	zeroAdjustmentID := closePart(0, 0)
	var zeroAUnits, zeroBUnits, zeroSummaryUnits, zeroAllocationCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-ZERO-A'`, schema)).Scan(&zeroAUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-ZERO-B'`, schema)).Scan(&zeroBUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=0 AND warehouse='customer_wh'`, schema)).Scan(&zeroSummaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustment_batch_allocations WHERE adjustment_id=$1 AND item_type='finished_product'`, schema), zeroAdjustmentID).Scan(&zeroAllocationCount); err != nil {
		t.Fatal(err)
	}
	if zeroAUnits != 0 || zeroBUnits != 0 || zeroSummaryUnits != 0 || zeroAllocationCount != 2 {
		t.Fatalf("legacy zero-spec clear: batches=%d/%d summary=%d allocations=%d, want 0/0/0/2", zeroAUnits, zeroBUnits, zeroSummaryUnits, zeroAllocationCount)
	}

	firstAdjustmentID := closePart(454, 14)
	var oldAUnits, oldBUnits, summaryUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-OLD-A'`, schema)).Scan(&oldAUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-OLD-B'`, schema)).Scan(&oldBUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=454 AND warehouse='customer_wh'`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if oldAUnits != 0 || oldBUnits != 14 || summaryUnits != 14 {
		t.Fatalf("after FIFO reduction: old batches=%d/%d summary=%d, want 0/14/14", oldAUnits, oldBUnits, summaryUnits)
	}
	var firstBatchCode string
	var firstBatchChange int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT batch_code,qty_change_units FROM %s.stock_adjustment_batch_allocations
		WHERE adjustment_id=$1 AND item_type='finished_product'
	`, schema), firstAdjustmentID).Scan(&firstBatchCode, &firstBatchChange); err != nil {
		t.Fatal(err)
	}
	if firstBatchCode != "FP-OLD-A" || firstBatchChange != -14 {
		t.Fatalf("FIFO allocation = %s/%d, want FP-OLD-A/-14", firstBatchCode, firstBatchChange)
	}

	secondAdjustmentID := closePart(454, 0)
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-OLD-B'`, schema)).Scan(&oldBUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=454 AND warehouse='customer_wh'`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if oldBUnits != 0 || summaryUnits != 0 {
		t.Fatalf("after zero target: old batch=%d summary=%d, want 0/0", oldBUnits, summaryUnits)
	}
	var secondBatchCode string
	var secondBatchChange int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT batch_code,qty_change_units FROM %s.stock_adjustment_batch_allocations
		WHERE adjustment_id=$1 AND item_type='finished_product'
	`, schema), secondAdjustmentID).Scan(&secondBatchCode, &secondBatchChange); err != nil {
		t.Fatal(err)
	}
	if secondBatchCode != "FP-OLD-B" || secondBatchChange != -14 {
		t.Fatalf("second FIFO allocation = %s/%d, want FP-OLD-B/-14", secondBatchCode, secondBatchChange)
	}

	for _, target := range []struct {
		specG int64
		units int64
	}{{specG: 454, units: 1}, {specG: 227, units: 4}} {
		result, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 7, SpecG: target.specG, Warehouse: "customer_wh",
			TargetUnits: target.units, Reason: "吴国宏仓库库存更正", Operator: "qa",
		})
		if err != nil {
			t.Fatalf("CreateAdjustment %dg x %d: %v", target.specG, target.units, err)
		}
		var adjustmentBatchUnits, adjustmentBatchGrams int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT remaining_units,remaining_g FROM %s.stock_batches WHERE batch_code=$1
		`, schema), fmt.Sprintf("ADJ-%010d", result.AdjustmentID)).Scan(&adjustmentBatchUnits, &adjustmentBatchGrams); err != nil {
			t.Fatal(err)
		}
		if adjustmentBatchUnits != target.units || adjustmentBatchGrams != target.specG*target.units {
			t.Fatalf("adjustment batch %dg remaining = %d/%dg, want %d/%dg", target.specG, adjustmentBatchUnits, adjustmentBatchGrams, target.units, target.specG*target.units)
		}
		var storedUnits, batchUnits, batchGrams int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units FROM %s.finished_inventory
			WHERE product_id=7 AND bom_spec_id=0 AND spec_g=$1 AND warehouse='customer_wh'
		`, schema), target.specG).Scan(&storedUnits); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(remaining_units),0)::bigint,COALESCE(SUM(remaining_g),0)::bigint
			FROM %s.stock_batches b
			LEFT JOIN LATERAL (
				SELECT warehouse FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
			) last_ledger ON true
			WHERE b.item_type='finished_product' AND b.item_id=7 AND b.spec_g=$1
			  AND COALESCE(last_ledger.warehouse,'finished_goods')='customer_wh'
		`, schema, schema), target.specG).Scan(&batchUnits, &batchGrams); err != nil {
			t.Fatal(err)
		}
		if storedUnits != target.units || batchUnits != target.units || batchGrams != target.specG*target.units {
			t.Fatalf("%dg inventory summary/batches = %d/%d/%dg, want %d/%d/%dg", target.specG, storedUnits, batchUnits, batchGrams, target.units, target.units, target.specG*target.units)
		}
	}

	var adjustmentCount, ledgerCount, auditCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id=7 AND warehouse='customer_wh'`, schema)).Scan(&adjustmentCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE item_id=7 AND warehouse='customer_wh' AND source_doc_type='stock_adjustment'`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='stock_adjustment' AND action='submit'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if adjustmentCount != 5 || ledgerCount != 5 || auditCount != 5 {
		t.Fatalf("adjustment/ledger/audit = %d/%d/%d, want 5/5/5", adjustmentCount, ledgerCount, auditCount)
	}
}

func TestFinishedProductBatchAdjustmentRollsBackWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	pool, schema, repo := newFinishedAdjustmentTestRepo(t)
	insertFinishedAdjustmentBatch(t, ctx, pool, schema, "FP-ROLLBACK", 7, 0, 3, "2026-01-01 00:00:00+00")
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(7,0,0,0,'customer_wh',3,0);
		ALTER TABLE %s.audit_logs ADD CONSTRAINT reject_stock_adjustment_audit
		CHECK (entity_type <> 'stock_adjustment');
	`, schema, schema))

	_, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 7, SpecG: 0, Warehouse: "customer_wh",
		TargetUnits: 0, Reason: "吴国宏仓库库存更正", Operator: "qa",
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}

	var batchUnits, summaryUnits, adjustmentCount, ledgerCount, allocationCount, adjustmentBatchCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_units FROM %s.stock_batches WHERE batch_code='FP-ROLLBACK'`, schema)).Scan(&batchUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=7 AND spec_g=0 AND warehouse='customer_wh'`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id=7`, schema)).Scan(&adjustmentCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='stock_adjustment'`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustment_batch_allocations`, schema)).Scan(&allocationCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE source_doc_type='stock_adjustment'`, schema)).Scan(&adjustmentBatchCount); err != nil {
		t.Fatal(err)
	}
	if batchUnits != 3 || summaryUnits != 3 || adjustmentCount != 0 || ledgerCount != 0 || allocationCount != 0 || adjustmentBatchCount != 0 {
		t.Fatalf("after rollback: batch/summary/adjustment/ledger/allocation/adjustment-batch = %d/%d/%d/%d/%d/%d, want 3/3/0/0/0/0", batchUnits, summaryUnits, adjustmentCount, ledgerCount, allocationCount, adjustmentBatchCount)
	}
}

func TestFinishedProductLegacyZeroSpecRequiresExistingBalance(t *testing.T) {
	ctx := context.Background()
	pool, schema, repo := newFinishedAdjustmentTestRepo(t)
	_, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 7, SpecG: 0, Warehouse: "customer_wh",
		TargetUnits: 0, Reason: "吴国宏仓库库存更正", Operator: "qa",
	})
	if err == nil || !strings.Contains(err.Error(), "legacy zero-spec inventory not found") {
		t.Fatalf("CreateAdjustment empty legacy zero-spec stock error = %v, want existing-balance error", err)
	}
	var adjustmentCount, ledgerCount, auditCount int64
	for query, dest := range map[string]*int64{
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments`, schema):    &adjustmentCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries`, schema): &ledgerCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs`, schema):           &auditCount,
	} {
		if err := pool.QueryRow(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if adjustmentCount != 0 || ledgerCount != 0 || auditCount != 0 {
		t.Fatalf("empty legacy adjustment wrote adjustment/ledger/audit = %d/%d/%d", adjustmentCount, ledgerCount, auditCount)
	}
}

func TestFinishedAdjustmentRejectsLegacyQuantityOnlyManualBatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,0,0,454,'finished_goods',10,0)
		ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
			source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator
		) VALUES('MANUAL-LEGACY-001','finished_product',9,'测试熟豆',0,0,454,'',0,'',4540,10,0,0,'stock');
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator
		) VALUES('finished_product',9,'测试熟豆',0,0,454,'finished_goods','manual_adjustment',0,
			'MANUAL-LEGACY-001','',0,4540,4540,0,10,10,'stock');
	`, schema, schema, schema))

	_, err := NewRepository(pool, schema).CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "finished_product", ItemID: 9, SpecG: 454, Warehouse: "finished_goods",
		TargetUnits: 9, Reason: "修复前历史盘点批次检测", Operator: "counter",
	})
	if err == nil || !strings.Contains(err.Error(), "legacy finished inventory batch requires reconciliation") {
		t.Fatalf("CreateAdjustment error = %v, want legacy batch reconciliation error", err)
	}
	var summaryUnits, adjustmentCount, ledgerCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=9 AND spec_g=454 AND warehouse='finished_goods'`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id=9`, schema)).Scan(&adjustmentCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='stock_adjustment' AND item_id=9`, schema)).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 10 || adjustmentCount != 0 || ledgerCount != 0 {
		t.Fatalf("summary/adjustment/ledger = %d/%d/%d, want 10/0/0", summaryUnits, adjustmentCount, ledgerCount)
	}
}

func TestFinishedAdjustmentRejectsSummaryOnlyFinishedInventory(t *testing.T) {
	ctx := context.Background()
	pool, schema, repo := newFinishedAdjustmentTestRepo(t)
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(7,0,0,454,'customer_wh',1,20)
	`, schema))

	_, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "finished_product", ItemID: 7, SpecG: 454, Warehouse: "customer_wh",
		TargetUnits: 3, TargetG: 20, Reason: "汇总库存无批次检测", Operator: "qa",
	})
	if err == nil || !strings.Contains(err.Error(), "finished inventory summary requires batch reconciliation") {
		t.Fatalf("CreateAdjustment summary-only error = %v, want batch reconciliation error", err)
	}
	var summaryUnits, summaryLoose, adjustmentCount, ledgerCount, batchCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=7 AND bom_spec_id=0 AND spec_g=454 AND warehouse='customer_wh'
	`, schema)).Scan(&summaryUnits, &summaryLoose); err != nil {
		t.Fatal(err)
	}
	for query, dest := range map[string]*int64{
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments`, schema):                                                             &adjustmentCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_ledger_entries WHERE source_doc_type='stock_adjustment'`, schema):                 &ledgerCount,
		fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_batches WHERE item_type='finished_product' AND item_id=7 AND spec_g=454`, schema): &batchCount,
	} {
		if err := pool.QueryRow(ctx, query).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if summaryUnits != 1 || summaryLoose != 20 || adjustmentCount != 0 || ledgerCount != 0 || batchCount != 0 {
		t.Fatalf("summary/counts = %d+%dg/%d/%d/%d, want 1+20g/0/0/0", summaryUnits, summaryLoose, adjustmentCount, ledgerCount, batchCount)
	}
}

func TestFinishedAdjustmentRejectsUnallocatedManualReductionBatch(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,0,0,454,'finished_goods',5,0)
		ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
			source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator
		) VALUES
			('PRODUCTION-REMAINDER-001','finished_product',9,'测试熟豆',0,0,454,'production_operation_event',1,'PRODUCTION-REMAINDER-001',4540,10,4540,10,'production'),
			('MANUAL-REDUCTION-001','finished_product',9,'测试熟豆',0,0,454,'manual_adjustment',0,'',-2270,-5,0,0,'stock');
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator
		) VALUES
			('finished_product',9,'测试熟豆',0,0,454,'finished_goods','production_operation_event',1,'PRODUCTION-REMAINDER-001',1,0,4540,4540,0,10,10,'production'),
			('finished_product',9,'测试熟豆',0,0,454,'finished_goods','manual_adjustment',0,'MANUAL-REDUCTION-001','',4540,-2270,2270,10,-5,5,'stock');
	`, schema, schema, schema))

	_, err := NewRepository(pool, schema).CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "finished_product", ItemID: 9, SpecG: 454, Warehouse: "finished_goods",
		TargetUnits: 4, Reason: "未分配人工减库存检测", Operator: "counter",
	})
	if err == nil || !strings.Contains(err.Error(), "legacy finished inventory batch requires reconciliation") {
		t.Fatalf("CreateAdjustment error = %v, want manual reduction reconciliation error", err)
	}
	var summaryUnits, adjustmentCount int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units FROM %s.finished_inventory WHERE product_id=9 AND spec_g=454 AND warehouse='finished_goods'`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stock_adjustments WHERE item_id=9`, schema)).Scan(&adjustmentCount); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 5 || adjustmentCount != 0 {
		t.Fatalf("summary/adjustment count=%d/%d, want 5/0", summaryUnits, adjustmentCount)
	}
}

func TestFinishedAdjustmentLocksBatchBeforeInventorySummary(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES(9,0,0,454,'finished_goods',10,0)
		ON CONFLICT(product_id,bom_spec_id,spec_g,warehouse) DO UPDATE
		SET onhand_units=excluded.onhand_units,onhand_loose_g=excluded.onhand_loose_g;
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
			source_doc_type,source_doc_id,source_batch_id,qty_g,qty_units,remaining_g,remaining_units,operator
		) VALUES('LOCK-ORDER-001','finished_product',9,'测试熟豆',0,0,454,'production_operation_event',1,'LOCK-ORDER-001',4540,10,4540,10,'production');
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator
		) VALUES('finished_product',9,'测试熟豆',0,0,454,'finished_goods','production_operation_event',1,
			'LOCK-ORDER-001',1,0,4540,4540,0,10,10,'production');
	`, schema, schema, schema))

	batchHolder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = batchHolder.Rollback(ctx) }()
	if _, err := batchHolder.Exec(ctx, fmt.Sprintf(`SELECT id FROM %s.stock_batches WHERE batch_code='LOCK-ORDER-001' FOR UPDATE`, schema)); err != nil {
		t.Fatal(err)
	}

	adjustmentCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := NewRepository(pool, schema).CreateAdjustment(adjustmentCtx, stockapp.StockAdjustmentCommand{
			ItemType: "finished_product", ItemID: 9, SpecG: 454, Warehouse: "finished_goods",
			TargetUnits: 9, Reason: "并发锁顺序验证", Operator: "counter",
		})
		done <- err
	}()

	waitDeadline := time.Now().Add(5 * time.Second)
	waitingForBatch := false
	for time.Now().Before(waitDeadline) {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname=current_database() AND state='active' AND wait_event_type='Lock'
				  AND query LIKE $1
			)
		`, "%"+schema+".stock_batches b%FOR UPDATE OF b%").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			waitingForBatch = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !waitingForBatch {
		t.Fatal("adjustment did not block while acquiring the source batch lock")
	}

	summaryProbe, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := summaryProbe.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=454 AND warehouse='finished_goods'
		FOR UPDATE NOWAIT
	`, schema)).Scan(new(int64)); err != nil {
		_ = summaryProbe.Rollback(ctx)
		t.Fatalf("summary row was locked before the batch: %v", err)
	}
	if err := summaryProbe.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := batchHolder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
}

func TestCanonicalFinishedTransferSerializesWithBatchAdjustment(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE %s.production_bom_output_bindings(output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL);
		CREATE TABLE %s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL);
		INSERT INTO %s.production_bom_versions(id,bom_id,status) VALUES(553,883,'published');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',9,883,553,true);
		INSERT INTO %s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(20,883,'spec-6','454g袋装','袋');
		INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES(1186,553,20,'454g袋装','袋',true,6);
	`, schema, schema, schema, schema, schema, schema, schema, schema))
	repo := NewRepository(pool, schema)
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, BomSpecID: 20, BomVariantID: 1186,
		UnitCode: "袋", Warehouse: "finished_goods", TargetUnits: 5, Reason: "并发转移初始化", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed finished stock: %v", err)
	}

	batchHolder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = batchHolder.Rollback(ctx) }()
	if err := batchHolder.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=9 AND bom_spec_id=20 AND remaining_units>0
		ORDER BY created_at,id LIMIT 1 FOR UPDATE
	`, schema)).Scan(new(int64)); err != nil {
		t.Fatal(err)
	}

	waitForLockQuery := func(pattern string) bool {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var waiting bool
			if err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_stat_activity
					WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query ILIKE $1
				)
			`, pattern).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}

	adjustCtx, cancelAdjust := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAdjust()
	adjustDone := make(chan error, 1)
	go func() {
		_, err := repo.CreateAdjustment(adjustCtx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 9, BomSpecID: 20, BomVariantID: 1186,
			UnitCode: "袋", Warehouse: "finished_goods", TargetUnits: 4, Reason: "并发转移调整", Operator: "qa",
		})
		adjustDone <- err
	}()
	if !waitForLockQuery("%stock_batches b%FOR UPDATE OF b%") {
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		t.Fatal("stock adjustment did not block while acquiring the source batch")
	}

	transferCtx, cancelTransfer := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTransfer()
	transferDone := make(chan error, 1)
	go func() {
		_, err := stockapp.NewService(repo).CreateAndSubmitStockDocument(transferCtx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeMaterialTransfer, Operator: "qa", Note: "并发转移锁顺序",
			Items: []stockapp.StockDocumentItemCommand{{
				ProductID: 9, ItemType: itemTypeFinishedProduct, BomSpecID: 20, BomVariantID: 1186,
				InventoryUnit: "袋", FromWarehouse: "finished_goods", ToWarehouse: "finished_shop", QtyUnits: 1,
			}},
		})
		transferDone <- err
	}()
	if !waitForLockQuery("%pg_advisory_xact_lock%") {
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		_ = <-transferDone
		t.Fatal("canonical finished transfer did not wait for the adjustment identity lock")
	}

	probe, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sourceUnits int64
	if err := probe.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_goods'
		FOR UPDATE NOWAIT
	`, schema)).Scan(&sourceUnits); err != nil {
		_ = probe.Rollback(ctx)
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		_ = <-transferDone
		t.Fatalf("transfer locked the summary before serializing with the adjustment: %v", err)
	}
	if err := probe.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := batchHolder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-adjustDone; err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
	if err := <-transferDone; err != nil {
		t.Fatalf("canonical finished transfer: %v", err)
	}

	var fromUnits, toUnits, batchUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&fromUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_shop'
	`, schema)).Scan(&toUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(b.remaining_units),0)::bigint
		FROM %s.stock_batches b LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id
			ORDER BY l.id DESC LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=20 AND COALESCE(last_ledger.warehouse,'finished_goods')='finished_goods'
	`, schema, schema)).Scan(&batchUnits); err != nil {
		t.Fatal(err)
	}
	if fromUnits != 3 || toUnits != 1 || batchUnits != fromUnits {
		t.Fatalf("source/destination/batch stock=%d/%d/%d, want 3/1/3", fromUnits, toUnits, batchUnits)
	}
}

func TestLegacyFinishedTransferMovesFIFOStockBatches(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
		TargetUnits: 6, TargetG: 250, Reason: "非 BOM 转仓初始化第一批", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed first finished batch: %v", err)
	}
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
		TargetUnits: 10, TargetG: 500, Reason: "非 BOM 转仓初始化第二批", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed second finished batch: %v", err)
	}
	transfer, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyUnits: 8, QtyLooseG: 250, Operator: "qa", IdempotencyKey: "legacy-finished-batch-transfer",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}
	if transfer.EntryID <= 0 {
		t.Fatalf("transfer = %+v, want a unified stock entry", transfer)
	}

	warehouseStock := func(warehouse string) (summaryUnits, batchUnits, summaryLooseG, batchG int64) {
		t.Helper()
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
			WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse=$1
		`, schema), warehouse).Scan(&summaryUnits, &summaryLooseG); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(b.remaining_units),0)::bigint,COALESCE(SUM(b.remaining_g),0)::bigint
			FROM %s.stock_batches b LEFT JOIN LATERAL (
				SELECT l.warehouse FROM %s.stock_ledger_entries l
				WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
				  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g
				ORDER BY l.id DESC LIMIT 1
			) last_ledger ON true
			WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=0 AND b.spec_g=1000
			  AND COALESCE(last_ledger.warehouse,'finished_goods')=$1
		`, schema, schema), warehouse).Scan(&batchUnits, &batchG); err != nil {
			t.Fatal(err)
		}
		return summaryUnits, batchUnits, summaryLooseG, batchG
	}
	fromUnits, fromBatchUnits, fromLooseG, fromBatchG := warehouseStock("finished_goods")
	toUnits, toBatchUnits, toLooseG, toBatchG := warehouseStock("finished_shop")
	if fromUnits != 2 || fromBatchUnits != fromUnits || fromLooseG != 250 || fromBatchG != 2250 ||
		toUnits != 8 || toBatchUnits != toUnits || toLooseG != 250 || toBatchG != 8250 {
		t.Fatalf("source units/looseG/batches=%d/%d/%d/%d, destination=%d/%d/%d/%d; want 2/250/2/2250 and 8/250/8/8250",
			fromUnits, fromLooseG, fromBatchUnits, fromBatchG, toUnits, toLooseG, toBatchUnits, toBatchG)
	}
	if _, err := stockapp.NewService(repo).CancelStockDocument(ctx, transfer.EntryID, "qa"); err != nil {
		t.Fatalf("cancel transfer: %v", err)
	}
	fromUnits, fromBatchUnits, fromLooseG, fromBatchG = warehouseStock("finished_goods")
	toUnits, toBatchUnits, toLooseG, toBatchG = warehouseStock("finished_shop")
	if fromUnits != 10 || fromBatchUnits != fromUnits || fromLooseG != 500 || fromBatchG != 10500 ||
		toUnits != 0 || toBatchUnits != toUnits || toLooseG != 0 || toBatchG != 0 {
		t.Fatalf("after cancel source units/looseG/batches=%d/%d/%d/%d, destination=%d/%d/%d/%d; want 10/500/10/10500 and zeros",
			fromUnits, fromLooseG, fromBatchUnits, fromBatchG, toUnits, toLooseG, toBatchUnits, toBatchG)
	}
}

func TestLegacyFinishedTransferDoesNotCreateOpeningBatchForFrozenStock(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
		TargetUnits: 10, Reason: "冻结批次转仓测试", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed finished stock: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.stock_batches SET quality_status='hold'
		WHERE item_type='finished_product' AND item_id=9 AND bom_spec_id=0 AND spec_g=1000;
	`, schema))

	_, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyUnits: 3, Operator: "qa", IdempotencyKey: "legacy-frozen-batch-transfer",
	})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("TransferFinishedProduct error = %v, want frozen-batch block", err)
	}
	var summaryUnits, sourceUnits, targetUnits, openingBatches int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
	`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatalf("read source summary: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=9 AND bom_spec_id=0 AND spec_g=1000 AND quality_status='hold'
	`, schema)).Scan(&sourceUnits); err != nil {
		t.Fatalf("read frozen source batches: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=9 AND bom_spec_id=0 AND spec_g=1000 AND source_doc_type='stock_entry_transfer'
	`, schema)).Scan(&targetUnits); err != nil {
		t.Fatalf("read transfer batches: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint FROM %s.stock_batches WHERE source_doc_type='stock_entry_transfer_opening'
	`, schema)).Scan(&openingBatches); err != nil {
		t.Fatalf("read opening batches: %v", err)
	}
	if summaryUnits != 10 || sourceUnits != 10 || targetUnits != 0 || openingBatches != 0 {
		t.Fatalf("after blocked transfer summary/source/target/opening=%d/%d/%d/%d, want 10/10/0/0", summaryUnits, sourceUnits, targetUnits, openingBatches)
	}
}

func TestLegacyFinishedTransferKeepsLooseGramsAcrossPackageBoundary(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	for _, seed := range []struct {
		warehouse string
		grams     int64
	}{
		{warehouse: "finished_goods", grams: 250},
		{warehouse: "finished_shop", grams: 750},
	} {
		if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: seed.warehouse,
			TargetG: seed.grams, Reason: "散装克重转仓测试", Operator: "qa",
		}); err != nil {
			t.Fatalf("seed loose stock in %s: %v", seed.warehouse, err)
		}
	}
	transfer, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyLooseG: 250, Operator: "qa", IdempotencyKey: "legacy-loose-package-boundary-transfer",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}
	if _, err := stockapp.NewService(repo).CancelStockDocument(ctx, transfer.EntryID, "qa"); err != nil {
		t.Fatalf("cancel transfer: %v", err)
	}
	if _, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyLooseG: 250, Operator: "qa", IdempotencyKey: "legacy-loose-package-boundary-transfer-2",
	}); err != nil {
		t.Fatalf("TransferFinishedProduct repeat: %v", err)
	}
	var summaryUnits, summaryLoose, batchUnits, batchGrams int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_shop'
	`, schema)).Scan(&summaryUnits, &summaryLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint,COALESCE(SUM(remaining_g),0)::bigint
		FROM %s.stock_batches b LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
			  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=0 AND b.spec_g=1000
		  AND COALESCE(last_ledger.warehouse,'finished_goods')='finished_shop'
	`, schema, schema)).Scan(&batchUnits, &batchGrams); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 0 || summaryLoose != 1000 || batchUnits != summaryUnits || batchGrams != summaryLoose {
		t.Fatalf("destination summary/batches=%d+%dg/%d/%dg, want 0+1000g/0/1000g", summaryUnits, summaryLoose, batchUnits, batchGrams)
	}
	if _, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_shop", ToWarehouse: "finished_goods",
		QtyLooseG: 1000, Operator: "qa", IdempotencyKey: "legacy-full-spec-loose-transfer",
	}); err != nil {
		t.Fatalf("transfer accumulated loose grams: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
	`, schema)).Scan(&summaryUnits, &summaryLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(b.remaining_units),0)::bigint,COALESCE(SUM(b.remaining_g),0)::bigint
		FROM %s.stock_batches b LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
			  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=0 AND b.spec_g=1000
		  AND COALESCE(last_ledger.warehouse,'finished_goods')='finished_goods'
	`, schema, schema)).Scan(&batchUnits, &batchGrams); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 0 || summaryLoose != 1000 || batchUnits != summaryUnits || batchGrams != summaryLoose {
		t.Fatalf("retransferred summary/batches=%d+%dg/%d/%dg, want 0+1000g/0/1000g", summaryUnits, summaryLoose, batchUnits, batchGrams)
	}
}

func TestLegacyFinishedTransferCancellationPreservesHistoricalNormalization(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	for _, seed := range []struct {
		warehouse string
		grams     int64
	}{
		{warehouse: "finished_goods", grams: 250},
		{warehouse: "finished_shop", grams: 750},
	} {
		if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: seed.warehouse,
			TargetG: seed.grams, Reason: "历史转仓撤销测试", Operator: "qa",
		}); err != nil {
			t.Fatalf("seed loose stock in %s: %v", seed.warehouse, err)
		}
	}
	transfer, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyLooseG: 250, Operator: "qa", IdempotencyKey: "legacy-historical-cancel",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}
	var sourceBatchID, targetBatchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT source_batch_id,target_batch_id FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_id=$1
	`, schema), transfer.EntryID).Scan(&sourceBatchID, &targetBatchID); err != nil {
		t.Fatalf("load transfer batch trace: %v", err)
	}
	// Model an older posted transfer: summaries were normalized and no batch-move trace existed.
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %s.stock_entry_finished_batch_moves WHERE stock_entry_id=%d;
		DELETE FROM %s.stock_batches WHERE id=%d;
		UPDATE %s.stock_batches SET remaining_g=250,remaining_units=0 WHERE id=%d;
		UPDATE %s.finished_inventory SET onhand_units=0,onhand_loose_g=0
		 WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods';
		UPDATE %s.finished_inventory SET onhand_units=1,onhand_loose_g=0
		 WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_shop';
	`, schema, transfer.EntryID, schema, targetBatchID, schema, sourceBatchID, schema, schema))
	if _, err := stockapp.NewService(repo).CancelStockDocument(ctx, transfer.EntryID, "qa"); err != nil {
		t.Fatalf("cancel historical transfer: %v", err)
	}
	var sourceUnits, sourceLoose, targetUnits, targetLoose int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
	`, schema)).Scan(&sourceUnits, &sourceLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_shop'
	`, schema)).Scan(&targetUnits, &targetLoose); err != nil {
		t.Fatal(err)
	}
	if sourceUnits != 0 || sourceLoose != 250 || targetUnits != 0 || targetLoose != 750 {
		t.Fatalf("historical transfer reversal source=%d+%dg target=%d+%dg; want 0+250g and 0+750g", sourceUnits, sourceLoose, targetUnits, targetLoose)
	}
}

func TestLegacyFinishedTransferCancellationAfterReceiptUsesBatchBalances(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
		TargetG: 750, Reason: "散克调拨撤销测试", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed source loose stock: %v", err)
	}
	transfer, err := stockapp.NewService(repo).TransferFinishedProduct(ctx, stockapp.FinishedProductTransferCommand{
		ProductID: 9, SpecG: 1000, FromWarehouse: "finished_goods", ToWarehouse: "finished_shop",
		QtyLooseG: 750, Operator: "qa", IdempotencyKey: "legacy-transfer-receipt-cancel",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations SET consumed_g=reserved_g WHERE work_order_id=88;
	`, schema))
	if _, err := stockapp.NewService(repo).CreateAndSubmitStockDocument(ctx, stockapp.StockDocumentCommand{
		Purpose: stockapp.PurposeManufacture, WorkOrderID: 88, Operator: "qa",
		Items: []stockapp.StockDocumentItemCommand{{
			ProductID: 9, ItemType: itemTypeFinishedProduct, SpecG: 1000,
			ToWarehouse: "finished_shop", QtyG: 250,
		}},
	}); err != nil {
		t.Fatalf("post manufacture receipt: %v", err)
	}
	var beforeUnits, beforeLoose int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_shop'
	`, schema)).Scan(&beforeUnits, &beforeLoose); err != nil {
		t.Fatal(err)
	}
	if beforeUnits != 1 || beforeLoose != 0 {
		t.Fatalf("manufacture receipt summary=%d+%dg, want normalized 1+0g", beforeUnits, beforeLoose)
	}
	if _, err := stockapp.NewService(repo).CancelStockDocument(ctx, transfer.EntryID, "qa"); err != nil {
		t.Fatalf("cancel transfer after receipt: %v", err)
	}
	var sourceUnits, sourceLoose, targetUnits, targetLoose int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
	`, schema)).Scan(&sourceUnits, &sourceLoose); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units,onhand_loose_g FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_shop'
	`, schema)).Scan(&targetUnits, &targetLoose); err != nil {
		t.Fatal(err)
	}
	if sourceUnits != 0 || sourceLoose != 750 || targetUnits != 0 || targetLoose != 250 {
		t.Fatalf("cancelled transfer after receipt source=%d+%dg target=%d+%dg; want 0+750g and 0+250g", sourceUnits, sourceLoose, targetUnits, targetLoose)
	}
}

func TestFinishedManufactureReceiptSerializesWithBatchAdjustment(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	if _, err := repo.CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
		TargetUnits: 10, Reason: "并发完工入库初始化", Operator: "qa",
	}); err != nil {
		t.Fatalf("seed finished stock: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET consumed_g=reserved_g WHERE work_order_id=88;
	`, schema))

	batchHolder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = batchHolder.Rollback(ctx) }()
	if err := batchHolder.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.stock_batches
		WHERE item_type='finished_product' AND item_id=9 AND bom_spec_id=0 AND spec_g=1000 AND remaining_units>0
		ORDER BY created_at,id LIMIT 1 FOR UPDATE
	`, schema)).Scan(new(int64)); err != nil {
		t.Fatal(err)
	}
	waitForLockQuery := func(pattern string) bool {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var waiting bool
			if err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_stat_activity
					WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query ILIKE $1
				)
			`, pattern).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}

	adjustCtx, cancelAdjust := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAdjust()
	adjustDone := make(chan error, 1)
	go func() {
		_, err := repo.CreateAdjustment(adjustCtx, stockapp.StockAdjustmentCommand{
			ItemType: itemTypeFinishedProduct, ItemID: 9, SpecG: 1000, Warehouse: "finished_goods",
			TargetUnits: 8, Reason: "并发完工入库盘点", Operator: "qa",
		})
		adjustDone <- err
	}()
	if !waitForLockQuery("%stock_batches b%FOR UPDATE OF b%") {
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		t.Fatal("stock adjustment did not block while acquiring the source batch")
	}

	manufactureCtx, cancelManufacture := context.WithTimeout(ctx, 10*time.Second)
	defer cancelManufacture()
	manufactureDone := make(chan error, 1)
	go func() {
		_, err := stockapp.NewService(repo).CreateAndSubmitStockDocument(manufactureCtx, stockapp.StockDocumentCommand{
			Purpose: stockapp.PurposeManufacture, WorkOrderID: 88, Operator: "qa",
			Items: []stockapp.StockDocumentItemCommand{{
				ProductID: 9, ItemType: itemTypeFinishedProduct, SpecG: 1000,
				ToWarehouse: "finished_goods", QtyUnits: 5,
			}},
		})
		manufactureDone <- err
	}()
	if !waitForLockQuery("%pg_advisory_xact_lock%") {
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		_ = <-manufactureDone
		t.Fatal("manufacture receipt did not wait for the adjustment identity lock")
	}

	probe, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := probe.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
		FOR UPDATE NOWAIT
	`, schema)).Scan(new(int64)); err != nil {
		_ = probe.Rollback(ctx)
		_ = batchHolder.Rollback(ctx)
		_ = <-adjustDone
		_ = <-manufactureDone
		t.Fatalf("manufacture locked the summary before serializing with the adjustment: %v", err)
	}
	if err := probe.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := batchHolder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-adjustDone; err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}
	if err := <-manufactureDone; err != nil {
		t.Fatalf("manufacture receipt: %v", err)
	}

	var summaryUnits, batchUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=0 AND spec_g=1000 AND warehouse='finished_goods'
	`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(b.remaining_units),0)::bigint
		FROM %s.stock_batches b LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
			  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g
			ORDER BY l.id DESC LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=0 AND b.spec_g=1000
		  AND COALESCE(last_ledger.warehouse,'finished_goods')='finished_goods'
	`, schema, schema)).Scan(&batchUnits); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 13 || batchUnits != summaryUnits {
		t.Fatalf("summary/batch stock=%d/%d, want 13/13", summaryUnits, batchUnits)
	}
}

func TestManualFinishedInventoryAdjustmentsCreateUsableStockBatches(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE %s.production_bom_output_bindings(output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL);
		CREATE TABLE %s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL);
		INSERT INTO %s.production_bom_versions(id,bom_id,status) VALUES(553,883,'published');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',9,883,553,true);
		INSERT INTO %s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(20,883,'spec-6','454g袋装','袋');
		INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES(1186,553,20,'454g袋装','袋',true,6);
	`, schema, schema, schema, schema, schema, schema, schema, schema))

	if err := inventorypg.NewRepository(pool, schema).AdjustFinished(ctx, inventoryapp.AdjustFinishedInventoryCommand{
		ProductID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods", Units: 4, Operator: "counter",
	}); err != nil {
		t.Fatalf("AdjustFinished: %v", err)
	}
	var qtyUnits, remainingUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_units,remaining_units FROM %s.stock_batches WHERE source_doc_type='manual_adjustment' AND item_id=9 AND bom_spec_id=20
	`, schema)).Scan(&qtyUnits, &remainingUnits); err != nil {
		t.Fatal(err)
	}
	if qtyUnits != 4 || remainingUnits != 4 {
		t.Fatalf("manual adjustment batch qty/remaining units=%d/%d, want 4/4", qtyUnits, remainingUnits)
	}
	if err := inventorypg.NewRepository(pool, schema).AdjustFinished(ctx, inventoryapp.AdjustFinishedInventoryCommand{
		ProductID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods", Units: 6, Operator: "counter",
	}); err != nil {
		t.Fatalf("second AdjustFinished: %v", err)
	}
	var manualBatchCount, totalRemainingUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*),COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches
		WHERE source_doc_type='manual_adjustment' AND item_id=9 AND bom_spec_id=20
	`, schema)).Scan(&manualBatchCount, &totalRemainingUnits); err != nil {
		t.Fatal(err)
	}
	if manualBatchCount != 2 || totalRemainingUnits != 6 {
		t.Fatalf("manual adjustment batches/remain = %d/%d, want 2/6", manualBatchCount, totalRemainingUnits)
	}
	if _, err := NewRepository(pool, schema).CreateAdjustment(ctx, stockapp.StockAdjustmentCommand{
		ItemType: "finished_product", ItemID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods",
		TargetUnits: 2, Reason: "手工入库批次后续盘点", Operator: "counter",
	}); err != nil {
		t.Fatalf("adjust against manual batch: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(remaining_units),0)::bigint FROM %s.stock_batches
		WHERE source_doc_type='manual_adjustment' AND item_id=9 AND bom_spec_id=20
	`, schema)).Scan(&remainingUnits); err != nil {
		t.Fatal(err)
	}
	if remainingUnits != 2 {
		t.Fatalf("manual adjustment batch remaining after count=%d, want 2", remainingUnits)
	}
}

func TestManualFinishedInventoryIncreaseSerializesWithBatchAdjustment(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %s.production_bom_versions(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE %s.production_bom_output_bindings(output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %s.production_bom_specs(id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,spec_key TEXT NOT NULL,name TEXT NOT NULL,inventory_unit TEXT NOT NULL);
		CREATE TABLE %s.production_bom_version_variants(id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL,spec_name_snapshot TEXT NOT NULL,inventory_unit TEXT NOT NULL,is_default BOOLEAN NOT NULL,sort_order INTEGER NOT NULL);
		INSERT INTO %s.production_bom_versions(id,bom_id,status) VALUES(553,883,'published');
		INSERT INTO %s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default) VALUES('product',9,883,553,true);
		INSERT INTO %s.production_bom_specs(id,bom_id,spec_key,name,inventory_unit) VALUES(20,883,'spec-6','454g袋装','袋');
		INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES(1186,553,20,'454g袋装','袋',true,6);
	`, schema, schema, schema, schema, schema, schema, schema, schema))
	inventoryRepo := inventorypg.NewRepository(pool, schema)
	if err := inventoryRepo.AdjustFinished(ctx, inventoryapp.AdjustFinishedInventoryCommand{
		ProductID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods", Units: 10, Operator: "counter",
	}); err != nil {
		t.Fatalf("seed manual inventory: %v", err)
	}

	// Hold the summary row so the manual increase pauses after it has acquired
	// the identity lock. A batch adjustment must wait for that same lock before
	// taking its batch snapshot.
	summaryHolder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = summaryHolder.Rollback(ctx) }()
	if err := summaryHolder.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_goods'
		FOR UPDATE
	`, schema)).Scan(new(int64)); err != nil {
		t.Fatal(err)
	}

	manualCtx, cancelManual := context.WithTimeout(ctx, 10*time.Second)
	defer cancelManual()
	manualDone := make(chan error, 1)
	go func() {
		manualDone <- inventoryRepo.AdjustFinished(manualCtx, inventoryapp.AdjustFinishedInventoryCommand{
			ProductID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods", Units: 15, Operator: "counter",
		})
	}()
	waitForLockQuery := func(pattern string) bool {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var waiting bool
			if err := pool.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1 FROM pg_stat_activity
					WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query ILIKE $1
				)
			`, pattern).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				return true
			}
			time.Sleep(20 * time.Millisecond)
		}
		return false
	}
	if !waitForLockQuery("%finished_inventory%FOR UPDATE%") {
		t.Fatal("manual increase did not reach the held summary row")
	}

	adjustCtx, cancelAdjust := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAdjust()
	adjustDone := make(chan error, 1)
	go func() {
		_, err := NewRepository(pool, schema).CreateAdjustment(adjustCtx, stockapp.StockAdjustmentCommand{
			ItemType: "finished_product", ItemID: 9, BomSpecID: 20, UnitCode: "袋", Warehouse: "finished_goods",
			TargetUnits: 12, Reason: "并发批次快照验证", Operator: "counter",
		})
		adjustDone <- err
	}()
	if !waitForLockQuery("%pg_advisory_xact_lock%") {
		_ = summaryHolder.Rollback(ctx)
		if err := <-manualDone; err != nil {
			t.Fatalf("manual increase: %v", err)
		}
		_ = <-adjustDone
		t.Fatal("stock adjustment did not wait for the manual batch writer before taking its snapshot")
	}
	if err := summaryHolder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-manualDone; err != nil {
		t.Fatalf("manual increase: %v", err)
	}
	if err := <-adjustDone; err != nil {
		t.Fatalf("CreateAdjustment: %v", err)
	}

	var summaryUnits, batchUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT onhand_units FROM %s.finished_inventory
		WHERE product_id=9 AND bom_spec_id=20 AND spec_g=0 AND warehouse='finished_goods'
	`, schema)).Scan(&summaryUnits); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(b.remaining_units),0)::bigint
		FROM %s.stock_batches b
		LEFT JOIN LATERAL (
			SELECT l.warehouse FROM %s.stock_ledger_entries l
			WHERE l.source_batch_code=b.batch_code AND l.item_type=b.item_type AND l.item_id=b.item_id
			  AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g ORDER BY l.id DESC LIMIT 1
		) last_ledger ON true
		WHERE b.item_type='finished_product' AND b.item_id=9 AND b.bom_spec_id=20
		  AND COALESCE(last_ledger.warehouse,'finished_goods')='finished_goods'
	`, schema, schema)).Scan(&batchUnits); err != nil {
		t.Fatal(err)
	}
	if summaryUnits != 12 || batchUnits != 12 {
		t.Fatalf("inventory summary/batches = %d/%d, want 12/12", summaryUnits, batchUnits)
	}
}

func newFinishedAdjustmentTestRepo(t *testing.T) (*pgxpool.Pool, string, Repository) {
	t.Helper()
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY, code TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT 'g', purchase_price NUMERIC(12,4) NOT NULL DEFAULT 0,
	onhand_g BIGINT NOT NULL DEFAULT 0, onhand_units BIGINT NOT NULL DEFAULT 0
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
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.warehouses(code,name,kind,active) VALUES('customer_wh','客户仓','customer',true)
		ON CONFLICT (code) DO UPDATE SET active=true;
	`, schema))
	return pool, schema, NewRepository(pool, schema)
}

func insertFinishedAdjustmentBatch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, batchCode string, productID, specG, units int64, createdAt string) {
	t.Helper()
	sourceID := time.Now().UnixNano()
	var batchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(
			batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,
			source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,operator,created_at
		) VALUES($1,'finished_product',$2,'初晓',$3,0,0,'production_operation_event',$4,$5,$6,$5,$6,'qa',$7::timestamptz)
		RETURNING id
	`, schema), batchCode, productID, specG, sourceID, units*specG, units, createdAt).Scan(&batchID); err != nil {
		t.Fatalf("insert stock batch %s: %v", batchCode, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(
			item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,warehouse,
			source_doc_type,source_doc_id,source_batch_code,source_batch_id,
			qty_before_g,qty_change_g,qty_after_g,qty_before_units,qty_change_units,qty_after_units,operator,created_at
		) VALUES('finished_product',$1,'初晓',$2,0,0,'customer_wh','production_operation_event',$3,$4,$4,0,$5,$5,0,$6,$6,'qa',$7::timestamptz)
	`, schema), productID, specG, sourceID, batchCode, units*specG, units, createdAt); err != nil {
		t.Fatalf("insert stock ledger for %s (batch %d): %v", batchCode, batchID, err)
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
