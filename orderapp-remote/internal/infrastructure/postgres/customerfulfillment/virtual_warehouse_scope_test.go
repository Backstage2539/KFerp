package customerfulfillment

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMiniUnboundVirtualWarehouseIsNotPublicCustomerStock(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	for _, w := range []struct {
		code       string
		customerID int64
		virtual    bool
	}{{"PR710-PUBLIC", 0, false}, {"PR710-UNBOUND", 0, true}, {"PR710-OWN", 501, true}, {"PR710-OTHER", 502, true}} {
		seedCustomerID := w.customerID
		if w.virtual {
			seedCustomerID = 501
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,customer_id,is_virtual) VALUES($1,$1,'finished',$2,$3)`, schema), w.code, seedCustomerID, w.virtual); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,owner_customer_id,onhand_units) VALUES(911,1000,$1,501,10)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,owner_customer_id,qty_units,qty_g,remaining_units,remaining_g,quality_status) VALUES($1,'finished_product',911,'PR710',1000,$1,501,10,10000,10,10000,'passed')`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units) VALUES('finished_product',911,'PR710',1000,$1,'manual',$1,10000,10000,10,10)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		var orderID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.orders(order_no,customer_id,source_warehouse) VALUES($1,501,$1) RETURNING id`, schema), w.code).Scan(&orderID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_stock_batch_allocations(order_id,product_id,spec_g,warehouse,allocated_g,allocated_units) VALUES($1,911,1000,$2,1000,1)`, schema), orderID, w.code); err != nil {
			t.Fatal(err)
		}
		// Model historical inventory left by an old unbind/rebind path. New
		// business writes are guarded, but read isolation must reject old rows too.
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.warehouses SET customer_id=$2 WHERE code=$1`, schema), w.code, w.customerID); err != nil {
			t.Fatal(err)
		}
	}
	r := NewRepository(pool, schema)
	stock, err := r.loadMiniCustomerFinishedStock(ctx, pool, 501, false)
	if err != nil {
		t.Fatal(err)
	}
	inventory, batches := map[string]bool{}, map[string]bool{}
	for _, row := range stock.Inventory {
		inventory[row.Warehouse] = true
	}
	for _, row := range stock.Batches {
		batches[row.Warehouse] = true
	}
	reservations, _, _, err := r.loadMiniStockReservations(ctx, pool, 501)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"PR710-PUBLIC", "PR710-OWN"} {
		if !inventory[code] || !batches[code] || reservations[miniWarehouseStockKey(911, 0, 1000, code)] != 1 {
			t.Errorf("eligible customer warehouse %s missing: inventory=%v batches=%v reservations=%v", code, inventory, batches, reservations)
		}
	}
	for _, code := range []string{"PR710-UNBOUND", "PR710-OTHER"} {
		if inventory[code] || batches[code] || reservations[miniWarehouseStockKey(911, 0, 1000, code)] != 0 {
			t.Errorf("ineligible warehouse %s exposed as customer stock: inventory=%v batches=%v reservations=%v", code, inventory, batches, reservations)
		}
	}
}

func TestFulfillmentWarehouseValidationLocksBindingUntilCommit(t *testing.T) {
	pool, schema := newMiniDirectShipTestDB(t)
	ctx := context.Background()
	seedMiniDirectShipStock(t, pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := NewRepository(pool, schema).validateFulfillmentWarehouse(ctx, tx, 501, "DEV-E2E-A1"); err != nil {
		t.Fatal(err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	if _, err := pool.Exec(writeCtx, fmt.Sprintf(`UPDATE %s.warehouses SET name='concurrent warehouse edit' WHERE code='DEV-E2E-A1'`, schema)); err == nil {
		t.Fatal("warehouse changed while fulfillment binding validation was pending commit")
	}
	if writeCtx.Err() == nil {
		t.Fatal("concurrent edit failed without waiting for the warehouse binding lock")
	}
}
