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

// PR-600: a cut-over product is ordered, stocked and manufactured as
// parent product + BOM specification. The product specification is already an
// inventory unit, so 100 bags remain 100 bags while the selected variant BOM
// consumes 22.7 kg roasted beans and 100 packaging bags.
func TestProductionPlanUsesBomSpecIdentityWithoutSalesUnitConversion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if err := postgresproductspecmigration.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("product spec migration EnsureSchema: %v", err)
	}
	seedMultilevelMaterialOutputFlow(t, ctx, pool, schema)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.product_unit_definitions(code,name,unit_type,allow_decimal,active)
		VALUES('袋','袋','package',false,true)
		ON CONFLICT (code) DO UPDATE SET active=true,deleted_at=NULL;

		DELETE FROM %s.production_bom_version_items WHERE version_id=100;
		UPDATE %s.production_bom_versions
		SET output_qty=1,output_unit='袋',process_route_id=0,material_loss_rate=0
		WHERE id=100;

		INSERT INTO %s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit,created_by,updated_by)
		VALUES
			(10001,100,'BSPEC-227','bag-227','227g袋','袋','test','test'),
			(10002,100,'BSPEC-454','bag-454','454g袋','袋','test','test');
		INSERT INTO %s.production_bom_version_variants(
			id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order,material_loss_rate,process_route_id
		) VALUES
			(10101,100,10001,'227g袋','袋',true,10,0,31),
			(10102,100,10002,'454g袋','袋',false,20,0,31);
		INSERT INTO %s.production_bom_version_items(
			version_id,variant_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct,material_loss_rate
		) VALUES
			(100,10101,10,'material','kg',0.227,0,0),
			(100,10101,20,'material','unit',1,0,0),
			(100,10102,10,'material','kg',0.454,0,0),
			(100,10102,20,'material','unit',2,0,0);

		UPDATE %s.order_items
		SET product_id=1,bom_spec_id=10001,bom_variant_id=10101,
		    item_name='227g袋',spec='227g袋',unit='袋',sales_unit='袋',qty=100,
		    price_source_json=jsonb_build_object(
		      'bom_spec_id',10001,'bom_variant_id',10101,
		      'production_quantity_snapshot',jsonb_build_object(
		        'parent_product_id',1,'bom_spec_id',10001,'bom_variant_id',10101,
		        'spec_label','227g袋','sales_unit','袋','inventory_unit','袋',
		        'inventory_qty_per_sales_unit',1,'conversion_source','bom_spec_identity'
		      )
		    )
		WHERE order_id=1;
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,readiness_json,cutover_at,cutover_by)
		VALUES(1,'cutover','{}'::jsonb,now(),'test')
		ON CONFLICT(product_id) DO UPDATE SET state='cutover',cutover_at=now(),cutover_by='test';
	`, schema, schema, schema, schema, schema, schema, schema, schema))
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.materials SET onhand_g=22700 WHERE id=10;
	`, schema))
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 61_010, 10, "MB-BSPEC-ROASTED", "在制熟豆", 22_700)
	seedProductionFlowWIPUnitBatch(t, ctx, pool, schema, 61_020, 20, "MB-BSPEC-BAG", "227g包装袋", 100)

	app := newProductionFlowTestEcho(pool, schema)
	preview := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/unproduced?from=2026-08-01&to=2026-08-31", nil)
	if preview.Code != http.StatusOK {
		t.Fatalf("GET unproduced status=%d body=%s", preview.Code, preview.Body.String())
	}
	for _, marker := range []string{
		`"product_id":1`, `"bom_spec_id":10001`, `"bom_variant_id":10101`,
		`"spec_label":"227g袋"`, `"inventory_unit":"袋"`,
		`"need_inventory_qty":100`, `"gap_inventory_qty":100`,
		`"selection_key":"product:1:bom_spec:10001"`,
	} {
		if !strings.Contains(preview.Body.String(), marker) {
			t.Fatalf("unproduced body missing %s: %s", marker, preview.Body.String())
		}
	}

	create := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production-plans", map[string]any{
		"from":     "2026-08-01",
		"to":       "2026-08-31",
		"selected": []string{"product:1:bom_spec:10001"},
	})
	if create.Code != http.StatusOK {
		t.Fatalf("POST production plan status=%d body=%s", create.Code, create.Body.String())
	}

	var planItemID int64
	var outputQty float64
	var outputUnit, snapshot string
	var bomSpecID, bomVariantID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,bom_spec_id,bom_variant_id,output_qty::float8,output_unit,component_snapshot_json::text
		FROM %s.production_plan_items
		WHERE output_type='product' AND output_product_id=1
		ORDER BY id DESC LIMIT 1
	`, schema)).Scan(&planItemID, &bomSpecID, &bomVariantID, &outputQty, &outputUnit, &snapshot); err != nil {
		t.Fatalf("load product specification plan item: %v", err)
	}
	if bomSpecID != 10001 || bomVariantID != 10101 || outputQty != 100 || outputUnit != "袋" {
		t.Fatalf("plan identity=(%d,%d) output=%v%s, want (10001,10101) 100袋", bomSpecID, bomVariantID, outputQty, outputUnit)
	}
	var frozen []struct {
		MaterialID int64   `json:"material_id"`
		QtyPerUnit float64 `json:"qty_per_unit"`
	}
	if err := json.Unmarshal([]byte(snapshot), &frozen); err != nil {
		t.Fatalf("decode selected variant component snapshot: %v (%s)", err, snapshot)
	}
	if len(frozen) != 2 || frozen[0].MaterialID != 10 || frozen[0].QtyPerUnit != 0.227 ||
		frozen[1].MaterialID != 20 || frozen[1].QtyPerUnit != 1 {
		t.Fatalf("selected variant component snapshot=%+v raw=%s", frozen, snapshot)
	}
	assertProductionFlowCount(t, pool, schema, "production_plan_items", fmt.Sprintf("id=%d AND planned_inventory_qty=100 AND inventory_unit='袋' AND planned_output_g=0", planItemID), 1)

	var planID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT production_plan_id FROM %s.production_plan_items WHERE id=$1`, schema), planItemID).Scan(&planID); err != nil {
		t.Fatalf("load BOM specification plan id: %v", err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		INSERT INTO %s.production_plan_operation_splits(
			production_plan_id,production_plan_item_id,operation_seq,operation,
			batch_size_qty,batch_size_unit,standard_minutes,planned_batch_count,
			planned_qty,planned_qty_g,planned_minutes
		) VALUES(%d,%d,1,'包装',100,'袋',15,1,100,0,15);
	`, schema, planID, planItemID))

	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/production-plans/%d/submit", planID), nil)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit BOM specification plan status=%d body=%s", submit.Code, submit.Body.String())
	}
	var workOrderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.work_orders
		WHERE production_plan_id=$1 AND production_plan_item_id=$2
	`, schema), planID, planItemID).Scan(&workOrderID); err != nil {
		t.Fatalf("load BOM specification work order: %v", err)
	}
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf(
		"id=%d AND product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101 AND output_qty=100 AND output_unit='袋' AND planned_inventory_qty=100", workOrderID,
	), 1)
	workOrderList := serveMultilevelProductionJSON(t, app, http.MethodGet, "/api/produce/work-orders?limit=20", nil)
	if workOrderList.Code != http.StatusOK ||
		!strings.Contains(workOrderList.Body.String(), `"bom_spec_id":10001`) ||
		!strings.Contains(workOrderList.Body.String(), `"bom_variant_id":10101`) {
		t.Fatalf("work-order list does not expose BOM specification identity status=%d body=%s", workOrderList.Code, workOrderList.Body.String())
	}

	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start BOM specification work order status=%d body=%s", start.Code, start.Body.String())
	}
	var runningItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT running_item_id FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&runningItemID); err != nil {
		t.Fatalf("load BOM specification running item: %v", err)
	}
	var runningProductID, runningSpecID, runningVariantID, runningPlannedUnits int64
	var runningOutputQty float64
	var runningOutputUnit string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT product_id,bom_spec_id,bom_variant_id,planned_units,output_qty::float8,output_unit
		FROM %s.produce_running_items WHERE id=$1
	`, schema), runningItemID).Scan(&runningProductID, &runningSpecID, &runningVariantID, &runningPlannedUnits, &runningOutputQty, &runningOutputUnit); err != nil {
		t.Fatalf("load BOM specification running identity: %v", err)
	}
	if runningProductID != 1 || runningSpecID != 10001 || runningVariantID != 10101 || runningPlannedUnits != 100 || runningOutputQty != 100 || runningOutputUnit != "袋" {
		t.Fatalf("running identity=(%d,%d,%d) plan=%d output=%v%s", runningProductID, runningSpecID, runningVariantID, runningPlannedUnits, runningOutputQty, runningOutputUnit)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %s.job_cards
		SET status='completed',started_at=COALESCE(started_at,now()),completed_at=now(),
		    actual_input_qty=100,actual_output_qty=100
		WHERE work_order_id=%d;
	`, schema, workOrderID))
	complete := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), map[string]any{
		"finished_units":   100,
		"finished_loose_g": 0,
		"warehouse":        "finished_goods",
		"note":             "BOM规格身份生产闭环",
	})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete BOM specification work order status=%d body=%s", complete.Code, complete.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101 AND spec_g=0 AND warehouse='finished_goods' AND onhand_units=100 AND onhand_loose_g=0", 1)
	assertProductionFlowCount(t, pool, schema, "stock_batches", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101 AND qty_units=100 AND remaining_units=100", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_ledger_entries", fmt.Sprintf("source_doc_type='production_run' AND source_doc_id=%d AND item_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101 AND qty_change_g=0 AND qty_change_units=100", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", "product_id=1 AND bom_spec_id=10001 AND bom_variant_id=10101 AND qty_g=0 AND qty_units=100", 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=10 AND deduct_g=22700", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=20 AND deduct_units=100", runningItemID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='complete'", workOrderID), 1)
}
