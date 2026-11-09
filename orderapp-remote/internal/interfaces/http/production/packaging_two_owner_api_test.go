package production

import (
	"context"
	"encoding/json"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestPieceDifferentOwnersCannotGroupAndCompleteIndependently(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, sourceWorkOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 88926)
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
		_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET material_consumption_policy='bom_start_v1',inventory_unit='袋',planned_inventory_qty=output_qty,operation_material_bindings='{}',process_snapshot_json='{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"receipt_warehouse\":\"finished_goods\",\"reporting_type\":\"piece\"}"}]}' WHERE id=$1`, schema), id)
		if err != nil {
			t.Fatal(err)
		}
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1,operation='包装' WHERE work_order_id=%d`, schema, id))
	}

	owner := int64(999)
	customerWO := workOrderIDs[1]
	customerCard := cardIDs[1]
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.customers(id,name) VALUES(999,'客户A');
 INSERT INTO %[1]s.warehouses(code,name,kind,customer_id,active) VALUES('customer999','客户A成品仓','customer_finished',999,true);
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,owner_customer_id,onhand_g,onhand_units,purchase_price,sale_price) VALUES(111,'OWNER-A-BEAN','客户熟豆','bean','kg','kg',999,0,0,0,0);
 INSERT INTO %[1]s.material_batches(id,batch_code,material_id,material_name,received_g,qty_g,remaining_g,unit_cost,status,quality_status,owner_customer_id) VALUES(889261,'OWNER-A-BATCH',111,'客户熟豆',1816,1816,1816,10,'active','pass',999);
 INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(889261,'OWNER-A-BATCH',111,'wip',1816);
 UPDATE %[1]s.work_orders SET customer_id=999,target_warehouse='customer999',process_snapshot_json='{"operations":[{"seq":1,"operation":"包装","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\",\"receipt_warehouse\":\"customer999\",\"reporting_type\":\"piece\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.work_orders w SET material_snapshot=(SELECT jsonb_agg(CASE WHEN (v->>'material_id')::bigint=10 THEN jsonb_set(jsonb_set(v,'{material_id}','111'::jsonb),'{material_name}','"客户熟豆"'::jsonb) ELSE v END) FROM jsonb_array_elements(w.material_snapshot) v) WHERE w.id=%[2]d;
 UPDATE %[1]s.work_order_material_reservations SET material_id=111,component_id=111,material_name='客户熟豆',source_owner_customer_id=999,source_warehouse='wip' WHERE work_order_id=%[2]d AND material_id=10;
 DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[2]d AND material_id=10;
 INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,status) SELECT id,%[2]d,111,'material',111,889261,'OWNER-A-BATCH','wip',999,1816,'reserved' FROM %[1]s.work_order_material_reservations WHERE work_order_id=%[2]d AND material_id=111;`, schema, customerWO))
	preview := func(card int64, path string) productionapp.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, app, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview: %s", res.Body.String())
		}
		var p productionapp.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, card := range []int64{cardIDs[0], customerCard} {
		p := preview(card, "start-preview")
		res := serveMultilevelProductionJSON(t, app, "POST", fmt.Sprintf("/api/job-cards/%d/start", card), map[string]any{"actual_input_qty": p.InputQty, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": fmt.Sprintf("owner-start-%d", card), "materials": p.Materials})
		if res.Code != 200 {
			t.Fatalf("owner start %d: %s p=%+v", card, res.Body.String(), p)
		}
	}
	group := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/completion/preview", map[string]any{"job_card_ids": []int64{cardIDs[0], customerCard}})
	if group.Code == 200 {
		t.Fatal("different owners grouped")
	}
	for index, card := range []int64{cardIDs[0], customerCard} {
		p := preview(card, "completion-preview")
		res := serveMultilevelProductionJSON(t, app, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.PlannedQty, "actual_output_qty": p.PlannedQty, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "actual_remainder_g": 0, "remainder_warehouse": "wip", "finish_operation": true, "expected_version": p.Version, "request_id": fmt.Sprintf("owner-complete-%d", card)})
		if res.Code != 200 {
			t.Fatalf("owner completion %d: %s p=%+v", card, res.Body.String(), p)
		}
		if index == 0 {
			var held float64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(c.remaining_qty),0)::float8 FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=$1 AND c.owner_customer_id=999 AND c.unit='g'`, schema), customerCard).Scan(&held); err != nil {
				t.Fatal(err)
			}
			if held != 1816 {
				t.Fatalf("company operation touched customer input: %v", held)
			}
		}
	}
	var actualOwner int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT owner_customer_id FROM %s.material_batches WHERE id=889261`, schema)).Scan(&actualOwner); err != nil {
		t.Fatal(err)
	}
	if actualOwner != owner {
		t.Fatalf("owner changed: %d", actualOwner)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 2)
	assertProductionFlowCount(t, pool, schema, "stock_batches", "item_type='finished_product' AND owner_customer_id=999 AND qty_units=8", 1)
}
