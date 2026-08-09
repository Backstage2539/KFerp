package production

import (
	"context"
	"fmt"
	"testing"
)

func TestCustomerProcessingReservationIssueOrderPrefersCustomerStock(t *testing.T) {
	rows := []customerProcessingMaterialReservation{
		{ID: 30, SourceOwnerType: "factory", SourceWarehouseCode: "raw_materials"},
		{ID: 20, SourceOwnerType: "customer", SourceCustomerID: 77, SourceWarehouseCode: "CUST-77-WIP"},
		{ID: 10, SourceOwnerType: "customer", SourceCustomerID: 77, SourceWarehouseCode: "CUST-77-RAW"},
	}
	sortCustomerProcessingReservationsForIssue(rows)
	if rows[0].ID != 10 || rows[1].ID != 20 || rows[2].ID != 30 {
		t.Fatalf("issue order = %d,%d,%d, want customer FIFO 10,20 then factory 30", rows[0].ID, rows[1].ID, rows[2].ID)
	}
}

func TestCustomerProcessingReservationRemainingTracksActualConsumption(t *testing.T) {
	row := customerProcessingMaterialReservation{ReservedG: 1000, ConsumedG: 350, ReturnedG: 100}
	if got := customerProcessingReservationRemainingG(row); got != 550 {
		t.Fatalf("remaining = %d, want 550g", got)
	}
	row.ConsumedG = 1200
	if got := customerProcessingReservationRemainingG(row); got != 0 {
		t.Fatalf("over-consumed remaining = %d, want 0", got)
	}
}

