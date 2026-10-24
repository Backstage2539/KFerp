package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestOperationCompletionAPIProtectsProcessingReservation(t *testing.T) {
	for _, own := range []bool{false, true} {
		t.Run(fmt.Sprint(own), func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, woID, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 72911)
			target := int64(0)
			if own {
				target = woID
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=100,output_unit='袋',inventory_unit='袋',planned_inventory_qty=100,process_snapshot_json='{"operations":[{"seq":1,"execution_json":"{\"version\":1,\"node_key\":\"pack\",\"receipt_enabled\":true,\"output_unit\":\"袋\"}"}]}' WHERE id=%[2]d;
 UPDATE %[1]s.job_cards SET status='pending',sequence_no=1,actual_output_qty=0,actual_input_qty=0,completed_at=NULL WHERE work_order_id=%[2]d;
 CREATE TABLE %[1]s.customer_processing_material_reservations(id BIGSERIAL PRIMARY KEY,request_id BIGINT,request_item_id BIGINT,customer_id BIGINT,material_id BIGINT DEFAULT 0,component_product_id BIGINT DEFAULT 0,material_batch_id BIGINT DEFAULT 0,finished_stock_batch_id BIGINT DEFAULT 0,reserved_g BIGINT DEFAULT 0,reserved_units BIGINT DEFAULT 0,consumed_g BIGINT DEFAULT 0,consumed_units BIGINT DEFAULT 0,returned_g BIGINT DEFAULT 0,returned_units BIGINT DEFAULT 0,source_owner_type TEXT DEFAULT 'factory',source_customer_id BIGINT DEFAULT 0,source_warehouse_code TEXT,work_order_id BIGINT DEFAULT 0,status TEXT DEFAULT 'reserved',updated_at TIMESTAMPTZ DEFAULT now());
 INSERT INTO %[1]s.customer_processing_material_reservations(request_id,request_item_id,customer_id,material_id,material_batch_id,reserved_g,source_warehouse_code,work_order_id) SELECT 72911,72911,72911,10,id,22700,'wip',%[3]d FROM %[1]s.material_batches WHERE material_id=10 LIMIT 1;`, schema, woID, target))
			var card int64
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.job_cards WHERE work_order_id=$1 LIMIT 1`, schema), woID).Scan(&card); err != nil {
				t.Fatal(err)
			}
			res := serveMultilevelProductionJSON(t, e, "GET", fmt.Sprintf("/api/job-cards/%d/completion-preview", card), nil)
			var p app.OperationCompletionPreview
			if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			res = serveMultilevelProductionJSON(t, e, "POST", fmt.Sprintf("/api/job-cards/%d/complete", card), map[string]any{"actual_input_qty": p.InputQty, "actual_output_qty": 100, "input_unit": p.InputUnit, "output_unit": p.OutputUnit, "materials": p.Materials, "request_id": "processing-protection", "expected_version": p.Version})
			if own {
				if res.Code != 200 {
					t.Fatal(res.Body.String())
				}
				assertProductionFlowCount(t, pool, schema, "customer_processing_material_reservations", "consumed_g=22700 AND status='consumed'", 1)
			} else {
				if res.Code != 400 {
					t.Fatalf("other request consumed: %d %s", res.Code, res.Body.String())
				}
				assertProductionFlowCount(t, pool, schema, "production_operation_events", "true", 0)
			}
		})
	}
}
