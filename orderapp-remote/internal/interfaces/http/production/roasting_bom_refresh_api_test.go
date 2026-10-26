package production

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

type roastingBOMRefreshPreview struct {
	BomRefreshPending     bool                                 `json:"bom_refresh_pending"`
	LegacyExecution       bool                                 `json:"legacy_execution"`
	Enabled               bool                                 `json:"enabled"`
	BlockedCode           string                               `json:"blocked_code"`
	BlockedReason         string                               `json:"blocked_reason"`
	OriginalBomVersionID  int64                                `json:"original_bom_version_id"`
	EffectiveBomVersionID int64                                `json:"effective_bom_version_id"`
	OriginalBomVersionNo  string                               `json:"original_bom_version_no"`
	EffectiveBomVersionNo string                               `json:"effective_bom_version_no"`
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

	route := `{"operations":[{"seq":1,"operation":"咖啡烘焙+除石","workstation":"智烘"}]}`
	frozen := `[{"material_id":10,"material_name":"在制熟豆","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"},{"material_id":50,"material_name":"已移除旧配料","unit":"kg","ratio_pct":20,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]`
	bindings := `{"material:10:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:50:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:20:0:0":{"node_key":"pack","source_warehouse":"packaging"}}`
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET bom_version_id=100,bom_variant_id=0,material_snapshot=$2::jsonb,process_snapshot_json=$3::jsonb,operation_material_bindings=$4::jsonb,planned_g=6000,planned_output_g=4994,planned_inventory_qty=4.994,output_qty=4.994,output_unit='kg',inventory_unit='kg',target_warehouse='wip',status='running' WHERE id=$1`, schema), workOrderID, frozen, route, bindings); err != nil {
		t.Fatal(err)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.job_cards SET sequence_no=1,operation='咖啡烘焙+除石',status='running',started_at=now()-interval '5 minutes',completed_at=NULL,planned_input_qty=3000,actual_input_qty=0,actual_output_qty=0 WHERE work_order_id=%[2]d;
		INSERT INTO %[1]s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES(%[2]d,1,'咖啡烘焙+除石','pending',3000);
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
	if !preview.Enabled || !preview.LegacyExecution || preview.BlockedReason != "" {
		t.Fatalf("legacy single-operation roasting order with complete frozen data should be executable: enabled=%v legacy=%v blocked=%q", preview.Enabled, preview.LegacyExecution, preview.BlockedReason)
	}
	if !preview.BomRefreshPending || preview.OriginalBomVersionID != 100 || preview.EffectiveBomVersionID != 101 || preview.RecipeRatioPct != 100 {
		t.Fatalf("preview did not select the complete published recipe: %+v", preview)
	}
	if preview.OriginalBomVersionNo != "V001" || preview.EffectiveBomVersionNo != "V002" {
		t.Fatalf("preview version labels = %q -> %q, want V001 -> V002", preview.OriginalBomVersionNo, preview.EffectiveBomVersionNo)
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
	if !preview.BomRefreshPending || preview.EffectiveBomVersionID != 102 || preview.EffectiveBomVersionNo != "V003" || preview.RecipeRatioPct != 100 {
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

func TestOperationCompletionPreviewUsesFrozenRoastingBOMWithoutReservations(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87401)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.materials(id,code,name,kind,unit,cost_unit,onhand_g,onhand_units,purchase_price,sale_price) VALUES
		 (40,'ROAST-SECOND','第二配料','bean','kg','kg',50000,0,1,0),
		 (50,'ROAST-THIRD','第三配料','bean','kg','kg',50000,0,1,0)
	`, schema)); err != nil {
		t.Fatal(err)
	}
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87402, 30, "ROAST-INGREDIENT-A", "孟连水洗A", 50000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87403, 40, "ROAST-INGREDIENT-B", "孟连水洗5T批次", 50000)
	seedProductionFlowWIPBatch(t, ctx, pool, schema, 87404, 50, "ROAST-INGREDIENT-C", "生豆-巴布亚之光", 50000)
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.material_batches SET unit_cost=100 WHERE id IN(87402,87403,87404)`, schema)); err != nil {
		t.Fatal(err)
	}

	route := `{"operations":[{"seq":1,"operation":"咖啡烘焙+除石","workstation":"智烘","execution_json":"{\"version\":1,\"node_key\":\"roast\",\"source_warehouse\":\"wip\",\"receipt_enabled\":true,\"receipt_warehouse\":\"wip\",\"output_unit\":\"kg\"}"}]}`
	frozen := `[{"material_id":30,"material_name":"孟连水洗A","unit":"kg","ratio_pct":25,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"},{"material_id":40,"material_name":"孟连水洗5T批次","unit":"kg","ratio_pct":25,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"},{"material_id":50,"material_name":"生豆-巴布亚之光","unit":"kg","ratio_pct":50,"source":"bom","component_type":"material","consume_unit":"ratio_pct","output_qty":1,"output_unit":"kg"}]`
	bindings := `{"material:30:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:40:0:0":{"node_key":"roast","source_warehouse":"wip"},"material:50:0:0":{"node_key":"roast","source_warehouse":"wip"}}`
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		DELETE FROM %[1]s.work_order_material_reservation_batches WHERE work_order_id=%[2]d;
		DELETE FROM %[1]s.work_order_material_reservations WHERE work_order_id=%[2]d;
		UPDATE %[1]s.production_bom_version_items SET ratio_pct=0,consume_unit='g' WHERE version_id=200;
		DELETE FROM %[1]s.production_bom_version_items WHERE version_id=200;
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES (200,30,'material','ratio_pct',0,25),(200,40,'material','ratio_pct',0,25),(200,50,'material','ratio_pct',0,50);
	`, schema, workOrderID))
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET bom_version_id=200,material_snapshot=$2::jsonb,process_snapshot_json=$3::jsonb,operation_material_bindings=$4::jsonb,planned_g=55000,planned_output_g=43584,planned_inventory_qty=43.584,output_qty=43.584,output_unit='kg',inventory_unit='kg',target_warehouse='wip',status='released' WHERE id=$1`, schema), workOrderID, frozen, route, bindings); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET sequence_no=1,operation='咖啡烘焙+除石',status='running',planned_input_qty=40000,actual_input_qty=0,actual_output_qty=0 WHERE work_order_id=$1`, schema), workOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,status,planned_input_qty) VALUES($1,1,'咖啡烘焙+除石','pending',15000)`, schema), workOrderID); err != nil {
		t.Fatal(err)
	}

	var cardIDs []int64
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id`, schema), workOrderID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		cardIDs = append(cardIDs, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if len(cardIDs) != 2 {
		t.Fatalf("job cards=%v, want two split roasting cards", cardIDs)
	}
	wipCoverage, err := postgresproduction.NewRepository(pool, schema).GetWorkOrderWIPCoverage(ctx, workOrderID)
	if err != nil {
		t.Fatalf("WIP coverage for complete frozen BOM: %v", err)
	}
	if !wipCoverage.DataComplete || len(wipCoverage.Materials) != 3 {
		t.Fatalf("WIP coverage must keep all three recipe identities: %+v", wipCoverage)
	}
	wantCoverageG := map[int64]int64{30: 13_750, 40: 13_750, 50: 27_500}
	for _, material := range wipCoverage.Materials {
		if material.RequiredG != wantCoverageG[material.MaterialID] || material.SourceWarehouse != "wip" {
			t.Fatalf("coverage material=%+v, want per-material demand %v from WIP", material, wantCoverageG)
		}
	}

	previews := make([]roastingBOMRefreshPreview, len(cardIDs))
	for i, wantOutput := range []float64{31.697, 11.887} {
		res := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardIDs[i]), nil)
		if res.Code != http.StatusOK {
			t.Fatalf("preview status=%d body=%s", res.Code, res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), &previews[i]); err != nil {
			t.Fatal(err)
		}
		preview := previews[i]
		if !preview.Enabled || preview.BlockedReason != "" || preview.RecipeRatioPct != 100 {
			t.Fatalf("complete frozen BOM without reservation should be executable: enabled=%v ratio=%v blocked=%q", preview.Enabled, preview.RecipeRatioPct, preview.BlockedReason)
		}
		if math.Abs(preview.PlannedQty-wantOutput) > 1e-9 {
			t.Fatalf("batch %d output reference=%v, want %v", i+1, preview.PlannedQty, wantOutput)
		}
		wantQty := []float64{10, 10, 20}
		if i == 1 {
			wantQty = []float64{3.75, 3.75, 7.5}
		}
		if len(preview.Materials) != 3 {
			t.Fatalf("batch %d materials=%+v, want the three frozen BOM rows", i+1, preview.Materials)
		}
		for j, material := range preview.Materials {
			if math.Abs(material.Qty-wantQty[j]) > 1e-9 || material.ShortageQty > 1e-9 {
				t.Fatalf("batch %d material %s demand=%v shortage=%v, want %v and no shortage", i+1, material.Name, material.Qty, material.ShortageQty, wantQty[j])
			}
		}
	}
	assertProductionFlowCount(t, pool, schema, "work_order_material_reservations", fmt.Sprintf("work_order_id=%d", workOrderID), 0)

	for i, wantOutput := range []float64{31.697, 11.887} {
		preview := previews[i]
		if i > 0 {
			res := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardIDs[i]), nil)
			if res.Code != http.StatusOK {
				t.Fatalf("batch %d refreshed preview status=%d body=%s", i+1, res.Code, res.Body.String())
			}
			if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
				t.Fatal(err)
			}
		}
		payload := map[string]any{
			"actual_input_qty": preview.InputQty, "actual_output_qty": preview.PlannedQty,
			"input_unit": preview.InputUnit, "output_unit": preview.OutputUnit,
			"expected_version": preview.Version, "request_id": fmt.Sprintf("frozen-no-reservation-batch-%d", i+1),
			"materials": preview.Materials, "finish_operation": true,
		}
		res := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", cardIDs[i]), payload)
		if res.Code != http.StatusOK {
			t.Fatalf("batch %d completion status=%d body=%s", i+1, res.Code, res.Body.String())
		}
		if math.Abs(preview.PlannedQty-wantOutput) > 1e-9 {
			t.Fatalf("batch %d completed output=%v, want %v", i+1, preview.PlannedQty, wantOutput)
		}
		if i == 0 {
			replay := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/job-cards/%d/complete", cardIDs[i]), payload)
			if replay.Code != http.StatusOK {
				t.Fatalf("batch 1 replay status=%d body=%s", replay.Code, replay.Body.String())
			}
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", fmt.Sprintf("work_order_id=%d AND kind='complete' AND status='posted'", workOrderID), 2)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND purpose='material_consumption_for_manufacture' AND status='submitted'", workOrderID), 2)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt' AND status='submitted'", workOrderID), 2)
	for materialID, wantG := range map[int64]int64{30: 13_750, 40: 13_750, 50: 27_500} {
		var actualG int64
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %s.stock_entry_items i JOIN %s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.purpose='material_consumption_for_manufacture' AND e.status='submitted' AND i.material_id=$2`, schema, schema), workOrderID, materialID).Scan(&actualG); err != nil {
			t.Fatal(err)
		}
		if actualG != wantG {
			t.Fatalf("material %d total consumption=%dg, want %dg", materialID, actualG, wantG)
		}
	}
	assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=(SELECT running_item_id FROM %s.work_orders WHERE id=%d) AND total_cost>0", schema, workOrderID), 1)
	var receivedG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(SUM(i.qty_g),0)::bigint FROM %s.stock_entry_items i JOIN %s.stock_entries e ON e.id=i.stock_entry_id WHERE e.work_order_id=$1 AND e.entry_type='finished_receipt' AND e.status='submitted'`, schema, schema), workOrderID).Scan(&receivedG); err != nil {
		t.Fatal(err)
	}
	if receivedG != 43_584 {
		t.Fatalf("finished output receipt=%dg, want 43584g", receivedG)
	}
}
