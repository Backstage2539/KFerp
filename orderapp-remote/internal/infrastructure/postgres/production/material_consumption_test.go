package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialNeedToDeduct(t *testing.T) {
	cases := []struct {
		unit      string
		qty       int64
		wantG     int64
		wantUnits int64
	}{
		{unit: "g", qty: 250, wantG: 250},
		{unit: "kg", qty: 2, wantG: 2000},
		{unit: "克", qty: 300, wantG: 300},
		{unit: "个", qty: 7, wantUnits: 7},
		{unit: "张", qty: 5, wantUnits: 5},
	}

	for _, tc := range cases {
		gotG, gotUnits := materialNeedToDeduct(tc.unit, tc.qty)
		if gotG != tc.wantG || gotUnits != tc.wantUnits {
			t.Fatalf("materialNeedToDeduct(%q,%d) = %d/%d, want %d/%d", tc.unit, tc.qty, gotG, gotUnits, tc.wantG, tc.wantUnits)
		}
	}
}

func TestWorkOrderRemainingWIPShortageSubtractsConsumedBeforeAvailable(t *testing.T) {
	if got := workOrderRemainingWIPShortage(7751, 2000, 5751); got != 0 {
		t.Fatalf("shortage = %d, want 0 after 2000g consumed and 5751g available", got)
	}
	if got := workOrderRemainingWIPShortage(8, 2, 3); got != 3 {
		t.Fatalf("count shortage = %d, want 3", got)
	}
}

func TestMaterialSnapshotNeedsPreserveExactKilograms(t *testing.T) {
	tests := []struct {
		name           string
		snapshot       string
		wantDeductG    int64
		wantQtyDecimal float64
	}{
		{
			name: "ratio",
			snapshot: `[{
				"material_id":9001,
				"material_name":"如目达摩生豆",
				"unit":"kg",
				"source":"bom",
				"consume_unit":"ratio_pct",
				"ratio_pct":100,
				"output_qty":1,
				"output_unit":"kg"
			}]`,
			wantDeductG:    1816,
			wantQtyDecimal: 1.816,
		},
		{
			name: "fixed quantity per kilogram output",
			snapshot: `[{
				"material_id":9001,
				"material_name":"如目达摩生豆",
				"unit":"kg",
				"source":"bom",
				"consume_unit":"fixed_qty",
				"qty_per_unit":1,
				"output_qty":1,
				"output_unit":"kg"
			}]`,
			wantDeductG:    1816,
			wantQtyDecimal: 1.816,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needs, ok, err := materialSnapshotNeedsTx(ProduceRunRow{
				ProductID:        789,
				Product:          "如目达摩",
				SpecG:            454,
				NeedG:            1816,
				InputG:           1816,
				MaterialSnapshot: tt.snapshot,
			}, InvQty{Units: 4})
			if err != nil {
				t.Fatal(err)
			}
			if !ok || len(needs) != 1 {
				t.Fatalf("material needs=%+v ok=%v, want one frozen need", needs, ok)
			}
			if needs[0].DeductG != tt.wantDeductG || needs[0].Qty != 2 {
				t.Fatalf("material need=%+v, want %dg with compatibility qty=2", needs[0], tt.wantDeductG)
			}
			if diff := needs[0].QtyDecimal - tt.wantQtyDecimal; diff < -0.0000001 || diff > 0.0000001 {
				t.Fatalf("qty decimal=%v, want %v", needs[0].QtyDecimal, tt.wantQtyDecimal)
			}
		})
	}
}

func TestMaterialLossIsAppliedExactlyOnceToExactWeight(t *testing.T) {
	got := componentConsumptionWeightGramsWithMaterialLoss(
		"ratio_pct", 0, 100, "kg",
		1816, 1816, 4, 0,
		1, "kg", 0.2,
	)
	if got != 2270 {
		t.Fatalf("loss-adjusted weight=%dg, want 2270g", got)
	}
}

func TestFixedPoundComponentUsesExactGramFactor(t *testing.T) {
	got := componentConsumptionWeightGramsWithMaterialLoss(
		"fixed_qty", 1, 0, "lb",
		1000, 1000, 0, 0,
		1, "kg", 0,
	)
	if got != 454 {
		t.Fatalf("one pound fixed component=%dg, want ceil(453.59237)=454g", got)
	}
}

func TestBomOutputBasisFactorConvertsPoundOutputFromFrozenGrams(t *testing.T) {
	got := bomOutputBasisFactor(907, 2, 1, "lb")
	want := float64(907) / 453.59237
	if diff := got - want; diff < -0.0000001 || diff > 0.0000001 {
		t.Fatalf("pound output factor=%v, want %v", got, want)
	}
}

