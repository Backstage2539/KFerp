package production

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAutomaticWorkOrderWIPStatusUsesPhysicalWIPWhenReservationBatchPointsElsewhere(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.work_orders(id BIGINT PRIMARY KEY,status TEXT NOT NULL);
CREATE TABLE %s.material_batches(
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,
	status TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,quality_status TEXT NOT NULL,
	remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL
);
CREATE TABLE %s.material_batch_locations(
	material_batch_id BIGINT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,
	qty_g BIGINT NOT NULL,qty_units BIGINT NOT NULL
);
CREATE TABLE %s.stock_batches(
	id BIGINT PRIMARY KEY,owner_customer_id BIGINT NOT NULL,quality_status TEXT NOT NULL,
	remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL
);
CREATE TABLE %s.work_order_material_reservations(
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,component_id BIGINT NOT NULL,component_bom_spec_id BIGINT NOT NULL,
	component_bom_variant_id BIGINT NOT NULL,component_spec_g BIGINT NOT NULL,
	material_name TEXT NOT NULL,unit TEXT NOT NULL,required_g BIGINT NOT NULL,required_units BIGINT NOT NULL,
	consumed_g BIGINT NOT NULL,consumed_units BIGINT NOT NULL,reserved_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_customer_id BIGINT NOT NULL,source_warehouse TEXT NOT NULL,status TEXT NOT NULL
);
CREATE TABLE %s.work_order_material_reservation_batches(
	id BIGINT PRIMARY KEY,reservation_id BIGINT NOT NULL,material_batch_id BIGINT NOT NULL,stock_batch_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,reserved_g BIGINT NOT NULL,consumed_g BIGINT NOT NULL,
	returned_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,consumed_units BIGINT NOT NULL,returned_units BIGINT NOT NULL,
	status TEXT NOT NULL,warehouse TEXT NOT NULL
);
INSERT INTO %s.work_orders VALUES(59,'running'),(60,'running'),(61,'released');
INSERT INTO %s.material_batches VALUES(1,'ADJ-0000000004',70,'active',0,'pass',0,200);
INSERT INTO %s.material_batch_locations VALUES(1,70,'wip',0,200);
INSERT INTO %s.stock_batches VALUES(1,0,'pass',0,0);

INSERT INTO %s.work_order_material_reservations(id,work_order_id,material_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g,material_name,unit,required_g,required_units,consumed_g,consumed_units,reserved_g,reserved_units,source_owner_customer_id,source_warehouse,status) VALUES
	(65,59,70,'material',70,0,0,0,'咖啡豆袋-454-镀铝','个',0,11,0,0,0,11,0,'wip','reserved'),
	(66,60,70,'material',70,0,0,0,'咖啡豆袋-454-镀铝','个',0,10,0,0,0,10,0,'wip','reserved'),
	(67,61,70,'material',70,0,0,0,'咖啡豆袋-454-镀铝','个',0,24,0,0,0,24,0,'wip','reserved');
INSERT INTO %s.work_order_material_reservation_batches VALUES
	(1,65,1,1,'material',0,0,0,0,11,0,0,'reserved','raw_materials');
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	status, err := automaticWorkOrderWIPStatusTx(ctx, tx, schema, 59)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "ok" || status.ShortageUnits != 0 || len(status.Materials) != 1 {
		t.Fatalf("WIP status = %+v, want no shortage for the task's 11 units; 200 physically in WIP and 34 protected for others", status)
	}
	if got := status.Materials[0].AvailableUnits; got != 11 {
		t.Fatalf("available WIP units = %d, want 11 after protecting other work orders", got)
	}
}

