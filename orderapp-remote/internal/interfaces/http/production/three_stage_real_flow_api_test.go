package production

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	app "orderapp/internal/application/production"
	productionrepo "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestThreeStageRealFlowMaterialThenIndependentPacking(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, packing, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 88930)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 889301, 30, "FULL-GREEN", "生豆", 100000)
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=100000 WHERE id=30;
 UPDATE %[1]s.materials SET onhand_g=0 WHERE id=10;
 UPDATE %[1]s.material_batches SET qty_g=0,remaining_g=0 WHERE material_id=10;
 UPDATE %[1]s.material_batch_locations SET qty_g=0 WHERE material_id=10;
 UPDATE %[1]s.stock_batches SET qty_g=0,remaining_g=0 WHERE item_type='material' AND item_id=10;`, schema))
	var run, wo int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.produce_running_items(batch_id,product_id,product_name,spec_g,need_g,status,started_by,started_at,output_type,output_material_id,output_name,output_qty,output_unit,target_warehouse) VALUES('FULL-ROAST',0,'熟豆',0,83000,'running','test',now(),'material',10,'熟豆',83,'kg','wip') RETURNING id`, schema)).Scan(&run); err != nil {
		t.Fatal(err)
	}
	route := `{"operations":[{"seq":1,"operation":"roast","execution_json":"{\"version\":1,\"node_key\":\"roast\",\"reporting_type\":\"batch\",\"output_unit\":\"kg\"}"},{"seq":2,"operation":"sort","execution_json":"{\"version\":1,\"node_key\":\"sort\",\"reporting_type\":\"incoming\",\"receipt_enabled\":true,\"receipt_warehouse\":\"wip\",\"output_unit\":\"kg\"}"}]}`
	snapshot := `[{"material_id":30,"material_name":"生豆","component_type":"material","unit":"kg","consume_unit":"kg","qty_per_unit":100,"output_qty":83,"output_unit":"kg"}]`
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.work_orders(work_order_no,running_item_id,batch_id,product_name,inventory_unit,planned_inventory_qty,planned_g,planned_output_g,status,material_snapshot,process_snapshot_json,material_consumption_policy,output_type,output_material_id,output_name,output_qty,output_unit,target_warehouse) VALUES('FULL-ROAST-WO',$1,'FULL-ROAST','熟豆','kg',83,100000,83000,'running',$2::jsonb,$3::jsonb,'bom_start_v1','material',10,'熟豆',83,'kg','wip') RETURNING id`, schema), run, snapshot, route).Scan(&wo); err != nil {
		t.Fatal(err)
	}
	var reservation int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservations(work_order_id,running_item_id,material_id,material_name,unit,component_type,component_id,source_warehouse,required_g,reserved_g,status) VALUES($1,$2,30,'生豆','kg','material',30,'wip',100000,100000,'reserved') RETURNING id`, schema), wo, run).Scan(&reservation); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,reserved_g,status) VALUES(%[2]d,%[3]d,30,'material',30,889301,'FULL-GREEN','wip',100000,'reserved'); INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,workstation,status,planned_input_qty) VALUES(%[3]d,1,'roast','炉','pending',100000),(%[3]d,2,'sort','色选','pending',84000);`, schema, reservation, wo))
	var roast, sort int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id),max(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), wo).Scan(&roast, &sort); err != nil {
		t.Fatal(err)
	}
	registerProductionWorkstationAPI(e, app.NewService(productionrepo.NewRepository(pool, schema)))
	checkOverviewType := func(card int64, want string) {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", "/api/production/workstation-overview?scope=all", nil)
		if res.Code != 200 {
			t.Fatalf("overview: %s", res.Body.String())
		}
		var overview app.ProductionWorkstationOverview
		if err := json.Unmarshal(res.Body.Bytes(), &overview); err != nil {
			t.Fatal(err)
		}
		for _, task := range overview.Tasks {
			if task.JobCardID == card {
				if task.ReportingType != want {
					t.Fatalf("task %d type=%q want %q", card, task.ReportingType, want)
				}
				return
			}
		}
		t.Fatalf("task %d missing from overview", card)
	}
	checkOverviewType(roast, "batch")
	checkOverviewType(sort, "incoming")
	preview := func(card int64, path string) app.OperationCompletionPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/%s", card, path), nil)
		if res.Code != 200 {
			t.Fatalf("preview %d: %s", res.Code, res.Body.String())
		}
		var p app.OperationCompletionPreview
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := preview(roast, "start-preview")
	t.Logf("roast preview %+v", p)
	start := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", roast), map[string]any{"actual_input_qty": 100, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "full-roast-start", "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("roast start: %s", start.Body.String())
	}
	p = preview(roast, "completion-preview")
	finish := true
	result := serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", roast), map[string]any{"actual_input_qty": 100, "actual_output_qty": 84, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "full-roast-finish", "finish_operation": finish, "difference_reason": "烘焙实际产出"})
	if result.Code != 200 {
		t.Fatalf("roast finish: %s", result.Body.String())
	}
	p = preview(sort, "start-preview")
	start = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", sort), map[string]any{"actual_input_qty": 84, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "full-sort-start", "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("sort start: %s", start.Body.String())
	}
	p = preview(sort, "completion-preview")
	result = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", sort), map[string]any{"actual_input_qty": 84, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "waste_qty": 1, "difference_reason": "色选废料", "expected_version": p.Version, "request_id": "full-sort-finish", "finish_operation": true})
	if result.Code != 200 {
		t.Fatalf("sort finish: %s", result.Body.String())
	}
	var produced int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(qty_g),0) FROM %s.material_batch_locations WHERE material_id=10 AND warehouse='wip'`, schema)).Scan(&produced); err != nil {
		t.Fatal(err)
	}
	if produced != 83000 {
		t.Fatalf("semi output = %d", produced)
	}
	var packCard, beanBatch int64
	var beanCode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT min(id) FROM %s.job_cards WHERE work_order_id=$1`, schema), packing).Scan(&packCard); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT b.id,b.batch_code FROM %[1]s.material_batches b JOIN %[1]s.material_batch_locations l ON l.material_batch_id=b.id WHERE b.material_id=10 AND l.warehouse='wip' AND l.qty_g=83000 ORDER BY b.id DESC LIMIT 1`, schema)).Scan(&beanBatch, &beanCode); err != nil {
		t.Fatal(err)
	}
	packRoute := `{"operations":[{"seq":1,"operation":"generic-pack","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"reporting_type\":\"piece\",\"receipt_enabled\":true,\"receipt_warehouse\":\"finished_goods\",\"output_unit\":\"袋\"}"}]}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.work_orders SET material_consumption_policy='bom_start_v1',process_snapshot_json=$2::jsonb,operation_material_bindings='{}',output_qty=180,output_unit='袋',inventory_unit='袋',planned_inventory_qty=180,planned_g=81720,planned_output_g=81720,spec_g=454 WHERE id=$1;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,operation='generic-pack',planned_input_qty=81720,actual_input_qty=0,actual_output_qty=0 WHERE work_order_id=$1;
 UPDATE %[1]s.work_order_material_reservations SET required_g=81720,reserved_g=81720,consumed_g=0,returned_g=0,status='reserved',source_warehouse='wip' WHERE work_order_id=$1 AND material_id=10;
 UPDATE %[1]s.work_order_material_reservations SET required_units=180,reserved_units=180,consumed_units=0,returned_units=0,status='reserved',source_warehouse='wip' WHERE work_order_id=$1 AND material_id=20;
 UPDATE %[1]s.work_order_material_reservation_batches SET material_batch_id=$3,batch_code=$4,reserved_g=81720,consumed_g=0,returned_g=0,status='reserved',warehouse='wip' WHERE work_order_id=$1 AND material_id=10;
 UPDATE %[1]s.work_order_material_reservation_batches SET reserved_units=180,consumed_units=0,returned_units=0,status='reserved',warehouse='wip' WHERE work_order_id=$1 AND material_id=20;
 UPDATE %[1]s.materials SET onhand_units=180 WHERE id=20;
 UPDATE %[1]s.material_batches SET qty_units=180,remaining_units=180 WHERE material_id=20;
 UPDATE %[1]s.material_batch_locations SET qty_units=180 WHERE material_id=20 AND warehouse='wip';
 UPDATE %[1]s.stock_batches SET qty_units=180,remaining_units=180 WHERE item_type='material' AND item_id=20;`, schema), pgx.QueryExecModeSimpleProtocol, packing, packRoute, beanBatch, beanCode); err != nil {
		t.Fatal(err)
	}
	p = preview(packCard, "start-preview")
	checkOverviewType(packCard, "piece")
	start = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/start", packCard), map[string]any{"actual_input_qty": 180, "input_unit": p.InputUnit, "expected_version": p.Version, "request_id": "full-pack-start", "materials": p.Materials})
	if start.Code != 200 {
		t.Fatalf("pack start: %s; preview=%+v", start.Body.String(), p)
	}
	p = preview(packCard, "completion-preview")
	if p.PackagingAvailableG != 81720 || p.PackagingStandardGPerPiece != 454 {
		t.Fatalf("unexpected packaging %v %v", p.PackagingAvailableG, p.PackagingStandardGPerPiece)
	}
	var candidate app.PackagingUnissuedRemainder
	for _, row := range p.PackagingUnissuedCandidates {
		if row.BatchCode == beanCode {
			candidate = row
		}
	}
	if candidate.AvailableG != 1280 {
		t.Fatalf("expected 1280 shared grams, got %+v", p.PackagingUnissuedCandidates)
	}
	command := map[string]any{"actual_input_qty": 180, "actual_output_qty": 180, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "expected_version": p.Version, "request_id": "full-pack-finish", "finish_operation": true, "actual_remainder_g": 0, "remainder_warehouse": "wip", "selected_unissued_remainders": []map[string]any{{"batch_code": beanCode, "warehouse": "wip", "owner_customer_id": candidate.OwnerCustomerID, "quantity_g": 1280}}}
	for i := 0; i < 2; i++ {
		result = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", packCard), command)
		if result.Code != 200 {
			t.Fatalf("pack finish: %s", result.Body.String())
		}
	}
	var remain int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='wip'`, schema), beanBatch).Scan(&remain); err != nil {
		t.Fatal(err)
	}
	if remain != 1280 {
		t.Fatalf("remaining=%d", remain)
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "request_id='full-pack-finish' AND kind='complete'", 1)
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "request_id='full-roast-finish' AND input_qty=100 AND output_qty=84 AND status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "job_cards", fmt.Sprintf("id=%d AND actual_input_qty=84 AND actual_output_qty=83 AND actual_loss_qty=1", sort), 1)
	assertProductionFlowCount(t, pool, schema, "stock_entry_items", fmt.Sprintf("stock_entry_id IN (SELECT id FROM %s.stock_entries WHERE source_type='operation_completion' AND work_order_id=%d) AND qty_units=180 AND qty_g=81720", schema, packing), 1)

}
