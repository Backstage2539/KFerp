package production

import (
	"context"
	"fmt"
	"testing"
)

func TestLegacyJobCardStartBindsReservedWIPFromPhysicalBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.work_orders(id BIGINT PRIMARY KEY,work_order_no TEXT NOT NULL,status TEXT NOT NULL);
CREATE TABLE %s.job_cards(
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,sequence_no INTEGER NOT NULL,
	assigned_to TEXT NOT NULL,workstation TEXT NOT NULL,work_center TEXT NOT NULL,
	planned_input_qty NUMERIC(14,3) NOT NULL,status TEXT NOT NULL
);
CREATE TABLE %s.work_order_material_reservations(
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,material_name TEXT NOT NULL,unit TEXT NOT NULL,
	required_g BIGINT NOT NULL,required_units BIGINT NOT NULL,reserved_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,
	consumed_g BIGINT NOT NULL,consumed_units BIGINT NOT NULL,returned_g BIGINT NOT NULL,returned_units BIGINT NOT NULL,
	source_warehouse TEXT NOT NULL,source_owner_customer_id BIGINT NOT NULL,status TEXT NOT NULL
);
CREATE TABLE %s.material_batches(
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,owner_customer_id BIGINT NOT NULL,
	status TEXT NOT NULL,quality_status TEXT NOT NULL,remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL,
	received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE %s.material_batch_locations(
	material_batch_id BIGINT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL,qty_units BIGINT NOT NULL
);
CREATE TABLE %s.work_order_material_reservation_batches(
	id BIGSERIAL PRIMARY KEY,reservation_id BIGINT NOT NULL,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,component_id BIGINT NOT NULL,component_bom_spec_id BIGINT NOT NULL DEFAULT 0,
	component_spec_g BIGINT NOT NULL DEFAULT 0,material_batch_id BIGINT NOT NULL,stock_batch_id BIGINT NOT NULL DEFAULT 0,
	batch_code TEXT NOT NULL,warehouse TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,
	reserved_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,consumed_g BIGINT NOT NULL DEFAULT 0,
	consumed_units BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id,warehouse)
);
CREATE TABLE %s.audit_logs(
	id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,
	field TEXT,old_value TEXT,new_value TEXT,meta JSONB
);
CREATE TABLE %s.quality_inspections(
	id BIGSERIAL PRIMARY KEY,work_order_id BIGINT NOT NULL,job_card_id BIGINT NOT NULL,
	reference_no TEXT NOT NULL,result TEXT NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO %s.work_orders VALUES(49,'WO-TEST-0049','running');
INSERT INTO %s.job_cards VALUES(72,49,1,'刘祎泊','PR-616 挂耳生产工位','',100,'pending');
INSERT INTO %s.work_order_material_reservations VALUES
	(50,49,66,'material','挂耳滤袋','个',0,10,0,10,0,0,0,0,'wip',0,'reserved');
INSERT INTO %s.material_batches VALUES(41,'MB-FILTER-0041',66,0,'active','unchecked',0,100,now());
INSERT INTO %s.material_batch_locations VALUES(41,66,'wip',0,10);
	`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := ensureJobCardTaskReadyTx(ctx, tx, schema, 49, 72, "", "刘祎泊"); err != nil {
		t.Fatalf("legacy reservation with ten physical WIP units should be bindable at start: %v", err)
	}
	var bound int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(reserved_units-consumed_units-returned_units),0) FROM %s.work_order_material_reservation_batches WHERE reservation_id=50 AND warehouse='wip'`, schema)).Scan(&bound); err != nil {
		t.Fatal(err)
	}
	if bound != 10 {
		t.Fatalf("bound WIP units=%d, want 10", bound)
	}
	_, available, err := taskReservationAvailableTx(ctx, tx, schema, 50, "material", "wip")
	if err != nil {
		t.Fatal(err)
	}
	if available != 10 {
		t.Fatalf("reserved WIP availability=%d, want 10", available)
	}
	var auditCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_id=50 AND action='bind_batch_for_job_card_start'`, schema)).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("reservation binding audit rows=%d, want 1", auditCount)
	}
}