func TestIsWeightMaterialUnit(t *testing.T) {
	for _, unit := range []string{"g", "kg", "lb", "克", "千克", "磅"} {
		if !isWeightMaterialUnit(unit) {
			t.Fatalf("expected %q to be weight unit", unit)
		}
	}
	if isWeightMaterialUnit("个") {
		t.Fatalf("expected 个 not to be weight unit")
	}
}

func TestComponentConsumptionQtyGrossesRatioMaterialLoss(t *testing.T) {
	got := componentConsumptionQtyWithMaterialLoss("ratio_pct", 0, 40, "g", 1000, 0, 0, 0, 0, "", 0.2)
	if got != 500 {
		t.Fatalf("ratio material loss quantity = %d, want 500g", got)
	}
	withoutLoss := componentConsumptionQtyWithMaterialLoss("ratio_pct", 0, 40, "g", 1000, 0, 0, 0, 0, "", 0)
	if withoutLoss != 400 {
		t.Fatalf("ratio without material loss = %d, want 400g", withoutLoss)
	}
	fixed := componentConsumptionQtyWithMaterialLoss("g", 125, 0, "g", 1000, 1000, 0, 0, 1, "kg", 0.2)
	if fixed != 125 {
		t.Fatalf("fixed quantity should ignore material loss, got %d", fixed)
	}
}

func TestMarshalMaterialConsumptionSummary(t *testing.T) {
	got, err := marshalMaterialConsumptionSummary([]materialConsumptionSummaryItem{
		{MaterialID: 1, MaterialName: "卡蒂姆水洗", Unit: "g", DeductG: 1200, BatchCode: "MB-0000000001"},
		{MaterialID: 9, MaterialName: "豆袋", Unit: "个", DeductUnits: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, needle := range []string{`"material_id":1`, `"material_name":"卡蒂姆水洗"`, `"deduct_g":1200`, `"batch_code":"MB-0000000001"`, `"material_name":"豆袋"`, `"deduct_units":8`} {
		if !strings.Contains(text, needle) {
			t.Fatalf("material summary json missing %q in %s", needle, text)
		}
	}
}

func TestMaterialBatchAllocationsConsumeOnlyWIPLocation(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	supplier TEXT NOT NULL DEFAULT '',
	receipt_id BIGINT NOT NULL DEFAULT 0,
	qty_g BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	note TEXT NOT NULL DEFAULT '',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,
	batch_code TEXT NOT NULL DEFAULT '',
	material_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(material_batch_id, warehouse)
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
	unit_cost NUMERIC(12,4) NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.material_batches(id,batch_code,material_id,qty_g,remaining_g,received_at)
VALUES (1,'MB-OLD',7,1000,1000,now() - interval '1 day');
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES (1,'MB-OLD',7,'raw_materials',1000);
INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,qty_g,remaining_g)
VALUES ('MB-OLD','material',7,'水洗豆',1000,1000);
`, schema, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = materialBatchAllocationsTx(ctx, tx, schema, 7, 600)
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("expected WIP stock insufficient error when only raw warehouse has the batch")
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %s.material_batch_locations SET qty_g=300 WHERE material_batch_id=1 AND warehouse='raw_materials';
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES (1,'MB-OLD',7,'wip',700);
`, schema, schema))

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	alloc, err := materialBatchAllocationsTx(ctx, tx, schema, 7, 600)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("materialBatchAllocationsTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if len(alloc) != 1 || alloc[0].BatchCode != "MB-OLD" || alloc[0].QtyG != 600 {
		t.Fatalf("alloc = %+v, want MB-OLD 600g", alloc)
	}

	var rawG, wipG, remainingG, stockBatchRemainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=1 AND warehouse='raw_materials'`, schema)).Scan(&rawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=1 AND warehouse='wip'`, schema)).Scan(&wipG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=1`, schema)).Scan(&remainingG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.stock_batches WHERE batch_code='MB-OLD'`, schema)).Scan(&stockBatchRemainingG); err != nil {
		t.Fatal(err)
	}
	if rawG != 300 || wipG != 100 || remainingG != 400 || stockBatchRemainingG != 400 {
		t.Fatalf("raw/wip/material/stock batch = %d/%d/%d/%d, want 300/100/400/400", rawG, wipG, remainingG, stockBatchRemainingG)
	}
}

