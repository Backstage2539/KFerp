package stock

import (
	"context"
	"fmt"
	app "orderapp/internal/application/stock"
	"strings"
	"testing"
)

func TestOperationProductConsumptionHonoursSelectedBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.work_orders(id bigint primary key,material_consumption_policy text);
INSERT INTO %[1]s.work_orders VALUES(10,'bom_start_v1');
CREATE TABLE %[1]s.finished_inventory(product_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,warehouse text,owner_customer_id bigint,onhand_units bigint,onhand_loose_g bigint,updated_at timestamptz default now(),unique(product_id,bom_spec_id,spec_g,warehouse));
INSERT INTO %[1]s.finished_inventory VALUES(7,0,3,1000,'wip',0,2,0,now());
CREATE TABLE %[1]s.work_order_material_reservations(id bigint primary key,work_order_id bigint,component_type text,component_id bigint,component_bom_spec_id bigint,component_bom_variant_id bigint,component_spec_g bigint,source_owner_customer_id bigint,status text,reserved_g bigint default 0,reserved_units bigint default 0,consumed_g bigint default 0,consumed_units bigint default 0,updated_at timestamptz default now());
INSERT INTO %[1]s.work_order_material_reservations(id,work_order_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,source_owner_customer_id,status) VALUES(1,10,'product',7,0,3,1000,0,'reserved');
CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigserial primary key,material_id bigint default 0,reservation_id bigint,work_order_id bigint,component_type text,component_id bigint,component_bom_spec_id bigint,component_bom_variant_id bigint,component_spec_g bigint,stock_batch_id bigint,material_batch_id bigint default 0,batch_code text,warehouse text,owner_customer_id bigint,reserved_g bigint default 0,reserved_units bigint default 0,consumed_g bigint default 0,consumed_units bigint default 0,returned_g bigint default 0,returned_units bigint default 0,status text,updated_at timestamptz default now());
CREATE TABLE %[1]s.stock_batches(id bigint primary key,batch_code text,item_type text,item_id bigint,bom_spec_id bigint,bom_variant_id bigint,spec_g bigint,owner_customer_id bigint,remaining_g bigint,remaining_units bigint,unit_cost numeric,quality_status text,created_at timestamptz default now());
INSERT INTO %[1]s.stock_batches VALUES(1,'OLDER','finished_product',7,0,3,1000,0,1000,1,80,'pass',now()-interval '1 day'),(2,'CHOSEN','finished_product',7,0,3,1000,0,1000,1,90,'pass',now());
CREATE TABLE %[1]s.stock_ledger_entries(id bigserial primary key,item_type text,item_id bigint,item_name text,owner_customer_id bigint,spec_g bigint,bom_spec_id bigint,bom_variant_id bigint,warehouse text,source_doc_type text,source_doc_id bigint,source_batch_code text,source_batch_id text,qty_before_g bigint,qty_change_g bigint,qty_after_g bigint,qty_before_units bigint,qty_change_units bigint,qty_after_units bigint,operator text,created_at timestamptz default now());
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,owner_customer_id,warehouse,source_batch_code) VALUES('finished_product',7,0,'wip','OLDER'),('finished_product',7,0,'wip','CHOSEN');
CREATE TABLE %[1]s.customer_stock_entitlements(id bigint primary key,status text);
CREATE TABLE %[1]s.customer_stock_entitlement_receipts(entitlement_id bigint,stock_batch_id bigint);
CREATE TABLE %[1]s.stock_entry_items(id bigint primary key,unit_cost numeric,total_cost numeric);
INSERT INTO %[1]s.stock_entry_items(id) VALUES(1);
CREATE TABLE %[1]s.stock_entry_batch_allocations(stock_entry_item_id bigint,material_batch_id bigint,stock_batch_id bigint,batch_code text,qty_g bigint,qty_units bigint,unit_cost numeric);`, schema))
	repo := NewRepository(pool, schema)
	cases := []struct {
		name, selected, want string
		qty                  int64
		wantErr              bool
		setup                string
	}{
		{"explicit selected", "CHOSEN", "CHOSEN", 1000, false, ""}, {"legacy FIFO", "", "OLDER", 1000, false, ""},
		{"missing selection", "MISSING", "", 1000, true, ""}, {"selected insufficient", "CHOSEN", "", 1500, true, ""},
		{"quality hold", "CHOSEN", "", 1000, true, `UPDATE %[1]s.stock_batches SET quality_status='hold' WHERE id=2`},
		{"wrong owner", "CHOSEN", "", 1000, true, `UPDATE %[1]s.stock_batches SET owner_customer_id=99 WHERE id=2`},
		{"wrong variant", "CHOSEN", "", 1000, true, `UPDATE %[1]s.stock_batches SET bom_variant_id=99 WHERE id=2`},
		{"reserved by another work order", "CHOSEN", "", 1000, true, `INSERT INTO %[1]s.work_order_material_reservations(id,status) VALUES(2,'reserved'); INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,stock_batch_id,warehouse,owner_customer_id,status,reserved_g,reserved_units) VALUES(2,11,2,'wip',0,'reserved',1000,1)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.setup != "" {
				if _, err = tx.Exec(ctx, fmt.Sprintf(tc.setup, schema)); err != nil {
					t.Fatal(err)
				}
			}
			item := app.StockDocumentItemRow{ID: 1, ProductID: 7, BomVariantID: 3, SpecG: 1000, FromWarehouse: "wip", QtyG: tc.qty, BatchCode: tc.selected, ItemName: "test"}
			detail := app.StockDocumentDetail{StockDocumentRow: app.StockDocumentRow{ID: 1, WorkOrderID: 10, EntryNo: "TEST"}}
			err = repo.consumeOperationProductTx(ctx, tx, detail, &item, "tester")
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "商品组件合格库存不足") {
					t.Fatalf("selected batch %q should reject rather than consume another batch: %v", tc.selected, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var code string
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT batch_code FROM %s.stock_entry_batch_allocations`, schema)).Scan(&code); err != nil {
				t.Fatal(err)
			}
			if code != tc.want {
				t.Fatalf("consumed %q, want %q", code, tc.want)
			}
			var cost float64
			if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT total_cost::float8 FROM %s.stock_entry_items WHERE id=1`, schema)).Scan(&cost); err != nil {
				t.Fatal(err)
			}
			wantCost := 80.0
			if tc.want == "CHOSEN" {
				wantCost = 90.0
			}
			if cost != wantCost {
				t.Fatalf("selected batch cost = %v, want %v", cost, wantCost)
			}
		})
	}
}

