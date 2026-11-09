package stock

import (
	"context"
	"fmt"
	stockapp "orderapp/internal/application/stock"
	"testing"
)

func TestProductionPickingProductBatchPreservesUnknownCost(t *testing.T) {
	pool, schema, _, _ := setupPickingSnapshotTest(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES(9,1000,'finished_goods',1,0);
 INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units,cost_unavailable,quality_status) VALUES('PICK-UNKNOWN','finished_product',9,'测试熟豆',1000,1000,1,1000,1,true,'pass');`, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	repo := NewRepository(pool, schema)
	detail := stockapp.StockDocumentDetail{StockDocumentRow: stockapp.StockDocumentRow{ID: 9, WorkOrderID: 88, EntryNo: "TEST"}}
	item := stockapp.StockDocumentItemRow{ID: 9, ProductID: 9, ItemName: "测试熟豆", SpecG: 1000, QtyUnits: 1, FromWarehouse: "finished_goods", ToWarehouse: "wip", BatchCode: "PICK-UNKNOWN"}
	if err = repo.postPickingProductTransferTx(ctx, tx, detail, item, "test", 1); err != nil {
		t.Fatal(err)
	}
	var unavailable bool
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT b.cost_unavailable FROM %[1]s.stock_batches b JOIN %[1]s.stock_entry_finished_batch_moves m ON m.target_batch_id=b.id WHERE m.stock_entry_item_id=9`, schema)).Scan(&unavailable); err != nil || !unavailable {
		t.Fatalf("picked batch unavailable=%v err=%v", unavailable, err)
	}
}