func TestMaterialBatchAllocationsSkipFrozenWIPBatches(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	qty_g BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,
	batch_code TEXT NOT NULL DEFAULT '',
	material_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(material_batch_id, warehouse)
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	remaining_g BIGINT NOT NULL DEFAULT 0
);
INSERT INTO %s.material_batches(id,batch_code,material_id,qty_g,remaining_g,quality_status,received_at)
VALUES (1,'MB-HOLD',7,1000,1000,'hold',now() - interval '1 day');
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES (1,'MB-HOLD',7,'wip',1000);
INSERT INTO %s.stock_batches(batch_code,remaining_g) VALUES ('MB-HOLD',1000);
`, schema, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = materialBatchAllocationsTx(ctx, tx, schema, 7, 600)
	_ = tx.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), "quality") {
		t.Fatalf("materialBatchAllocationsTx frozen error = %v, want quality block", err)
	}
}

func TestEnsureWIPStockForNeedsTxAggregatesAllShortages(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.material_batches (
	id BIGSERIAL PRIMARY KEY,
	batch_code TEXT NOT NULL UNIQUE,
	material_id BIGINT NOT NULL,
	qty_g BIGINT NOT NULL DEFAULT 0,
	remaining_g BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	quality_status TEXT NOT NULL DEFAULT 'unchecked',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,
	batch_code TEXT NOT NULL DEFAULT '',
	material_id BIGINT NOT NULL DEFAULT 0,
	warehouse TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(material_batch_id, warehouse)
);
CREATE TABLE %s.work_order_material_reservations (
	id BIGSERIAL PRIMARY KEY,
	material_id BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved'
);
INSERT INTO %s.material_batches(id,batch_code,material_id,qty_g,remaining_g)
VALUES
	(1,'MB-CATIM',11,200,200),
	(2,'MB-GESHA',12,120,120);
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
VALUES
	(1,'MB-CATIM',11,'wip',200),
	(2,'MB-GESHA',12,'wip',120);
`, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = ensureWIPStockForNeedsTx(ctx, tx, schema, []materialConsumptionNeed{
		{MaterialID: 11, MaterialName: "卡蒂姆", DeductG: 500},
		{MaterialID: 12, MaterialName: "瑰夏", DeductG: 300},
	})
	_ = tx.Rollback(ctx)
	if err == nil {
		t.Fatal("expected aggregated WIP stock insufficient error")
	}
	msg := err.Error()
	for _, needle := range []string{
		"WIP stock insufficient:",
		"卡蒂姆 need 500g, available 200g, reserved 0g",
		"瑰夏 need 300g, available 120g, reserved 0g",
		"transfer raw material to WIP before starting production",
	} {
		if !strings.Contains(msg, needle) {
			t.Fatalf("shortage message %q missing %q", msg, needle)
		}
	}
}

