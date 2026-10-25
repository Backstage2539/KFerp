package stock

import (
	"context"
	"fmt"
	"testing"

	app "orderapp/internal/application/stock"
)

func TestRepairMaterialBatchLocationsConservesBalanceMovesReservationAndIsIdempotent(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
		CREATE TABLE %[1]s.material_batches(id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,material_name TEXT NOT NULL DEFAULT '',owner_customer_id BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.material_batch_locations(material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL DEFAULT '',material_id BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse));
		CREATE TABLE %[1]s.work_orders(id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL);
		CREATE TABLE %[1]s.work_order_material_reservations(id BIGINT PRIMARY KEY,material_name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.work_order_material_reservation_batches(id BIGINT PRIMARY KEY,reservation_id BIGINT NOT NULL,work_order_id BIGINT NOT NULL,material_batch_id BIGINT NOT NULL,warehouse TEXT NOT NULL,reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE %[1]s.warehouses(code TEXT PRIMARY KEY,active BOOLEAN NOT NULL,customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
		CREATE TABLE %[1]s.stock_material_location_repairs(request_id TEXT PRIMARY KEY,request_hash TEXT NOT NULL,result JSONB,created_by TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		INSERT INTO %[1]s.materials VALUES(3,'曜石2.0-半成品');
		INSERT INTO %[1]s.material_batches(id,batch_code,material_id,material_name,remaining_g) VALUES(7,'MP-0000000001',3,'曜石2.0-半成品',1000);
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(7,'MP-0000000001',3,'raw_materials',1000),(7,'MP-0000000001',3,'wip',1000);
		INSERT INTO %[1]s.work_orders VALUES(88,'WO-88');
		INSERT INTO %[1]s.work_order_material_reservations VALUES(99,'曜石2.0-半成品');
		INSERT INTO %[1]s.work_order_material_reservation_batches(id,reservation_id,work_order_id,material_batch_id,warehouse,reserved_g,status) VALUES(101,99,88,7,'raw_materials',600,'reserved');
		INSERT INTO %[1]s.warehouses(code,active,customer_id) VALUES('raw_materials',true,0),('wip',true,0);
	`, schema))
	repo := NewRepository(pool, schema)
	preview, err := repo.PreviewMaterialBatchLocationRepair(ctx, 7)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.RemainingG != 1000 || len(preview.Locations) != 2 || len(preview.Reservations) != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	cmd := materialBatchLocationRepairCommandForTest(preview.Version)
	result, err := repo.RepairMaterialBatchLocations(ctx, cmd)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if result.MovedReservations != 1 || len(result.Locations) != 1 || result.Locations[0].Warehouse != "wip" {
		t.Fatalf("repair result: %+v", result)
	}
	var qtyG, reservationG int64
	var warehouse string
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT warehouse,qty_g FROM %s.material_batch_locations WHERE material_batch_id=7`, schema)).Scan(&warehouse, &qtyG); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT warehouse,reserved_g FROM %s.work_order_material_reservation_batches WHERE id=101`, schema)).Scan(&warehouse, &reservationG); err != nil {
		t.Fatal(err)
	}
	if warehouse != "wip" || qtyG != 1000 || reservationG != 600 {
		t.Fatalf("post-repair location/reservation warehouse=%s qty=%d reserved=%d", warehouse, qtyG, reservationG)
	}
	var auditCount int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='reconcile_locations' AND actor='site-user'`, schema)).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit count=%d err=%v", auditCount, err)
	}
	replayed, err := repo.RepairMaterialBatchLocations(ctx, cmd)
	if err != nil || replayed.RequestID != result.RequestID {
		t.Fatalf("idempotent replay=%+v err=%v", replayed, err)
	}
	cmd.Locations[0].QtyG++
	if _, err = repo.RepairMaterialBatchLocations(ctx, cmd); err == nil {
		t.Fatal("expected same request id with changed payload to be rejected")
	}
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs`, schema)).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("changed retry must not mutate audit rows count=%d err=%v", auditCount, err)
	}
}

func materialBatchLocationRepairCommandForTest(version string) app.MaterialBatchLocationRepairCommand {
	return app.MaterialBatchLocationRepairCommand{
		MaterialBatchID: 7,
		ExpectedVersion: version,
		RequestID:       "repair-mp-1",
		Reason:          "现场核实实际在WIP，清理启动重复库位",
		Actor:           "site-user",
		Locations:       []app.MaterialBatchLocationRepairTarget{{Warehouse: "wip", QtyG: 1000}},
		ReservationMoves: []app.MaterialBatchReservationMove{{
			ReservationBatchID: 101,
			Warehouse:          "wip",
		}},
	}
}

func TestRepairMaterialBatchLocationsRejectsStaleVersionWithoutMutation(t *testing.T) {
	pool, schema := newStockTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	// Keep this case focused on version verification by using the same minimal fixture as the happy path.
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL);
		CREATE TABLE %[1]s.material_batches(id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,material_name TEXT NOT NULL DEFAULT '',owner_customer_id BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.material_batch_locations(material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL DEFAULT '',material_id BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse));
		CREATE TABLE %[1]s.work_orders(id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL);
		CREATE TABLE %[1]s.work_order_material_reservations(id BIGINT PRIMARY KEY,material_name TEXT NOT NULL DEFAULT '');
		CREATE TABLE %[1]s.work_order_material_reservation_batches(id BIGINT PRIMARY KEY,reservation_id BIGINT NOT NULL,work_order_id BIGINT NOT NULL,material_batch_id BIGINT NOT NULL,warehouse TEXT NOT NULL,reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE %[1]s.warehouses(code TEXT PRIMARY KEY,active BOOLEAN NOT NULL,customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,field TEXT,old_value TEXT,new_value TEXT,meta JSONB);
		CREATE TABLE %[1]s.stock_material_location_repairs(request_id TEXT PRIMARY KEY,request_hash TEXT NOT NULL,result JSONB,created_by TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		INSERT INTO %[1]s.materials VALUES(3,'曜石2.0-半成品');
		INSERT INTO %[1]s.material_batches(id,batch_code,material_id,material_name,remaining_g) VALUES(7,'MP-0000000001',3,'曜石2.0-半成品',1000);
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(7,'MP-0000000001',3,'wip',1000);
		INSERT INTO %[1]s.warehouses(code,active,customer_id) VALUES('wip',true,0);
	`, schema))
	repo := NewRepository(pool, schema)
	preview, err := repo.PreviewMaterialBatchLocationRepair(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	cmd := materialBatchLocationRepairCommandForTest(preview.Version)
	cmd.RequestID = "stale-repair"
	cmd.ExpectedVersion = "stale"
	if _, err = repo.RepairMaterialBatchLocations(ctx, cmd); err == nil {
		t.Fatal("expected stale repair version to fail")
	}
	var qty int64
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=7 AND warehouse='wip'`, schema)).Scan(&qty); err != nil || qty != 1000 {
		t.Fatalf("stale repair changed location qty=%d err=%v", qty, err)
	}
	var requests int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_material_location_repairs`, schema)).Scan(&requests); err != nil || requests != 0 {
		t.Fatalf("stale request should roll back idempotency row count=%d err=%v", requests, err)
	}
}
