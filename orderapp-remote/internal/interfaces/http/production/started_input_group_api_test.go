package production

import (
	"context"
	"encoding/json"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestBOMStartGroupAtomicFailureAndReplay(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, sourceWorkOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74735)
	registerProductionWorkstationAPI(app, productionapp.NewService(postgresproduction.NewRepository(pool, schema)))

	// Model three old packaging cards whose bag quantities live on their work
	// orders while planned_input_qty is zero. The cards share the same frozen
	// product, BOM, route, materials, and target warehouse.
	workOrderIDs := []int64{sourceWorkOrderID}
	for _, entry := range []struct {
		no  string
		qty float64
	}{{"WO-LEGACY-PACK-2", 8}, {"WO-LEGACY-PACK-3", 2}} {
		var id int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_orders(
				work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,
				output_type,output_product_id,output_material_id,output_name,output_qty,output_unit,
				product_id,bom_spec_id,bom_variant_id,product_name,spec_g,inventory_unit,planned_inventory_qty,
				planned_g,planned_output_g,status,material_snapshot,bom_version_id,operation_template_id,
				process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
				production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
				sales_spec_snapshot_json,operation_material_bindings
			)
			SELECT $2,0,production_plan_id,production_plan_item_id,batch_id,
				output_type,output_product_id,output_material_id,output_name,$3,'袋',
				product_id,bom_spec_id,bom_variant_id,product_name,spec_g,'袋',$3,
				planned_g,planned_output_g,'released',material_snapshot,bom_version_id,operation_template_id,
				process_template_id,process_template_name,process_snapshot_json,operation_summary_json,
				production_config_snapshot_json,customer_product_snapshot_json,customer_id,target_warehouse,
				sales_spec_snapshot_json,operation_material_bindings
			FROM %s.work_orders WHERE id=$1 RETURNING id`, schema, schema), sourceWorkOrderID, entry.no, entry.qty).Scan(&id)
		if err != nil {
			t.Fatalf("clone packaging work order: %v", err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty,actual_input_qty,actual_output_qty,status)
			SELECT $2,sequence_no,operation,workstation,0,0,0,'pending'
			FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema, schema), sourceWorkOrderID, id); err != nil {
			t.Fatalf("clone packaging job card: %v", err)
		}
		if _, err = pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservations(
				work_order_id,running_item_id,material_id,material_name,unit,required_g,required_units,
				reserved_g,reserved_units,consumed_g,consumed_units,returned_g,returned_units,status,
				source_warehouse,source_owner_customer_id,component_type,component_id,
				component_bom_spec_id,component_bom_variant_id,component_spec_g
			)
			SELECT $2,0,material_id,material_name,unit,
				round(required_g*$3::numeric)::bigint,round(required_units*$3::numeric)::bigint,
				round(reserved_g*$3::numeric)::bigint,round(reserved_units*$3::numeric)::bigint,
				0,0,0,0,'reserved',source_warehouse,source_owner_customer_id,
				component_type,component_id,component_bom_spec_id,component_bom_variant_id,component_spec_g
			FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND status='reserved'`, schema, schema), sourceWorkOrderID, id, entry.qty/100); err != nil {
			t.Fatalf("clone packaging material reservations: %v", err)
		}
		workOrderIDs = append(workOrderIDs, id)
	}

	// The original plan reserved the whole 100-bag order. Reduce its legacy
	// reservation and frozen work-order quantity to the first 10-bag source;
	// the other two reservations above represent 8 and 2 bags.
	for _, statement := range []string{
		fmt.Sprintf(`UPDATE %s.work_orders SET status='released',output_qty=10,output_unit='袋',inventory_unit='',planned_inventory_qty=0,target_warehouse='finished_goods' WHERE id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.job_cards SET planned_input_qty=0,actual_input_qty=0,actual_output_qty=0,status='pending' WHERE work_order_id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=round(required_g*0.1)::bigint,required_units=round(required_units*0.1)::bigint,reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
	} {
		if _, err := pool.Exec(ctx, statement, sourceWorkOrderID); err != nil {
			t.Fatalf("normalize first legacy packaging work order: %v", err)
		}
	}
	for i, qty := range []float64{8, 2} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET output_qty=$2,inventory_unit='',planned_inventory_qty=0,target_warehouse='finished_goods' WHERE id=$1`, schema), workOrderIDs[i+1], qty); err != nil {
			t.Fatalf("set cloned packaging quantity: %v", err)
		}
	}
	cardIDs := make([]int64, 0, len(workOrderIDs))
	for _, id := range workOrderIDs {
		var cardID int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1`, schema), id).Scan(&cardID); err != nil {
			t.Fatalf("load grouped packaging card: %v", err)
		}
		cardIDs = append(cardIDs, cardID)
	}
	for _, id := range workOrderIDs {
		_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',inventory_unit='袋',planned_inventory_qty=output_qty,operation_material_bindings='{}',process_snapshot_json='{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"receipt_warehouse\":\"finished_goods\"}"}]}' WHERE id=$1`, schema), id)
		if err != nil {
			t.Fatal(err)
		}
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1,operation='包装' WHERE work_order_id=%d`, schema, id))
	}
	response := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/preview", map[string]any{"job_card_ids": cardIDs})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var p productionapp.OperationStartGroupPreview
	if err := json.Unmarshal(response.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	items := []map[string]any{}
	for _, task := range p.Tasks {
		items = append(items, map[string]any{"id": task.JobCardID, "actual_input_qty": task.InputQty, "input_unit": task.InputUnit, "expected_version": task.Version, "materials": task.Materials})
	}
	items[1]["actual_input_qty"] = 0
	request := map[string]any{"items": items, "request_id": "group-start-747", "expected_version": p.Version}
	failed := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/submit", request)
	if failed.Code == 200 {
		t.Fatal("invalid second task committed")
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", "true", 0)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='start'", 0)
	for i, task := range p.Tasks {
		items[i]["actual_input_qty"] = task.InputQty
	}
	for n := 0; n < 2; n++ {
		response = serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/submit", request)
		if response.Code != 200 {
			t.Fatalf("start %d: %s", n, response.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", "status='posted'", 3)
 finishedPreview:=serveMultilevelProductionJSON(t,app,"POST","/api/production/workstation/completion/preview",map[string]any{"job_card_ids":cardIDs})
 if finishedPreview.Code!=200{t.Fatal(finishedPreview.Body.String())}
 var cp productionapp.OperationCompletionGroupPreview
 if err:=json.Unmarshal(finishedPreview.Body.Bytes(),&cp);err!=nil{t.Fatal(err)}
 missing:=serveMultilevelProductionJSON(t,app,"POST","/api/production/workstation/completion/submit",map[string]any{"job_card_ids":cardIDs,"actual_output_qty":cp.RemainingQty,"output_unit":cp.OutputUnit,"finish_group":true,"expected_version":cp.Version,"request_id":"missing-processed-inputs"})
 if missing.Code==200{t.Fatal("new group completion silently inferred actual processed input")}
 inputByCard:=map[int64]float64{};for _,task:=range p.Tasks{inputByCard[task.JobCardID]=task.InputQty}
 done:=serveMultilevelProductionJSON(t,app,"POST","/api/production/workstation/completion/submit",map[string]any{"job_card_ids":cardIDs,"actual_output_qty":cp.RemainingQty,"output_unit":cp.OutputUnit,"processed_inputs":inputByCard,"finish_group":true,"expected_version":cp.Version,"request_id":"group-start-finish"})
 if done.Code!=200{t.Fatalf("group complete after consumption: %s",done.Body.String())}
 assertProductionFlowCount(t,pool,schema,"production_started_inputs","remaining_qty=0 AND remaining_cost=0",3)
 assertProductionFlowCount(t,pool,schema,"production_operation_events","kind='complete' AND material_cost<>0",0)

}