func TestAutoPickedWorkOrderCoverageUsesFrozenDemandInsteadOfUndersizedReservation(t *testing.T) {
	ctx := context.Background()
	pool, schema := newProductionTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %s.production_plans(id BIGINT PRIMARY KEY,picking_version INTEGER NOT NULL DEFAULT 0);
CREATE TABLE %s.work_orders(
	id BIGINT PRIMARY KEY,production_plan_id BIGINT NOT NULL,work_order_no TEXT NOT NULL,
	running_item_id BIGINT NOT NULL DEFAULT 0,product_id BIGINT NOT NULL DEFAULT 0,product_name TEXT NOT NULL DEFAULT '',
	spec_g BIGINT NOT NULL DEFAULT 0,planned_g BIGINT NOT NULL DEFAULT 0,planned_output_g BIGINT NOT NULL DEFAULT 0,
	sales_spec_count NUMERIC(18,6) NOT NULL DEFAULT 0,order_nos TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'released',
		inventory_qty_per_sales_unit NUMERIC(18,9) NOT NULL DEFAULT 0,
		inventory_unit TEXT NOT NULL DEFAULT '',planned_inventory_qty NUMERIC(18,9) NOT NULL DEFAULT 0,
	operation_template_id BIGINT NOT NULL DEFAULT 0,bom_version_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
	material_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,process_snapshot_json JSONB NOT NULL DEFAULT '{}'::jsonb,
	operation_material_bindings JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE %s.materials(id BIGINT PRIMARY KEY,owner_customer_id BIGINT NOT NULL DEFAULT 0);
CREATE TABLE %s.material_batches(
	id BIGINT PRIMARY KEY,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,
	status TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,quality_status TEXT NOT NULL,
	remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL
);
CREATE TABLE %s.material_batch_locations(
	material_batch_id BIGINT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,
	qty_g BIGINT NOT NULL,qty_units BIGINT NOT NULL
);
CREATE TABLE %s.stock_batches(
	id BIGINT PRIMARY KEY,owner_customer_id BIGINT NOT NULL,quality_status TEXT NOT NULL,
	remaining_g BIGINT NOT NULL,remaining_units BIGINT NOT NULL
);
CREATE TABLE %s.work_order_material_reservations(
	id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,component_id BIGINT NOT NULL,component_bom_spec_id BIGINT NOT NULL,
	component_bom_variant_id BIGINT NOT NULL,component_spec_g BIGINT NOT NULL,
	material_name TEXT NOT NULL,unit TEXT NOT NULL,required_g BIGINT NOT NULL,required_units BIGINT NOT NULL,
	consumed_g BIGINT NOT NULL,consumed_units BIGINT NOT NULL,reserved_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,
	returned_g BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_owner_customer_id BIGINT NOT NULL,source_warehouse TEXT NOT NULL,status TEXT NOT NULL
);
CREATE TABLE %s.work_order_material_reservation_batches(
	id BIGINT PRIMARY KEY,reservation_id BIGINT NOT NULL,material_batch_id BIGINT NOT NULL,stock_batch_id BIGINT NOT NULL,
	component_type TEXT NOT NULL,owner_customer_id BIGINT NOT NULL,reserved_g BIGINT NOT NULL,consumed_g BIGINT NOT NULL,
	returned_g BIGINT NOT NULL,reserved_units BIGINT NOT NULL,consumed_units BIGINT NOT NULL,returned_units BIGINT NOT NULL,
	status TEXT NOT NULL,warehouse TEXT NOT NULL
);
INSERT INTO %s.production_plans VALUES(1,1);
INSERT INTO %s.work_orders(
	id,production_plan_id,work_order_no,product_name,spec_g,planned_g,planned_output_g,sales_spec_count,inventory_unit,planned_inventory_qty,status,
	material_snapshot,process_snapshot_json
) VALUES(66,1,'WO-TEST-0066','挂耳包装',0,100,0,0,'袋',10,'released',
	'[{"material_id":701,"material_name":"挂耳滤袋","unit":"个","source":"packaging","qty_per_unit":1},
	  {"material_id":702,"material_name":"挂耳内膜","unit":"个","source":"packaging","qty_per_unit":1}]'::jsonb,
	'{"operations":[{"seq":1,"operation":"包装","workstation":"包装台"}]}'::jsonb);
INSERT INTO %s.materials(id) VALUES(701),(702);
INSERT INTO %s.material_batches VALUES
	(701,'FILTER-1',701,'active',0,'passed',0,1),
	(702,'LINER-1',702,'active',0,'passed',0,1);
INSERT INTO %s.material_batch_locations VALUES(701,701,'wip',0,1),(702,702,'wip',0,1);
INSERT INTO %s.work_order_material_reservations(
	id,work_order_id,material_id,component_type,component_id,component_bom_spec_id,component_bom_variant_id,
	component_spec_g,material_name,unit,required_g,required_units,consumed_g,consumed_units,reserved_g,reserved_units,
	source_owner_customer_id,source_warehouse,status
) VALUES
	(1,66,701,'material',701,0,0,0,'挂耳滤袋','个',0,1,0,0,0,1,0,'wip','reserved'),
	(2,66,702,'material',702,0,0,0,'挂耳内膜','个',0,1,0,0,0,1,0,'wip','reserved');
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema))

	status, err := NewRepository(pool, schema).GetWorkOrderWIPCoverage(ctx, 66)
	if err != nil {
		t.Fatal(err)
	}
	if !status.DataComplete || status.Status != "blocked" || len(status.Materials) != 2 {
		t.Fatalf("coverage=%+v, want two frozen BOM components and a real shortage", status)
	}
	for _, row := range status.Materials {
		if row.RequiredUnits != 10 || row.AvailableUnits != 1 || row.ShortageUnits != 9 {
			t.Errorf("material %s coverage=%+v, want required 10, physical WIP 1, shortage 9 (reservation records incorrectly say 1)", row.MaterialName, row)
		}
	}
}
