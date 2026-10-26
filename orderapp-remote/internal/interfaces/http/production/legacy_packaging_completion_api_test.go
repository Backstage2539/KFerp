package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	productionapp "orderapp/internal/application/production"
	postgresproduction "orderapp/internal/infrastructure/postgres/production"
	"testing"
)

func TestLegacyPackagingCompletionUsesFrozenBagQuantityWithoutInputQuantity(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, workOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87315)

	// Legacy packaging cards store the planned bag count on the work order.
	// planned_input_qty is intentionally zero: it is not the quantity basis
	// for a packaged product operation.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.work_orders
		SET status='released',output_qty=100,output_unit='袋',inventory_unit='袋',
		    planned_inventory_qty=100,target_warehouse='finished_goods'
		WHERE id=%[2]d;
		UPDATE %[1]s.job_cards
		SET planned_input_qty=0,actual_input_qty=0,actual_output_qty=0,status='pending'
		WHERE work_order_id=%[2]d;
	`, schema, workOrderID))

	var cardID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema), workOrderID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	previewResponse := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("completion preview status=%d body=%s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview struct {
		Enabled         bool    `json:"enabled"`
		LegacyExecution bool    `json:"legacy_execution"`
		LegacyPackaging bool    `json:"legacy_packaging"`
		BlockedCode     string  `json:"blocked_code"`
		BlockedReason   string  `json:"blocked_reason"`
		PlannedQty      float64 `json:"planned_qty"`
		OutputUnit      string  `json:"output_unit"`
		Materials       []struct {
			MaterialID int64   `json:"material_id"`
			Qty        float64 `json:"qty"`
			Unit       string  `json:"unit"`
		} `json:"materials"`
	}
	if err := json.Unmarshal(previewResponse.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Enabled || !preview.LegacyExecution || !preview.LegacyPackaging || preview.BlockedCode != "" || preview.BlockedReason != "" {
		t.Fatalf("unambiguous legacy packaging should be executable: enabled=%v legacy=%v packaging=%v blocked=%q reason=%q", preview.Enabled, preview.LegacyExecution, preview.LegacyPackaging, preview.BlockedCode, preview.BlockedReason)
	}
	if preview.PlannedQty != 100 || preview.OutputUnit != "袋" {
		t.Fatalf("packaging completion quantity=%v %q, want 100 bags from work order quantity", preview.PlannedQty, preview.OutputUnit)
	}
	if len(preview.Materials) != 2 {
		t.Fatalf("packaging material uses=%+v, want the frozen bean and bag BOM items", preview.Materials)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.job_cards(work_order_id,sequence_no,operation,workstation,planned_input_qty,status) SELECT $1,sequence_no,operation,workstation,0,'pending' FROM %s.job_cards WHERE work_order_id=$1 ORDER BY id LIMIT 1`, schema, schema), workOrderID); err != nil {
		t.Fatalf("add ambiguous legacy packaging split: %v", err)
	}
	ambiguousResponse := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
	var ambiguous struct {
		Enabled     bool   `json:"enabled"`
		BlockedCode string `json:"blocked_code"`
	}
	if err := json.Unmarshal(ambiguousResponse.Body.Bytes(), &ambiguous); err != nil {
		t.Fatal(err)
	}
	if ambiguousResponse.Code != http.StatusOK || ambiguous.Enabled || ambiguous.BlockedCode != "legacy_execution_unresolved" {
		t.Fatalf("ambiguous multi-card legacy packaging should remain blocked: status=%d body=%s", ambiguousResponse.Code, ambiguousResponse.Body.String())
	}
}

