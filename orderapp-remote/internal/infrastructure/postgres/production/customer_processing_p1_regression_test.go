package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"
)

func TestCustomerProcessingPlanBasisPrefersFrozenRequestVersionAndSnapshot(t *testing.T) {
	current := customerProcessingPlanBasis{BomVersionID: 202, MaterialSnapshot: `[{"material_id":2}]`}
	frozen := customerProcessingPlanBasis{BomVersionID: 101, MaterialSnapshot: `[{"material_id":1}]`}
	got := chooseCustomerProcessingPlanBasis(99, current, frozen)
	if got.BomVersionID != 101 || got.MaterialSnapshot != frozen.MaterialSnapshot {
		t.Fatalf("plan basis = %+v, want frozen V1 request basis", got)
	}
	if got := chooseCustomerProcessingPlanBasis(0, current, frozen); got.BomVersionID != 202 {
		t.Fatalf("sales plan basis = %+v, want current BOM", got)
	}
}

func TestCustomerProcessingPlanBasisLoadsArchivedV1AfterV2Published(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.processing_job_request_items(
	id BIGINT PRIMARY KEY,product_id BIGINT NOT NULL,parent_product_id BIGINT NOT NULL DEFAULT 0,
	bom_source_product_id BIGINT NOT NULL,product_name TEXT NOT NULL,spec_g BIGINT NOT NULL,need_g BIGINT NOT NULL,
	bom_version_id BIGINT NOT NULL,material_snapshot_json JSONB NOT NULL,bom_inherited BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE %[1]s.production_boms(id BIGINT PRIMARY KEY,output_product_id BIGINT NOT NULL,code TEXT,name TEXT);
CREATE TABLE %[1]s.production_bom_versions(
	id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,version_no TEXT,status TEXT,process_route_id BIGINT,
	yield_rate NUMERIC,material_loss_rate NUMERIC,output_qty NUMERIC,output_unit TEXT
);
CREATE TABLE %[1]s.process_routes(id BIGINT PRIMARY KEY,name TEXT,status TEXT);
INSERT INTO %[1]s.production_boms VALUES(10,700,'BOM-700','目标产品BOM');
INSERT INTO %[1]s.process_routes VALUES(30,'烘焙路线','active');
INSERT INTO %[1]s.production_bom_versions VALUES
	(101,10,'V1','archived',30,0.9,0.1,1,'unit'),
	(202,10,'V2','published',30,0.95,0.05,1,'unit');
INSERT INTO %[1]s.processing_job_request_items VALUES
	(99,700,0,700,'目标SKU',454,908,101,'[{"material_id":1,"qty":1000}]'::jsonb,false);
`, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	basis, err := loadCustomerProcessingPlanBasisTx(ctx, tx, schema, 99, startRunGroup{
		ProductID: 700, ProductName: "目标SKU", SpecG: 454, NeedG: 908,
	})
	if err != nil {
		t.Fatalf("load frozen archived V1: %v", err)
	}
	if basis.BomVersionID != 101 || basis.BomRoute.BomVersionNo != "V1" || !strings.Contains(basis.MaterialSnapshot, `"material_id": 1`) {
		t.Fatalf("frozen basis = %+v, want archived V1 and its material snapshot", basis)
	}
}

func TestResolveRequestedFinishWarehouseLocksCustomerTarget(t *testing.T) {
	if got, err := resolveRequestedFinishWarehouse("", "CUSTOMER-FINISHED"); err != nil || got != "CUSTOMER-FINISHED" {
		t.Fatalf("empty requested warehouse = %q/%v", got, err)
	}
	if got, err := resolveRequestedFinishWarehouse("CUSTOMER-FINISHED", "CUSTOMER-FINISHED"); err != nil || got != "CUSTOMER-FINISHED" {
		t.Fatalf("matching requested warehouse = %q/%v", got, err)
	}
	if _, err := resolveRequestedFinishWarehouse("finished_goods", "CUSTOMER-FINISHED"); err == nil {
		t.Fatal("expected customer processing target warehouse override rejection")
	}
}

func TestDirectFinishRejectsCustomerTargetWarehouseOverride(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.customer_processing_production_demands(
	linked_running_item_id BIGINT NOT NULL,status TEXT NOT NULL,target_warehouse TEXT NOT NULL
);
INSERT INTO %s.customer_processing_production_demands VALUES(88,'running','CUSTOMER-77-FINISHED');
`, schema, schema))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if got, err := finishWarehouseForRunningItemTx(ctx, tx, schema, 88, ""); err != nil || got != "CUSTOMER-77-FINISHED" {
		t.Fatalf("direct Finish target = %q/%v, want customer warehouse", got, err)
	}
	if _, err := finishWarehouseForRunningItemTx(ctx, tx, schema, 88, "finished_goods"); err == nil {
		t.Fatal("direct Finish accepted warehouse override")
	}
}

