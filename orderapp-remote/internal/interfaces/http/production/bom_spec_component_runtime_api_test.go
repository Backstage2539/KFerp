package production

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	postgresproduction "orderapp/internal/infrastructure/postgres/production"
)

func TestProductionPlanBOMSpecComponentUsesOnlySelectedSpecStockAndCreatesSpecUpstream(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %[1]s.products(
			id,name,sku_name,sku_code,product_kind,active,unit_rule_override_json
		) VALUES(2,'PR600上游父商品','PR600上游父商品','PR600-UPSTREAM','roasted_bean',true,'{"inventory_unit":"袋"}'::jsonb);

		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		UPDATE %[1]s.production_bom_versions
		SET output_qty=1,output_unit='袋',process_route_id=31,material_loss_rate=0 WHERE id=100;
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES
			(17001,100,'BSP-ROOT-100','root-100','根商品100袋','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,process_route_id
		) VALUES(17101,100,17001,'根商品100袋','袋',true,1,31);

		INSERT INTO %[1]s.production_boms(id,code,name,output_type,output_product_id,status)
		VALUES(300,'PBOM-UPSTREAM-SPECS','上游规格组BOM','product',2,'active');
		INSERT INTO %[1]s.production_bom_versions(
			id,bom_id,version_no,status,output_qty,output_unit,process_route_id,published_at
		) VALUES(300,300,'V001','published',1,'袋',32,now());
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES
			(18001,300,'BSP-UP-A','up-a','上游A袋','袋'),
			(18002,300,'BSP-UP-B','up-b','上游B袋','袋');
		INSERT INTO %[1]s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,process_route_id
		) VALUES
			(18101,300,18001,'上游A袋','袋',true,1,32),
			(18102,300,18002,'上游B袋','袋',false,2,32);
		INSERT INTO %[1]s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct
		) VALUES
			(300,18101,20,'material','unit',1,0),
			(300,18102,30,'material','kg',0.001,0);
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default,updated_by)
		VALUES('product',2,300,300,true,'test');

		INSERT INTO %[1]s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,component_product_id,
			component_bom_spec_id,component_spec_g,consume_unit,qty_per_unit,ratio_pct
		) VALUES(100,17101,0,'finished_product',2,18001,0,'unit_per_bag',1,0);

		UPDATE %[1]s.order_items
		SET product_id=1,bom_spec_id=17001,bom_variant_id=17101,item_name='根商品100袋',
		    spec='根商品100袋',unit='袋',sales_unit='袋',qty=100,
		    price_source_json='{"production_quantity_snapshot":{"parent_product_id":1,"bom_spec_id":17001,"bom_variant_id":17101,"spec_label":"根商品100袋","sales_unit":"袋","inventory_unit":"袋","inventory_qty_per_sales_unit":1,"conversion_source":"bom_spec_identity"}}'::jsonb
		WHERE order_id=1;

		INSERT INTO %[1]s.finished_inventory(product_id,bom_spec_id,bom_variant_id,spec_g,warehouse,onhand_units,onhand_loose_g) VALUES
			(2,18001,18101,0,'finished_goods',40,0),
			(2,18002,18102,0,'finished_goods',100,0);
		INSERT INTO %[1]s.stock_batches(
			batch_code,item_type,item_id,item_name,bom_spec_id,bom_variant_id,spec_g,
			source_doc_type,source_doc_id,qty_g,qty_units,remaining_g,remaining_units,quality_status
		) VALUES
			('UP-A-STOCK','finished_product',2,'上游A袋',18001,18101,0,'seed',1,0,40,0,40,'passed'),
			('UP-B-STOCK','finished_product',2,'上游B袋',18002,18102,0,'seed',2,0,100,0,100,'passed');
	`, schema))
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 62_020, 20, "MB-UP-A-BAG", "227g包装袋", 60)

	app := newProductionFlowTestEcho(pool, schema)
	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from": "2026-08-01", "to": "2026-08-31",
		"selected": []string{"product:1:bom_spec:17001"},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("create BOM-spec component plan status=%d body=%s", create.Code, create.Body.String())
	}
	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.production_plans ORDER BY id DESC LIMIT 1`, schema)).Scan(&planID); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_type='product' AND output_product_id=2 AND bom_spec_id=18001 AND bom_variant_id=18101 AND output_qty=60 AND output_unit='袋'", planID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf(
		"production_plan_id=%d AND output_product_id=2 AND bom_spec_id=18002", planID,
	), 0)
	assertProductionFlowCount(t, pool, schema, "production_plan_item_dependencies", fmt.Sprintf(
		"production_plan_id=%d AND component_type='product' AND component_id=2 AND component_bom_spec_id=18001 AND required_units=60", planID,
	), 1)

	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		)
		SELECT production_plan_id,id,1,
		       CASE WHEN output_product_id=2 THEN '烘焙' ELSE '包装' END,
		       planned_inventory_qty,inventory_unit,15,1,
		       planned_inventory_qty,planned_g,15
		FROM %s.production_plan_items WHERE production_plan_id=%d;
	`, schema, schema, planID))
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit BOM-spec component plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "work_order_dependencies",
		"component_type='product' AND component_id=2 AND component_bom_spec_id=18001 AND component_bom_variant_id=18101 AND required_units=60", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations",
		"component_type='product' AND component_id=2 AND component_bom_spec_id=18001 AND component_bom_variant_id=18101 AND required_units=100 AND reserved_units=40", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches",
		"component_type='product' AND component_id=2 AND component_bom_spec_id=18001 AND component_bom_variant_id=18101 AND reserved_units=40", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations",
		"component_type='product' AND component_id=2 AND component_bom_spec_id=18002", 0)

	var rootWorkOrderID, upstreamWorkOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_product_id=1 AND bom_spec_id=17001
	`, schema), planID).Scan(&rootWorkOrderID); err != nil {
		t.Fatalf("load BOM-spec downstream work order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND output_product_id=2 AND bom_spec_id=18001
	`, schema), planID).Scan(&upstreamWorkOrderID); err != nil {
		t.Fatalf("load BOM-spec upstream work order: %v", err)
	}
	blocked := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("start BOM-spec downstream before upstream status=%d body=%s, want dependency rejection", blocked.Code, blocked.Body.String())
	}

	upstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", upstreamWorkOrderID), nil)
	if upstreamStart.Code != http.StatusOK {
		t.Fatalf("start BOM-spec upstream status=%d body=%s", upstreamStart.Code, upstreamStart.Body.String())
	}
	var upstreamRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), upstreamWorkOrderID).Scan(&upstreamRunningItemID); err != nil {
		t.Fatalf("load BOM-spec upstream running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=60,actual_output_qty=60
		WHERE work_order_id=%d;
	`, schema, upstreamWorkOrderID))
	upstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", upstreamWorkOrderID), map[string]any{
		"finished_units": 60,
		"warehouse":      "finished_goods",
		"note":           "produce only upstream BOM spec A",
	})
	if upstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete BOM-spec upstream status=%d body=%s", upstreamComplete.Code, upstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory",
		"product_id=2 AND bom_spec_id=18001 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory",
		"product_id=2 AND bom_spec_id=18002 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND component_bom_spec_id=18001 AND required_units=100 AND reserved_units=100", rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservation_batches", fmt.Sprintf(
		"work_order_id=%d AND component_bom_spec_id=18001 AND reserved_units>0", rootWorkOrderID,
	), 2)

	downstreamStart := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", rootWorkOrderID), nil)
	if downstreamStart.Code != http.StatusOK {
		t.Fatalf("start BOM-spec downstream status=%d body=%s", downstreamStart.Code, downstreamStart.Body.String())
	}
	var downstreamRunningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), rootWorkOrderID).Scan(&downstreamRunningItemID); err != nil {
		t.Fatalf("load BOM-spec downstream running item: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=100,actual_output_qty=100
		WHERE work_order_id=%d;
	`, schema, rootWorkOrderID))
	downstreamComplete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", rootWorkOrderID), map[string]any{
		"finished_units": 100,
		"warehouse":      "finished_goods",
		"note":           "consume only upstream BOM spec A",
	})
	if downstreamComplete.Code != http.StatusOK {
		t.Fatalf("complete BOM-spec downstream status=%d body=%s", downstreamComplete.Code, downstreamComplete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory",
		"product_id=2 AND bom_spec_id=18001 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=0", 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory",
		"product_id=2 AND bom_spec_id=18002 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf(
		"work_order_id=%d AND component_bom_spec_id=18001 AND consumed_units=100", rootWorkOrderID,
	), 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches",
		"item_type='finished_product' AND item_id=2 AND bom_spec_id=18001 AND remaining_units=0", 2)
	assertProductionFlowCount(t, pool, schema, "stock_batches",
		"item_type='finished_product' AND item_id=2 AND bom_spec_id=18002 AND remaining_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf(
		"source_doc_type='production_run' AND source_doc_id=%d AND item_id=2 AND bom_spec_id=18001 AND qty_change_units<0", downstreamRunningItemID,
	), 2)
}

func TestProductionBOMSpecReservationSchemaMigrationIsIdempotent(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	for attempt := 0; attempt < 2; attempt++ {
		if err := postgresproduction.EnsureSchema(ctx, pool, schema); err != nil {
			t.Fatalf("production EnsureSchema repeat %d: %v", attempt+1, err)
		}
	}
	for indexName, marker := range map[string]string{
		"production_plan_item_dependencies_typed_uq":       "component_bom_spec_id",
		"work_order_dependencies_typed_uq":                 "component_bom_spec_id",
		"work_order_material_reservation_batches_typed_uq": "component_bom_spec_id",
		"work_order_material_reservations_component_idx":   "component_bom_spec_id",
	} {
		var definition string
		if err := pool.QueryRow(ctx, `
			SELECT indexdef FROM pg_indexes
			WHERE schemaname=$1 AND indexname=$2
		`, schema, indexName).Scan(&definition); err != nil {
			t.Fatalf("load migrated index %s: %v", indexName, err)
		}
		if !strings.Contains(definition, marker) {
			t.Fatalf("migrated index %s=%s, want %s identity", indexName, definition, marker)
		}
	}
}
