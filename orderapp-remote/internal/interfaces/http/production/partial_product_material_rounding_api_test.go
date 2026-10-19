package production

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestProductPartialReceiptsKeepFrozenMaterialWithinWholeRunRounding(t *testing.T) {
	t.Run("exact frozen stock", func(t *testing.T) { testPartialProductMaterialRounding(t, false) })
	t.Run("real shortage remains blocked", func(t *testing.T) { testPartialProductMaterialRounding(t, true) })
}

func testPartialProductMaterialRounding(t *testing.T, shortage bool) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	app, workOrderID, runningID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 19030)
	// Model an existing running BOM-spec order with the exact frozen recipe
	// encountered in DEV: 24 bags, 0.28375 kg material per bag, 6811g reserved.
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
	INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
	VALUES(19031,100,'PR703-454','pr703-454','454g袋','袋');
	INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default)
	VALUES(19032,100,19031,'454g袋','袋',true);
	UPDATE %[1]s.produce_running_items SET bom_spec_id=19031,bom_variant_id=19032,spec_g=0,
	 planned_units=24,planned_loose_g=0,need_g=0,input_g=6811,output_qty=24,output_unit='袋',
	 material_snapshot='[{"material_id":10,"material_name":"在制熟豆","unit":"kg","source":"bom","component_type":"material","consume_unit":"kg","qty_per_unit":0.28375,"output_qty":1,"output_unit":"袋"}]'
	 WHERE id=%[2]d;
	UPDATE %[1]s.work_orders SET bom_spec_id=19031,bom_variant_id=19032,spec_g=0,
	 output_qty=24,output_unit='袋',planned_g=0,planned_output_g=0 WHERE id=%[3]d;
	UPDATE %[1]s.work_order_material_reservations SET required_g=6811,reserved_g=6811
	 WHERE work_order_id=%[3]d AND material_id=10;
	UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=6811
	 WHERE work_order_id=%[3]d AND material_id=10;
	UPDATE %[1]s.material_batches SET remaining_g=6811 WHERE id=19030;
	UPDATE %[1]s.material_batch_locations SET qty_g=6811 WHERE material_batch_id=19030;
	UPDATE %[1]s.materials SET onhand_g=6811 WHERE id=10;
	`, schema, runningID, workOrderID))
	if shortage {
		mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.work_order_material_reservations SET reserved_g=6810 WHERE work_order_id=%[2]d AND material_id=10;
		UPDATE %[1]s.work_order_material_reservation_batches SET reserved_g=6810 WHERE work_order_id=%[2]d AND material_id=10;
		UPDATE %[1]s.material_batch_locations SET qty_g=6810 WHERE material_batch_id=19030;
		UPDATE %[1]s.material_batches SET remaining_g=6810 WHERE id=19030;
		UPDATE %[1]s.materials SET onhand_g=6810 WHERE id=10;
		`, schema, workOrderID))
	}
	for i, mode := range []string{"partial", "final"} {
		body := map[string]any{"completion_mode": mode, "request_id": fmt.Sprintf("PR703-rounding-%d", i), "finished_units": 12, "warehouse": "finished_goods"}
		rec := serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), body)
		if shortage && i == 1 {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("real shortage must remain blocked: %d %s", rec.Code, rec.Body.String())
			}
			assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=19031 AND onhand_units=12", 1)
			assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt'", workOrderID), 1)
			return
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s receipt: status=%d body=%s", mode, rec.Code, rec.Body.String())
		}
		if i == 0 {
			assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=10 AND deduct_g=3406", runningID), 1)
			// An identical retry cannot deduct another batch or receive more bags.
			rec = serveMultilevelProductionJSON(t, app, http.MethodPost, fmt.Sprintf("/api/produce/work-orders/%d/complete", workOrderID), body)
			if rec.Code != http.StatusOK {
				t.Fatalf("partial replay: %s", rec.Body.String())
			}
		}
	}
	var consumed int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT SUM(deduct_g)::bigint FROM %s.material_consumption_logs WHERE running_item_id=$1 AND material_id=10`, schema), runningID).Scan(&consumed); err != nil {
		t.Fatal(err)
	}
	if consumed != 6811 {
		t.Fatalf("total consumption=%dg, want exactly frozen 6811g", consumed)
	}
	assertProductionFlowCount(t, pool, schema, "material_consumption_logs", fmt.Sprintf("running_item_id=%d AND material_id=10 AND deduct_g=3405", runningID), 1)
	assertProductionFlowCount(t, pool, schema, "finished_inventory", "product_id=1 AND bom_spec_id=19031 AND onhand_units=24", 1)
	assertProductionFlowCount(t, pool, schema, "stock_entries", fmt.Sprintf("work_order_id=%d AND entry_type='finished_receipt'", workOrderID), 2)
	assertProductionFlowCount(t, pool, schema, "work_orders", fmt.Sprintf("id=%d AND status='completed'", workOrderID), 1)
}
