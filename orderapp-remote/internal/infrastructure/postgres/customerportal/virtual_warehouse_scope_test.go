package customerportal

import (
	"context"
	"fmt"
	"testing"
	"time"

	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresstock "orderapp/internal/infrastructure/postgres/stock"
)

func TestProcessingAvailabilityExcludesUnboundVirtualWarehouseSources(t *testing.T) {
	pool, schema := newCustomerPortalTestDB(t)
	ctx := context.Background()
	if err := postgresstock.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(77,'PR710客户A'),(78,'PR710客户B');INSERT INTO %[1]s.materials(id,code,name,unit,cost_unit) VALUES(50,'PR710-BEAN','PR710原料','kg','kg');INSERT INTO %[1]s.products(id,name) VALUES(50,'PR710半成品')`, schema)); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct {
		code       string
		customerID int64
		virtual    bool
	}{{"PR710-FACTORY", 0, false}, {"PR710-UNBOUND", 0, true}, {"PR710-OWN", 77, true}, {"PR710-OTHER", 78, true}} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.warehouses(code,name,kind,customer_id,is_virtual) VALUES($1,$1,'raw',$2,$3)`, schema), w.code, w.customerID, w.virtual); err != nil {
			t.Fatal(err)
		}
		var batchID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.material_batches(batch_code,material_id,qty_g,remaining_g,status,quality_status) VALUES($1,50,1000,1000,'active','unchecked') RETURNING id`, schema), w.code).Scan(&batchID); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES($1,$2,50,$2,1000)`, schema), batchID, w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_batches(batch_code,item_type,item_id,item_name,spec_g,source_doc_type,qty_units,qty_g,remaining_units,remaining_g,quality_status) VALUES($1,'finished_product',50,'PR710',1000,$1,1,1000,1,1000,'passed')`, schema), w.code); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_doc_type,source_batch_code,qty_change_g,qty_after_g,qty_change_units,qty_after_units) VALUES('finished_product',50,'PR710',1000,$1,'manual',$1,1000,1000,1,1)`, schema), w.code); err != nil {
			t.Fatal(err)
		}
	}
	r := NewRepository(pool, schema)
	for _, componentType := range []string{"material", "finished_product"} {
		for _, lock := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lock=%v", componentType, lock), func(t *testing.T) {
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback(ctx)
				specG := int64(0)
				if componentType == "finished_product" {
					specG = 1000
				}
				sources, preview, err := r.processingAvailabilityTx(ctx, tx, 77, 50, componentType, specG, lock)
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]bool{}
				for _, source := range sources {
					got[source.WarehouseCode] = true
				}
				if len(sources) != 2 || !got["PR710-FACTORY"] || !got["PR710-OWN"] || preview.AvailableG != 2000 {
					t.Fatalf("warehouse isolation violated: sources=%+v preview=%+v", sources, preview)
				}
			})
		}
	}
	t.Run("material submit does not lock unbound stock", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := r.lockProcessingStockRowsTx(ctx, tx, 77, 50, "material", 0); err != nil {
			t.Fatal(err)
		}
		writeCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		if _, err := pool.Exec(writeCtx, fmt.Sprintf(`UPDATE %s.material_batch_locations SET qty_g=qty_g+1 WHERE warehouse='PR710-UNBOUND'`, schema)); err != nil {
			t.Fatalf("customer processing locked unbound virtual warehouse stock: %v", err)
		}
	})
}
