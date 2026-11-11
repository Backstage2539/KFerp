package stock

import (
	"context"
	"fmt"
	stockapp "orderapp/internal/application/stock"
	"testing"
)

func TestBatchListsExposeUnknownCostState(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 1000, UnitCost: 10, Operator: "test"})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE id=%[2]d;UPDATE %[1]s.stock_batches SET cost_unavailable=true WHERE batch_code='%[3]s'`, schema, receipt.BatchID, receipt.BatchCode))
	a, err := repo.ListMaterialBatches(ctx, stockapp.MaterialBatchQuery{Limit: 10})
	if err != nil || len(a.Rows) != 1 || !a.Rows[0].CostUnavailable {
		t.Fatalf("material %+v err=%v", a.Rows, err)
	}
	b, err := repo.ListBatches(ctx, stockapp.BatchQuery{Limit: 10})
	if err != nil || len(b.Rows) != 1 || !b.Rows[0].CostUnavailable {
		t.Fatalf("stock %+v err=%v", b.Rows, err)
	}
}

func TestWarehouseInventoryExposesUnknownCost(t *testing.T) {
	pool, schema := setupUnifiedStockDocumentTest(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	receipt, err := repo.ReceiveMaterial(ctx, stockapp.MaterialReceiptCommand{MaterialID: 1, QtyG: 1000, UnitCost: 10, Operator: "test"})
	if err != nil {
		t.Fatal(err)
	}
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.material_batches SET cost_unavailable=true WHERE id=%[2]d;
 CREATE TABLE IF NOT EXISTS %[1]s.production_bom_specs(id bigint primary key,name text,inventory_unit text);
 CREATE TABLE IF NOT EXISTS %[1]s.production_bom_version_variants(id bigint primary key,bom_spec_id bigint,spec_name_snapshot text,inventory_unit text);
 CREATE TABLE IF NOT EXISTS %[1]s.business_groups(id bigint primary key,name text);
 CREATE TABLE IF NOT EXISTS %[1]s.business_group_items(id bigint primary key,name text);
 CREATE TABLE IF NOT EXISTS %[1]s.business_group_assignments(id bigint,group_id bigint,group_item_id bigint,object_ref text,object_id bigint,usage_key text,object_key text);`, schema, receipt.BatchID))
	result, err := repo.ListWarehouseInventory(ctx, stockapp.WarehouseInventoryQuery{ItemType: "material", Limit: 10})
	if err != nil || len(result.Rows) != 1 || !result.Rows[0].CostUnavailable {
		t.Fatalf("warehouse=%+v err=%v", result.Rows, err)
	}
}
