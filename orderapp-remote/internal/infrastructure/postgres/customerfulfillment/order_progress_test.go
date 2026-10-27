package customerfulfillment

import (
	"context"
	"fmt"
	salesapp "orderapp/internal/application/sales"
	cs "orderapp/internal/infrastructure/postgres/customerstock"
	progress "orderapp/internal/infrastructure/postgres/orderprogress"
	sales "orderapp/internal/infrastructure/postgres/sales"
	"testing"
)

func TestOrderProgressUsesOwnPhysicalStockAndStableSpec(t *testing.T) {
	f := newFulfillmentEditFixture(t)
	ctx := context.Background()
	id := f.request.OrderID
	check := func(want bool) {
		t.Helper()
		v, e := progress.Load(ctx, f.pool, f.schema, []int64{id})
		if e != nil || v[id] == nil || v[id].CanShip != want {
			t.Fatalf("progress want %v got %+v error %v", want, v[id], e)
		}
	}
	check(false)
	tx, e := f.pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if e = cs.LockFulfillmentCustomersTx(ctx, tx, f.schema, "EDIT-SOURCE"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,bom_spec_id,bom_variant_id,owner_customer_id,qty_units,remaining_units,quality_status)VALUES('PROGRESS-RECEIPT','finished_product',910,'实际入库',0,7031,7032,501,4,4,'passed');
 UPDATE %[1]s.finished_inventory SET onhand_units=onhand_units+4 WHERE product_id=910 AND bom_spec_id=7031 AND warehouse='DEV-E2E-A1';
 INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,owner_customer_id,spec_g,bom_spec_id,bom_variant_id,warehouse,source_doc_type,source_doc_id,source_batch_code,qty_change_units,qty_after_units)VALUES('finished_product',910,'实际入库',501,0,7031,7032,'DEV-E2E-A1','production_run',703,'PROGRESS-RECEIPT',4,4)`, f.schema)); e != nil {
		t.Fatal(e)
	}
	if e = cs.ReceiveProductionTx(ctx, tx, f.schema, 703, 910, 7031, 7032, 0, 4, "PROGRESS-RECEIPT", "PROGRESS-RECEIPT", "DEV-E2E-A1", "EDIT-SOURCE", "test"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	check(true)
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET quality_status='unchecked' WHERE batch_code='PROGRESS-RECEIPT'`, f.schema)); e != nil {
		t.Fatal(e)
	}
	check(true) // Existing shipping policy permits unchecked production receipts.
	// Frozen manufacturing version can differ from the current order's version.
	if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET bom_variant_id=999999 WHERE batch_code='PROGRESS-RECEIPT'`, f.schema)); e != nil {
		t.Fatal(e)
	}
	check(true)
	repo := sales.NewRepository(f.pool, f.schema)
	rows, e := repo.ListOrders(ctx, salesapp.OrderListQuery{OrderID: id, Limit: 10})
	if e != nil || len(rows.Rows) != 1 || rows.Rows[0].FulfillmentProgress == nil || rows.Rows[0].FulfillmentProgress.Label != "库存充足 · 可以发货" {
		t.Fatalf("list %+v %v", rows, e)
	}
	for _, change := range []string{"quality_status='frozen'", "remaining_units=3", "owner_customer_id=502"} {
		if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET %s WHERE batch_code='PROGRESS-RECEIPT'`, f.schema, change)); e != nil {
			t.Fatal(e)
		}
		check(false)
		if _, e = f.pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.stock_batches SET quality_status='passed',remaining_units=4,owner_customer_id=501 WHERE batch_code='PROGRESS-RECEIPT'`, f.schema)); e != nil {
			t.Fatal(e)
		}
		check(true)
	}
}