func TestWorkOrderWIPCoverageSupportsWeightCountAndOtherReservations(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.material_batches (
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,
	remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,
	qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.work_order_material_reservations (
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved'
);
INSERT INTO %s.material_batches VALUES
	(1,'WEIGHT',11,1000,0,'active','pass'),
	(2,'COUNT',12,0,10,'active','pass'),
	(3,'HOLD',11,500,0,'active','hold');
INSERT INTO %s.material_batch_locations VALUES
	(1,11,'wip',1000,0),(2,12,'wip',0,10),(3,11,'wip',500,0);
INSERT INTO %s.work_order_material_reservations VALUES
	(1,88,11,300,0,200,0,0,0,'reserved'),
	(2,89,11,200,0,0,0,0,0,'reserved'),
	(3,89,12,0,4,0,0,0,0,'reserved');
`, schema, schema, schema, schema, schema, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := workOrderWIPCoverageForNeedsTx(ctx, tx, schema, 88, []materialConsumptionNeed{
		{MaterialID: 11, MaterialName: "生豆", Unit: "g", DeductG: 1000},
		{MaterialID: 12, MaterialName: "豆袋", Unit: "个", DeductUnits: 8},
	})
	_ = tx.Rollback(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("coverage rows = %+v", rows)
	}
	if rows[0].AvailableG != 800 || rows[0].CurrentConsumedG != 200 || rows[0].ShortageG != 0 {
		t.Fatalf("weight coverage = %+v", rows[0])
	}
	if rows[1].AvailableUnits != 6 || rows[1].ShortageUnits != 2 {
		t.Fatalf("count coverage = %+v", rows[1])
	}
}

func TestHistoricalWorkOrderWIPCoverageFallsBackToReservationRequirementsAndIgnoresClosedOrders(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.work_orders (
	id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL,running_item_id BIGINT NOT NULL DEFAULT 0,
	product_id BIGINT NOT NULL DEFAULT 0,product_name TEXT NOT NULL DEFAULT '',spec_g BIGINT NOT NULL DEFAULT 0,
	planned_g BIGINT NOT NULL DEFAULT 0,planned_output_g BIGINT NOT NULL DEFAULT 0,
	sales_spec_count NUMERIC(18,6) NOT NULL DEFAULT 0,order_nos TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'released',operation_template_id BIGINT NOT NULL DEFAULT 0,
	material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb
);
CREATE TABLE %s.material_batches (
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,
	remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,
	qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.work_order_material_reservations (
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	material_name TEXT NOT NULL DEFAULT '',unit TEXT NOT NULL DEFAULT 'g',
	required_g BIGINT NOT NULL DEFAULT 0,required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved'
);
INSERT INTO %s.work_orders(id,work_order_no,product_name,status) VALUES
	(88,'WO-HIST-0088','历史工单','released'),
	(89,'WO-OPEN-0089','开放工单','released'),
	(90,'WO-DONE-0090','已完成工单','completed'),
	(91,'WO-CANCEL-0091','已取消工单','cancelled');
INSERT INTO %s.material_batches VALUES
	(1,'WEIGHT',11,400,0,'active','pass'),
	(2,'COUNT',12,0,3,'active','pass');
INSERT INTO %s.material_batch_locations VALUES
	(1,11,'wip',400,0),(2,12,'wip',0,3);
INSERT INTO %s.work_order_material_reservations VALUES
	(1,88,11,'生豆','g',1000,0,1000,0,0,0,0,0,'reserved'),
	(2,88,12,'豆袋','个',0,8,0,8,0,0,0,0,'reserved'),
	(3,89,11,'生豆','g',100,0,100,0,0,0,0,0,'reserved'),
	(4,89,12,'豆袋','个',0,1,0,1,0,0,0,0,'reserved'),
	(5,90,11,'生豆','g',250,0,250,0,0,0,0,0,'reserved'),
	(6,90,12,'豆袋','个',0,2,0,2,0,0,0,0,'reserved'),
	(7,91,11,'生豆','g',150,0,150,0,0,0,0,0,'reserved'),
	(8,91,12,'豆袋','个',0,1,0,1,0,0,0,0,'reserved');
`, schema, schema, schema, schema, schema, schema, schema, schema))

	status, err := NewRepository(pool, schema).GetWorkOrderWIPCoverage(ctx, 88)
	if err != nil {
		t.Fatal(err)
	}
	if !status.DataComplete || status.Status != "blocked" || len(status.Materials) != 2 {
		t.Fatalf("historical coverage status = %+v", status)
	}
	byMaterial := make(map[int64]productionapp.WIPReservationRow, len(status.Materials))
	for _, row := range status.Materials {
		byMaterial[row.MaterialID] = row
	}
	weight := byMaterial[11]
	if weight.RequiredG != 1000 || weight.WIPG != 400 || weight.AvailableG != 300 || weight.ShortageG != 700 {
		t.Fatalf("historical weight coverage = %+v, want requirement 1000, physical 400, open-order available 300, shortage 700", weight)
	}
	count := byMaterial[12]
	if count.RequiredUnits != 8 || count.WIPUnits != 3 || count.AvailableUnits != 2 || count.ShortageUnits != 6 {
		t.Fatalf("historical count coverage = %+v, want requirement 8, physical 3, open-order available 2, shortage 6", count)
	}
}

