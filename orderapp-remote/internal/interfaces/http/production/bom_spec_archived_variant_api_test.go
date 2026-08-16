package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	postgresproductspecmigration "orderapp/internal/infrastructure/postgres/productspecmigration"
)

func TestProductionPlanKeepsFrozenArchivedBOMVariantAfterSameBOMVersionPublish(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("product spec migration EnsureSchema: %v", err)
	}
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES('袋','袋','package',false,true)
		ON CONFLICT (code) DO UPDATE SET active=true,deleted_at=NULL;

		INSERT INTO %[1]s.process_routes(id,name,status,default_equipment,default_minutes)
		VALUES(33,'新版包装路线','active','新版包装机',20);
		INSERT INTO %[1]s.process_route_operations(route_id,seq,operation,workstation,default_equipment,default_minutes,records_loss)
		VALUES(33,1,'新版包装','新版包装台','新版包装机',20,false);

		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		UPDATE %[1]s.production_bom_versions
		SET output_qty=1,output_unit='袋',process_route_id=0,material_loss_rate=0
		WHERE id=100;
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit,created_by,updated_by)
		VALUES(10001,100,'BSPEC-HISTORY-227','bag-227','227g袋','袋','test','test');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
		) VALUES(10101,100,10001,'227g袋 V1','袋',true,10,0,31);
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate
		) VALUES
			(100,10101,10,'material','kg',0.227,0,0),
			(100,10101,20,'material','unit',1,0,0);

		UPDATE %[1]s.order_items
		SET product_id=1,bom_spec_id=10001,bom_variant_id=10101,
		    item_name='227g袋 V1',spec='227g袋 V1',unit='袋',sales_unit='袋',qty=100,
		    price_source_json=jsonb_build_object(
		      'bom_spec_id',10001,'bom_variant_id',10101,
		      'production_quantity_snapshot',jsonb_build_object(
		        'parent_product_id',1,'bom_spec_id',10001,'bom_variant_id',10101,
		        'spec_label','227g袋 V1','sales_unit','袋','inventory_unit','袋',
		        'inventory_qty_per_sales_unit',1,'conversion_source','bom_spec_identity'
		      )
		    )
		WHERE order_id=1;

		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=100;
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at
		) VALUES(101,100,'V002','published',1,0,1,'袋',0,now());
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
		) VALUES(10201,101,10001,'227g袋 V2','袋',true,10,0,33);
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate
		) VALUES
			(101,10201,10,'material','kg',0.300,0,0),
			(101,10201,20,'material','unit',2,0,0);
		UPDATE %[1]s.product_production_bom_bindings SET bom_version_id=101 WHERE product_id=1;
		UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=101 WHERE output_type='product' AND output_id=1;
		UPDATE %[1]s.product_production_configs
		SET production_bom_version_id=0,process_route_id=0 WHERE product_id=1;

		INSERT INTO %[1]s.orders(id,order_no,order_date,is_void,process_status_id)
		VALUES(2,'SO-ML-227-V2','2026-08-11',false,(SELECT id FROM %[1]s.order_process_statuses WHERE name='待处理' LIMIT 1));
		INSERT INTO %[1]s.order_items(
			order_id,line_no,item_name,qty,unit,spec,product_id,unit_price,line_total,
			bom_spec_id,bom_variant_id,sales_unit,price_source_json
		) VALUES(
			2,1,'227g袋 V2',100,'袋','227g袋 V2',1,50,5000,
			10001,10201,'袋',jsonb_build_object(
			  'bom_spec_id',10001,'bom_variant_id',10201,
			  'production_quantity_snapshot',jsonb_build_object(
			    'parent_product_id',1,'bom_spec_id',10001,'bom_variant_id',10201,
			    'spec_label','227g袋 V2','sales_unit','袋','inventory_unit','袋',
			    'inventory_qty_per_sales_unit',1,'conversion_source','bom_spec_identity'
			  )
			)
		);
		INSERT INTO %[1]s.product_bom_spec_migrations(product_id,state,readiness_json,cutover_at,cutover_by)
		VALUES(1,'cutover','{}'::jsonb,now(),'test')
		ON CONFLICT(product_id) DO UPDATE SET state='cutover',cutover_at=now(),cutover_by='test';
		UPDATE %[1]s.materials SET onhand_g=60000 WHERE id=10;
		UPDATE %[1]s.materials SET onhand_units=500 WHERE id=20;
	`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 62_010, 10, "MB-BSPEC-HISTORY-ROASTED", "在制熟豆", 60_000)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 62_020, 20, "MB-BSPEC-HISTORY-BAG", "包装袋", 500)

	app := newProductionFlowTestEcho(pool, schema)
	preview := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if preview.Code != http.StatusOK {
		t.Fatalf("GET unproduced status=%d body=%s", preview.Code, preview.Body.String())
	}
	for _, marker := range []string{`"bom_variant_id":10101`, `"bom_variant_id":10201`, `"selection_key":"product:1:bom_spec:10001"`} {
		if !strings.Contains(preview.Body.String(), marker) {
			t.Fatalf("unproduced body missing %s: %s", marker, preview.Body.String())
		}
	}

	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"product:1:bom_spec:10001"},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("POST production plan status=%d body=%s", create.Code, create.Body.String())
	}

	type plannedVersion struct {
		itemID, versionID, variantID, routeID int64
		componentJSON, routeJSON              string
	}
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id,bom_version_id,bom_variant_id,process_route_id,
		       component_snapshot_json::text,process_route_snapshot_json::text
		FROM %s.production_plan_items
		WHERE output_type='product' AND output_product_id=1 AND bom_spec_id=10001
		ORDER BY bom_version_id
	`, schema))
	if err != nil {
		t.Fatal(err)
	}
	planned := make([]plannedVersion, 0, 2)
	for rows.Next() {
		var row plannedVersion
		if err := rows.Scan(&row.itemID, &row.versionID, &row.variantID, &row.routeID, &row.componentJSON, &row.routeJSON); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		planned = append(planned, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if len(planned) != 2 {
		t.Fatalf("planned BOM specification versions=%d, want V1 and V2", len(planned))
	}
	if planned[0].versionID != 100 || planned[0].variantID != 10101 || planned[0].routeID != 31 ||
		planned[1].versionID != 101 || planned[1].variantID != 10201 || planned[1].routeID != 33 {
		t.Fatalf("planned frozen identities=%+v", planned)
	}
	assertFrozenComponentQty := func(raw string, wantBeanQty, wantBagQty float64) {
		t.Helper()
		var items []struct {
			MaterialID int64   `json:"material_id"`
			QtyPerUnit float64 `json:"qty_per_unit"`
		}
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			t.Fatalf("decode frozen components: %v (%s)", err, raw)
		}
		if len(items) != 2 || items[0].MaterialID != 10 || items[0].QtyPerUnit != wantBeanQty ||
			items[1].MaterialID != 20 || items[1].QtyPerUnit != wantBagQty {
			t.Fatalf("frozen components=%+v, want bean=%v bag=%v", items, wantBeanQty, wantBagQty)
		}
	}
	assertFrozenComponentQty(planned[0].componentJSON, 0.227, 1)
	assertFrozenComponentQty(planned[1].componentJSON, 0.300, 2)
	if !strings.Contains(planned[0].routeJSON, `"route_id": 31`) || !strings.Contains(planned[1].routeJSON, `"route_id": 33`) {
		t.Fatalf("route snapshots do not preserve V1/V2: %s / %s", planned[0].routeJSON, planned[1].routeJSON)
	}

	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT production_plan_id FROM %s.production_plan_items WHERE id=$1`, schema), planned[0].itemID).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		) VALUES
			(%[2]d,%[3]d,1,'包装',100,'袋',15,1,100,0,15),
			(%[2]d,%[4]d,1,'新版包装',100,'袋',20,1,100,0,20);
	`, schema, planID, planned[0].itemID, planned[1].itemID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit production plan status=%d body=%s", submit.Code, submit.Body.String())
	}

	type workOrderVersion struct {
		id, versionID, variantID  int64
		materialJSON, processJSON string
	}
	workRows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id,bom_version_id,bom_variant_id,material_snapshot::text,process_snapshot_json::text
		FROM %s.work_orders
		WHERE production_plan_id=$1 AND bom_spec_id=10001
		ORDER BY bom_version_id
	`, schema), planID)
	if err != nil {
		t.Fatal(err)
	}
	workOrders := make([]workOrderVersion, 0, 2)
	for workRows.Next() {
		var row workOrderVersion
		if err := workRows.Scan(&row.id, &row.versionID, &row.variantID, &row.materialJSON, &row.processJSON); err != nil {
			workRows.Close()
			t.Fatal(err)
		}
		workOrders = append(workOrders, row)
	}
	if err := workRows.Err(); err != nil {
		workRows.Close()
		t.Fatal(err)
	}
	workRows.Close()
	if len(workOrders) != 2 || workOrders[0].versionID != 100 || workOrders[0].variantID != 10101 ||
		workOrders[1].versionID != 101 || workOrders[1].variantID != 10201 {
		t.Fatalf("work orders did not preserve V1/V2 identities: %+v", workOrders)
	}
	assertFrozenComponentQty(workOrders[0].materialJSON, 0.227, 1)
	assertFrozenComponentQty(workOrders[1].materialJSON, 0.300, 2)
	if !strings.Contains(workOrders[0].processJSON, `"route_id": 31`) || !strings.Contains(workOrders[1].processJSON, `"route_id": 33`) {
		t.Fatalf("work order routes do not preserve V1/V2: %s / %s", workOrders[0].processJSON, workOrders[1].processJSON)
	}

	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrders[0].id), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start archived V1 work order status=%d body=%s", start.Code, start.Body.String())
	}
	var runningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), workOrders[0].id).Scan(&runningItemID); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=100,actual_output_qty=100
		WHERE work_order_id=%d;
	`, schema, workOrders[0].id))
	complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrders[0].id), map[string]any{
		"finished_units": 100, "finished_loose_g": 0, "warehouse": "finished_goods", "note": "归档 V1 冻结规格完工",
	})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete archived V1 work order status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10201 AND warehouse='finished_goods' AND onhand_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "produce_running_items", fmt.Sprintf("id=%d AND product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "production_logs", fmt.Sprintf("running_item_id=%d AND product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", fmt.Sprintf("stock_entry_id IN (SELECT id FROM %s.stock_entries WHERE running_item_id=%d) AND product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101", schema, runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=10 AND deduct_g=22700", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=20 AND deduct_units=100", runningItemID), 1)

	startV2 := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrders[1].id), nil)
	if startV2.Code != http.StatusOK {
		t.Fatalf("start current V2 work order status=%d body=%s", startV2.Code, startV2.Body.String())
	}
	var runningItemV2ID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), workOrders[1].id).Scan(&runningItemV2ID); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),actual_input_qty=100,actual_output_qty=100
		WHERE work_order_id=%d;
	`, schema, workOrders[1].id))
	completeV2 := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrders[1].id), map[string]any{
		"finished_units": 100, "finished_loose_g": 0, "warehouse": "finished_goods", "note": "当前 V2 冻结规格完工",
	})
	if completeV2.Code != http.StatusOK {
		t.Fatalf("complete current V2 work order status=%d body=%s", completeV2.Code, completeV2.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10201 AND warehouse='finished_goods' AND onhand_units=200", 1)
	assertProductionFlowCount(t, pool, schema, "production_logs", fmt.Sprintf("running_item_id=%d AND product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10201", runningItemV2ID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=10001 AND bom_variant_id=10201", runningItemV2ID), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=10 AND deduct_g=30000", runningItemV2ID), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=20 AND deduct_units=200", runningItemV2ID), 1)
}