func TestShouldDelegateFullWorkOrderCancelUsesActiveRunningItemState(t *testing.T) {
	for _, status := range []string{"running", "paused", "partially_completed"} {
		if !shouldDelegateFullWorkOrderCancel(status, 88, "running") {
			t.Fatalf("work order %s with active running item must use full cancel", status)
		}
	}
	if shouldDelegateFullWorkOrderCancel("released", 0, "") || shouldDelegateFullWorkOrderCancel("completed", 88, "done") {
		t.Fatal("inactive work order must not delegate full running cancel")
	}
}

func TestPausedAndPartiallyCompletedCancelKeepsRunningReservationWIPAndDemandConsistent(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.produce_running_items(
	id BIGINT PRIMARY KEY,batch_id TEXT,product_name TEXT,product_id BIGINT,spec_g BIGINT,need_g BIGINT,
	input_g BIGINT,bom_yield_rate NUMERIC,planned_units BIGINT,planned_loose_g BIGINT,order_nos TEXT,
	started_by TEXT,started_at TIMESTAMPTZ,material_snapshot JSONB,operation_template_id BIGINT,status TEXT,
	finished_by TEXT,finished_at TIMESTAMPTZ
);
CREATE TABLE %[1]s.produce_running_outputs(
	id BIGINT PRIMARY KEY,running_item_id BIGINT,product_id BIGINT,product_name TEXT,spec_g BIGINT,need_g BIGINT,
	order_nos TEXT,planned_units BIGINT,planned_loose_g BIGINT,finished_units BIGINT,finished_loose_g BIGINT
);
CREATE TABLE %[1]s.finished_allocation_logs(
	id BIGSERIAL PRIMARY KEY,batch_id TEXT,product_id BIGINT,spec_g BIGINT,deducted_g BIGINT
);
CREATE TABLE %[1]s.work_orders(
	id BIGINT PRIMARY KEY,running_item_id BIGINT,status TEXT,processing_request_item_id BIGINT,completed_at TIMESTAMPTZ,
	output_type TEXT NOT NULL DEFAULT 'product',output_material_id BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.job_cards(
	id BIGINT PRIMARY KEY,work_order_id BIGINT,status TEXT,completed_at TIMESTAMPTZ,operator TEXT
);
CREATE TABLE %[1]s.materials(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
CREATE TABLE %[1]s.material_batches(
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,material_id BIGINT NOT NULL,
	remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',quality_status TEXT NOT NULL DEFAULT 'unchecked',received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.material_batch_locations(
	material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL DEFAULT '',material_id BIGINT NOT NULL,
	warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse)
);
CREATE TABLE %[1]s.stock_batches(
	id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE %[1]s.stock_ledger_entries(
	id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT '',source_doc_type TEXT NOT NULL DEFAULT '',source_doc_id BIGINT NOT NULL DEFAULT 0,
	source_batch_code TEXT NOT NULL DEFAULT '',source_batch_id TEXT NOT NULL DEFAULT '',qty_before_g BIGINT NOT NULL DEFAULT 0,
	qty_change_g BIGINT NOT NULL DEFAULT 0,qty_after_g BIGINT NOT NULL DEFAULT 0,qty_before_units BIGINT NOT NULL DEFAULT 0,
	qty_change_units BIGINT NOT NULL DEFAULT 0,qty_after_units BIGINT NOT NULL DEFAULT 0,operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.customer_processing_material_reservations(
	id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL,request_item_id BIGINT NOT NULL,customer_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL,component_type TEXT NOT NULL DEFAULT 'material',component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,required_g BIGINT NOT NULL DEFAULT 0,required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_type TEXT NOT NULL,source_customer_id BIGINT NOT NULL DEFAULT 0,source_warehouse_code TEXT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,finished_stock_batch_id BIGINT NOT NULL DEFAULT 0,
	production_plan_id BIGINT NOT NULL DEFAULT 0,production_plan_item_id BIGINT NOT NULL DEFAULT 0,work_order_id BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'reserved',created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.work_order_material_reservations(
	running_item_id BIGINT,status TEXT,reserved_g BIGINT,consumed_g BIGINT,returned_g BIGINT,
	reserved_units BIGINT,consumed_units BIGINT,returned_units BIGINT,updated_at TIMESTAMPTZ
);
CREATE TABLE %[1]s.processing_job_request_items(id BIGINT PRIMARY KEY,status TEXT,updated_at TIMESTAMPTZ);
CREATE TABLE %[1]s.customer_processing_production_demands(
	request_item_id BIGINT,linked_running_item_id BIGINT,status TEXT,updated_at TIMESTAMPTZ
);
CREATE TABLE %[1]s.audit_logs(
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,
	field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.material_consumption_logs(
	id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,batch_id TEXT,product_id BIGINT,product_name TEXT,spec_g BIGINT,
	material_id BIGINT,material_name TEXT,unit TEXT,deduct_g BIGINT,deduct_units BIGINT,before_g BIGINT,after_g BIGINT,
	before_units BIGINT,after_units BIGINT,operator TEXT,material_batch_id BIGINT,material_batch_code TEXT
);
INSERT INTO %[1]s.materials VALUES(7,'测试物料');
INSERT INTO %[1]s.produce_running_items(
	id,batch_id,product_name,product_id,spec_g,need_g,input_g,bom_yield_rate,planned_units,planned_loose_g,
	order_nos,started_by,started_at,material_snapshot,operation_template_id,status
) VALUES
	(90,'RUN-PAUSED','目标产品',700,454,454,500,0.9,1,0,'','tester',now(),'[]',0,'paused'),
	(91,'RUN-PARTIAL','目标产品',700,454,454,500,0.9,1,0,'','tester',now(),'[]',0,'partially_completed');
INSERT INTO %[1]s.work_orders(
	id,running_item_id,status,processing_request_item_id,completed_at,output_type,output_material_id
) VALUES
	(190,90,'paused',290,NULL,'material',7),
	(191,91,'partially_completed',291,NULL,'material',7);
INSERT INTO %[1]s.job_cards VALUES(390,190,'paused',NULL,''),(391,191,'running',NULL,'');
INSERT INTO %[1]s.material_batches(id,batch_code,material_id,remaining_g,quality_status) VALUES
	(490,'MAT-PAUSED',7,500,'pass'),(491,'MAT-PARTIAL',7,500,'pass');
INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES
	(490,'MAT-PAUSED',7,'wip',500),(490,'MAT-PAUSED',7,'CUSTOMER-77-RAW',0),
	(491,'MAT-PARTIAL',7,'wip',500),(491,'MAT-PARTIAL',7,'CUSTOMER-77-RAW',0);
INSERT INTO %[1]s.stock_batches(batch_code,remaining_g) VALUES('MAT-PAUSED',500),('MAT-PARTIAL',500);
INSERT INTO %[1]s.processing_job_request_items VALUES(290,'running',now()),(291,'running',now());
INSERT INTO %[1]s.customer_processing_production_demands VALUES(290,90,'running',now()),(291,91,'running',now());
INSERT INTO %[1]s.customer_processing_material_reservations(
	id,request_id,request_item_id,customer_id,material_id,required_g,reserved_g,source_owner_type,
	source_customer_id,source_warehouse_code,material_batch_id,work_order_id
) VALUES
	(590,1,290,77,7,500,500,'customer',77,'CUSTOMER-77-RAW',490,190),
	(591,2,291,77,7,500,500,'customer',77,'CUSTOMER-77-RAW',491,191);
`, schema))
	repo := NewRepository(pool, schema)
	for _, tc := range []struct {
		runningID, workOrderID, requestItemID, reservationID, materialBatchID int64
	}{
		{90, 190, 290, 590, 490},
		{91, 191, 291, 591, 491},
	} {
		if err := repo.Cancel(ctx, productionapp.CancelCommand{ID: tc.runningID, Operator: "tester"}); err != nil {
			t.Fatalf("cancel running %d: %v", tc.runningID, err)
		}
		var runningStatus, workOrderStatus, requestStatus, demandStatus, reservationStatus string
		var returnedG, wipG, sourceG int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT r.status,wo.status,i.status,d.status,mr.status,mr.returned_g,
			       COALESCE(wip.qty_g,0),COALESCE(source.qty_g,0)
			FROM %[1]s.produce_running_items r
			JOIN %[1]s.work_orders wo ON wo.id=$2
			JOIN %[1]s.processing_job_request_items i ON i.id=$3
			JOIN %[1]s.customer_processing_production_demands d ON d.request_item_id=i.id
			JOIN %[1]s.customer_processing_material_reservations mr ON mr.id=$4
			LEFT JOIN %[1]s.material_batch_locations wip ON wip.material_batch_id=$5 AND wip.warehouse='wip'
			LEFT JOIN %[1]s.material_batch_locations source ON source.material_batch_id=$5 AND source.warehouse='CUSTOMER-77-RAW'
			WHERE r.id=$1
		`, schema), tc.runningID, tc.workOrderID, tc.requestItemID, tc.reservationID, tc.materialBatchID).Scan(
			&runningStatus, &workOrderStatus, &requestStatus, &demandStatus, &reservationStatus, &returnedG, &wipG, &sourceG,
		); err != nil {
			t.Fatal(err)
		}
		if runningStatus != "cancelled" || workOrderStatus != "cancelled" || requestStatus != "cancelled" || demandStatus != "cancelled" || reservationStatus != "released" || returnedG != 500 || wipG != 0 || sourceG != 500 {
			t.Fatalf("cancel %d consistency = run:%s wo:%s request:%s demand:%s reservation:%s returned:%d wip:%d source:%d",
				tc.runningID, runningStatus, workOrderStatus, requestStatus, demandStatus, reservationStatus, returnedG, wipG, sourceG)
		}
		var cancelAuditCount int
		if err := pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM %s.audit_logs
			WHERE action='cancel'
			  AND ((entity_type='produce_running' AND entity_id=$1)
			       OR (entity_type='work_order' AND entity_id=$2))
		`, schema), tc.runningID, tc.workOrderID).Scan(&cancelAuditCount); err != nil {
			t.Fatal(err)
		}
		if cancelAuditCount != 2 {
			t.Fatalf("cancel %d audit count = %d, want atomic produce_running + work_order audit", tc.runningID, cancelAuditCount)
		}
	}

	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.produce_running_items(
			id,batch_id,product_name,product_id,spec_g,need_g,input_g,bom_yield_rate,planned_units,planned_loose_g,
			order_nos,started_by,started_at,material_snapshot,operation_template_id,status
		) VALUES(92,'RUN-AUDIT-ROLLBACK','待回滚半成品',0,0,500,500,1,0,500,'','tester',now(),'[]',0,'running');
		INSERT INTO %[1]s.work_orders(
			id,running_item_id,status,processing_request_item_id,completed_at,output_type,output_material_id
		) VALUES(192,92,'running',0,NULL,'material',7);
		INSERT INTO %[1]s.job_cards VALUES(392,192,'running',NULL,'');
		INSERT INTO %[1]s.work_order_material_reservations(
			running_item_id,status,reserved_g,consumed_g,returned_g,reserved_units,consumed_units,returned_units,updated_at
		) VALUES(92,'reserved',500,0,0,0,0,0,now());
		CREATE OR REPLACE FUNCTION %[1]s.reject_running_cancel_audit() RETURNS trigger AS $audit$
		BEGIN
			IF NEW.action='cancel' AND NEW.entity_type='produce_running' THEN
				RAISE EXCEPTION 'forced cancel audit failure';
			END IF;
			RETURN NEW;
		END
		$audit$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_running_cancel_audit
		BEFORE INSERT ON %[1]s.audit_logs
		FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_running_cancel_audit();
	`, schema))
	if err := repo.Cancel(ctx, productionapp.CancelCommand{ID: 92, Operator: "tester"}); err == nil || !strings.Contains(err.Error(), "forced cancel audit failure") {
		t.Fatalf("cancel audit failure = %v, want transaction error", err)
	}
	var runningStatus, workOrderStatus, jobCardStatus, reservationStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT run.status,wo.status,jc.status,res.status
		FROM %[1]s.produce_running_items run
		JOIN %[1]s.work_orders wo ON wo.running_item_id=run.id
		JOIN %[1]s.job_cards jc ON jc.work_order_id=wo.id
		JOIN %[1]s.work_order_material_reservations res ON res.running_item_id=run.id
		WHERE run.id=92
	`, schema)).Scan(&runningStatus, &workOrderStatus, &jobCardStatus, &reservationStatus); err != nil {
		t.Fatal(err)
	}
	if runningStatus != "running" || workOrderStatus != "running" || jobCardStatus != "running" || reservationStatus != "reserved" {
		t.Fatalf("audit failure did not roll back cancellation: run=%s wo=%s job=%s reservation=%s", runningStatus, workOrderStatus, jobCardStatus, reservationStatus)
	}
	var cancelAuditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE action='cancel'`, schema)).Scan(&cancelAuditCount); err != nil {
		t.Fatal(err)
	}
	if cancelAuditCount != 4 {
		t.Fatalf("final running audit failure left prior audit rows: count=%d, want two successful cancel pairs only", cancelAuditCount)
	}
}

