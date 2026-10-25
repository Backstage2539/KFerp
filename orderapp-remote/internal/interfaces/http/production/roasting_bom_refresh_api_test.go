package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	productionapp "orderapp/internal/application/production"
	"testing"
)

type roastingBOMRefreshPreview struct {
	BomRefreshPending     bool                                 `json:"bom_refresh_pending"`
	OriginalBomVersionID  int64                                `json:"original_bom_version_id"`
	EffectiveBomVersionID int64                                `json:"effective_bom_version_id"`
	RecipeRatioPct        float64                              `json:"recipe_ratio_pct"`
	Version               string                               `json:"version"`
	InputQty              float64                              `json:"input_qty"`
	PlannedQty            float64                              `json:"planned_qty"`
	InputUnit             string                               `json:"input_unit"`
	OutputUnit            string                               `json:"output_unit"`
	Materials             []productionapp.OperationMaterialUse `json:"materials"`
}

func TestOperationCompletionRefreshesPublishedRoastingBOMAtomically(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87301)
	start := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/start", workOrderID), nil)
	if start.Code != http.StatusOK {
		t.Fatalf("start work order status=%d body=%s", start.Code, start.Body.String())
	}
	seedMaterial := func(id int64, code, name string) {
		t.Helper()
		if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES($1,$2,$3,'bean','kg','kg',5000,0,1,0)`, schema), id, code, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET name=CASE id WHEN 10 THEN '孟连水洗A' WHEN 30 THEN '孟连水洗5T' ELSE name END WHERE id IN(10,30)`, schema)); err != nil {
		t.Fatal(err)
	}
	seedMaterial(40, "ROAST-REFRESH-NEW-B", "巴布亚之光")
	seedMaterial(50, "ROAST-REFRESH-OLD", "已移除旧配料")
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87302, 30, "ROAST-REFRESH-BATCH-A", "孟连水洗5T", 5_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87303, 40, "ROAST-REFRESH-BATCH-B", "巴布亚之光", 5_000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87304, 50, "ROAST-REFRESH-BATCH-OLD", "已移除旧配料", 5_000)

	// V001 is a frozen, incomplete 70% recipe. V002 is the newer complete
	// published version of the same BOM, with a new component identity.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=100;
		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=100;
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,ratio_pct)
		VALUES (100,10,'material',50),(100,50,'material',20);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (101,100,'V002','published',1,0,1,'kg',32,now()+interval '1 second');
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,ratio_pct)
		VALUES (101,10,'material',70),(101,30,'material',15),(101,40,'material',15);
	`, schema))

	var originalReservationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.work_order_material_reservations WHERE work_order_id=$1 AND material_id=10 AND status='reserved'`, schema), workOrderID).Scan(&originalReservationID); err != nil {
		t.Fatalf("find original BOM reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=1500,reserved_g=1500,source_warehouse='wip',source_owner_customer_id=0 WHERE id=$1`, schema), originalReservationID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=1500,reserved_units=0,warehouse='wip',owner_customer_id=0 WHERE reservation_id=$1`, schema), originalReservationID); err != nil {
		t.Fatal(err)
	}
	var staleReservationID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_order_material_reservations(work_order_id,running_item_id,material_id,material_name,unit,required_g,reserved_g,component_type,component_id,source_warehouse,source_owner_customer_id,status)
		SELECT $1,running_item_id,50,'已移除旧配料','kg',600,600,'material',50,'wip',0,'reserved'
		FROM %s.work_orders WHERE id=$1 RETURNING id`, schema, schema), workOrderID).Scan(&staleReservationID); err != nil {
		t.Fatalf("seed stale reservation: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservation_batches(reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,batch_code,warehouse,owner_customer_id,reserved_g,status) VALUES($1,$2,50,'material',50,87304,'ROAST-REFRESH-BATCH-OLD','wip',0,600,'reserved')`, schema), staleReservationID, workOrderID); err != nil {
		t.Fatal(err)
	}

	route := `{"operations":[{"seq":1,"operation":"咖啡烘焙+除石","workstation":"智烘","execution_json":"{\"version\":1,\"node_key\":\"roast\",\"source_warehouse\":\"wip\",\"receipt_enabled\":false,\"output_unit\":\"kg\"}"},{"seq":2,"operation":"包装","workstation":"包装台","execution_json":"{\"version\":1,\"node_key\":\"pack\",\"source_warehouse\":\"wip\",\"receipt_enabled\":true,\"receipt_warehouse\":\"finished_goods\",\"output_unit\":\"袋\"}"}]}`
	frozen := `[{"material_id":10,"material_name":"在制熟豆","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"},{"material_id":50,"material_name":"已移除旧配料","unit":"kg","ratio_pct":20,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]`
	bindings := `{"material:10:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:50:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:20:0:0":{"node_key":"pack","source_warehouse":"packaging"}}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET bom_version_id=100,bom_variant_id=0,material_snapshot=$2::jsonb,process_snapshot_json=$3::jsonb,operation_material_bindings=$4::jsonb,planned_g=6000,planned_output_g=4994,planned_inventory_qty=4.994,output_qty=4.994,output_unit='kg',inventory_unit='kg',target_warehouse='wip',status='running' WHERE id=$1`, schema), workOrderID, frozen, route, bindings); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.job_cards SET sequence_no=1,operation='咖啡烘焙+除石',status='running',started_at=now()-interval '5 minutes',completed_at=NULL,planned_input_qty=3000,actual_input_qty=0,actual_output_qty=0 WHERE work_order_id=%[2]d;
		INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'咖啡烘焙+除石','pending',3000),(%[2]d,2,'包装','pending',6000);
		UPDATE %[1]s.work_order_material_reservations SET required_g=1500,reserved_g=1500 WHERE id=%[3]d;
		UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=1500 WHERE reservation_id=%[3]d;
	`, schema, workOrderID, originalReservationID))

	var firstCardID, secondCardID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=1 ORDER BY id LIMIT 1`, schema), workOrderID).Scan(&firstCardID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 AND sequence_no=1 ORDER BY id DESC LIMIT 1`, schema), workOrderID).Scan(&secondCardID); err != nil {
		t.Fatal(err)
	}
	readPreview := func(cardID int64) roastingBOMRefreshPreview {
		t.Helper()
		res := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
		if res.Code != http.StatusOK {
			t.Fatalf("preview status=%d body=%s", res.Code, res.Body.String())
		}
		var preview roastingBOMRefreshPreview
		if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		return preview
	}

	var frozenVersion int64
	var frozenBefore string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_version_id,material_snapshot::text FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&frozenVersion, &frozenBefore); err != nil {
		t.Fatal(err)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='refresh_roasting_bom_before_consumption'", workOrderID), 0)
	preview := readPreview(firstCardID)
	if !preview.BomRefreshPending || preview.OriginalBomVersionID != 100 || preview.EffectiveBomVersionID != 101 || preview.RecipeRatioPct != 100 {
		t.Fatalf("preview did not select the complete published recipe: %+v", preview)
	}
	wantQty := map[int64]float64{10: 2.1, 30: 0.45, 40: 0.45}
	if len(preview.Materials) != len(wantQty) {
		t.Fatalf("effective materials = %+v", preview.Materials)
	}
	for _, material := range preview.Materials {
		want, exists := wantQty[material.MaterialID]
		if !exists || math.Abs(material.Qty-want) > 1e-9 || material.ShortageQty > 1e-9 {
			t.Fatalf("effective material = %+v, want quantities %v and no shortages", material, wantQty)
		}
	}
	var afterPreviewVersion int64
	var afterPreviewSnapshot string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_version_id,material_snapshot::text FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&afterPreviewVersion, &afterPreviewSnapshot); err != nil {
		t.Fatal(err)
	}
	if afterPreviewVersion != frozenVersion || afterPreviewSnapshot != frozenBefore {
		t.Fatalf("preview wrote the frozen BOM: version %d -> %d", frozenVersion, afterPreviewVersion)
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='refresh_roasting_bom_before_consumption'", workOrderID), 0)

	// Publishing a newer complete version after preview makes that preview
	// stale. The first submission must not write any stock or audit rows; a
	// refreshed preview can then complete against the new published version.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=101;
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (102,100,'V003','published',1,0,1,'kg',32,now()+interval '2 seconds');
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,ratio_pct)
		VALUES (102,10,'material',70),(102,30,'material',15),(102,40,'material',15);
	`, schema))

	payload := map[string]any{
		"actual_input_qty": preview.InputQty, "actual_output_qty": preview.PlannedQty,
		"input_unit": preview.InputUnit, "output_unit": preview.OutputUnit,
		"expected_version": preview.Version, "request_id": "roast-bom-refresh-first-post",
		"materials": preview.Materials, "finish_operation": true,
	}
	stale := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", firstCardID), payload)
	if stale.Code == http.StatusOK {
		t.Fatalf("stale BOM preview unexpectedly completed: %s", stale.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 0)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND purpose='material_consumption_for_manufacture' AND status='submitted'", workOrderID), 0)
	preview = readPreview(firstCardID)
	if !preview.BomRefreshPending || preview.EffectiveBomVersionID != 102 || preview.RecipeRatioPct != 100 {
		t.Fatalf("refreshed preview did not select the newly published complete recipe: %+v", preview)
	}
	payload["expected_version"] = preview.Version
	payload["materials"] = preview.Materials
	for attempt := 0; attempt < 2; attempt++ {
		res := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", firstCardID), payload)
		if res.Code != http.StatusOK {
			t.Fatalf("complete/replay status=%d body=%s", res.Code, res.Body.String())
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND purpose='material_consumption_for_manufacture' AND status='submitted'", workOrderID), 1)
	assertProductionFlowCount(t, pool, schema, "audit_logs", fmt.Sprintf("entity_type='work_order' AND entity_id=%d AND action='refresh_roasting_bom_before_consumption'", workOrderID), 1)
	var refreshedVersion int64
	var staleStatus string
	var staleReturned int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bom_version_id FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&refreshedVersion); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status,returned_g FROM %s.work_order_material_reservations WHERE id=$1`, schema), staleReservationID).Scan(&staleStatus, &staleReturned); err != nil {
		t.Fatal(err)
	}
	if refreshedVersion != 102 || staleStatus != "released" || staleReturned != 600 {
		t.Fatalf("effective freeze/stale reservation = version %d, reservation %s returned %d", refreshedVersion, staleStatus, staleReturned)
	}
	for materialID, wantG := range map[int64]int64{10: 2_100, 30: 450, 40: 450} {
		var actualG int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %s.stock_entry_items i JOIN %s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.purpose='material_consumption_for_manufacture' AND e.status='submitted' AND i.material_id=$2`, schema, schema), workOrderID, materialID).Scan(&actualG); err != nil {
			t.Fatal(err)
		}
		if actualG != wantG {
			t.Fatalf("material %d posted consumption = %dg, want %dg", materialID, actualG, wantG)
		}
	}
	for materialID, wantRemaining := range map[int64]int64{10: 22_700 - 2_100, 30: 5_000 - 450, 40: 5_000 - 450} {
		batchID := int64(87301)
		switch materialID {
		case 30:
			batchID = 87302
		case 40:
			batchID = 87303
		}
		var remaining int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT qty_g FROM %s.material_batch_locations WHERE material_batch_id=$1 AND warehouse='wip'`, schema), batchID).Scan(&remaining); err != nil {
			t.Fatalf("read material %d WIP location: %v", materialID, err)
		}
		if remaining != wantRemaining {
			t.Fatalf("material %d WIP balance = %d, want %d", materialID, remaining, wantRemaining)
		}
	}
	second := readPreview(secondCardID)
	if second.BomRefreshPending || second.OriginalBomVersionID != 102 || second.EffectiveBomVersionID != 102 || second.RecipeRatioPct != 100 {
		t.Fatalf("later batch must keep the first posted BOM version: %+v", second)
	}
	if len(second.Materials) != 3 {
		t.Fatalf("later batch effective materials = %+v", second.Materials)
	}
}