func TestLegacyPackagingCompletionGroupPreviewUsesFrozenBagQuantities(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, _, sourceWorkOrderID := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 87325)
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
		fmt.Sprintf(`UPDATE %s.work_orders SET status='released',output_qty=10,output_unit='袋',inventory_unit='袋',planned_inventory_qty=10,target_warehouse='finished_goods' WHERE id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.job_cards SET planned_input_qty=0,actual_input_qty=0,actual_output_qty=0,status='pending' WHERE work_order_id=$1`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservations SET required_g=round(required_g*0.1)::bigint,required_units=round(required_units*0.1)::bigint,reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
		fmt.Sprintf(`UPDATE %s.work_order_material_reservation_batches SET reserved_g=round(reserved_g*0.1)::bigint,reserved_units=round(reserved_units*0.1)::bigint WHERE work_order_id=$1 AND status='reserved'`, schema),
	} {
		if _, err := pool.Exec(ctx, statement, sourceWorkOrderID); err != nil {
			t.Fatalf("normalize first legacy packaging work order: %v", err)
		}
	}
	for i, qty := range []float64{8, 2} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET output_qty=$2,planned_inventory_qty=$2,target_warehouse='finished_goods' WHERE id=$1`, schema), workOrderIDs[i+1], qty); err != nil {
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
	for i, cardID := range cardIDs {
		individual := serveMultilevelProductionJSON(t, app, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
		if individual.Code != http.StatusOK {
			t.Fatalf("source work order %d completion preview status=%d body=%s", i, individual.Code, individual.Body.String())
		}
		var source productionapp.OperationCompletionPreview
		if err := json.Unmarshal(individual.Body.Bytes(), &source); err != nil {
			t.Fatal(err)
		}
		wantQty := []float64{10, 8, 2}[i]
		if !source.Enabled || !source.LegacyPackaging || source.PlannedQty != wantQty || len(source.Materials) != 2 {
			t.Fatalf("source packaging preview=%+v, want enabled %v, %v bags, and two frozen materials", source, true, wantQty)
		}
	}
	response := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/workstation/completion/preview", map[string]any{"job_card_ids": cardIDs})
	if response.Code != http.StatusOK {
		t.Fatalf("group completion preview status=%d body=%s", response.Code, response.Body.String())
	}
	var preview struct {
		PlannedQty float64 `json:"planned_qty"`
		OutputUnit string  `json:"output_unit"`
		Version    string  `json:"version"`
		Items      []struct {
			JobCardID int64 `json:"job_card_id"`
		} `json:"items"`
		Materials []struct {
			Name        string  `json:"name"`
			RequiredQty float64 `json:"required_qty"`
			Unit        string  `json:"unit"`
		} `json:"materials"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.PlannedQty != 20 || preview.OutputUnit != "袋" || len(preview.Items) != 3 {
		t.Fatalf("grouped legacy packaging preview=%s, want 20 bags across 3 source work orders", response.Body.String())
	}
	if len(preview.Materials) != 2 {
		t.Fatalf("grouped materials=%+v, want frozen bean and bag components", preview.Materials)
	}
	submit := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/workstation/completion/submit", map[string]any{
		"job_card_ids": cardIDs, "actual_output_qty": 20, "output_unit": "袋", "actual_minutes": 15,
		"finish_group": true, "expected_version": preview.Version, "request_id": "legacy-pack-group-submit",
	})
	if submit.Code != http.StatusOK {
		t.Fatalf("group completion submit status=%d body=%s", submit.Code, submit.Body.String())
	}
	var completed struct {
		CompletedQty float64 `json:"completed_qty"`
		Items        []struct {
			JobCard struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"job_card"`
			Completion *struct {
				EventID int64 `json:"event_id"`
			} `json:"completion"`
		} `json:"items"`
	}
	if err := json.Unmarshal(submit.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if completed.CompletedQty != 20 || len(completed.Items) != 3 {
		t.Fatalf("group completion result=%s, want 20 bags across three source tasks", submit.Body.String())
	}
	for _, workOrderID := range workOrderIDs {
		var status string
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.work_orders WHERE id=$1`, schema), workOrderID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "completed" {
			t.Fatalf("work order %d status=%q after grouped completion, want completed", workOrderID, status)
		}
	}
	var receiptWorkOrders, receivedBags int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT e.work_order_id),COALESCE(SUM(i.qty_units),0)::bigint
		FROM %s.stock_entries e JOIN %s.stock_entry_items i ON i.stock_entry_id=e.id
		WHERE e.entry_type='finished_receipt' AND e.status='submitted' AND e.work_order_id=ANY($1)`, schema, schema), workOrderIDs).Scan(&receiptWorkOrders, &receivedBags); err != nil {
		t.Fatal(err)
	}
	if receiptWorkOrders != 3 || receivedBags != 20 {
		t.Fatalf("grouped receipt work orders=%d bags=%d, want 3 work orders and 20 bags", receiptWorkOrders, receivedBags)
	}
	var consumptionWorkOrders, consumedBeansG, consumedBags int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT e.work_order_id),
			COALESCE(SUM(i.qty_g) FILTER(WHERE i.material_id=10),0)::bigint,
			COALESCE(SUM(i.qty_units) FILTER(WHERE i.material_id=20),0)::bigint
		FROM %s.stock_entries e JOIN %s.stock_entry_items i ON i.stock_entry_id=e.id
		WHERE e.purpose='material_consumption_for_manufacture' AND e.status='submitted' AND e.work_order_id=ANY($1)`, schema, schema), workOrderIDs).Scan(&consumptionWorkOrders, &consumedBeansG, &consumedBags); err != nil {
		t.Fatal(err)
	}
	if consumptionWorkOrders != 3 || consumedBeansG != 4540 || consumedBags != 20 {
		t.Fatalf("grouped consumption work orders=%d beans=%dg bags=%d, want 3 work orders, 4540g, 20 bags", consumptionWorkOrders, consumedBeansG, consumedBags)
	}
	replay := serveMultilevelProductionJSON(t, app, http.MethodPost, "/api/production/workstation/completion/submit", map[string]any{
		"job_card_ids": cardIDs, "actual_output_qty": 20, "output_unit": "袋", "actual_minutes": 15,
		"finish_group": true, "expected_version": preview.Version, "request_id": "legacy-pack-group-submit",
	})
	if replay.Code != http.StatusOK {
		t.Fatalf("idempotent grouped completion retry status=%d body=%s", replay.Code, replay.Body.String())
	}
	assertProductionFlowCount(t, pool, schema, "production_operation_events", "kind='complete' AND status='posted'", 3)
	assertProductionFlowCount(t, pool, schema, "audit_logs", "action='complete_packaging_group'", 1)
}