func TestCustomerProcessingFinishedProductFIFOAllocatesConcreteBatches(t *testing.T) {
	rows := []customerProcessingFinishedBatchAvailability{
		{StockBatchID: 11, BatchCode: "FP-OLD", AvailableG: 400},
		{StockBatchID: 12, BatchCode: "FP-NEW", AvailableG: 500},
	}
	got, err := allocateCustomerProcessingFinishedBatches(rows, 700, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].StockBatchID != 11 || got[0].QtyG != 400 || got[1].StockBatchID != 12 || got[1].QtyG != 300 {
		t.Fatalf("finished FIFO = %+v, want old 400g then new 300g", got)
	}
}

func TestFinishedComponentConsumptionWarehouseSeparatesCustomerWIPFromOrdinaryStock(t *testing.T) {
	if got, err := finishedComponentConsumptionWarehouse(nil); err != nil || got != "finished_goods" {
		t.Fatalf("ordinary legacy warehouse = %q/%v", got, err)
	}
	if got, err := finishedComponentConsumptionWarehouse([]customerProcessingFinishedBatchAllocation{{StockBatchID: 1}}); err != nil || got != "finished_goods" {
		t.Fatalf("ordinary concrete-batch warehouse = %q/%v", got, err)
	}
	if got, err := finishedComponentConsumptionWarehouse([]customerProcessingFinishedBatchAllocation{{StockBatchID: 2, ReservationID: 9}}); err != nil || got != "wip" {
		t.Fatalf("customer processing warehouse = %q/%v", got, err)
	}
}

