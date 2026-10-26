package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	app "orderapp/internal/application/production"
	"orderapp/internal/infrastructure/postgres/customerstock"
	"orderapp/internal/infrastructure/postgres/productspecmigration"
	"strings"
	"testing"
)

// Both reported paths receive a frozen old BOM output and then convert a
// previously reserved retained-sale promise to physical stock for a dropship.
func TestFrozenProductionReceiptConvertsRetainedSupplyAfterBOMRepublish(t *testing.T) {
	for _, recorded := range []bool{true, false} {
		t.Run(fmt.Sprintf("recorded_%t", recorded), func(t *testing.T) {
			pool, schema := newProductionFlowTestDB(t)
			ctx := context.Background()
			e, woID, runID := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 73601)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(900,100,'OLD-SPEC','old-spec','旧规格','袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default) VALUES(901,100,900,'旧规格','袋',true);
UPDATE %[1]s.order_items SET bom_spec_id=900,bom_variant_id=901,spec='旧规格',qty=14 WHERE order_id=1 AND product_id=1;
UPDATE %[1]s.production_plan_items SET bom_spec_id=900,bom_variant_id=901 WHERE product_id=1;
UPDATE %[1]s.work_orders SET product_id=1,output_product_id=1,bom_spec_id=900,bom_variant_id=901,bom_version_id=100,spec_g=0,output_qty=14,output_unit='袋',inventory_unit='袋',planned_inventory_qty=14,target_warehouse='fixture_customer',order_nos='FIXTURE-RETAINED' WHERE id=%[2]d;
UPDATE %[1]s.produce_running_items SET product_id=1,bom_spec_id=900,bom_variant_id=901,spec_g=0,order_nos='FIXTURE-RETAINED' WHERE id=%[3]d;
UPDATE %[1]s.job_cards SET actual_output_qty=14,actual_operation_cost=28,metrics_json='{"quantity_basis":"inventory_unit","inventory_unit":"袋"}' WHERE work_order_id=%[2]d;
UPDATE %[1]s.material_batches SET unit_cost=100 WHERE material_id=10;
INSERT INTO %[1]s.warehouses(code,name,kind,active,customer_id) VALUES('fixture_customer','隔离客户仓','customer_finished',true,1);
UPDATE %[1]s.orders SET order_no='FIXTURE-RETAINED',customer_id=1 WHERE id=1;
CREATE TABLE %[1]s.customer_direct_ship_request_orders(id BIGINT PRIMARY KEY,request_id BIGINT,order_id BIGINT);
CREATE TABLE %[1]s.customer_direct_ship_request_allocations(id BIGSERIAL PRIMARY KEY,request_id BIGINT,request_item_id BIGINT,request_order_id BIGINT,order_id BIGINT,order_item_id BIGINT,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,spec_g BIGINT,warehouse_code TEXT,batch_id BIGINT,batch_code TEXT,allocated_qty BIGINT,allocated_units BIGINT,allocated_g BIGINT,status TEXT);
CREATE TABLE IF NOT EXISTS %[1]s.order_stock_batch_allocations(id BIGSERIAL PRIMARY KEY,order_id BIGINT,order_item_id BIGINT,product_id BIGINT,bom_spec_id BIGINT,bom_variant_id BIGINT,spec_g BIGINT,need_g BIGINT,need_units BIGINT,batch_id BIGINT,batch_code TEXT,allocated_g BIGINT,allocated_units BIGINT,warehouse TEXT,request_id BIGINT,operator TEXT);
ALTER TABLE %[1]s.order_stock_batch_allocations ADD COLUMN IF NOT EXISTS order_item_id BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS need_units BIGINT NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS warehouse TEXT NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS request_id BIGINT NOT NULL DEFAULT 0;
`, schema, woID, runID))
			if err := customerstock.EnsureSchema(ctx, pool, schema); err != nil {
				t.Fatal(err)
			}
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
UPDATE %[1]s.orders SET delivery_mode='customer_stock',delivery_warehouse_code='fixture_customer' WHERE id=1;
INSERT INTO %[1]s.orders(id,order_no,customer_id) VALUES(73602,'FIXTURE-DROPSHIP',1);
INSERT INTO %[1]s.order_items(id,order_id,line_no,product_id,bom_spec_id,bom_variant_id,qty,item_name) VALUES(73602,73602,1,1,900,901,14,'旧冻结商品');
INSERT INTO %[1]s.customer_direct_ship_request_orders(id,request_id,order_id) VALUES(73602,73602,73602);
INSERT INTO %[1]s.customer_stock_supply_reservations(entitlement_id,customer_id,request_id,request_item_id,order_id,order_item_id,reserved_qty) SELECT id,1,73602,73602,73602,73602,14 FROM %[1]s.customer_stock_entitlements WHERE source_order_id=1;
UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=100;
INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,output_qty,output_unit) VALUES(902,100,'V2','published',1,'袋');
INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default) VALUES(902,902,900,'新版同规格','袋',true);
UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=902 WHERE output_type='product' AND output_id=1;
-- The retained sale is still frozen at V1, while its reserved dropship line
-- has since adopted the newly published V2 identity for the same spec.
UPDATE %[1]s.order_items SET bom_variant_id=902 WHERE id=73602;
`, schema))
			if err := productspecmigration.EnsureAuthorityProjection(ctx, pool, schema); err != nil {
				t.Fatal(err)
			}
			before := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
			var path string
			var payload map[string]any
			if recorded {
				preview := serveMultilevelProductionJSON(t, e, http.MethodPost, "/api/production/legacy-review/preview", map[string]any{"work_order_ids": []int64{woID}})
				var body struct {
					Rows []app.LegacyReview `json:"rows"`
				}
				if err := json.Unmarshal(preview.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 {
					t.Fatalf("preview: %s %v", preview.Body.String(), err)
				}
				path = fmt.Sprintf("/api/produce/work-orders/%d/receive-completed-output", woID)
				payload = map[string]any{"expected_version": body.Rows[0].Version, "request_id": "frozen-retained-receipt"}
			} else {
				mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`UPDATE %[1]s.work_orders SET output_qty=4,planned_inventory_qty=4,process_snapshot_json='{"operations":[{"seq":1,"operation":"包装"}]}' WHERE id=%[2]d; UPDATE %[1]s.job_cards SET status='running',operation='包装',planned_input_qty=14,actual_input_qty=0,actual_output_qty=0,completed_at=NULL WHERE work_order_id=%[2]d`, schema, woID))
				var cardID int64
				if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT id FROM %s.job_cards WHERE work_order_id=$1", schema), woID).Scan(&cardID); err != nil {
					t.Fatal(err)
				}
				preview := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/job-cards/%d/completion-preview", cardID), nil)
				var body app.OperationCompletionPreview
				if err := json.Unmarshal(preview.Body.Bytes(), &body); err != nil || !body.Enabled || body.BlockedReason != "" || body.PlannedQty != 4 {
					t.Fatalf("legacy packaging preview: %s %v", preview.Body.String(), err)
				}
				path = fmt.Sprintf("/api/job-cards/%d/complete", cardID)
				payload = map[string]any{"expected_version": body.Version, "request_id": "frozen-retained-complete", "actual_output_qty": 4, "output_unit": body.OutputUnit, "input_unit": body.InputUnit}
			}
			quantity := 14
			if !recorded {
				quantity = 4
			}
			// A failure after creating the ordinary allocation must roll back
			// the receipt, inventory, costs and promise conversion together.
			beforeFailure := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_fixture_allocation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture allocation failure'; END $$; CREATE TRIGGER reject_fixture_allocation BEFORE INSERT ON %[1]s.customer_direct_ship_request_allocations FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_fixture_allocation()`, schema))
			failed := serveMultilevelProductionJSON(t, e, http.MethodPost, path, payload)
			if failed.Code != http.StatusBadRequest || !strings.Contains(failed.Body.String(), "fixture allocation failure") {
				t.Fatalf("expected late allocation failure: %d %s", failed.Code, failed.Body.String())
			}
			if after := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID); after != beforeFailure {
				t.Fatalf("failed allocation changed production state: before=%+v after=%+v", beforeFailure, after)
			}
			for _, table := range []string{"order_stock_batch_allocations", "customer_direct_ship_request_allocations", "customer_stock_entitlement_receipts"} {
				assertProductionFlowCount(t, pool, schema, table, "true", 0)
			}
			assertProductionFlowCount(t, pool, schema, "customer_stock_supply_reservations", "converted_qty=0", 1)
			mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf("DROP TRIGGER reject_fixture_allocation ON %s.customer_direct_ship_request_allocations", schema))
			for attempt := 0; attempt < 2; attempt++ {
				result := serveMultilevelProductionJSON(t, e, http.MethodPost, path, payload)
				if result.Code != http.StatusOK {
					t.Fatalf("frozen receipt/replay %d: %d %s", attempt, result.Code, result.Body.String())
				}
			}
			for _, table := range []string{"order_stock_batch_allocations", "customer_direct_ship_request_allocations"} {
				assertProductionFlowCount(t, pool, schema, table, fmt.Sprintf("order_id=73602 AND product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND allocated_units=%d", quantity), 1)
			}
			assertProductionFlowCount(t, pool, schema, "customer_stock_supply_reservations", fmt.Sprintf("converted_qty=%d", quantity), 1)
			assertProductionFlowCount(t, pool, schema, "customer_stock_entitlement_receipts", fmt.Sprintf("quantity=%d", quantity), 1)
			assertProductionFlowCount(t, pool, schema, "finished_inventory", fmt.Sprintf("product_id=1 AND bom_spec_id=900 AND bom_variant_id=901 AND warehouse='fixture_customer' AND onhand_units=%d", quantity), 1)
			assertProductionFlowCount(t, pool, schema, "production_operation_events", "status='posted'", 1)
			assertProductionFlowCount(t, pool, schema, "production_batch_costs", fmt.Sprintf("running_item_id=%d AND total_cost>0", runID), 1)
			assertProductionFlowCount(t, pool, schema, "stock_entry_items", fmt.Sprintf("total_cost>0 AND stock_entry_id IN (SELECT id FROM %s.stock_entries WHERE work_order_id=%d AND entry_type='finished_receipt')", schema, woID), 1)
			// Historical exceptions require both proofs; they must not become
			// a general permission to assign arbitrary archived stock.
			for _, table := range []string{"order_stock_batch_allocations", "customer_direct_ship_request_allocations"} {
				for _, changed := range []string{"order_id", "order_item_id", "batch_id", "batch_code", "spec_g", "owner"} {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					fields := []string{"order_id", "order_item_id", "product_id", "bom_spec_id", "bom_variant_id", "spec_g", "batch_id", "batch_code"}
					values := append([]string(nil), fields...)
					for i, field := range fields {
						if field == changed {
							values[i] = "999999"
							if field == "batch_code" {
								values[i] = "'wrong-batch'"
							}
						}
					}
					if changed == "owner" {
						if _, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.stock_batches SET owner_customer_id=2 WHERE item_id=1", schema)); err != nil {
							_ = tx.Rollback(ctx)
							t.Fatal(err)
						}
					}
					_, err = tx.Exec(ctx, fmt.Sprintf("INSERT INTO %[1]s.%[2]s(%[3]s) SELECT %[4]s FROM %[1]s.%[2]s WHERE order_id=73602 LIMIT 1", schema, table, strings.Join(fields, ","), strings.Join(values, ",")))
					_ = tx.Rollback(ctx)
					if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
						t.Fatalf("%s mismatched %s was not rejected: %v", table, changed, err)
					}
				}
			}
			if recorded {
				after := loadProductCompletionAtomicityState(t, ctx, pool, schema, woID, runID)
				if after.ReservationConsumedG != before.ReservationConsumedG || after.MaterialLocationG != before.MaterialLocationG {
					t.Fatal("recorded output receipt consumed materials again")
				}
			}
		})
	}
}
