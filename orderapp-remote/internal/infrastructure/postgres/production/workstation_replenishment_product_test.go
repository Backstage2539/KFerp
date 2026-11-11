package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestReplenishmentProductSharedBatchAllocatedOnce(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	ctx := context.Background()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.stock_batches(id bigint primary key,quality_status text,remaining_g bigint,remaining_units bigint,owner_customer_id bigint); CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigint primary key,reservation_id bigint,stock_batch_id bigint,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint,status text,warehouse text,owner_customer_id bigint); INSERT INTO %[1]s.stock_batches VALUES (1,'passed',10000,0,0),(2,'hold',50000,0,0); INSERT INTO %[1]s.work_order_material_reservation_batches VALUES (1,11,1,10000,0,0,0,0,0,'reserved','wip',0),(2,12,1,10000,0,0,0,0,0,'reserved','wip',0),(3,13,2,50000,0,0,0,0,0,'reserved','wip',0)`, schema))
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	batchUsed := map[int64]int64{}
	freeUsed := map[int64]int64{}
	ownUsed := map[int64]float64{}
	for _, tc := range []struct {
		id   int64
		want float64
	}{{11, 10}, {12, 0}, {13, 0}} {
		got, err := repo.replenishmentProductCoverageTx(ctx, tx, app.OperationMaterialUse{ReservationID: tc.id, ProductID: 2, Unit: "kg", Qty: 10}, batchUsed, freeUsed, ownUsed)
		if err != nil || got != tc.want {
			t.Fatalf("reservation %d: got %v want %v err %v", tc.id, got, tc.want, err)
		}
	}
}

func TestReplenishmentProductSharedUnreservedWIPAllocatedOnce(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	ctx := context.Background()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.stock_batches(id bigint primary key,quality_status text,remaining_g bigint,remaining_units bigint,owner_customer_id bigint); CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigint primary key,reservation_id bigint,stock_batch_id bigint,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint,status text,warehouse text,owner_customer_id bigint); INSERT INTO %[1]s.stock_batches VALUES (1,'passed',10000,0,0)`, schema))
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	batchUsed := map[int64]int64{}
	freeUsed := map[int64]int64{}
	ownUsed := map[int64]float64{}
	for _, tc := range []struct {
		id   int64
		want float64
	}{{14, 10}, {15, 0}} {
		m := app.OperationMaterialUse{ReservationID: tc.id, ProductID: 2, Unit: "kg", Qty: 10, AvailableQty: 10, EligibleBatches: []app.OperationMaterialBatch{{BatchID: 1, BatchCode: "WIP", AvailableQty: 10}}}
		got, err := repo.replenishmentProductCoverageTx(ctx, tx, m, batchUsed, freeUsed, ownUsed)
		if err != nil || got != tc.want {
			t.Fatalf("reservation %d: got %v want %v err %v", tc.id, got, tc.want, err)
		}
	}
}

func TestReplenishmentProductReservedThenFreeDoNotSubtractTwice(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	ctx := context.Background()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %[1]s.stock_batches(id bigint primary key,quality_status text,remaining_g bigint,remaining_units bigint,owner_customer_id bigint); CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigint primary key,reservation_id bigint,stock_batch_id bigint,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint,status text,warehouse text,owner_customer_id bigint); INSERT INTO %[1]s.stock_batches VALUES (1,'passed',10000,0,0); INSERT INTO %[1]s.work_order_material_reservation_batches VALUES (1,11,1,4000,0,0,0,0,0,'reserved','wip',0)`, schema))
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	batchUsed := map[int64]int64{}
	freeUsed := map[int64]int64{}
	ownUsed := map[int64]float64{}
	first, err := repo.replenishmentProductCoverageTx(ctx, tx, app.OperationMaterialUse{ReservationID: 11, ProductID: 2, Unit: "kg", Qty: 4}, batchUsed, freeUsed, ownUsed)
	if err != nil || first != 4 {
		t.Fatalf("reserved %v %v", first, err)
	}
	second, err := repo.replenishmentProductCoverageTx(ctx, tx, app.OperationMaterialUse{ReservationID: 12, ProductID: 2, Unit: "kg", Qty: 6, AvailableQty: 6, EligibleBatches: []app.OperationMaterialBatch{{BatchID: 1, AvailableQty: 6}}}, batchUsed, freeUsed, ownUsed)
	if err != nil || second != 6 {
		t.Fatalf("free %v %v", second, err)
	}
}

func TestReplenishmentUnreservedProductCandidatesFromDatabase(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	ctx := context.Background()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.stock_batches(id bigint primary key,item_type text,item_id bigint,batch_code text,remaining_g bigint,remaining_units bigint,owner_customer_id bigint,bom_spec_id bigint,spec_g bigint,bom_variant_id bigint,quality_status text,created_at timestamp default now());
 CREATE TABLE %[1]s.stock_ledger_entries(id bigint,item_type text,item_id bigint,source_batch_code text,owner_customer_id bigint,warehouse text);
 CREATE TABLE %[1]s.customer_stock_entitlements(id bigint,status text); CREATE TABLE %[1]s.customer_stock_entitlement_receipts(entitlement_id bigint,stock_batch_id bigint);
 CREATE TABLE %[1]s.work_order_material_reservations(id bigint,status text); CREATE TABLE %[1]s.work_order_material_reservation_batches(id bigint primary key,reservation_id bigint,work_order_id bigint,stock_batch_id bigint,reserved_g bigint,consumed_g bigint,returned_g bigint,reserved_units bigint,consumed_units bigint,returned_units bigint,status text,warehouse text,owner_customer_id bigint);
 INSERT INTO %[1]s.stock_batches VALUES (1,'finished_product',2,'FREE',10000,0,0,3,1,4,'passed',now()),(2,'finished_product',2,'HOLD',50000,0,0,3,1,4,'hold',now()),(3,'finished_product',2,'OTHER-OWNER',50000,0,9,3,1,4,'passed',now());
 INSERT INTO %[1]s.stock_ledger_entries VALUES (1,'finished_product',2,'FREE',0,'wip'),(2,'finished_product',2,'HOLD',0,'wip'),(3,'finished_product',2,'OTHER-OWNER',9,'wip');`, schema))
	repo := NewRepository(pool, schema)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	batchUsed := map[int64]int64{}
	freeUsed := map[int64]int64{}
	ownUsed := map[int64]float64{}
	for _, tc := range []struct {
		reservation, work int64
		want              float64
	}{{14, 1, 10}, {15, 2, 0}} {
		m := app.OperationMaterialUse{ReservationID: tc.reservation, ProductID: 2, Unit: "kg", Qty: 10, Warehouse: "wip", BomSpecID: 3, SpecG: 1, BomVariantID: 4}
		m.EligibleBatches, err = startedMaterialBatchesTx(ctx, tx, schema, tc.work, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.EligibleBatches) != 1 || m.EligibleBatches[0].BatchCode != "FREE" {
			t.Fatalf("wrong identities: %+v", m.EligibleBatches)
		}
		m.AvailableQty = m.EligibleBatches[0].AvailableQty
		got, e := repo.replenishmentProductCoverageTx(ctx, tx, m, batchUsed, freeUsed, ownUsed)
		if e != nil || got != tc.want {
			t.Fatalf("reservation %d: got %v want %v err %v", tc.reservation, got, tc.want, e)
		}
	}
}