func TestCustomerProcessingFinishedProductOwnershipLifecycle(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	}()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.products(id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '');
CREATE TABLE %[1]s.work_orders(id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0);
CREATE TABLE %[1]s.finished_inventory(
	product_id BIGINT NOT NULL,spec_g BIGINT NOT NULL,warehouse TEXT NOT NULL,
	onhand_units BIGINT NOT NULL DEFAULT 0,onhand_loose_g BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(product_id,spec_g,warehouse)
);
CREATE TABLE %[1]s.stock_batches(
	id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,
	item_name TEXT NOT NULL DEFAULT '',spec_g BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,
	remaining_units BIGINT NOT NULL DEFAULT 0,quality_status TEXT NOT NULL DEFAULT 'unchecked',unit_cost NUMERIC NOT NULL DEFAULT 0,
	source_doc_type TEXT NOT NULL DEFAULT '',source_doc_id BIGINT NOT NULL DEFAULT 0,source_batch_id TEXT NOT NULL DEFAULT '',
	qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,operator TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.stock_ledger_entries(
	id BIGSERIAL PRIMARY KEY,item_type TEXT NOT NULL DEFAULT '',item_id BIGINT NOT NULL DEFAULT 0,item_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT '',source_doc_type TEXT NOT NULL DEFAULT '',
	source_doc_id BIGINT NOT NULL DEFAULT 0,source_batch_code TEXT NOT NULL DEFAULT '',source_batch_id TEXT NOT NULL DEFAULT '',
	qty_before_g BIGINT NOT NULL DEFAULT 0,qty_change_g BIGINT NOT NULL DEFAULT 0,qty_after_g BIGINT NOT NULL DEFAULT 0,
	qty_before_units BIGINT NOT NULL DEFAULT 0,qty_change_units BIGINT NOT NULL DEFAULT 0,qty_after_units BIGINT NOT NULL DEFAULT 0,
	operator TEXT NOT NULL DEFAULT '',created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.audit_logs(
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,
	field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %[1]s.material_consumption_logs(
	id BIGSERIAL PRIMARY KEY,running_item_id BIGINT,batch_id TEXT,product_id BIGINT,product_name TEXT,spec_g BIGINT,
	material_id BIGINT,material_name TEXT,unit TEXT,deduct_g BIGINT,deduct_units BIGINT,before_g BIGINT,after_g BIGINT,
	before_units BIGINT,after_units BIGINT,operator TEXT,material_batch_id BIGINT,material_batch_code TEXT
);
CREATE TABLE %[1]s.customer_processing_material_reservations(
	id BIGSERIAL PRIMARY KEY,request_id BIGINT NOT NULL,request_item_id BIGINT NOT NULL,customer_id BIGINT NOT NULL,
	material_id BIGINT NOT NULL,component_type TEXT NOT NULL,component_product_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,required_g BIGINT NOT NULL DEFAULT 0,required_units BIGINT NOT NULL DEFAULT 0,
	reserved_g BIGINT NOT NULL DEFAULT 0,reserved_units BIGINT NOT NULL DEFAULT 0,consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_type TEXT NOT NULL,source_customer_id BIGINT NOT NULL DEFAULT 0,source_warehouse_code TEXT NOT NULL,
	material_batch_id BIGINT NOT NULL DEFAULT 0,finished_stock_batch_id BIGINT NOT NULL DEFAULT 0,
	production_plan_id BIGINT NOT NULL DEFAULT 0,production_plan_item_id BIGINT NOT NULL DEFAULT 0,
	work_order_id BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'reserved',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %[1]s.products VALUES(50,'客户拼配熟豆');
INSERT INTO %[1]s.work_orders(id) VALUES(9),(10);
INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_loose_g) VALUES
	(50,1000,'CUSTOMER-77-FINISHED',900),(50,1000,'finished_goods',600);
INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,item_name,spec_g,remaining_g,quality_status,created_at) VALUES
	(11,'FP-CUSTOMER-OLD','finished_product',50,'客户拼配熟豆',1000,900,'pass',now()-interval '3 days'),
	(12,'FP-FACTORY','finished_product',50,'客户拼配熟豆',1000,600,'pass',now()-interval '2 days');
SELECT setval(pg_get_serial_sequence('%[1]s.stock_batches','id'),12,true);
INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_batch_code,source_batch_id) VALUES
	('finished_product',50,'客户拼配熟豆',1000,'CUSTOMER-77-FINISHED','FP-CUSTOMER-OLD','FP-CUSTOMER-OLD'),
	('finished_product',50,'客户拼配熟豆',1000,'finished_goods','FP-FACTORY','FP-FACTORY');
INSERT INTO %[1]s.customer_processing_material_reservations(
	id,request_id,request_item_id,customer_id,material_id,component_type,component_product_id,component_spec_g,
	required_g,reserved_g,source_owner_type,source_customer_id,source_warehouse_code,work_order_id
) VALUES
	(20,1,11,77,50,'finished_product',50,1000,400,400,'customer',77,'CUSTOMER-77-FINISHED',9),
	(30,1,11,77,50,'finished_product',50,1000,600,600,'factory',0,'finished_goods',9),
	(40,2,12,77,50,'finished_product',50,1000,500,500,'customer',77,'CUSTOMER-77-FINISHED',10);
SELECT setval(pg_get_serial_sequence('%[1]s.customer_processing_material_reservations','id'),40,true);
`, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bound, err := issueCustomerProcessingReservationsToWIPTx(ctx, tx, schema, 9, "tester"); err != nil || bound != 2 {
		_ = tx.Rollback(ctx)
		t.Fatalf("issue mixed finished reservations = %d/%v", bound, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var issuedCustomerG, issuedFactoryG, issuedWIPG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='CUSTOMER-77-FINISHED'),
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='finished_goods'),
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='wip')
	`, schema)).Scan(&issuedCustomerG, &issuedFactoryG, &issuedWIPG); err != nil {
		t.Fatal(err)
	}
	if issuedCustomerG != 500 || issuedFactoryG != 0 || issuedWIPG != 1000 {
		t.Fatalf("issued aggregate customer/factory/WIP = %d/%d/%d, want 500/0/1000", issuedCustomerG, issuedFactoryG, issuedWIPG)
	}
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.work_orders SET running_item_id=99 WHERE id=9`, schema))
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := deductFinishedProductComponentForRunningItemTx(ctx, tx, schema, ProduceRunRow{
		ID: 99, BatchID: "RUN-99", ProductID: 700, Product: "目标产品", SpecG: 1000, NeedG: 700,
	}, materialConsumptionNeed{
		MaterialID: 50, MaterialName: "客户拼配熟豆", ComponentType: "finished_product",
		ComponentProductID: 50, ComponentSpecG: 1000, DeductG: 700, Unit: "g",
	}, "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var consumedWIPG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_units*spec_g+onhand_loose_g FROM %s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='wip'`, schema)).Scan(&consumedWIPG); err != nil {
		t.Fatal(err)
	}
	if consumedWIPG != 300 {
		t.Fatalf("actual finished-component WIP = %d, want 300 after consuming 700", consumedWIPG)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := completeCustomerProcessingReservationsForRunningItemTx(ctx, tx, schema, 99, "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var customerConsumed, factoryConsumed, factoryReturned int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT MAX(consumed_g) FILTER(WHERE source_owner_type='customer'),
		       MAX(consumed_g) FILTER(WHERE source_owner_type='factory'),
		       MAX(returned_g) FILTER(WHERE source_owner_type='factory')
		FROM %s.customer_processing_material_reservations WHERE work_order_id=9
	`, schema)).Scan(&customerConsumed, &factoryConsumed, &factoryReturned); err != nil {
		t.Fatal(err)
	}
	if customerConsumed != 400 || factoryConsumed != 300 || factoryReturned != 300 {
		t.Fatalf("mixed ownership actual/return = %d/%d/%d, want 400/300/300", customerConsumed, factoryConsumed, factoryReturned)
	}
	var settledCustomerG, settledFactoryG, settledWIPG, customerBatchG, factoryBatchG, totalBatchG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='CUSTOMER-77-FINISHED'),
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='finished_goods'),
			(SELECT onhand_units*spec_g+onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='wip'),
			(SELECT remaining_g FROM %[1]s.stock_batches WHERE id=11),
			(SELECT remaining_g FROM %[1]s.stock_batches WHERE id=12),
			(SELECT COALESCE(SUM(remaining_g),0) FROM %[1]s.stock_batches WHERE item_type='finished_product' AND item_id=50)
	`, schema)).Scan(&settledCustomerG, &settledFactoryG, &settledWIPG, &customerBatchG, &factoryBatchG, &totalBatchG); err != nil {
		t.Fatal(err)
	}
	if settledCustomerG != 500 || settledFactoryG != 300 || settledWIPG != 0 || customerBatchG != 500 || factoryBatchG != 300 || totalBatchG != 800 {
		t.Fatalf("settled aggregate/batches customer:%d factory:%d WIP:%d customerBatch:%d factoryBatch:%d totalBatch:%d",
			settledCustomerG, settledFactoryG, settledWIPG, customerBatchG, factoryBatchG, totalBatchG)
	}

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
	var customerWarehouseG, cancelReturned int64
	var cancelStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_loose_g FROM %s.finished_inventory WHERE product_id=50 AND spec_g=1000 AND warehouse='CUSTOMER-77-FINISHED'`, schema)).Scan(&customerWarehouseG); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,returned_g FROM %s.customer_processing_material_reservations WHERE id=40`, schema)).Scan(&cancelStatus, &cancelReturned); err != nil {
		t.Fatal(err)
	}
	if customerWarehouseG != 500 || cancelStatus != "released" || cancelReturned != 500 {
		t.Fatalf("customer-only cancel warehouse/status/returned = %d/%s/%d, want 500/released/500", customerWarehouseG, cancelStatus, cancelReturned)
	}

	// A finished batch may carry both 1000g and one display unit. Grams are the
	// stock authority: issuing only 400g must not leave a complete-unit ghost on
	// the 600g parent or create one on the 400g child.
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products VALUES(51,'完整件拆分回归熟豆');
		INSERT INTO %[1]s.work_orders(id,running_item_id) VALUES(11,101);
		INSERT INTO %[1]s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES
			(51,1000,'finished_goods',1,0);
		INSERT INTO %[1]s.stock_batches(
			id,batch_code,item_type,item_id,item_name,spec_g,qty_g,qty_units,remaining_g,remaining_units,quality_status,created_at
		) VALUES(50,'FP-UNIT-ONE','finished_product',51,'完整件拆分回归熟豆',1000,1000,1,1000,1,'pass',now());
		SELECT setval(pg_get_serial_sequence('%[1]s.stock_batches','id'),50,true);
		INSERT INTO %[1]s.stock_ledger_entries(item_type,item_id,item_name,spec_g,warehouse,source_batch_code,source_batch_id)
		VALUES('finished_product',51,'完整件拆分回归熟豆',1000,'finished_goods','FP-UNIT-ONE','FP-UNIT-ONE');
		INSERT INTO %[1]s.customer_processing_material_reservations(
			id,request_id,request_item_id,customer_id,material_id,component_type,component_product_id,component_spec_g,
			required_g,required_units,reserved_g,reserved_units,source_owner_type,source_customer_id,source_warehouse_code,work_order_id
		) VALUES(50,3,13,77,51,'finished_product',51,1000,400,0,400,0,'factory',0,'finished_goods',11);
	`, schema))
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bound, issueErr := issueCustomerProcessingReservationsToWIPTx(ctx, tx, schema, 11, "tester"); issueErr != nil || bound != 1 {
		_ = tx.Rollback(ctx)
		t.Fatalf("issue partial complete-unit batch = %d/%v", bound, issueErr)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var parentG, parentUnits, childG, childUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT remaining_g FROM %[1]s.stock_batches WHERE id=50),
			(SELECT remaining_units FROM %[1]s.stock_batches WHERE id=50),
			(SELECT remaining_g FROM %[1]s.stock_batches WHERE source_batch_id='FP-UNIT-ONE'),
			(SELECT remaining_units FROM %[1]s.stock_batches WHERE source_batch_id='FP-UNIT-ONE')
	`, schema)).Scan(&parentG, &parentUnits, &childG, &childUnits); err != nil {
		t.Fatal(err)
	}
	if parentG != 600 || parentUnits != 0 || childG != 400 || childUnits != 0 {
		t.Fatalf("partial issue parent/child g+units = %d/%d + %d/%d, want 600/0 + 400/0", parentG, parentUnits, childG, childUnits)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := deductFinishedProductComponentForRunningItemTx(ctx, tx, schema, ProduceRunRow{
		ID: 101, BatchID: "RUN-101", ProductID: 701, Product: "目标产品", SpecG: 1000, NeedG: 250,
	}, materialConsumptionNeed{
		MaterialID: 51, MaterialName: "完整件拆分回归熟豆", ComponentType: "finished_product",
		ComponentProductID: 51, ComponentSpecG: 1000, DeductG: 250, Unit: "g",
	}, "tester"); err != nil {
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
	if _, err := completeCustomerProcessingReservationsForRunningItemTx(ctx, tx, schema, 101, "tester"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var sourceUnits, sourceLoose, wipUnits, wipLoose, unitBatchTotalG, unitBatchTotalUnits, consumedG, returnedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			(SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=51 AND spec_g=1000 AND warehouse='finished_goods'),
			(SELECT onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=51 AND spec_g=1000 AND warehouse='finished_goods'),
			(SELECT onhand_units FROM %[1]s.finished_inventory WHERE product_id=51 AND spec_g=1000 AND warehouse='wip'),
			(SELECT onhand_loose_g FROM %[1]s.finished_inventory WHERE product_id=51 AND spec_g=1000 AND warehouse='wip'),
			(SELECT SUM(remaining_g) FROM %[1]s.stock_batches WHERE item_type='finished_product' AND item_id=51),
			(SELECT SUM(remaining_units) FROM %[1]s.stock_batches WHERE item_type='finished_product' AND item_id=51),
			(SELECT consumed_g FROM %[1]s.customer_processing_material_reservations WHERE id=50),
			(SELECT returned_g FROM %[1]s.customer_processing_material_reservations WHERE id=50)
	`, schema)).Scan(&sourceUnits, &sourceLoose, &wipUnits, &wipLoose, &unitBatchTotalG, &unitBatchTotalUnits, &consumedG, &returnedG); err != nil {
		t.Fatal(err)
	}
	if sourceUnits != 0 || sourceLoose != 750 || wipUnits != 0 || wipLoose != 0 || unitBatchTotalG != 750 || unitBatchTotalUnits != 0 || consumedG != 250 || returnedG != 150 {
		t.Fatalf("settled grams authority source=%d/%d wip=%d/%d batches=%d/%d consumed/returned=%d/%d",
			sourceUnits, sourceLoose, wipUnits, wipLoose, unitBatchTotalG, unitBatchTotalUnits, consumedG, returnedG)
	}
}