func TestFinishedProductComponentConsumptionDeductsFinishedInventoryNotRawMaterialBatches(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.products (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	roast_level TEXT NOT NULL DEFAULT '',
	drip_box_bag_count BIGINT NOT NULL DEFAULT 10,
	active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %s.product_bom_sources (
	product_id BIGINT PRIMARY KEY,
	source_type TEXT NOT NULL DEFAULT '',
	source_product_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.product_production_configs (
	product_id BIGINT PRIMARY KEY,
	production_bom_id BIGINT NOT NULL DEFAULT 0,
	production_bom_version_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.product_production_bom_bindings (
	product_id BIGINT PRIMARY KEY,
	bom_id BIGINT NOT NULL DEFAULT 0,
	bom_version_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.production_boms (
	id BIGINT PRIMARY KEY,
	output_product_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.production_bom_versions (
	id BIGINT PRIMARY KEY,
	bom_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'draft',
	yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1,
	output_qty NUMERIC(14,6) NOT NULL DEFAULT 1,
	output_unit TEXT NOT NULL DEFAULT 'kg',
	published_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.production_bom_version_items (
	id BIGSERIAL PRIMARY KEY,
	version_id BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	ratio_pct NUMERIC(10,4) NOT NULL DEFAULT 0,
	material_loss_rate NUMERIC(10,4) NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
	qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0
);
CREATE TABLE %s.product_bom (
	product_id BIGINT PRIMARY KEY,
	yield_rate NUMERIC(10,4) NOT NULL DEFAULT 1
);
CREATE TABLE %s.product_bom_items (
	id BIGSERIAL PRIMARY KEY,
	product_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL DEFAULT 0,
	ratio_pct NUMERIC(10,4) NOT NULL DEFAULT 0,
	component_type TEXT NOT NULL DEFAULT 'material',
	component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,
	consume_unit TEXT NOT NULL DEFAULT 'ratio_pct',
	qty_per_unit NUMERIC(14,6) NOT NULL DEFAULT 0
);
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '',
	onhand_g BIGINT NOT NULL DEFAULT 0,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.packaging_spec_material_map (
	spec_g BIGINT PRIMARY KEY,
	material_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.finished_inventory (
	product_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL,
	warehouse TEXT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,
	onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(product_id,spec_g,warehouse)
);
CREATE TABLE %s.material_consumption_logs (
	id BIGSERIAL PRIMARY KEY,
	running_item_id BIGINT NOT NULL DEFAULT 0,
	batch_id TEXT NOT NULL DEFAULT '',
	product_id BIGINT NOT NULL DEFAULT 0,
	product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,
	material_id BIGINT NOT NULL DEFAULT 0,
	material_name TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL DEFAULT '',
	deduct_g BIGINT NOT NULL DEFAULT 0,
	deduct_units BIGINT NOT NULL DEFAULT 0,
	before_g BIGINT NOT NULL DEFAULT 0,
	after_g BIGINT NOT NULL DEFAULT 0,
	before_units BIGINT NOT NULL DEFAULT 0,
	after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',
	material_batch_id BIGINT NOT NULL DEFAULT 0,
	material_batch_code TEXT NOT NULL DEFAULT ''
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
INSERT INTO %s.products(id,name,roast_level) VALUES
	(1,'蓝山挂耳','深烘'),
	(2,'蓝山熟豆','深烘');
INSERT INTO %s.product_bom(product_id,yield_rate) VALUES (1,1.0000);
INSERT INTO %s.product_bom_items(
	product_id,material_id,ratio_pct,component_type,component_product_id,component_spec_g,consume_unit,qty_per_unit
) VALUES (1,0,0,'finished_product',2,0,'g_per_bag',10);
INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
VALUES (2,0,'finished_goods',0,200);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	run := ProduceRunRow{
		ID:        9,
		BatchID:   "BATCH-DRIP",
		ProductID: 1,
		Product:   "蓝山挂耳",
		SpecG:     10,
		NeedG:     150,
		InputG:    150,
		PlanUnits: 15,
	}
	needs, err := currentMaterialNeedsTx(ctx, tx, schema, run, InvQty{Units: 15})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("currentMaterialNeedsTx: %v", err)
	}
	if len(needs) != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("needs = %+v, want one finished product component", needs)
	}
	if needs[0].Source != "finished_product" || needs[0].MaterialID != 2 || needs[0].DeductG != 150 || needs[0].DeductUnits != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("finished product component need = %+v, want product 2 150g", needs[0])
	}
	if err := deductMaterialNeedsForRunningItemTx(ctx, tx, schema, run, needs, "测试员"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("deductMaterialNeedsForRunningItemTx: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var remainingG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_loose_g FROM %s.finished_inventory WHERE product_id=2 AND spec_g=0 AND warehouse='finished_goods'`, schema)).Scan(&remainingG); err != nil {
		t.Fatal(err)
	}
	if remainingG != 50 {
		t.Fatalf("upstream finished inventory remaining = %d, want 50", remainingG)
	}
	var ledgerType string
	var changeG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT item_type, qty_change_g FROM %s.stock_ledger_entries WHERE source_doc_id=9`, schema)).Scan(&ledgerType, &changeG); err != nil {
		t.Fatal(err)
	}
	if ledgerType != "finished_product" || changeG != -150 {
		t.Fatalf("ledger = %s/%d, want finished_product/-150", ledgerType, changeG)
	}
	var meta string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(meta::text,'') FROM %s.audit_logs WHERE entity_type='produce_running' AND action='consume_finished_product_component'`, schema)).Scan(&meta); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"finished_product_component_consumption", "drip_demand", "upstream_roast_demand_g"} {
		if !strings.Contains(meta, want) {
			t.Fatalf("audit meta %s missing %q", meta, want)
		}
	}
}

func newProductionTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for production tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_production_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	return pool, schema
}

func mustExecProductionSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("exec sql: %v", err)
	}
}