func TestOperationMaterialAvailabilityHonoursSelectedBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() })
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.material_batches(id bigint primary key,batch_code text,owner_customer_id bigint,unit_cost numeric,quality_status text,status text,received_at timestamptz);
CREATE TABLE %[1]s.material_batch_locations(material_batch_id bigint,material_id bigint,warehouse text,qty_g bigint,qty_units bigint);
CREATE TABLE %[1]s.work_order_material_reservations(id bigint primary key,status text);
CREATE TABLE %[1]s.work_order_material_reservation_batches(reservation_id bigint,work_order_id bigint,material_batch_id bigint,warehouse text,owner_customer_id bigint,status text,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint);
INSERT INTO %[1]s.material_batches VALUES(1,'OLDER',0,80,'pass','active',now()-interval '1 day'),(2,'CHOSEN',0,90,'pass','active',now());
INSERT INTO %[1]s.material_batch_locations VALUES(1,7,'wip',1000,0),(2,7,'wip',1000,0);`, schema))
	cases := []struct {
		name, selected, setup, want string
		wantG                       int64
	}{
		{"explicit selection", "CHOSEN", "", "CHOSEN", 1000},
		{"legacy FIFO", "", "", "OLDER", 1000},
		{"missing selection", "MISSING", "", "", 0},
		{"wrong owner", "CHOSEN", `UPDATE %[1]s.material_batches SET owner_customer_id=99 WHERE id=2`, "", 0},
		{"quality hold", "CHOSEN", `UPDATE %[1]s.material_batches SET quality_status='hold' WHERE id=2`, "", 0},
		{"other work order reserved", "CHOSEN", `INSERT INTO %[1]s.work_order_material_reservations VALUES(2,'reserved'); INSERT INTO %[1]s.work_order_material_reservation_batches VALUES(2,11,2,'wip',0,'reserved',1000,0,0,0,0,0)`, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.setup != "" {
				if _, err = tx.Exec(ctx, fmt.Sprintf(tc.setup, schema)); err != nil {
					t.Fatal(err)
				}
			}
			opCtx := context.WithValue(ctx, operationConsumptionContextKey{}, int64(10))
			rows, _, _, err := NewRepository(pool, schema).materialMoveAvailabilityTx(opCtx, tx, app.StockDocumentDetail{StockDocumentRow: app.StockDocumentRow{WorkOrderID: 10, Purpose: app.PurposeMaterialConsumption}}, app.StockDocumentItemRow{MaterialID: 7, FromWarehouse: "wip", BatchCode: tc.selected})
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(rows) != 0 {
					t.Fatalf("selected material must not fall back to FIFO: %+v", rows)
				}
				return
			}
			if len(rows) == 0 || rows[0].BatchCode != tc.want || rows[0].AvailableG != tc.wantG {
				t.Fatalf("unexpected selection or availability: %+v", rows)
			}
		})
	}
}
