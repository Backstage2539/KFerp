package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	postgresmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
	"testing"
)

func TestCrossSpecGroupStartDistinctFrozenPackagingMaterials(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, sourceWorkOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 88916)
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
	// Two real frozen BOM specification identities and different packaging
	// materials share exactly the same semi-finished lot in WIP.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_units,purchase_price,sale_price) VALUES(40,'BAG-454','454g包装袋','pack','unit','unit',100,2,0);
 INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(501,100,'GROUP-227','group-227','227g袋装','袋'),(502,100,'GROUP-454','group-454','454g袋装','袋');
 INSERT INTO %[1]s.customers(id,name,active) VALUES(455,'客户甲',true),(456,'客户乙',true);
 INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,process_route_id) VALUES(601,100,501,'227g袋装','袋',31),(602,100,502,'454g袋装','袋',31);
 INSERT INTO %[1]s.production_bom_version_items(version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit) VALUES(100,601,10,'material','g_per_bag',227),(100,601,20,'material','unit_per_bag',1),(100,602,10,'material','g_per_bag',454),(100,602,40,'material','unit_per_bag',1);
 UPDATE %[1]s.work_orders SET bom_spec_id=501,bom_variant_id=601,customer_id=455,sales_spec_snapshot_json='{"spec_label":"227g袋装"}' WHERE id IN (%[2]d,%[4]d);
 UPDATE %[1]s.work_orders SET bom_spec_id=502,bom_variant_id=602,customer_id=456,spec_g=454,sales_spec_snapshot_json='{"spec_label":"454g袋装"}',material_snapshot='[{"unit":"kg","source":"bom","output_qty":1,"material_id":10,"output_unit":"kg","consume_unit":"g_per_bag","qty_per_unit":454,"material_name":"在制熟豆","component_type":"material","loss_calculation_mode":"yield_denominator"},{"unit":"unit","source":"bom","output_qty":1,"material_id":40,"output_unit":"kg","consume_unit":"unit_per_bag","qty_per_unit":1,"material_name":"454g包装袋","component_type":"material","loss_calculation_mode":"yield_denominator"}]'::jsonb WHERE id=%[3]d;
 UPDATE %[1]s.work_order_material_reservations SET required_g=3632,reserved_g=3632 WHERE work_order_id=%[3]d AND material_id=10;
 UPDATE %[1]s.work_order_material_reservations SET material_id=40,component_id=40,material_name='454g包装袋' WHERE work_order_id=%[3]d AND material_id=20;
 `, schema, workOrderIDs[0], workOrderIDs[1], workOrderIDs[2]))
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 88919, 40, "MB-BAG-454", "454g包装袋", 100)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=CASE id WHEN 88916 THEN 80 WHEN 88917 THEN 1 WHEN 88919 THEN 2 ELSE unit_cost END WHERE id IN (88916,88917,88919)`, schema))
	if err := postgresmigration.EnsureAuthorityView(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]int64{{cardIDs[0]}, {cardIDs[0], cardIDs[0]}} {
		rejected := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/preview", map[string]any{"job_card_ids": invalid})
		if rejected.Code == 200 {
			t.Fatal("invalid selection accepted")
		}
	}
	var originalStation string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT workstation FROM %s.job_cards WHERE id=$1`, schema), cardIDs[1]).Scan(&originalStation); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET workstation='另一工位' WHERE id=$1`, schema), cardIDs[1]); err != nil {
		t.Fatal(err)
	}
	rejected := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/preview", map[string]any{"job_card_ids": cardIDs})
	if rejected.Code == 200 {
		t.Fatal("cross-station selection accepted")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET workstation=$2 WHERE id=$1`, schema), cardIDs[1], originalStation); err != nil {
		t.Fatal(err)
	}
	response := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/preview", map[string]any{"job_card_ids": cardIDs})
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var p productionapp.OperationStartGroupPreview
	if err := json.Unmarshal(response.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	stale := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/start/submit", map[string]any{"request_id": "stale-group", "expected_version": "outdated", "items": []map[string]any{{"id": cardIDs[0]}, {"id": cardIDs[1]}}})
	if stale.Code == 200 {
		t.Fatal("stale group version accepted")
	}
	assertProductionFlowCount(t, pool, schema, "production_started_inputs", "true", 0)
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
	for _, check := range []struct {
		batch            int64
		wantG, wantUnits int64
	}{{88916, 22700, 0}, {88917, 0, 100}, {88919, 0, 100}} {
		var g, units int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0)::bigint,COALESCE(SUM(qty_units),0)::bigint FROM %s.material_batch_locations WHERE material_batch_id=$1`, schema), check.batch).Scan(&g, &units)
		if err != nil || g != check.wantG || units != check.wantUnits {
			t.Fatalf("failed group changed lot %d: %dg/%d units, err=%v", check.batch, g, units, err)
		}
	}
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
	for _, expected := range []struct {
		card      int64
		beanG     float64
		bagID     int64
		bagQty    float64
		totalCost float64
	}{
		{cardIDs[0], 2270, 20, 10, 191.6}, {cardIDs[1], 3632, 40, 8, 306.56}, {cardIDs[2], 454, 20, 2, 38.32},
	} {
		var beanQty, bagQty, cost float64
		var beanBatch int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(c.qty) FILTER (WHERE c.component_id=10),0)::float8,COALESCE(SUM(c.qty) FILTER (WHERE c.component_id=$2),0)::float8,COALESCE(SUM(c.cost),0)::float8,COALESCE(MIN(c.material_batch_id) FILTER (WHERE c.component_id=10),0) FROM %[1]s.production_started_input_components c JOIN %[1]s.production_started_inputs i ON i.id=c.input_id WHERE i.job_card_id=$1`, schema), expected.card, expected.bagID).Scan(&beanQty, &bagQty, &cost, &beanBatch)
		if err != nil || beanQty != expected.beanG || bagQty != expected.bagQty || beanBatch != 88916 || math.Abs(cost-expected.totalCost) > 0.00001 {
			t.Fatalf("card %d: bean=%v bag=%v batch=%d cost=%v want %+v error=%v", expected.card, beanQty, bagQty, beanBatch, cost, expected, err)
		}
	}
	for _, task := range p.Tasks {
		var qty float64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT input_qty::float8 FROM %s.production_started_inputs WHERE job_card_id=$1 AND status='posted'`, schema), task.JobCardID).Scan(&qty); err != nil {
			t.Fatal(err)
		}
		if qty != task.InputQty {
			t.Fatalf("task %d input = %v want %v", task.JobCardID, qty, task.InputQty)
		}
	}
	// Different specifications must still not be merged into one output receipt.
	finished := serveMultilevelProductionJSON(t, app, "POST", "/api/production/workstation/completion/preview", map[string]any{"job_card_ids": cardIDs})
	if finished.Code == 200 {
		t.Fatal("cross-spec start must not loosen receipt compatibility")
	}
}