func TestCustomerProcessingMaterialLifecycleUsesCustomerFirstAndConsumesOnlyAtFinish(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.materials (
	id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %s.material_batches (
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,material_id BIGINT NOT NULL,
	remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',quality_status TEXT NOT NULL DEFAULT 'unchecked',
	received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations (
	material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL DEFAULT '',material_id BIGINT NOT NULL,
	warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse)
);
CREATE TABLE %s.stock_batches (
	id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.stock_ledger_entries (
	id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,
	item_name TEXT NOT NULL DEFAULT '',spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT '',
	source_doc_type TEXT NOT NULL DEFAULT '',source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,qty_change_g BIGINT NOT NULL DEFAULT 0,qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,qty_change_units BIGINT NOT NULL DEFAULT 0,qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.audit_logs (
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,
	action TEXT NOT NULL,field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.work_orders (
	id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %s.customer_processing_material_reservations (
	id BIGINT PRIMARY KEY,request_id BIGINT NOT NULL,request_item_id BIGINT NOT NULL,customer_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL,component_type TEXT NOT NULL DEFAULT 'material',component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,required_g BIGINT NOT NULL DEFAULT 0,required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,
	consumed_g BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_type TEXT NOT NULL,source_customer_id BIGINT NOT NULL DEFAULT 0,source_warehouse_code TEXT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,finished_stock_batch_id BIGINT NOT NULL DEFAULT 0,production_plan_id BIGINT NOT NULL DEFAULT 0,
	production_plan_item_id BIGINT NOT NULL DEFAULT 0,work_order_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.materials VALUES(7,'测试生豆');
INSERT INTO %s.material_batches(id,batch_code,material_id,remaining_g,status,quality_status,received_at) VALUES
	(1,'CUSTOMER-BATCH',7,600,'active','pass',now()-interval '2 days'),
	(2,'FACTORY-BATCH',7,600,'active','pass',now()-interval '1 day');
INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES
	(1,'CUSTOMER-BATCH',7,'CUSTOMER-77-RAW',600),
	(2,'FACTORY-BATCH',7,'raw_materials',600);
INSERT INTO %s.stock_batches(batch_code,remaining_g) VALUES('CUSTOMER-BATCH',600),('FACTORY-BATCH',600);
INSERT INTO %s.work_orders(id) VALUES(9);
INSERT INTO %s.customer_processing_material_reservations(
	id,request_id,request_item_id,customer_id,material_id,required_g,reserved_g,
	source_owner_type,source_customer_id,source_warehouse_code,work_order_id
) VALUES
	(20,1,11,77,7,600,600,'customer',77,'CUSTOMER-77-RAW',9),
	(30,1,11,77,7,400,400,'factory',0,'raw_materials',9);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema,
		schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := issueCustomerProcessingReservationsToWIPTx(ctx, tx, schema, 9, "tester")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if bound != 2 {
		t.Fatalf("bound reservations = %d, want 2", bound)
	}
	var reservedStatus string
	var customerBatchID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,material_batch_id FROM %s.customer_processing_material_reservations WHERE id=20`, schema)).Scan(&reservedStatus, &customerBatchID); err != nil {
		t.Fatal(err)
	}
	if reservedStatus != "reserved" || customerBatchID != 1 {
		t.Fatalf("after issue status/batch = %s/%d, want reserved/1", reservedStatus, customerBatchID)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, isolationErr := unreservedWIPBatchConsumptionsTx(ctx, tx, schema, 7, 100, 0)
	_ = tx.Rollback(ctx)
	if isolationErr == nil {
		t.Fatal("ordinary work order allocation consumed customer-processing reserved WIP")
	}
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET running_item_id=99 WHERE id=9`, schema))
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	allocations, err := materialBatchConsumptionsForRunningItemTx(ctx, tx, schema, 99, 7, 800, 0)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if len(allocations) != 2 || allocations[0].BatchID != 1 || allocations[0].QtyG != 600 || allocations[1].BatchID != 2 || allocations[1].QtyG != 200 {
		t.Fatalf("actual allocations = %+v, want customer 600g then factory 200g", allocations)
	}
	var customerConsumed, factoryConsumed int64
	var customerStatus, factoryStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT MAX(consumed_g) FILTER (WHERE source_owner_type='customer'),
		       MAX(consumed_g) FILTER (WHERE source_owner_type='factory'),
		       MAX(status) FILTER (WHERE source_owner_type='customer'),
		       MAX(status) FILTER (WHERE source_owner_type='factory')
		FROM %s.customer_processing_material_reservations
	`, schema)).Scan(&customerConsumed, &factoryConsumed, &customerStatus, &factoryStatus); err != nil {
		t.Fatal(err)
	}
	if customerConsumed != 600 || factoryConsumed != 200 || customerStatus != "reserved" || factoryStatus != "reserved" {
		t.Fatalf("actual/status = customer %d/%s factory %d/%s, want 600/reserved 200/reserved", customerConsumed, customerStatus, factoryConsumed, factoryStatus)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settled, err := completeCustomerProcessingReservationsForRunningItemTx(ctx, tx, schema, 99, "tester")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if settled != 2 {
		t.Fatalf("settled = %d, want 2", settled)
	}
	var factoryRawG, factoryWIPG, factoryReturned int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=2 AND warehouse='raw_materials'`, schema)).Scan(&factoryRawG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=2 AND warehouse='wip'`, schema)).Scan(&factoryWIPG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT returned_g FROM %s.customer_processing_material_reservations WHERE id=30`, schema)).Scan(&factoryReturned); err != nil {
		t.Fatal(err)
	}
	if factoryRawG != 400 || factoryWIPG != 0 || factoryReturned != 200 {
		t.Fatalf("factory returned raw/wip/record = %d/%d/%d, want 400/0/200", factoryRawG, factoryWIPG, factoryReturned)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.customer_processing_material_reservations WHERE id=30`, schema)).Scan(&factoryStatus); err != nil {
		t.Fatal(err)
	}
	if factoryStatus != "consumed" {
		t.Fatalf("final reservation status = %s, want consumed", factoryStatus)
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.material_batches(id,batch_code,material_id,remaining_g,status,quality_status,received_at)
		VALUES(3,'CANCEL-BATCH',7,500,'active','pass',now());
		INSERT INTO %s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g)
		VALUES(3,'CANCEL-BATCH',7,'CUSTOMER-77-RAW',500);
		INSERT INTO %s.stock_batches(batch_code,remaining_g) VALUES('CANCEL-BATCH',500);
		INSERT INTO %s.work_orders(id) VALUES(10);
		INSERT INTO %s.customer_processing_material_reservations(
			id,request_id,request_item_id,customer_id,material_id,required_g,reserved_g,
			source_owner_type,source_customer_id,source_warehouse_code,work_order_id
		) VALUES(40,2,12,77,7,500,500,'customer',77,'CUSTOMER-77-RAW',10);
	`, schema, schema, schema, schema, schema))
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issueCustomerProcessingReservationsToWIPTx(ctx, tx, schema, 10, "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := settleCustomerProcessingReservationsForWorkOrderTx(ctx, tx, schema, 10, true, "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var cancelSourceG, cancelWIPG, cancelReturned int64
	var cancelStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=3 AND warehouse='CUSTOMER-77-RAW'`, schema)).Scan(&cancelSourceG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=3 AND warehouse='wip'`, schema)).Scan(&cancelWIPG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,returned_g FROM %s.customer_processing_material_reservations WHERE id=40`, schema)).Scan(&cancelStatus, &cancelReturned); err != nil {
		t.Fatal(err)
	}
	if cancelSourceG != 500 || cancelWIPG != 0 || cancelStatus != "released" || cancelReturned != 500 {
		t.Fatalf("cancel source/wip/status/returned = %d/%d/%s/%d, want 500/0/released/500", cancelSourceG, cancelWIPG, cancelStatus, cancelReturned)
	}
}
