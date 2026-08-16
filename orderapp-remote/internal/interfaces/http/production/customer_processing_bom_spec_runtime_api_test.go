package production

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"
)

func TestCustomerProcessingBOMSpecDemandFreezesOneToOneIdentityIntoPlan(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		ALTER TABLE %[1]s.customer_processing_production_demands
			ADD COLUMN IF NOT EXISTS request_id BIGINT NOT NULL DEFAULT 0;
	`, schema))
	if err := postgrescustomerportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customer portal EnsureSchema: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %[1]s.order_items;
		DELETE FROM %[1]s.orders;
		INSERT INTO %[1]s.customers(id,name,active) VALUES(6001,'PR600加工客户',true);
		INSERT INTO %[1]s.warehouses(code,name,kind,active,customer_id)
		VALUES('PR600-CUSTOMER-FINISHED','PR600客户成品仓','customer_processing',true,6001);

		UPDATE %[1]s.products
		SET name='PR600加工父商品',sku_name='PR600加工父商品',sku_code='PR600-PARENT',
		    unit_rule_override_json='{"inventory_unit":"袋"}'::jsonb
		WHERE id=1;
		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		UPDATE %[1]s.production_bom_versions
		SET output_qty=1,output_unit='袋',process_route_id=31,material_loss_rate=0
		WHERE id=100;
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
		VALUES(16001,100,'BSP-PROCESS-227','bag-227','227g袋','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,
			material_loss_rate,process_route_id
		) VALUES(16101,100,16001,'227g袋','袋',true,1,0,31);
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct
		) VALUES
			(100,16101,10,'material','kg',0.227,0),
			(100,16101,20,'material','unit',1,0);

		INSERT INTO %[1]s.processing_job_requests(
			id,customer_id,request_no,target_product_id,target_spec_g,target_qty,status
		) VALUES(60001,6001,'CP-PR600-SPEC',1,0,2,'submitted');
		INSERT INTO %[1]s.processing_job_request_items(
			id,request_id,line_no,product_id,parent_product_id,bom_spec_id,bom_variant_id,
			product_name,spec_name,inventory_unit,spec_g,target_qty,need_g,target_warehouse,
			bom_version_id,bom_version_no,bom_source_product_id,bom_inherited,material_snapshot_json,status
		) VALUES(
			60011,60001,1,1,1,16001,16101,'PR600加工父商品','227g袋','袋',0,2,0,
			'PR600-CUSTOMER-FINISHED',100,'V001',1,false,
			'[{"material_id":10,"material_name":"在制熟豆","unit":"kg","source":"bom","component_type":"material","consume_unit":"kg","qty_per_unit":0.227,"output_qty":1,"output_unit":"袋"},{"material_id":20,"material_name":"227g包装袋","unit":"unit","source":"bom","component_type":"material","consume_unit":"unit","qty_per_unit":1,"output_qty":1,"output_unit":"袋"}]'::jsonb,
			'submitted'
		);
		INSERT INTO %[1]s.customer_processing_production_demands(
			request_id,request_item_id,request_no,customer_id,product_id,bom_spec_id,bom_variant_id,
			product_name,spec_name,inventory_unit,spec_g,target_qty,need_g,target_warehouse,status
		) VALUES(
			60001,60011,'CP-PR600-SPEC',6001,1,16001,16101,
			'PR600加工父商品','227g袋','袋',0,2,0,'PR600-CUSTOMER-FINISHED','planned'
		);
	`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 63_010, 10, "MB-PROCESS-ROASTED", "在制熟豆", 454)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 63_020, 20, "MB-PROCESS-BAG", "227g包装袋", 2)

	app := newProductionFlowTestEcho(pool, schema)
	preview := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if preview.Code != http.StatusOK {
		t.Fatalf("GET customer processing BOM spec demand status=%d body=%s", preview.Code, preview.Body.String())
	}
	for _, marker := range []string{
		`"product_id":1`, `"bom_spec_id":16001`, `"bom_variant_id":16101`,
		`"selection_key":"product:1:bom_spec:16001"`, `"inventory_unit":"袋"`,
		`"need_inventory_qty":2`, `"gap_inventory_qty":2`,
	} {
		if !strings.Contains(preview.Body.String(), marker) {
			t.Fatalf("customer processing preview missing %s: %s", marker, preview.Body.String())
		}
	}

	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"product:1:bom_spec:16001"},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create customer processing BOM spec plan status=%d body=%s", create.Code, create.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items",
		"product_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND spec_g=0 AND inventory_unit='袋' AND planned_inventory_qty=2 AND target_warehouse='PR600-CUSTOMER-FINISHED' AND processing_request_item_id=60011", 1)

	var planID, planItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT production_plan_id,id FROM %s.production_plan_items
		WHERE processing_request_item_id=60011
	`, schema)).Scan(&planID, &planItemID); err != nil {
		t.Fatalf("load customer processing BOM spec plan: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		) VALUES(%d,%d,1,'包装',2,'袋',15,1,2,0,15);
	`, schema, planID, planItemID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit customer processing BOM spec plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	var workOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders WHERE production_plan_id=$1 AND production_plan_item_id=$2
	`, schema), planID, planItemID).Scan(&workOrderID); err != nil {
		t.Fatalf("load customer processing BOM spec work order: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf(
		"id=%d AND product_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND spec_g=0 AND output_qty=2 AND output_unit='袋' AND target_warehouse='PR600-CUSTOMER-FINISHED' AND processing_request_item_id=60011", workOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "customer_processing_production_demands", fmt.Sprintf(
		"request_item_id=60011 AND linked_work_order_id=%d", workOrderID,
	), 1)

	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start customer processing BOM spec work order status=%d body=%s", start.Code, start.Body.String())
	}
	var runningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&runningItemID); err != nil {
		t.Fatalf("load customer processing BOM spec running item: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "produce_running_items", fmt.Sprintf(
		"id=%d AND product_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND spec_g=0 AND planned_units=2 AND output_qty=2 AND output_unit='袋' AND target_warehouse='PR600-CUSTOMER-FINISHED'", runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "customer_processing_production_demands", fmt.Sprintf(
		"request_item_id=60011 AND linked_work_order_id=%d AND linked_running_item_id=%d AND status='running'", workOrderID, runningItemID,
	), 1)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=2,actual_output_qty=2
		WHERE work_order_id=%d;
	`, schema, workOrderID))
	complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), map[string]any{
		"finished_units": 2,
		"warehouse":      "PR600-CUSTOMER-FINISHED",
		"note":           "customer processing BOM spec completion",
	})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete customer processing BOM spec work order status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory",
		"product_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND spec_g=0 AND warehouse='PR600-CUSTOMER-FINISHED' AND onhand_units=2", 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf(
		"source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND qty_units=2 AND remaining_units=2", runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf(
		"source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=16001 AND bom_variant_id=16101 AND warehouse='PR600-CUSTOMER-FINISHED' AND qty_change_units=2", runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf(
		"running_item_id=%d AND material_id=10 AND deduct_g=454", runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf(
		"running_item_id=%d AND material_id=20 AND deduct_units=2", runningItemID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "customer_processing_production_demands",
		"request_item_id=60011 AND status='done'", 1)
	assertProductionFlowCount(t, pool, schema, "processing_job_request_items",
		"id=60011 AND status='completed'", 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf(
		"entity_type='work_order' AND entity_id=%d AND action='complete'", workOrderID,
	), 1)
}
