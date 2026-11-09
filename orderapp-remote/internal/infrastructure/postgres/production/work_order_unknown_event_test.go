package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestWorkOrderAndJobCardUnknownEventAcrossCostModes(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	defer func() { _, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); pool.Close() }()
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
 CREATE TABLE %[1]s.production_plans(id bigint primary key,plan_no text);
 CREATE TABLE %[1]s.manufacturing_operation_employees(operation_id bigint,employee_id bigint,default_role text);
 CREATE TABLE %[1]s.company_employees(id bigint,name text,active boolean);
 CREATE TABLE %[1]s.work_orders(production_plan_id bigint,status text,inventory_unit text,sales_spec_count numeric,planned_inventory_qty numeric,id bigint primary key,work_order_no text,product_id bigint,product_name text,spec_g bigint,order_nos text,planned_g bigint,planned_output_g bigint,bom_version_id bigint,material_snapshot jsonb,process_snapshot_json jsonb,production_config_snapshot_json jsonb,customer_product_snapshot_json jsonb);
 CREATE TABLE %[1]s.job_cards(id bigint primary key,work_order_id bigint,sequence_no int,operation_id bigint,workstation_id bigint,operation text,workstation text,workstation_capacity_id bigint,workstation_capacity_name text,batch_size_qty numeric,batch_size_unit text,planned_batch_count int,planned_minutes int,hourly_rate numeric,cost_method text,piece_rate numeric,planned_operation_cost numeric,actual_minutes int,actual_operation_cost numeric,status text,started_at timestamptz,paused_at timestamptz,resumed_at timestamptz,completed_at timestamptz,operator text,planned_input_qty numeric,actual_input_qty numeric,actual_output_qty numeric,actual_loss_qty numeric,actual_loss_rate numeric,records_loss boolean,loss_reason text,exception_reason text,metrics_json jsonb,parameter_schema_json jsonb,planned_start_at timestamptz,planned_end_at timestamptz,shift_code text,assigned_to text,priority int,scheduling_note text,work_center text,scheduling_mode text,throughput_per_hour numeric);
 CREATE TABLE %[1]s.production_operation_events(job_card_id bigint,kind text,status text,payload jsonb);
 INSERT INTO %[1]s.work_orders(id,work_order_no,product_name) VALUES(1,'batch','批次'),(2,'piece','计件'),(3,'known','正常');
 INSERT INTO %[1]s.job_cards(id,work_order_id,sequence_no,operation,workstation,cost_method,status,operator,scheduling_mode,throughput_per_hour,actual_operation_cost) VALUES(11,1,1,'batch','','time','completed','','batch',0,15),(22,2,1,'piece','','piece','completed','','quantity',0,20),(33,3,1,'batch','','time','completed','','batch',0,25);
 INSERT INTO %[1]s.production_operation_events VALUES(11,'complete','posted','{"cost_unavailable":true}'),(22,'complete','posted','{"cost_unavailable":true}'),(33,'complete','posted','{"cost_unavailable":false}'),(33,'complete','void','{"cost_unavailable":true}');`, schema))
	repo := NewRepository(pool, schema)
	for _, tc := range []struct {
		id   int64
		want bool
	}{{1, true}, {2, true}, {3, false}} {
		_, actual, err := repo.workOrderCostUnavailable(ctx, tc.id)
		if err != nil || actual != tc.want {
			t.Errorf("work order %d unknown=%v err=%v, want %v", tc.id, actual, err, tc.want)
		}
	}
	cards, err := repo.ListJobCards(ctx, app.JobCardQuery{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 3 {
		t.Fatalf("cards: %+v", cards)
	}
	for _, card := range cards {
		want := card.ID != 33
		if card.ActualCostUnavailable != want {
			t.Errorf("card %d unknown=%v, want %v", card.ID, card.ActualCostUnavailable, want)
		}
	}
}
