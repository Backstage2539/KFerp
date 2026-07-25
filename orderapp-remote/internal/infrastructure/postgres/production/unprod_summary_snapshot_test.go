package production

import (
	"context"
	"fmt"
	"testing"
)

func TestProductionInventoryConversionFactorSupportsNestedFlatAndUnitAliases(t *testing.T) {
	tests := []struct {
		name          string
		source        map[string]any
		effective     map[string]any
		salesUnit     string
		inventoryUnit string
		want          float64
	}{
		{
			name: "nested aliases and case",
			source: map[string]any{
				"inventory_conversion_json": map[string]any{
					"LB": map[string]any{"KG": 0.45359237},
				},
			},
			salesUnit:     "磅",
			inventoryUnit: "kg",
			want:          0.45359237,
		},
		{
			name: "flat inventory unit",
			effective: map[string]any{
				"inventory_conversion_json": map[string]any{
					"公斤": 0.454,
				},
			},
			salesUnit:     "454g",
			inventoryUnit: "KG",
			want:          0.454,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := productionInventoryConversionFactor(tt.source, tt.effective, tt.salesUnit, tt.inventoryUnit); !sameProductionQuantity(got, tt.want) {
				t.Fatalf("conversion factor = %.9f, want %.9f", got, tt.want)
			}
		})
	}
}

func TestFinalizeUnproducedNeedsAllocatesSharedFinishedInventoryOnceDeterministically(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.finished_inventory (
			product_id BIGINT NOT NULL,
			spec_g BIGINT NOT NULL,
			warehouse TEXT NOT NULL DEFAULT 'finished_goods',
			onhand_units BIGINT NOT NULL DEFAULT 0,
			onhand_loose_g BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY(product_id,spec_g,warehouse)
		);
		CREATE TABLE %[1]s.order_stock_batch_allocations (
			order_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			spec_g BIGINT NOT NULL,
			batch_code TEXT NOT NULL DEFAULT '',
			allocated_g BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.order_stock_deductions (
			order_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			spec_g BIGINT NOT NULL,
			batch_code TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g)
		VALUES (789,454,'finished_goods',3,0);
	`, schema))

	makeDemand := func(orderNo string, parentID int64) productionDemand {
		return productionDemand{
			UnprodNeedRow: UnprodNeedRow{
				ProductID: 789, ParentProductID: parentID, Product: "如目达摩",
				OrderNos:  orderNo,
				SpecLabel: "454g", SalesUnit: "454g", SpecG: 454,
				InventoryQtyPerSalesUnit: 0.454, InventoryUnit: "kg",
				SalesSpecSnapshotJSON: fmt.Sprintf(
					`{"sku_id":789,"parent_product_id":%d,"spec_label":"454g","sales_unit":"454g","inventory_unit":"kg","inventory_qty_per_sales_unit":0.454}`,
					parentID,
				),
			},
			orderNos: map[string]bool{orderNo: true},
		}
	}
	later := makeDemand("SO-B", 645)
	later.SalesSpecCount = 2
	earlier := makeDemand("SO-A", 644)
	earlier.SalesSpecCount = 2

	rows, err := finalizeUnproducedNeeds(ctx, pool, schema, []productionDemand{later, earlier})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2: %+v", len(rows), rows)
	}
	byOrder := map[string]UnprodNeedRow{}
	for _, row := range rows {
		byOrder[row.OrderNos] = row
	}
	if got := byOrder["SO-A"]; got.GapG != 0 || !sameProductionQuantity(got.GapInventoryQty, 0) {
		t.Fatalf("first deterministic group should consume two of three units: %+v", got)
	}
	if got := byOrder["SO-B"]; got.GapG != 454 || !sameProductionQuantity(got.GapInventoryQty, 0.454) {
		t.Fatalf("second group must see only one remaining unit: %+v", got)
	}
}
